// Package selfmanage verifies and replaces the running binary (PAWL-023).
//
// pawl decides what merges, and its attestations name the binary that produced
// them. A user who cannot answer "is the pawl I am running the one that was
// published?" cannot act on the digest in their own trails.
package selfmanage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const repo = "trunion-io/pawl"

var client = &http.Client{Timeout: 30 * time.Second}

// Status is what `pawl doctor` reports about the binary itself.
type Status string

const (
	// Verified: the running binary matches the checksum published for its version.
	Verified Status = "verified"
	// Mismatch: it does not. That is a finding, not a glitch.
	Mismatch Status = "mismatch"
	// Unverifiable: a development build, which has no published checksum and
	// never will (AC3). Calling that a failure trains people to ignore output.
	Unverifiable Status = "unverifiable"
	// Unchecked: the checksums could not be fetched (AC2). An unreachable
	// network is not evidence of authenticity, so this is never "verified".
	Unchecked Status = "unchecked"
)

type Result struct {
	Status   Status
	Version  string
	Path     string
	Digest   string
	Expected string
	Detail   string
}

func assetName(version string) string {
	name := fmt.Sprintf("pawl-%s-%s-%s", version, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// Verify checks the running binary against the checksums published for its
// version (AC1–AC3).
func Verify(version string) Result {
	r := Result{Version: version}

	exe, err := os.Executable()
	if err != nil {
		return Result{Status: Unchecked, Version: version, Detail: err.Error()}
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	r.Path = exe

	if version == "" || version == "dev" || strings.Contains(version, "-dirty") {
		r.Status = Unverifiable
		r.Detail = "a development build has no published checksum"
		return r
	}

	digest, err := fileDigest(exe)
	if err != nil {
		r.Status = Unchecked
		r.Detail = err.Error()
		return r
	}
	r.Digest = digest

	sums, err := fetch(fmt.Sprintf(
		"https://github.com/%s/releases/download/v%s/SHA256SUMS", repo, version))
	if err != nil {
		r.Status = Unchecked
		r.Detail = "could not fetch the published checksums: " + err.Error()
		return r
	}

	want := lookup(string(sums), assetName(version))
	if want == "" {
		r.Status = Unchecked
		r.Detail = assetName(version) + " is not listed in the published checksums"
		return r
	}
	r.Expected = want

	if want == digest {
		r.Status = Verified
	} else {
		r.Status = Mismatch
	}
	return r
}

func lookup(sums, asset string) string {
	for _, line := range strings.Split(sums, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			return f[0]
		}
	}
	return ""
}

func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fetch(url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

// LatestVersion resolves the newest published release.
func LatestVersion() (string, error) {
	body, err := fetch("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", err
	}
	return strings.TrimPrefix(rel.TagName, "v"), nil
}

// InCI reports whether this looks like a CI runner (AC10).
//
// PAWL-013 tells clients to pin by digest because the version that runs decides
// whether their changesets merge. A CI job that upgrades itself has silently
// unpinned the thing they pinned.
func InCI() bool {
	for _, k := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "JENKINS_URL", "TF_BUILD"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

// Upgrade downloads a version, verifies it, and replaces the running binary
// (AC5–AC8).
//
// Verification happens before anything is replaced. A tool that can replace
// itself is a tool that can be made to replace itself with something else, and
// this check is the whole of what makes that acceptable in something that
// decides what merges.
func Upgrade(version string) (path string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	asset := assetName(version)
	base := fmt.Sprintf("https://github.com/%s/releases/download/v%s", repo, version)

	binary, err := fetch(base + "/" + asset)
	if err != nil {
		return exe, fmt.Errorf("downloading %s: %w", asset, err)
	}
	sums, err := fetch(base + "/SHA256SUMS")
	if err != nil {
		return exe, fmt.Errorf("release has no published checksums; refusing to install unverified: %w", err)
	}

	want := lookup(string(sums), asset)
	if want == "" {
		return exe, fmt.Errorf("%s is not listed in the published checksums; refusing to install", asset)
	}
	sum := sha256.Sum256(binary)
	got := hex.EncodeToString(sum[:])
	if got != want {
		// AC6: abort leaving the existing binary untouched.
		return exe, fmt.Errorf("checksum mismatch for %s\n  expected %s\n  got      %s\n"+
			"Refusing to replace anything. A mismatch is a finding, not a glitch", asset, want, got)
	}

	// AC7: write beside the target and rename, so the binary on the PATH is
	// always a whole one. A half-written binary is worse than an old one.
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".pawl-upgrade-*")
	if err != nil {
		// AC8: name the path rather than failing obscurely.
		return exe, fmt.Errorf("cannot write to %s: %w\nInstall elsewhere, or re-run with the rights to write there", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return exe, err
	}
	if err := tmp.Close(); err != nil {
		return exe, err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return exe, err
	}
	if err := os.Rename(tmpName, exe); err != nil {
		return exe, fmt.Errorf("cannot replace %s: %w", exe, err)
	}
	return exe, nil
}

// Command nextversion computes the next version from commit history (PAWL-027).
//
// Not part of the shipped binary — `make dist` builds ./cmd/pawl only. It runs
// in CI via `go run`, which keeps release tooling out of the artifact clients
// install while still holding it to the same tests as everything else.
//
// Output is written as GITHUB_OUTPUT key=value lines when that variable is set,
// and to stdout otherwise so it can be run by hand.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"trunion.io/pawl/internal/release"
)

const recordSep = "\x1e"

func main() {
	rc := flag.Bool("rc", false, "compute the next release-candidate tag rather than a release")
	notesFile := flag.String("notes-file", "", "write release notes for the included commits to this file (AC16)")
	flag.Parse()

	prev, prevTag := previousRelease()

	commits, unconventional, err := commitsSince(prevTag)
	if err != nil {
		fail(err)
	}

	bump := release.BumpFor(commits)
	next, applied := release.Apply(prev, bump)

	out := map[string]string{
		"previous":       prev.String(),
		"previous_tag":   prevTag,
		"bump":           applied.String(),
		"version":        next.String(),
		"commits":        strconv.Itoa(len(commits)),
		"unconventional": strconv.Itoa(unconventional),
	}

	// AC10: nothing that implies a version change means no candidate and no
	// release. Tagging a docs-only commit would bury the real candidates.
	if applied == release.BumpNone {
		out["release"] = "false"
		out["tag"] = ""
		emit(out)
		return
	}
	out["release"] = "true"

	if *notesFile != "" {
		notes := release.Notes(commits, prevTag, "v"+next.String(), applied)
		if err := os.WriteFile(*notesFile, []byte(notes), 0o644); err != nil {
			fail(err)
		}
	}

	if *rc {
		// A candidate already pointing at this commit is reused rather than
		// superseded. Without this the workflow is not idempotent: it fires once
		// per finishing workflow, so a second run for one commit would see rc.1
		// exists, compute rc.2, and tag the same commit twice. tag.sh's
		// --retry-same-commit could never fire, because it was never handed a
		// tag that already existed.
		if tag, n := existingCandidateAt(next.String(), "HEAD"); tag != "" {
			out["tag"] = tag
			out["rc"] = strconv.Itoa(n)
			out["reused"] = "true"
		} else {
			n := nextRCNumber(next.String())
			out["tag"] = fmt.Sprintf("v%s-rc.%d", next, n)
			out["rc"] = strconv.Itoa(n)
			out["reused"] = "false"
		}
	} else {
		tag := "v" + next.String()
		// AC15: refuse to release over an existing tag.
		if tagExists(tag) {
			fail(fmt.Errorf("%s already exists; refusing to release over it", tag))
		}
		out["tag"] = tag
	}
	emit(out)
}

// previousRelease finds the newest release tag, ignoring candidates. Returns
// 0.0.0 when there is none, which is the state of a repository before its first
// release and must not be an error.
//
// A *failed* `git tag` is a different thing entirely and is fatal. Treating it
// as "no releases" is C-3 in the release path: the check did not run, so its
// silence is not evidence of the state it would have reported. The consequence
// is concrete — on a repository already at v3.0.0, a git failure would compute a
// fresh low version and the workflow would tag and publish it.
func previousRelease() (release.Version, string) {
	out, err := git("tag", "--list", "v*", "--sort=-v:refname")
	if err != nil {
		fail(fmt.Errorf("cannot list tags, so the previous release is unknown: %w", err))
	}
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.Contains(t, "-") { // skip prereleases
			continue
		}
		if v, err := release.ParseVersion(t); err == nil {
			return v, t
		}
	}
	return release.Version{}, ""
}

// commitsSince parses every commit after tag. A commit that is not Conventional
// Commits is counted rather than fatal: history predating PAWL-027 will not
// conform, and rewriting it to make a tool happy is the wrong trade.
func commitsSince(tag string) ([]release.Commit, int, error) {
	rang := "HEAD"
	if tag != "" {
		rang = tag + "..HEAD"
	}
	out, err := git("log", "--no-merges", "--format=%B"+recordSep, rang)
	if err != nil {
		return nil, 0, err
	}

	var commits []release.Commit
	unconventional := 0
	for _, raw := range strings.Split(out, recordSep) {
		msg := strings.TrimSpace(raw)
		if msg == "" {
			continue
		}
		c, ok := release.ParseCommit(msg)
		// An unrecognised type is as unconventional as an unparseable header
		// (AC2). Counting it as conventional would report a clean history while
		// `chore-ish: x` silently contributed nothing to the bump.
		if !ok || !release.KnownType(c.Type) {
			unconventional++
			continue
		}
		commits = append(commits, c)
	}
	return commits, unconventional, nil
}

// existingCandidateAt returns the candidate tag for version that already points
// at ref, if there is one.
//
// AC9 says candidate numbers are never reused. Reusing the *tag* for the same
// commit is the opposite of that: it is what stops a second run minting a new
// number for work that has already been tagged.
func existingCandidateAt(version, ref string) (string, int) {
	want, err := git("rev-parse", ref+"^{commit}")
	if err != nil {
		fail(fmt.Errorf("cannot resolve %s: %w", ref, err))
	}
	want = strings.TrimSpace(want)

	out, err := git("tag", "--list", "v"+version+"-rc.*")
	if err != nil {
		fail(fmt.Errorf("cannot list candidate tags for %s: %w", version, err))
	}
	prefix := "v" + version + "-rc."
	for _, line := range strings.Split(out, "\n") {
		tag := strings.TrimSpace(line)
		if !strings.HasPrefix(tag, prefix) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(tag, prefix))
		if err != nil {
			continue
		}
		at, err := git("rev-parse", tag+"^{commit}")
		if err != nil {
			fail(fmt.Errorf("cannot resolve %s: %w", tag, err))
		}
		if strings.TrimSpace(at) != want {
			continue
		}

		// Only an annotated tag counts as a candidate already published.
		// tag.sh refuses a lightweight one, so reusing it here would hand back
		// the same name on every attempt and the workflow would exhaust its
		// retries without advancing — two correct checks that deadlock when
		// composed. Leaving it unclaimed lets nextRCNumber step past it.
		kind, err := git("cat-file", "-t", "refs/tags/"+tag)
		if err != nil {
			fail(fmt.Errorf("cannot determine the type of %s: %w", tag, err))
		}
		if strings.TrimSpace(kind) == "tag" {
			return tag, n
		}
	}
	return "", 0
}

// nextRCNumber returns one more than the highest candidate for this version
// (AC9). Numbers are never reused, so a withdrawn candidate leaves a gap rather
// than being overwritten.
func nextRCNumber(version string) int {
	out, err := git("tag", "--list", "v"+version+"-rc.*")
	if err != nil {
		// Falling back to 1 would reuse a candidate number, which AC9 forbids,
		// and would do it precisely when we cannot see what already exists.
		fail(fmt.Errorf("cannot list candidate tags for %s: %w", version, err))
	}
	var used []int
	prefix := "v" + version + "-rc."
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, prefix) {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(t, prefix)); err == nil {
			used = append(used, n)
		}
	}
	if len(used) == 0 {
		return 1
	}
	sort.Ints(used)
	return used[len(used)-1] + 1
}

// tagExists backs AC15, which refuses to release over an existing tag. An error
// must not read as "absent": that would turn the guard into a no-op at the only
// moment it matters.
func tagExists(tag string) bool {
	out, err := git("tag", "--list", tag)
	if err != nil {
		fail(fmt.Errorf("cannot check whether %s already exists: %w", tag, err))
	}
	return strings.TrimSpace(out) != ""
}

func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	return string(b), err
}

func emit(out map[string]string) {
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if p := os.Getenv("GITHUB_OUTPUT"); p != "" {
		f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fail(err)
		}
		defer f.Close()
		for _, k := range keys {
			fmt.Fprintf(f, "%s=%s\n", k, out[k])
		}
	}
	for _, k := range keys {
		fmt.Printf("%s=%s\n", k, out[k])
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "nextversion:", err)
	os.Exit(1)
}

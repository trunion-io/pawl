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
		n := nextRCNumber(next.String())
		out["tag"] = fmt.Sprintf("v%s-rc.%d", next, n)
		out["rc"] = strconv.Itoa(n)
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
func previousRelease() (release.Version, string) {
	out, err := git("tag", "--list", "v*", "--sort=-v:refname")
	if err != nil {
		return release.Version{}, ""
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

// nextRCNumber returns one more than the highest candidate for this version
// (AC9). Numbers are never reused, so a withdrawn candidate leaves a gap rather
// than being overwritten.
func nextRCNumber(version string) int {
	out, err := git("tag", "--list", "v"+version+"-rc.*")
	if err != nil {
		return 1
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

func tagExists(tag string) bool {
	out, err := git("tag", "--list", tag)
	return err == nil && strings.TrimSpace(out) != ""
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

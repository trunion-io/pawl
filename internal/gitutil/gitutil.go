// Package gitutil is a thin git wrapper. Subprocess rather than a library
// dependency: this has to drop into a client repo and CI runner with nothing
// installed but the CLI.
package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"trunion.io/pawl/internal/model"
)

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

type Error struct {
	Args   []string
	Stderr string
}

func (e *Error) Error() string {
	return fmt.Sprintf("git %s failed: %s", strings.Join(e.Args, " "), e.Stderr)
}

func run(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String())}
	}
	return string(out), nil
}

func TreeHash(repo, rev string) (string, error) {
	out, err := run(repo, "rev-parse", rev+"^{tree}")
	return strings.TrimSpace(out), err
}

func CommitSHA(repo, rev string) (string, error) {
	out, err := run(repo, "rev-parse", rev)
	return strings.TrimSpace(out), err
}

// RemoteURL returns "" when there is no origin, matching the Python version's
// None. A repo with no remote is normal in a test fixture and not an error.
func RemoteURL(repo string) string {
	out, err := run(repo, "config", "--get", "remote.origin.url")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func MergeBase(repo, base, head string) (string, error) {
	out, err := run(repo, "merge-base", base, head)
	return strings.TrimSpace(out), err
}

// DefaultExcludes: the claim log is part of the changeset on disk but is not
// code anyone reviews. Counting it would inflate the denominator and put the
// trail on its own reading list, which is both wrong and embarrassing in front
// of a client.
var DefaultExcludes = []string{".pawl/"}

// ChangedHunks returns added and modified line spans in head relative to base.
//
// Deletions are not returned: there is no line in the delivered tree for a human
// to read. A claim can still be attached to the surrounding context, which is
// the right place for "why this went away".
func ChangedHunks(repo, base, head string, excludes []string) ([]model.Hunk, error) {
	if excludes == nil {
		excludes = DefaultExcludes
	}
	diff, err := run(repo, "diff", "--unified=0", "--no-color", base+"..."+head)
	if err != nil {
		return nil, err
	}
	return parseHunks(diff, excludes), nil
}

// WorktreeHunks returns changed line spans in the working tree relative to HEAD,
// staged or not.
//
// This is the edit-time question — "what have I changed and not yet committed?"
// — as opposed to ChangedHunks' PR-time question. PAWL-016 needs it because a
// hook fires while the edit is still uncommitted.
func WorktreeHunks(repo string, excludes []string) ([]model.Hunk, error) {
	if excludes == nil {
		excludes = DefaultExcludes
	}
	diff, err := run(repo, "diff", "--unified=0", "--no-color", "HEAD")
	if err != nil {
		return nil, err
	}
	hunks := parseHunks(diff, excludes)

	// `git diff HEAD` does not mention untracked files, so a brand new file
	// would report nothing pending and an agent creating one would never be
	// asked to account for it — the largest possible accounting gap, hiding in
	// the most ordinary action.
	//
	// Listing them separately keeps the index untouched, which matters: a hook
	// runs on every edit and must not mutate git state as a side effect.
	// --exclude-standard honours .gitignore, so build output and the record log
	// stay out.
	untracked, err := run(repo, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return hunks, nil // best effort; a hook must still get the tracked spans
	}
	for _, path := range strings.Split(untracked, "\n") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		skip := false
		for _, prefix := range excludes {
			if strings.HasPrefix(path, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if lines := ReadWorktree(repo, path); len(lines) > 0 {
			hunks = append(hunks, model.Hunk{Path: path, StartLine: 1, EndLine: len(lines)})
		}
	}
	return hunks, nil
}

func parseHunks(diff string, excludes []string) []model.Hunk {
	var hunks []model.Hunk
	currentPath := ""
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			currentPath = line[6:]
			for _, prefix := range excludes {
				if strings.HasPrefix(currentPath, prefix) {
					currentPath = ""
					break
				}
			}
			continue
		}
		if strings.HasPrefix(line, "+++ /dev/null") {
			currentPath = ""
			continue
		}
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil || currentPath == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		if count == 0 {
			continue // pure deletion
		}
		hunks = append(hunks, model.Hunk{
			Path:      currentPath,
			StartLine: start,
			EndLine:   start + count - 1,
		})
	}
	return hunks
}

// ReadFileAt returns nil when the path does not exist at rev, which the anchor
// layer reads as an orphaned claim.
func ReadFileAt(repo, path, rev string) []string {
	content, err := run(repo, "show", rev+":"+path)
	if err != nil {
		return nil
	}
	return model.SplitLines(content)
}

func ReadWorktree(repo, path string) []string {
	b, err := os.ReadFile(filepath.Join(repo, path))
	if err != nil {
		return nil
	}
	return model.SplitLines(string(b))
}

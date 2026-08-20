package e2e

// PAWL-027 AC15 — the release path refuses to release over an existing tag.
//
// .github/scripts/tag.sh is the only place a tag is written, and it had no test.
// Three defects have now been found in this path — a stale check list, a missing
// tagger, and an idempotent early exit that let a release continue as though it
// had tagged a commit it had not. All three were in the least-exercised code
// here, which is the argument for testing it rather than the argument against.
//
// Real git in temporary directories, including a real bare remote so the push is
// exercised rather than mocked (C-9).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func tagScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../.github/scripts/tag.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("tag.sh not found: %v", err)
	}
	return p
}

// tagRepo builds a work repo with a real bare remote and two commits, returning
// the work tree and the two commit SHAs.
func tagRepo(t *testing.T) (dir, first, second string) {
	t.Helper()
	base := t.TempDir()
	bare := filepath.Join(base, "origin.git")
	dir = filepath.Join(base, "work")

	run := func(d string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = d
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}

	run(base, "init", "--bare", "-q", bare)
	run(base, "init", "-q", dir)
	// Deliberately no user.name / user.email: the bug this guards against was a
	// runner with no identity configured.
	run(dir, "remote", "add", "origin", bare)
	run(dir, "-c", "user.name=t", "-c", "user.email=t@e", "commit", "-q", "--allow-empty", "-m", "one")
	first = run(dir, "rev-parse", "HEAD")
	run(dir, "-c", "user.name=t", "-c", "user.email=t@e", "commit", "-q", "--allow-empty", "-m", "two")
	second = run(dir, "rev-parse", "HEAD")
	run(dir, "push", "-q", "origin", "HEAD:refs/heads/main")
	return dir, first, second
}

func runTag(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(tagScript(t), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func tagTarget(t *testing.T, dir, tag string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", tag+"^{commit}")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// It must write an annotated tag in a repository with no identity configured —
// the scenario that failed every release candidate.
func TestTagScriptWritesAnnotatedTagWithoutAnIdentity(t *testing.T) {
	dir, _, second := tagRepo(t)

	out, err := runTag(t, dir, "v0.1.0", "HEAD", "subject", "body")
	if err != nil {
		t.Fatalf("expected success, got %v\n%s", err, out)
	}
	if got := tagTarget(t, dir, "v0.1.0"); got != second {
		t.Errorf("tag points at %s, want %s", got, second)
	}

	cmd := exec.Command("git", "for-each-ref", "--format=%(taggername) %(taggeremail)", "refs/tags/v0.1.0")
	cmd.Dir = dir
	b, _ := cmd.Output()
	if !strings.Contains(string(b), "github-actions[bot]") {
		t.Errorf("tag has no tagger identity: %q", strings.TrimSpace(string(b)))
	}
}

// AC15: without --retry-same-commit an existing tag is fatal, even when it
// points at the same commit. The release path must not continue as though it
// tagged something it did not.
func TestTagScriptRefusesAnExistingTagByDefault(t *testing.T) {
	dir, _, _ := tagRepo(t)

	if out, err := runTag(t, dir, "v0.1.0", "HEAD", "first"); err != nil {
		t.Fatalf("setup tag failed: %v\n%s", err, out)
	}
	out, err := runTag(t, dir, "v0.1.0", "HEAD", "again")
	if err == nil {
		t.Fatalf("expected refusal, got success:\n%s", out)
	}
	if !strings.Contains(out, "already exists") {
		t.Errorf("error must say the tag exists, got: %s", out)
	}
}

// Candidates may legitimately run twice for one commit.
func TestTagScriptRetryIsIdempotentForTheSameCommit(t *testing.T) {
	dir, _, second := tagRepo(t)

	if out, err := runTag(t, dir, "--retry-same-commit", "v0.1.0-rc.1", "HEAD", "first"); err != nil {
		t.Fatalf("first call failed: %v\n%s", err, out)
	}
	out, err := runTag(t, dir, "--retry-same-commit", "v0.1.0-rc.1", "HEAD", "second")
	if err != nil {
		t.Fatalf("retry for the same commit must succeed, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "nothing to do") {
		t.Errorf("expected a no-op, got: %s", out)
	}
	if got := tagTarget(t, dir, "v0.1.0-rc.1"); got != second {
		t.Errorf("tag moved to %s, want %s", got, second)
	}
}

// A candidate tag on a *different* commit is a real problem, and retry mode must
// not paper over it.
func TestTagScriptRefusesRetryOnADifferentCommit(t *testing.T) {
	dir, first, _ := tagRepo(t)

	if out, err := runTag(t, dir, "--retry-same-commit", "v0.1.0-rc.1", first, "first"); err != nil {
		t.Fatalf("setup tag failed: %v\n%s", err, out)
	}
	out, err := runTag(t, dir, "--retry-same-commit", "v0.1.0-rc.1", "HEAD", "second")
	if err == nil {
		t.Fatalf("expected refusal when the tag names another commit, got:\n%s", out)
	}
	if !strings.Contains(out, "already exists") {
		t.Errorf("error must say the tag exists, got: %s", out)
	}
	if got := tagTarget(t, dir, "v0.1.0-rc.1"); got != first {
		t.Errorf("the existing tag was moved to %s; it must be left alone", got)
	}
}

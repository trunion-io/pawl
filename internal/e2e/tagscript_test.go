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

// pushedTag returns the tag's ref on origin, or "" if it never arrived. The
// suite claimed a real bare remote exercised the push and nothing asserted it —
// deleting `git push` from tag.sh left every test passing.
func pushedTag(t *testing.T, dir, tag string) string {
	t.Helper()
	cmd := exec.Command("git", "ls-remote", "--tags", "origin", "refs/tags/"+tag)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ls-remote: %v", err)
	}
	return strings.TrimSpace(string(out))
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
	if pushedTag(t, dir, "v0.1.0") == "" {
		t.Error("the tag was never pushed to origin; a workflow would report success without publishing it")
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
	// Asserting behaviour rather than wording: the tag must not move, and must
	// be on origin. An earlier version of this test matched the message text and
	// broke when the message changed, which tested the string and not the
	// property.
	if got := tagTarget(t, dir, "v0.1.0-rc.1"); got != second {
		t.Errorf("tag moved to %s, want %s", got, second)
	}
	if pushedTag(t, dir, "v0.1.0-rc.1") == "" {
		t.Error("retry left origin without the candidate")
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

// Both workflows passed a subject and body paragraphs before tag.sh existed.
// Joining them with "$*" would have collapsed the annotation to one line.
func TestTagScriptPreservesMessageParagraphs(t *testing.T) {
	dir, _, _ := tagRepo(t)

	if out, err := runTag(t, dir, "v0.1.0", "HEAD",
		"Release v0.1.0", "Bump: minor from 0.0.1", "Third paragraph"); err != nil {
		t.Fatalf("tag failed: %v\n%s", err, out)
	}

	cmd := exec.Command("git", "tag", "-l", "--format=%(contents)", "v0.1.0")
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimRight(string(b), "\n")

	want := "Release v0.1.0\n\nBump: minor from 0.0.1\n\nThird paragraph"
	if got != want {
		t.Errorf("annotation lost its paragraphs:\n got: %q\nwant: %q", got, want)
	}
}

// nextversion --rc must reuse a candidate that already points at this commit.
//
// It fires once per finishing workflow, so it can legitimately run twice for one
// commit. Returning max+1 unconditionally meant the second run computed rc.2 and
// tagged the same commit again — and tag.sh's --retry-same-commit could never
// fire, because it was never handed a tag that already existed.
func TestRCReusesTheCandidateForTheSameCommit(t *testing.T) {
	dir, _, _ := tagRepo(t)

	nv := filepath.Join(t.TempDir(), "nextversion")
	build := exec.Command("go", "build", "-o", nv, "./internal/release/nextversion")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building nextversion: %v\n%s", err, out)
	}

	run := func() map[string]string {
		t.Helper()
		cmd := exec.Command(nv, "--rc")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("nextversion --rc: %v", err)
		}
		kv := map[string]string{}
		for _, line := range strings.Split(string(out), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
				kv[k] = v
			}
		}
		return kv
	}

	// The seed commits are not conventional, so give it something to bump on.
	c := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@e",
		"commit", "-q", "--allow-empty", "-m", "feat: something")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	first := run()
	if first["reused"] != "false" {
		t.Fatalf("first run should mint a candidate, got %+v", first)
	}
	if out, err := runTag(t, dir, "--retry-same-commit", first["tag"], "HEAD", "candidate"); err != nil {
		t.Fatalf("tagging the candidate failed: %v\n%s", err, out)
	}

	second := run()
	if second["reused"] != "true" {
		t.Errorf("a second run for the same commit must reuse, got %+v", second)
	}
	if second["tag"] != first["tag"] {
		t.Errorf("second run computed %s, want %s", second["tag"], first["tag"])
	}

	// A new commit still advances.
	c = exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@e",
		"commit", "-q", "--allow-empty", "-m", "feat: another")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
	third := run()
	if third["reused"] != "false" || third["tag"] == first["tag"] {
		t.Errorf("a new commit must get a new candidate, got %+v", third)
	}
}

// A lightweight tag points at the right commit while carrying no annotation and
// no tagger — the exact defect this script exists to prevent. Retry mode must
// not accept one as a completed candidate.
func TestTagScriptRefusesRetryOnALightweightTag(t *testing.T) {
	dir, _, _ := tagRepo(t)

	lw := exec.Command("git", "tag", "v0.1.0-rc.1")
	lw.Dir = dir
	if out, err := lw.CombinedOutput(); err != nil {
		t.Fatalf("creating a lightweight tag: %v\n%s", err, out)
	}

	out, err := runTag(t, dir, "--retry-same-commit", "v0.1.0-rc.1", "HEAD", "candidate")
	if err == nil {
		t.Fatalf("expected refusal for a lightweight tag, got success:\n%s", out)
	}
	if !strings.Contains(out, "not an annotated tag") {
		t.Errorf("the error must say why, got: %s", out)
	}
}

// A tag created locally by an earlier run that failed to push is not a published
// candidate. Retrying must reconcile the remote rather than reporting success on
// the strength of the local ref alone.
func TestTagScriptRetryPushesATagTheRemoteNeverGot(t *testing.T) {
	dir, _, second := tagRepo(t)

	// Simulate the earlier run: annotated tag created, push never happened.
	for _, args := range [][]string{
		{"config", "user.name", "github-actions[bot]"},
		{"config", "user.email", "41898282+github-actions[bot]@users.noreply.github.com"},
		{"tag", "-a", "v0.1.0-rc.1", "-m", "candidate"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if pushedTag(t, dir, "v0.1.0-rc.1") != "" {
		t.Fatal("precondition: the tag must not be on origin yet")
	}

	out, err := runTag(t, dir, "--retry-same-commit", "v0.1.0-rc.1", "HEAD", "candidate")
	if err != nil {
		t.Fatalf("retry should reconcile the remote, got %v\n%s", err, out)
	}
	if pushedTag(t, dir, "v0.1.0-rc.1") == "" {
		t.Error("retry returned success while origin still has no candidate")
	}
	if got := tagTarget(t, dir, "v0.1.0-rc.1"); got != second {
		t.Errorf("tag moved to %s, want %s", got, second)
	}
}

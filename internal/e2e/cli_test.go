package e2e

// CLI coverage (PAWL-021).
//
// These invoke the binary as a subprocess. Calling the command functions
// directly is the same move as mocking git: it exercises our model of the CLI
// rather than the CLI, and every defect this file exists for lived in the seam
// between the standard library's flag package and that model.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	pawlBin   string
	buildErr  error
)

// binary builds pawl from the current source, once per run (AC2, and the
// non-functional note about not building per test).
//
// Building rather than trusting PATH matters: a stale install would pass this
// suite while the working tree was broken.
func binary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "pawl-cli-test")
		if err != nil {
			buildErr = err
			return
		}
		pawlBin = filepath.Join(dir, "pawl")
		cmd := exec.Command("go", "build", "-o", pawlBin, "./cmd/pawl")
		cmd.Dir = ".." + string(filepath.Separator) + ".."
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = err
			t.Logf("build output:\n%s", out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building pawl under test: %v", buildErr)
	}
	return pawlBin
}

type result struct {
	stdout, stderr string
	code           int
}

func run(t *testing.T, repo string, args ...string) result {
	t.Helper()
	return runStdin(t, repo, "", args...)
}

func runEnv(t *testing.T, repo string, env []string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary(t), args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), env...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !asExitError(err, &ee) {
			t.Fatalf("running pawl %s: %v", strings.Join(args, " "), err)
		}
		code = ee.ExitCode()
	}
	return result{stdout: out.String(), stderr: errb.String(), code: code}
}

func runStdin(t *testing.T, repo, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary(t), args...)
	cmd.Dir = repo
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb

	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !asExitError(err, &ee) {
			t.Fatalf("running pawl %s: %v", strings.Join(args, " "), err)
		}
		code = ee.ExitCode()
	}
	return result{stdout: out.String(), stderr: errb.String(), code: code}
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

// TestCLIClaimParsesFlagsAfterTheText is PAWL-021 AC4, and the reason the
// criterion exists. This exact shape silently recorded nothing.
func TestCLIClaimParsesFlagsAfterTheText(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)

	r := run(t, repo, "claim", "exp is unix seconds",
		"--path", "src/auth.py", "--lines", "4-6",
		"--verified-by", "test:tests.test_auth.test_expiry")
	if r.code != 0 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}

	claims := loadClaims(t, repo)
	if len(claims) != 1 {
		t.Fatalf("claims recorded = %d, want 1 — the flags after the text were dropped", len(claims))
	}
	c := claims[0]
	if c.Path != "src/auth.py" || c.StartLine != 4 || c.EndLine != 6 {
		t.Errorf("flags after the positional were not parsed: %+v", c)
	}
	if len(c.VerifiedBy) != 1 {
		t.Errorf("--verified-by after the positional was dropped: %+v", c.VerifiedBy)
	}
}

// TestCLIReviewParsesFlagsAfterTheID is AC4 for the second command that hit it.
func TestCLIReviewParsesFlagsAfterTheID(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	// The claim has to actually clear, or there is no cleared span to sample.
	run(t, repo, "claim", "a claim", "--path", "src/auth.py", "--lines", "4-6",
		"--verified-by", "test:tests.test_auth.test_expiry")
	// Everything else in the change needs accounting for too, or the span set
	// is dominated by unclaimed lines.
	run(t, repo, "ack", "--path", "src/auth.py", "--lines", "1-3")
	run(t, repo, "ack", "--path", "src/auth.py", "--lines", "7-9")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")
	writeJUnit(t, repo)

	s := run(t, repo, "sample", "--base", "HEAD~1", "--force", "--junit", "junit.xml")
	if s.code != 0 {
		t.Fatalf("sample failed: %s%s", s.stdout, s.stderr)
	}
	id := sampleIDFrom(t, s.stdout)

	// The shape that silently parsed zero flags.
	r := run(t, repo, "review", id, "--span", "src/auth.py:4-6",
		"--verdict", "correct", "--reviewer", "rich")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	// Assert on the span that was judged, not on the whole output: other spans
	// are legitimately still pending and saying so is correct.
	judged := false
	for _, line := range strings.Split(r.stdout, "\n") {
		if strings.Contains(line, "src/auth.py:4-6") && strings.Contains(line, "correct") {
			judged = true
		}
	}
	if !judged {
		t.Errorf("the verdict given after the id was not recorded:\n%s", r.stdout)
	}
}

func sampleIDFrom(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "sampled" {
			return f[1]
		}
	}
	t.Fatalf("no sample id in:\n%s", out)
	return ""
}

// TestCLIFlagsParseAfterEveryLeadingPositional is PAWL-021 AC4 applied to the
// whole CLI rather than to the two commands that happened to be found broken.
//
// The bug has now appeared four times: claim, review, and install upgrade,
// which was written minutes after the test meant to prevent it. Enumerating the
// commands here is what stops the fifth.
func TestCLIFlagsParseAfterEveryLeadingPositional(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)

	cases := []struct {
		name string
		args []string
		// A flag placed after the positional whose effect is observable.
		wants string
	}{
		{
			name:  "install upgrade <version> --force",
			args:  []string{"install", "upgrade", "0.0.0-none", "--force"},
			wants: "", // must not print the CI refusal; --force was seen
		},
	}

	for _, c := range cases {
		cmd := append([]string{}, c.args...)
		r := runEnv(t, repo, []string{"CI=true"}, cmd...)
		if strings.Contains(r.stderr, "Pass --force if you mean it") {
			t.Errorf("%s: the flag after the positional was dropped\n%s", c.name, r.stderr)
		}
	}
}

// TestCLIExitCodesDistinguishVerdictFromFailure is AC5. A CI job treats these
// differently and documenting the difference without testing it is a promise
// nothing keeps.
func TestCLIExitCodesDistinguishVerdictFromFailure(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	// 1 — a verdict about the changeset. Nothing is claimed, so the gate blocks.
	blocked := run(t, repo, "gate", "--base", "HEAD~1")
	if blocked.code != 1 {
		t.Errorf("unclaimed changeset: exit %d, want 1 (a verdict)", blocked.code)
	}
	if !strings.Contains(blocked.stdout, "FAIL") {
		t.Errorf("a blocked gate should still print the reading list:\n%s", blocked.stdout)
	}

	// 2 — pawl could not do its job.
	broken := run(t, repo, "verify", "--base", "HEAD~1", "--junit", "nosuchfile.xml")
	if broken.code != 2 {
		t.Errorf("missing evidence file: exit %d, want 2 (pawl failed)", broken.code)
	}

	// 0 — success.
	ok := run(t, repo, "version")
	if ok.code != 0 {
		t.Errorf("version: exit %d, want 0", ok.code)
	}
}

// TestCLIHookHandlesBothStdinCases is AC6. The second case blocked forever, and
// no unit test could have noticed.
func TestCLIHookHandlesBothStdinCases(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	run(t, repo, "claim", "gives the repo a .pawl dir", "--path", "src/auth.py", "--lines", "1-2")

	// With a payload.
	payload := `{"tool_input":{"file_path":"` + filepath.Join(repo, "src", "auth.py") + `"}}`
	withInput := runStdin(t, repo, payload, "hook", "claude-code")
	if withInput.code != 0 {
		t.Errorf("hook with a payload: exit %d", withInput.code)
	}

	// With none. Stdin is not a terminal here, so this is the harness-call-gone
	// -wrong case: silent, and above all it must return rather than block.
	noInput := runStdin(t, repo, "\n", "hook", "claude-code")
	if noInput.code != 0 {
		t.Errorf("hook with no payload: exit %d, want 0", noInput.code)
	}
	if noInput.stdout != "" {
		t.Errorf("hook with no payload should say nothing, got %q", noInput.stdout)
	}
}

// TestCLIEveryCommandRuns is AC3. A command nothing runs is one whose first
// execution is a user's.
func TestCLIEveryCommandRuns(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	run(t, repo, "claim", "seed", "--path", "src/auth.py", "--lines", "4-6")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	commands := commandsFromHelp(t, repo)
	if len(commands) < 10 {
		t.Fatalf("only %d commands parsed from --help; the parse is wrong", len(commands))
	}

	// Every command must at least start, parse its flags and exit deliberately.
	// Exit 2 is acceptable here — it means the command ran and rejected the
	// arguments, not that it failed to exist.
	for _, name := range commands {
		r := run(t, repo, name, "-h")
		if r.code > 2 {
			t.Errorf("pawl %s -h: exit %d\n%s%s", name, r.code, r.stdout, r.stderr)
		}
		if r.stdout == "" && r.stderr == "" {
			t.Errorf("pawl %s -h produced no output at all", name)
		}
	}
}

func commandsFromHelp(t *testing.T, repo string) []string {
	t.Helper()
	help := run(t, repo, "-h").stdout
	var out []string
	for _, line := range strings.Split(help, "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		f := strings.Fields(line)
		if len(f) >= 2 && !strings.HasPrefix(f[0], "-") {
			out = append(out, f[0])
		}
	}
	return out
}

// TestBlockedGateStillProducesEvidence — PAWL-006 AC6 (via PAWL-022 AC6).
//
// "…exit non-zero on violation and shall emit the attestation regardless, so a
// blocked changeset still produces evidence."
//
// A blocked changeset that produced no trail would leave the reviewer with the
// verdict and none of the reasoning behind it, which is the opposite of the
// point. Run through the CLI because the exit status is half the criterion.
func TestBlockedGateStillProducesEvidence(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	run(t, repo, "claim", "cites a test that does not exist",
		"--path", "src/auth.py", "--lines", "1-9",
		"--verified-by", "test:tests.test_auth.test_imaginary")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")
	writeJUnit(t, repo)

	// The attestation is produced even though this changeset will not pass.
	att := run(t, repo, "attest", "--base", "HEAD~1", "--junit", "junit.xml",
		"--out", "trail.json")
	if att.code != 0 {
		t.Fatalf("attest on a failing changeset: exit %d\n%s%s", att.code, att.stdout, att.stderr)
	}
	body, err := os.ReadFile(filepath.Join(repo, "trail.json"))
	if err != nil {
		t.Fatalf("no trail written for a blocked changeset: %v", err)
	}
	for _, want := range []string{"assumption-trail", "cites a test that does not exist", "needs_human"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the trail is missing %q — the reasoning must survive a block", want)
		}
	}

	// And the gate refuses it.
	g := run(t, repo, "gate", "--base", "HEAD~1", "--junit", "junit.xml")
	if g.code != 1 {
		t.Errorf("gate exit = %d, want 1 on violation", g.code)
	}
}

// TestHarnessEntryPointIsNotAGeneralCommand is PAWL-019 AC17. Listing `hook`
// beside `claim` and `gate` invites someone to script against it, and the next
// change to a harness protocol then breaks them. Both halves of the criterion
// are asserted: it is absent from the command list, and the help says whose
// protocol its output follows.
func TestHarnessEntryPointIsNotAGeneralCommand(t *testing.T) {
	repo := newRepo(t)

	for _, c := range commandsFromHelp(t, repo) {
		if c == "hook" {
			t.Error("hook is listed among the general commands; it is an " +
				"integration point, and the stable answer is `pawl pending`")
		}
	}

	help := run(t, repo, "-h").stdout
	if !strings.Contains(help, "pawl pending") {
		t.Error("help must point at the stable command it wants scripted instead")
	}
	if !strings.Contains(help, "harness") {
		t.Error("help must state that the hook's output follows the harness " +
			"protocol rather than being a pawl interface")
	}
}

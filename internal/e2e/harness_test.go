package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"trunion.io/pawl/internal/claimlog"
	"trunion.io/pawl/internal/harness"
	"trunion.io/pawl/internal/model"
)

func writeSettings(t *testing.T, home, body string) string {
	t.Helper()
	p := harness.SettingsPath(home)
	mustMkdir(t, filepath.Dir(p))
	mustWrite(t, p, body)
	return p
}

func readSettings(t *testing.T, home string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(harness.SettingsPath(home))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestInstallPreservesEverythingItDidNotAdd is PAWL-019 AC2, and the criterion
// that matters most: these are the user's settings, holding things pawl knows
// nothing about.
func TestInstallPreservesEverythingItDidNotAdd(t *testing.T) {
	home := t.TempDir()
	writeSettings(t, home, `{
	  "model": "opus",
	  "permissions": {"allow": ["Bash(git *)"]},
	  "hooks": {
	    "SessionStart": [{"hooks": [{"type": "command", "command": "echo hi"}]}],
	    "PostToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "somebody-elses-tool"}]}]
	  }
	}`)

	p, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}

	got := readSettings(t, home)
	if got["model"] != "opus" {
		t.Error("unrelated top-level key lost")
	}
	if got["permissions"] == nil {
		t.Error("permissions lost")
	}
	hooks := got["hooks"].(map[string]any)
	if hooks["SessionStart"] == nil {
		t.Error("another event's hooks lost")
	}
	post := hooks["PostToolUse"].([]any)
	if len(post) != 2 {
		t.Fatalf("PostToolUse groups = %d, want the existing one plus ours", len(post))
	}
	if !strings.Contains(string(p.Result), "somebody-elses-tool") {
		t.Error("another tool's hook was dropped")
	}
}

// TestInstallIsIdempotent is AC3 — a hooks array that grows an entry per
// invocation is the obvious way to get this wrong.
func TestInstallIsIdempotent(t *testing.T) {
	home := t.TempDir()
	first, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Apply(); err != nil {
		t.Fatal(err)
	}

	second, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	if !second.AlreadySet {
		t.Error("a second install must be a no-op")
	}
	if err := second.Apply(); err != nil {
		t.Fatal(err)
	}

	hooks := readSettings(t, home)["hooks"].(map[string]any)
	if n := len(hooks["PostToolUse"].([]any)); n != 1 {
		t.Errorf("PostToolUse groups = %d after two installs, want 1", n)
	}
}

// TestUninstallRemovesOnlyOurs is AC6.
func TestUninstallRemovesOnlyOurs(t *testing.T) {
	home := t.TempDir()
	writeSettings(t, home, `{"hooks": {"PostToolUse": [
	  {"matcher": "Bash", "hooks": [{"type": "command", "command": "somebody-elses-tool"}]}
	]}}`)

	p, _ := harness.Install(home)
	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	u, err := harness.Uninstall(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Apply(); err != nil {
		t.Fatal(err)
	}

	body := string(u.Result)
	if strings.Contains(body, harness.HookCommand()) {
		t.Error("uninstall left pawl's hook behind")
	}
	if !strings.Contains(body, "somebody-elses-tool") {
		t.Error("uninstall removed somebody else's hook")
	}
}

// TestInstallBacksUpFirst is AC4.
func TestInstallBacksUpFirst(t *testing.T) {
	home := t.TempDir()
	writeSettings(t, home, `{"model": "opus"}`)
	p, _ := harness.Install(home)
	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	if p.Backup == "" {
		t.Fatal("no backup recorded")
	}
	b, err := os.ReadFile(p.Backup)
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if !strings.Contains(string(b), "opus") {
		t.Error("backup does not hold the original content")
	}
}

// TestInstallRefusesUnparseableSettings — rewriting a file we could not parse
// would discard whatever it holds.
func TestInstallRefusesUnparseableSettings(t *testing.T) {
	home := t.TempDir()
	writeSettings(t, home, `{ this is not json`)
	if _, err := harness.Install(home); err == nil {
		t.Error("must refuse to touch settings it cannot parse")
	}
}

// TestHookIsSilentOutsideAPawlRepo is AC7. User settings apply to every
// project, so a repository not using pawl must hear nothing and cost nothing.
func TestHookIsSilentOutsideAPawlRepo(t *testing.T) {
	repo := newRepo(t) // a git repo with no .pawl directory
	var out strings.Builder
	payload := `{"tool_name":"Edit","tool_input":{"file_path":"` +
		filepath.Join(repo, "src", "auth.py") + `"}}`

	if err := harness.ClaudeCodeHook(harness.Input{Stdin: strings.NewReader(payload)}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Errorf("hook spoke in a repo with no .pawl: %q", out.String())
	}
}

// TestHookFindsTheRepoFromTheEditedFile is AC8 — deriving it from the hook's
// own location is why the shell script only worked when copied in.
func TestHookFindsTheRepoFromTheEditedFile(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	// Give it a .pawl directory so the hook engages.
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "unrelated",
		Path: "src/auth.py", StartLine: 1, EndLine: 2,
	})

	var out strings.Builder
	payload := `{"tool_name":"Edit","tool_input":{"file_path":"` +
		filepath.Join(repo, "src", "auth.py") + `"}}`
	if err := harness.ClaudeCodeHook(harness.Input{Stdin: strings.NewReader(payload)}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unaccounted") {
		t.Errorf("expected the hook to report pending spans, got %q", out.String())
	}
	if !strings.Contains(out.String(), "PostToolUse") {
		t.Error("reply is missing the harness event name")
	}
}

// TestHookSurvivesGarbage is AC10. A user-level hook fires on every edit in
// every project; one that can break an edit loop would be uninstalled the first
// time it did.
func TestHookSurvivesGarbage(t *testing.T) {
	for _, payload := range []string{
		``, `not json at all`, `{}`, `{"tool_input":{}}`,
		`{"tool_input":{"file_path":"/nonexistent/nowhere.go"}}`,
		`{"tool_input":{"file_path":""}}`,
	} {
		var out strings.Builder
		if err := harness.ClaudeCodeHook(harness.Input{Stdin: strings.NewReader(payload)}, &out); err != nil {
			t.Errorf("payload %q returned an error: %v", payload, err)
		}
		if out.String() != "" {
			t.Errorf("payload %q produced output: %q", payload, out.String())
		}
	}
}

// TestHookRefusesATerminal is PAWL-019 AC10a. Found by running it at a prompt,
// where it hung with no output and no indication why.
//
// Silence is correct for a harness and wrong for a human. The two are
// distinguishable, so the tool should distinguish them rather than making a
// person guess.
func TestHookRefusesATerminal(t *testing.T) {
	// The guard lives in the CLI because it is about how the process was
	// invoked, not about the payload. What is asserted here is the half that is
	// testable without a pty: a non-terminal stdin must still be read normally.
	var out strings.Builder
	if err := harness.ClaudeCodeHook(harness.Input{Stdin: strings.NewReader(`{}`)}, &out); err != nil {
		t.Fatalf("a piped payload must still be handled: %v", err)
	}
	if out.String() != "" {
		t.Errorf("an empty payload should produce nothing, got %q", out.String())
	}
}

// TestHookDefaultsToTheWorkingTree is PAWL-019 AC11 and AC12. Running the
// command bare must do something useful — the first version blocked forever,
// the second refused outright, and both treated "no input" as an error when it
// is the case with the most obvious default.
func TestHookDefaultsToTheWorkingTree(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "gives the repo a .pawl directory",
		Path: "src/auth.py", StartLine: 1, EndLine: 2,
	})

	var out strings.Builder
	err := harness.ClaudeCodeHook(harness.Input{Interactive: true, Repo: repo}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unaccounted") {
		t.Errorf("bare invocation should report the working tree, got %q", out.String())
	}
}

// TestHookPrefersAnExplicitPath is AC11's ordering: an argument wins over
// whatever is on stdin.
func TestHookPrefersAnExplicitPath(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	mustWrite(t, filepath.Join(repo, "src", "other.py"), "x = 1\ny = 2\n")
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "gives the repo a .pawl directory",
		Path: "src/auth.py", StartLine: 1, EndLine: 2,
	})

	// stdin names auth.py; the argument names other.py. The argument wins.
	payload := `{"tool_input":{"file_path":"` + filepath.Join(repo, "src", "auth.py") + `"}}`
	var out strings.Builder
	err := harness.ClaudeCodeHook(harness.Input{
		Path:  filepath.Join(repo, "src", "other.py"),
		Stdin: strings.NewReader(payload),
		Repo:  repo,
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "other.py") {
		t.Errorf("explicit path should win over the payload, got %q", out.String())
	}
	if strings.Contains(out.String(), "auth.py") {
		t.Error("the payload should have been ignored when a path was given")
	}
}

// TestNonInteractiveWithNoPayloadStaysSilent is AC13. A pipe with nothing
// usable on it is a harness call that went wrong, not an invitation to scan the
// tree on every edit.
func TestNonInteractiveWithNoPayloadStaysSilent(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "gives the repo a .pawl directory",
		Path: "src/auth.py", StartLine: 1, EndLine: 2,
	})

	var out strings.Builder
	err := harness.ClaudeCodeHook(harness.Input{
		Stdin: strings.NewReader(``), Interactive: false, Repo: repo,
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Errorf("a harness call with no payload must stay silent, got %q", out.String())
	}
}

// TestInstalledConfigIsTheEmbeddedOne is AC14 and AC15 — one definition,
// shipped with the binary that implements it.
func TestInstalledConfigIsTheEmbeddedOne(t *testing.T) {
	if harness.HookCommand() == "" {
		t.Fatal("the marker must be readable from the embedded config, not declared twice")
	}

	home := t.TempDir()
	p, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	// The command installed is the embedded one with its bare `pawl` resolved to
	// an absolute path (AC18), so assert the arguments rather than the whole
	// string.
	if !strings.Contains(string(p.Result), "hook claude-code") {
		t.Error("the installed settings do not carry the embedded command's arguments")
	}
	// What is installed must be exactly what ships, not an assembled copy.
	if !strings.Contains(string(p.Result), "Edit|Write|MultiEdit") {
		t.Error("the embedded matcher did not reach the installed settings")
	}
}

// TestInstallHonoursAnExplicitDirectory is PAWL-019 AC16. The home default is
// what makes the hook fire everywhere; --dir is for a team that wants the
// configuration committed beside their repository instead.
func TestInstallHonoursAnExplicitDirectory(t *testing.T) {
	dir := t.TempDir()
	p, err := harness.Install(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}

	want := harness.SettingsPath(dir)
	if p.Path != want {
		t.Errorf("installed to %s, want %s", p.Path, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("settings not written to the given directory: %v", err)
	}
	// And nothing was written to the real home directory.
	if home, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(p.Path, home) && !strings.HasPrefix(dir, home) {
			t.Error("an explicit --dir must not fall back to home")
		}
	}
}

// TestTurnEndCatchesAShellEdit is PAWL-020 AC1 and AC2, and the gap the spec
// exists for: nothing here goes through an edit tool.
func TestTurnEndCatchesAShellEdit(t *testing.T) {
	repo := newRepo(t)
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "gives the repo a .pawl directory",
		Path: "src/auth.py", StartLine: 1, EndLine: 2,
	})
	// A change made the way a shell would make it — no file_path, no payload,
	// no edit-tool event anywhere.
	writeFeature(t, repo)

	var out strings.Builder
	if err := harness.ClaudeCodeTurnEnd(repo, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unaccounted") {
		t.Fatalf("a shell edit must still be caught at the turn boundary, got %q", out.String())
	}
	if !strings.Contains(out.String(), "block") {
		t.Error("the reply should feed the reason back to the agent")
	}
}

// TestTurnEndRaisesTheSameSetOnlyOnce is AC5 — the criterion that keeps this
// from trapping an agent in a loop it cannot escape.
func TestTurnEndRaisesTheSameSetOnlyOnce(t *testing.T) {
	repo := newRepo(t)
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "gives the repo a .pawl directory",
		Path: "src/auth.py", StartLine: 1, EndLine: 2,
	})
	writeFeature(t, repo)

	var first, second strings.Builder
	if err := harness.ClaudeCodeTurnEnd(repo, &first); err != nil {
		t.Fatal(err)
	}
	if first.String() == "" {
		t.Fatal("the first turn boundary should raise the outstanding set")
	}
	if err := harness.ClaudeCodeTurnEnd(repo, &second); err != nil {
		t.Fatal(err)
	}
	if second.String() != "" {
		t.Errorf("the same set must not be raised twice; a turn that cannot end "+
			"is a loop. got %q", second.String())
	}
}

// TestTurnEndIsSilentWhenEverythingIsAccounted is AC4. Most turns will be
// silent, and a hook that speaks regardless is one an agent learns to ignore.
func TestTurnEndIsSilentWhenEverythingIsAccounted(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	// Account for the whole change.
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 1, EndLine: 9})

	var out strings.Builder
	if err := harness.ClaudeCodeTurnEnd(repo, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Errorf("nothing outstanding should mean nothing said, got %q", out.String())
	}
}

// TestTurnEndIsSilentOutsideAPawlRepo — user settings apply to every project.
func TestTurnEndIsSilentOutsideAPawlRepo(t *testing.T) {
	repo := newRepo(t) // no .pawl directory
	writeFeature(t, repo)
	var out strings.Builder
	if err := harness.ClaudeCodeTurnEnd(repo, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Errorf("spoke in a repo not using pawl: %q", out.String())
	}
}

// TestSetupInstallsBothBindings is AC3 and AC6: the turn boundary is primary,
// the per-edit binding supplements it and is the fallback if the turn event
// proves unreliable.
func TestSetupInstallsBothBindings(t *testing.T) {
	home := t.TempDir()
	p, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	body := string(p.Result)
	for _, want := range []string{"Stop", "PostToolUse", "--event stop", "Edit|Write|MultiEdit"} {
		if !strings.Contains(body, want) {
			t.Errorf("installed settings missing %q", want)
		}
	}
}

// TestUninstallRemovesBothBindings — an uninstall that leaves one behind is
// worse than one that fails loudly.
func TestUninstallRemovesBothBindings(t *testing.T) {
	home := t.TempDir()
	p, _ := harness.Install(home)
	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	u, err := harness.Uninstall(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Apply(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(u.Result), "pawl hook") {
		t.Errorf("uninstall left a binding behind: %s", u.Result)
	}
}

// TestInstallWritesAnAbsolutePath is PAWL-019 AC18. A bare `pawl` resolves only
// if pawl is on the PATH the harness hands its hooks, which it was not.
func TestInstallWritesAnAbsolutePath(t *testing.T) {
	home := t.TempDir()
	p, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(p.Result), `"command": "pawl hook`) {
		t.Error("a bare command name was installed; it resolves only on the " +
			"harness's PATH and fails silently when it does not")
	}
	// Assert absoluteness, not the binary's name: under test the running
	// executable is the test binary, and in the wild pawl may be installed
	// under any name.
	found := false
	for _, line := range strings.Split(string(p.Result), "\n") {
		if !strings.Contains(line, "hook claude-code") {
			continue
		}
		cmd := strings.TrimSpace(strings.SplitN(line, `": "`, 2)[1])
		if strings.HasPrefix(cmd, "/") {
			found = true
		}
	}
	if !found {
		t.Errorf("no absolute path in the installed settings:\n%s", p.Result)
	}
}

// TestInstallRepairsAnOutdatedEntry is AC21. Without it, idempotency is a trap:
// a broken entry is recognised as ours, skipped as already installed, and never
// fixed.
func TestInstallRepairsAnOutdatedEntry(t *testing.T) {
	home := t.TempDir()
	// An installation from an older pawl, naming a bare command.
	writeSettings(t, home, `{"hooks":{"PostToolUse":[{"matcher":"Edit|Write|MultiEdit",
	  "hooks":[{"type":"command","command":"pawl hook claude-code"}]}]}}`)

	p, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	if p.AlreadySet {
		t.Fatal("an entry naming a different command must be repaired, not skipped")
	}
	if strings.Contains(string(p.Result), `"command": "pawl hook claude-code"`) {
		t.Error("the stale bare-path entry survived the repair")
	}

	// AC3 still holds: installing again from the same binary changes nothing.
	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	again, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	if !again.AlreadySet {
		t.Error("a second install from the same binary must still be a no-op")
	}
}

// TestCheckReportsABrokenInstallation is AC19 and AC20 — the diagnostic that
// tells a broken installation apart from a working one with nothing to say.
func TestCheckReportsABrokenInstallation(t *testing.T) {
	home := t.TempDir()

	if r := harness.Check(home); r.Installed {
		t.Error("nothing is installed yet")
	}

	writeSettings(t, home, `{"hooks":{"PostToolUse":[{"matcher":"Edit",
	  "hooks":[{"type":"command","command":"/nonexistent/pawl hook claude-code"}]}]}}`)
	r := harness.Check(home)
	if !r.Installed {
		t.Fatal("the configuration is present and should be reported as installed")
	}
	if len(r.Working) != 1 || r.Working[0] {
		t.Error("a command that cannot be run must be reported as not working; " +
			"otherwise a broken install is indistinguishable from a quiet one")
	}
}

// TestInstallReportsTheChangeWithoutApplyingIt is PAWL-019 AC5. Install returns
// a plan; nothing reaches disk until Apply. Nobody should have to trust a tool's
// description of what it is about to do to their configuration.
func TestInstallReportsTheChangeWithoutApplyingIt(t *testing.T) {
	home := t.TempDir()
	settings := harness.SettingsPath(home)

	p, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Result) == 0 {
		t.Error("the plan must report the settings it would write")
	}
	if _, err := os.Stat(settings); !os.IsNotExist(err) {
		t.Fatalf("planning wrote %s; the report must not apply itself", settings)
	}

	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settings); err != nil {
		t.Errorf("Apply did not write the settings it reported: %v", err)
	}
}

// TestInstalledHookNeedsNoInterpreter is PAWL-019 AC9. pawl brings nothing with
// it; a hook that needs a shell or a JSON processor installed quietly gives that
// argument up.
func TestInstalledHookNeedsNoInterpreter(t *testing.T) {
	home := t.TempDir()
	p, err := harness.Install(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(p.Result), "\n") {
		if !strings.Contains(line, `"command"`) {
			continue
		}
		for _, forbidden := range []string{"jq", "sh -c", "bash", "|", "&&", ";", "$("} {
			if strings.Contains(line, forbidden) {
				t.Errorf("installed command needs %q, so the hook depends on "+
					"something pawl does not ship: %s", forbidden, strings.TrimSpace(line))
			}
		}
	}
}

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
	if strings.Contains(body, harness.HookCommand) {
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

	if err := harness.ClaudeCodeHook(strings.NewReader(payload), &out); err != nil {
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
	if err := harness.ClaudeCodeHook(strings.NewReader(payload), &out); err != nil {
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
		if err := harness.ClaudeCodeHook(strings.NewReader(payload), &out); err != nil {
			t.Errorf("payload %q returned an error: %v", payload, err)
		}
		if out.String() != "" {
			t.Errorf("payload %q produced output: %q", payload, out.String())
		}
	}
}

package harness

// Installing the hook into a user's harness settings (PAWL-019 AC1–AC6).
//
// These are the user's settings, not pawl's. They already hold hooks,
// permissions and preferences pawl knows nothing about, so every operation here
// is written to be conservative: merge rather than replace, refuse to duplicate,
// back up before touching anything, and offer to show the change without making
// it. A tool that clobbers an editor configuration gets uninstalled immediately
// and deserves to.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// claudeCodeConfig is the configuration pawl installs, compiled into the binary
// (AC14).
//
// Assembling this JSON at the point of use would spread the shape of a working
// configuration across code, documentation, and whatever a user pasted out of a
// README. One definition, shipped with the binary that implements it, cannot
// drift from itself.
//
//go:embed claude-code.json
var claudeCodeConfig []byte

// commandPrefix identifies pawl's entries, and is read out of the embedded
// configuration rather than declared again (AC15).
//
// A prefix rather than an exact string, because pawl now installs more than one
// entry — a per-edit binding and a turn-boundary one — and both must be found
// by uninstall. A separately declared constant would be a second source of truth
// for "which entry is ours", and the failure when the two disagree is an
// uninstall that silently leaves a hook in place.
// commandArgs identifies pawl's entries by the arguments rather than the
// binary path, because AC18 installs an absolute path and a prefix match on
// "pawl " would then miss our own entry at uninstall time.
const commandArgs = "hook claude-code"

// HookCommand reports the first command pawl installs, for callers that want to
// name it. Uninstall matches on the prefix instead.
func HookCommand() string {
	cfg, err := config()
	if err != nil {
		return ""
	}
	for _, groups := range cfg {
		for _, g := range groups {
			for _, h := range g["hooks"].([]any) {
				if entry, ok := h.(map[string]any); ok {
					if cmd, ok := entry["command"].(string); ok {
						return cmd
					}
				}
			}
		}
	}
	return ""
}

func isOurs(entry map[string]any) bool {
	cmd, _ := entry["command"].(string)
	// Matched on the argument signature alone, not on the binary's name. A name
	// check breaks the moment the binary is installed as anything else —
	// `pawl-0.1.0`, a symlink, a test harness — and the failure mode is an
	// uninstall that cannot find its own entry.
	return strings.Contains(cmd, commandArgs)
}

// resolveCommand rewrites the embedded config's bare `pawl` into the absolute
// path of the running binary (AC18).
//
// A bare name resolves only if pawl is on the PATH the harness hands its hooks,
// which is not a login shell's PATH and not one a direnv-scoped install
// provides. pawl knows where it is now; assuming the harness will find it later
// is what failed silently.
func resolveCommand(cmd string) string {
	exe, err := os.Executable()
	if err != nil {
		return cmd
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if rest, ok := strings.CutPrefix(cmd, "pawl "); ok {
		return exe + " " + rest
	}
	return cmd
}

// config decodes the embedded configuration as event name -> groups. A fresh
// copy each call, so a caller mutating what it merges cannot corrupt the
// definition for the next one.
func config() (map[string][]map[string]any, error) {
	var raw map[string][]map[string]any
	if err := json.Unmarshal(claudeCodeConfig, &raw); err != nil {
		return nil, fmt.Errorf("embedded harness config is not valid JSON: %w", err)
	}
	return raw, nil
}

// SettingsPath is where Claude Code keeps user-level settings. User level is
// the point: project-level configuration only loads when that project is the
// root, which is exactly the failure this fixes.
func SettingsPath(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}

// Plan is what an install or uninstall would do, so it can be shown before it
// is done (AC5).
type Plan struct {
	Path       string
	Existed    bool
	AlreadySet bool
	Result     []byte
	Backup     string
}

// Install merges pawl's hook into the settings, returning the plan without
// writing. Call Apply to write it.
func Install(home string) (Plan, error) {
	return plan(home, true)
}

// Uninstall removes pawl's hook, and only pawl's hook (AC6).
func Uninstall(home string) (Plan, error) {
	return plan(home, false)
}

func plan(home string, add bool) (Plan, error) {
	p := Plan{Path: SettingsPath(home)}

	settings := map[string]any{}
	b, err := os.ReadFile(p.Path)
	switch {
	case err == nil:
		p.Existed = true
		if err := json.Unmarshal(b, &settings); err != nil {
			// Refusing here is the safe answer. Rewriting a file we could not
			// parse would discard whatever it holds, and a settings file that
			// fails to parse is already a problem the user needs to see.
			return p, fmt.Errorf("%s is not valid JSON; refusing to touch it: %w", p.Path, err)
		}
	case os.IsNotExist(err):
	default:
		return p, err
	}

	changed := mutate(settings, add)
	p.AlreadySet = !changed

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return p, err
	}
	p.Result = append(out, '\n')
	return p, nil
}

// mutate adds or removes pawl's hook entry, reporting whether anything changed.
//
// It navigates the settings generically rather than through a typed struct:
// decoding into a struct would silently drop every key pawl does not know
// about, which is precisely what AC2 forbids.
func mutate(settings map[string]any, add bool) bool {
	cfg, err := config()
	if err != nil {
		return false
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		if !add {
			return false
		}
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}

	// Deterministic order so a dry run shows the same thing twice.
	eventNames := make([]string, 0, len(cfg))
	for name := range cfg {
		eventNames = append(eventNames, name)
	}
	sort.Strings(eventNames)

	changed := false
	for _, event := range eventNames {
		if mutateEvent(hooks, event, cfg[event], add) {
			changed = true
		}
	}

	// Leave nothing behind that we created and then emptied.
	if !add && len(hooks) == 0 {
		delete(settings, "hooks")
	}
	return changed
}

// mutateEvent adds or removes pawl's groups for one event, leaving every other
// tool's entries exactly as they were.
func mutateEvent(hooks map[string]any, event string, ours []map[string]any, add bool) bool {
	existing, _ := hooks[event].([]any)

	if !add {
		kept := make([]any, 0, len(existing))
		removed := false
		for _, g := range existing {
			group, _ := g.(map[string]any)
			if group == nil {
				kept = append(kept, g)
				continue
			}
			inner, _ := group["hooks"].([]any)
			survivors := make([]any, 0, len(inner))
			for _, h := range inner {
				entry, _ := h.(map[string]any)
				if entry != nil && isOurs(entry) {
					removed = true
					continue
				}
				survivors = append(survivors, h)
			}
			// A group that held only our hook goes; one that held somebody
			// else's keeps it.
			if len(survivors) == 0 && len(inner) > 0 {
				continue
			}
			group["hooks"] = survivors
			kept = append(kept, group)
		}
		if !removed {
			return false
		}
		if len(kept) > 0 {
			hooks[event] = kept
		} else {
			delete(hooks, event)
		}
		return true
	}

	// Already installed? AC3 says a second run is a no-op — but only when the
	// entry still names what we would install now. AC21: an entry naming a
	// different command is repaired rather than skipped, or idempotency becomes
	// a trap that recognises a broken entry as ours and never fixes it.
	want := ""
	if len(ours) > 0 {
		if inner, ok := ours[0]["hooks"].([]any); ok && len(inner) > 0 {
			if e, ok := inner[0].(map[string]any); ok {
				want, _ = e["command"].(string)
				want = resolveCommand(want)
			}
		}
	}
	for _, g := range existing {
		group, _ := g.(map[string]any)
		if group == nil {
			continue
		}
		for _, h := range group["hooks"].([]any) {
			entry, ok := h.(map[string]any)
			if !ok || !isOurs(entry) {
				continue
			}
			if cmd, _ := entry["command"].(string); cmd == want {
				return false
			}
			entry["command"] = want
			return true
		}
	}

	for _, g := range ours {
		if inner, ok := g["hooks"].([]any); ok {
			for _, h := range inner {
				if entry, ok := h.(map[string]any); ok {
					if cmd, ok := entry["command"].(string); ok {
						entry["command"] = resolveCommand(cmd)
					}
				}
			}
		}
		existing = append(existing, g)
	}
	hooks[event] = existing
	return true
}

// Apply writes the plan, backing up any existing file first (AC4).
func (p *Plan) Apply() error {
	if p.AlreadySet {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
		return err
	}
	if p.Existed {
		p.Backup = fmt.Sprintf("%s.bak-%s", p.Path, time.Now().UTC().Format("20060102150405"))
		b, err := os.ReadFile(p.Path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(p.Backup, b, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(p.Path, p.Result, 0o644)
}

// CheckResult is what a --check reports (AC20).
type CheckResult struct {
	Path      string
	Installed bool
	Commands  []string
	Working   []bool
	Problem   string
}

// Check reports whether an installation is present and whether the commands it
// names actually run (AC19, AC20).
//
// This exists because AC10 requires the hook to stay silent on failure so it can
// never break an edit loop — and the cost of that is a broken installation
// looking exactly like a working one with nothing to say. Something has to tell
// them apart, and it must not be the hook.
func Check(home string) CheckResult {
	r := CheckResult{Path: SettingsPath(home)}

	b, err := os.ReadFile(r.Path)
	if err != nil {
		r.Problem = "no settings file: " + err.Error()
		return r
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		r.Problem = "settings file is not valid JSON"
		return r
	}

	hooks, _ := settings["hooks"].(map[string]any)
	for _, groups := range hooks {
		list, _ := groups.([]any)
		for _, g := range list {
			group, _ := g.(map[string]any)
			if group == nil {
				continue
			}
			inner, _ := group["hooks"].([]any)
			for _, h := range inner {
				entry, _ := h.(map[string]any)
				if entry == nil || !isOurs(entry) {
					continue
				}
				cmd, _ := entry["command"].(string)
				r.Installed = true
				r.Commands = append(r.Commands, cmd)
				r.Working = append(r.Working, commandRuns(cmd))
			}
		}
	}
	if !r.Installed {
		r.Problem = "pawl's hook is not in the settings"
	}
	return r
}

// commandRuns executes the installed command the way a harness would, and
// reports whether it is even findable. An empty payload is a no-op for the hook
// itself, so this tests reachability rather than behaviour.
func commandRuns(cmd string) bool {
	bin, rest, _ := strings.Cut(cmd, " ")
	args := strings.Fields(rest)
	c := exec.Command(bin, args...)
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	return c.Run() == nil
}

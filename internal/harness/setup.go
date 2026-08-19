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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HookCommand identifies pawl's entry in a settings file. It is the marker used
// for both idempotency and uninstall — anything else in there belongs to
// somebody else and is never touched.
const HookCommand = "pawl hook claude-code"

const hookMatcher = "Edit|Write|MultiEdit"

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
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		if !add {
			return false
		}
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}

	events, _ := hooks["PostToolUse"].([]any)

	// Find our entry, if it is already there.
	for gi, g := range events {
		group, _ := g.(map[string]any)
		if group == nil {
			continue
		}
		inner, _ := group["hooks"].([]any)
		for hi, h := range inner {
			entry, _ := h.(map[string]any)
			if entry == nil || entry["command"] != HookCommand {
				continue
			}
			if add {
				return false // AC3: already installed, second run is a no-op
			}
			// AC6: remove ours, and only ours. Groups and arrays that still
			// hold somebody else's hooks are left exactly as they were.
			inner = append(inner[:hi], inner[hi+1:]...)
			if len(inner) > 0 {
				group["hooks"] = inner
				return true
			}
			events = append(events[:gi], events[gi+1:]...)
			if len(events) > 0 {
				hooks["PostToolUse"] = events
			} else {
				delete(hooks, "PostToolUse")
				if len(hooks) == 0 {
					delete(settings, "hooks")
				}
			}
			return true
		}
	}

	if !add {
		return false
	}
	hooks["PostToolUse"] = append(events, map[string]any{
		"matcher": hookMatcher,
		"hooks": []any{map[string]any{
			"type":          "command",
			"command":       HookCommand,
			"timeout":       10,
			"statusMessage": "pawl: checking accounting",
		}},
	})
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

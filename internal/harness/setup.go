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
	"os"
	"path/filepath"
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

// HookCommand identifies pawl's entry in a settings file, and is read out of
// the embedded configuration rather than declared again (AC15).
//
// A separately declared constant would be a second source of truth for "which
// entry is ours", and the failure when the two disagree is an uninstall that
// silently leaves the hook in place.
func HookCommand() string {
	group, err := configGroup()
	if err != nil {
		return ""
	}
	inner, _ := group["hooks"].([]any)
	for _, h := range inner {
		if entry, ok := h.(map[string]any); ok {
			if cmd, ok := entry["command"].(string); ok {
				return cmd
			}
		}
	}
	return ""
}

// configGroup decodes the embedded configuration. A fresh copy each call, so a
// caller mutating what it merges cannot corrupt the definition for the next one.
func configGroup() (map[string]any, error) {
	var g map[string]any
	if err := json.Unmarshal(claudeCodeConfig, &g); err != nil {
		return nil, fmt.Errorf("embedded harness config is not valid JSON: %w", err)
	}
	return g, nil
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
	group, err := configGroup()
	if err != nil {
		return false
	}
	marker := HookCommand()
	if marker == "" {
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
			if entry == nil || entry["command"] != marker {
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
	hooks["PostToolUse"] = append(events, group)
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

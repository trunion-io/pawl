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
const commandPrefix = "pawl hook "

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
	return strings.HasPrefix(cmd, commandPrefix)
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

	// Already installed? AC3: a second run is a no-op.
	for _, g := range existing {
		group, _ := g.(map[string]any)
		if group == nil {
			continue
		}
		for _, h := range group["hooks"].([]any) {
			if entry, ok := h.(map[string]any); ok && isOurs(entry) {
				return false
			}
		}
	}

	for _, g := range ours {
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

// Package harness adapts pawl to a coding harness's hook protocol (PAWL-019).
//
// This is an adapter, not a harness. The build/buy position is explicit that
// pawl does not build one; teaching it to read a payload and write a reply is a
// translation at an integration boundary. If anything here grows logic beyond
// that translation, it has drifted and should be moved into pawl proper.
package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"trunion.io/pawl/internal/claimlog"
	"trunion.io/pawl/internal/resolve"
)

// hookPayload is the subset of Claude Code's PostToolUse input pawl reads.
// Unknown fields are ignored, deliberately: the harness may add any it likes
// and an adapter that breaks on one it has not seen is a liability.
type hookPayload struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
	ToolResponse struct {
		FilePath string `json:"filePath"`
	} `json:"tool_response"`
}

type hookReply struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// maxRanges caps what a single firing enumerates. This runs on every edit and
// the context is paid out of the agent's own budget (PAWL-017 AC3); the count
// carries the rest.
const maxRanges = 6

// ClaudeCodeHook reads a PostToolUse payload and writes any reply to w.
//
// Every failure path returns nil having written nothing (AC10). A user-level
// hook fires on every edit in every project, so one that can break an edit loop
// when it malfunctions would be uninstalled the first time it did, and deserve
// to be. There is no error worth reporting here that is worth that risk.
func ClaudeCodeHook(r io.Reader, w io.Writer) error {
	var p hookPayload
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}

	file := p.ToolInput.FilePath
	if file == "" {
		file = p.ToolResponse.FilePath
	}
	if file == "" {
		return nil
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil
	}

	// AC8: the repository comes from the edited file, not from where the hook
	// lives. Deriving it from the hook's own location is why the shell script
	// only worked when copied into the repository it served.
	repo, ok := repoOf(abs)
	if !ok {
		return nil
	}

	// AC7: user settings apply to every project. A repository that is not using
	// pawl must cost nothing and hear nothing.
	if _, err := os.Stat(filepath.Join(repo, ".pawl")); err != nil {
		return nil
	}

	rel, err := filepath.Rel(repo, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil
	}

	claims, err := claimlog.Load(repo)
	if err != nil {
		return nil
	}
	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		return nil
	}
	spans, err := resolve.Pending(repo, claims, acks, []string{rel})
	if err != nil || len(spans) == 0 {
		return nil
	}

	// PAWL-017 AC14: say nothing when there is nothing new to say.
	if resolve.AlreadySurfaced(repo, rel, spans) {
		return nil
	}
	resolve.MarkSurfaced(repo, rel, spans)

	var reply hookReply
	reply.HookSpecificOutput.HookEventName = "PostToolUse"
	reply.HookSpecificOutput.AdditionalContext = message(rel, spans)
	return json.NewEncoder(w).Encode(reply)
}

// message is deliberately terse. How to claim is standing context and lives in
// the agent's instructions, not in every firing (PAWL-017 AC12).
func message(rel string, spans []resolve.PendingSpan) string {
	lines := 0
	parts := make([]string, 0, maxRanges)
	for i, s := range spans {
		lines += s.Lines()
		if i < maxRanges {
			parts = append(parts, fmt.Sprintf("%s:%d-%d", s.Path, s.StartLine, s.EndLine))
		}
	}
	ranges := strings.Join(parts, ", ")
	if len(spans) > maxRanges {
		ranges += fmt.Sprintf(", and %d more (pawl pending)", len(spans)-maxRanges)
	}
	return fmt.Sprintf("pawl: %d unaccounted span(s), %d line(s) in %s — %s",
		len(spans), lines, rel, ranges)
}

// repoOf walks up from a file looking for a git repository root.
func repoOf(path string) (string, bool) {
	dir := filepath.Dir(path)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

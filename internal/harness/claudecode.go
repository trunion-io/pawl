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

// Input tells the hook what to report on (AC11).
//
// Resolution order is: an explicit path, then a payload on stdin, then the
// working tree. Every invocation does something useful — "no input" is not an
// error, it is the case with the most obvious default.
type Input struct {
	// Path, when set, wins over anything on stdin.
	Path string
	// Stdin carries a harness payload. Nil when there is none to read.
	Stdin io.Reader
	// Interactive is true when a person ran the command rather than a harness.
	// It decides what happens when there is no usable payload: a person gets
	// the working tree, a harness gets silence (AC12, AC13).
	Interactive bool
	// Repo is where to look when falling back to the working tree.
	Repo string
}

// ClaudeCodeHook reports unaccounted spans and writes any reply to w.
//
// Every failure path returns nil having written nothing (AC10). A user-level
// hook fires on every edit in every project, so one that can break an edit loop
// when it malfunctions would be uninstalled the first time it did, and deserve
// to be. There is no error worth reporting here that is worth that risk.
func ClaudeCodeHook(in Input, w io.Writer) error {
	file := in.Path

	// A payload only matters when no path was given explicitly.
	if file == "" && in.Stdin != nil {
		raw, err := io.ReadAll(in.Stdin)
		if err == nil {
			var p hookPayload
			if json.Unmarshal(raw, &p) == nil {
				file = p.ToolInput.FilePath
				if file == "" {
					file = p.ToolResponse.FilePath
				}
			}
		}
	}

	// AC13: a pipe with nothing usable on it is a harness call that went wrong,
	// and silence is the right answer. AC11/AC12: a person gets the working
	// tree instead.
	if file == "" {
		if !in.Interactive {
			return nil
		}
		return wholeTree(in.Repo, w)
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

// wholeTree reports everything unaccounted in a repository, which is what a
// person running the command bare almost always means (AC11).
func wholeTree(repo string, w io.Writer) error {
	if repo == "" {
		repo = "."
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return nil
	}
	root, ok := repoOf(filepath.Join(abs, "x"))
	if !ok {
		return nil
	}
	if _, err := os.Stat(filepath.Join(root, ".pawl")); err != nil {
		return nil
	}

	claims, err := claimlog.Load(root)
	if err != nil {
		return nil
	}
	acks, err := claimlog.LoadAcks(root)
	if err != nil {
		return nil
	}
	spans, err := resolve.Pending(root, claims, acks, nil)
	if err != nil || len(spans) == 0 {
		return nil
	}

	var reply hookReply
	reply.HookSpecificOutput.HookEventName = "PostToolUse"
	reply.HookSpecificOutput.AdditionalContext = message("the working tree", spans)
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

// stopReply is the Stop-event response shape. `block` feeds the reason back and
// lets the turn continue, which is how the agent is told there is accounting
// outstanding while it still holds the reasoning.
type stopReply struct {
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// ClaudeCodeTurnEnd reports unaccounted spans at a turn boundary (PAWL-020).
//
// This is the binding that does not depend on which tool made the change. An
// agent editing through the shell matches no edit-tool matcher, and every
// binding to a tool name is a list of ways to edit a file that is never
// finished. By the time a turn ends the tree has changed, whatever route the
// change took.
//
// AC5 is the safety argument and it rests on the surfacing cache: a turn-
// boundary hook that keeps refusing to let a turn end is one edit away from a
// loop an agent cannot escape. The same unaccounted set is raised once and then
// never again, so the worst case is one extra exchange, not a trap.
func ClaudeCodeTurnEnd(repo string, w io.Writer) error {
	if repo == "" {
		repo = "."
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return nil
	}
	root, ok := repoOf(filepath.Join(abs, "x"))
	if !ok {
		return nil
	}
	if _, err := os.Stat(filepath.Join(root, ".pawl")); err != nil {
		return nil
	}

	claims, err := claimlog.Load(root)
	if err != nil {
		return nil
	}
	acks, err := claimlog.LoadAcks(root)
	if err != nil {
		return nil
	}
	spans, err := resolve.Pending(root, claims, acks, nil)
	if err != nil {
		return nil
	}

	// AC4: most turns account for everything, and a hook that speaks every turn
	// regardless is one an agent learns to ignore.
	if len(spans) == 0 {
		return nil
	}

	// AC5: raised once per unchanged set. The cache is the loop guard.
	const turnKey = "\x00turn"
	if resolve.AlreadyRaised(root, turnKey, spans) {
		return nil
	}
	resolve.MarkRaised(root, turnKey, spans)

	var reply stopReply
	reply.Decision = "block"
	reply.Reason = message("the working tree", spans) +
		"\n\nRecord these before finishing — `pawl claim` for anything you assumed, " +
		"`pawl ack` where there was nothing to assume. Doing it now keeps the " +
		"reasoning attached to the change; doing it later is reconstruction."
	return json.NewEncoder(w).Encode(reply)
}

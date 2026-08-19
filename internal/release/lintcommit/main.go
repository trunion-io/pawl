// Command lintcommit validates one commit message (PAWL-027 AC1–AC4).
//
// Used by .githooks/commit-msg so an author finds out while they are still
// writing the message, rather than after a push. CI runs commitlint over the
// same commits; the two must agree, and TestGoTypesMatchCommitlintConfig holds
// them together rather than trusting that somebody remembers.
package main

import (
	"fmt"
	"os"
	"strings"

	"trunion.io/pawl/internal/release"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: lintcommit <message-file> [changed-path...]")
		os.Exit(2)
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "lintcommit:", err)
		os.Exit(2)
	}
	msg := stripComments(string(b))

	if strings.TrimSpace(msg) == "" {
		os.Exit(0) // an empty message aborts the commit anyway
	}

	c, ok := release.ParseCommit(msg)
	if !ok {
		fail(msg, "the first line is not a Conventional Commit",
			"Expected:  <type>[optional (scope)][!]: <subject>",
			"Example:   fix: reject thresholds the gate cannot apply")
	}
	if !release.KnownType(c.Type) {
		fail(msg, fmt.Sprintf("%q is not an allowed type", c.Type),
			"Allowed:   "+strings.Join(release.Types(), ", "))
	}

	// AC4 — only for commits touching a module that decides verdicts.
	if len(os.Args) > 2 && touchesDeciding(os.Args[2:]) && !declaresVerdict(msg) {
		fail(msg, "this commit touches a module that decides verdicts but does not declare its effect",
			"Add one of:",
			"  Verdict-Affecting: yes",
			"  Verdict-Affecting: no",
			"",
			"\"yes\" forces a MAJOR bump (PAWL-013 AC2) whatever the type, because a",
			"client's contract with pawl is which changesets pass. Answering \"no\" is",
			"expected and normal; what is refused is silence, which is not evidence",
			"that a change does not move verdicts (C-3).")
	}
}

func touchesDeciding(paths []string) bool {
	for _, p := range paths {
		for _, d := range release.DecidingModules {
			if strings.HasPrefix(p, d) {
				return true
			}
		}
	}
	return false
}

func declaresVerdict(msg string) bool {
	for _, l := range strings.Split(msg, "\n") {
		t := strings.ToLower(strings.TrimSpace(l))
		if strings.HasPrefix(t, "verdict-affecting:") {
			v := strings.TrimSpace(strings.TrimPrefix(t, "verdict-affecting:"))
			return v == "yes" || v == "no"
		}
	}
	return false
}

func stripComments(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func fail(msg string, reason string, help ...string) {
	fmt.Fprintf(os.Stderr, "\ncommit rejected: %s\n\n", reason)
	for _, h := range help {
		fmt.Fprintln(os.Stderr, "  "+h)
	}
	fmt.Fprintf(os.Stderr, "\nYour message was:\n\n%s\n\n", indent(strings.TrimSpace(msg)))
	fmt.Fprintln(os.Stderr, "The hook is a convenience, not a control — CI checks this too.")
	fmt.Fprintln(os.Stderr, "To bypass deliberately: git commit --no-verify")
	os.Exit(1)
}

func indent(s string) string {
	return "  " + strings.ReplaceAll(s, "\n", "\n  ")
}

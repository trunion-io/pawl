package release

// Release notes (PAWL-027 AC16).
//
// Verdict-affecting changes are grouped separately and printed first, because a
// client deciding whether to take an upgrade has exactly one question ahead of
// every other: does this change which of my changesets merge? PAWL-013 AC2 makes
// that the definition of a breaking change here, and a notes format that buried
// it among the fixes would be answering a different question than the one asked.

import (
	"fmt"
	"sort"
	"strings"
)

var typeHeadings = map[string]string{
	"feat":     "Features",
	"fix":      "Fixes",
	"perf":     "Performance",
	"refactor": "Internal",
	"docs":     "Documentation",
	"build":    "Build",
	"ci":       "CI",
	"test":     "Tests",
	"chore":    "Chores",
	"style":    "Style",
	"revert":   "Reverts",
}

// order controls the sections a reader sees first.
var order = []string{"feat", "fix", "perf", "docs", "refactor", "build", "ci", "test", "revert", "chore", "style"}

func Notes(commits []Commit, from, to string, bump Bump) string {
	var b strings.Builder

	var verdict []Commit
	byType := map[string][]Commit{}
	for _, c := range commits {
		if c.VerdictAffecting {
			verdict = append(verdict, c)
			continue
		}
		byType[c.Type] = append(byType[c.Type], c)
	}

	if len(verdict) > 0 {
		b.WriteString("## Verdict-affecting changes\n\n")
		b.WriteString("These change which changesets pass the gate, for unchanged input and an\n")
		b.WriteString("unchanged policy. Review before upgrading a pinned installation.\n\n")
		for _, c := range verdict {
			fmt.Fprintf(&b, "- %s\n", c.Subject)
		}
		b.WriteString("\n")
	}

	var breaking []Commit
	for _, cs := range byType {
		for _, c := range cs {
			if c.Breaking {
				breaking = append(breaking, c)
			}
		}
	}
	if len(breaking) > 0 {
		b.WriteString("## Breaking changes\n\n")
		sort.Slice(breaking, func(i, j int) bool { return breaking[i].Subject < breaking[j].Subject })
		for _, c := range breaking {
			fmt.Fprintf(&b, "- %s\n", c.Subject)
		}
		b.WriteString("\n")
	}

	for _, ty := range order {
		cs := byType[ty]
		if len(cs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", typeHeadings[ty])
		for _, c := range cs {
			fmt.Fprintf(&b, "- %s\n", c.Subject)
		}
		b.WriteString("\n")
	}

	if from != "" {
		fmt.Fprintf(&b, "---\n\nBump: **%s**. Changes from %s to %s.\n", bump, from, to)
	} else {
		fmt.Fprintf(&b, "---\n\nFirst release. Bump: **%s**.\n", bump)
	}
	b.WriteString("\nVerify this release against the workflow identity before installing; see `docs/install.md`.\n")
	return b.String()
}

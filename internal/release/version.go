// Package release computes the next version from commit history (PAWL-027).
//
// Written here rather than delegated to an off-the-shelf release tool for one
// reason, stated in PAWL-027's non-functional section: every such tool maps
// `fix` to a patch bump, and PAWL-013 AC2 says a fix that can change a gate
// verdict is a MAJOR change. A tool configured wrongly for that row ships a
// verdict change as a patch on every release rather than occasionally, which is
// worse than the manual process it replaces.
package release

import (
	"fmt"
	"strconv"
	"strings"
)

type Bump int

const (
	BumpNone Bump = iota
	BumpPatch
	BumpMinor
	BumpMajor
)

func (b Bump) String() string {
	switch b {
	case BumpPatch:
		return "patch"
	case BumpMinor:
		return "minor"
	case BumpMajor:
		return "major"
	}
	return "none"
}

// Commit is one parsed Conventional Commit.
type Commit struct {
	Type             string
	Breaking         bool // `!` or a BREAKING CHANGE footer
	VerdictAffecting bool // the Verdict-Affecting: yes trailer (AC3)
	Subject          string
}

// Types that imply a version change. Everything else — docs, test, ci, build,
// chore, refactor, style — implies none (AC13, last row).
var (
	minorTypes = map[string]bool{"feat": true}
	patchTypes = map[string]bool{"fix": true, "perf": true}
	knownTypes = map[string]bool{
		"feat": true, "fix": true, "perf": true, "docs": true, "test": true,
		"ci": true, "build": true, "chore": true, "refactor": true, "style": true,
		"revert": true,
	}
)

func KnownType(t string) bool { return knownTypes[t] }

// ParseCommit reads one commit message. A message that is not Conventional
// Commits returns ok=false; the caller decides whether that is fatal, because
// history predating PAWL-027 is not going to conform and rewriting it would
// violate the same instinct that makes records write-once.
func ParseCommit(msg string) (Commit, bool) {
	lines := strings.Split(msg, "\n")
	if len(lines) == 0 {
		return Commit{}, false
	}
	header := strings.TrimSpace(lines[0])

	colon := strings.Index(header, ":")
	if colon < 0 {
		return Commit{}, false
	}
	prefix := header[:colon]
	c := Commit{Subject: strings.TrimSpace(header[colon+1:])}

	if strings.HasSuffix(prefix, "!") {
		c.Breaking = true
		prefix = strings.TrimSuffix(prefix, "!")
	}
	// Drop an optional (scope).
	if p := strings.Index(prefix, "("); p >= 0 {
		if !strings.HasSuffix(prefix, ")") {
			return Commit{}, false
		}
		prefix = prefix[:p]
	}
	c.Type = strings.TrimSpace(prefix)
	if c.Type == "" || c.Subject == "" {
		return Commit{}, false
	}

	for _, l := range lines[1:] {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "BREAKING CHANGE:") || strings.HasPrefix(t, "BREAKING-CHANGE:") {
			c.Breaking = true
		}
		if v, ok := trailer(t, "Verdict-Affecting"); ok && strings.EqualFold(v, "yes") {
			c.VerdictAffecting = true
		}
	}
	return c, true
}

// trailer reads a `Key: value` footer, case-insensitively on the key.
func trailer(line, key string) (string, bool) {
	i := strings.Index(line, ":")
	if i < 0 {
		return "", false
	}
	if !strings.EqualFold(strings.TrimSpace(line[:i]), key) {
		return "", false
	}
	return strings.TrimSpace(line[i+1:]), true
}

// BumpFor returns the highest bump implied by the commits (AC13).
//
// Verdict-affecting is checked first and separately from Breaking, because the
// two are different promises to a client: a removed flag breaks their pipeline
// loudly, and a moved threshold does not break it at all — it silently changes
// which of their changesets merge.
func BumpFor(commits []Commit) Bump {
	b := BumpNone
	for _, c := range commits {
		var this Bump
		switch {
		case c.VerdictAffecting, c.Breaking:
			this = BumpMajor
		case minorTypes[c.Type]:
			this = BumpMinor
		case patchTypes[c.Type]:
			this = BumpPatch
		default:
			this = BumpNone
		}
		if this > b {
			b = this
		}
	}
	return b
}

type Version struct{ Major, Minor, Patch int }

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

func ParseVersion(s string) (Version, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("not a version: %q", s)
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("not a version: %q", s)
		}
		out[i] = n
	}
	return Version{out[0], out[1], out[2]}, nil
}

// Apply returns the next version.
//
// While the major version is 0, MAJOR moves MINOR and MINOR moves PATCH (AC14,
// and PAWL-013 AC5). PATCH stays PATCH, because there is no position below it —
// so this shifts what it can rather than everything, and the tests assert that.
//
// SemVer permits anything below 1.0 to change arbitrarily; a tool that can block
// a client's release does not get to use that latitude, and every off-the-shelf
// implementation takes it by default.
func Apply(v Version, b Bump) (Version, Bump) {
	if v.Major == 0 {
		switch b {
		case BumpMajor:
			b = BumpMinor
		case BumpMinor:
			b = BumpPatch
		}
	}
	switch b {
	case BumpMajor:
		return Version{v.Major + 1, 0, 0}, b
	case BumpMinor:
		return Version{v.Major, v.Minor + 1, 0}, b
	case BumpPatch:
		return Version{v.Major, v.Minor, v.Patch + 1}, b
	}
	return v, BumpNone
}

// Types returns the allowed commit types, in a stable order. Exported so the
// commit-msg hook can print them and so a test can hold this set and
// commitlint.config.js to the same list.
func Types() []string {
	return []string{
		"feat", "fix", "perf", "docs", "test",
		"ci", "build", "chore", "refactor", "style", "revert",
	}
}

// DecidingModules are the packages whose behaviour can change which changesets
// pass, and which therefore require a Verdict-Affecting declaration (AC4).
var DecidingModules = []string{
	"internal/policy/",
	"internal/resolve/",
	"internal/accounting/",
	"internal/evidence/",
}

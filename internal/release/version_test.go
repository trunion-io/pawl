package release

// PAWL-027 AC13 and AC14.

import (
	"strings"
	"testing"
)

func parse(t *testing.T, msgs ...string) []Commit {
	t.Helper()
	var out []Commit
	for _, m := range msgs {
		c, ok := ParseCommit(m)
		if !ok {
			t.Fatalf("could not parse %q", m)
		}
		out = append(out, c)
	}
	return out
}

// The row that differs from every off-the-shelf implementation. A fix that can
// move a verdict is MAJOR (PAWL-013 AC2), not a patch.
func TestVerdictAffectingFixIsMajor(t *testing.T) {
	c := parse(t, "fix: reject policy thresholds the gate cannot apply\n\nVerdict-Affecting: yes\n")
	if got := BumpFor(c); got != BumpMajor {
		t.Fatalf("a verdict-affecting fix must be major, got %s", got)
	}

	// Without the trailer the same commit is a patch — which is exactly why AC4
	// refuses to let a commit touching the deciding modules omit it.
	c = parse(t, "fix: reject policy thresholds the gate cannot apply\n")
	if got := BumpFor(c); got != BumpPatch {
		t.Fatalf("a plain fix is a patch, got %s", got)
	}
}

func TestBumpForCoversEveryRow(t *testing.T) {
	cases := []struct {
		msg  string
		want Bump
	}{
		{"feat: add pawl install verify", BumpMinor},
		{"fix: correct the anchor offset", BumpPatch},
		{"perf: cache the pending scan", BumpPatch},
		{"docs: explain the licence", BumpNone},
		{"test: cover the gate", BumpNone},
		{"ci: pin actions by sha", BumpNone},
		{"chore: tidy", BumpNone},
		{"refactor: split the resolver", BumpNone},
		{"feat!: drop the --legacy flag", BumpMajor},
		{"feat(gate)!: rename a flag", BumpMajor},
		{"fix: x\n\nBREAKING CHANGE: the flag moved\n", BumpMajor},
		{"chore: bump\n\nVerdict-Affecting: yes\n", BumpMajor},
		{"fix: x\n\nVerdict-Affecting: no\n", BumpPatch},
	}
	for _, c := range cases {
		got := BumpFor(parse(t, c.msg))
		if got != c.want {
			t.Errorf("%q -> %s, want %s", c.msg, got, c.want)
		}
	}
}

func TestHighestBumpWins(t *testing.T) {
	c := parse(t,
		"docs: tidy",
		"fix: correct an offset",
		"feat: add a command",
	)
	if got := BumpFor(c); got != BumpMinor {
		t.Errorf("got %s, want minor", got)
	}
	c = append(c, parse(t, "fix: threshold\n\nVerdict-Affecting: yes\n")...)
	if got := BumpFor(c); got != BumpMajor {
		t.Errorf("got %s, want major", got)
	}
}

// AC14 / PAWL-013 AC5 — pre-1.0 is not unconstrained. Every off-the-shelf tool
// gets this wrong by treating 0.x as a free-for-all.
func TestPreOneZeroShiftsBumpsDown(t *testing.T) {
	cases := []struct {
		from string
		b    Bump
		want string
	}{
		{"0.1.0", BumpMajor, "0.2.0"},
		{"0.1.0", BumpMinor, "0.1.1"},
		{"0.1.0", BumpPatch, "0.1.1"},
		{"0.1.0", BumpNone, "0.1.0"},
		{"1.2.3", BumpMajor, "2.0.0"},
		{"1.2.3", BumpMinor, "1.3.0"},
		{"1.2.3", BumpPatch, "1.2.4"},
	}
	for _, c := range cases {
		v, err := ParseVersion(c.from)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := Apply(v, c.b)
		if got.String() != c.want {
			t.Errorf("%s + %s -> %s, want %s", c.from, c.b, got, c.want)
		}
	}
}

func TestParseCommitRejectsNonConventional(t *testing.T) {
	for _, m := range []string{
		"just a message",
		"fix",
		": no type",
		"fix: ",
		"feat(scope: unbalanced",
		"",
	} {
		if c, ok := ParseCommit(m); ok {
			t.Errorf("%q parsed as %+v, want rejection", m, c)
		}
	}
}

func TestParseVersionTolerantOfPrerelease(t *testing.T) {
	for _, s := range []string{"v0.2.0-rc.1", "0.2.0", "v0.2.0", "0.2.0+build"} {
		v, err := ParseVersion(s)
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if v.String() != "0.2.0" {
			t.Errorf("%q -> %s", s, v)
		}
	}
}

// AC16 — a client's first question is whether verdicts moved, so it is answered
// first and separately rather than mixed in with the fixes.
func TestNotesPutVerdictAffectingFirst(t *testing.T) {
	commits := parse(t,
		"feat: add a command",
		"fix: correct an offset",
		"fix: reject bad thresholds\n\nVerdict-Affecting: yes\n",
		"docs: tidy",
	)
	n := Notes(commits, "v0.1.0", "v0.2.0", BumpMinor)

	iv := strings.Index(n, "Verdict-affecting changes")
	ifeat := strings.Index(n, "## Features")
	if iv < 0 {
		t.Fatal("no verdict-affecting section")
	}
	if ifeat >= 0 && iv > ifeat {
		t.Error("verdict-affecting changes must come before features")
	}
	if !strings.Contains(n, "reject bad thresholds") {
		t.Error("the verdict-affecting subject is missing")
	}
	// It must not also appear under Fixes.
	if strings.Count(n, "reject bad thresholds") != 1 {
		t.Error("a verdict-affecting commit must appear once, in its own section")
	}
}

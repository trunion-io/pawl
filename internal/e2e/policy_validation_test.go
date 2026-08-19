package e2e

// PAWL-026 — policy input validation.
//
// The thresholds are the client's (C-5) and the gate compares against them
// (PAWL-006 AC2, AC4). A threshold that silently becomes a different number than
// the one written in the file is this product's own failure mode aimed at
// itself: the operator reads their policy and believes it.

import (
	"path/filepath"
	"strings"
	"testing"

	"trunion.io/pawl/internal/policy"
)

func writePolicy(t *testing.T, repo, body string) {
	t.Helper()
	mustMkdir(t, filepath.Join(repo, ".pawl"))
	mustWrite(t, filepath.Join(repo, ".pawl", "policy.toml"), body)
}

// TestPolicyRejectsUnusableThresholds — PAWL-026 AC1, AC2, AC3, AC4.
func TestPolicyRejectsUnusableThresholds(t *testing.T) {
	cases := []struct {
		name, body, wantKey string
	}{
		{"negative changed lines", "[gate]\nmax_changed_lines = -1\n", "max_changed_lines"},
		{"negative unclaimed", "[gate]\nmax_unclaimed_lines = -5\n", "max_unclaimed_lines"},
		{"negative ratio", "[gate]\nmax_must_read_ratio = -0.5\n", "max_must_read_ratio"},
		{"string for a number", "[gate]\nmax_changed_lines = \"lots\"\n", "max_changed_lines"},
		{"number for a bool", "[gate]\nblock_on_undetermined = 3\n", "block_on_undetermined"},
		{"string for a list", "[gate]\nsensitive_paths = \"src/auth/\"\n", "sensitive_paths"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newRepo(t)
			writePolicy(t, repo, c.body)

			got, err := policy.Load(repo)
			if err == nil {
				t.Fatalf("expected rejection, got policy %+v", got)
			}
			// AC4: the operator has to know which line to edit.
			if !strings.Contains(err.Error(), c.wantKey) {
				t.Errorf("error must name the key %q, got: %v", c.wantKey, err)
			}
			if !strings.Contains(err.Error(), "policy.toml") {
				t.Errorf("error must name the file, got: %v", err)
			}
		})
	}
}

// TestPolicyRejectionDoesNotLeakAPartialPolicy — PAWL-026 AC6 and the
// fail-closed note.
//
// A rejected policy must not hand back the valid half of the file. Enforcing
// half a policy is enforcing a threshold set nobody wrote.
func TestPolicyRejectionDoesNotLeakAPartialPolicy(t *testing.T) {
	repo := newRepo(t)
	writePolicy(t, repo, "[gate]\nmax_changed_lines = 7\nmax_unclaimed_lines = -1\n")

	got, err := policy.Load(repo)
	if err == nil {
		t.Fatal("expected rejection")
	}
	if got.MaxChangedLines == 7 {
		t.Error("a rejected policy must not apply the values it did parse")
	}
	if got.MaxChangedLines != policy.Defaults().MaxChangedLines {
		t.Errorf("expected defaults on rejection, got %+v", got)
	}
}

// TestPolicyWarnsOnUnrecognisedKey — PAWL-026 AC5.
//
// Not fatal, so a file written for a later pawl still loads. Never silent: a
// typo leaving a default in force is the same failure as a truncated value.
func TestPolicyWarnsOnUnrecognisedKey(t *testing.T) {
	repo := newRepo(t)
	writePolicy(t, repo, "[gate]\nmax_changed_lines = 7\nmax_changed_line = 3\n")

	got, err := policy.Load(repo)
	if err != nil {
		t.Fatalf("an unrecognised key must not be fatal: %v", err)
	}
	if got.MaxChangedLines != 7 {
		t.Errorf("the recognised keys must still apply, got %d", got.MaxChangedLines)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("expected one warning, got %v", got.Warnings)
	}
	if !strings.Contains(got.Warnings[0], "max_changed_line") {
		t.Errorf("the warning must name the key, got %q", got.Warnings[0])
	}
}

// TestPolicyAcceptsWholeNumberRatio — a ratio of 1 should not be pedantry.
func TestPolicyAcceptsWholeNumberRatio(t *testing.T) {
	repo := newRepo(t)
	writePolicy(t, repo, "[gate]\nmax_must_read_ratio = 1\n")

	got, err := policy.Load(repo)
	if err != nil {
		t.Fatalf("a whole number is a valid ratio: %v", err)
	}
	if got.MaxMustReadRatio != 1.0 {
		t.Errorf("max_must_read_ratio = %v, want 1.0", got.MaxMustReadRatio)
	}
}

// TestPolicyValidValuesStillLoad guards against the validation rejecting things
// it should accept — the way a check like this usually goes wrong.
func TestPolicyValidValuesStillLoad(t *testing.T) {
	repo := newRepo(t)
	writePolicy(t, repo, `[gate]
max_changed_lines = 0
max_must_read_ratio = 0.0
max_unclaimed_lines = 0
block_on_undetermined = true
sensitive_paths = ["src/auth/"]
`)
	got, err := policy.Load(repo)
	if err != nil {
		t.Fatalf("zero is a legitimate threshold: %v", err)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("no warnings expected, got %v", got.Warnings)
	}
}

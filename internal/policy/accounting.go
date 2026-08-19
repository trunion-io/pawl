package policy

// Deterministic acknowledgement rules (PAWL-017 AC1–AC5).
//
// A rule may record that there was nothing to assume about a span. A rule may
// never assert *what* was assumed — it does not know, and a fabricated
// assumption is worse than an absent one. Acknowledgement is automatable
// precisely because it asserts nothing (AC3).
//
// Rules live in .pawl/policy.toml because they decide what escapes human
// attention, which makes them the client's under C-5 and puts a change to them
// in a reviewable diff. They must not be settable from the environment, for the
// same reason gate thresholds are not.

import (
	"path/filepath"
	"strings"

	"trunion.io/pawl/internal/model"
)

// Accounting holds the rules. Every one must be decidable from repository
// contents alone (AC2); anything needing judgement is a claim and needs a
// claimant.
type Accounting struct {
	// AcknowledgePaths are path prefixes or globs whose changes carry nothing
	// to assume — generated code, vendored trees, lockfiles.
	AcknowledgePaths []string
	// AcknowledgeFormattingOnly acknowledges a span whose whitespace-normalised
	// content is unchanged. pawl already normalises whitespace for
	// fingerprints, so a formatting-only change is provably non-semantic rather
	// than heuristically so.
	AcknowledgeFormattingOnly bool
}

// RuleMatch is a span a rule accounts for, and which rule did it.
type RuleMatch struct {
	Path      string
	StartLine int
	EndLine   int
	Rule      string
}

// MatchPath reports the rule acknowledging this path, or "".
//
// Both a trailing-slash prefix ("vendor/") and a glob ("*.pb.go") are accepted,
// because a client will reach for whichever fits and neither is ambiguous.
func (a Accounting) MatchPath(path string) string {
	for _, pat := range a.AcknowledgePaths {
		if strings.HasSuffix(pat, "/") {
			if strings.HasPrefix(path, pat) {
				return "path:" + pat
			}
			continue
		}
		if ok, _ := filepath.Match(pat, path); ok {
			return "path:" + pat
		}
		if ok, _ := filepath.Match(pat, filepath.Base(path)); ok {
			return "path:" + pat
		}
	}
	return ""
}

// FormattingOnly reports whether before and after differ only in whitespace.
//
// Deliberately reuses the fingerprint normalisation rather than a second
// notion of "same": if these two ever disagreed, a span could be acknowledged
// as unchanged while its claim reported drift.
func (a Accounting) FormattingOnly(before, after []string) bool {
	if !a.AcknowledgeFormattingOnly {
		return false
	}
	if before == nil || after == nil {
		return false
	}
	return model.FingerprintLines(before) == model.FingerprintLines(after)
}

func (a Accounting) Empty() bool {
	return len(a.AcknowledgePaths) == 0 && !a.AcknowledgeFormattingOnly
}

// loadAccounting reads the [accounting] table. Absent means no rules, which is
// the right default: a client opts in to automation, it is not done to them.
func loadAccounting(tables map[string]map[string]any) Accounting {
	var a Accounting
	values := tables["accounting"]
	if values == nil {
		return a
	}
	if v, ok := values["auto_acknowledge_paths"].([]string); ok {
		a.AcknowledgePaths = v
	}
	if v, ok := values["auto_acknowledge_formatting_only"].(bool); ok {
		a.AcknowledgeFormattingOnly = v
	}
	return a
}

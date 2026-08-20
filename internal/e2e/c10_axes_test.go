package e2e

// Constitution C-10 — the mechanical resolution of a span and its calibration
// sample verdict are separate fields, and neither is derived from the other.
//
// Written to close the rule rather than leave it "enforced by review". The
// property is already true of SampledSpan; what this stops is a later change
// that collapses the two — storing the review outcome back over the original
// verdict, or inferring one from the other — which would make the corpus unable
// to express the disagreement it exists to record.

import (
	"testing"

	"trunion.io/pawl/internal/calibrate"
	"trunion.io/pawl/internal/model"
)

func TestSampleVerdictAndMechanicalVerdictAreSeparateFields(t *testing.T) {
	// A span the machine cleared, which a reviewer says should have been read.
	// If either field were derived from the other this combination could not be
	// represented — and it is the combination the false-clear rate is built from.
	s := calibrate.SampledSpan{
		Path:      "internal/x.go",
		StartLine: 1,
		EndLine:   4,
		Verdict:   model.VerdictClear,
		Reviewed:  calibrate.VerdictFalseClear,
	}

	if s.Verdict != model.VerdictClear {
		t.Errorf("mechanical verdict = %q, want %q", s.Verdict, model.VerdictClear)
	}
	if s.Reviewed != calibrate.VerdictFalseClear {
		t.Errorf("sample verdict = %q, want %q", s.Reviewed, calibrate.VerdictFalseClear)
	}

	// The disagreeing pair is the point: recording the review must not alter what
	// the machine originally decided.
	if string(s.Verdict) == string(s.Reviewed) {
		t.Fatal("the two axes collapsed: a cleared span reviewed as a false clear must keep both values")
	}

	// And an unreviewed span carries a mechanical verdict with no sample verdict,
	// which a single collapsed field could not express either.
	pending := calibrate.SampledSpan{Verdict: model.VerdictAcknowledged}
	if pending.Reviewed != calibrate.VerdictPending {
		t.Errorf("an unreviewed span must have no sample verdict, got %q", pending.Reviewed)
	}
	if pending.Verdict != model.VerdictAcknowledged {
		t.Errorf("an unreviewed span must keep its mechanical verdict, got %q", pending.Verdict)
	}
}

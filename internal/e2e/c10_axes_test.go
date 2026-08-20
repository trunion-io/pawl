package e2e

// Constitution C-10 — the mechanical resolution of a span and its calibration
// sample verdict are separate fields, and neither is derived from the other.
//
// Written to close the rule rather than leave it "enforced by review". The
// property is already true of SampledSpan; what this stops is a later change
// that collapses the two — storing the review outcome back over the original
// verdict, or inferring one from the other — which would make the corpus unable
// to express the disagreement it exists to record.
//
// It exercises the two production paths that write these fields, because those
// are where a collapse would be introduced. An earlier version of this test
// built a SampledSpan literal and asserted the fields read back, which proved
// only that a struct can hold two values: it passed regardless of what
// FromReadingList and RecordVerdict did, including if either collapsed the
// axes. A test that cannot fail for the reason it was written is not a check.

import (
	"testing"
	"time"

	"trunion.io/pawl/internal/calibrate"
	"trunion.io/pawl/internal/model"
	"trunion.io/pawl/internal/policy"
)

// readingList is one cleared span and one acknowledged span — the two the
// sampler admits.
func readingList() model.ReadingList {
	return model.ReadingList{
		Tree:   "t",
		Commit: "c",
		Base:   "b",
		Spans: []model.ReadSpan{
			{Path: "internal/x.go", StartLine: 1, EndLine: 4, Verdict: model.VerdictClear},
			{Path: "internal/y.go", StartLine: 7, EndLine: 9, Verdict: model.VerdictAcknowledged},
		},
	}
}

// TestSamplingLeavesTheSampleVerdictUnsetAndTheMechanicalVerdictIntact covers
// the first half of C-10: sampling must not seed axis 2 from axis 1.
func TestSamplingLeavesTheSampleVerdictUnsetAndTheMechanicalVerdictIntact(t *testing.T) {
	s := calibrate.FromReadingList(readingList(), "v0", policy.Policy{}, "s1", time.Now())

	if len(s.Spans) != 2 {
		t.Fatalf("sampled %d spans, want 2", len(s.Spans))
	}
	want := map[string]model.SpanVerdict{
		"internal/x.go": model.VerdictClear,
		"internal/y.go": model.VerdictAcknowledged,
	}
	for _, sp := range s.Spans {
		// Axis 1 survives sampling unchanged.
		if sp.Verdict != want[sp.Path] {
			t.Errorf("%s: mechanical verdict = %q, want %q", sp.Path, sp.Verdict, want[sp.Path])
		}
		// Axis 2 is unset. A derived field would be populated here, and a
		// collapsed one would carry the mechanical verdict.
		if sp.Reviewed != calibrate.VerdictPending {
			t.Errorf("%s: sample verdict = %q, want unset — sampling must not decide it",
				sp.Path, sp.Reviewed)
		}
	}
}

// TestRecordingAReviewDoesNotOverwriteTheMechanicalVerdict covers the second
// half: the disagreeing pair must be representable after a review lands.
func TestRecordingAReviewDoesNotOverwriteTheMechanicalVerdict(t *testing.T) {
	s := calibrate.FromReadingList(readingList(), "v0", policy.Policy{}, "s1", time.Now())

	// The machine cleared this span; a reviewer says it should have been read.
	// This is the pair the false-clear rate is built from, and the one a single
	// collapsed field could not express.
	if err := s.RecordVerdict("internal/x.go", 1, 4, calibrate.VerdictFalseClear, "r", time.Now()); err != nil {
		t.Fatalf("RecordVerdict: %v", err)
	}

	sp := s.Spans[0]
	if sp.Verdict != model.VerdictClear {
		t.Errorf("recording a review changed the mechanical verdict to %q; C-10 requires it unchanged", sp.Verdict)
	}
	if sp.Reviewed != calibrate.VerdictFalseClear {
		t.Errorf("sample verdict = %q, want %q", sp.Reviewed, calibrate.VerdictFalseClear)
	}
	if string(sp.Verdict) == string(sp.Reviewed) {
		t.Fatal("the two axes collapsed: a cleared span reviewed as a false clear must keep both values")
	}

	// The untouched span keeps axis 1 and stays unreviewed — recording a verdict
	// on one span must not propagate.
	if other := s.Spans[1]; other.Reviewed != calibrate.VerdictPending || other.Verdict != model.VerdictAcknowledged {
		t.Errorf("second span = (%q, %q), want (%q, unset)",
			other.Verdict, other.Reviewed, model.VerdictAcknowledged)
	}
}

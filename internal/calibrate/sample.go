// Package calibrate is the calibration sampler (PAWL-007).
//
// If pawl is doing its job, humans read a small fraction of hunks — which means
// there is no ongoing signal on whether the trail is *accurate*. The trail
// becomes ritual, agents learn to emit whatever passes the checker, and it
// surfaces during an incident.
//
// This forces a full human read on a randomly selected cleared changeset and
// records whether the trail was faithful to what the diff actually did. It is
// the only asset here a competitor cannot fork: the schema is an afternoon's
// work, four hundred sampled changesets with recorded outcomes are not.
package calibrate

import (
	"time"

	"trunion.io/pawl/internal/model"
)

const SchemaVersion = "0.1"

// Verdict is axis 1: was clearing this span correct?
//
// Binary and permanent. The taxonomy that blocked this spec for months
// collapsed two orthogonal questions into one enum; splitting them is what makes
// the corpus survive later refinement. A fifth cause discovered in month six
// leaves every previously recorded false-clear rate comparable, because this
// axis cannot move.
type Verdict string

const (
	// VerdictPending means the span has not been reviewed yet.
	VerdictPending Verdict = ""
	// VerdictCorrect means no human needed to read this span.
	VerdictCorrect Verdict = "correct"
	// VerdictFalseClear means a human needed to read it and did not.
	VerdictFalseClear Verdict = "false_clear"
)

// Cause is axis 2: why did it clear when it should not have?
//
// Recorded per (span, claim) pair, because a span may be cleared by several
// claims and one may be sound while another is not.
type Cause string

const (
	// CauseClaimFalse — the claim asserts something untrue about the code.
	CauseClaimFalse Cause = "claim_false"
	// CauseClaimIncomplete — true, but does not address what needed review.
	CauseClaimIncomplete Cause = "claim_incomplete"
	// CauseAnchorWrong — bound to a span it does not describe. A pawl defect,
	// not a judgement about the claim, which is why it lives on this axis
	// rather than beside the others in one flat enum.
	CauseAnchorWrong Cause = "anchor_wrong"
	// CauseEvidenceHollow — the cited check exists and passes but does not
	// exercise the claim. The failure this spec's context names, and the one
	// only a human read can ever detect: pawl can check a check exists, never
	// that it is meaningful.
	CauseEvidenceHollow Cause = "evidence_hollow"
)

func (c Cause) Valid() bool {
	switch c {
	case CauseClaimFalse, CauseClaimIncomplete, CauseAnchorWrong, CauseEvidenceHollow:
		return true
	}
	return false
}

func Causes() []Cause {
	return []Cause{CauseClaimFalse, CauseClaimIncomplete, CauseAnchorWrong, CauseEvidenceHollow}
}

// SpanCause attributes one false clear to one claim.
type SpanCause struct {
	ClaimID string `json:"claim_id"`
	Cause   Cause  `json:"cause"`
}

// SampledSpan is one cleared or acknowledged span awaiting a human read.
type SampledSpan struct {
	Path      string             `json:"path"`
	StartLine int                `json:"start_line"`
	EndLine   int                `json:"end_line"`
	Verdict   model.SpanVerdict  `json:"original_verdict"`
	ClaimIDs  []string           `json:"claim_ids"`
	Roles     []model.AuthorRole `json:"author_roles"`

	Reviewed Verdict     `json:"reviewed"`
	Causes   []SpanCause `json:"causes,omitempty"`
}

func (s SampledSpan) Lines() int { return s.EndLine - s.StartLine + 1 }

// PolicySnapshot is the thresholds in force when the changeset was cleared
// (AC8). A false-clear rate mixing verdicts from different thresholds is not a
// rate of anything.
type PolicySnapshot struct {
	MaxChangedLines     int     `json:"max_changed_lines"`
	MaxMustReadRatio    float64 `json:"max_must_read_ratio"`
	MaxUnclaimedLines   int     `json:"max_unclaimed_lines"`
	BlockOnUndetermined bool    `json:"block_on_undetermined"`
}

// Sample is one changeset selected for review.
type Sample struct {
	SchemaVersion string    `json:"schema_version"`
	ID            string    `json:"id"`
	TS            time.Time `json:"ts"`

	Tree   string `json:"tree"`
	Commit string `json:"commit"`
	Base   string `json:"base"`

	// ToolVersion and Policy are what produced the verdicts under review
	// (AC8). PAWL-011 put the tool identity in the attestation; this is where
	// it earns its keep — pawl's verdicts change between versions, so a rate
	// that mixes them measures nothing.
	ToolVersion string         `json:"tool_version"`
	Policy      PolicySnapshot `json:"policy"`

	Spans []SampledSpan `json:"spans"`

	Reviewer   string     `json:"reviewer,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
}

// Phase1Complete reports whether every span carries an axis 1 verdict.
//
// This gates the reveal of claim text (AC7). Until it is true, a reviewer has
// not yet committed to an opinion and showing them the claim would anchor it.
func (s Sample) Phase1Complete() bool {
	for _, sp := range s.Spans {
		if sp.Reviewed == VerdictPending {
			return false
		}
	}
	return len(s.Spans) > 0
}

// FalseClears returns the spans a reviewer judged should have been read.
func (s Sample) FalseClears() []SampledSpan {
	var out []SampledSpan
	for _, sp := range s.Spans {
		if sp.Reviewed == VerdictFalseClear {
			out = append(out, sp)
		}
	}
	return out
}

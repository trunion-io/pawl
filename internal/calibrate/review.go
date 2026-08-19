package calibrate

import (
	"errors"
	"fmt"
	"time"
)

// ErrPhase1Incomplete is returned when phase 2 is attempted early.
//
// AC7 in one error: a reviewer must commit to whether a span needed reading
// *before* seeing what the agent claimed about it. An unblinded review is
// nearly worthless — the claim is a plausible story about the code, and reading
// it first makes it very hard to conclude the code needed attention anyway.
//
// Enforcing the order in the tool is what turns AC7 from a design aspiration
// into a mechanical check.
var ErrPhase1Incomplete = errors.New(
	"every span needs a verdict before claims are revealed: recording a cause " +
		"means judging a claim, and judging a claim you have already read is not " +
		"an independent judgement")

var ErrSpanNotFound = errors.New("no such span in this sample")

func (s *Sample) findSpan(path string, start, end int) *SampledSpan {
	for i := range s.Spans {
		sp := &s.Spans[i]
		if sp.Path == path && sp.StartLine == start && sp.EndLine == end {
			return sp
		}
	}
	return nil
}

// RecordVerdict sets axis 1 for one span (AC2).
func (s *Sample) RecordVerdict(path string, start, end int, v Verdict, reviewer string, now time.Time) error {
	if v != VerdictCorrect && v != VerdictFalseClear {
		return fmt.Errorf("verdict must be %q or %q", VerdictCorrect, VerdictFalseClear)
	}
	sp := s.findSpan(path, start, end)
	if sp == nil {
		return ErrSpanNotFound
	}
	sp.Reviewed = v
	s.Reviewer = reviewer
	t := now.UTC()
	s.ReviewedAt = &t
	return nil
}

// RecordCause sets axis 2 for one (span, claim) pair (AC3).
//
// Refuses until phase 1 is complete, and refuses on a span the reviewer judged
// correct — a cause explains a false clear, and attaching one to a span that
// cleared correctly would put noise in the only corpus that matters.
func (s *Sample) RecordCause(path string, start, end int, claimID string, c Cause) error {
	if !c.Valid() {
		return fmt.Errorf("unknown cause %q", c)
	}
	if !s.Phase1Complete() {
		return ErrPhase1Incomplete
	}
	sp := s.findSpan(path, start, end)
	if sp == nil {
		return ErrSpanNotFound
	}
	if sp.Reviewed != VerdictFalseClear {
		return fmt.Errorf("span was judged %q; a cause explains a false clear", sp.Reviewed)
	}
	for i := range sp.Causes {
		if sp.Causes[i].ClaimID == claimID {
			sp.Causes[i].Cause = c
			return nil
		}
	}
	sp.Causes = append(sp.Causes, SpanCause{ClaimID: claimID, Cause: c})
	return nil
}

// MayRevealClaims reports whether claim text can be shown yet (AC7).
func (s Sample) MayRevealClaims() bool { return s.Phase1Complete() }

// Pending reports spans still awaiting an axis 1 verdict.
func (s Sample) Pending() []SampledSpan {
	var out []SampledSpan
	for _, sp := range s.Spans {
		if sp.Reviewed == VerdictPending {
			out = append(out, sp)
		}
	}
	return out
}

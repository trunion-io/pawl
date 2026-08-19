// Package resolve turns claims plus evidence into a reading list.
//
// This package is the product. Everything else feeds it.
//
// The arbiter's job is not approve/reject — it is to compute the minimum set of
// hunks a human must actually read, and to assert that the remainder is
// mechanically covered. Two failure modes it must never have:
//
//   - Clearing a hunk nobody claimed. Silence from the agent is not coverage.
//   - Clearing a claim on the agent's say-so. An asserted test that does not
//     exist in the junit output is worse than no assertion, because it looks
//     like rigour.
package resolve

import (
	"fmt"
	"sort"
	"strings"

	"trunion.io/pawl/internal/anchor"
	"trunion.io/pawl/internal/evidence"
	"trunion.io/pawl/internal/gitutil"
	"trunion.io/pawl/internal/model"
)

func ResolveCoverage(
	claim model.Claim,
	ev *evidence.Evidence,
	start, end *int,
) (model.CoverageStatus, []string) {
	var detail []string

	if len(claim.VerifiedBy) == 0 {
		// No assertion. Fall back to whether the suite exercises the span at all.
		if start != nil && end != nil && ev.LinesCovered(claim.Path, *start, *end) {
			return model.CoverageImplicit, []string{
				fmt.Sprintf("lines %d-%d exercised by the suite; no check asserted", *start, *end),
			}
		}
		return model.CoverageUncovered, []string{"no check asserted and span not exercised"}
	}

	ok := true
	for _, ref := range claim.VerifiedBy {
		switch ref.Type {
		case model.EvidenceTest:
			passed, present := ev.TestPassed(ref.Ref)
			switch {
			case !present:
				ok = false
				detail = append(detail, "test not found in results: "+ref.Ref)
			case !passed:
				ok = false
				detail = append(detail, "test failed: "+ref.Ref)
			default:
				detail = append(detail, "test passed: "+ref.Ref)
			}

		case model.EvidenceCoverage:
			if start != nil && end != nil && ev.LinesCovered(claim.Path, *start, *end) {
				detail = append(detail, fmt.Sprintf("lines %d-%d exercised", *start, *end))
			} else {
				ok = false
				detail = append(detail, fmt.Sprintf("lines %s not fully exercised", rangeText(start, end)))
			}

		case model.EvidenceTypecheck:
			target := ref.Ref
			if target == "" {
				target = claim.Path
			}
			switch {
			case !ev.TypecheckRan:
				ok = false
				detail = append(detail, "typecheck asserted but no typecheck report supplied")
			case ev.CleanTypecheck[ref.Ref] || ev.CleanTypecheck[claim.Path]:
				detail = append(detail, "typecheck clean: "+target)
			default:
				ok = false
				detail = append(detail, "typecheck not clean: "+target)
			}

		case model.EvidencePolicy:
			allowed, present := ev.PolicyResults[ref.Ref]
			switch {
			case !present:
				ok = false
				detail = append(detail, "policy rule not evaluated: "+ref.Ref)
			case !allowed:
				ok = false
				detail = append(detail, "policy rule denied: "+ref.Ref)
			default:
				detail = append(detail, "policy rule allowed: "+ref.Ref)
			}

		case model.EvidenceSpec:
			if ev.SpecCriteria[ref.Ref] {
				detail = append(detail, "traces to checkable criterion: "+ref.Ref)
			} else {
				ok = false
				detail = append(detail,
					"criterion absent from signed spec, or not checkable: "+ref.Ref)
			}
		}
	}

	if ok {
		return model.CoverageVerified, detail
	}
	return model.CoverageUnverified, detail
}

func rangeText(start, end *int) string {
	if start == nil || end == nil {
		return "None-None"
	}
	return fmt.Sprintf("%d-%d", *start, *end)
}

func ResolveClaims(
	repo string,
	claims []model.Claim,
	ev *evidence.Evidence,
	rev string,
) []model.ResolvedClaim {
	resolved := make([]model.ResolvedClaim, 0, len(claims))
	for _, claim := range claims {
		status, start, end := anchor.Resolve(repo, claim, rev)
		if status == model.AnchorDrifted || status == model.AnchorOrphaned {
			resolved = append(resolved, model.ResolvedClaim{
				Claim:    claim,
				Anchor:   status,
				Coverage: model.CoverageUnverified,
				CoverageDetail: []string{
					"claim no longer binds to delivered code; " +
						"treated as unverified regardless of asserted checks",
				},
			})
			continue
		}
		coverage, detail := ResolveCoverage(claim, ev, start, end)
		resolved = append(resolved, model.ResolvedClaim{
			Claim:          claim,
			Anchor:         status,
			AnchoredStart:  start,
			AnchoredEnd:    end,
			Coverage:       coverage,
			CoverageDetail: detail,
		})
	}
	return resolved
}

// lineVerdict is the verdict for a single changed line.
//
// A line clears only if some claim covers it and no claim over it needs a human.
// Needs-human always wins the overlap, because the cost of wrongly collapsing a
// line is an unreviewed defect and the cost of wrongly expanding one is a few
// seconds of reading.
func lineVerdict(resolved []model.ResolvedClaim, path string, line int) (model.SpanVerdict, []string) {
	var over []model.ResolvedClaim
	for _, rc := range resolved {
		if rc.Claim.Path != path || rc.AnchoredStart == nil || rc.AnchoredEnd == nil {
			continue
		}
		if *rc.AnchoredStart <= line && line <= *rc.AnchoredEnd {
			over = append(over, rc)
		}
	}
	if len(over) == 0 {
		return model.VerdictUnclaimed, nil
	}
	ids := make([]string, 0, len(over))
	needsHuman := false
	for _, rc := range over {
		ids = append(ids, rc.Claim.ID)
		if rc.NeedsHuman() {
			needsHuman = true
		}
	}
	if needsHuman {
		return model.VerdictUnverified, ids
	}
	return model.VerdictClear, ids
}

// isReviewable reports whether a line carries meaning to review.
//
// Blank lines and bare delimiters do not. Counting them as unclaimed changed
// code inflates the denominator and puts noise on the reading list, which is how
// a good ratio ends up looking bad. Deliberately narrow: comments are
// reviewable, because a wrong comment is a defect and agents write plenty.
func isReviewable(text string) bool {
	stripped := strings.TrimSpace(text)
	if stripped == "" {
		return false
	}
	switch stripped {
	case "}", ")", "]", "};", ");", "],", "),":
		return false
	}
	return true
}

// spansForHunk splits a hunk into contiguous runs of lines sharing a verdict.
func spansForHunk(resolved []model.ResolvedClaim, hunk model.Hunk, source []string) []model.ReadSpan {
	var spans []model.ReadSpan

	var runStart, runEnd int
	var runVerdict model.SpanVerdict
	var runIDs []string
	haveRun := false

	flush := func() {
		if !haveRun {
			return
		}
		ids := runIDs
		if ids == nil {
			ids = []string{}
		}
		spans = append(spans, model.ReadSpan{
			Path:      hunk.Path,
			StartLine: runStart,
			EndLine:   runEnd,
			Verdict:   runVerdict,
			ClaimIDs:  ids,
		})
	}

	for line := hunk.StartLine; line <= hunk.EndLine; line++ {
		if source != nil {
			if line > len(source) || !isReviewable(source[line-1]) {
				continue
			}
		}
		verdict, ids := lineVerdict(resolved, hunk.Path, line)
		sorted := append([]string(nil), ids...)
		sort.Strings(sorted)

		sameKey := haveRun && verdict == runVerdict && equalIDs(sorted, runIDs)
		contiguous := haveRun && line == runEnd+1
		// Break the run on a verdict change or a gap left by a skipped line.
		if !sameKey || !contiguous {
			flush()
			runStart, runVerdict, runIDs = line, verdict, sorted
			haveRun = true
		}
		runEnd = line
	}
	flush()
	return spans
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func BuildReadingList(
	repo string,
	base string,
	claims []model.Claim,
	ev *evidence.Evidence,
	rev string,
) (model.ReadingList, error) {
	hunks, err := gitutil.ChangedHunks(repo, base, rev, nil)
	if err != nil {
		return model.ReadingList{}, err
	}
	resolved := ResolveClaims(repo, claims, ev, rev)

	spans := []model.ReadSpan{}
	sourceCache := map[string][]string{}
	for _, hunk := range hunks {
		if _, ok := sourceCache[hunk.Path]; !ok {
			sourceCache[hunk.Path] = gitutil.ReadFileAt(repo, hunk.Path, rev)
		}
		spans = append(spans, spansForHunk(resolved, hunk, sourceCache[hunk.Path])...)
	}

	tree, err := gitutil.TreeHash(repo, rev)
	if err != nil {
		return model.ReadingList{}, err
	}
	commit, err := gitutil.CommitSHA(repo, rev)
	if err != nil {
		return model.ReadingList{}, err
	}

	if resolved == nil {
		resolved = []model.ResolvedClaim{}
	}
	return model.ReadingList{
		Tree:   tree,
		Commit: commit,
		Base:   base,
		Spans:  spans,
		Claims: resolved,
	}, nil
}

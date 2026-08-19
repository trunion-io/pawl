package calibrate

import (
	"sort"

	"trunion.io/pawl/internal/model"
)

// Report is what goes in front of a client, and what an auditor is shown
// instead of "we have a process" (AC4–AC6).
type Report struct {
	Samples        int     `json:"samples"`
	ReviewedSpans  int     `json:"reviewed_spans"`
	FalseClears    int     `json:"false_clears"`
	FalseClearRate float64 `json:"false_clear_rate"`

	// ByRole is the handover curve (AC5): the rate on client-authored
	// changesets falling over time is the leading indicator that an engagement
	// is finishable.
	ByRole map[string]RoleRate `json:"by_role"`
	// ByCause separates what to act on (AC6). "Improve the agents" and "fix the
	// anchoring" are different projects, and a single rate cannot tell them
	// apart.
	ByCause map[string]int `json:"by_cause"`

	// PendingReview is sample count awaiting a human. Reported because a
	// false-clear rate computed over three reviewed samples is not a number
	// anyone should quote.
	PendingReview int `json:"pending_review"`
	// ToolVersions present in the corpus. A rate mixing verdicts from
	// different verifiers is not a rate of anything (AC8), so if this has more
	// than one entry the headline needs qualifying.
	ToolVersions []string `json:"tool_versions"`
}

type RoleRate struct {
	Spans       int     `json:"spans"`
	FalseClears int     `json:"false_clears"`
	Rate        float64 `json:"rate"`
}

// Summarise computes the report over the given samples.
//
// Only reviewed spans count toward the rate. An unreviewed span is not evidence
// that clearing was correct — it is evidence that nobody has looked, and
// counting it as correct would make the number improve simply by sampling more
// and reviewing less.
func Summarise(samples []Sample) Report {
	r := Report{
		ByRole:  map[string]RoleRate{},
		ByCause: map[string]int{},
	}

	versions := map[string]bool{}
	for _, s := range samples {
		r.Samples++
		if !s.Phase1Complete() {
			r.PendingReview++
		}
		if s.ToolVersion != "" {
			versions[s.ToolVersion] = true
		}

		for _, sp := range s.Spans {
			if sp.Reviewed == VerdictPending {
				continue
			}
			r.ReviewedSpans++
			isFalse := sp.Reviewed == VerdictFalseClear
			if isFalse {
				r.FalseClears++
				for _, c := range sp.Causes {
					r.ByCause[string(c.Cause)]++
				}
			}

			for _, role := range rolesOf(sp) {
				e := r.ByRole[role]
				e.Spans++
				if isFalse {
					e.FalseClears++
				}
				if e.Spans > 0 {
					e.Rate = float64(e.FalseClears) / float64(e.Spans)
				}
				r.ByRole[role] = e
			}
		}
	}

	if r.ReviewedSpans > 0 {
		r.FalseClearRate = float64(r.FalseClears) / float64(r.ReviewedSpans)
	}
	for v := range versions {
		r.ToolVersions = append(r.ToolVersions, v)
	}
	sort.Strings(r.ToolVersions)
	return r
}

// rolesOf attributes a span to author roles. A span cleared by an
// acknowledgement carries no claim and therefore no role; it is attributed to
// "acknowledged" so that waving code through is visible in the breakdown rather
// than vanishing from it.
func rolesOf(sp SampledSpan) []string {
	if sp.Verdict == model.VerdictAcknowledged || len(sp.Roles) == 0 {
		return []string{"acknowledged"}
	}
	seen := map[string]bool{}
	var out []string
	for _, role := range sp.Roles {
		if !seen[string(role)] {
			seen[string(role)] = true
			out = append(out, string(role))
		}
	}
	sort.Strings(out)
	return out
}

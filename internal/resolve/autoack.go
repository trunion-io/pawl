package resolve

import (
	"trunion.io/pawl/internal/gitutil"
	"trunion.io/pawl/internal/model"
	"trunion.io/pawl/internal/policy"
)

// AutoAck reports the pending spans a deterministic rule accounts for, and
// which rule accounts for each (PAWL-017 AC1).
//
// It decides only; recording is the caller's job. Keeping the decision separate
// from the write means a caller can show what would happen — and means this
// function has no side effects to reason about.
//
// Nothing here is a heuristic. A rule either matches a path the client listed,
// or the span's whitespace-normalised content is unchanged against HEAD, which
// is decidable from repository contents alone (AC2).
func AutoAck(
	repo string,
	pending []PendingSpan,
	acc policy.Accounting,
) []policy.RuleMatch {
	if acc.Empty() || len(pending) == 0 {
		return nil
	}

	// Read each side once. HEAD is what the span is compared against for the
	// formatting-only rule; a file absent from HEAD is new, and a new file has
	// no unchanged formatting to detect.
	head := map[string][]string{}
	work := map[string][]string{}
	readHead := func(path string) []string {
		if _, ok := head[path]; !ok {
			head[path] = gitutil.ReadFileAt(repo, path, "HEAD")
		}
		return head[path]
	}
	readWork := func(path string) []string {
		if _, ok := work[path]; !ok {
			work[path] = gitutil.ReadWorktree(repo, path)
		}
		return work[path]
	}

	var out []policy.RuleMatch
	for _, s := range pending {
		if rule := acc.MatchPath(s.Path); rule != "" {
			out = append(out, policy.RuleMatch{
				Path: s.Path, StartLine: s.StartLine, EndLine: s.EndLine, Rule: rule,
			})
			continue
		}

		before, after := readHead(s.Path), readWork(s.Path)
		if before == nil || after == nil {
			continue
		}
		if s.StartLine < 1 || s.EndLine > len(after) {
			continue
		}
		// Compare the whole file rather than the span: a span's line numbers
		// mean different things on either side of a reformat, so comparing
		// span-to-span would compare unrelated lines. If the file is unchanged
		// once normalised, every span in it is.
		if acc.FormattingOnly(before, after) {
			out = append(out, policy.RuleMatch{
				Path: s.Path, StartLine: s.StartLine, EndLine: s.EndLine,
				Rule: "formatting_only",
			})
		}
	}
	return out
}

// RuleAcknowledgement builds the record for a rule match. Origin and Rule are
// set here so no caller can record a rule-produced acknowledgement that fails
// to say so (AC6, AC7).
func RuleAcknowledgement(m policy.RuleMatch) (path string, start, end int, origin model.RecordOrigin, rule string) {
	return m.Path, m.StartLine, m.EndLine, model.OriginRule, m.Rule
}

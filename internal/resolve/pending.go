package resolve

import (
	"trunion.io/pawl/internal/anchor"
	"trunion.io/pawl/internal/gitutil"
	"trunion.io/pawl/internal/model"
)

// PendingSpan is changed code in the working tree carrying neither a claim nor
// an acknowledgement.
type PendingSpan struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

func (p PendingSpan) Lines() int { return p.EndLine - p.StartLine + 1 }

// Pending reports unaccounted spans in the **working tree** (PAWL-016).
//
// This answers a different question from BuildReadingList, at a different
// moment. `verify` asks "is this committed changeset verified?" against a base
// ref, and needs evidence. This asks "is what I just edited accounted for?"
// against uncommitted work, and deliberately needs no evidence at all (AC3) —
// the tests for an edit made thirty seconds ago have not run, and requiring them
// would make the hook wait on a run that has not happened.
//
// Records resolve against the working tree rather than a committed revision
// (AC2), because the edit being accounted for is by definition not committed.
// Same reason `pawl claim` reads its span from the worktree.
//
// paths, when non-empty, restricts the report to those files — a hook firing on
// one edit does not want the whole tree.
func Pending(
	repo string,
	claims []model.Claim,
	acks []model.Acknowledgement,
	paths []string,
) ([]PendingSpan, error) {
	hunks, err := gitutil.WorktreeHunks(repo, nil)
	if err != nil {
		return nil, err
	}

	wanted := map[string]bool{}
	for _, p := range paths {
		wanted[p] = true
	}

	source := map[string][]string{}
	linesFor := func(path string) []string {
		if _, ok := source[path]; !ok {
			source[path] = gitutil.ReadWorktree(repo, path)
		}
		return source[path]
	}

	// Locate every record against the worktree. A record whose fingerprint no
	// longer matches has stopped describing this code, so its span is pending
	// again — the judgement C-4 makes about a drifted claim, applied earlier.
	type span struct{ start, end int }
	covered := map[string][]span{}
	addCover := func(path string, start, end int, fp string) {
		lines := linesFor(path)
		if lines == nil {
			return
		}
		if s, e, ok := anchor.ResolveInLines(lines, start, end, fp); ok {
			covered[path] = append(covered[path], span{s, e})
		}
	}
	for _, c := range claims {
		addCover(c.Path, c.StartLine, c.EndLine, c.Fingerprint)
	}
	for _, a := range acks {
		addCover(a.Path, a.StartLine, a.EndLine, a.Fingerprint)
	}

	var out []PendingSpan
	for _, h := range hunks {
		if len(wanted) > 0 && !wanted[h.Path] {
			continue
		}
		lines := linesFor(h.Path)

		// Walk the hunk and emit contiguous runs of unaccounted, reviewable
		// lines. Same filters the reading list applies (AC5): asking an agent to
		// account for a blank line teaches it to ignore the hook.
		runStart, runEnd, have := 0, 0, false
		flush := func() {
			if have {
				out = append(out, PendingSpan{Path: h.Path, StartLine: runStart, EndLine: runEnd})
				have = false
			}
		}
		for line := h.StartLine; line <= h.EndLine; line++ {
			if lines != nil && (line > len(lines) || !isReviewable(lines[line-1])) {
				flush()
				continue
			}
			accounted := false
			for _, c := range covered[h.Path] {
				if c.start <= line && line <= c.end {
					accounted = true
					break
				}
			}
			if accounted {
				flush()
				continue
			}
			if !have || line != runEnd+1 {
				flush()
				runStart, have = line, true
			}
			runEnd = line
		}
		flush()
	}
	return out, nil
}

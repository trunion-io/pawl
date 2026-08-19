// Package policy is policy pack v0.
//
// Three rules, because three rules you actually believe in beat thirty you
// copied. Everything here is a candidate for migration into an OPA bundle once a
// client needs rules this shape cannot express — the split is deliberate: this
// file is the mechanism, the thresholds are the client's.
//
// That ownership split is not decoration. If the supplier both writes the code
// and sets the bar it clears, the gate is theatre. The factory brings the
// mechanism and a defensible default; the client sets the numbers and holds the
// sign-off.
package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"trunion.io/pawl/internal/model"
)

const PolicyFile = ".pawl/policy.toml"

// Policy is a struct rather than the Python's dict[str, object]. The thresholds
// are the client's, but the *set* of thresholds is ours, and a typo in a key
// name should not silently fall back to a default at gate time.
type Policy struct {
	// MaxChangedLines: comprehension has a hard ceiling regardless of trail
	// quality. A 3,000 line changeset is unreadable even fully annotated, and
	// the claims degrade too as the agent's own context fills. Size is a gate,
	// not a style note.
	MaxChangedLines int
	// MaxMustReadRatio: what fraction of changed lines may land on the reading
	// list before the changeset is judged not worth reviewing incrementally.
	MaxMustReadRatio float64
	// MaxUnclaimedLines: changed code with no claim over it at all.
	MaxUnclaimedLines int
	// BlockOnUndetermined: an agent that could not establish something and
	// proceeded anyway always escalates, whatever the tests say.
	BlockOnUndetermined bool
	// SensitivePaths: paths where implicit coverage is not good enough and a
	// claim must assert a named check.
	SensitivePaths []string
}

func Defaults() Policy {
	return Policy{
		MaxChangedLines:     400,
		MaxMustReadRatio:    0.35,
		MaxUnclaimedLines:   0,
		BlockOnUndetermined: true,
		SensitivePaths:      nil,
	}
}

type Violation struct {
	Rule   string `json:"rule"`
	Detail string `json:"detail"`
}

type Decision struct {
	Allowed    bool        `json:"allowed"`
	Escalate   bool        `json:"escalate"`
	Violations []Violation `json:"violations"`
}

func Load(repo string) (Policy, error) {
	p := Defaults()
	path := filepath.Join(repo, PolicyFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil
		}
		return p, err
	}

	tables, err := parseTOML(string(b))
	if err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	// Accept either a [gate] table or a bare top-level document.
	values := tables["gate"]
	if values == nil {
		values = tables[""]
	}

	if v, ok := values["max_changed_lines"].(int64); ok {
		p.MaxChangedLines = int(v)
	}
	if v, ok := values["max_must_read_ratio"].(float64); ok {
		p.MaxMustReadRatio = v
	}
	if v, ok := values["max_must_read_ratio"].(int64); ok {
		p.MaxMustReadRatio = float64(v)
	}
	if v, ok := values["max_unclaimed_lines"].(int64); ok {
		p.MaxUnclaimedLines = int(v)
	}
	if v, ok := values["block_on_undetermined"].(bool); ok {
		p.BlockOnUndetermined = v
	}
	if v, ok := values["sensitive_paths"].([]string); ok {
		p.SensitivePaths = v
	}
	return p, nil
}

func Evaluate(rl model.ReadingList, p Policy) Decision {
	var violations []Violation
	s := rl.Summary()

	if s.ChangedLines > p.MaxChangedLines {
		violations = append(violations, Violation{
			Rule: "changeset_size",
			Detail: fmt.Sprintf("%d changed lines exceeds budget of %d; decompose",
				s.ChangedLines, p.MaxChangedLines),
		})
	}

	if s.ChangedLines > 0 {
		ratio := float64(s.MustReadLines) / float64(s.ChangedLines)
		if ratio > p.MaxMustReadRatio {
			violations = append(violations, Violation{
				Rule: "must_read_ratio",
				Detail: fmt.Sprintf("%.0f%% of changed lines need a human (limit %.0f%%)",
					ratio*100, p.MaxMustReadRatio*100),
			})
		}
	}

	if s.UnclaimedLines > p.MaxUnclaimedLines {
		violations = append(violations, Violation{
			Rule: "unclaimed_lines",
			Detail: fmt.Sprintf("%d changed lines carry no claim (limit %d)",
				s.UnclaimedLines, p.MaxUnclaimedLines),
		})
	}

	if p.BlockOnUndetermined {
		undetermined := 0
		for _, rc := range rl.Claims {
			if rc.Claim.Kind == model.KindUndetermined {
				undetermined++
			}
		}
		if undetermined > 0 {
			violations = append(violations, Violation{
				Rule:   "undetermined_claims",
				Detail: fmt.Sprintf("%d claim(s) the agent could not establish", undetermined),
			})
		}
	}

	for _, prefix := range p.SensitivePaths {
		for _, rc := range rl.Claims {
			if !strings.HasPrefix(rc.Claim.Path, prefix) {
				continue
			}
			if len(rc.Claim.VerifiedBy) == 0 {
				violations = append(violations, Violation{
					Rule: "sensitive_path_needs_named_check",
					Detail: fmt.Sprintf("%s:%d is on a sensitive path and asserts no named check",
						rc.Claim.Path, rc.Claim.StartLine),
				})
			}
		}
	}

	if violations == nil {
		violations = []Violation{}
	}
	return Decision{
		Allowed:    len(violations) == 0,
		Escalate:   len(violations) > 0,
		Violations: violations,
	}
}

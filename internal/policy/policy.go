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
	"sort"
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
	// Warnings holds diagnostics that do not invalidate the policy (PAWL-026
	// AC5), such as a key the gate does not recognise.
	Warnings []string
	// SensitivePaths: paths where implicit coverage is not good enough and a
	// claim must assert a named check.
	SensitivePaths []string
	// Accounting holds the deterministic acknowledgement rules (PAWL-017).
	// Separate from the gate thresholds above: these decide what an agent is
	// not asked about, not what may merge.
	Accounting Accounting
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

// Threshold parsing (PAWL-026).
//
// AC6 is the rule these implement: reject a policy no less strictly than it
// would have been applied. Refusing to run is a safe outcome; enforcing a
// threshold the operator did not write is not, and the operator has no way to
// notice — they read their own file and believe it.
//
// The int64-to-int conversions these replace were reported as high severity by
// static analysis. They could not truncate on any platform pawl ships, where int
// is 64 bits, so the finding was not exploitable. It is fixed by construction
// anyway: the safety of that conversion is a property of the current build
// targets rather than of the language, and a future GOARCH=386 would invalidate
// it silently.

func typeError(path, key, want string, got any) error {
	return fmt.Errorf("%s: %s must be %s, got %v (%T)", path, key, want, got, got)
}

// intThreshold reads a whole-number threshold, rejecting anything the gate
// cannot hold exactly (AC1), anything negative (AC2) and anything of the wrong
// type (AC3).
func intThreshold(values map[string]any, key, path string) (int, bool, error) {
	raw, present := values[key]
	if !present {
		return 0, false, nil
	}
	v, ok := raw.(int64)
	if !ok {
		return 0, false, typeError(path, key, "a whole number", raw)
	}
	if v < 0 {
		// Not merely meaningless: a negative bound invites a comparison written
		// for a positive one to read it as "no limit".
		return 0, false, fmt.Errorf("%s: %s must not be negative, got %d", path, key, v)
	}
	if int64(int(v)) != v {
		return 0, false, fmt.Errorf(
			"%s: %s is %d, which this build cannot represent exactly", path, key, v)
	}
	return int(v), true, nil
}

// floatThreshold accepts a whole number too, since 1 is a reasonable way to
// write a ratio of 1.0 and rejecting it would be pedantry rather than safety.
func floatThreshold(values map[string]any, key, path string) (float64, bool, error) {
	raw, present := values[key]
	if !present {
		return 0, false, nil
	}
	var v float64
	switch n := raw.(type) {
	case float64:
		v = n
	case int64:
		v = float64(n)
	default:
		return 0, false, typeError(path, key, "a number", raw)
	}
	if v < 0 {
		return 0, false, fmt.Errorf("%s: %s must not be negative, got %v", path, key, v)
	}
	return v, true, nil
}

// knownKeys is the set the gate acts on. The struct comment above has always
// said a typo in a key name should not silently fall back to a default; this is
// what makes that true.
var knownKeys = map[string]bool{
	"max_changed_lines":     true,
	"max_must_read_ratio":   true,
	"max_unclaimed_lines":   true,
	"block_on_undetermined": true,
	"sensitive_paths":       true,
}

// unknownKeys reports keys the gate does not act on (AC5).
//
// A warning rather than a rejection, deliberately. A misspelled key leaving a
// default silently in force is the same failure as a truncated value — the
// operator believes they configured something and did not — but rejecting
// outright would break a policy file written for a later pawl against an older
// binary. Resolving that tension properly needs a schema version, which PAWL-026
// names and does not decide.
func unknownKeys(values map[string]any, path string) []string {
	var out []string
	for k := range values {
		if !knownKeys[k] {
			out = append(out, fmt.Sprintf(
				"%s: unrecognised key %q; it has no effect and the default remains in force", path, k))
		}
	}
	sort.Strings(out)
	return out
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

	if v, ok, err := intThreshold(values, "max_changed_lines", path); err != nil {
		return Defaults(), err
	} else if ok {
		p.MaxChangedLines = v
	}
	if v, ok, err := floatThreshold(values, "max_must_read_ratio", path); err != nil {
		return Defaults(), err
	} else if ok {
		p.MaxMustReadRatio = v
	}
	if v, ok, err := intThreshold(values, "max_unclaimed_lines", path); err != nil {
		return Defaults(), err
	} else if ok {
		p.MaxUnclaimedLines = v
	}
	if raw, present := values["block_on_undetermined"]; present {
		v, ok := raw.(bool)
		if !ok {
			return Defaults(), typeError(path, "block_on_undetermined", "true or false", raw)
		}
		p.BlockOnUndetermined = v
	}
	if raw, present := values["sensitive_paths"]; present {
		v, ok := raw.([]string)
		if !ok {
			return Defaults(), typeError(path, "sensitive_paths", "a list of strings", raw)
		}
		p.SensitivePaths = v
	}

	p.Warnings = unknownKeys(values, path)
	p.Accounting = loadAccounting(tables)
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

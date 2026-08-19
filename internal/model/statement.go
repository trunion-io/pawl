package model

// in-toto Statement v1. The subject is the tree, not a built artifact: the
// changeset is the deliverable, so that is what gets attested.
//
// The Python version models this with a `type` field and then swaps the key to
// `_type` in a dump() hook, because `_type` is not a legal Python identifier.
// A struct tag says it directly, which removes a hand-written serialisation step
// and the chance of it drifting from the model.

type Statement struct {
	Type          string    `json:"_type"`
	Subject       []Subject `json:"subject"`
	PredicateType string    `json:"predicateType"`
	Predicate     Predicate `json:"predicate"`
}

type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type Predicate struct {
	SchemaVersion string                  `json:"schemaVersion"`
	GeneratedAt   string                  `json:"generatedAt"`
	Base          string                  `json:"base"`
	Commit        string                  `json:"commit"`
	Ticket        *string                 `json:"ticket"`
	PolicyPack    *string                 `json:"policyPack"`
	Summary       Summary                 `json:"summary"`
	Claims        []AttestedClaim         `json:"claims"`
	ReadingList   []AttestedSpan          `json:"readingList"`
	RoleBreakdown map[string]RoleTallyOut `json:"authorRoleBreakdown"`
}

type AttestedClaim struct {
	ID             string          `json:"id"`
	Kind           ClaimKind       `json:"kind"`
	Text           string          `json:"text"`
	Path           string          `json:"path"`
	RecordedRange  []int           `json:"recorded_range"`
	AnchoredRange  []int           `json:"anchored_range"`
	Anchor         AnchorStatus    `json:"anchor"`
	Coverage       CoverageStatus  `json:"coverage"`
	CoverageDetail []string        `json:"coverage_detail"`
	Asserted       []AssertedCheck `json:"asserted"`
	AuthorRole     AuthorRole      `json:"author_role"`
	Harness        *string         `json:"harness"`
	Model          *string         `json:"model"`
	RecordedAt     string          `json:"recorded_at"`
	NeedsHuman     bool            `json:"needs_human"`
}

type AssertedCheck struct {
	Type EvidenceType `json:"type"`
	Ref  string       `json:"ref"`
}

type AttestedSpan struct {
	Path    string      `json:"path"`
	Range   []int       `json:"range"`
	Verdict SpanVerdict `json:"verdict"`
	Claims  []string    `json:"claims"`
}

// RoleTallyOut is recorded so a later audit can tell what the trail was worth
// without re-deriving it. Calibration data is the only part of this that a
// competitor cannot fork.
type RoleTallyOut struct {
	Claims     int `json:"claims"`
	NeedsHuman int `json:"needs_human"`
}

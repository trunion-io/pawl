// Package model is the core schema for pawl.
//
// The claim is the load-bearing artifact. Everything else in this module exists
// to bind claims to a changeset and decide which of them a human must read.
//
// Design notes that are deliberate, not incidental:
//
//   - Claims are emitted at edit time, not at PR time. A claim assembled after
//     the fact is a model re-reading its own diff, which is confabulation, not
//     evidence.
//   - Claims anchor to a content fingerprint, not just a line range. Line
//     numbers drift as an agent keeps working; the fingerprint survives that and
//     lets us report honestly when it does not.
//   - VerifiedBy is a claim about how the claim is checked. The verifier decides
//     whether that check actually exists and actually passed. An agent asserting
//     its own coverage is worth nothing on its own.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

const (
	ClaimPredicateType = "https://trunion.io/attestations/assumption-trail/v0.1"
	SchemaVersion      = "0.1"
)

// ClaimKind is what sort of statement the agent is making about its own work.
//
// Go has no enum type, so these are named string constants with a Valid method
// the CLI calls explicitly. Pydantic gave the Python version this validation for
// free at the parse boundary; here it is a thing you have to remember to do.
type ClaimKind string

const (
	// KindAssumption is something taken as true without proof at the point of
	// writing.
	KindAssumption ClaimKind = "assumption"
	// KindRejectedAlternative is a path considered and not taken. Unrecoverable
	// from the diff after the fact, which is exactly why it has to be captured
	// at edit time.
	KindRejectedAlternative ClaimKind = "rejected_alternative"
	// KindUndetermined means the agent could not establish something and
	// proceeded anyway. Always lands on the reading list regardless of coverage.
	KindUndetermined ClaimKind = "undetermined"
	// KindConstraint is a requirement the agent believes this code must
	// satisfy, usually traced back to a spec acceptance criterion.
	KindConstraint ClaimKind = "constraint"
)

func (k ClaimKind) Valid() bool {
	switch k {
	case KindAssumption, KindRejectedAlternative, KindUndetermined, KindConstraint:
		return true
	}
	return false
}

func ClaimKinds() []ClaimKind {
	return []ClaimKind{KindAssumption, KindRejectedAlternative, KindUndetermined, KindConstraint}
}

// AuthorRole records who produced the edit. Tagged so handover can be measured
// directly: the ratio of clean clears on client-authored changesets over time is
// the leading indicator that an engagement is finishable.
type AuthorRole string

const (
	RoleAgent  AuthorRole = "agent"
	RoleExpert AuthorRole = "expert"
	RoleClient AuthorRole = "client"
)

func (r AuthorRole) Valid() bool {
	switch r {
	case RoleAgent, RoleExpert, RoleClient:
		return true
	}
	return false
}

func AuthorRoles() []AuthorRole {
	return []AuthorRole{RoleAgent, RoleExpert, RoleClient}
}

type EvidenceType string

const (
	// EvidenceTest is a named test node id, e.g. tests/test_auth.py::test_expiry.
	EvidenceTest EvidenceType = "test"
	// EvidenceTypecheck means the file is clean under mypy/tsc.
	EvidenceTypecheck EvidenceType = "typecheck"
	// EvidencePolicy is a named policy rule evaluated by OPA.
	EvidencePolicy EvidenceType = "policy"
	// EvidenceCoverage means the anchored line range is exercised by the suite.
	EvidenceCoverage EvidenceType = "coverage"
	// EvidenceSpec is a signed spec acceptance criterion, by id.
	EvidenceSpec EvidenceType = "spec"
)

func (e EvidenceType) Valid() bool {
	switch e {
	case EvidenceTest, EvidenceTypecheck, EvidencePolicy, EvidenceCoverage, EvidenceSpec:
		return true
	}
	return false
}

func EvidenceTypes() []EvidenceType {
	return []EvidenceType{EvidenceTest, EvidenceTypecheck, EvidencePolicy, EvidenceCoverage, EvidenceSpec}
}

// EvidenceRef names a check the agent asserts covers a claim. Ref is a test node
// id, rule name, criterion id, and so on.
type EvidenceRef struct {
	Type EvidenceType `json:"type"`
	Ref  string       `json:"ref"`
}

type Author struct {
	Role     AuthorRole `json:"role"`
	Harness  string     `json:"harness,omitempty"`
	Model    string     `json:"model,omitempty"`
	Identity string     `json:"identity,omitempty"`
}

// FingerprintLines is the content fingerprint for a span of source.
//
// Whitespace-normalised per line so that a reformat does not silently orphan
// every claim in the file. Deliberately does not normalise identifiers: a rename
// is a real change and should show as drift.
func FingerprintLines(lines []string) string {
	normalised := make([]string, len(lines))
	for i, line := range lines {
		normalised[i] = strings.TrimSpace(line)
	}
	sum := sha256.Sum256([]byte(strings.Join(normalised, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// SplitLines matches Python's str.splitlines closely enough for source files:
// it drops a single trailing newline rather than yielding a final empty string,
// and tolerates CRLF.
func SplitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

// Claim is a single statement bound to a span of source.
type Claim struct {
	SchemaVersion string        `json:"schema_version"`
	ID            string        `json:"id"`
	TS            time.Time     `json:"ts"`
	Kind          ClaimKind     `json:"kind"`
	Text          string        `json:"text"`
	Path          string        `json:"path"`
	StartLine     int           `json:"start_line"`
	EndLine       int           `json:"end_line"`
	Fingerprint   string        `json:"fingerprint"`
	VerifiedBy    []EvidenceRef `json:"verified_by"`
	Author        Author        `json:"author"`
	Session       string        `json:"session,omitempty"`
	Ticket        string        `json:"ticket,omitempty"`
}

func (c Claim) Overlaps(path string, start, end int) bool {
	return c.Path == path && !(c.EndLine < start || c.StartLine > end)
}

type AnchorStatus string

const (
	// AnchorAnchored means the fingerprint was found at the recorded location.
	AnchorAnchored AnchorStatus = "anchored"
	// AnchorRelocated means the fingerprint was found elsewhere in the file;
	// line numbers moved under it.
	AnchorRelocated AnchorStatus = "relocated"
	// AnchorDrifted means the fingerprint was not found. The code the claim was
	// about has since changed, so the claim is not evidence about the delivered
	// changeset.
	AnchorDrifted AnchorStatus = "drifted"
	// AnchorOrphaned means the file no longer exists in the changeset.
	AnchorOrphaned AnchorStatus = "orphaned"
)

type CoverageStatus string

const (
	// CoverageVerified: every asserted check exists in the evidence and passed.
	CoverageVerified CoverageStatus = "verified"
	// CoverageImplicit: no check asserted, but the anchored span is exercised
	// by passing tests.
	CoverageImplicit CoverageStatus = "implicit"
	// CoverageUnverified: a check was asserted but is missing or failed.
	CoverageUnverified CoverageStatus = "unverified"
	// CoverageUncovered: nothing asserted and nothing exercises the span.
	CoverageUncovered CoverageStatus = "uncovered"
)

type ResolvedClaim struct {
	Claim          Claim          `json:"claim"`
	Anchor         AnchorStatus   `json:"anchor"`
	AnchoredStart  *int           `json:"anchored_start"`
	AnchoredEnd    *int           `json:"anchored_end"`
	Coverage       CoverageStatus `json:"coverage"`
	CoverageDetail []string       `json:"coverage_detail"`
}

// NeedsHuman is the whole product. Everything else is plumbing.
func (r ResolvedClaim) NeedsHuman() bool {
	if r.Claim.Kind == KindUndetermined {
		return true
	}
	if r.Anchor == AnchorDrifted || r.Anchor == AnchorOrphaned {
		return true
	}
	return r.Coverage == CoverageUnverified || r.Coverage == CoverageUncovered
}

// Hunk is a changed span in the diff against the base ref.
type Hunk struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type SpanVerdict string

const (
	// VerdictUnclaimed is changed code with no claim over it at all. The agent
	// edited and said nothing. Always readable.
	VerdictUnclaimed SpanVerdict = "unclaimed"
	// VerdictUnverified is claimed, but at least one claim over it needs a human.
	VerdictUnverified SpanVerdict = "unverified"
	// VerdictClear is claimed and mechanically covered. Collapse it.
	VerdictClear SpanVerdict = "clear"
)

// ReadSpan is a contiguous run of changed lines sharing a verdict.
//
// Line granularity, not hunk granularity. A hunk frequently contains one span a
// verified claim covers and another that nobody claimed; sending the whole hunk
// to a human because of the second would make "minimum set a human must read"
// false, and the ratio is what the client is buying.
type ReadSpan struct {
	Path      string      `json:"path"`
	StartLine int         `json:"start_line"`
	EndLine   int         `json:"end_line"`
	Verdict   SpanVerdict `json:"verdict"`
	ClaimIDs  []string    `json:"claim_ids"`
}

func (s ReadSpan) Lines() int { return s.EndLine - s.StartLine + 1 }

// Summary is the shape the Python version returns as dict[str, int | float].
// Making it a struct is the one place this port is unambiguously better: the
// ratio that goes in front of a client is now a typed field rather than a string
// key that a typo silently turns into a KeyError at demo time.
type Summary struct {
	ChangedLines       int     `json:"changed_lines"`
	MustReadLines      int     `json:"must_read_lines"`
	ReductionPct       float64 `json:"reduction_pct"`
	Claims             int     `json:"claims"`
	ClaimsNeedingHuman int     `json:"claims_needing_human"`
	Spans              int     `json:"spans"`
	UnclaimedLines     int     `json:"unclaimed_lines"`
}

// ReadingList is the output that matters: the minimum set of lines a human must
// read.
type ReadingList struct {
	Tree   string          `json:"tree"`
	Commit string          `json:"commit"`
	Base   string          `json:"base"`
	Spans  []ReadSpan      `json:"spans"`
	Claims []ResolvedClaim `json:"claims"`
}

func (r ReadingList) MustRead() []ReadSpan {
	out := make([]ReadSpan, 0, len(r.Spans))
	for _, s := range r.Spans {
		if s.Verdict != VerdictClear {
			out = append(out, s)
		}
	}
	return out
}

func (r ReadingList) ChangedLines() int {
	total := 0
	for _, s := range r.Spans {
		total += s.Lines()
	}
	return total
}

func (r ReadingList) MustReadLines() int {
	total := 0
	for _, s := range r.MustRead() {
		total += s.Lines()
	}
	return total
}

func (r ReadingList) Summary() Summary {
	changed := r.ChangedLines()
	mustRead := r.MustReadLines()

	var reduction float64
	if changed > 0 {
		reduction = round1(100 * (1 - float64(mustRead)/float64(changed)))
	}

	needing := 0
	for _, c := range r.Claims {
		if c.NeedsHuman() {
			needing++
		}
	}
	unclaimed := 0
	for _, s := range r.Spans {
		if s.Verdict == VerdictUnclaimed {
			unclaimed += s.Lines()
		}
	}

	return Summary{
		ChangedLines:       changed,
		MustReadLines:      mustRead,
		ReductionPct:       reduction,
		Claims:             len(r.Claims),
		ClaimsNeedingHuman: needing,
		Spans:              len(r.Spans),
		UnclaimedLines:     unclaimed,
	}
}

func round1(f float64) float64 {
	scaled := f * 10
	if scaled < 0 {
		return float64(int(scaled-0.5)) / 10
	}
	return float64(int(scaled+0.5)) / 10
}

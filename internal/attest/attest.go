// Package attest builds the in-toto Statement for a resolved changeset.
//
// The subject is the git tree, not a built artifact. SLSA v1.2 promoted the
// Source track to approved and deliberately leaves source provenance
// attestations undefined, up to whoever implements them — which is the slot this
// predicate occupies. Binding to an image digest would be the wrong anchor: in
// agentic delivery the changeset is the deliverable, and the build is downstream
// of it.
//
// Signing is out of scope here on purpose. `cosign attest-blob` with a CI OIDC
// token does the job with no key custody, and reimplementing it would be a
// liability rather than a feature.
package attest

import (
	"time"

	"trunion.io/pawl/internal/model"
)

type Options struct {
	Repository string
	Ticket     string
	PolicyPack string
}

func BuildStatement(rl model.ReadingList, opts Options) model.Statement {
	claims := make([]model.AttestedClaim, 0, len(rl.Claims))
	for _, rc := range rl.Claims {
		var anchored []int
		if rc.AnchoredStart != nil && rc.AnchoredEnd != nil {
			anchored = []int{*rc.AnchoredStart, *rc.AnchoredEnd}
		}

		asserted := make([]model.AssertedCheck, 0, len(rc.Claim.VerifiedBy))
		for _, e := range rc.Claim.VerifiedBy {
			asserted = append(asserted, model.AssertedCheck{Type: e.Type, Ref: e.Ref})
		}

		claims = append(claims, model.AttestedClaim{
			ID:             rc.Claim.ID,
			Kind:           rc.Claim.Kind,
			Text:           rc.Claim.Text,
			Path:           rc.Claim.Path,
			RecordedRange:  []int{rc.Claim.StartLine, rc.Claim.EndLine},
			AnchoredRange:  anchored,
			Anchor:         rc.Anchor,
			Coverage:       rc.Coverage,
			CoverageDetail: rc.CoverageDetail,
			Asserted:       asserted,
			AuthorRole:     rc.Claim.Author.Role,
			Harness:        optional(rc.Claim.Author.Harness),
			Model:          optional(rc.Claim.Author.Model),
			RecordedAt:     rc.Claim.TS.Format(time.RFC3339Nano),
			NeedsHuman:     rc.NeedsHuman(),
		})
	}

	readingList := make([]model.AttestedSpan, 0)
	for _, h := range rl.MustRead() {
		readingList = append(readingList, model.AttestedSpan{
			Path:    h.Path,
			Range:   []int{h.StartLine, h.EndLine},
			Verdict: h.Verdict,
			Claims:  h.ClaimIDs,
		})
	}

	name := opts.Repository
	if name == "" {
		name = "git-tree"
	}

	return model.Statement{
		Type:          "https://in-toto.io/Statement/v1",
		PredicateType: model.ClaimPredicateType,
		Subject: []model.Subject{{
			Name: name,
			Digest: map[string]string{
				"gitTree":   rl.Tree,
				"gitCommit": rl.Commit,
			},
		}},
		Predicate: model.Predicate{
			SchemaVersion: model.SchemaVersion,
			GeneratedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			Base:          rl.Base,
			Commit:        rl.Commit,
			Ticket:        optional(opts.Ticket),
			PolicyPack:    optional(opts.PolicyPack),
			Summary:       rl.Summary(),
			Claims:        claims,
			ReadingList:   readingList,
			RoleBreakdown: roleBreakdown(rl),
		},
	}
}

// optional maps "" to a JSON null, matching the Python's Optional[str] fields.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func roleBreakdown(rl model.ReadingList) map[string]model.RoleTallyOut {
	out := map[string]model.RoleTallyOut{}
	for _, rc := range rl.Claims {
		role := string(rc.Claim.Author.Role)
		bucket := out[role]
		bucket.Claims++
		if rc.NeedsHuman() {
			bucket.NeedsHuman++
		}
		out[role] = bucket
	}
	return out
}

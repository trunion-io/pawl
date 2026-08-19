// Package claimlog is the append-only claim log.
//
// JSONL in the working tree at .pawl/claims.jsonl. Deliberately boring: no
// database, no daemon, nothing to operate. The whole kit has to install into a
// client repo in under a day, and anything that needs standing infrastructure
// fails that test.
//
// Append-only is a property we rely on later. A claim that was written and then
// quietly edited to match the final code is not evidence of anything.
package claimlog

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"trunion.io/pawl/internal/model"
)

const (
	ClaimDir  = ".pawl"
	ClaimFile = "claims.jsonl"
)

func LogPath(repo string) string {
	return filepath.Join(repo, ClaimDir, ClaimFile)
}

// Append writes a claim to its own file (PAWL-018). The name is historical —
// the log is still append-only, it just no longer appends to a shared file that
// every branch collides on.
func Append(repo string, claim model.Claim) (model.Claim, error) {
	return claim, writeRecord(claimsDir(repo), claim.ID, claim)
}

// Load reads every claim: per-record files, plus the legacy shared log where a
// repository still has one (AC6).
func Load(repo string) ([]model.Claim, error) {
	claims, err := readRecords[model.Claim](claimsDir(repo))
	if err != nil {
		return nil, err
	}
	legacy, err := readLegacyJSONL[model.Claim](LogPath(repo))
	if err != nil {
		return nil, err
	}
	claims = append(claims, legacy...)

	for i := range claims {
		if claims[i].VerifiedBy == nil {
			claims[i].VerifiedBy = []model.EvidenceRef{}
		}
	}
	sortRecords(claims, func(c model.Claim) (string, string) {
		return c.TS.Format(time.RFC3339Nano), c.ID
	})
	return claims, nil
}

// Options is the Go answer to the Python emitter's keyword arguments. Go has no
// keyword or default arguments, so a nine-parameter call either becomes an
// options struct or an unreadable positional list.
type Options struct {
	Kind       model.ClaimKind
	Text       string
	Path       string
	StartLine  int
	EndLine    int
	VerifiedBy []model.EvidenceRef
	Author     *model.Author
	Session    string
	Ticket     string
	Origin     model.RecordOrigin
	Cost       *model.Cost
}

// Record is the emitter entry point, called from a harness hook at the moment of
// the edit.
//
// Reads the span from the working tree, not from git, because the edit has not
// been committed yet. That is the point.
func Record(repo string, opts Options) (model.Claim, error) {
	target := filepath.Join(repo, opts.Path)
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return model.Claim{}, fmt.Errorf("cannot claim against missing file: %s", opts.Path)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		return model.Claim{}, err
	}
	lines := model.SplitLines(string(b))
	if opts.StartLine > len(lines) {
		return model.Claim{}, fmt.Errorf(
			"%s: start_line %d beyond end of file (%d lines)",
			opts.Path, opts.StartLine, len(lines),
		)
	}
	end := opts.EndLine
	if end > len(lines) {
		end = len(lines)
	}
	span := lines[opts.StartLine-1 : end]

	id, err := newID()
	if err != nil {
		return model.Claim{}, err
	}

	author := model.Author{Role: model.RoleAgent}
	if opts.Author != nil {
		author = *opts.Author
	}
	// A rule may never produce a claim (PAWL-017 AC3). A rule does not know
	// what was assumed, and a fabricated assumption is worse than an absent one.
	origin := opts.Origin
	if origin == "" {
		origin = model.OriginAgent
	}
	if origin == model.OriginRule {
		return model.Claim{}, fmt.Errorf(
			"a claim cannot be produced by a rule: rules may record that there " +
				"was nothing to assume, never what was assumed")
	}
	verifiedBy := opts.VerifiedBy
	if verifiedBy == nil {
		verifiedBy = []model.EvidenceRef{}
	}

	claim := model.Claim{
		SchemaVersion: model.ClaimSchemaVersion,
		ID:            id,
		TS:            time.Now().UTC(),
		Kind:          opts.Kind,
		Text:          opts.Text,
		Path:          opts.Path,
		StartLine:     opts.StartLine,
		EndLine:       end,
		Fingerprint:   model.FingerprintLines(span),
		VerifiedBy:    verifiedBy,
		Author:        author,
		Session:       opts.Session,
		Ticket:        opts.Ticket,
		Origin:        origin,
		Cost:          opts.Cost,
	}
	return Append(repo, claim)
}

func newID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func Append(repo string, claim model.Claim) (model.Claim, error) {
	p := LogPath(repo)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return claim, err
	}
	line, err := json.Marshal(claim)
	if err != nil {
		return claim, err
	}
	// O_APPEND write so concurrent pod members do not interleave partial lines.
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return claim, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return claim, err
	}
	return claim, nil
}

func Load(repo string) ([]model.Claim, error) {
	p := LogPath(repo)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var claims []model.Claim
	for i, raw := range strings.Split(string(b), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var c model.Claim
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, fmt.Errorf("%s:%d: malformed claim: %w", p, i+1, err)
		}
		if c.VerifiedBy == nil {
			c.VerifiedBy = []model.EvidenceRef{}
		}
		claims = append(claims, c)
	}
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
	verifiedBy := opts.VerifiedBy
	if verifiedBy == nil {
		verifiedBy = []model.EvidenceRef{}
	}

	claim := model.Claim{
		SchemaVersion: model.SchemaVersion,
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

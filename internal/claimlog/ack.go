package claimlog

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"trunion.io/pawl/internal/model"
)

// AckFile is a separate file from the claim log, deliberately.
//
// PAWL-008 AC2 requires an acknowledgement never be representable as a claim.
// Two files makes that structural: nothing reading claims.jsonl can encounter an
// acknowledgement by accident, so no future code path can quietly start counting
// one as a claim.
const AckFile = "acknowledgements.jsonl"

func AckPath(repo string) string {
	return filepath.Join(repo, ClaimDir, AckFile)
}

func AppendAck(repo string, ack model.Acknowledgement) (model.Acknowledgement, error) {
	return ack, writeRecord(acksDir(repo), ack.ID, ack)
}

func LoadAcks(repo string) ([]model.Acknowledgement, error) {
	acks, err := readRecords[model.Acknowledgement](acksDir(repo))
	if err != nil {
		return nil, err
	}
	legacy, err := readLegacyJSONL[model.Acknowledgement](AckPath(repo))
	if err != nil {
		return nil, err
	}
	acks = append(acks, legacy...)
	sortRecords(acks, func(a model.Acknowledgement) (string, string) {
		return a.TS.Format(time.RFC3339Nano), a.ID
	})
	return acks, nil
}

// AckOptions carries no text field. See model.Acknowledgement — AC3 bounds the
// cost of enforced accounting by leaving an agent nothing to compose.
type AckOptions struct {
	Path      string
	StartLine int
	EndLine   int
	Author    *model.Author
	Session   string
}

// RecordAck reads the span from the working tree, not from git, for the same
// reason Record does: at the moment of the edit the change is not committed yet.
func RecordAck(repo string, opts AckOptions) (model.Acknowledgement, error) {
	target := filepath.Join(repo, opts.Path)
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return model.Acknowledgement{}, fmt.Errorf("cannot acknowledge a missing file: %s", opts.Path)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		return model.Acknowledgement{}, err
	}
	lines := model.SplitLines(string(b))
	if opts.StartLine > len(lines) {
		return model.Acknowledgement{}, fmt.Errorf(
			"%s: start_line %d beyond end of file (%d lines)",
			opts.Path, opts.StartLine, len(lines),
		)
	}
	end := min(opts.EndLine, len(lines))

	id, err := newID()
	if err != nil {
		return model.Acknowledgement{}, err
	}
	author := model.Author{Role: model.RoleAgent}
	if opts.Author != nil {
		author = *opts.Author
	}

	return AppendAck(repo, model.Acknowledgement{
		SchemaVersion: model.ClaimSchemaVersion,
		ID:            id,
		TS:            time.Now().UTC(),
		Path:          opts.Path,
		StartLine:     opts.StartLine,
		EndLine:       end,
		Fingerprint:   model.FingerprintLines(lines[opts.StartLine-1 : end]),
		Author:        author,
		Session:       opts.Session,
	})
}

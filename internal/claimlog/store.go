package claimlog

// Record storage (PAWL-018).
//
// Each record is its own file, named from its identifier. Identifiers are
// unique, so two branches cannot produce the same filename and git never has to
// merge anything — conflict freedom is a property of the layout rather than of
// a .gitattributes somebody can drop during a migration.
//
// This replaced two shared append-only JSONL files, which conflicted on every
// second merge: every branch appended at the same end of the same file. A merge
// queue made it worse rather than better, and hand-resolving a conflicted
// evidence log is the moment its guarantees are worth least — a human picking
// sides can silently drop records and nothing downstream would know.
//
// The legacy files are still read (AC6). Losing evidence is the one thing this
// component may never do, and repositories mid-adoption have records in them.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// ClaimsSubdir and AcksSubdir hold one file per record.
	ClaimsSubdir = "claims"
	AcksSubdir   = "acks"
)

func claimsDir(repo string) string { return filepath.Join(repo, ClaimDir, ClaimsSubdir) }
func acksDir(repo string) string   { return filepath.Join(repo, ClaimDir, AcksSubdir) }

// writeRecord writes one record to its own file.
//
// O_EXCL rather than O_TRUNC: a record file is written once and never modified
// (AC4). Append-only was the property worth keeping from the JSONL log, and
// write-once holds it more strongly — there is no edit path at all, so an
// amended record cannot be produced by this code, only by someone editing the
// tree, where it shows in a diff as a change to a file that should never see
// one.
func writeRecord(dir, id string, v any) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, id+".json")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("record %s already exists; records are written once and never modified", id)
		}
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// readRecords decodes every *.json in dir. Order from the filesystem is not a
// guarantee, so callers sort (AC5) — nothing may depend on directory order.
func readRecords[T any](dir string) ([]T, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []T
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("%s: malformed record: %w", path, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// readLegacyJSONL reads the pre-PAWL-018 shared log, if it is still there.
func readLegacyJSONL[T any](path string) ([]T, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []T
	for i, raw := range strings.Split(string(b), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var v T
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, fmt.Errorf("%s:%d: malformed record: %w", path, i+1, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// sortRecords orders by timestamp then id, so a reading list built twice from
// the same tree is identical. Filesystem order is not stable across machines and
// nothing user-visible may inherit it.
func sortRecords[T any](rs []T, key func(T) (string, string)) {
	sort.SliceStable(rs, func(i, j int) bool {
		ti, ii := key(rs[i])
		tj, ij := key(rs[j])
		if ti != tj {
			return ti < tj
		}
		return ii < ij
	})
}

// Prune removes the record files named by ids (PAWL-018 AC7).
//
// Records are working state for an unmerged changeset, not a permanent archive:
// the signed attestation embeds every one of them and git history keeps them
// regardless. Pruning after attestation keeps .pawl/ holding in-flight work
// rather than every record ever made.
//
// Legacy JSONL entries are never pruned — removing one line would rewrite a
// shared append-only file, which is exactly the edit AC4 exists to prevent.
func Prune(repo string, ids []string) (removed int, skipped []string, err error) {
	for _, id := range ids {
		found := false
		for _, dir := range []string{claimsDir(repo), acksDir(repo)} {
			path := filepath.Join(dir, id+".json")
			if _, statErr := os.Stat(path); statErr == nil {
				if rmErr := os.Remove(path); rmErr != nil {
					return removed, skipped, rmErr
				}
				removed++
				found = true
			}
		}
		if !found {
			skipped = append(skipped, id)
		}
	}
	return removed, skipped, nil
}

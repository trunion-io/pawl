package calibrate

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"trunion.io/pawl/internal/model"
	"trunion.io/pawl/internal/policy"
)

// Dir holds one file per sample, for the reason PAWL-018 gives: two branches
// recording samples must not conflict on merge.
const Dir = ".pawl/calibration"

func dir(repo string) string { return filepath.Join(repo, Dir) }

// Selected decides whether a changeset is sampled, deterministically from its
// tree hash (AC1).
//
// Deriving the decision from the tree rather than from fresh randomness matters
// more than it looks. A rate check that rolled dice each time could be re-run
// until it said no, which would let anyone opt a changeset out of review by
// running the command twice. Seeded from the tree, the answer is fixed: the same
// changeset always gets the same decision, and re-running is idempotent rather
// than another attempt.
func Selected(tree string, rate float64) bool {
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	sum := sha256.Sum256([]byte(tree))
	// Top 32 bits as a uniform fraction.
	n := binary.BigEndian.Uint32(sum[:4])
	return float64(n)/float64(1<<32) < rate
}

// FromReadingList builds a sample from a cleared changeset.
//
// Both `clear` and `acknowledged` spans are included (PAWL-008 AC5): an
// acknowledgement asserts there was nothing to assume, and an assertion nobody
// checks is the thing this product exists to refuse. Waving something through
// that mattered must be able to surface here.
func FromReadingList(
	rl model.ReadingList,
	toolVersion string,
	pol policy.Policy,
	id string,
	now time.Time,
) Sample {
	byClaim := map[string]model.ResolvedClaim{}
	for _, rc := range rl.Claims {
		byClaim[rc.Claim.ID] = rc
	}

	spans := make([]SampledSpan, 0)
	for _, s := range rl.Spans {
		if s.Verdict != model.VerdictClear && s.Verdict != model.VerdictAcknowledged {
			continue
		}
		roles := make([]model.AuthorRole, 0, len(s.ClaimIDs))
		for _, id := range s.ClaimIDs {
			if rc, ok := byClaim[id]; ok {
				roles = append(roles, rc.Claim.Author.Role)
			}
		}
		spans = append(spans, SampledSpan{
			Path:      s.Path,
			StartLine: s.StartLine,
			EndLine:   s.EndLine,
			Verdict:   s.Verdict,
			ClaimIDs:  append([]string(nil), s.ClaimIDs...),
			Roles:     roles,
			Reviewed:  VerdictPending,
		})
	}

	return Sample{
		SchemaVersion: SchemaVersion,
		ID:            id,
		TS:            now.UTC(),
		Tree:          rl.Tree,
		Commit:        rl.Commit,
		Base:          rl.Base,
		ToolVersion:   toolVersion,
		Policy: PolicySnapshot{
			MaxChangedLines:     pol.MaxChangedLines,
			MaxMustReadRatio:    pol.MaxMustReadRatio,
			MaxUnclaimedLines:   pol.MaxUnclaimedLines,
			BlockOnUndetermined: pol.BlockOnUndetermined,
		},
		Spans: spans,
	}
}

func Save(repo string, s Sample) error {
	d := dir(repo)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, s.ID+".json"), append(b, '\n'), 0o644)
}

func Load(repo, id string) (Sample, error) {
	var s Sample
	b, err := os.ReadFile(filepath.Join(dir(repo), id+".json"))
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(b, &s)
	return s, err
}

func LoadAll(repo string) ([]Sample, error) {
	entries, err := os.ReadDir(dir(repo))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Sample
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := Load(repo, strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].TS.Equal(out[j].TS) {
			return out[i].TS.Before(out[j].TS)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

package resolve

// The surfacing cache (PAWL-017 AC14, AC16–AC18).
//
// An agent editing file A, then B, then A again gets told about A twice, the
// second time with nothing new to say. This remembers what has already been
// surfaced for a file so the repeat can be skipped.
//
// Three properties keep it safe:
//
//   - It is machine-local and never committed (AC16). It is per-clone working
//     state, not part of the changeset.
//   - Deleting it changes no reading list, verdict or gate outcome (AC17). It
//     decides only *when* something is said, never what pawl concludes —
//     otherwise a file nobody reviews would be deciding what merges.
//   - Every failure path behaves as though nothing had been surfaced (AC18).
//     Failing toward speaking again costs a repeated message; failing the other
//     way costs silence about unaccounted code.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const cacheSubdir = ".cache/surfaced"

func cacheDir(repo string) string {
	return filepath.Join(repo, ".pawl", cacheSubdir)
}

// spanDigest fingerprints a pending set. Sorted on the way in, because the
// digest must not change when the same spans arrive in a different order.
func spanDigest(spans []PendingSpan) string {
	parts := make([]string, 0, len(spans))
	for _, s := range spans {
		parts = append(parts, fmt.Sprintf("%s:%d-%d", s.Path, s.StartLine, s.EndLine))
	}
	sortStrings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func cacheKey(repo, file string) string {
	sum := sha256.Sum256([]byte(file))
	return filepath.Join(cacheDir(repo), hex.EncodeToString(sum[:])[:16])
}

// AlreadyRaised and MarkRaised are the same mechanism under a name that reads
// correctly for the turn-boundary caller, which raises a set rather than
// surfacing a file (PAWL-020 AC5). Sharing the implementation is deliberate:
// two loop guards that could disagree is one more than is safe.
func AlreadyRaised(repo, key string, spans []PendingSpan) bool {
	return AlreadySurfaced(repo, key, spans)
}

func MarkRaised(repo, key string, spans []PendingSpan) { MarkSurfaced(repo, key, spans) }

// AlreadySurfaced reports whether this exact pending set was the last thing
// surfaced for this file.
//
// Any error reads as "not surfaced" (AC18): an unreadable cache must produce a
// repeated message, never silence.
func AlreadySurfaced(repo, file string, spans []PendingSpan) bool {
	b, err := os.ReadFile(cacheKey(repo, file))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == spanDigest(spans)
}

// MarkSurfaced records what was surfaced. Errors are deliberately ignored: a
// cache that cannot be written must not break the caller, it must only cause
// the next message to repeat.
func MarkSurfaced(repo, file string, spans []PendingSpan) {
	dir := cacheDir(repo)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(cacheKey(repo, file), []byte(spanDigest(spans)), 0o644)
}

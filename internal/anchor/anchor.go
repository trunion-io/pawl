// Package anchor re-anchors edit-time claims against the delivered tree.
//
// Line numbers move constantly while an agent works. If claims were bound to
// line numbers alone, most of a session's claims would silently point at the
// wrong code by the time a PR opens, and the trail would look complete while
// being wrong.
//
// So each claim carries a content fingerprint and we relocate it. Where the
// fingerprint cannot be found, we say so rather than guessing: a drifted claim
// is reported as drift and sends its hunks to the reading list. Failing loud
// here is the difference between an assumption trail and a ritual.
package anchor

import (
	"trunion.io/pawl/internal/gitutil"
	"trunion.io/pawl/internal/model"
)

func spanMatches(lines []string, start, end int, fingerprint string) bool {
	if start < 1 || end > len(lines) || start > end {
		return false
	}
	return model.FingerprintLines(lines[start-1:end]) == fingerprint
}

// relocate scans the file for a span of the same length with the same
// fingerprint. O(file length) per claim: fine at PR scale, wrong for a
// monorepo-wide sweep, and unchanged from the Python for that reason — this port
// is a like-for-like comparison, not an optimisation pass.
func relocate(lines []string, length int, fingerprint string) (int, int, bool) {
	if length <= 0 || length > len(lines) {
		return 0, 0, false
	}
	for start := 1; start <= len(lines)-length+1; start++ {
		end := start + length - 1
		if model.FingerprintLines(lines[start-1:end]) == fingerprint {
			return start, end, true
		}
	}
	return 0, 0, false
}

// Resolve reports where a claim's fingerprint lands in the delivered tree.
func Resolve(repo string, claim model.Claim, rev string) (model.AnchorStatus, *int, *int) {
	return ResolveSpan(repo, claim.Path, claim.StartLine, claim.EndLine, claim.Fingerprint, rev)
}

// ResolveAck does the same for an acknowledgement.
//
// Deliberately the identical mechanism: an acknowledgement that no longer binds
// to delivered code has stopped describing anything, exactly as a drifted claim
// has. C-4 does not care which record type failed to anchor.
func ResolveAck(repo string, ack model.Acknowledgement, rev string) (model.AnchorStatus, *int, *int) {
	return ResolveSpan(repo, ack.Path, ack.StartLine, ack.EndLine, ack.Fingerprint, rev)
}

// ResolveSpan locates a fingerprinted span in the delivered tree.
func ResolveSpan(repo, path string, startLine, endLine int, fingerprint, rev string) (model.AnchorStatus, *int, *int) {
	lines := gitutil.ReadFileAt(repo, path, rev)
	if lines == nil {
		return model.AnchorOrphaned, nil, nil
	}

	if spanMatches(lines, startLine, endLine, fingerprint) {
		start, end := startLine, endLine
		return model.AnchorAnchored, &start, &end
	}

	length := endLine - startLine + 1
	if start, end, ok := relocate(lines, length, fingerprint); ok {
		return model.AnchorRelocated, &start, &end
	}

	return model.AnchorDrifted, nil, nil
}

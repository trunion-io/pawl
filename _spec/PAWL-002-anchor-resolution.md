# PAWL-002 — Anchor resolution

**Status:** delivered · **Module:** `pawl.anchor`

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (SRE) | *unsigned* |

## Context

Line numbers move constantly while an agent works. Claims bound to line numbers
alone would silently point at the wrong code by the time a PR opens, and the
trail would look complete while being wrong.

## Acceptance criteria

**AC1** — When resolving a claim against the delivered tree, the system shall
first check the recorded line range for a fingerprint match.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestVerifiedClaimCollapsesItsHunk`

**AC2** — If the recorded range does not match, then the system shall scan the
file for a span of equal length with a matching fingerprint and report the claim
as relocated.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestClaimRelocatesWhenCodeMoves`

**AC3** — If no matching span exists, then the system shall report drift and
mark the claim unverified irrespective of any asserted check.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestDriftedClaimIsReportedNotSilentlyKept`

**AC4** — If the claimed file is absent from the tree, then the system shall
report the claim as orphaned.
`checkable: yes` → `anchor.resolve` returns `ORPHANED`

**AC5** — The system shall normalise leading and trailing whitespace per line
when computing a fingerprint, and shall not normalise identifiers.
`checkable: partially` — behaviour is implemented; the rename-shows-as-drift
consequence is intended and untested.

## Non-functional

- **Complexity** — relocation is O(file length) per claim. Acceptable at PR
  scale, wrong for a monorepo-wide sweep. **Known gap.**

## Out of scope

- Rename-aware relocation. Large refactors will generate drift noise until this
  exists.

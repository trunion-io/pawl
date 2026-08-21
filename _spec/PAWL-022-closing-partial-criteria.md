# PAWL-022 — Closing the partially-checkable criteria

**Status:** RETIRED — its eight criteria were folded back into the specs they
referenced, once immutability moved to signature and those specs became
editable. The tests it commissioned all still exist and run; only the
indirection is gone. Kept as the record of why the tests were written.
**Module:** `internal/e2e`
**Extends:** [PAWL-003](./PAWL-003-coverage-resolution.md),
[PAWL-004](./PAWL-004-reading-list.md),
[PAWL-006](./PAWL-006-policy-gate.md) — none of them signed.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Why this is a spec and not an edit

Eight criteria across three delivered specs are marked `checkable: partially`
with no test behind them. Writing the tests does not change what those criteria
require, but it does change the `checkable` field — and that field is described
in this directory as load-bearing, because it decides what is a permanent tax on
human attention.

Delivered specs are immutable. So this records the checks rather than editing
them in place, and the pair is the contract.

> **Superseded by the rule change.** Immutability now attaches on signature
> rather than on `delivered`, and no spec here is signed — so the three specs
> this works around are ordinary prose and their `partially` fields can be
> repointed in place. This spec stays as the record of what was done; retiring
> the indirection is separate work.

## Context

`checkable: partially` is where a criterion goes to be forgotten. Two of these
were found in a live demo rather than by the suite, which is recorded in
PAWL-004 itself and is the wrong way round: a criterion nobody checks is
indistinguishable from one nobody implemented until a client finds it.

One of them is also mis-scoped. PAWL-003 AC5 says it is "untested pending
PAWL-009 (spec tool)", but the spec tool is needed to *produce* signed specs in
practice, not to test the code that *reads* one. A hand-written attestation
exercises the loader today.

## Acceptance criteria

Each criterion below is now mechanically checked. The wording of the original is
unchanged; what follows is where its check lives.

**AC1** — PAWL-004 AC4, that pawl excludes its own claim log from the changeset
it analyses, shall be checked.
`checkable: yes` → `TestClaimLogIsExcludedFromTheChangeset`

**AC2** — PAWL-004 AC5, that blank lines and bare delimiters are excluded while
comments remain reviewable, shall be checked.
`checkable: yes` → `TestNonSemanticLinesAreExcludedButCommentsAreNot`

**AC3** — PAWL-006 AC1, that thresholds are read from `.pawl/policy.toml` and
fall back to defaults where absent, shall be checked.
`checkable: yes` → `TestPolicyIsReadFromTheRepository`

**AC4** — PAWL-006 AC3, that a changeset exceeding the must-read ratio fails,
shall be checked.
`checkable: yes` → `TestGateBlocksOnTheMustReadRatio`

**AC5** — PAWL-006 AC5, that a sensitive path requires a named check and does
not accept implicit coverage, shall be checked.
`checkable: yes` → `TestSensitivePathRefusesImplicitCoverage`

**AC6** — PAWL-006 AC6, that the gate exits non-zero on violation while still
producing evidence, shall be checked.
`checkable: yes` → `TestBlockedGateStillProducesEvidence` — a blocked changeset
that produced no trail would leave the reviewer with the verdict and none of the
reasoning behind it, which is the opposite of the point.

**AC7** — PAWL-003 AC3, that a skipped test is treated as absent rather than
passing, shall be checked.
`checkable: yes` → `TestSkippedTestIsAbsentNotPassing`

**AC8** — PAWL-003 AC5, that only criteria marked checkable in a spec
attestation count as evidence, shall be checked.
`checkable: yes` → `TestOnlyCheckableSpecCriteriaAreEvidence`

## Non-functional

- **Testing what the criterion says, not what the code does.** Each test was
  written from the criterion's wording. Writing it from the implementation would
  produce a test that passes by construction and proves nothing.
- **These are behaviour tests, not coverage.** The aim is that each criterion
  now fails loudly if the behaviour regresses, not that a line count moves.

## Out of scope

- **The stale prose in the delivered specs.** They still name Python modules —
  `gitutil.DEFAULT_EXCLUDES`, `resolve._is_reviewable`, `policy.load_policy`,
  `evidence.load_junit`, `evidence.load_spec`. The port repointed `checkable:`
  references but not these, and the immutability rule did not permit fixing
  them. **Now fixed** — all five sat on the very `checkable:` lines this spec's
  criteria replaced, so retiring the indirection removed them.
- **PAWL-006 AC6's workflow half.** That the attestation step runs before the
  gate step is a property of the CI job in `examples/`, not of pawl, and cannot
  be asserted from inside this repository.

# PAWL-032 — Spec consistency

**Status:** DRAFTED, NOT BUILT · **Module:** `Makefile`, `internal/spec`
**Related:** [PAWL-010](./PAWL-010-operator-documentation.md) (built) — that spec
binds documentation to behaviour. This one binds a spec to the other specs it
refers to. Nothing here supersedes anything.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

PAWL-029 was reviewed five times and rewritten five times. The findings per round
ran 12, 6, 3, 3, 6 — down, flat, and then back up, which is not a document
converging. Two of the round-five defects were created while fixing round three.

Reading them together, they are not twelve unrelated mistakes and then six more.
Five shapes repeat:

| Shape | Roughly | Example |
|---|---|---|
| One fact recorded in several places, changed in one | 8 | the index said nine superseded criteria while the spec said ten |
| Prose asserting more than the criteria establish | 7 | "a new major every few days", from one instance |
| One criterion carrying a decidable half and an undecidable one | 3 | AC5, AC8, AC10 |
| A criterion superseded without tracing what depended on it | 4 | AC3's obligation survived only as AC5's proxy |
| A rule with no stated mechanism | 3 | "newest", "states the bump" |

The first, third and fifth are mechanical. The repository has twice built a test
whose only job is to hold two copies of one fact together —
`TestGoTypesMatchCommitlintConfig` and `TestRCChecksMatchRuleset` — and both
exist because those copies had already drifted. Specs have no such check, and one
supersession set currently lives in a spec's table, that spec's prose, the index,
and a pull request description.

Three rules stated in `CLAUDE.md` are enforced by nothing:

- a new `checkable: partially` must be closed in the same changeset
- a delivered spec is never amended
- nothing lands without a criterion

And an assertion nobody could check turns out to be false in three places at
once: PAWL-024, PAWL-025 and PAWL-027 each describe PAWL-013 as *delivered,
immutable*, while PAWL-013's own status line says `DRAFTED, NOT BUILT`. Every
reader who relied on that was told a spec was frozen when it was not.

## Criteria a machine decides

**AC1** — Where a spec names a criterion in another spec, the system shall verify
that criterion exists.
`checkable: yes` (once built) — a supersession table naming an AC that was
renumbered or never existed reads as a change nobody made.

**AC2** — The system shall verify that a spec's index entry names the same set of
superseded criteria as that spec declares.
`checkable: yes` (once built) — the index is a second copy of that set and drifted
from it within one commit.

**AC3** — Where a spec describes another spec's status, the system shall verify
the description matches that spec's status line.
`checkable: yes` (once built) — three specs assert PAWL-013 is delivered today and
it is not.

**AC4** — The system shall verify that every criterion marked `checkable:
partially` names the check that closes it.
`checkable: yes` (once built) — the rule exists in `CLAUDE.md` and has been broken
once already, by a spec written after it.

**AC5** — The system shall verify that criterion identifiers are unique within a
spec and that every internal reference resolves.
`checkable: yes` (once built)

**AC6** — The system shall verify that every file in `_spec/` has an index entry
and every index entry names a file that exists.
`checkable: yes` (once built)

**AC7** — The system shall run these checks as part of the check suite.
`checkable: yes` (once built) — a consistency check nobody runs is a second copy
of the problem.

## Criteria a reader decides

**AC8** — A criterion shall state one requirement; where part is mechanically
decidable and part is not, they shall be separate criteria.
`checkable: no` — deciding whether a sentence contains two requirements is
reading. Recorded as unchecked rather than `partially`, because a partial with no
check is the tax AC4 exists to stop.

> This is the shape that produced AC5, AC8 and AC10 in PAWL-029: each bundled a
> trigger a machine can evaluate with a judgement it cannot, under one
> `checkable: yes`. Split, each half can be labelled honestly. Bundled, the
> criterion claims verification it does not have.

**AC9** — A spec's prose shall not assert more than its criteria and its recorded
claims establish.
`checkable: no` — and this one is worth naming precisely because it cannot be
automated. Four of round one's findings were places where the claims recorded
against the draft already said what the prose did not: the claim was honest and
the sentence above it was not. The accounting was doing its job and nobody read
it back.

## Non-functional

- **This does not make specs correct.** It makes a class of them mechanically
  wrong-detectable, which is a smaller claim. Consequence blindness and
  overclaiming both need a reader, and two of the five shapes above are untouched
  by everything in this spec.
- **The checks must be cheap enough to run every time.** `make check` is seven and
  a half seconds cold; a spec linter that doubles that will be moved to a nightly
  job and stop being a gate.
- **Failing must be the normal outcome of a real mistake.** A linter that has
  never failed is indistinguishable from one that cannot, which is the fault
  PAWL-029 AC10 shipped and a review caught.

## Out of scope

- **Spec content, style or length.** Nothing here reads a criterion for sense.
- **Enforcing "spec first".** Whether a change has a criterion behind it is not
  decidable from the tree; the hook and review carry it.
- **Correcting the three specs that misdescribe PAWL-013's status.** AC3 makes
  them detectable; fixing them is a separate change, and PAWL-024, PAWL-025 and
  PAWL-027 are built rather than delivered, so amending them is permitted.

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

| Shape | Roughly | Example | Machine-checkable |
|---|---|---|---|
| One fact recorded in several places, changed in one | 8 | the index said nine superseded criteria while the spec said ten | yes |
| Prose asserting more than the criteria establish | 7 | "a new major every few days", from one instance | no |
| One criterion carrying a decidable half and an undecidable one | 3 | PAWL-029 AC5, AC8, AC10 | no |
| A criterion superseded without tracing what depended on it | 4 | AC3's obligation survived only as AC5's proxy | no |
| A rule with no stated mechanism | 3 | "newest", "states the bump" | partly |

Only the first is fully mechanical, and part of the fifth. Deciding whether a
criterion bundles two requirements is reading, which AC8 says plainly — an
earlier draft of this table called that shape mechanical and contradicted its own
criterion two pages later. The repository has twice built a test
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

## The declarations these checks read

A check comparing two copies of a fact needs to know which text carries it.
An earlier draft specified the comparisons and not the grammar, so an
implementation could not have told a supersession table from a sentence about
one. Three forms are fixed here, and nothing outside them is read.

**A supersession table** — a Markdown table immediately under a line reading
exactly `**Supersedes by reference:**`, whose first column holds entries of the
form `` PAWL-0NN **ACn** `` or `` PAWL-0NN **ACn**, **ACm** ``. Prose mentioning a
superseded criterion is not a declaration.

**A status annotation** — a parenthesis immediately after a spec link, holding
one of `drafted`, `built` or `delivered`, optionally followed by `, immutable`.

**A status line** — the `**Status:**` line of a spec, whose status is the text up
to the first `·`, matched case-insensitively against an annotation after
mapping `DRAFTED, NOT BUILT` to `drafted`.

## Criteria a machine decides

**AC1** — Where a supersession table names a criterion in another spec, the
system shall verify that criterion exists in that spec.
`checkable: yes` (once built)

**AC2** — Where a supersession table names a spec that is not present, the system
shall report the reference as unresolvable and fail.
`checkable: yes` (once built) — silently passing a reference to something absent
would make the check weakest exactly when a spec is mid-flight.
>
> An earlier draft claimed this spec was itself that case, because it cites
> PAWL-029 from an unmerged branch, and concluded PAWL-029 must land first. That
> was wrong and wrong in a way the grammar above already answered: AC1 and AC2
> read supersession tables, this spec declares none, and "prose mentioning a
> superseded criterion is not a declaration" is a sentence in this document. The
> conclusion was drawn from the criteria without being checked against them.

**AC3** — The system shall verify that a spec's index entry names the same set of
superseded criteria as that spec's supersession table.
`checkable: yes` (once built)

**AC4** — The system shall verify that a status annotation matches the status line
of the spec it annotates.
`checkable: yes` (once built) — four specs fail this today. PAWL-024, PAWL-025 and
PAWL-027 call PAWL-013 delivered, and PAWL-028 calls PAWL-019 delivered; both
targets say `DRAFTED, NOT BUILT`.

**AC5** — The system shall verify that a criterion introduced by a changeset and
marked `checkable: partially` names the check that closes it, or names the spec
that closes it.
`checkable: yes` (once built) — scoped to criteria the changeset introduces, and
allowing an external closure, because neither is optional. `PAWL-001 AC2` and
others are `partially` with their closures recorded in PAWL-022, which exists
precisely because a delivered spec cannot be edited to record its own. A global
rule would demand rewriting immutable specs to satisfy a linter.

**AC6** — The system shall verify that criterion identifiers are unique within a
spec.
`checkable: yes` (once built)

**AC7** — The system shall verify that a criterion reference within a spec
resolves to a criterion in that spec.
`checkable: yes` (once built) — separated from AC6 because they fail
independently and AC10 asks for one requirement per criterion. The earlier draft
bundled them, in the spec that introduces the rule against bundling.

**AC8** — The system shall verify that every numbered spec file has an index entry
and that every index entry names a file that exists.
`checkable: yes` (once built) — numbered files only. `README.md` is the index and
`constitution.md` is not a numbered spec; a rule over every file in `_spec/` would
fail on both and demand the index index itself.

**AC9** — The system shall run these checks as part of the check suite.
`checkable: yes` (once built) — a consistency check nobody runs is a second copy
of the problem.

## Criteria a reader decides

**AC10** — A criterion shall state one requirement; where part is mechanically
decidable and part is not, they shall be separate criteria.
`checkable: no` — deciding whether a sentence contains two requirements is
reading. Recorded as unchecked rather than `partially`, because a partial with no
check is the tax AC4 exists to stop.

> This is the shape that produced AC5, AC8 and AC10 in PAWL-029 — and AC6 here,
> which bundled identifier uniqueness with reference resolution until a review
> pointed out that the spec introducing this rule had broken it. Each bundled a
> trigger a machine can evaluate with a judgement it cannot, under one
> `checkable: yes`. Split, each half can be labelled honestly. Bundled, the
> criterion claims verification it does not have.

**AC11** — A spec's prose shall not contradict the recorded state of a claim
covering the same span.
`checkable: no` — and the wording matters. An earlier draft said prose must not
assert more than "its criteria and its recorded claims establish", which makes a
claim into support for the sentence above it. A claim establishes nothing by
existing: C-1 requires named evidence that was found and passed, and a claim may
be `undetermined`, which records that nothing was established at all. Treating an
unverified claim as backing for stronger prose is the failure this product
refuses, written into the spec meant to catch it.

> What can be read for is contradiction. Four of round one's findings on PAWL-029
> were places where a claim said "not measured" and the paragraph above it stated
> a rate. The accounting was doing its job; nobody read it back.

## Non-functional

- **This does not make specs correct.** It makes one shape mechanically
  wrong-detectable, and part of another. AC10 and AC11 address bundling and
  overclaiming as things a reader decides, so those are not untouched — what is
  true is that no machine check here covers them. Consequence blindness is the
  one shape nothing in this spec addresses at all.
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
- **Correcting the four specs that misdescribe another spec's status.** AC4 makes
  them detectable. PAWL-024, PAWL-025 and PAWL-027 call PAWL-013 delivered;
  PAWL-028 calls PAWL-019 delivered; both targets say `DRAFTED, NOT BUILT`. All
  four are built rather than delivered, so amending them is permitted, and it is
  a separate change.

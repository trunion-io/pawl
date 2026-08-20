# PAWL-034 — `pawl render`

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/render`, `internal/cli`
**Related:** [PAWL-004](./PAWL-004-reading-list.md) (delivered),
[PAWL-006](./PAWL-006-policy-gate.md) (delivered),
[PAWL-010](./PAWL-010-documentation.md) (built),
[PAWL-033](./PAWL-033-contradicted-claims.md) (drafted).
Nothing here supersedes anything.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

A changeset needs a human-facing surface — a pull request body, or whatever else
a reviewer reads before merging. The prevailing pattern in agentic tooling is for
the agent to write that summary in prose: what changed, why it is safe, where to
look.

That pattern reintroduces the risk it claims to remove. The summary is written by
the party whose work is under review, it substitutes for reading the diff, and it
has no error rate — prose cannot be sampled, so it cannot be calibrated, so it
accrues nothing.

`pawl render` takes the same slot with the opposite provenance. Every section is
derived from the verdict record and the record is produced mechanically. The
reviewer's "where should I look" section becomes the reading list, which is the
only version of that section with a denominator.

Everything this tool produces today is a reading list and an exit code. Neither
is what a reviewer opens.

## Derivation

**AC1** — `pawl render` shall accept a verdict record as its only input and emit
markdown as its only output.
`checkable: yes` (once built)

**AC2** — The renderer shall not read the working tree, invoke git, open a network
connection, or call a model.
`checkable: yes` (once built) — asserted by rendering successfully in a directory
that is not a repository and contains no project files.

> **AC2 is the whole spec and every other criterion is a consequence.** If the
> renderer can reach the repository, someone will add a section summarising the
> diff, and the trust inversion is gone with nothing in the tests to catch it.
> Constraining the input means a section that cannot be populated from the record
> cannot be written at all.

**AC3** — The renderer module shall import nothing from the git or
repository-access layer.
`checkable: yes` (once built) — over the import graph, which is a different claim
from AC2 and fails independently. It catches a direct dependency; a call reached
indirectly at runtime is AC2's to catch, which is why both exist.

**AC4** — The renderer shall produce byte-identical output for repeated
invocations on one record.
`checkable: yes` (once built)

**AC5** — The renderer shall emit no figure that is not present in the verdict
record, and shall not compute a statistic the verifier did not produce.
`checkable: no` — and this is the requirement that matters, so it is stated
rather than omitted for being unenforceable. AC1, AC2, AC3 and AC6 together make
it hard to violate accidentally: there is no repository to derive a figure from
and a golden file catches an invented one. None of that is a proof, and pretending
otherwise would be the failure this repository refuses.

## Content

**AC6** — The rendered document shall contain the reading list, the claim counts,
the policy evaluation with each threshold's configured and measured value, and a
reference to the attestation subject.
`checkable: yes` (once built) — against a golden file.

**AC7** — Where the record carries contradicted claims, the document shall present
them under their own heading with their contract references.
`checkable: yes` (once built) — PAWL-033.

**AC8** — Where a changeset passes the gate the document shall say so, and where
it fails the document shall name every reason.
`checkable: yes` (once built)

**AC9** — Where the record carries agent-supplied intent, the renderer shall place
it in a section visually separated from all derived content and labelled as
unattested.
`checkable: yes` (once built)

> **Why intent is quarantined rather than excluded.** A reviewer needs to know
> what the change was for, and that is not derivable. Excluding it makes the
> document less useful and pushes the agent to write intent somewhere else,
> unlabelled. Putting it above a rule, in a labelled section, makes the document
> teach the distinction: a reviewer learns in one pull request which half of the
> page carries weight.

**AC10** — Where the record carries no agent-supplied intent, the renderer shall
omit that section rather than emit an empty or placeholder heading.
`checkable: yes` (once built)

**AC11** — Where the verdict record is malformed or incomplete, the renderer shall
fail naming the defect and shall emit no partial document.
`checkable: yes` (once built) — C-3. A half-rendered document read by a reviewer
is worse than none, because it looks complete.

## Non-functional

- **What deliberately has no section.** No system-level narrative, no safety
  assertion, and no "where the reviewer should focus" beyond the reading list.
  Each would require the renderer to know something the record does not contain,
  which is the same as saying each would be invented.
- **AC2 should be structural, not editorial.** A module that cannot import the git
  layer cannot regress into reading the tree; a review convention can.

## Open decisions

**DECISION-3 — verdict record persistence.** AC1 and AC2 presume the record is a
durable artifact with a stable location and format, rather than an in-process
structure handed to the gate.

**It is not durable today.** `pawl verify --json` emits the reading list to
stdout and `--annotations` writes a check-annotations file; there is no verdict
record artifact anyone can point at. Making one is a prerequisite for this spec
and is probably its own unit of work. **Confirm the shape before implementation
is planned.**

## Out of scope

- **Any output format other than markdown.** Templating for other targets can
  follow once the derivation rules have been used in anger.
- **Publishing the document.** Rendering to stdout is the whole job; posting it
  to a forge belongs to the CI integration.
- **Configurability of section order or content.** A configurable renderer is a
  renderer that can be configured to omit the reading list.

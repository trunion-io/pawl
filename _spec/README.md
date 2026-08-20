# pawl/_spec

Specs for [pawl](../README.md), written in the format the spec tool will
eventually generate and consume. Dogfooding: if the format is too heavy to use
on our own work, it is too heavy to hand a client.

Specs live inside the product so each product in the monorepo is self-contained.
That differs from the layout [`PAWL-009`](./PAWL-009-spec-tool.md) AC1 scaffolds
in a client repo (`spec/<project>/` at its root) — deliberately, because a
client repo is usually one project and this one is not. The *format* is what is
being dogfooded here, not the directory.

The `_` prefix keeps this sorted above the source directories, marks it as not
source — never compiled in, never shipped in the binary — and keeps it out of
globs aimed at source trees. See
[the repository AGENTS.md](../../AGENTS.md#adding-a-product).

## Process — two rules, not negotiable

**1. Spec first. Always.**

Nothing lands without a spec that precedes it. Not code, not documentation, not
a schema change, not a build change. If you are about to write something and
there is no criterion it answers to, you are writing the spec — stop and write
it properly.

"This isn't behaviour, so it doesn't need a spec" is not an exemption, it is the
excuse that produced every unsigned reconstructed spec in this directory. The
test is not *is it code*, it is *could this be wrong in a way somebody would
have to catch by reading it*. Documentation fails that test constantly.

**2. Delivered specs are immutable.**

A spec marked `delivered` is never amended, never edited, never "clarified". Its
criteria are the contract that the delivered code was accepted against, and
rewriting them retroactively changes what was agreed after the fact — which is
the same failure as a claim edited to match the final code.

To change or extend delivered behaviour, **write a new spec that references the
delivered one**:

```markdown
**Extends:** PAWL-005 (delivered, immutable)
```

State plainly which criteria of the referenced spec still hold, which the new
work supersedes, and which it leaves alone. The pair is then the contract.

The only permitted edit to a delivered spec is repointing a `checkable:`
reference when a check moves — a rename or a port. The criterion's *text* must
not change. If you find yourself rewording a criterion during mechanical work,
that is a decision and needs a new spec.

## Format

Each spec is one file per unit of work, named `<KEY>-<slug>.md`, where the key is
the tracker issue it came from. The parts that matter:

- **Stakeholders** — named roles who sign. Product owns behaviour; the NFR
  stakeholder (SRE, QA, security) owns the non-functional sections. A spec with
  no NFR signature is a spec where nobody asked what happens under load.
- **Acceptance criteria** — EARS form, one testable claim each, individually
  identified so a claim in the code can cite `spec:PAWL-003-AC2`.
- **`checkable`** — the load-bearing field. Any criterion that cannot become a
  mechanical check is a permanent tax on human attention, forever, on every
  future changeset. Surfacing that during spec review is the whole point of the
  exercise, and it is the conversation no client is currently having.
- **Out of scope** — written down, because agents fill silence.

## EARS patterns

| Pattern | Shape |
|---|---|
| Ubiquitous | The system shall `<response>` |
| Event-driven | When `<trigger>`, the system shall `<response>` |
| State-driven | While `<state>`, the system shall `<response>` |
| Unwanted | If `<condition>`, then the system shall `<response>` |
| Optional | Where `<feature>`, the system shall `<response>` |

## Status

| Spec | Status |
|---|---|
| `constitution.md` | live — binding on all work |
| `PAWL-001-claim-capture.md` | delivered |
| `PAWL-002-anchor-resolution.md` | delivered |
| `PAWL-003-coverage-resolution.md` | delivered |
| `PAWL-004-reading-list.md` | delivered |
| `PAWL-005-attestation.md` | delivered |
| `PAWL-006-policy-gate.md` | delivered |
| `PAWL-007-calibration-sampler.md` | **built** — sampler, two-phase review, report |
| `PAWL-008-harness-hooks.md` | **AC2–AC6 built** (CLI half); AC1, AC7, AC8 await the hook · extends PAWL-001, PAWL-006 |
| `PAWL-009-spec-tool.md` | drafted, not built |
| `PAWL-010-documentation.md` | drafted · **written after the docs it specifies** |
| `PAWL-011-tool-provenance.md` | extends PAWL-005 · **AC1–AC6, AC9 built**; AC7–AC8 await the release workflow |
| `PAWL-012-configuration.md` | drafted, not built · extends PAWL-006 |
| `PAWL-013-versioning-and-release.md` | drafted, not built · **open decision: supported versions** |
| `PAWL-014-escalation-precision.md` | drafted, not built · mirrors PAWL-007 |
| `PAWL-015-decision-capture.md` | drafted, not built · took AC3 from PAWL-008 |
| `PAWL-016-edit-time-accounting-hook.md` | **built** — `pawl pending` + Claude Code hook · implements PAWL-008 AC1 |
| `PAWL-017-deterministic-accounting.md` | **built** — AC13 delivered by PAWL-020 |
| `PAWL-018-record-storage.md` | **built** · extends PAWL-001 · **granularity provisional** |
| `PAWL-019-harness-installation.md` | **built** · `pawl setup claude`, `pawl hook claude-code` · Bash gap moved to PAWL-020 |
| `PAWL-020-turn-boundary-accounting.md` | **built** · turn-boundary binding · **AC1 needs live verification** |

| `PAWL-023-self-management.md` | **built** · `pawl install verify` / `upgrade` |
| `PAWL-021-cli-coverage.md` | **built** · subprocess tests for the CLI seam |
| `PAWL-022-closing-partial-criteria.md` | **built** · extends PAWL-003, PAWL-004, PAWL-006 |
| `PAWL-024-licensing-and-source-availability.md` | **built** · public source, proprietary licence · extends PAWL-013 |
| `PAWL-025-security-posture.md` | **built** · pinning, scanning, fuzzing · extends PAWL-024 |
| `PAWL-026-policy-input-validation.md` | **built** · rejects unusable thresholds · extends PAWL-006 |
| `PAWL-027-contribution-and-release-flow.md` | **built** · conventional commits, rc tags, computed versions · extends PAWL-013 |
| `PAWL-028-agent-skills.md` | **built** · `skills/` · extends PAWL-019 |
| `PAWL-030-review-skill.md` | **built** · `.github/skills/` · extends PAWL-028 |
| `PAWL-031-automated-contributor-environment.md` | **built** · `copilot-setup-steps.yml` · extends PAWL-030 |
| `PAWL-029-versioning-model.md` | **drafted** · supersedes PAWL-013 AC1/AC2/AC5/AC7/AC13 and PAWL-027 AC3/AC4/AC13/AC14 |

Signatures are absent throughout: PAWL-001 to PAWL-006 were written after the
code, which is the wrong order and is itself the argument for the spec tool.
Treat them as reconstructed intent, not as agreed contracts.

PAWL-010 is the same failure, committed knowingly and recently: the `docs/` tree
was written before the spec that describes it, on the reasoning that
documentation is not behaviour. It is recorded here rather than tidied away,
because a repository that cannot admit its own process failures has no business
selling a tool that catches them. PAWL-011 and PAWL-012 were written first, as
everything from here on must be.

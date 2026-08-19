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
| `PAWL-007-calibration-sampler.md` | **drafted, not built — next** |
| `PAWL-008-harness-hooks.md` | drafted, not built |

Signatures are absent throughout: these were written after the code, which is the
wrong order and is itself the argument for the spec tool. Treat them as
reconstructed intent, not as agreed contracts.

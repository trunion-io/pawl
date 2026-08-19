# PAWL-016 — Edit-time accounting hook

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/cli` (`pawl pending`), `hooks/claude-code/`
**Related:** [PAWL-008](./PAWL-008-harness-hooks.md) — implements its AC1 and the
Claude Code half of its AC7, and resolves a contradiction between its AC1 and
AC8.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

PAWL-008's CLI half is built: `pawl claim`, `pawl ack`, the acknowledgement
record, the gate distinction and the ratio. Nothing yet *asks* an agent to use
them, so the measured 85% unclaimed rate stands and the corpus PAWL-007 needs to
sample does not accumulate.

This is the head of the chain to the demo. Claims at volume produce a corpus; a
corpus makes sampling meaningful; sampling produces the numbers Phase 2 item 8
sells. Building the sampler first would deliver a tool with almost nothing to
sample — this repository currently clears 6.9% of its own changed lines.

### The contradiction this resolves

PAWL-008 states both:

> **AC1** — the hook **shall require** every changed span to carry either a claim
> or an acknowledgement.
>
> **AC8** — the hook **shall not block** or perceptibly slow the edit loop.

These cannot both hold literally. A hook that genuinely requires something must
refuse to proceed without it, which is blocking. A hook that never blocks cannot
require anything.

The resolution is that **the hook's value was never enforcement**. Mechanical
enforcement already exists and already works, at the gate: unaccounted spans
never clear, and `max_unclaimed_lines = 0` blocks the merge. Adding a second
enforcement point at edit time buys nothing and costs the edit loop.

What the gate cannot do, and only a hook can, is **supply the span at the moment
of the edit**. An agent told at PR time that 143 lines are unaccounted must
reconstruct what it was thinking, which is the confabulation C-2 exists to
prevent. An agent told *as it finishes an edit* that `src/auth.go:44-58` is
unaccounted still has the reasoning in context, and its answer is evidence rather
than a story.

So: the hook informs, immediately and precisely. The gate enforces, later and
mechanically. AC1's "require" is satisfied by the pair, not by the hook alone.

### The missing primitive

No command today reports unaccounted spans in the **working tree**. `pawl verify`
resolves a committed changeset against a base ref, which is the wrong question
and the wrong moment for a hook that fires mid-edit.

Without it, every harness integration would reimplement diffing and span
resolution in shell. That is how a thin plugin becomes a harness, which the
build/buy position forbids.

## Acceptance criteria

### The primitive

**AC1** — The system shall report the changed spans in the working tree that
carry neither a claim nor an acknowledgement.
`checkable: yes` (once built)

**AC2** — The system shall resolve claims and acknowledgements against the
working tree, not against a committed revision, when reporting pending spans.
`checkable: yes` (once built) — the edit being accounted for is by definition not
committed. This is the same reason `pawl claim` reads the span from the working
tree.

**AC3** — The system shall report pending spans without requiring any evidence
file.
`checkable: yes` (once built) — pending answers *"is this accounted for?"*, not
*"is this verified?"*. Requiring junit to answer it would make the hook depend on
a test run that has not happened yet.

**AC4** — The system shall offer machine-readable output for pending spans.
`checkable: yes` (once built)

**AC5** — The system shall exclude from pending spans the same paths and
non-semantic lines that the reading list excludes.
`checkable: yes` (once built) — a hook that asks an agent to account for a blank
line teaches it to ignore the hook.

### The hook

**AC6** — After a tool call that modifies a file, the hook shall report that
file's pending spans to the agent.
`checkable: partially` — harness integration.

**AC7** — The hook shall not prevent the modifying tool call from completing.
`checkable: yes` (once built) — resolves PAWL-008 AC1/AC8. Enforcement is the
gate's job.

**AC8** — The hook shall supply the changed line range, and shall not require the
agent to determine which lines it changed.
`checkable: partially` — this is the criterion the whole spec exists for. An
agent computing its own diff is reconstructing, and C-2 rules that out.

**AC9** — If pawl is absent, errors, or is slow, the hook shall allow the edit
loop to continue unaffected.
`checkable: yes` (once built) — a tool that can break a client's agent when it
malfunctions will be uninstalled the first time it does, and deserves to be.

**AC10** — The hook shall not suggest `spec:` evidence while PAWL-009 is unbuilt.
`checkable: yes` (once built) — `spec:` cannot resolve, so a claim citing it is
permanently unverified. A hook that suggests it would poison the corpus at
exactly the moment the corpus starts accumulating.

## Non-functional

- **Thin, and one harness first.** Claude Code before the portable Agent Plugins
  packaging. The ergonomics of claim-or-ack on every edit are untested by anyone;
  learning that on one harness is far cheaper than discovering the model is wrong
  after building the portable version. Generalising is PAWL-008 AC7.
- **Repo-scoped configuration.** The hook config belongs in this repository, not
  in a developer's global settings, so that a checkout is sufficient to reproduce
  the behaviour and a reviewer can see it.
- **The plugin must stay a plugin.** All diffing, resolution and filtering lives
  in `pawl`; the hook is a caller. Anything clever in the hook is duplicated work
  per harness, and the build/buy position is explicit that we do not build a
  harness.
- **This is an experiment with an expected finding.** The measurable outcome is
  whether the unclaimed rate falls and where the acknowledgement ratio settles.
  If acknowledgement becomes the path of least resistance for everything, the
  ratio will say so, and that is a finding about the model rather than a bug.

## Out of scope

- **Codex and other harnesses.** PAWL-008 AC7, after this has run on one.
- **Rejected alternatives.** [PAWL-015](./PAWL-015-decision-capture.md) — a
  different trigger entirely, and an edit hook cannot capture a decision that
  produced no edit.
- **Blocking or gating at edit time.** AC7 above, and the gate already does it.
- **Prompting a human.** This addresses the agent edit loop. What a human is
  asked, and when, is a different question with different ergonomics.

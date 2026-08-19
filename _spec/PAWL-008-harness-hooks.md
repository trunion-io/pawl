# PAWL-008 — Harness hooks and edit-time accounting

**Status:** DRAFTED, NOT BUILT — **open question settled**
**Module:** `hooks/` (does not exist), `internal/model`, `internal/claimlog`, `internal/policy`
**Extends:** [PAWL-001](./PAWL-001-claim-capture.md) (delivered, immutable) —
adds a record type alongside the claim; [PAWL-006](./PAWL-006-policy-gate.md)
(delivered, immutable) — the gate gains a distinction it did not have.
**Related:** [PAWL-007](./PAWL-007-calibration-sampler.md) samples what this
produces. [PAWL-015](./PAWL-015-decision-capture.md) takes over rejected
alternatives.

## Context

`pawl claim` currently has to be invoked deliberately. For the trail to be
complete rather than aspirational, emission must be wired into the harness edit
loop so that claiming is the default and silence is the exception.

Ship in Agent Plugins 1.0.0 layout (`plugin.json` bundling SKILL.md and MCP
config), not as a Claude-Code-specific plugin. Agent Skills is portable across
roughly forty products; Agent Plugins shipped 6 August 2026 with Amazon,
Anysphere, GitHub, Microsoft, OpenAI and Vercel behind it. Anthropic is not a
signatory and Claude Code's "plugin" feature is a different, older thing — check
which one any given doc means.

### What the prompted model actually costs, measured

Dogfooding this repository, with a deliberately motivated agent that had just
written the documentation instructing itself to claim:

| Changeset | Unclaimed |
|---|---|
| Dogfooding commit | 202 of 237 lines — **85%** |
| Taxonomy commit | 159 of 194 lines — **82%** |

"Prompted produces gaps" is not a hypothesis. It is roughly 85%.

### The tension this spec exists to remove

`max_unclaimed_lines = 0` is the shipped default, so the gate **blocks** on
exactly the gaps prompted claiming produces. An agent facing that message goes
back and writes claims against a finished diff — which is precisely what **C-2
forbids**, for precisely the reason C-2 gives.

The current arrangement pushes agents toward the failure C-2 exists to prevent.
Neither "prompted" nor "enforced" fixes that by itself; the unit of obligation
has to change.

## The settled model — accounting, not claiming

Not every changed line carries an assumption. A test file that *is* the evidence
for a claim does not need a claim about itself. Requiring one claim per changed
line manufactures noise, and that — not latency — is the real objection to
enforcement.

So every changed span must be **accounted for**, by one of two records:

| Record | Meaning |
|---|---|
| **Claim** | A substantive assumption, undetermined, or constraint |
| **Acknowledgement** | "I changed this and there is nothing here to assume" |

An acknowledgement is **not silence**. It is an assertion, and this codebase
never trusts an assertion — it records it and checks it later. C-3 is intact:
a span with neither record still reaches a human.

The obvious objection is that acknowledgement becomes the default escape hatch.
It is answered by instrumentation rather than prohibition: acknowledged spans
enter the PAWL-007 sample pool, so an over-acknowledging agent surfaces as a
rising false-clear rate attributable by cause. **The escape hatch is safe
because it is measured.**

## Acceptance criteria

**AC1** — When an agent completes an edit, the hook shall require every changed
span to carry either a claim or an acknowledgement.
`checkable: partially` — depends on harness hook capability.

**AC2** — The system shall record an acknowledgement as a record type distinct
from a claim, and shall not represent it as a claim kind.
`checkable: yes` (once built) — a `trivial` claim kind would land in the claim
corpus and in the attestation as a claim, making the trail look better reasoned
than it is. The count of claims is a number shown to clients; it must mean
substantive claims only.

**AC3** — The system shall not require an agent to generate prose to record an
acknowledgement.
`checkable: yes` (once built) — this bounds the real cost of enforcement. At
2.5ms startup the tool is not the bottleneck; the agent composing claim text is,
and that cost is invisible to any timing assertion. An acknowledgement must be a
single call with no text to compose.

**AC4** — The gate shall distinguish a span with no record at all from a span
carrying an acknowledgement, blocking on the former and clearing the latter.
`checkable: yes` (once built)

**AC5** — The system shall include acknowledged spans in the population sampled
by PAWL-007.
`checkable: yes` (once built) — without this, AC2's escape hatch is unmeasured
and the objection stands.

**AC6** — The system shall report the acknowledgement ratio over a window.
`checkable: yes` (once built) — the first-order signal that claiming has
degraded into box-ticking, available long before the sampler has enough data.

**AC7** — The hook shall support Claude Code and Codex at minimum.
`checkable: no` — integration surface, verified by hand.

**AC8** — The hook shall not block or perceptibly slow the edit loop.
`checkable: yes` — timing assertion once built. Note this measures the tool, not
the agent-attention cost that AC3 bounds.

## Non-functional

- **Build order.** The acknowledgement record and the gate distinction (AC2–AC6)
  are buildable and testable in the CLI today, with no harness involved. Do that
  first; the hook work (AC1, AC7, AC8) depends on it and is the part with
  external integration risk.
- **The ratio is the early warning.** AC6 lands before PAWL-007 has a corpus. An
  acknowledgement ratio climbing toward 1.0 says the trail is decaying now,
  without waiting for sampled review.
- **Acknowledgement is not exoneration.** A span can be acknowledged, sampled,
  and found to have needed a claim. That is a `false_clear` like any other.

## Out of scope

- **Rejected alternatives.** Moved to [PAWL-015](./PAWL-015-decision-capture.md).
  They correlate with decisions rather than edits, and an edit-triggered hook
  cannot fire for a decision that produced no edit — which is where the largest
  ones live.
- **Building a harness.** That layer is commoditised — 26+ harnesses,
  meta-harnesses, and a 135k-star open-source entrant inside four days. This is a
  plugin.
- **Retrofitting acknowledgements onto existing changesets.** That is backfill
  against a finished diff, which is the thing C-2 forbids and this spec exists to
  stop.

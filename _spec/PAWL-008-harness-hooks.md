# PAWL-008 — Harness hooks

**Status:** DRAFTED, NOT BUILT · **Module:** `hooks/` (does not exist)

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

## Draft acceptance criteria

**AC1** — When an agent completes an edit to a file, the hook shall prompt for a
claim covering the changed span.
`checkable: partially` — depends on harness hook capability.

**AC2** — The hook shall support Claude Code and Codex at minimum.
`checkable: no` — integration surface, verified by hand.

**AC3** — Where an agent rejects an alternative approach during an edit, the
hook shall capture it as a `rejected_alternative` claim.
`checkable: no` — behavioural, prompt-dependent. **The hardest part of this
spec and the one most likely to fail quietly.**

**AC4** — The hook shall not block or perceptibly slow the edit loop.
`checkable: yes` — timing assertion once built.

## Open question

Whether claims are prompted (agent decides when to claim) or enforced (every
edit must carry one). Prompted produces gaps; enforced produces noise. Likely
answer is enforced with a `trivial` kind that auto-clears, but that risks
becoming the default escape hatch.

## Out of scope

- Building a harness. That layer is commoditised — 26+ harnesses, meta-harnesses,
  and a 135k-star open-source entrant inside four days. This is a plugin.

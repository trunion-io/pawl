# PAWL-028 — Agent skills

**Status:** DRAFTED, NOT BUILT · **Module:** `skills/`
**Extends:** [PAWL-019](./PAWL-019-harness-installation.md) (delivered) — the
hook installs the mechanism. This defines the instruction that tells an agent
what to do with it.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |

## Context

The hook is half a system. It fires on an edit, computes what is unaccounted for,
and puts that in front of the agent — and then relies on the agent already
knowing what a claim is, when one is worth recording, and what distinguishes an
assumption from something it verified. Nothing currently supplies that knowledge.
An agent meeting pawl for the first time sees a message about unclaimed lines and
has no basis for acting on it beyond guessing.

The result is the failure mode C-1 names. An agent that does not know what a
claim is for will write one that asserts rather than records — "this is correct",
"tests pass" — which is worse than writing none, because it produces evidence
shaped like a claim that carries no information and cannot be checked.

The same gap exists for humans in CI. `pawl gate` exits non-zero and prints
violations, and the person reading that output needs to know whether the right
response is to review the reading list, record a missing claim, or change the
policy — and that the third option is legitimate but is the client's call, not a
way to make the build pass.

## Skills

**AC1** — The system shall provide a skill instructing an agent when to record a
claim, what belongs in one, and what does not.
`checkable: yes` (once built)

**AC2** — The skill shall state that a claim records what was assumed or could
not be established, and shall state that a claim asserting correctness is wrong.
`checkable: yes` (once built) — C-1. This is the single most important sentence
in the skill, because the intuitive reading of "record what you did" produces
exactly the assertion C-1 forbids.

**AC3** — The skill shall instruct the agent to record a claim at the time of the
edit rather than at the end of the task.
`checkable: yes` (once built) — C-2. An agent summarising at the end reconstructs
what it must have assumed, which is a different and less reliable artifact than
recording it while the assumption is live.

**AC4** — The skill shall state that having nothing to claim is a legitimate
outcome and shall not encourage manufacturing claims to satisfy the gate.
`checkable: yes` (once built) — the incentive is real and immediate: unclaimed
lines fail the gate, and the cheapest way to clear them is noise. A skill that
does not name this trains the behaviour it was meant to prevent, and C-3 is
violated from the other direction — coverage that is not evidence.

**AC5** — The system shall provide a skill for interpreting a failed gate in CI,
covering each violation the gate can emit.
`checkable: yes` (once built)

**AC6** — The skill shall state that changing the policy to clear a violation is
the client's decision and shall not present it as a remedy for a failing build.
`checkable: yes` (once built) — C-5 makes thresholds the client's, which makes
lowering one legitimate and makes lowering one *to go green* a way of deleting
the gate while appearing to keep it.

**AC7** — Each skill shall state the conditions under which it applies, so a
harness can select it without loading its body.
`checkable: yes` (once built)

**AC8** — The skills shall not duplicate the operator reference, and shall link
to it for command and flag detail.
`checkable: partially` — PAWL-010 AC3 already requires every command and flag to
be documented in `docs/reference.md`. Two documents describing the same flags
drift, and the one an agent reads drifting is worse than the one a human reads
drifting.

## Non-functional

- **A skill is documentation, and PAWL-010 binds it.** Docs for a CLI are an
  output of the tool in the same way the binary is. A skill describing behaviour
  the tool does not have is a defect, not a stale doc.
- **Written for an agent that has never seen pawl.** The reader has no context
  and cannot ask. Anything the skill assumes, it does not convey.
- **Short enough to be loaded on every relevant turn.** The hook was reduced from
  159 tokens to 73 for this reason; a skill that costs more than the thing it
  explains will be disabled.

## Out of scope

- **Harness-specific packaging** beyond the file layout — installing skills into
  a particular harness's directory belongs with PAWL-019's installer.
- **Skills for pawl's own development.** `AGENTS.md` and `CLAUDE.md` cover that
  and are aimed at a different reader.
- **Teaching an agent to write good code.** The skills describe pawl's mechanism
  and nothing else.

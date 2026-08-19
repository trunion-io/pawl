# PAWL-015 — Decision capture

**Status:** DRAFTED, NOT BUILT · **Module:** `hooks/` (does not exist)
**Related:** [PAWL-008](./PAWL-008-harness-hooks.md) — this took AC3 from it.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

`rejected_alternative` is the claim kind AGENTS.md calls *"the one thing nobody
can reconstruct from the diff afterwards"*. It is also, on current evidence, the
kind least likely to ever be recorded.

This began as PAWL-008 AC3, flagged there as "the hardest part of this spec and
the one most likely to fail quietly". It was moved out because the failure is
**structural, not behavioural**:

> An edit-triggered hook cannot fire for a decision that produced no edit.

Rejected alternatives correlate with **decisions**, not with file writes. Some
produce an edit — *"I used a map rather than a slice here"*. The largest ones
produce none at all — *"I considered restructuring this module and decided
against it"*, *"I nearly added a dependency and did not"*. Those leave no trace
anywhere: not in the diff, not in the file system, not in an edit hook's event
stream. They exist only in the agent's reasoning, for the duration of the
reasoning.

Bundling this with edit hooks hid that they are different mechanisms with
different triggers and different failure modes, and let the harder one shelter
behind the tractable one.

## The trigger problem

There is no reliable signal for "a decision was made". Candidate triggers, none
sufficient alone:

| Trigger | Catches | Misses |
|---|---|---|
| Plan or todo transitions | Deliberate, structured choices | Decisions inside a single step |
| Tool-use events (a search that changed course) | Investigation that closed a path | Choices made without tooling |
| Session end | Anything the agent still remembers | Everything it does not, and it is retrospective |
| Explicit agent invocation | What the agent judged worth recording | Whatever it did not — the PAWL-008 gap, again |

Session-end sweeps are the tempting answer and are close to a C-2 violation: an
agent asked at the end of a session to recall what it rejected is reconstructing,
which is the same confabulation risk C-2 identifies at PR time, merely earlier.
The distinction worth holding is *whether the state that produced the decision
still exists* when the record is written.

## Acceptance criteria

**AC1** — Where a harness exposes plan or task-state transitions, the system
shall prompt for a `rejected_alternative` at each transition.
`checkable: partially` — depends on harness capability, which differs per
harness and is the reason AC4 exists.

**AC2** — The system shall record, with each `rejected_alternative`, whether the
decision produced an edit in the same changeset.
`checkable: yes` (once built) — this is the measurement that tells us whether an
edit-triggered mechanism could ever have been sufficient. If most rejected
alternatives turn out to produce no edit, PAWL-008's original AC3 was
unbuildable and this spec's existence is justified by data rather than argument.

**AC3** — The system shall not prompt for a rejected alternative by asking an
agent to recall decisions made earlier in the session.
`checkable: partially` — enforced by hook design. Recall is reconstruction, and
C-2 exists because reconstruction confabulates. A prompt at the moment of the
decision is evidence; the same prompt an hour later is a story.

**AC4** — The system shall degrade to explicit invocation where a harness exposes
no decision-level events, and shall record which mode produced each claim.
`checkable: yes` (once built) — so that capture rates are comparable only within
a mode. A harness with rich events and one without will produce very different
numbers, and averaging them would hide both.

**AC5** — The system shall report the rejected-alternative capture rate per
harness and per mode.
`checkable: yes` (once built)

## Non-functional

- **This may not be solvable, and that is worth knowing.** If AC2 and AC5 show
  that capture stays negligible across every harness and mode, the honest
  conclusion is that `rejected_alternative` is a kind humans record and agents
  mostly do not — which is a finding about the product, not a bug to grind at.
  Specify the measurement before committing to the mechanism.
- **Prompt-dependent and brittle.** Whatever is built here rests on prompt
  behaviour and will decay silently as models change. Treat capture rate as a
  monitored number, not a delivered feature.
- **Value is asymmetric.** One recorded rejected alternative on a load-bearing
  decision is worth more than a hundred on trivia. Capture *rate* is the wrong
  sole measure of success; PAWL-007's sampler is the only thing that can say
  whether the captured ones mattered.

## Out of scope

- **Edit-time claiming and acknowledgement.** PAWL-008.
- **Inferring rejected alternatives from an agent's transcript or reasoning
  traces.** Tempting and wrong: it is reconstruction by another route, it makes
  the trail depend on retaining transcripts, and C-2 rules it out.
- **A UI for reviewing rejected alternatives.** They surface on the reading list
  like any other claim.

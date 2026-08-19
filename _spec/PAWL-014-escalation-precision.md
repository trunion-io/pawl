# PAWL-014 — Escalation precision

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/calibrate` (does not exist)
**Related:** [PAWL-007](./PAWL-007-calibration-sampler.md) — same machinery,
opposite population.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

PAWL-007 asks *is clearing safe?* by sampling spans pawl cleared. This asks the
mirror question — *is escalating honest?* — by sampling spans pawl sent to a
human.

Both are needed, and roadmap item 8 names both. They fail in opposite
directions and neither number is trustworthy alone:

- A tool that escalates everything has a **perfect false-clear rate** and is
  worthless.
- A tool that clears everything has **perfect escalation precision** and is
  dangerous.

The specific failure this measures is the one that kills adoption quietly.
`must_read_lines / changed_lines` is the commercial number, and every span
escalated for no reason inflates it. Worse than the arithmetic: reviewers learn
that escalations are noise, start skimming, and the reading list becomes the
thing it exists to replace. By the time that is visible in an incident, the
habit is a year old.

This needs a different sampled population from PAWL-007 — escalated spans rather
than cleared ones — and a different sampling rate, because escalations are far
fewer and each already has a human attached.

## Verdict taxonomy

Structured to mirror PAWL-007: a binary outcome, and a cause recorded only when
the outcome is bad. Same reasoning — the headline number stays comparable while
the cause enum is free to grow.

### Axis 1 — was escalating this span warranted?

Recorded **per escalated span**.

| Verdict | Meaning |
|---|---|
| `warranted` | The reviewer found something that needed a human. Escalation was right. |
| `unwarranted` | Nothing here needed a human. The escalation was noise. |

`escalation_precision = warranted / sampled escalated spans`.

### Axis 2 — why was it escalated when it need not have been?

Recorded **per escalated span**, only where axis 1 is `unwarranted`.

| Cause | Meaning | Owner |
|---|---|---|
| `drift_noise` | A rename or reformat drifted the anchor; the code was fine | **pawl** — the known rename-aware gap |
| `unclaimed_trivial` | Genuinely unclaimed, but mechanical enough that reading it bought nothing | Process — claiming habit, or the reviewable-line filter |
| `evidence_absent` | The check existed but was not in the evidence supplied to pawl | Pipeline — a wiring fault, not a code fault |
| `conservative_overlap` | C-8 sent the whole span to a human because one claim over it needed one | **Deliberate** — the cost of a rule we would keep anyway |

`conservative_overlap` exists to be measured rather than eliminated. C-8 is
correct — wrongly collapsing a line costs an unreviewed defect, wrongly expanding
one costs seconds of reading — but nobody has ever quantified what it costs. If
that number turns out to be large, that is a finding about the rule; if it is
small, it is the evidence that defends the rule to a client asking why their
ratio is not better.

`evidence_absent` will be the most common cause early and is not a pawl defect.
Separating it prevents a misconfigured pipeline being read as the tool crying
wolf.

## Acceptance criteria

**AC1** — The system shall select escalated spans for review at a sampling rate
configurable independently of the PAWL-007 rate.
`checkable: yes` (once built) — escalations are fewer and already have a human
attached; one rate for both populations would over-sample one and starve the
other.

**AC2** — When a sampled escalated span is reviewed, the system shall record an
axis 1 verdict, the reviewer identity and the review timestamp.
`checkable: yes` (once built)

**AC3** — Where a span is recorded as `unwarranted`, the system shall record an
axis 2 cause.
`checkable: yes` (once built)

**AC4** — The system shall compute and report escalation precision over a
configurable window.
`checkable: yes` (once built)

**AC5** — The system shall report escalation precision alongside the PAWL-007
false-clear rate, and shall not report either in isolation.
`checkable: yes` (once built) — **the load-bearing criterion.** Each number is
trivially gamed by moving the gate in one direction, and a client shown only one
is being sold a number rather than a measurement. They are reported together or
not at all.

**AC6** — The system shall record the pawl version and the policy in force when
the sampled span was originally escalated.
`checkable: yes` (once built) — mirrors PAWL-007 AC8, same reason.

**AC7** — The system shall permit a reviewer to record that an escalated span was
warranted **and** that the reading list gave an unhelpful reason for it.
`checkable: partially` — a correct escalation with a useless explanation still
erodes trust, and it is invisible to a binary warranted/unwarranted verdict.

## Non-functional

- **Cheaper than PAWL-007 by construction.** An escalated span already had a
  human read it. This asks that human one extra question at the moment they are
  already looking, rather than commissioning a fresh full read.
- **Blinding is not required here.** In PAWL-007 the reviewer must not see the
  claim before judging the span, because anchoring bias would destroy the
  verdict. Here the reviewer has already read the span in the course of their
  normal work and the question is about their own experience of it. Imposing
  PAWL-007's two-phase flow would add ceremony for no gain.
- **Shared storage with PAWL-007.** One corpus, two populations. The C-6
  question — what stores this — is settled once, in PAWL-007.

## Out of scope

- **False-clear rate.** PAWL-007.
- **Acting on the numbers.** Tuning thresholds in response to a poor precision
  figure is the client's decision under C-5, and pawl must not do it
  automatically.
- **Attributing an escalation to a named reviewer's time.** Review-minutes-per
  -changeset is roadmap item 8's other measurement and does not need this
  sampler.

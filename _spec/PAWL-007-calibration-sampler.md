# PAWL-007 — Calibration sampler

**Status:** DRAFTED, NOT BUILT — **taxonomy settled, ready to build**
**Module:** `internal/calibrate` (does not exist)
**Related:** PAWL-014 (escalation precision) samples the opposite population.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

Artefacts nobody reads decay silently. If pawl is doing its job, humans read a
small fraction of hunks — which means there is no ongoing signal on whether the
trail is *accurate*. The trail becomes ritual, agents learn to emit whatever
passes the checker, and it surfaces during an incident.

The sampler forces a full human read on a randomly selected cleared changeset
and records whether the trail was faithful to what the diff actually did.

This produces the only asset in the business a competitor cannot fork. A rival
can copy the schema in an afternoon; they cannot copy 400 sampled changesets
with recorded outcomes. It is also the most credible thing to show an auditor —
not "we have a process" but "here is our measured false-clear rate over the last
200 changesets".

## Verdict taxonomy — settled

The taxonomy records **two orthogonal things**, because the original four-value
proposal (`faithful` / `wrong` / `immaterial` / `irrelevant`) conflated them.

### Axis 1 — was clearing this span correct?

Recorded **per cleared span**.

| Verdict | Meaning |
|---|---|
| `correct` | No human needed to read this span. Clearing was right. |
| `false_clear` | A human needed to read this span and did not. Clearing was wrong. |

This is the headline. `false_clear_rate = false_clear / sampled cleared spans`.

### Axis 2 — why did it clear when it should not have?

Recorded **per (span, claim) pair**, and only where axis 1 is `false_clear`.
Per-pair rather than per-span because a span may be cleared by several claims —
`ReadSpan.ClaimIDs` is a list — and one may be sound while another is not.

| Cause | Meaning | Owner |
|---|---|---|
| `claim_false` | The claim asserts something untrue about the code | Agent |
| `claim_incomplete` | The claim is true but does not address what actually needed review | Agent |
| `anchor_wrong` | The claim is bound to a span it does not describe; drift detection missed it | **pawl defect** |
| `evidence_hollow` | The cited check exists and passes but does not exercise the claim | **Tool blind spot** |

`anchor_wrong` is a bug report about pawl, not a judgement about a claim. Keeping
it on a separate axis stops a tool defect being averaged into a statement about
agent honesty — they have different owners and different fixes.

`evidence_hollow` is the failure mode this Context names and the original
taxonomy had nowhere to record: a claim that is true, complete, correctly
anchored, and cites a passing test that asserts nothing meaningful. pawl checks
that a check exists and passed; it cannot check that it is *meaningful*. The
sampler is the only mechanism in the system that can ever detect this, which
makes it the most commercially important value in the enum.

### Why two axes

The stated fear was that a taxonomy cannot be retrofitted onto data already
collected. Splitting the axes largely removes that risk: **axis 1 is binary and
can never be invalidated by later refinement.** Only the cause enum grows, and
growing it does not disturb the number being sold. A fifth cause discovered in
month six leaves every previously recorded false-clear rate comparable.

## Acceptance criteria

**AC1** — The system shall select cleared changesets for review at a configured
sampling rate.
`checkable: yes` (once built)

**AC2** — When a sampled changeset is reviewed, the system shall record an axis 1
verdict for every cleared span, together with the reviewer identity and the
review timestamp.
`checkable: yes` (once built)

**AC3** — Where a span is recorded as `false_clear`, the system shall record an
axis 2 cause for each claim that cleared that span.
`checkable: yes` (once built)

**AC4** — The system shall compute and report a false-clear rate over a
configurable window.
`checkable: yes` (once built)

**AC5** — The system shall report the false-clear rate broken down by author
role, so the client-capability handover curve is directly observable.
`checkable: yes` (once built)

**AC6** — The system shall report false clears broken down by cause, so that
defects attributable to pawl are separable from failures attributable to an
agent.
`checkable: yes` (once built) — without this the headline rate cannot be acted
on: "improve the agents" and "fix the anchoring" are different projects.

**AC7** — The system shall not present a claim's text to a reviewer before that
reviewer has recorded the axis 1 verdict for its span.
`checkable: yes` (once built) — the review is two-phase. **Phase 1, blind:** the
reviewer reads the span cold and answers whether anything in it needed a human.
**Phase 2, unblinded:** the claims are revealed and the reviewer attributes the
cause. Ordering is enforced by the tool, which is what turns the anchoring-bias
problem from a design aspiration into a mechanical check.

**AC8** — The system shall record the pawl version and the policy in force at the
time a sampled changeset was originally cleared.
`checkable: yes` (once built) — a false-clear rate that mixes verdicts from
different verifiers and different thresholds is not a rate of anything. PAWL-011
put the tool identity in the attestation; this is where it earns its keep.

## Non-functional

- **Retrofit resistance.** Axis 1 is binary and permanent. Axis 2 is expected to
  grow. Any future change to axis 1 invalidates the corpus and needs a new spec
  and a documented cut-over, not an edit here.
- **Storage** — sampling results must survive the engagement and aggregate
  across clients. This is the first component with a legitimate claim to needing
  a store, and therefore the first real test of C-6 (nothing to operate). A file
  in a private repo is the honest starting answer.
- **Reviewer disagreement is data.** `claim_incomplete` requires a judgement
  about what "mattered", and two reviewers will sometimes differ. Sampling the
  same changeset twice occasionally, and recording the disagreement rate, is
  cheaper than pretending the judgement is objective.

## Out of scope

- **Escalation precision.** Whether spans that *did* reach a human were worth
  their attention is the mirror question, needs a different sampled population,
  and is specified separately as PAWL-014. Roadmap item 8 wants both.
- Any inference of quality from the sample without a human verdict. The point of
  the sampler is that a machine cannot do this job.
- **Automatic remediation.** The sampler measures. Acting on a rising
  `anchor_wrong` count is a separate decision by a person.

# PAWL-007 — Calibration sampler

**Status:** DRAFTED, NOT BUILT — this is the next unit of work
**Module:** `pawl.calibrate` (does not exist)

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

## OPEN DECISION — resolve before writing code

The verdict taxonomy cannot be retrofitted onto data already collected. Current
proposal, per cleared span:

- `faithful` — the claim accurately described what the code does
- `wrong` — the claim was false or misleading about the code
- `immaterial` — the claim was true but did not address what mattered in the span
- `irrelevant` — the claim did not describe this span at all (anchoring failure
  that drift detection missed)

`immaterial` and `irrelevant` are doing suspicious work and may collapse into
one, or may need splitting further. **Rich to decide.** Do not start coding
until this is settled.

## Draft acceptance criteria

**AC1** — The system shall select cleared changesets for review at a configured
sampling rate.
`checkable: yes` (once built)

**AC2** — When a sampled changeset is reviewed, the system shall record a verdict
per cleared span, the reviewer identity, and the review timestamp.
`checkable: yes`

**AC3** — The system shall compute and report a false-clear rate over a
configurable window.
`checkable: yes`

**AC4** — The system shall report the false-clear rate broken down by author
role, so the client-capability handover curve is directly observable.
`checkable: yes`

**AC5** — The system shall not allow a reviewer to see the claim text before
recording whether the code does what they believe it does.
`checkable: no` — an ordering constraint in a UI that does not exist. Anchoring
bias makes an unblinded review nearly worthless, so this matters more than it
looks. **Design problem, unsolved.**

## Non-functional

- **Storage** — sampling results must survive the engagement and aggregate
  across clients. This is the first component with a legitimate claim to needing
  a store, and therefore the first real test of C-6 (nothing to operate). A file
  in a private repo is the honest starting answer.

## Out of scope

- Any inference of quality from the sample without a human verdict. The point of
  the sampler is that a machine cannot do this job.

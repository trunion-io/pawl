# Constitution

Immutable principles for every change in this repository. These are not
preferences; a change that violates one is wrong even if it works and even if a
spec asked for it. If a spec conflicts with this file, the spec is wrong.

Written in EARS ubiquitous form so they can be read by an agent and, where the
`checkable` flag says so, enforced mechanically.

> **Amendment, on the Python → Go port.** The `checkable` references below were
> repointed from pytest node ids to the Go suite. **No principle changed**, and
> none should be quietly changed under cover of a port — the pointers moved
> because the checks moved, and every one of them still resolves to a real test
> that still passes. If you find yourself editing the text of a C-rule during
> mechanical work, stop: that is a decision, not a migration.

---

## C-1 — Evidence is produced, never asserted

The system shall treat a claim about verification as unproven until the named
check is found in real tool output and observed to pass.

*Rationale.* An agent asserting `test:test_expiry` is asserting that a check
exists. An asserted-but-missing test is worse than no assertion, because it
looks like rigour. Every path where the tool could take a claim's word for
something is a defect.

**checkable:** yes — `trunion.io/pawl/internal/e2e.TestAssertedTestThatDoesNotExistIsNotCoverage`

---

## C-2 — Claims are captured at edit time

The system shall record claims at the moment of the change, from the state that
produced it, and shall never generate claims by inspecting a finished diff.

*Rationale.* A model asked at PR time to explain its changeset re-reads the diff
and confabulates a plausible rationale. Rejected alternatives are unrecoverable
after the fact — the diff contains no trace of the path not taken.

**checkable:** partially — no automated check exists. Enforced by review.

---

## C-3 — Silence is not coverage

The system shall never clear a changed line that no claim covers.

*Rationale.* The failure mode that destroys trust is an agent editing quietly and
the gate passing. Unclaimed changed code is always readable.

**checkable:** yes — `trunion.io/pawl/internal/e2e.TestUnclaimedChangeNeverClears`

---

## C-4 — Drift fails loud

If a claim's content fingerprint cannot be located in the delivered tree, then
the system shall mark the claim unverified regardless of any check it asserts.

*Rationale.* The moment a stale claim can clear a span on a passing test, the
whole trail is decorative.

**checkable:** yes — `trunion.io/pawl/internal/e2e.TestDriftedClaimIsReportedNotSilentlyKept`

---

## C-5 — The client owns the thresholds

The system shall read every gate threshold from a policy file in the target
repository and shall ship no threshold that cannot be overridden there.

*Rationale.* A supplier who writes both the gate and the bar it clears has built
theatre. We bring the mechanism and a defensible default; the team that owns the
service sets the number.

**checkable:** partially — defaults exist in `internal/policy` and are overridable;
no test asserts that every default is reachable from config.

---

## C-6 — Nothing to operate

The system shall run as a CLI against a git working tree and CI artefacts, and
shall require no database, daemon, network service or hosted component.

*Rationale.* The kit has to install into a client repo and CI runner inside a
day. Anything needing standing infrastructure means engagement one is spent on
platform work instead of shipping changesets, and the engagement dies.

**checkable:** partially — `go.mod` carries no `require` block, so the
dependency list is empty by inspection rather than by audit; no test asserts it
stays that way.

---

## C-7 — Uncertainty outranks green tests

Where an agent records that it could not establish something, the system shall
escalate to a human irrespective of test, coverage or policy outcome.

*Rationale.* "I proceeded anyway" is exactly the signal that mechanical checks
cannot evaluate, and exactly the one worth a human's attention.

**checkable:** yes — `trunion.io/pawl/internal/e2e.TestUndeterminedAlwaysEscalates`

---

## C-8 — Conservative on overlap

Where a changed line is covered by both a clearing claim and a claim needing a
human, the system shall treat the line as needing a human.

*Rationale.* Wrongly collapsing a line costs an unreviewed defect. Wrongly
expanding one costs a few seconds of reading. The asymmetry is not close.

**checkable:** yes — `trunion.io/pawl/internal/e2e.TestPartialCollapseWithinASingleHunk`

---

## C-9 — Tests exercise real repositories

The system's tests shall operate against real git repositories and real tool
output formats, and shall not mock git, diff parsing or evidence files.

*Rationale.* Every defect found so far — hunk granularity, the claim log
auditing itself, blank-line noise — lived in the seam between git's behaviour
and our model of it. A mock would have hidden all three.

**checkable:** partially — enforced by review.

## C-10 — Mechanical resolution and sample verdict are distinct axes

The system shall represent the mechanical resolution of a claimed span and the
calibration sample verdict of that span as two separate fields, and shall derive
neither from the other.

**checkable:** yes — `trunion.io/pawl/internal/e2e.TestSamplingLeavesTheSampleVerdictUnsetAndTheMechanicalVerdictIntact`
and `trunion.io/pawl/internal/e2e.TestRecordingAReviewDoesNotOverwriteTheMechanicalVerdict`

> An earlier draft said "the lifecycle state of a claim", made the rule
> `checkable: yes` with no test named, and then `partially` with no check at all.
> All of those were wrong. `Claim` has no lifecycle-state field and gains none — `contradicted` is a
> kind (PAWL-033) — so the rule would have bound something that does not exist,
> and this file's own convention is that `checkable: yes` names the test, as C-1,
> C-3 and C-4 all do.
>
> Restated over what is already there: `SampledSpan.Verdict` carries the
> mechanical resolution and `SampledSpan.Reviewed` the sample verdict. The rule is
> that those must not collapse, and the named tests hold it — a cleared span
> reviewed as a false clear must keep both values, which is the disagreement the
> false-clear rate is built from and which a single field could not express.
>
> The tests drive `calibrate.FromReadingList` and `Sample.RecordVerdict`, which
> are the two production paths that write these fields and therefore the two
> places a collapse would appear. A first version asserted over a `SampledSpan`
> literal instead; it passed whatever those functions did, including deriving one
> field from the other, and was replaced after review caught it. Both mutations
> were run against the replacement and both fail it.
>
> Marking it `partially — enforced by review` was the second error. The property
> is mechanically testable against fields that already exist, so assigning it to
> review indefinitely would have made it permanent attention debt for want of
> fifteen lines.

**Mechanical span verdict** is what the tool resolved about a changed span
within one changeset — `model.SpanVerdict`, one of `clear`, `acknowledged`,
`unclaimed` or `unverified`. It is a property of the span's relationship to the
evidence cited over it, and it is decided without a human.

> Not "lifecycle state of a claim", which is what an earlier draft called this
> axis and which the note above rejects. Restating it here would have
> reintroduced the term the rule exists to avoid: a `SpanVerdict` is over a span,
> is decided per changeset, and belongs to no claim in particular — a span can be
> `unclaimed` and have no claim at all.

**Sample verdict** is what a human says about that span during calibration,
applied after the fact to build an error rate. It is a property of the span's
relationship to reality.

Every combination that can occur is meaningful. A span may be mechanically
`clear` and sampled `false_clear` — the machine collapsed it and a human says it
should have been read, which is the number the whole sampler exists to produce.
It may be `acknowledged` and sampled `correct` — an agent said there was nothing
to assume and was right.

Not every pairing occurs: `internal/calibrate/store.go` admits only `clear` and
`acknowledged` spans, so an `unverified` span has no sample verdict at all. The
rule is that the two axes must not be collapsed into one another, not that the
product of their values is populated.

The claim's *kind* is a third thing again — what the agent established at edit
time, fixed when the record is written. `contradicted` is a kind for that reason
(PAWL-033), and a contradicted claim is not sampled at all, because its changeset
fails and the sampler selects cleared ones.

Collapsing the axes produces a dataset that cannot answer the question
calibration exists to answer: a binary pass or fail cannot distinguish a claim
that was false from a claim that was true and did not matter, which is the
difference between a defect and a threshold that needs moving. It cannot be
retrofitted onto data already collected, so it binds from before the first
sample is taken.

> PAWL-007 already splits its sampling into two axes — a binary verdict and an
> attributable cause. This rule is about a different pair, and all three are
> represented independently rather than nested: `SampledSpan.Verdict` is the
> mechanical verdict this rule protects, `SampledSpan.Reviewed` is PAWL-007's
> binary verdict, and `SampledSpan.Causes` is PAWL-007's attributable cause, a
> sibling field rather than something held inside `Reviewed`.
>
> An earlier draft said PAWL-007's two axes "both live inside sample verdict".
> That reads as a layering the code does not have, and review caught it: `Causes`
> is its own field, written by `RecordCause` after `Reviewed` is set. This rule
> binds only the first pair. Whether three independent axes carry their weight
> under real sampling is untested, because no samples exist.

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

## C-10 — Lifecycle state and sample verdict are distinct axes

The system shall represent the lifecycle state of a claim and the calibration
sample verdict of a claim as two separate fields, and shall derive neither from
the other.

`checkable: yes` — a record carrying one where the other belongs is detectable.

**Lifecycle state** is what the tool knows about a claim within one changeset:
emitted at edit time or resolved mechanically at verification time. It is a
property of the claim's relationship to its own verification.

**Sample verdict** is what a human says about a claim during calibration,
applied after the fact to build an error rate. It is a property of the claim's
relationship to reality.

Every combination is meaningful. A claim may be mechanically `contradicted` and
sampled `immaterial`; it may be mechanically verified and sampled `wrong`. The
calibration dataset is only useful because it can express all of them.

Collapsing the axes produces a dataset that cannot answer the question
calibration exists to answer: a binary pass or fail cannot distinguish a claim
that was false from a claim that was true and did not matter, which is the
difference between a defect and a threshold that needs moving. It cannot be
retrofitted onto data already collected, so it binds from before the first
sample is taken.

> PAWL-007 already splits its sampling into two axes — a binary verdict and an
> attributable cause. This rule is about a different pair and the two compose:
> PAWL-007's axes both live inside sample verdict, which is one half of this
> one. Whether that layering holds under real sampling is untested, because no
> samples exist.

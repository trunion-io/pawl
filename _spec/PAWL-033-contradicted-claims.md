# PAWL-033 — Contradicted claims

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/model`, `internal/policy`, `internal/cli`
**Extends:** [PAWL-001](./PAWL-001-claim-capture.md) (delivered, immutable) —
changes what a claim can record, not how or when it is recorded.
**Related:** [PAWL-003](./PAWL-003-coverage-resolution.md),
[PAWL-006](./PAWL-006-policy-gate.md), [PAWL-007](./PAWL-007-calibration-sampler.md)
(drafted), and constitution **C-10**.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

An agent working a changeset can reach a state the schema cannot express: it has
established that the contract it was given **cannot be satisfied**. The spec is
wrong, the library does not support the pattern the interface requires, the two
constraints in the ticket are mutually exclusive.

Today that lands in `undetermined` — the same record as "I could not establish
whether this path is reachable". Those are different events with different
destinations. An undetermined claim routes to a human reviewer, who reads and
decides. A contradiction routes to whoever owns the contract, and no amount of
reading the diff resolves it.

This is the reverse loop, and it is the point at which implementation falsifies
design. Most harnesses discard it as a retry.

**Why now rather than later.** This adds a value to a record schema. No
calibration data exists yet — PAWL-007 is drafted and unbuilt, and the sampler
has nothing sampled. Once sampling is live, adding this means backfilling
labelled data or accepting a permanent hole in the dataset. The window is open
and closes on its own.

> **This spec is worth writing now and may not be worth building yet.** Its value
> depends on something consuming the signal, and routing a contradiction to the
> contract's owner is out of scope below. Built alone, it is a state that blocks
> a merge and points nowhere. Written now, it costs a schema decision taken while
> the schema is still free.

## Where the value goes

**AC1** — The claim record shall carry a lifecycle state distinct from its kind,
with the values `verified`, `unverified` and `contradicted`.
`checkable: yes` (once built)

**AC2** — Where a claim is recorded as `contradicted`, the system shall require a
reference to the contract contradicted and a reason, and shall reject the record
as malformed without both.
`checkable: yes` (once built) — a contradiction naming nothing is an assertion
that something is wrong somewhere, which is the shape C-1 refuses.

**AC3** — The system shall not derive, infer or promote `contradicted`, and shall
carry a recorded contradiction through verification unchanged.
`checkable: yes` (once built)

> **Why an agent-asserted state is acceptable here, when it is not elsewhere.**
> The emitter is the agent and the verdict is mechanical everywhere else in this
> tool, and AC2 looks like a violation: contradiction cannot be derived, only the
> agent knows it hit the wall.
>
> It holds because the assertion is monotonic in the safe direction. Asserting
> `contradicted` can only block a merge; there is no input an agent can supply
> that turns a contradiction into a pass. The worst available failure is an agent
> that over-asserts and stalls its own changeset — visible, cheap, and
> self-correcting.

## Effect on the gate

**AC4** — While any claim in a changeset is `contradicted`, the gate shall fail
regardless of every other measurement.
`checkable: yes` (once built)

**AC5** — The gate shall exclude contradicted claims from every threshold in
`.pawl/policy.toml`, in both numerator and denominator.
`checkable: yes` (once built) — a contradiction is not a quantity of risk. Left
in a ratio it would dilute the measure it cannot belong to.

**AC6** — The system shall reject a policy file carrying any tolerance, threshold
or waiver for contradicted claims, naming the offending key.
`checkable: yes` (once built) — and this is the one deliberate exception to C-5.

> C-5 makes thresholds the client's, because a supplier who both writes the code
> and sets the bar it clears is running theatre. This carves one thing out of
> that: a tolerance for contradictions is a tolerance for shipping against a
> contract known to be false. If it were configurable, the first response to an
> inconvenient contradiction would be to raise it. The ratchet does not turn
> backwards.

**AC7** — When the gate fails only because of contradicted claims, it shall exit
with a status distinct from a threshold failure.
`checkable: yes` (once built) — a caller that cannot tell "your contract is
wrong" from "you are over a limit" will retry both.

**AC8** — The reading list shall present contradicted claims under a heading
separate from lines requiring human review, naming the contradicted contract for
each.
`checkable: yes` (once built)

**AC9** — The attestation predicate shall carry a contradicted claim with its
contract reference and reason intact.
`checkable: yes` (once built)

**AC10** — Where a claim carries a lifecycle state the system does not recognise,
the system shall fail rather than treat the claim as absent or unverified.
`checkable: yes` (once built) — C-3. An unreadable record is not an empty one.

**AC11** — The verifier shall leave a recorded `contradicted` state unchanged for
every test-result shape the suite exercises, including a passing test of the same
name.
`checkable: yes` (once built) — scoped to the shapes the suite exercises, which
is what a test can establish. Stating it over *all* possible verifier inputs
would be a criterion no run could satisfy.

## Non-functional

- **Lifecycle state is not sample verdict.** C-10 binds. A contradicted claim
  remains samplable and may be labelled `immaterial` — the contract was wrong and
  it did not matter — and the dataset has to be able to say so.
- **The signal is worth more than the gate behaviour.** Everything above stops a
  merge. None of it moves the contradiction to whoever can act on it, and until
  something does, the value here is that the record exists and is not lost in
  `undetermined`.

## Open decisions

**DECISION-1 — predicate version.** Adding a value to a state enum is additive
for a tolerant consumer and breaking for a strict one. The predicate type URL
`https://trunion.io/attestations/assumption-trail/v0.1` describes the artifact
rather than the tool and must not change for cosmetic reasons — but a state a
consumer cannot interpret is not cosmetic. Either the URL bumps to `v0.2`, or
`v0.1` is documented as requiring consumers to ignore unrecognised state values.
**Rich decides. No code past this point.**

**DECISION-2 — a state, or a fourth kind.** This spec adds a lifecycle state
because that is what the hand-off proposed, and the tree does not obviously agree.
`Claim.Kind` already holds `assumption`, `rejected_alternative` and
`undetermined`, and `undetermined` already means the agent could not establish
something and proceeded. `SpanVerdict` separately holds `unclaimed`,
`unverified`, `clear` and `acknowledged`; there is no lifecycle-state field to
extend.

So `contradicted` is either a new field on the claim, as written, or a fourth
`Kind` beside `undetermined`. The second is a much smaller change and would not
touch the shape of PAWL-001 at all. The first keeps kind and state orthogonal,
which C-10 argues for on the calibration axes and which may or may not be the
same argument here. **Rich decides, and it changes AC1.**

## Out of scope

- **Routing a contradiction to the contract's owner.** This defines the signal
  and its effect on the gate. What consumes it is a separate unit of work, and
  the one that gives this spec its value.
- **Waivers.** A waiver against a threshold and a waiver against a contradiction
  are different objects with different approvers, and only one should be
  grantable by the client. That depends on this state existing.
- **The sampler's treatment of contradicted claims.** PAWL-007.

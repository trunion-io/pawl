# PAWL-033 — Contradicted claims

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/model`, `internal/policy`, `internal/cli`
**Extends:** [PAWL-001](./PAWL-001-claim-capture.md) (delivered, immutable) —
changes what a claim can record, not how or when it is recorded.
**Related:** [PAWL-003](./PAWL-003-coverage-resolution.md),
[PAWL-006](./PAWL-006-policy-gate.md), [PAWL-007](./PAWL-007-calibration-sampler.md)
(built), and constitution **C-10**.

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

**AC1** — The claim record shall accept `contradicted` as a claim kind, alongside
`assumption`, `rejected_alternative`, `undetermined` and `constraint`.
`checkable: yes` (once built)
>
> A kind rather than a new lifecycle field, decided against an earlier draft that
> proposed the field. Three arguments, and the second is the one that settles it.
>
> `Kind` is already the axis for what the agent established at edit time — took
> for granted, considered and rejected, could not establish, believes the code
> must satisfy. "Established that the contract cannot be satisfied" is the same
> shape of thing.
>
> **A kind never changes and a lifecycle state does.** `Claim.Kind` is never
> assigned outside construction anywhere in `internal/`; verification wraps a
> claim in a `ResolvedClaim` and adds anchor and coverage without touching what it
> wraps. AC3 below requires `contradicted` to be immutable after recording, so a
> value that never changes was being proposed as a lifecycle field — the wrong
> axis on its own terms.
>
> And the distinction is epistemic, not procedural: `undetermined` is an unknown,
> `contradicted` is a known. Nesting a known state under a secondary property
> puts the higher-order signal in the lower-order place.

**AC2** — Where a claim is recorded as `contradicted`, the system shall require a
reference to the contract contradicted and a reason, and shall reject the record
as malformed without both.
`checkable: yes` (once built) — a contradiction naming nothing is an assertion
that something is wrong somewhere, which is the shape C-1 refuses.

**AC3** — The system shall not derive, infer or promote `contradicted`, and shall
carry a recorded contradiction through verification unchanged.
>
> This is already how every kind behaves, so the criterion states a property
> rather than adding a mechanism — which is part of why a kind is the right axis.
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

**AC10** — Where a claim carries a kind the system does not recognise, the system
shall fail rather than treat the claim as absent or unverified.
`checkable: yes` (once built) — C-3. An unreadable record is not an empty one.

**AC11** — The verifier shall leave a recorded `contradicted` state unchanged for
every test-result shape the suite exercises, including a passing test of the same
name.
`checkable: yes` (once built) — scoped to the shapes the suite exercises, which
is what a test can establish. Stating it over *all* possible verifier inputs
would be a criterion no run could satisfy.

## Non-functional

- **A contradicted claim is not sampled, and this spec does not change that.** An
  earlier draft said it remained samplable and could be labelled `immaterial`.
  Both halves were wrong. PAWL-007 AC1 selects *cleared* changesets, and AC4 below
  makes a changeset carrying a contradiction fail, so it never enters the sampled
  population. And `immaterial` is from the four-value proposal PAWL-007 explicitly
  rejected for conflating its two axes.
>
  Whether contradictions should be sampled at all is a real question — a
  contradiction that turns out to be the agent misreading the contract is exactly
  the error rate worth knowing — but answering it means extending PAWL-007's
  population and taxonomy, which is that spec's to do and not this one's.
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

**DECISION-2 — resolved: a fourth kind.** An earlier draft proposed a lifecycle
state and recorded this as open. Settled in favour of a kind, for the reasons
under AC1: `Kind` is already the edit-time axis, a kind is immutable where a
lifecycle state is not, and `undetermined` being an unknown while `contradicted`
is a known makes the latter the higher-order signal rather than a nested one.

The consequence is that this spec is much smaller than drafted. It adds an enum
value rather than a field, so the claim schema keeps its shape and the amendment
question the hand-off raised mostly evaporates.

## Out of scope

- **Routing a contradiction to the contract's owner.** This defines the signal
  and its effect on the gate. What consumes it is a separate unit of work, and
  the one that gives this spec its value.
- **Waivers.** A waiver against a threshold and a waiver against a contradiction
  are different objects with different approvers, and only one should be
  grantable by the client. That depends on this state existing.
- **The sampler's treatment of contradicted claims.** PAWL-007.

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

**Why now rather than later.** This adds a value to a record schema. The sampler
is built — `internal/calibrate` ships and `pawl calibrate` is wired in
`internal/cli` — but nothing has been sampled, because that needs sustained real
use rather than more code. The window is therefore about the *corpus*, not the
tool: once verdicts exist, adding a kind means either backfilling labelled data
or accepting a hole in it.

> An earlier draft said PAWL-007 was "drafted and unbuilt", which is false and was
> the load-bearing premise of this paragraph. PAWL-007's own status line still
> reads `DRAFTED, NOT BUILT` and names `internal/calibrate` as not existing, while
> the package ships — the stale header is the likely source of the error, and
> correcting it belongs to that spec.

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
> proposed the field. It is the fifth kind, not the fourth — `constraint` exists
> and is documented, unused only because `spec:` evidence cannot resolve until
> PAWL-009 is built. Three arguments, and the second settles it.
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
`checkable: yes` (once built) — and this is not an exception to C-5, which it
could not be.

> C-5 says the system shall read every gate **threshold** from the policy file and
> ship no threshold that cannot be overridden there. Contradiction blocking is not
> a threshold: there is no quantity, no bar, and nothing to tune. It is an
> invariant, and C-5 does not reach it.
>
> An earlier draft called this "the one deliberate exception to C-5", which a
> drafted spec has no standing to make — the constitution says in its opening
> lines that it outranks every spec and that a spec conflicting with it is the
> thing that is wrong. Framing the same behaviour as an exception rather than as
> out of scope would have been a spec quietly amending the constitution.
>
> The reason it must not be tunable is unchanged: a tolerance for contradictions
> is a tolerance for shipping against a contract known to be false, and if it were
> configurable the first response to an inconvenient contradiction would be to
> raise it. The ratchet does not turn backwards.
>
> **This is a departure from how unknown policy keys are handled today, and the
> departure is the point.** PAWL-026 AC5 makes an unrecognised key a warning
> rather than a rejection, deliberately, so a policy file written for a later pawl
> still loads against an older binary. That reasoning holds for a key the gate
> does not act on. It does not hold for a key whose whole purpose is to weaken a
> refusal: warning and continuing would leave the operator believing they had
> configured a tolerance while the gate ignored it, which is worse than either
> honouring it or refusing it.

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

**AC12** — The system shall raise the predicate `schemaVersion` to `0.3` and the
claim record `schema_version` to `0.2`, and shall leave the predicate type URL
unchanged.
`checkable: yes` (once built) — the mechanism PAWL-011 built for this. AC5 there
raised `schemaVersion` to `0.2` for an additive change while holding the URL
fixed, because the URL describes the artifact rather than the tool; it survived
the `factory-kit` → `pawl` rename on that reasoning and an added enum value is a
smaller thing than a rename.
>
> Both versions move, because both records gain the value. `ClaimSchemaVersion` is
> `0.1` in `internal/model` and the claim log is where a `contradicted` kind is
> written; leaving it there while AC10 hard-fails on an unrecognised kind would
> give an older binary a record it must reject and no version to reject it *by*.
>
> **Raising the version does not by itself protect an older binary, and an
> earlier draft implied it did.** Nothing reads `schema_version` today:
> `claimlog.Load` unmarshals JSON and validates neither the version nor the kind
> (`internal/claimlog/claimlog.go`, `internal/claimlog/store.go`), so a release
> already in the field will parse a `0.2` record, see a `contradicted` kind it
> has no case for, and carry on. The bump is only a label until AC14 gives it
> teeth, which is why AC14 is in this spec rather than a later one — a fail-open
> reader is the C-3 antipattern with a version number written on it.

**AC14** — Where a record carries a `schema_version` the system does not
recognise, the system shall fail rather than parse it, and shall say which
version it found and which it supports.
`checkable: yes` (once built) — the criterion AC12 depends on. A version field no
reader enforces cannot protect anything, and the protection has to ship *before*
records carrying the new version are written, not alongside them.
>
> Ordering matters and is not free. Every binary in the field predates AC14, so
> those readers are fail-open whatever this spec says — the guarantee starts at
> the first release carrying AC14 and covers only readers from that release
> onward. Deployments must take that release before any producer writes `0.2`.
> Stating this is the honest version; claiming the bump protects existing
> installations would not be true.

## Non-functional

- **A contradicted claim is not sampled, and this spec does not change that.** An
  earlier draft said it remained samplable and could be labelled `immaterial`.
  Both halves were wrong. PAWL-007 AC1 selects *cleared* changesets, and AC4 above
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

**AC13** — The system shall not select a changeset carrying a contradicted claim
for calibration sampling.
`checkable: yes` (once built) — the sampler builds a corpus of changesets that
*cleared*, and a changeset the gate refused is not one of those. Without this the
exclusion is an assertion about code that does not implement it.

## Decisions taken here

**DECISION-1 — resolved: raise `schemaVersion`.** The predicate carries
`schemaVersion`, currently `0.2` in `internal/model`, and PAWL-011 AC5 raised it
to that value for exactly this kind of change while holding the type URL fixed.
Adding `contradicted` raises it to `0.3`. The type URL
`https://trunion.io/attestations/assumption-trail/v0.1` does not move, because
PAWL-011 says it describes the artifact rather than the tool and it survived the
`factory-kit` → `pawl` rename on that reasoning.

> An earlier draft framed this as a choice between moving the type URL and
> documenting `v0.1` as requiring tolerant consumers, and a review pointed out
> that both ignore the mechanism already in the tree. There was no decision to
> take: the repository had answered this before the question was asked, and the
> draft proposed changing a URL that PAWL-011 exists to keep still.
>
> What a consumer should do on meeting a `schemaVersion` it does not recognise is
> named as separate in PAWL-011 and stays separate here.

**DECISION-2 — resolved: a fifth kind.** An earlier draft proposed a lifecycle
state and recorded this as open. Settled in favour of a kind, for the reasons
under AC1 — a fifth, not a fourth: `assumption`, `rejected_alternative`,
`undetermined` and `constraint` already exist: `Kind` is already the edit-time axis, a kind is immutable where a
lifecycle state is not, and `undetermined` being an unknown while `contradicted`
is a known makes the latter the higher-order signal rather than a nested one.

The consequence is that this spec is much smaller than drafted, though not as
small as an earlier version of this paragraph claimed. It said the spec "adds an
enum value rather than a field, so the claim schema keeps its shape" — which
AC2, AC8 and AC9 contradict, because all three require a reference to the
contradicted contract to be validated, displayed and carried into the
attestation, and `Claim` has no field that can hold one.

**DECISION-3 — resolved: a dedicated field.** `Claim` gains
`contradicts string \`json:"contradicts,omitempty"\``, required when and only
when `Kind` is `contradicted`. The alternatives were both worse. `Ticket` is a
tracker reference and means something else, so overloading it would make the two
indistinguishable to any consumer. `VerifiedBy` holds `EvidenceRef` values,
which are *checks asserted to verify the claim* — putting the thing a claim
contradicts into the list of things that confirm it inverts the field's meaning
in the one record type where being precise matters most.

So the schema does change shape, by one optional field, and that is a further
reason the `schema_version` bump in AC12 is right rather than ceremonial.

## Out of scope

- **Routing a contradiction to the contract's owner.** This defines the signal
  and its effect on the gate. What consumes it is a separate unit of work, and
  the one that gives this spec its value.
- **Waivers.** A waiver against a threshold and a waiver against a contradiction
  are different objects with different approvers, and only one should be
  grantable by the client. That depends on this state existing.
- **How the sampler treats contradicted claims that reach it by another route,
  and what the corpus taxonomy should call them.** Not whether they are sampled:
  AC13 settles that and makes it checkable. An earlier draft put the whole
  subject out of scope while AC13 specified it, which left the spec with two
  scope boundaries that disagreed. PAWL-007.

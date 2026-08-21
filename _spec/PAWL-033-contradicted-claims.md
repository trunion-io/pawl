# PAWL-033 — Contradicted claims

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/model`, `internal/policy`, `internal/cli`
**Extends:** [PAWL-001](./PAWL-001-claim-capture.md) —
changes what a claim can record, not how or when it is recorded.
**Extends:** [PAWL-006](./PAWL-006-policy-gate.md) — AC5
below changes what its thresholds count.
**Related:** [PAWL-003](./PAWL-003-coverage-resolution.md),
[PAWL-007](./PAWL-007-calibration-sampler.md) (built), and constitution **C-10**.

### What this changes in PAWL-006

Required by [`_spec/README.md`](./README.md) rule 2: a spec that changes
delivered behaviour declares the extension and states criterion by criterion what
happens to it. An earlier draft listed PAWL-006 as merely *Related* while AC5
here altered four of its six criteria, which review caught.

| PAWL-006 | Effect |
|---|---|
| AC1 — read thresholds from `.pawl/policy.toml` | **holds**, plus AC6 here reserves a key pattern the file may not carry |
| AC2 — fail above the line budget | **holds, unchanged** — contradicted lines are counted like any other |
| AC3 — fail above the must-read ratio | **holds, unchanged** — contradicted lines are counted like any other |
| AC4 — fail on unclaimed changed lines | **holds**, and AC5 here settles that a contradicted line is not unclaimed |
| AC5 — sensitive paths need a named check | **holds except** for contradicted claims, which cite no evidence by DECISION-3 and are exempted here |
| AC6 — exit non-zero and emit the decision | **holds**, unchanged |

Nothing above is superseded, and the gate only ever gets stricter. Two of
PAWL-006's criteria are untouched; AC4 gains a clarification that keeps a
contradicted line out of the *unclaimed* count without removing it from anything
else, and AC5 gains a narrow exemption that prevents a second, misleading failure
for one event. AC4 in this spec fails the changeset outright regardless.

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

**AC2a** — Where a claim of any other kind carries a contract reference, the
system shall reject the record as malformed, naming the field and the kind.
`checkable: yes` (once built) — the reverse direction, which AC2 does not cover.
DECISION-3 says the field is valid "when and only when" the kind is
`contradicted`; without this an implementation that accepts `contradicts` on an
`assumption` satisfies every criterion while contradicting the decision that
introduced the field. Review found the gap. A field whose meaning depends on
another field's value has to be checked from both ends or it is not constrained
at all.

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

**AC5** — A contradicted claim shall not reduce any gate measurement. The lines
it covers shall be counted as changed and as requiring a human read, shall not be
counted as unclaimed, and shall not require a named check where they fall on a
sensitive path.
`checkable: yes` (once built) — four assertions over one changeset carrying one
contradicted claim.

| Threshold | Lines covered by a contradicted claim |
|---|---|
| `max_changed_lines` | **counted**, like any other changed line |
| `max_must_read_ratio` | **counted in the numerator and the denominator** |
| `max_unclaimed_lines` | **not** counted as unclaimed |
| `sensitive_paths` | exempt from `sensitive_path_needs_named_check` |
| `block_on_undetermined` | untouched — a different kind, and AC3 forbids promoting between them |

> **This criterion previously said the opposite**, and review was right to refuse
> it. It excluded contradicted lines from the counted thresholds "in both
> numerator and denominator", reasoning that a contradiction is not a quantity of
> risk. That reasoning does not survive PAWL-006's own non-functional note:
> *comprehension has a hard ceiling regardless of trail quality*. A contradicted
> line is still a line a human must read and understand. Whether the trail over it
> is good, bad or self-refuting changes what the reviewer concludes, not how much
> there is to read — so excluding it made the reported changeset size false, and
> made the gate **looser** in exactly the case that should worry a reader most.
>
> The rule is now one sentence: **a contradiction never makes a number smaller.**
> Both exceptions go the other way and neither is a relaxation:
>
> `max_unclaimed_lines` — a contradicted line is not unclaimed. An agent recorded
> something about it, and the most important thing it could record. Counting it as
> unclaimed would fail the changeset twice for one event and make the unclaimed
> count mean two different things.
>
> `sensitive_paths` — a contradicted claim cites no `VerifiedBy` evidence by
> DECISION-3, so the named-check rule would fire and report a *missing check* on
> top of the contradiction. Two failures for one event, and the second one
> misleading about which problem to fix.
>
> None of this decides the outcome: AC4 fails the changeset outright whenever any
> claim is contradicted. AC5 exists so the numbers reported alongside that failure
> are true.

**AC6** — The system shall reject a policy file carrying any key matching
`contradicted*` or `*_contradicted`, naming the offending key and saying that
contradiction is an invariant rather than a threshold.
`checkable: yes` (once built) — and this is not an exception to C-5, which it
could not be.
>
> Stated as a pattern because the first wording — "any tolerance, threshold or
> waiver for contradicted claims" — named no key and no rule for recognising one,
> so no implementation could tell such a key from any other unrecognised one.
> `unknownKeys` in `internal/policy` warns and continues for anything it does not
> know, which is the precise behaviour AC6 exists to prevent: a client writes
> `contradicted_tolerance = 3`, sees a warning scroll past, and believes it took
> effect. A reserved pattern is what makes the difference between rejecting and
> shrugging.

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
reader enforces cannot protect anything.

**AC15** — The system shall accept every schema version it previously wrote, and
shall name the supported set rather than a single value.
`checkable: yes` (once built) — every record on disk today is `0.1`: this
repository's own claim log, and acknowledgements too, since `claimlog/ack.go`
stamps them with the same `model.ClaimSchemaVersion` constant AC12 raises. An
AC14 implementation that recognises only the current version would make its first
release reject the records pawl itself wrote, turning a compatibility guard into
a data-loss event. Fail closed on *unrecognised*, not on *not-latest*.
>
> An earlier version of this note said acknowledgements were a separate case
> "whose version AC12 does not move". That is false — one constant covers both
> record types — and review caught it. The correction does not change what AC15
> requires, but it does change the reason: acknowledgements are not an exception
> needing special handling, they are the same case, and an implementation written
> from the wrong premise might have treated them as one.

**AC16** — The release pipeline shall refuse to publish a pawl release that
writes `schema_version` `0.2` unless the immediately preceding pawl release
already contains the AC14 reader, naming the release that lacks it.
`checkable: yes` (once built) — against pawl's own release history, in the shape
`internal/e2e/tagscript_test.go` already uses: build a repository whose previous
tag lacks the reader and assert publication is refused.
>
> **This is a publication refusal, not a runtime one**, and two earlier drafts
> got that wrong in opposite directions. The first made it a release-ordering
> rule with no check. The second made it "refuse to write `0.2` unless the
> previous released tag contains the reader" — which reads as a check inside
> `pawl claim`, and cannot work: `pawl claim` runs in a *client* repository,
> whose tags are the client's software and say nothing about which pawl release
> shipped. Review caught it. The only place that can see pawl's release history
> is pawl's release pipeline, which is where the refusal belongs.
>
> [`PAWL-013`](./PAWL-013-versioning-and-release.md) owns that pipeline and
> implements this; the criterion is stated here because it is this spec's
> compatibility guarantee, and a criterion in the spec that creates the hazard is
> harder to lose than a note in the spec that owns the machinery.

**This spec ships in two releases, and AC12 belongs to the second.** Review found
that AC12 and AC16 as first written could not both be satisfied: AC12 mandates
writing `0.2`, AC16 refuses that write unless the *previous* tag carries the
reader, and a single release containing both is its own previous-tag-less case.
The spec simultaneously required and forbade the first `0.2` write.

| Release | Contains | Writes |
|---|---|---|
| **1 — reader** | AC10, AC14, AC15, AC16 | still `0.1` |
| **2 — writer** | AC1, AC2, **AC2a**, AC3–AC9, AC11, AC12, AC13 | `0.2` |

AC2a is named explicitly because "AC1–AC9" does not obviously contain it, and a
release shipping AC2 without its reverse-direction check accepts `contradicts` on
kinds where DECISION-3 says it is invalid.

Release 1 teaches every consumer to refuse what it cannot read, and refuses to
emit the new version itself. Release 2 may write `0.2` because release 1 is by
then the previous tag. Nothing in release 1 is useful on its own, which is the
cost of the ordering and is smaller than the alternative: a fail-open reader
meeting a record it silently mishandles.
>
> A first draft of this criterion was written `checkable: partially` on the
> grounds that release ordering is not a property of one checkout, and declared
> itself closed by a refusal in PAWL-013's workflow that nobody had written.
> Both halves were wrong in the way this repository keeps finding: a `partially`
> with the closure asserted rather than built is attention debt with a
> reassurance attached, and it is what PAWL-022 exists to clean up. Reformulating
> the requirement over the tag history — which *is* readable from a checkout —
> makes it an ordinary testable criterion.
>
> This is the criterion that makes AC14 mean anything. AC12 mandates writing
> `0.2` and AC14 mandates the reader that rejects unknown versions; as peer
> criteria of one deliverable, a single release satisfying both does precisely
> what AC14 is for, and without AC16 nothing would detect it.
>
> What this cannot fix: every binary already in the field predates AC14 and is
> fail-open whatever this spec says. The guarantee starts at the first release
> carrying it and covers only readers from that release onward. Claiming the bump
> protects existing installations would not be true.

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

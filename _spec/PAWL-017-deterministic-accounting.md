# PAWL-017 — Deterministic accounting and cost

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/policy`, `internal/model`, `internal/claimlog`, `hooks/`
**Related:** [PAWL-008](./PAWL-008-harness-hooks.md) defined the acknowledgement;
[PAWL-016](./PAWL-016-edit-time-accounting-hook.md) built the hook this makes
cheaper; [PAWL-007](./PAWL-007-calibration-sampler.md) is what keeps it honest.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

Accounting costs tokens, and the cost is paid out of the agent's own budget on
every edit. Measured on the hook as built:

| | bytes | ~tokens | |
|---|---|---|---|
| Variable — the spans that changed | 128 | 32 | the information |
| Boilerplate — instructions | 508 | **127** | **identical every firing** |

Plus the response: an `ack` costs roughly 150 tokens of tool round trip, a
`claim` 250–350 because the agent composes prose first. Over a forty-edit
session, on the order of 11,000 tokens, four fifths of the injection being the
same sentences repeated.

That number matters beyond politeness. A tool whose overhead is felt on every
edit gets switched off, and an accounting habit that is expensive to keep is one
that decays — which is the failure PAWL-007 exists to detect and this spec
exists to avoid causing.

### Where the cost actually is

The cost is dominated by **volume**, and volume is the automatable half:

- An **acknowledgement carries no prose by design** (PAWL-008 AC3). It is a span
  and nothing else. Nothing about producing one requires a language model.
- A **claim's text** cannot be produced by any local tool, ever. It is the
  assumption, and only the thing that made the assumption knows what it was.

So the expensive-per-unit half is the valuable half, and the cheap-per-unit half
is the high-volume half. Automating the second leaves the first untouched.

## Deterministic acknowledgement

**AC1** — Where a changed span matches a configured rule, the system shall record
an acknowledgement for it without involving an agent.
`checkable: yes` (once built)

**AC2** — The system shall apply only rules decidable from the repository
contents alone, without judgement.
`checkable: partially` — the rule *set* is inspectable; that each rule is
genuinely decidable is a review judgement at the point one is added. Candidates
that qualify: path globs (generated code, vendored trees, lockfiles), and
changes whose whitespace-normalised form is unchanged — pawl already normalises
for fingerprints, so a formatting-only diff is provably non-semantic.

**AC3** — The system shall never generate a claim by rule.
`checkable: yes` (once built) — **the boundary this spec turns on.** A rule may
record that there was nothing to assume. A rule may never assert *what* was
assumed, because it does not know, and a fabricated assumption is worse than an
absent one. Acknowledgement is automatable precisely because it asserts nothing.

**AC4** — The system shall include rule-produced acknowledgements in the
PAWL-007 sample pool on the same terms as agent-produced ones.
`checkable: yes` (once built) — this is the entire safety argument. A wrong rule
clears code that needed reading; sampling is what surfaces that, as false clears
attributable to a named rule.

**AC5** — The system shall read acknowledgement rules only from
`.pawl/policy.toml`.
`checkable: yes` (once built) — C-5. Rules decide what escapes human attention,
so they are the client's, and a change to them appears in a diff and gets
reviewed like any other. They must not be settable from the environment, for the
same reason gate thresholds are not (PAWL-012 AC4).

## Record origin

**AC6** — The system shall record, for every claim and acknowledgement, whether
it was produced by an agent, by a human, or by a named rule.
`checkable: yes` (once built) — distinct from `author.role`, which says *who* the
work is attributed to. This says *what mechanism* produced the record.

**AC7** — Where a record was produced by a rule, the system shall record which
rule produced it.
`checkable: yes` (once built) — without it, a rule that turns out to be wrong
cannot be traced to the records it produced, and the corpus cannot be corrected.

**AC8** — The system shall report the share of records produced by rule.
`checkable: yes` (once built) — and this is a **verifiable** proxy for what
accounting costs, because pawl produced those records itself and needs nobody's
word for it.

## Cost provenance

**AC9** — Where a cost is recorded against a record, the system shall require it
to declare what was counted.
`checkable: yes` (once built) — "tokens" alone is mush: composing the claim text,
the hook round trip, and the edit that produced the span are three different
numbers, and a corpus mixing them measures nothing.

**AC10** — The system shall not read any recorded cost when evaluating the gate.
`checkable: yes` (once built) — **load-bearing.** The moment cost influences a
verdict, an agent is paid to claim less, and under-claiming is the failure the
whole tool exists to catch.

**AC11** — The system shall compute and report the cost proxies it can measure
without self-report: claim text length, record counts, and the claim-to-
acknowledgement split.
`checkable: yes` (once built) — a checkable floor under an unverifiable number.

## Surfacing

The hook currently speaks after every edit. An agent working through a task
edits file A, then B, then A again, then C, and gets four injections whose
contents overlap — each one interrupting work that is not finished.

**Eager surfacing does not merely cost tokens; it produces worse records.** A
claim recorded against lines 40–58 while the agent is still editing them will not
match those lines by the time the work settles. The fingerprint fails, the claim
drifts, and C-4 correctly reverts the span to needing a human. Claiming too early
*manufactures* the drift that drift detection exists to report.

So the right moment to surface is when a span has **stopped moving**, not when it
changes. Within a turn that is approximated by the turn ending; nothing available
to a hook can know it exactly.

**AC12** — The hook shall not repeat standing instructions on each firing.
`checkable: yes` (once built) — guidance on how to claim is standing context and
belongs in the agent's instructions once. Removing it cuts the injection roughly
five-fold with no loss of signal.

**AC13** — The system shall accumulate pending spans as edits occur and surface
them once the changed spans have settled, rather than after each edit.
`checkable: partially` — "settled" is approximated by the end of the agent's
turn, which is the only such signal a harness reliably exposes.

**AC14** — The system shall not surface a pending set it has already surfaced
unchanged.
`checkable: yes` (once built)

**AC15** — The system shall report the size of what it injects.
`checkable: yes` (once built) — the overhead is a number somebody will ask about,
and measuring it is how AC12–AC14 stay honest rather than decaying back.

### The cache

**AC16** — The surfacing cache shall be machine-local and shall never be
committed.
`checkable: yes` (once built) — it is per-clone working state, not part of the
changeset. The claim and acknowledgement logs are the opposite and must be
committed; conflating the two is how a repository ends up ignoring its own
evidence.

**AC17** — Deleting the cache shall not change any reading list, verdict, or gate
outcome.
`checkable: yes` (once built) — **the criterion that keeps the cache safe.** It
is an ergonomics optimisation over *when a human or agent is told* something, and
nothing more. The moment a verdict depends on it, a stale or absent cache changes
what merges, and a machine-local file nobody reviews has become load-bearing.

**AC18** — Where the cache is unreadable, corrupt or absent, the system shall
behave as though nothing had been surfaced before.
`checkable: yes` (once built) — failing toward speaking again is the safe
direction: the cost is a repeated message, where the opposite failure is silence
about unaccounted code.

### On C-2

Deferring surfacing to the end of a turn is further from the edit than firing
immediately, and C-2 exists to stop claims being reconstructed after the fact.
Two things make the trade defensible, and they should be revisited if either
stops holding:

- Within a turn the agent still holds the reasoning that produced the edit. C-2's
  target is a model re-reading a finished diff at PR time, with the working
  context long gone — a different situation, not a milder version of this one.
- The alternative is worse on C-2's own terms. A claim recorded mid-edit drifts,
  and a drifted claim is not evidence about the delivered changeset at all.

What must not happen is deferral past the turn. At that point the reasoning is
gone and the record would be reconstruction, which is the thing C-2 forbids
outright.

## Non-functional

- **Determinism is the safety argument, not an implementation preference.** A
  rule that needs judgement is a rule that can be wrong in ways nobody can
  predict from reading it. If a candidate rule cannot be decided from repository
  contents, it is a claim and needs a claimant.
- **A recorded cost is provenance, not evidence.** It sits with `ts`, `harness`
  and `model` — all self-reported, none verified, none consulted by the gate. The
  distinction that admits it: the claim's *text* is a statement about the code
  and must be verifiable; a token count is metadata about the record's
  production. C-1 governs the first and has nothing to say about the second.
- **Rules only ever reduce human attention.** They cannot escalate. So the
  failure mode is one-directional and exactly what the sampler measures, which
  is why AC4 is not optional.
- **No LLM-assisted acknowledgement.** A model deciding "this span needs no
  claim" is making a judgement, and an unrecorded judgement is the thing this
  product exists to surface. Either a rule can decide it deterministically or an
  agent records a claim about it.

## Out of scope

- **Auto-claiming, permanently.** AC3.
- **Cost-based gating.** AC10.
- **Cache-dependent verdicts.** AC17. The cache decides only *when* something is
  said, never *what* pawl concludes.
- **Deferring surfacing beyond the turn that produced the edit.** Past that point
  the reasoning is gone and the record is reconstruction.
- **Rename-aware anchoring.** Renames surfacing as drift is a known gap and a
  real source of noise, but relocating a claim across a rename is anchoring
  work, not accounting work. Its own spec.
- **Cross-model cost normalisation.** Token counts are model- and
  harness-specific. Comparing them across a corpus needs a normalisation nobody
  has defined; until then they are readable per-session and not aggregable.
- **Billing or budget enforcement.** pawl reports what accounting cost. What
  anyone does about that is not a gate decision.

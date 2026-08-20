# PAWL-029 — Versioning model

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/release`, `.github/workflows/`
**Extends:** [PAWL-013](./PAWL-013-versioning-and-release.md) (delivered,
immutable) and [PAWL-027](./PAWL-027-contribution-and-release-flow.md).

**Supersedes by reference:**

| Criterion | Was | Replaced by |
|---|---|---|
| PAWL-013 **AC2** | any verdict-altering change is MAJOR, including a bug fix | AC1, AC2 |
| PAWL-013 **AC5** | below 1.0, apply AC2 and AC3 one position down | AC4 |
| PAWL-013 **AC13** | no release branches; every release cut from `main` | AC7 |
| PAWL-013 **AC1** | tagged `vMAJOR.MINOR.PATCH` on a commit reachable from `main` | AC9 — unchanged for ordinary releases, narrowed for a maintenance release |
| PAWL-013 **AC7** | a tag not reachable from `main` fails the release | AC9 |
| PAWL-027 **AC3** | `Verdict-Affecting: yes` forces MAJOR whatever the type | AC3, AC5 — it stops being a version input |
| PAWL-027 **AC4** | the trailer is stated as `yes` or `no` | AC5 — the same trigger, a three-way answer |
| PAWL-027 **AC13** | bump table with `Verdict-Affecting: yes` → MAJOR | AC3 |
| PAWL-027 **AC14** | pre-1.0 shifts every bump down one position | AC4 |

Listing AC1, AC5 and AC7 of PAWL-013 matters: leaving them active would have left
a contract that contradicts itself, because AC1 and AC7 require every release tag
to be reachable from `main` while AC7 below permits a maintenance release that is
not, and AC5 requires the pre-1.0 shifting AC4 removes. Superseding four criteria
and leaving three that disagree with them is not a narrower change, it is an
ambiguous one.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

PAWL-013 AC2 made any change that could alter a gate verdict a MAJOR change,
including a bug fix. The reasoning was that a client's contract with pawl is
which changesets pass, so a corrected line count breaks that contract even though
every flag still works.

**That is being replaced because it does not survive automation.** Under AC2 a
fix in the resolver, the policy or the accounting is MAJOR — PAWL-026's two-line
bounds check would have been. A human cutting releases batches those and bumps
occasionally; PAWL-027 computes a version from every release, so the rule applies
every time rather than sometimes.

How often that produces a major depends on how often such fixes land, which has
not been measured: the evidence is one qualifying fix and the per-release
mechanism, not a rate. What is established without a rate is that the rule now
applies mechanically and unconditionally, where AC2 was written for a process
that applied it by judgement.

Under standard SemVer a correction of behaviour to its intended outcome is a
PATCH. That is the ordinary reading of the specification and it needs no
appeal to what other tools do.

### What that concedes, and what still has to be handled

pawl is a gate, and a gate has an asymmetry:

| A fix that makes the gate… | What the client sees |
|---|---|
| **stricter** — escalates more | a build fails. Loud, immediate, investigated. |
| **more permissive** — escalates less | for a client reading only pass and fail, nothing. |

The strict direction announces itself and needs no version machinery. The
permissive direction produces no failure to observe, and the absence of an
escalation reads the same as a changeset that never needed one.

That is a claim about what a client sees, not about what is observable in
principle: a team tracking escalation counts over time would notice the rate
change. The disclosure below is for the client who consumes a verdict and nothing
else, which is the client the gate is aimed at.

`Verdict-Affecting` already captures the signal at commit time (PAWL-027 AC3,
AC4). Today it collapses to yes or no. Recording the direction reuses that moment
— though not for free: deciding *which side* the gate moved is a judgement the
boolean did not ask for.

## Versioning

**AC1** — The system shall treat a change that removes a command or flag, or
alters the meaning of an existing command, flag, input format or output format,
as MAJOR.
`checkable: no` — whether a change alters a meaning is a review judgement and
nothing in the tree can decide it. Recorded as unchecked rather than as
`partially`, because a partial criterion with no check behind it is a permanent
tax on attention dressed as coverage. AC3 and AC9 are what a machine enforces
around it: the bump a commit's type implies, and where a tag may point.

**AC2** — The system shall treat a backward-compatible addition to the
command-line surface as MINOR, and a correction that brings behaviour back to
its already-published meaning as PATCH.
`checkable: no` — same reasoning as AC1. AC3 is what a machine enforces.

**AC3** — The system shall compute the version bump as follows, highest wins:

| Condition | Bump |
|---|---|
| `!` or `BREAKING CHANGE` | MAJOR |
| any `feat` | MINOR |
| any `fix`, `perf` | PATCH |
| any `revert` | the bump of the change it reverts, or PATCH if that is not stated |
| only `docs`, `test`, `ci`, `build`, `chore`, `refactor`, `style` | none |

`checkable: yes` (once built) — `Verdict-Affecting` no longer appears. It ceases
to be a version input and becomes a disclosure input (AC5).
>
> `revert` was accepted by commitlint and by `KnownType` and had no row here, so
> the implementation maps it to no bump — a revert of a breaking change currently
> produces nothing to release. It cannot be derived mechanically, because the
> bump depends on what was reverted; the commit states it, and PATCH is the
> floor when it does not.

**AC4** — The system shall apply the same bump below 1.0 as above it.
`checkable: yes` (once built) — supersedes PAWL-013 AC5 and PAWL-027 AC14, which
shifted every bump down so that pre-1.0 could not be used as licence to change
anything. With MAJOR narrowed by AC1 the shifting no longer buys that, and the
protection that mattered now lives in AC5 and AC6.

**AC10** — The system shall refuse to publish a release whose notes omit a change
that a commit declared `more-permissive`.
`checkable: yes` (once built) — the enforceable half of AC6, not of AC1.
>
> An earlier draft made this "refuse a MAJOR release unless a commit carries `!`
> or `BREAKING CHANGE`", which cannot fail: AC3 produces MAJOR only when one of
> those markers is present, so the check restated its own precondition. It was
> written to discharge a `checkable: partially` and discharged nothing — a check
> that cannot fail is the thing this repository exists to refuse, and it took a
> review to notice I had written one.

## Verdict direction

**AC5** — Where a change touches a module that participates in deciding a
verdict, the system shall require the commit to declare `Verdict-Affecting` as
`stricter`, `more-permissive` or `no`, and shall reject a commit that omits it.
`checkable: yes` (once built) — the trigger is PAWL-027 AC4's, unchanged and
mechanically detectable: `internal/policy`, `internal/resolve`,
`internal/accounting`, `internal/evidence`.

> The trigger deliberately is not "where a change alters which changesets pass".
> That phrasing makes `no` unanswerable — a change that alters them cannot
> truthfully declare `no` — and makes enforcement depend on detecting the very
> thing the non-functional section says cannot be verified. Touching a deciding
> module is a fact about the diff. Whether it moved verdicts, and in which
> direction, is the author's claim about that fact.

**AC6** — The system shall name changes declared `more-permissive` first and
separately in the release notes, and shall state that they may allow changesets
to merge that previously escalated.
`checkable: yes` (once built) — a client deciding whether to take an upgrade has
one question ahead of every other, and it is not "did the API change".

**AC7** — Where a fix is required for a released version that a client cannot
take the current release to obtain, the system may cut it on a maintenance
branch from that version's tag.
`checkable: no` — whether a client can take the current release is a judgement
about that client. AC9 is the mechanical rule this permits.

**AC9** — The system shall tag releases `vMAJOR.MINOR.PATCH`; shall accept a tag
reachable from `main`, or one whose merge-base with `main` is itself a release
tag; shall reject any other; and shall fail a release built from a tree that is
not clean.
`checkable: yes` (once built) — supersedes PAWL-013 AC1 and AC7.
>
> Those two carried three requirements between them and an earlier draft replaced
> only one. The SemVer tag format and the refusal to build from a dirty tree are
> restated here because superseding a criterion wholesale to change part of it
> discards the rest silently.
>
> "A branch whose first commit is a released tag" was also not decidable: git
> does not record where a branch began. The merge-base with `main` is a fact git
> can answer, and it says the same thing for the case AC7 permits.

**AC8** — A backport shall carry a direction declaration classified against the
branch it lands on.
`checkable: no` — AC5 checks that a declaration is present and well-formed, which
is a different claim. Whether it was classified against the right baseline needs
the replay harness this spec puts out of scope, so it is recorded as unchecked
rather than asserted.

What this criterion fixes is the baseline the declaration is measured against.
Copying the source commit's answer would be wrong often enough to matter: the
same logical correction can be stricter relative to one baseline and more
permissive relative to another, because the branch has drifted. A disclosure
computed against the wrong baseline is worse than none, since it is believed.

## Non-functional

- **This is a narrowing of what MAJOR promises, and it must be stated where
  clients read it.** `docs/install.md` tells clients that any change to gate
  behaviour is a major bump. That stops being true here, and a client pinning on
  the strength of it would be pinning on a promise that no longer exists.
- **The direction is a claim, and claims are what this product is about.** An
  author writing `no` on a change that loosens the gate has asserted something
  they did not establish. Nothing here can check it — the honest position is that
  it is recorded, attributable and reviewable, not that it is verified.
- **The `Verdict-Affecting` trailer keeps its name.** It is in the history, in
  hooks, in CI and in two skills. Renaming it to `Verdict-Direction` would be
  tidier and would break every commit already carrying it.

## What this closes, and what it does not

PAWL-013's open decision asked two questions: **which versions are supported**,
and **is there a backport path**. AC7 and AC9 answer the second. The first is
**not** answered — no support window is stated, and PAWL-013 said that question
cannot be deferred past a first engagement because a client's security team asks
it during procurement. Deferring it again is a choice recorded as one.

## Out of scope

- **The support window.** Named above rather than quietly omitted.
- **Verifying a declared direction.** A replay harness could do it — same
  changeset, two versions, compare verdicts — and it is the natural use of
  PAWL-007's sampler. Worth its own spec.
- **Migrating existing history.** Commits already carry `Verdict-Affecting: yes`
  or `no`; nothing rewrites them, and AC5 binds from adoption.

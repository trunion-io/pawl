# PAWL-029 — Versioning model

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/release`, `.github/workflows/`
**Extends:** [PAWL-013](./PAWL-013-versioning-and-release.md) (delivered,
immutable) and [PAWL-027](./PAWL-027-contribution-and-release-flow.md).

**Supersedes by reference:**

| Criterion | Was | Now |
|---|---|---|
| PAWL-013 **AC2** | any verdict-altering change is MAJOR, including a bug fix | AC1 below — MAJOR is a break in the command-line contract |
| PAWL-013 **AC13** | no release branches; every release cut from `main` | AC7 below — a maintenance branch is permitted for a fix to a released version |
| PAWL-027 **AC13** (bump table) | `Verdict-Affecting: yes` → MAJOR | AC3 below |
| PAWL-027 **AC14** | pre-1.0 shifts every bump down one position | AC4 below |

It also closes PAWL-013's **open decision** on supported versions and backports.

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

**That is being replaced, and the reason it is being replaced is that it does not
survive automation.** Under AC2, almost any real fix in the resolver, the policy
or the accounting is MAJOR — the two-line bounds check in PAWL-026 would have
been. A human cutting releases batches those and bumps occasionally; PAWL-027
computes a version from every release, so the same rule produces a new major
every few days. Version 47 then means the verdicts moved 47 times, and MAJOR has
stopped carrying the thing every reader assumes it carries.

There is a second cost. Clients are told to pin (`docs/install.md`), so "upgrade
to get the security fix" came to mean "accept a change in which changesets pass"
— precisely what a pinning client was avoiding. PAWL-013 recorded that as an open
decision and it has been open since.

So: SemVer means here what it means everywhere. MAJOR is a break in the
command-line contract, and a fix that corrects behaviour is a fix.

### What that concedes, and what still has to be handled

Every other analyser does this. ESLint corrects a false positive in a patch and
some builds start passing that did not; nobody calls that breaking, and a tool
that could not fix a bug without a major bump could not fix bugs.

But pawl is a gate, and a gate has an asymmetry a linter does not:

| A fix that makes the gate… | What the client sees |
|---|---|
| **stricter** — escalates more | a build fails. Loud, immediate, investigated. |
| **more permissive** — escalates less | nothing. Changesets that needed human review now merge clean. |

The strict direction announces itself and needs no version machinery. **The
permissive direction is silent by construction**: there is no failure to observe,
no error, no log line — the absence of an escalation is indistinguishable from a
changeset that never needed one.

That asymmetry is not addressed by SemVer, and it is not addressed by backports
either: backporting a fix that loosens the gate delivers the loosening to the
version a cautious client pinned *because* they did not want change.

`Verdict-Affecting` already captures the signal at commit time (PAWL-027 AC3,
AC4). Today it collapses to yes or no. Recording the direction costs an author
nothing extra at the moment they already have to answer.

## Versioning

**AC1** — The system shall treat a change that removes or alters the meaning of a
command, flag, input format or output format as MAJOR.
`checkable: partially` — the classification is a review judgement; that a MAJOR
release accompanies a change to the CLI surface can be checked.

**AC2** — The system shall treat an addition that does not alter existing
behaviour as MINOR, and a correction of behaviour to its documented or intended
outcome as PATCH.
`checkable: partially`

**AC3** — The system shall compute the version bump as follows, highest wins:

| Condition | Bump |
|---|---|
| `!` or `BREAKING CHANGE` | MAJOR |
| any `feat` | MINOR |
| any `fix`, `perf` | PATCH |
| only `docs`, `test`, `ci`, `build`, `chore`, `refactor`, `style` | none |

`checkable: yes` (once built) — `Verdict-Affecting` no longer appears in this
table. It ceases to be a version input and becomes a disclosure input (AC5).

**AC4** — The system shall apply the same bump below 1.0 as above it.
`checkable: yes` (once built) — PAWL-027 AC14 shifted every bump down a position
so that pre-1.0 could not be used as licence to change anything. With AC1 the
shifting is no longer buying that, and it produces a first release of `v0.0.1`
for a tool that `docs/install.md` and `examples/pawl-gate.yml` both document as
`0.1.0`. The protection that mattered is now in AC5 and AC6.

## Verdict direction

**AC5** — Where a change alters which changesets pass, the system shall require
the commit to declare the direction as `stricter`, `more-permissive`, or `no`.
`checkable: yes` (once built) — extends PAWL-027 AC4, which already refuses
silence from a commit touching a deciding module. The question an author is asked
changes from a yes-or-no to a three-way; the moment they are asked does not.

> **`more-permissive` is the answer this whole spec exists to surface.** A
> stricter gate fails a build and gets investigated. A more permissive gate
> produces no signal at all, and under standard SemVer it now ships in a PATCH —
> which is the right call for a bug fix and the wrong one to leave undisclosed.
> Making the author name the direction is what turns an invisible change into a
> recorded one, at the point where the knowledge exists (C-2).

**AC6** — The system shall name changes declared `more-permissive` first and
separately in the release notes, and shall state that they may allow changesets
to merge that previously escalated.
`checkable: yes` (once built) — a client deciding whether to take an upgrade has
one question ahead of every other, and it is not "did the API change".

**AC7** — Where a fix is required for a released version that a client cannot
take the current release to obtain, the system may cut it on a maintenance
branch from that version's tag.
`checkable: partially` — supersedes PAWL-013 AC13 and closes its open decision.
Trunk-based remains the default and every ordinary release is still cut from
`main`; this exists for the case PAWL-013 named and had no answer to, which is a
security fix a pinning client needs without the changes that followed it.

**AC8** — A backport shall carry the same direction declaration as the change it
originates from.
`checkable: yes` (once built) — a fix that loosens the gate loosens it on the
branch too, and a client pinned there is the one least expecting it. Backporting
without the declaration would deliver silently exactly what AC5 exists to
disclose.

## Non-functional

- **This is a narrowing of what MAJOR promises, and it must be stated where
  clients read it.** `docs/install.md` currently tells clients that any change to
  gate behaviour is a major bump. That stops being true here, and a client pinning
  on the strength of it would be pinning on a promise that no longer exists.
- **The direction is a claim, and claims are what this product is about.** An
  author writing `no` on a change that loosens the gate has asserted something
  they did not establish. Nothing here can check it — the honest position is that
  it is recorded, attributable and reviewable, not that it is verified.
- **The `Verdict-Affecting` trailer keeps its name.** It is in the history, in
  hooks, in CI and in two skills. Renaming it to `Verdict-Direction` would be
  tidier and would break every commit already carrying it.

## Out of scope

- **Which versions are supported for how long.** AC7 permits a maintenance
  branch; it does not commit to a support window, and a client's security team
  will ask for one.
- **Verifying a declared direction.** A replay harness could do it — same
  changeset, two versions, compare verdicts — and it is the natural use of
  PAWL-007's sampler. Worth its own spec.
- **Migrating existing history.** Commits already carry `Verdict-Affecting: yes`
  or `no`; nothing rewrites them, and AC5 binds from adoption.

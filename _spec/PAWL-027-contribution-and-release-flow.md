# PAWL-027 — Contribution and release flow

**Status:** DRAFTED, NOT BUILT · **Module:** `.github/workflows/`, `commitlint.config.js`
**Extends:** [PAWL-013](./PAWL-013-versioning-and-release.md) (delivered,
immutable) — that spec defines what a version *means* and what a release must
produce. This one defines how the version is arrived at and how a change reaches
`main`. AC2 of PAWL-013 is unchanged and constrains everything below.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (SRE) | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

Releases are currently cut by a human tagging a commit. That satisfies PAWL-013
AC6 — the version comes from the tag, not from an operator's flag — but only by
moving the human decision one step earlier: somebody still chooses the numbers.
Hazard 2 in PAWL-013 is therefore still open in practice. Nothing stops a
verdict-changing fix being tagged as a patch, because nothing looks at what the
commits did.

This spec closes it by deriving the version from the commit history, which means
the commit history has to be machine-readable. That is what Conventional Commits
buys, and it is the only reason to adopt it.

### The conflict that has to be resolved first

Conventional Commits' conventional mapping is `fix:` → PATCH, `feat:` → MINOR,
`BREAKING CHANGE` → MAJOR. **pawl cannot use that mapping.** PAWL-013 AC2:

> …any change that can alter a gate verdict, for an unchanged changeset against
> an unchanged policy, as a MAJOR change — **including a bug fix**.

The two rules disagree about the most common case in this repository. A concrete
example already in the history: `Reject policy thresholds the gate cannot apply`
is a `fix` by every conventional reading — it corrects a defect and adds no
feature. It also changes which policy files load, and therefore which changesets
are gated at all. Under the conventional mapping it ships as a patch, and a
client who pinned a patch range wakes up to a pipeline behaving differently.

Adopting Conventional Commits without addressing this would automate the exact
hazard PAWL-013 was written to close, and would do it faster and more reliably
than a human ever could.

## Commit format

**AC1** — The system shall require every commit merged to `main` to follow
Conventional Commits.
`checkable: yes` → `test:trunion.io/pawl/internal/release.TestParseCommitRejectsNonConventional`

**AC2** — The system shall reject a commit whose type is not in the configured
set.
`checkable: yes` → `test:trunion.io/pawl/internal/release.TestEveryTypeIsKnown`

**AC3** — The system shall require a commit that can alter a gate verdict to
declare that it does, and shall treat such a commit as verdict-affecting when
computing the version regardless of its type.
`checkable: yes` → `test:trunion.io/pawl/internal/release.TestVerdictAffectingFixIsMajor` — a `Verdict-Affecting: yes` trailer. `!`/`BREAKING
CHANGE` is not reused for this: it already means the command-line contract
changed, and the two are genuinely different failures for a client. A flag that
disappeared breaks their pipeline loudly; a threshold that moved does not.

**AC4** — Where a change touches a module that participates in deciding a
verdict, the system shall require the commit to state `Verdict-Affecting` as
either yes or no, and shall reject a commit that omits it.
`checkable: yes` (once built) — the modules are `internal/policy`,
`internal/resolve`, `internal/accounting` and `internal/evidence`.

> **AC4 is the criterion that makes AC3 work, and it is the one worth defending.**
> Asking authors to remember a trailer produces a rule that is followed until it
> is inconvenient. Asking them to *answer a question they cannot skip*, only when
> they touch code that decides verdicts, produces a record of a decision. It is
> C-3 applied to our own process: silence about whether a change moves verdicts
> is not evidence that it does not, so silence is refused rather than
> interpreted.
>
> The cost is real and accepted: some commits will declare `no` and be right,
> and the declaration will look like ceremony. That is what a conscious `no`
> costs, and it is cheaper than one wrong patch release.

## Merging

**AC5** — The system shall require every change to reach `main` through a pull
request, and shall not permit a direct push.
`checkable: partially` → `test:trunion.io/pawl/internal/release.TestRulesetEncodesTheContributionRules`
holds the intended ruleset to this criterion; GitHub is the enforcement point
and nothing in this tree proves what is live. Already enforced in practice —
verified by an actual rejected push (`GH006 … Changes must be made through a
pull request`).

**AC6** — The system shall require the full check suite to pass before a pull
request may merge, and shall apply that requirement to administrators.
`checkable: partially` → `test:trunion.io/pawl/internal/release.TestRulesetEncodesTheContributionRules`
holds the intended ruleset to this criterion, including that the only bypass is
the single documented break-glass route. GitHub is the enforcement point and
nothing in this tree proves what is live. PAWL-013 AC12 required the first half;
extending it to administrators is what stops the rule being advisory in a
repository with one of them.

**AC7** — The system shall require linear history on `main`.
`checkable: partially` → `test:trunion.io/pawl/internal/release.TestRulesetEncodesTheContributionRules`
holds the intended ruleset to this criterion; GitHub is the enforcement point
and nothing in this tree proves what is live. A version derived from commit
history needs a history that reads in one order.

## Release candidates

**AC8** — When the full check suite passes on `main`, the system shall tag that
commit as a release candidate for the version the accumulated commits imply.
`checkable: yes` → `test:trunion.io/pawl/internal/release.TestRCChecksMatchRuleset`

**AC9** — The system shall number release candidates sequentially within a target
version and shall not reuse a candidate number.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestRCReusesTheCandidateForTheSameCommit` — `v0.2.0-rc.1`, `v0.2.0-rc.2`.

**AC10** — Where no commit since the last release implies a version change, the
system shall create no release candidate.
`checkable: yes` (once built) — a docs-only or CI-only commit is not a candidate
for anything, and tagging one would bury the real candidates.

**AC11** — A release candidate shall be built, verified and signed by the same
path as a release, and shall be marked as a prerelease.
`checkable: yes` (once built) — a candidate that is built differently from the
release tests nothing. This is the point of having them.

## Release

**AC12** — The system shall compute the release version from the commits since
the previous release tag, and shall accept no operator-supplied version.
`checkable: yes` (once built) — this is PAWL-013 AC6's intent carried further.
AC6 forbade an operator typing a version into the build; a human choosing which
tag to push is the same decision wearing a different hat, and this removes it.

**AC13** — The system shall compute the version bump as follows, highest wins:

| Condition | Bump |
|---|---|
| `Verdict-Affecting: yes` on any commit | **MAJOR** |
| `!` or `BREAKING CHANGE` on any commit | MAJOR |
| any `feat` | MINOR |
| any `fix`, `perf` | PATCH |
| only `docs`, `test`, `ci`, `build`, `chore`, `refactor`, `style` | none |

`checkable: yes` → `test:trunion.io/pawl/internal/release.TestBumpForCoversEveryRow` — the first row is the one that differs from every
off-the-shelf implementation, and it is the whole reason this table is written
out rather than delegated.

**AC14** — While the major version is `0`, the system shall shift the bumps in
AC13 down one position, so that a MAJOR bump moves MINOR.
`checkable: yes` → `test:trunion.io/pawl/internal/release.TestPreOneZeroShiftsBumpsDown` — PAWL-013 AC5 already requires this; stated here
because the tool implementing AC13 must implement it too, and every off-the-shelf
one gets it wrong by treating pre-1.0 as unconstrained.

**AC15** — The system shall refuse to release where the computed version already
exists as a tag.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestTagScriptRefusesAnExistingTagByDefault`

**AC16** — The system shall publish release notes derived from the commits
included in the release, grouped by type, naming verdict-affecting changes
separately and first.
`checkable: yes` → `test:trunion.io/pawl/internal/release.TestNotesPutVerdictAffectingFirst` — a client deciding whether to take an upgrade
cares about exactly one question first, and PAWL-013 AC2 says that question is
whether verdicts move.

## Non-functional

- **The version must never be a judgement made at tag time.** Every input to
  AC13 is written by the author of the change, while they are making it, when
  they know whether it moves verdicts. This is C-2's argument — claims are
  captured at edit time because that is when the knowledge exists — applied to
  our own release process.
- **A tool that gets AC13's first row wrong is worse than no tool.** Automation
  that silently ships a verdict change as a patch does it on every release rather
  than occasionally. Any off-the-shelf release tool adopted here must be
  configured for that row and tested against it, or written here instead.
- **commitlint brings a Node toolchain into a repository that has deliberately
  avoided one.** It is a CI-only dependency and never enters the binary, so the
  zero-dependency property of the artifact is intact — but PAWL-025 AC1 applies
  to it exactly as to any action, and it must be pinned and updated
  deliberately. A Go implementation of the same check would avoid the toolchain
  entirely and is the obvious replacement if the Node surface becomes a cost.
- **Release candidates are not a staging environment.** They exist so the release
  path is exercised continuously rather than for the first time under pressure at
  a release. If nobody ever installs one, they have still done their job.

## Out of scope

- **Backports and supported versions.** Still the open decision in PAWL-013, and
  AC13 makes it sharper: automation will produce MAJOR bumps more often than a
  human would have.
- **Package-manager publishing.** PAWL-013, out of scope there too.
- **Squash versus merge commit policy** beyond the linear-history requirement.
- **Changelog file in the tree.** AC16 covers release notes; a `CHANGELOG.md` is
  a separate question about where history should live.

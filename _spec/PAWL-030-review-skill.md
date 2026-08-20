# PAWL-030 — Review skill

**Status:** DRAFTED, NOT BUILT · **Module:** `.github/skills/`
**Extends:** [PAWL-028](./PAWL-028-agent-skills.md) (drafted) — that spec covers
skills aimed at an agent *writing* code in a client repository, and puts skills
for pawl's own development explicitly out of scope. This one takes that scope up.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |

## Context

An automated reviewer now comments on every pull request. Its first review found
three real defects, one of which hid a worse one: a dead variable in `rc.yml`
that turned out to be masking a check list three entries out of date, so a
release candidate could have been tagged on four of seven required checks.

That is a good result from a reviewer that knows nothing about this repository.
It is also the ceiling of what a reviewer can do unaided, and the ceiling has a
specific shape: it catches what is wrong *in general* — dead code, overclaiming
comments, unpinned dependencies — and cannot catch what is wrong *here*.

The rules that make a change wrong in this repository are not inferable from the
diff:

- a criterion must exist before the code does, and documentation is an output
  bound by the same rule
- a delivered spec is immutable, so an edit to one is a defect even when the edit
  is an improvement
- `go.mod` having no `require` block is an argument the distribution rests on,
  not a coincidence
- tests build real git repositories, and a mock would be a violation rather than
  a simplification

A reviewer without those will approve changes that break them, and — the more
expensive failure — will raise findings against deliberate decisions. The
hand-rolled TOML subset, comments counting as reviewable lines, and everything
living under `internal/` all look like defects and are all deliberate. A reviewer
that reports them every time trains the maintainer to skim its output, which
costs more than the reviewer returns.

## Criteria

**AC1** — The system shall provide a skill that states the repository's binding
rules in a form a reviewer can check a diff against.
`checkable: yes` (once built)

**AC2** — The skill shall state that a change without a criterion answering to it
is a finding, including for documentation and build configuration.
`checkable: yes` (once built) — the rule most likely to be broken by a reviewer
that assumes documentation is exempt.

**AC3** — The skill shall state that a delivered spec is immutable and that an
edit to one is a finding regardless of merit.
`checkable: yes` (once built)

**AC4** — The skill shall name the deliberate decisions that resemble defects,
and state that reporting them is itself a finding against the review.
`checkable: partially` — that the list is present is checkable; whether a
reviewer honours it is not. Closed in the same changeset by keeping the list in
one place, so it cannot drift from the one in `CLAUDE.md`.

> **This is the criterion that earns the skill its cost.** A reviewer's value is
> destroyed faster by confident wrong findings than by missed ones: a missed
> finding costs one defect, a false one costs attention on every pull request
> until the output is ignored entirely. PAWL-025 makes the same argument about
> scanners and noise.

**AC5** — The skill shall describe the failure this product exists to refuse —
an assertion made without the evidence to support it — and state that a comment
or document claiming a property the code does not deliver is a finding.
`checkable: yes` (once built) — C-1 turned on the repository itself. The
reviewer's own third comment was exactly this class, found without being told;
stating it should make the class routine rather than lucky.

**AC6** — The skill shall state that a change to a module that decides verdicts
requires a `Verdict-Affecting` declaration, and that its absence is a finding.
`checkable: yes` (once built) — PAWL-027 AC4 enforces this in CI; a reviewer that
does not know it will read the trailer as noise and may suggest removing it.

**AC7** — The skill shall not restate what the operator reference and the
constitution already say, and shall link to them.
`checkable: partially` — PAWL-028 AC8 makes the same demand for the same reason:
two documents describing one rule drift, and the copy a machine reads drifting is
worse than the copy a human reads drifting.

## Non-functional

- **A skill is documentation, and PAWL-010 binds it.** A skill describing a rule
  the repository does not hold is a defect, not a stale doc.
- **Delivery is not settled by this spec.** Where a reviewer reads instructions
  from is that product's decision and will change; the skill is written so it
  remains correct wherever it is loaded from, and the location is recorded rather
  than assumed.
- **Written for a reader with no memory between pull requests.** Anything it
  needs, it must carry or link to.

## Out of scope

- **Making the reviewer's findings gate a merge.** They are advisory, and PAWL-025
  is where a check that can fail the build belongs.
- **Instructions for a human reviewer.** `AGENTS.md` and `CLAUDE.md` serve that
  reader and are not replaced.
- **Any other automated reviewer's configuration format.**

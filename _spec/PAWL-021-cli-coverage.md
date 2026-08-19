# PAWL-021 — CLI coverage

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/e2e`

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

The suite drives the packages directly and never invokes the binary. Four
defects in one session appeared only when someone ran the command by hand:

| Defect | How it failed |
|---|---|
| `pawl claim "text" --path x` parsed zero flags | Silent no-op; the claim was never recorded |
| The same bug again in `pawl review <id> --span …` | Silent no-op; a verdict was never recorded |
| `pawl hook claude-code` blocked forever at a prompt | No output, no error, no indication why |
| A new command slipped past the doc-drift check | The check compared flag names, not commands |

Every one passed the whole suite. The first two are the same bug found twice,
which is the signature of a class nothing is looking for rather than of bad
luck.

They share a cause: all of them live in the **seam between the standard
library's `flag` package and our model of it**, not in any logic the tests
exercise. `flag` stops parsing at the first non-flag argument; a command reading
stdin blocks when stdin is a terminal. Neither fact is visible from a function
call.

That is C-9's argument, applied to a second seam:

> Tests run against real git repositories and real tool output formats. Never
> mock git, diff parsing, or evidence files. Every defect found so far lived in
> the seam between git's behaviour and our model of it; a mock would have hidden
> all three.

Calling `cmdClaim` from a test is the same move as mocking git. It exercises our
model of the CLI rather than the CLI.

## Acceptance criteria

**AC1** — The tests shall invoke pawl as a subprocess rather than calling its
command functions.
`checkable: yes` (once built) — the defects are in process behaviour: argument
parsing, stdin, exit status. A function call cannot see any of them.

**AC2** — The tests shall build the binary under test from the current source.
`checkable: yes` (once built) — running whatever `pawl` is on `PATH` would test
somebody's stale install and pass while the working tree is broken.

**AC3** — Every command shall be invoked at least once by the suite.
`checkable: yes` (once built) — a command nothing runs is a command whose first
execution is a user's.

**AC4** — For every command taking a leading positional argument, the tests
shall assert that flags following it are parsed.
`checkable: yes` (once built) — **the criterion that exists because of history.**
This exact bug shipped twice. A shared helper now handles it, and this is what
stops the third occurrence.

**AC5** — The tests shall assert exit status, distinguishing a verdict about the
changeset from a failure of pawl itself.
`checkable: yes` (once built) — `1` and `2` mean different things to a CI job,
and documenting the difference without testing it is a promise nothing keeps.

**AC6** — Where a command reads standard input, the tests shall exercise it both
with input and with none.
`checkable: yes` (once built) — the hook blocked forever on the second case, and
no unit test could have noticed.

## Non-functional

- **Slower, and worth it.** Subprocess tests cost process spawns where a
  function call costs nothing. Four defects in one session is the price of the
  cheaper option, and every one reached a human rather than a test.
- **Not a second copy of the logic tests.** These exercise the seam: parsing,
  streams, exit status, output shape. What a reading list contains is settled
  elsewhere and should not be re-asserted here.
- **A binary built once per run.** Building per test would dominate the runtime
  and teach people to skip the suite.

## Out of scope

- **Testing the harness protocol end to end.** `pawl hook` is exercised through
  its package; whether Claude Code invokes it correctly is PAWL-020's open
  verification and cannot be settled from inside this repository.
- **Golden-file comparison of human-readable output.** The reading list's
  wording will change and a test that freezes it becomes a tax on every edit.
  Assert what must be true, not the whole string.

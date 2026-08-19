# PAWL-020 — Accounting at the turn boundary

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/harness`, `internal/cli`
**Resolves:** the open question on [PAWL-019](./PAWL-019-harness-installation.md)
**Delivers:** [PAWL-017](./PAWL-017-deterministic-accounting.md) AC13, which was
specified and never built
**Related:** [PAWL-016](./PAWL-016-edit-time-accounting-hook.md) chose the
per-edit binding this replaces as primary.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

The hook binds to `Edit|Write|MultiEdit`. An agent that changes files through
the shell — `sed -i`, a heredoc, `>` redirection, a script — matches none of
them. No hook fires, no prompt appears, and the omission surfaces only at PR
time when the gate blocks on unaccounted lines. That is exactly the C-2 backfill
situation the hook was built to prevent, reached by a route the hook cannot see.

This is not hypothetical. Most of the work on pawl in the session that found it
was done through shell edits, and none of it was accounted for.

### Why adding `Bash` to the matcher is only half an answer

A `Bash` payload carries a command, not a file path. Determining which files a
shell command touched means either parsing arbitrary shell — unbounded and
fragile — or not caring, and asking the working tree instead.

Asking the tree turns out to be affordable. A whole-tree scan measures **17ms**
on this repository, and the surfacing cache already suppresses output when the
pending set has not changed, so a `Bash` call that edited nothing costs 17ms and
says nothing. Cost is therefore not what decides this.

What decides it is that **the tool that made a change is the wrong thing to bind
to**. Every binding to a tool name is a list of ways to edit a file, and a list
like that is never finished: shell today, an MCP server tomorrow, a harness
feature next month. Each omission is silent, which is the worst property a gap
in accounting can have.

### The binding that is not a list

A turn boundary is tool-agnostic. Whatever changed the tree, and by whatever
route, the tree has changed by the time the turn ends.

PAWL-017 AC13 already specified this and it was never built:

> The system shall accumulate pending spans as edits occur and surface them once
> the changed spans have settled, rather than after each edit.

The argument there was about cost and about drift — a claim recorded while its
span is still moving will not match once the work settles, so claiming too early
manufactures the drift that drift detection reports. Closing the shell gap is a
third reason for the same change.

## Acceptance criteria

**AC1** — The system shall report unaccounted spans at the end of an agent's
turn, regardless of which tool made the change.
`checkable: partially` — depends on the harness exposing a turn-boundary event.

**AC2** — The turn-boundary report shall cover the working tree rather than a
named file.
`checkable: yes` (once built) — a report scoped to a file cannot be produced
without knowing which file changed, which is the thing a shell payload does not
say.

**AC3** — Where the harness exposes a per-edit event, the system may report
immediately in addition, but shall not rely on it as the only binding.
`checkable: yes` (once built) — immediacy is worth having for the tools that
offer it. Depending on it is what produced this gap.

**AC4** — The system shall stay silent at a turn boundary where nothing is
unaccounted.
`checkable: yes` (once built) — most turns will be silent, and a hook that
speaks every turn regardless is one an agent learns to ignore.

**AC5** — The system shall not prevent a turn from ending more than once for the
same set of unaccounted spans.
`checkable: yes` (once built) — **the criterion that keeps this safe.** A
turn-boundary hook that refuses to let the turn end is one edit away from a loop
an agent cannot escape. If the harness supports blocking at all, it may be used
at most once per unchanged set, and never as a way to compel a record.

**AC6** — Where the turn-boundary event is unavailable or fails, the system
shall fall back to the per-edit binding rather than reporting nothing.
`checkable: partially` — losing accounting entirely is worse than losing its
completeness.

## Non-functional

- **Verify the harness event before building on it.** These criteria assume a
  turn-boundary event exists, fires reliably, and can put text where the agent
  will act on it. That is an assumption about somebody else's product and it
  should be confirmed by observation, not by reading a schema. If it fires
  unreliably, AC6 is what the design rests on.
- **Silence is the normal case.** A turn in which everything was accounted for
  produces nothing. This differs from the per-edit binding, where the common
  case was a message.
- **This does not enforce.** Enforcement remains the gate's, exactly as
  PAWL-016 settled. A turn boundary is a better moment to inform, not a licence
  to start blocking.

## Out of scope

- **Parsing shell commands to infer which files changed.** Unbounded, fragile,
  and unnecessary once the tree is the unit of report.
- **Removing the per-edit binding.** AC3 keeps it as a supplement; whether it
  earns its place is a question for after the turn-boundary binding has run for
  a while.
- **Blocking a turn to compel a record.** AC5, and the enforcement position.

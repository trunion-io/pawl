---
name: recording-claims
description: Use when editing code in a repository that has a .pawl directory, or when pawl reports unaccounted lines. Explains what a claim records, when to record one, and why a claim that asserts correctness is wrong.
---

# Recording claims

pawl records **what you assumed** while editing, so a human reviewing your work
knows which lines they must read and why. It computes the minimum reading list
from those records.

## The one rule that matters

**A claim records what you assumed or could not establish. It never asserts that
your code is correct.**

You do not know whether your code is correct. Nothing you write in a claim makes
it more correct, and a claim that says so is worse than no claim at all: it
occupies the space where evidence should be and cannot be checked by anything.

| Wrong | Right |
|---|---|
| `this function is correct` | `assumed the caller holds the lock; not verified` |
| `tests pass` | `could not determine whether empty input reaches this path` |
| `refactored safely` | `assumed no external caller depends on the old field name` |

Evidence that something works comes from tests and coverage, which pawl reads
from your CI. Your job is the part machines cannot supply: what you took for
granted.

## When to record

**At the moment you make the assumption, not at the end of the task.** Recording
at the end means reconstructing what you must have assumed, which is a different
and less reliable thing than noticing it as it happens.

Record when you:

- take something for granted you did not verify
- cannot establish something and proceed anyway
- infer intent from a name, a comment, or surrounding style
- rely on behaviour of code you did not read

## How

```bash
pawl claim "assumed the caller validates the path" --path internal/x.go --lines 40-52
pawl claim "could not establish whether this runs concurrently" --path internal/x.go --lines 60-64 --kind undetermined
```

Use `--kind undetermined` when you could not establish something. Those always
escalate to a human, whatever the tests say — that is the point of them.

## Having nothing to claim is a correct outcome

Unclaimed lines fail the gate, so there is an obvious shortcut: write something
for every span until the number goes down. **Do not.** A claim you invented to
clear a gate is noise that makes the reading list useless, and it defeats the
only reason anyone would trust the output.

If an edit involved no assumptions — a rename your tooling verified, a
formatting change — say so with an acknowledgement rather than inventing a
claim:

```bash
pawl ack --path internal/x.go --lines 10-12
```

Where the repository configures deterministic rules, `pawl ack --auto` applies
them across the changeset — use `--dry-run` first to see what they match.

An acknowledgement asserts nothing, which is precisely why it can be automated
and a claim cannot.

## Full command detail

See [`docs/reference.md`](../../docs/reference.md). Flags are documented there
and not repeated here, so the two cannot drift.

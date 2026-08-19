---
name: reading-a-failed-gate
description: Use when `pawl gate` exits non-zero in CI or locally, to interpret each violation and choose the right response. Covers why changing the policy to go green is usually the wrong fix.
---

# Reading a failed gate

`pawl gate` exits non-zero when a changeset cannot be reviewed within the
thresholds the repository has set. It is telling you a human needs to look, not
that your code is wrong.

## The violations

| Violation | What it means | Usual response |
|---|---|---|
| `unclaimed_lines` | Changed lines with no claim or acknowledgement over them | Record what you assumed for those spans, or acknowledge them if nothing was assumed |
| `must_read_ratio` | Too much of the changeset needs human reading to review incrementally | Split the changeset |
| `changeset_size` | The changeset is too large to comprehend regardless of trail quality | Split the changeset |
| `undetermined_claims` | Something could not be established and the work continued anyway | A human decides; this is the gate working as intended |
| `sensitive_path_needs_named_check` | A path requiring named sign-off was touched | Get the named approval |

`pawl verify` prints the reading list — the actual spans and why each one is
there. Read that before changing anything.

## Splitting is usually the real answer

Two of the five violations are size. A 3,000-line changeset is unreadable even
when every line is annotated, and claim quality degrades across a long task as
the agent's own context fills. If the gate says the changeset is too big, the
changeset is too big.

## About changing the policy

The thresholds live in `.pawl/policy.toml` and **belong to the repository's
owners, not to pawl and not to you.** Changing them is legitimate — the defaults
are a starting point, and a team that finds one wrong should change it.

**But do not change a threshold to make a build pass.** That converts a gate into
a formality while leaving it in place, which is worse than removing it: the
pipeline still reports that a check ran. If a threshold is wrong, say so and
change it deliberately, as its own change, with the reasoning recorded — not as
part of the changeset it happens to be blocking.

If you are an agent: propose the policy change and let a human decide. Do not
edit `.pawl/policy.toml` to clear your own gate failure.

## A rejected policy is not a passed gate

If pawl reports that the policy itself is invalid, the gate did not run. That is
never a pass. Fix the policy file — the error names the key, the value and the
file.

## Full command detail

See [`docs/reference.md`](../../docs/reference.md).

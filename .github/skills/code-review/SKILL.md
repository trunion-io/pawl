---
name: code-review
description: Use when reviewing a change to this repository. States the rules that make a change wrong here, and the deliberate decisions that look like defects and must not be reported.
---

# Reviewing pawl

pawl computes the minimum set of lines a human must read before merge. Its own
rules are stricter than most repositories', and several are not inferable from a
diff. Read this before commenting.

## The failure this repository exists to refuse

**An assertion made without the evidence to support it.**

That is the product, and it applies to the repository itself. A comment, document
or spec claiming a property the code does not deliver is a finding — even when
the code is correct and only the claim is too broad.

Real examples from this repository, both found in review:

- a step commented "pinned exactly" that pinned direct packages but not their
  transitive graph
- a function commented "every bump shifts down one position" whose code shifted
  two of three cases

Neither was a bug. Both were findings.

## Rules a change can break

**A criterion must exist before the code does.** Every change answers to a
numbered criterion in [`_spec/`](../../../_spec). This includes documentation,
build configuration and CI: for a CLI tool the documentation is an output, the
same as the binary. "This isn't behaviour" is not an exemption. Code with no
criterion is a finding.

**Delivered specs are immutable.** A spec marked delivered is never amended. A
change extends it by reference with `**Extends:** PAWL-00N (delivered,
immutable)`. **An edit to a delivered criterion is a finding regardless of
merit** — including fixing something genuinely wrong in it. The only permitted
edit is repointing a `checkable:` reference when a check moves.

**`go.mod` has no `require` block, and must not gain one.** This is the argument
the distribution rests on — a supply-chain assurance tool that arrives with a
supply chain of its own argues against itself. A new dependency is a finding, not
a judgement call. CI tooling is separate and must be pinned to a commit.

**Tests never mock git.** They build real repositories in `t.TempDir()` and run
real `git` (C-9). A mock is a finding, not a simplification.

**`CGO_ENABLED=0` and `-buildvcs=false` must not be removed.** The first makes the
binary static; the second keeps builds reproducible from source alone, so a third
party can rebuild a tag and match the published checksum.

**A change to `internal/policy`, `internal/resolve`, `internal/accounting` or
`internal/evidence` must declare `Verdict-Affecting: yes|no`** in its commit
message or the pull request body. These decide which changesets pass, so a change
here alters a client's merge set. A missing declaration is a finding; a present
one is not noise and must not be suggested for removal.

**`checkable: partially` must be closed in the same changeset that writes it.**
That field is where a criterion goes to be forgotten.

**Silence is not coverage (C-3).** A check that cannot run must never report
success. Look for error paths that swallow a failure and continue, and for
statuses that report ok when something was skipped.

## Deliberate decisions that look like defects

**Reporting any of these is a finding against the review.** They have been
decided, and re-raising them costs attention on every pull request until the
output stops being read.

- `internal/policy/toml.go` is a hand-rolled TOML subset. It exists so the module
  graph stays empty, and rejects what it cannot parse rather than guessing.
- Everything is under `internal/`. Nothing here is a published library yet.
- Comments count as reviewable lines and cost collapse ratio. Agents write plenty
  of wrong ones.
- A skipped test is treated as absent rather than passing. Strict on purpose.
- Renames report as drift. Whitespace is normalised in fingerprints; identifiers
  are not.
- "The factory" in `internal/policy` means the delivery offering, not this tool.
- Commit messages are long and carry reasoning. That is the record, deliberately.

## Where the detail lives

Do not restate these; link to them.

- [`_spec/constitution.md`](../../../_spec/constitution.md) — binding, outranks everything
- [`docs/reference.md`](../../../docs/reference.md) — every command and flag
- [`AGENTS.md`](../../../AGENTS.md) — design decisions and current position
- [`_spec/README.md`](../../../_spec/README.md) — the spec index and process rules

## What a good finding looks like

Concrete, and tied to a rule or a failure. "This could be cleaner" is not useful
here. "This comment claims the transitive graph is pinned; without a lockfile it
is not" is, because it names the gap between what is asserted and what is
delivered — which is the whole subject of this repository.

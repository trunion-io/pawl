# CLAUDE.md — pawl

**Read [AGENTS.md](./AGENTS.md) first — it holds the full pawl context.**
Then read [./_spec/constitution.md](./_spec/constitution.md), which is binding
and outranks both.

Repo-wide Claude Code guidance is in [../CLAUDE.md](../CLAUDE.md). This file
holds only what is specific to working in pawl.

---

## Quick orientation

```bash
cd pawl && make check
```

`make check` is fmt + vet + the e2e suite: 64 tests, all must pass before and
after any change. If they don't pass on a clean checkout, fix that before
anything else — do not work around it.

[`.envrc`](./.envrc) puts `./bin` on `PATH`, so after `make build` the `pawl` you
invoke is your local build. It stops meaning anything when you leave the
directory.

## Where to start

Phase 1 is complete. PAWL-007, PAWL-016, PAWL-017, PAWL-018, PAWL-019 and
PAWL-020 are all built; the tool is not what is missing.

**What is missing is a corpus.** The sampler has almost nothing to sample and
the false-clear rate is a number over a handful of spans. That comes from
sustained real use (roadmap item 7), not from more code.

**Verify PAWL-020 before relying on it.** The turn-boundary binding closes the
shell-edit gap and every criterion is tested, but whether the harness's Stop
event fires — and puts its reason where an agent acts on it — has not been seen
in a live session. Confirm by observation. AC6 keeps the per-edit binding as the
fallback if it turns out unreliable.

Other unblocked work, in rough order of value:

1. Close the untested criteria flagged `checkable: partially` in
   [`./_spec/PAWL-004-reading-list.md`](./_spec/PAWL-004-reading-list.md)
   AC4 and AC5 — the claim-log exclusion and the non-semantic-line filter were
   both found in a live demo rather than by the suite, which is the wrong way
   round.
2. [`./_spec/PAWL-006-policy-gate.md`](./_spec/PAWL-006-policy-gate.md)
   AC1, AC3, AC5 — policy file loading, must-read ratio and sensitive paths are
   implemented but untested.
3. [`./_spec/PAWL-003-coverage-resolution.md`](./_spec/PAWL-003-coverage-resolution.md)
   AC3 — skipped-test handling has no dedicated test.

## Accounting for what you change

Every changed span must carry a **claim** or an **acknowledgement**. The hook
tells you which spans are outstanding; it deliberately does not repeat these
instructions each time, because that cost 127 of 159 tokens per edit.

```bash
pawl claim "<what you assumed>" --path <file> --lines <a-b> [--verified-by test:<TestName>]
pawl ack --path <file> --lines <a-b>     # nothing to assume here
pawl ack --auto                          # apply the rules in .pawl/policy.toml
pawl pending                             # what is still outstanding
```

- **Use `ack` freely** for mechanical edits. It costs no prose by design, and it
  is measured — over-acknowledging shows up as a rising ratio and as false
  clears in the sampler. It is not a way of hiding.
- **Use `claim`** when you assumed something, rejected an alternative, or could
  not establish something and proceeded anyway. `undetermined` is a correct
  answer, not a failure.
- **Do not cite `spec:` evidence.** It cannot resolve until PAWL-009 exists, so
  a claim citing it is permanently unverified.
- **Account while the reasoning is still in context.** Recording later against a
  finished diff is reconstruction, which C-2 forbids — and a claim recorded
  before its span stops moving will drift and end up needing a human anyway.

## Working style in this product

- **Spec first. Always.** Nothing lands without a criterion it answers to — not
  code, not documentation, not a schema or build change. "This isn't behaviour"
  is not an exemption; for a CLI tool the documentation is an **output**, the
  same as the binary, and `--help` drifting from `docs/` is a defect in the
  delivered product. See [`_spec/README.md`](./_spec/README.md).
- **Delivered specs are immutable.** Never amend one. Write a new spec that
  declares `**Extends:** PAWL-00N (delivered, immutable)` and states which of
  its criteria still hold. The only permitted edit to a delivered spec is
  repointing a `checkable:` reference when a check moves; the criterion text
  must not change.
- **Never mock git.** Tests build real repositories in `t.TempDir()` and run
  real `git` (C-9). This is not negotiable and it has already paid for itself
  three times.
- **Do not add a dependency.** `go.mod` has no `require` block. Keeping it that
  way is decision 6 in [AGENTS.md](./AGENTS.md), not a style preference — it is
  the argument the whole distribution rests on. If something genuinely needs a
  module, that is a conversation, not a commit.
- **Never unset `CGO_ENABLED=0`.** It is what makes the binary static.
- **Do not relitigate the six design decisions in [AGENTS.md](./AGENTS.md)**
  without saying explicitly which one you are challenging and why.
- **Do not edit the text of a C-rule during mechanical work.** Repointing a
  `checkable:` reference because a test moved is migration; changing what a rule
  says is a decision that needs Rich.

## Things that look like bugs and are not

- Renames report as drift. Whitespace is normalised in fingerprints; identifiers
  are not. A rename is a real change.
- Comments count as reviewable lines and cost collapse ratio. Deliberate — agents
  write plenty of wrong ones.
- `internal/policy` and `examples/policy.toml` refer to "the factory". That means
  the delivery offering, not this tool. Leave them.
- A skipped test is treated as absent rather than passing. Strict on purpose.
- `internal/policy/toml.go` is a hand-rolled TOML subset, not an oversight. It
  exists so the module graph stays empty; it rejects what it cannot parse rather
  than guessing.
- Everything is under `internal/`. Nothing here is a published library yet;
  promoting packages out is a Phase 4 decision, not a tidy-up.

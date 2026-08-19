# AGENTS.md — pawl

Context for any agent working on **pawl**. Read [the repository
AGENTS.md](../AGENTS.md) first for trunion-wide context, then this file.
[`./_spec/constitution.md`](./_spec/constitution.md) is binding and outranks
both.

---

## What this is

**pawl** — Provenance of Agent-Written Lines. Edit-time claim capture and
changeset verification for agentic delivery. The first trunion product.

> A pawl is the component in a ratchet that permits motion in one direction and
> prevents reversal. That is what this is: an append-only record of what an
> agent assumed while it worked, and a gate that will not turn backwards.

The changeset is the deliverable. pawl computes **the minimum set of lines a
human must actually read** before it merges, and emits a signed record of every
assumption the agent made and how each one is verified.

## Layout

```
pawl/
├── AGENTS.md              this file
├── CLAUDE.md              Claude Code specifics for pawl; points here
├── README.md              the public product README
├── Makefile               build, test, cross-compile, checksums
├── go.mod                 module + toolchain pin; no require block
├── cmd/pawl/              main; the only package that is not internal
├── internal/
│   ├── model/             claim schema, reading list, in-toto statement
│   ├── claimlog/          write-once records under .pawl/claims and .pawl/acks
│   ├── gitutil/           subprocess git; no library dependency
│   ├── anchor/            fingerprint relocation against the delivered tree
│   ├── evidence/          junit, cobertura, typecheck, policy, spec ingest
│   ├── resolve/           the product: claims + evidence -> reading list
│   ├── policy/            policy pack v0 and a TOML subset reader
│   ├── attest/            in-toto Statement builder
│   ├── harness/           harness hook adapters and settings install
│   ├── calibrate/         sampling, two-phase review, false-clear rate
│   ├── cli/               the commands
│   └── e2e/               end-to-end against real git repos
├── examples/              policy.toml, pawl-gate.yml
└── _spec/                 specs, in the format the spec tool will use
    ├── constitution.md    BINDING PRINCIPLES — read first
    └── PAWL-00N-*.md      one per unit of work
```

## Commands

Run from this directory. [`.envrc`](./.envrc) puts `./bin` on PATH, so once
you have built, `pawl` means this build.

```bash
make test                           # 59 e2e tests, all must pass
make build                          # ./bin/pawl
make dist                           # 5 platforms + SHA256SUMS
make check                          # fmt + vet + test, what CI runs

pawl claim "…" --path src/x.go --lines 40-58 --verified-by test:TestX
pawl verify --base origin/main --junit junit.xml --coverage coverage.xml
pawl attest --base origin/main --junit junit.xml --out trail.intoto.json
pawl gate   --base origin/main --junit junit.xml    # exit 1 on violation
```

---

## The six decisions that define the design

Do not relitigate these without a reason. Each was reached deliberately and at
least two were paid for with a failing test.

**1. Claims are captured at edit time, never assembled from a diff.**
A model asked at PR time to explain its changeset re-reads the diff and
confabulates. That artefact is worse than nothing because it looks like
evidence. Rejected alternatives are unrecoverable after the fact.

**2. Claims anchor to a content fingerprint, not a line number.**
Line numbers move constantly while an agent works. Claims relocate against the
delivered tree; where the fingerprint cannot be found, that is reported as drift
and the span goes to a human. Failing loud is the difference between a trail and
a ritual.

**3. The verifier never takes the agent's word for anything.**
An asserted `test:…` is a claim that a check exists. pawl decides whether it
exists in the junit output and whether it passed. Absent ≠ failed, and neither
clears. In Go this is `TestPassed() (passed, present bool)` — the distinction is
in the type system rather than in a nullable bool.

**4. Verdicts are computed at line granularity, not hunk granularity.**
The first test suite run failed on a case where one unverified claim dragged its
whole hunk to a human, including lines a verified claim already covered — making
"minimum set a human must read" false. Regression test:
`TestPartialCollapseWithinASingleHunk`.

**5. The attestation subject is the git tree, not a built artifact.**
SLSA v1.2 promoted the Source track to approved and deliberately leaves source
provenance attestations undefined. This predicate occupies that slot. Binding to
an image digest would be the wrong anchor: the changeset is the deliverable and
the build is downstream.

**6. pawl is a single static binary with no dependencies.**
The newest decision, and the one that governs the language. pawl's pitch is
signed evidence about a changeset; a tool that asks a security-conscious client
to install a transitive dependency tree before it can verify anything undermines
its own argument. Go was chosen for the artifact, not for the language: one
~3MB static binary, no module graph to audit, byte-identical rebuilds, and five
platforms cross-compiled from any one machine. See
[the distribution section of the README](./README.md#install).

## The ratio is a commercial number

`must_read_lines / changed_lines` is what goes in front of a client. Anything
inflating the denominator is a **commercial bug, not a cosmetic one**. The first
live demo read 20% collapsed and the reading list was mostly blank lines and the
claim log auditing itself; after excluding `.pawl/` and non-semantic lines the
same changeset read 37.5%.

---

## Roadmap and current position

Phase 1 (build the kit) — **items 1–5 DONE**

1. ✅ Claim schema — `internal/model`
2. ✅ Claim emitter — `internal/claimlog`, `cli` claim
3. ✅ Assembler + verifier — `internal/anchor`, `internal/evidence`, `internal/resolve`
4. ✅ Policy pack v0 — `internal/policy`, `examples/policy.toml`
5. ✅ CI check + annotations — `examples/pawl-gate.yml`, `verify --annotations`
6. ✅ Calibration sampler — `pawl sample`, `pawl review`, `pawl calibrate`.
   Two axes: a binary `correct`/`false_clear` per span, a cause per (span,
   claim) pair. Escalation precision is PAWL-014 and is not built.

Phase 2 (prove it)

7. 🔄 Run pawl on a real repo for a sustained stretch — **started**. `git init` on
   this repo made it possible; pawl now runs on itself and the first claims are
   recorded. It reported 0% collapsed and 143 unclaimed lines on its own
   changeset, which is honest and is the baseline to improve from.
8. ⬜ Capture before/after: human review minutes per merged changeset, escalation
   precision, false-clear rate. **This is the demo.**
9. ⬜ Spec bridge — see [`./_spec/PAWL-009-spec-tool.md`](./_spec/PAWL-009-spec-tool.md)

Phase 4 items that land here: judgement-calls log, policy pack v1, role tagging
analysis, publishing the schema + verifier as Apache-2.0. The engagement-shaped
phases (3, and the commercial half of 4) are tracked in
[the repository AGENTS.md](../AGENTS.md).

## Open decisions blocking work

**PAWL-013 supported versions and backports.** Trunk-based development says fix
forward, but PAWL-013 AC2 makes any verdict-affecting change a MAJOR bump — so
telling a pinned client to upgrade for a security fix also tells them to accept
a change in which changesets pass. Either only the latest version is supported,
or there are maintenance branches and AC13 gives way. **A client's security team
will ask this during procurement**, so it cannot wait past a first engagement.

## Conventions

Repo-wide conventions are in [the repository AGENTS.md](../AGENTS.md). Specific
to pawl:

- **Go, standard library only.** `go.mod` has no `require` block and should not
  grow one. Adding a dependency needs a reason that survives decision 6 above.
  The TOML subset reader in `internal/policy/toml.go` exists precisely so that
  the module graph stays empty.
- **`CGO_ENABLED=0` always.** cgo would link the host libc and break the static
  binary property. `.envrc` sets it; the Makefile sets it; do not unset it.
- **Tests run against real git repositories and real tool output formats. Never
  mock git, diff parsing, or evidence files** (C-9). Every defect found so far
  lived in the seam between git's behaviour and our model of it; a mock would
  have hidden all three.
- **`internal/` for everything except `cmd/pawl`.** Nothing here is a library
  yet. When the schema and verifier are published as Apache-2.0 (Phase 4), that
  is the moment to promote packages out of `internal/`, deliberately.
- The predicate type URL `https://trunion.io/attestations/assumption-trail/v0.1`
  describes the artifact, not the tool. It survived the `factory-kit` → `pawl`
  rename and the Python → Go port deliberately. **Do not version it to the tool.**

## Known gaps — do not present these as solved

- **The sampler exists but the corpus does not.** A false-clear rate over a
  handful of reviewed spans is not a number to quote at anyone. Everything stays
  unmeasured until sustained real use produces samples — which is Phase 2 item 7,
  not a build task.
- **Escalation precision is not built** (PAWL-014). Reporting a false-clear rate
  without it is reporting half a measurement: a tool that escalates everything
  scores perfectly and is worthless.
- **Claiming is still prompted at edit time.** PAWL-008's CLI half is built —
  `pawl ack`, the acknowledgement record, the gate distinction and the ratio —
  but AC1's hook is not, so nothing yet *requires* an agent to account for a
  span. Until it does, the ~85% unclaimed rate measured on this repo stands, and
  `max_unclaimed_lines = 0` keeps pressuring agents to backfill against a
  finished diff, which is the failure C-2 forbids.
- **The turn-boundary binding is built but unverified in a live session.** PAWL-020
  closes the shell-edit gap by binding to the end of a turn rather than to a list
  of editing tools, and every criterion is tested — but whether the harness's
  Stop event actually fires, and puts its reason where an agent acts on it, is an
  assumption about somebody else's product. Confirm by observation before
  relying on it; PAWL-020 AC6 keeps the per-edit binding as the fallback.
- **The `spec:` evidence type cannot resolve, and citing it makes a claim
  permanently unverified.** It requires a signed spec attestation, which
  requires the spec tool — PAWL-009, drafted and not built. Found by dogfooding
  within minutes of starting: three of the first four claims recorded in this
  repo cite `spec:PAWL-011-ACn` and can never clear. The claim log is
  append-only, so those claims stay wrong as a permanent record, which is the
  design behaving correctly. **Until PAWL-009 ships, do not cite `spec:`
  evidence** — an assertion that cannot be resolved is the exact
  asserted-but-missing-check antipattern C-1 exists to refuse.
- **The PAWL-010 AC3 doc-drift check is weaker than the criterion.** Comparing
  flag *names* between `--help` and `docs/reference.md` passes when a whole new
  command reuses existing flag names — `pawl ack` was added and slipped through
  undetected. Any implementation of AC3 in `make check` must compare the command
  set as well as the flag set.
- **The CLI itself is untested.** The e2e suite calls the packages directly and
  never invokes `cmd/pawl`. A flag-parsing bug that made `pawl claim` a no-op
  shipped past all ten tests and was caught by hand. Same hole existed in the
  Python suite.
- Concurrent claim writes rely on `O_APPEND` atomicity and are untested.
- Relocation is O(file length) per claim; wrong for a monorepo-wide sweep.
- Renames show as drift. Intended, but noisy on large refactors.
- Deleted lines produce no span; "why this went away" must be claimed against
  surrounding context.
- Skipped tests are treated as absent, not passing. Strict; will bite on suites
  with legitimate platform skips.
- The TOML reader is a deliberate subset: no nested tables, arrays of tables,
  inline tables, multi-line strings or dates. It rejects rather than misreads,
  but a client with an exotic policy file will hit it.
- Specs in `_spec/` were written *after* the code and are unsigned. They are
  reconstructed intent, not agreed contracts — which is itself the argument for
  PAWL-009.

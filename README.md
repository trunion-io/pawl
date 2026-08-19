# pawl — Provenance of Agent-Written Lines

**Edit-time claim capture and changeset verification for agentic delivery.**

> A pawl is the component in a ratchet that permits motion in one direction and
> prevents reversal. That is what this is: an append-only record of what an agent
> assumed while it worked, and a gate that will not turn backwards.

Part of [trunion](https://trunion.io) — *Agents write. Humans read.*

The changeset is the deliverable. This computes **the minimum set of lines a
human must actually read** before it merges, and emits a signed record of every
assumption the agent made and how each one is verified.

## The model

1. **Claims are emitted at edit time**, from a harness hook, at the moment of the
   change. A claim assembled at PR time is a model re-reading its own diff, which
   is confabulation dressed as evidence. Rejected alternatives in particular are
   unrecoverable afterwards — the diff contains no trace of the path not taken.

2. **Claims anchor to content, not line numbers.** Line numbers move constantly
   while an agent works. Each claim carries a fingerprint of the span it
   describes and is relocated against the delivered tree. Where the fingerprint
   cannot be found, that is reported as drift and the span goes to the human.
   Failing loud is the difference between a trail and a ritual.

3. **The verifier does not take the agent's word for anything.** An agent
   asserting `test:TestExpiry` is asserting that a check exists. The verifier
   decides whether it exists in the junit output and whether it passed. An
   asserted-but-missing test is worse than no assertion, because it looks like
   rigour.

4. **The output is a reading list, not an approval.** Line granularity, so a
   verified span collapses even when an unverified one sits three lines below it.

5. **The client owns the thresholds.** `.pawl/policy.toml` lives in their
   repo. A supplier who writes both the gate and the bar it clears has built
   theatre.

## Install

pawl is **one static binary**. No runtime, no interpreter, no dependency tree —
which matters more here than it would for most tools: pawl's whole pitch is
signed evidence about your changeset, and a supply-chain assurance tool that
arrives with its own supply chain to audit is arguing against itself.

- ~3MB, statically linked, no libc requirement
- linux, macOS and Windows, on amd64 and arm64
- Builds are **reproducible** — rebuild from source and the checksum matches

### From source — works today

```bash
make build          # ./bin/pawl
make check          # fmt + vet + the e2e suite
```

Requires a Go toolchain. Nothing else: `go.mod` has no `require` block, so there
is no module download and no lockfile to review.

### Released binaries — not yet published

The table below is the intended distribution. It is designed and the build side
is wired (`make dist`), but nothing is published yet — until the first tagged
release, build from source.

| Your stack | Install |
|---|---|
| Anything | Download from GitHub Releases, check against `SHA256SUMS`, drop on `PATH` |
| macOS | `brew install trunion/tap/pawl` |
| TypeScript | `npx @trunion/pawl` — per-platform packages via `optionalDependencies`, no postinstall script |
| Python | `uv tool install trunion-pawl` — per-platform wheels wrapping the binary |
| CI, containerised | `ghcr.io/trunion/pawl:<version>` — `scratch` image, ~5MB |

The binary is the artifact in every row. The package managers are only delivery
channels, so a client installs pawl the way they already install everything
else; none of them need to know or care what it is written in.

`make dist` produces the full matrix plus `SHA256SUMS`:

```bash
make dist VERSION=0.1.0
```

### Verifying what you got

Release binaries will be signed with **cosign keyless** off the CI OIDC token —
the same tool and the same flow pawl tells you to use for `pawl attest`, so
verifying pawl itself uses the muscle you are building anyway.

```bash
cosign verify-blob \
  --certificate-identity-regexp 'https://github.com/trunion/pawl/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle pawl-0.1.0-linux-amd64.sigstore.json \
  pawl-0.1.0-linux-amd64
```

**In CI, pin by digest, not by tag.** pawl gates merges, so the version that runs
decides whether a PR passes. Any change to gate behaviour is a major version
bump; treat an unpinned pawl the way you would treat an unpinned linter that can
block a release.

## Use

```bash
# From a harness hook, at the moment of the edit
pawl claim "token.exp is unix seconds in the same clock domain as now" \
  --path src/auth.go --lines 44-58 \
  --kind assumption \
  --verified-by test:TestExpiry \
  --harness claude-code --ticket PROJ-142

# In CI
pawl verify --base origin/main --junit junit.xml --coverage coverage.xml
pawl attest --base origin/main --junit junit.xml --out trail.intoto.json
pawl gate   --base origin/main --junit junit.xml    # exit 1 on violation
```

A worked CI job is in [`examples/pawl-gate.yml`](./examples/pawl-gate.yml).

Example output:

```
8 changed lines, 5 need a human (37.5% collapsed)
4 claims, 2 unresolved, 0 unclaimed lines

READING LIST
  ? src/auth.go:8-10  [unverified]
      assumption: Refresh is a passthrough until rotation lands
        - test not found in results: TestRotation
  ? src/auth.go:12-13  [unverified]
      undetermined: Could not establish whether audit sink tolerates unstructured lines
        - no check asserted and span not exercised
```

pawl reads **language-neutral** CI artifacts — junit XML, Cobertura coverage, git
plumbing. It does not care what your repo is written in.

## Claim kinds

| kind | meaning | escalates? |
|---|---|---|
| `assumption` | taken as true without proof at the point of writing | if unverified |
| `rejected_alternative` | a path considered and not taken | if unverified |
| `undetermined` | could not establish it, proceeded anyway | **always** |
| `constraint` | a requirement traced to a spec criterion | if unverified |

## Evidence types

`test` (junit node id) · `coverage` (line hits) · `typecheck` (file clean) ·
`policy` (named OPA rule) · `spec` (checkable acceptance criterion)

Only criteria marked *checkable* in a signed spec count. An acceptance criterion
that cannot become a mechanical check is a permanent tax on human attention, and
surfacing that during spec review is the point.

## Attestation

`pawl attest` emits an in-toto Statement whose **subject is the git tree**,
not a built artifact. SLSA v1.2 promoted the Source track to approved and
deliberately leaves source provenance attestations undefined, up to whoever
implements them; this predicate occupies that slot. Binding to an image digest
would be the wrong anchor — in agentic delivery the changeset is the deliverable
and the build is downstream of it.

Signing is `cosign attest-blob` with a CI OIDC token. Keyless, no custody.

## Deliberately not here

- **A UI.** Check-run annotations put the reading list where review already happens.
- **An evidence graph.** GUAC exists and is the right home for this once a client
  has enough attestations to query. Not on day one.
- **A canary judge.** Argo Rollouts' `web` provider and Flagger's
  confirm-promotion webhook already carry every metrics backend worth having.
  The arbiter implements one endpoint; it does not write metric providers.
- **A harness.** That layer is commoditised. This is a plugin, and it should ship
  in Agent Plugins layout so it runs under Codex and Cursor too, not just Claude Code.
- **Dependencies.** `go.mod` has no `require` block and should not grow one.

## Known gaps

- **The attestation does not record which pawl produced it.** No tool version or
  binary digest in the predicate, so a signed trail cannot be traced back to the
  verifier that issued it. To be fixed before a first client.
- The CLI is not covered by the test suite; the suite drives the packages
  directly. A flag-parsing bug that made `pawl claim` a no-op once shipped past
  all ten tests.
- Deleted lines produce no span, so "why this went away" has to be claimed
  against surrounding context.
- The relocation scan is O(file length) per claim; fine at PR scale, wrong for a
  10k-file monorepo sweep.
- Whitespace normalisation means a reformat preserves anchors but a rename shows
  as drift. That is intended, but it will generate drift noise on large
  refactors until there is a rename-aware path.
- The policy TOML reader is a deliberate subset — no nested tables, arrays of
  tables, inline tables or dates. It rejects rather than misreads.
- No calibration sampler yet. It is the next thing to build and the only part of
  this a competitor cannot fork.

## Development

pawl lives in the [trunion monorepo](../README.md). Before changing anything
here, read [AGENTS.md](./AGENTS.md) for the design decisions and current
position, and [`./_spec/constitution.md`](./_spec/constitution.md), which is
binding.

```bash
make help      # list targets
make check     # fmt + vet + 10 e2e tests against real git repositories
```

[`.envrc`](./.envrc) (direnv) puts `./bin` on `PATH` and pins `CGO_ENABLED=0`, so
`pawl` means your local build while you are in this directory and nothing once
you leave.

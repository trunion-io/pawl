# PAWL-025 — Security posture of a public repository

**Status:** DRAFTED, NOT BUILT · **Module:** `.github/`, `internal/evidence`
**Extends:** [PAWL-024](./PAWL-024-licensing-and-source-availability.md) (drafted)
— AC1 of that spec publishes the repository. This one states what must be true
because it is public.
**Related:** [PAWL-013](./PAWL-013-versioning-and-release.md) (delivered) — AC12
and AC13 require a green `main` and cutting releases from it; nothing currently
enforces either.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

pawl decides whether other people's changesets can merge, and it will shortly
publish signed binaries that clients install and run in their own pipelines. The
repository becoming public changes who can study it, not what it is worth
attacking: the prize was always the release workflow, because anything it signs
carries pawl's identity and clients are told to trust that identity.

Publication does two things at once. It removes obscurity as a defence, and it
makes the repository's own practices part of the product — a supply-chain
assurance tool whose supply chain is visibly unhardened is an argument against
itself, and the first thing a client's security team will do is look.

Three findings are live in the tree today, in order of severity.

**1. Every GitHub Action is referenced by a mutable tag.**

```
release.yml   actions/checkout@v4   actions/setup-go@v5   sigstore/cosign-installer@v3
ci.yml        actions/checkout@v4   actions/setup-go@v5
```

A tag is a pointer its owner can move. `release.yml` runs with `id-token: write`
and `contents: write`, so code arriving through a repointed tag executes in a job
that can mint an OIDC token and sign arbitrary bytes as pawl. This is not
hypothetical: it is the mechanism of the `tj-actions/changed-files` compromise,
where a moved tag exfiltrated secrets from tens of thousands of repositories.

**2. Nothing enforces the branch rules the delivered spec already requires.**
PAWL-013 AC12 and AC13 state that the check suite must pass before merge and that
every release is cut from `main`. Both are currently honoured by convention.

**3. The most untrusted input has no fuzz coverage.** `internal/evidence` parses
JUnit and Cobertura XML produced by the client's CI. On a fork pull request that
content is attacker-influenced, and it is the only place pawl reads bytes it did
not write.

## Build integrity

**AC1** — The system shall reference every GitHub Action by an immutable commit
identifier rather than by a tag or branch.
`checkable: yes` (once built) — grep the workflows; a `uses:` value that is not a
40-character hex identifier fails. This is the highest-severity item in the spec
and the cheapest to check.

**AC2** — The system shall record, alongside each pinned identifier, the human
-readable version it corresponds to.
`checkable: yes` (once built) — a pinned SHA with no comment is unmaintainable,
and unmaintainable pinning is abandoned pinning.

**AC3** — The system shall grant each workflow job the minimum token permissions
it requires, and shall not grant `id-token` or write permissions to any job that
executes code from a pull request.
`checkable: partially` — the declared permissions are checkable; that a job
executes no untrusted code is a review judgement.

**AC4** — The system shall not use a trigger that grants a fork's code access to
repository secrets.
`checkable: yes` (once built) — `pull_request_target` and
`workflow_run`-with-checkout are the two shapes that do this.

## Vulnerability detection

**AC5** — The system shall check the code and its toolchain for known
vulnerabilities on every pull request and on a schedule, and shall fail the check
when one affecting a reachable path is found.
`checkable: yes` (once built)

> The scheduled half is the half that matters. A vulnerability disclosed against
> the Go standard library after a pull request merges is never seen by a
> pull-request-only trigger, and this repository is unusually exposed to exactly
> that: `go.mod` carries no `require` block, so **every** vulnerability pawl can
> have is a standard library or toolchain vulnerability. Zero dependencies moves
> the risk, it does not remove it.

**AC6** — The system shall perform static security analysis of the Go source on
every pull request and on a schedule.
`checkable: yes` (once built)

**AC7** — The system shall enable secret scanning with push protection on the
repository.
`checkable: partially` — a repository setting, not a property of the tree.
History was scanned clean before publication; this criterion is about what
happens next, and push protection is the control that prevents the mistake rather
than reporting it after the fact.

## Untrusted input

**AC8** — The system shall provide fuzz targets for every parser that reads input
the system did not produce, and shall execute them in continuous integration.
`checkable: yes` (once built) — JUnit XML, Cobertura XML, the typecheck JSON
readers, and the record readers of PAWL-018.

**AC9** — When given malformed, hostile or truncated evidence input, the system
shall fail with a diagnostic and shall not panic.
`checkable: yes` (once built) — a panic in the gate is a denial of the merge
pipeline it was installed to protect. C-3 applies with force here: a parser that
crashes has produced no evidence, and no evidence must never read as coverage.

## Branch and release protection

**AC10** — The system shall prevent merging to `main` unless the full check suite
has passed.
`checkable: partially` — enforcement is a repository setting; that the required
check exists and is named correctly is checkable.

**AC11** — The system shall prevent force-pushing to and deletion of `main`.
`checkable: partially` — repository setting. Trunk-based development makes `main`
the only line of history there is; PAWL-013 AC13 has no meaning if it can be
rewritten.

## Disclosure

**AC12** — The system shall publish a security policy stating how to report a
vulnerability privately and what response to expect.
`checkable: yes` (once built) — a public repository with no disclosure route
converts a researcher who wanted to help into a researcher who opens a public
issue.

## Non-functional

- **Every check must be able to fail the build.** A security workflow that only
  annotates is the failure mode pawl exists to name: a check that is asserted,
  runs, and changes nothing. If a finding is acceptable it is suppressed
  explicitly, with a reason, in the tree.
- **Findings are evidence, and belong in the same place as other evidence.** The
  long-term shape is for these results to be inputs to pawl's own gate rather than
  a parallel system. Not required by this spec; do not design them so this becomes
  impossible.
- **Third-party scanners are dependencies of the pipeline even when they are not
  dependencies of the binary.** AC1 applies to them exactly as to any other
  action, and the argument for zero `require` entries does not extend to CI.
- **Noise is the failure mode of this whole category.** A scanner that reports
  constantly on a repository with no dependencies will be muted within a month,
  and a muted scanner is worse than an absent one because it appears in the
  documentation as a control.

## Out of scope

- **SLSA build provenance for the pawl binary.** Already out of scope in
  PAWL-013 and still deserving its own spec.
- **Runtime hardening of client pipelines.** pawl is a CLI in someone else's job;
  how that job is sandboxed is theirs.
- **Threat model for the attestation predicate.** PAWL-005 and PAWL-011.
- **Vulnerability response process** — triage times, embargo, advisories beyond
  the reporting route required by AC12.
- **Signing of the checksum file.** PAWL-024 AC7 covers it.

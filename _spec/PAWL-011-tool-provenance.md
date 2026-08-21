# PAWL-011 — Tool provenance in the attestation

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/attest`
**Extends:** [PAWL-005](./PAWL-005-attestation.md)

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |

## Relationship to PAWL-005

PAWL-005 is delivered and is not modified by this spec. All of its criteria
continue to hold:

| PAWL-005 | Status under this spec |
|---|---|
| AC1 — subject is the git tree and commit | unchanged |
| AC2 — predicate type `…/assumption-trail/v0.1` | unchanged, see AC4 below |
| AC3 — per-claim records | unchanged |
| AC4 — author role breakdown | unchanged |
| AC5 — the system does not sign | unchanged |

This spec adds criteria about the **producer** of a statement. PAWL-005 says
what is recorded about the changeset; nothing in it says anything about what
produced the record.

## Context

pawl emits signed evidence that a client's security team is expected to trust.
An auditor holding a trail today can verify *who signed it* and *which tree it
describes*, but not **which verifier issued it**.

That gap matters because pawl's verdicts change between versions. A trail
produced by a permissive older build and one produced by the current build are
indistinguishable once signed, so "these lines were cleared" cannot be
interpreted without knowing what cleared them. SLSA provenance identifies the
builder for exactly this reason; here the thing building the assumption trail is
pawl itself, and it is currently anonymous.

This is the most significant hole in the schema and it should close before a
first client, not after.

## Acceptance criteria

**AC1** — When emitting a Statement, the system shall record in the predicate a
`tool` object containing the tool name and the version of the running binary.
`checkable: yes` (once built)

**AC2** — The system shall record in `tool` the SHA-256 digest of the running
binary.
`checkable: yes` (once built)

**AC3** — If the digest of the running binary cannot be determined, then the
system shall omit the digest field and still record name and version.
`checkable: yes` (once built) — a placeholder or zero digest is worse than an
absent one, because it looks like an answer.

**AC4** — The system shall report in `tool.version` the same string that
`pawl version` reports.
`checkable: yes` (once built) — two version sources that can disagree is a
defect waiting to happen.

**AC5** — The system shall leave the predicate type URL unchanged and shall
raise `schemaVersion` to `0.2`.
`checkable: yes` (once built)

**AC6** — The system shall record `tool` regardless of whether the binary was
built from a tagged release, so that a development build is identifiable as one.
`checkable: yes` (once built) — an unattributable trail from a local build is a
worse outcome than an obviously-untrusted one.

### Version identity

A recorded version is only provenance if it names something real. These criteria
exist because a trail asserting `pawl 0.1.0` is worthless if any tree can be
built and labelled `0.1.0`.

**AC7** — The system shall derive the version it reports, the version it records
in the attestation, and the version naming the published artifact from a single
source.
`checkable: yes` (once built) — three version strings that can be set
independently will eventually disagree, and the one in the signed trail is the
one nobody can correct afterwards.

**AC8** — If a binary is built from a working tree that does not correspond
exactly to a release tag, then the version it reports shall identify it as such.
`checkable: yes` (once built) — `git describe --dirty` already distinguishes a
tagged build, a commit past a tag, and a modified tree. Nothing may discard that
distinction on the way to the predicate.

**AC9** — The system shall not publish an artifact whose declared version differs
from the version reported by the binary inside it.
`checkable: yes` (once built) — executing `pawl version` on each built artifact
and comparing it against that artifact's own name is a cheap release check, and
the only one that closes the loop end to end.

## Non-functional

- **Predicate stability.** PAWL-005's NFR states that the type URL describes the
  artifact and not the tool, and that a tool rename must not change it. That
  still holds and does not conflict with this spec: the URL names the predicate
  **format**, `tool` names the **producer of one instance**. A second
  implementation emitting this predicate would use the same URL and a different
  `tool.name`. Do not resolve the apparent tension by deleting either.
- **Additive change.** Adding a field does not break a consumer that ignores
  unknown fields, which is why the type URL does not move. `schemaVersion` rises
  so that a consumer *can* tell the difference if it wants to.
- **Digest source.** Hashing `os.Executable()` at attest time reflects the
  binary actually running, which is the honest answer, but costs I/O and fails
  where the executable is unreadable. Injecting a digest at build time cannot
  observe tampering. AC3 exists because the honest source is the one that can
  fail.
- **Known hazard in the current build.** `make dist VERSION=0.1.0` accepts an
  arbitrary override, so today a modified untagged tree can produce artifacts
  named `pawl-0.1.0-…` whose binaries would attest to a version matching no
  release. AC7–AC9 exist specifically to close that, and closing it means the
  release path stops trusting an operator-supplied version string.
- **Publication is part of the trust chain.** The version in a signed trail is
  only useful if a reader can fetch the artifact it names, verify its signature,
  and get a binary reporting the same version. Any link that can be set by hand
  breaks the chain silently, because nothing downstream can detect it.

## Out of scope

- **Signing.** PAWL-005 AC5 stands: `cosign attest-blob` with a CI OIDC token.
- **Recording the harness or model.** Already captured per claim, where it
  belongs — that is provenance of the *edit*, not of the verification.
- **Verifying the tool against a published checksum.** pawl attesting to its own
  integrity proves nothing; a tampered binary would attest to whatever it liked.
  The client verifies the binary at install time — see `docs/install.md`.
- **A version-compatibility policy for consumers.** What a downstream verifier
  should do when it meets `schemaVersion` it does not recognise is a separate
  question and needs its own spec.

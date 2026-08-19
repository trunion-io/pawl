# PAWL-013 — Versioning and release

**Status:** DRAFTED, NOT BUILT · **Module:** `.github/workflows/` (does not exist)
**Related:** [PAWL-011](./PAWL-011-tool-provenance.md) (drafted) — that spec
requires version identity; this one defines how the pipeline achieves and
enforces it.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |
| NFR (SRE) | *unsigned* |

## Context

pawl is distributed as a binary that clients pin in CI, and it **decides whether
their changesets can merge**. That makes versioning a product concern rather than
a packaging detail: a client who bumps pawl and finds their pipeline blocking
changesets it passed yesterday has been handed an outage by their supplier.

Development is **trunk-based on `main`**. Short-lived branches, merged fast, no
release branches, every commit on `main` releasable. Releases are cut by tagging
a commit on `main`.

Two hazards this closes, both live today:

1. `make dist VERSION=0.1.0` accepts an arbitrary operator-supplied version, so a
   modified untagged tree can produce artifacts named as a release that never
   existed. Their attestations would name a version nobody can fetch.
2. There is no rule for what a version bump *means*, so nothing stops a
   verdict-changing fix shipping as a patch.

## Versioning

**AC1** — The system shall version releases as SemVer 2.0.0, tagged
`vMAJOR.MINOR.PATCH` on a commit reachable from `main`.
`checkable: yes` (once built)

**AC2** — The system shall treat any change that can alter a gate verdict, for an
unchanged changeset against an unchanged policy, as a MAJOR change — **including
a bug fix**.
`checkable: partially` — the classification is a review judgement; that a MAJOR
tag accompanies a change to the resolver or policy defaults can be checked.

> This is the criterion that makes pawl's versioning unusual, and it is
> deliberate. Everywhere else "breaking" means the API changed. Here a client's
> contract with pawl is *which changesets pass*, so a stricter default, a new
> violation rule, or a corrected line count breaks that contract even though
> every flag still works. The hunk-granularity fix in Phase 1 would have been a
> MAJOR release under this rule, and correctly so — clients would have seen
> changesets start clearing that did not clear before.

**AC3** — Where a change adds a command, a flag, an evidence type, or an
additive predicate field without altering any verdict, the system shall release
it as MINOR.
`checkable: partially`

**AC4** — The system shall version the attestation predicate schema
independently of the tool version.
`checkable: yes` — PAWL-005's NFR and PAWL-011 AC5. A consumer keys on
`predicateType` and `schemaVersion`, never on the tool version.

**AC5** — While the major version is `0`, the system shall apply AC2 and AC3 to
the MINOR and PATCH positions respectively, rather than treating pre-1.0 as
unconstrained.
`checkable: partially` — SemVer permits anything below 1.0. A tool that can block
a client's release does not get to use that latitude.

## Release

**AC6** — The system shall derive the release version solely from the git tag,
and shall not accept an operator-supplied version.
`checkable: yes` (once built) — closes hazard 1 above. A human-typed version in
the release path is the one link in the provenance chain nothing downstream can
detect as wrong.

**AC7** — If the tagged commit is not reachable from `main`, or the working tree
at build time is not clean, then the release shall fail.
`checkable: yes` (once built)

**AC8** — When publishing, the system shall produce for every supported platform
a binary, a checksum file covering all binaries, and a signature for each
artifact.
`checkable: yes` (once built)

**AC9** — Before publishing, the system shall execute each built artifact and
verify that the version it reports matches the version named in that artifact's
filename.
`checkable: yes` (once built) — implements PAWL-011 AC9. Cheap, and the only
check that closes the loop end to end.

**AC10** — The system shall sign release artifacts using keyless signing bound to
the release workflow's OIDC identity.
`checkable: yes` (once built) — the identity a client verifies against is the
workflow, not a person. See `docs/install.md`, whose verification command depends
on that identity remaining stable.

**AC11** — The system shall publish only artifacts that a third party can
reproduce byte-for-byte by building the same tag.
`checkable: yes` — `-trimpath` and `CGO_ENABLED=0` already achieve this; the
criterion exists so a change that breaks it is a release blocker rather than a
curiosity.

## Trunk-based development

**AC12** — The system shall require the full check suite — format, vet, tests,
and the documentation checks of PAWL-010 — to pass on a pull request before it
may merge to `main`.
`checkable: yes` (once built)

**AC13** — The system shall not use release branches, and shall cut every release
from a commit on `main`.
`checkable: yes` (once built) — AC7 enforces the second half.

## Non-functional

- **No manual step in the release path.** Every value in a published artifact
  derives from the tag and the tree. An operator typing a version, uploading a
  binary by hand, or signing locally reintroduces exactly the gap AC6 closes.
- **The client is the auditor.** A client must be able to take a published
  binary, verify its signature, rebuild the tag, and match the checksum, without
  asking us for anything. AC11 exists for that reader, not for us.
- **Trunk-based has a cost, and it lands here.** No release branches means no
  obvious place to fix an old version. AC2 makes MAJOR bumps frequent, which
  makes the supported-version question sharper rather than softer — see the open
  decision below.
- **Green main is the precondition, not an aspiration.** Trunk-based development
  with a tool that gates other people's releases cannot tolerate a red `main`;
  there is no branch to cut from instead.

## Open decision — needs Rich

**Which versions are supported, and is there a backport path?**

Trunk-based development says fix forward. But clients pin pawl deliberately (see
`docs/install.md`), and AC2 means verdict-affecting fixes force a MAJOR bump — so
"upgrade to get the fix" can mean "accept a change in which changesets pass",
which is precisely what a pinning client was avoiding.

The two coherent answers:

1. **Latest only.** Simple, honest, and consistent with trunk-based. A client on
   an old major who needs a security fix takes the verdict changes with it.
2. **Supported majors with backports.** Requires maintenance branches, which
   contradicts AC13 and adds a release matrix.

This cannot be deferred past a first engagement, because it is a question a
client's security team will ask during procurement, and "we hadn't decided" is a
bad answer to give at that point.

## Out of scope

- **Package-manager publishing** (Homebrew tap, npm, PyPI wheels, ghcr). The
  distribution channels in `docs/install.md` consume release artifacts; wiring
  each one is a separate unit of work with its own failure modes.
- **Release notes and changelog generation.**
- **SLSA build provenance for the pawl binary itself.** Distinct from the
  assumption-trail predicate, worth having, and its own spec.
- **Signing key custody.** There is none, by design — AC10 is keyless.
- **Versioning of other trunion products.** This spec binds pawl only, though the
  reasoning in AC2 will transfer to anything else that gates a client's pipeline.

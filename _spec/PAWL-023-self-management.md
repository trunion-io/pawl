# PAWL-023 — Verifying and upgrading the installed binary

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/selfmanage`, `internal/cli`
**Related:** [PAWL-011](./PAWL-011-tool-provenance.md) already computes the
running binary's digest; [PAWL-013](./PAWL-013-versioning-and-release.md)
publishes the checksums this checks against;
[PAWL-019](./PAWL-019-harness-installation.md) AC21 repairs a hook entry whose
command has changed.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

`install.sh` verifies a download against the published checksums. Nothing checks
the binary again afterwards, and nothing helps a user move to a new version
without repeating the install by hand.

Both matter more for pawl than they would for most tools. pawl decides what
merges, and its attestations name the binary that produced them (PAWL-011). A
user who cannot answer *"is the pawl I am running the one that was published?"*
cannot act on the digest in their own trails.

### On naming

The commands are `pawl install verify` and `pawl install upgrade [version]`.

The first shape considered was `pawl verify install`, and it does not survive
contact with the existing CLI: `pawl verify` already means *resolve claims
against evidence*, takes no positional argument, and today

```
$ pawl verify install
```

runs the changeset verifier and ignores the word `install` entirely. A
subcommand there would make a typo silently do something else — the failure mode
this tool exists to refuse.

`install` as a noun with verbs under it overloads nothing, reads correctly, and
leaves room for whatever else the installation needs later.

## Verifying the installation

**AC1** — The system shall report whether the running binary matches the
checksum published for its version.
`checkable: yes` (once built)

**AC2** — Where the published checksums cannot be fetched, the system shall say
so and shall not report the binary as verified.
`checkable: yes` (once built) — an unreachable network is not evidence of
authenticity. Reporting "ok" because a check could not run is the
asserted-but-missing-check failure C-1 exists to refuse, turned on ourselves.

**AC3** — Where the running binary reports a development version, the system
shall report it as unverifiable rather than as failed.
`checkable: yes` (once built) — a locally built binary has no published
checksum and never will. Calling that a failure trains people to ignore the
output.

**AC4** — The system shall report, in one place, the binary's authenticity, the
harness hook's installation and whether the hook's command can run.
`checkable: yes` (once built) — these are the three questions asked when
something is not working, and answering them separately means a user must
already know which one to ask.

## Upgrading

**AC5** — The system shall install a named version, defaulting to the latest
release.
`checkable: yes` (once built)

**AC6** — The system shall verify a downloaded binary against the published
checksum before replacing anything, and shall abort on a mismatch leaving the
existing binary in place.
`checkable: yes` (once built) — **the criterion the command turns on.** A tool
that can replace itself is a tool that can be made to replace itself with
something else. Verification is the whole of what makes self-upgrade acceptable
in a tool that decides what merges.

**AC7** — The system shall replace the binary atomically, leaving a working
binary at the path whether the upgrade succeeds or fails.
`checkable: yes` (once built) — a half-written binary on the PATH is worse than
an old one.

**AC8** — Where the binary cannot be replaced for want of permission, the system
shall say so and name the path, rather than failing obscurely.
`checkable: yes` (once built)

**AC9** — When the upgrade completes, the system shall repair any harness
configuration naming the old binary.
`checkable: yes` (once built) — PAWL-019 AC18 installs an absolute path, so an
upgrade that moved the binary would leave a hook pointing at nothing, failing
silently on every edit. That silence has already cost time here once.

**AC10** — Where it appears to be running in CI, the system shall refuse to
upgrade unless explicitly forced.
`checkable: yes` (once built) — PAWL-013 tells clients to pin by digest because
the version that runs decides whether their changesets merge. A CI job that
upgrades itself has silently unpinned the thing they pinned.

## Non-functional

- **A downgrade is an upgrade.** `pawl upgrade 0.1.0` from 0.2.0 installs 0.1.0.
  The name is conventional rather than accurate, and the command should not
  refuse — pinning backwards is a legitimate thing to want, especially given
  PAWL-013 AC2 makes verdict-affecting changes MAJOR.
- **Self-upgrade is a supply-chain surface, and is offered anyway.** The
  alternative is users on stale binaries, which is worse. AC6 is what makes the
  trade defensible; if it is ever weakened, the command should be removed rather
  than shipped unverified.

## Out of scope

- **Signature verification in `upgrade`.** Checksums come from the same release
  the binary does, so they prove integrity, not provenance. Verifying the cosign
  signature would prove more, and would mean shipping or requiring cosign. The
  documentation already shows the manual path; revisit if a client asks for it,
  and do not pretend the checksum is a signature in the meantime.
- **Automatic background upgrades.** A gate that changes itself without being
  asked is a gate nobody can reason about.

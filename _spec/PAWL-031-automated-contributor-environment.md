# PAWL-031 — Environment for an automated contributor

**Status:** DRAFTED, NOT BUILT · **Module:** `.github/workflows/copilot-setup-steps.yml`
**Extends:** [PAWL-030](./PAWL-030-review-skill.md) (drafted) — that spec equips
an agent that *reads* a change. This one equips an agent that *makes* one.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

An automated contributor that cannot run the check suite can only assert that its
change works. That is the failure this product exists to refuse, arriving through
the front door: C-1 requires evidence produced rather than asserted, and an agent
with no way to execute a test has nothing but assertion available to it.

The cost of removing that limitation is unusually low here, because of decisions
already taken for other reasons:

| | |
|---|---|
| Dependencies to resolve | **none** — `go.mod` has no `require` block |
| Network needed by the suite | **none** |
| Full gate from a cold cache | **7.5 seconds** |

A verification loop that fast, with nothing to download, means there is no
performance argument for letting an agent skip it, and no supply-chain argument
either — there is no dependency graph to compromise.

This is deliberately **not** a devcontainer. A devcontainer serves Codespaces and
editors, which is a question about humans. What an automated contributor reads is
its own product's mechanism, and that mechanism is recorded here rather than
assumed to be stable.

## Criteria

**AC1** — The system shall provide an environment definition that installs
everything required to run the full check suite.
`checkable: yes` (once built)

**AC2** — The environment shall obtain the toolchain version the repository pins,
rather than naming a version separately.
`checkable: yes` (once built) — a second place naming the Go version is a second
place for it to go stale, and the reproducible-build property depends on the
toolchain being the pinned one.

**AC3** — The environment shall grant no credentials and no write scope.
`checkable: yes` (once built) — an agent's output is reviewed as a pull request
like anyone else's. An environment that could push, publish or sign would let it
bypass the gates that make its work reviewable, and PAWL-025 AC3 already refuses
`id-token` or write permissions to any job running code from a pull request.

**AC4** — The environment shall run the same suite CI runs, and shall not define a
reduced one.
`checkable: yes` (once built) — a subset would let an agent report green against
a weaker bar than the pull request will be held to, which is worse than no
environment at all: it produces evidence for the wrong claim.

**AC5** — Where the environment definition changes, the system shall verify it
still succeeds before that change merges.
`checkable: yes` (once built) — an environment definition is only known to work
when it has run. Left untriggered it rots silently and fails first at the moment
an agent is trying to use it.

## Non-functional

- **This grants capability, not authority.** Nothing here lets an automated
  contributor merge anything. The ruleset, the required checks and the review all
  apply to its pull requests unchanged.
- **Stated as unverified until observed.** Whether the automated contributor is
  enabled on this repository, and whether it reads this file, has not been seen.
  PAWL-030 records the same about the review skill. Confirm by observation; do not
  record it as working because it was written.
- **The cheapness is a consequence, not a coincidence.** No dependency graph and
  no network come from decisions taken for the distribution's sake. If either ever
  changes, this file's cost changes with it and the trade should be re-examined.

## Out of scope

- **Devcontainers and Codespaces.** A question about humans, and unrelated to
  what an automated contributor reads.
- **Enabling or configuring the automated contributor itself**, which is a
  repository setting rather than a property of the tree — the same boundary
  PAWL-025 draws around AC7, AC10 and AC11.
- **Granting an agent any ability to release.** PAWL-013 and PAWL-027 own the
  release path, and nothing in it is delegable.

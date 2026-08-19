# PAWL-006 — Policy gate

**Status:** delivered · **Module:** `pawl.policy`

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (SRE) | *unsigned* |

## Context

The thresholds belong to the client. We bring the mechanism and a defensible
default; the team that owns the service sets the bar.

## Acceptance criteria

**AC1** — The system shall read thresholds from `.pawl/policy.toml` in the
target repository and shall fall back to defaults where absent.
`checkable: partially` — `policy.load_policy`; no test loads a file.

**AC2** — The system shall fail a changeset exceeding the configured line
budget.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestGateBlocksOversizedChangeset`

**AC3** — The system shall fail a changeset where must-read lines exceed the
configured ratio.
`checkable: partially` — implemented; observed in live demo, no dedicated test.

**AC4** — The system shall fail a changeset containing unclaimed changed lines
above the configured limit, defaulting to zero.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestUnclaimedChangeNeverClears`

**AC5** — Where sensitive paths are configured, the system shall require a named
check on claims touching them and shall not accept implicit coverage.
`checkable: partially` — implemented; no test.

**AC6** — The system shall exit non-zero on violation and shall emit the
attestation regardless, so a blocked changeset still produces evidence.
`checkable: partially` — exit code implemented; workflow ordering enforces the
rest.

## Non-functional

- **Size as a gate, not a style note.** Comprehension has a hard ceiling
  regardless of trail quality, and claims degrade as the agent's own context
  fills. Decomposition is a delivery requirement on the pod.

## Out of scope

- Rego. When a client needs rules this shape cannot express, migrate to an OPA
  bundle — the split is deliberate.

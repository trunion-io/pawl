# PAWL-005 — Attestation

**Status:** delivered · **Module:** `pawl.attest`

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

SLSA v1.2 promoted the Source track to approved and deliberately leaves source
provenance attestations undefined, up to whoever implements them. This predicate
occupies that slot.

## Acceptance criteria

**AC1** — The system shall emit an in-toto Statement v1 whose subject is the git
tree and commit, not a built artifact.
`checkable: yes` → `test:tests.test_e2e.test_attestation_shape`

**AC2** — The system shall use predicate type
`https://trunion.io/attestations/assumption-trail/v0.1`.
`checkable: yes` → `test:tests.test_e2e.test_attestation_shape`

**AC3** — The system shall record, per claim, the asserted checks, the resolved
coverage status, the anchor status and the human-attention verdict.
`checkable: yes` → `test:tests.test_e2e.test_attestation_shape`

**AC4** — The system shall record a breakdown of claims by author role.
`checkable: yes` → `test:tests.test_e2e.test_attestation_shape`

**AC5** — The system shall not sign. Signing is `cosign attest-blob` with a CI
OIDC token.
`checkable: no` — absence of a feature. Deliberate: keyless signing means no key
custody to negotiate with a client's security team.

## Non-functional

- **Predicate stability** — the type URL describes the artifact, not the tool. A
  tool rename must not change it. It survived the `factory-kit` → `pawl` rename
  for exactly this reason.

## Out of scope

- An attestation store. Archivista is purpose-built for in-toto attestations and
  builds a queryable graph from predicate types and subjects; GUAC sits at the
  aggregation layer. Neither belongs on day one.

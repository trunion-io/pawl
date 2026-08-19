# PAWL-003 — Coverage resolution

**Status:** delivered · **Module:** `pawl.evidence`, `pawl.resolve`

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

The verifier decides whether an asserted check exists and passed. The agent
never gets to establish its own coverage.

## Acceptance criteria

**AC1** — When a claim asserts a test, the system shall resolve it against real
junit output and shall treat an absent test as unverified, distinctly from a
failing one.
`checkable: yes` → `test:tests.test_e2e.test_asserted_test_that_does_not_exist_is_not_coverage`

**AC2** — When a claim asserts no check, the system shall grant implicit
coverage only if every line of the anchored span is exercised by the suite.
`checkable: yes` → `test:tests.test_e2e.test_implicit_coverage_from_line_hits`

**AC3** — The system shall treat a skipped test as absent, not as passing.
`checkable: partially` — implemented in `evidence.load_junit`; no dedicated test.
Strict, and it will bite on suites with legitimate platform skips.

**AC4** — When a claim of kind `undetermined` is resolved, the system shall
escalate it to a human whatever the coverage outcome.
`checkable: yes` → `test:tests.test_e2e.test_undetermined_always_escalates`

**AC5** — Where a spec attestation is supplied, the system shall accept a
criterion as evidence only if that criterion is marked checkable.
`checkable: partially` — implemented in `evidence.load_spec`; untested pending
PAWL-009 (spec tool).

## Non-functional

- **Evidence formats** — junit XML, Cobertura coverage XML, mypy/tsc JSON, OPA
  decision JSON. Consume what the client's pipeline already emits; never replace
  the pipeline.

## Out of scope

- Running any check. pawl reads reports; CI runs tools.
- Semantic judgement of whether a test actually tests the claim. That is a
  human's job and is precisely what the reading list is for.

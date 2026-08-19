# PAWL-009 — Spec bridge

**Status:** DRAFTED, NOT BUILT · **Module:** `pawl-spec` (does not exist)

## Context

In a product, a spec authoring tool is a weak feature in a crowded category. In
an engagement it is the **contract boundary**, and it is what makes everything
downstream mean anything: an arbiter run against criteria agreed *after* the work
is just the supplier asserting its own tests pass.

Deliberately thin. No UI. A markdown file, a CLI, and a comment written back to
the tracker.

## Draft acceptance criteria

**AC1** — When given a tracker issue key, the system shall scaffold
`spec/<project>/<KEY>-<slug>.md` in the target repository with EARS-shaped
acceptance criteria and named NFR stakeholder sections.
`checkable: yes` (once built)

**AC2** — The system shall mark each acceptance criterion as checkable or not
checkable.
`checkable: yes`

**AC3** — When a named stakeholder signs, the system shall produce a spec
attestation bound to the issue key using Sigstore keyless signing against the
stakeholder's SSO identity.
`checkable: partially`

**AC4** — The system shall write one comment back to the tracker containing the
spec link and current signature state.
`checkable: no` — integration surface.

**AC5** — The system shall refuse to mark a spec agreed while any required
stakeholder signature is absent.
`checkable: yes`

## Why AC2 is the whole thing

Any criterion that cannot become a mechanical check is a permanent tax on human
attention — on this changeset and every future one touching the same behaviour.
Surfacing that during spec review is where the SRE and QA stakeholders earn
their seat, and it is a conversation no client is currently having.

## Sequencing note

Deliberately last of the built components. Run it only once real engagements
show which fields stakeholders actually fill in. Spec Kit (93k stars, 30+ agents)
covers the developer-CLI half already; the gap is stakeholder legibility and the
signed attestation, not authoring.

## Out of scope

- Spec-as-source-of-truth. The consensus position is spec-*anchored*: criteria in
  EARS, code as source of truth, tests as enforcer.
- Competing with Spec Kit or Kiro on authoring ergonomics.

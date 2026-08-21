# PAWL-004 — Reading list

**Status:** delivered · **Module:** `pawl.resolve`, `pawl.cli`

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (QA) | *unsigned* |

## Context

The product is this output. The gate is not an approval engine; it computes the
minimum set of lines a human must actually read and asserts the rest is
mechanically covered.

## Acceptance criteria

**AC1** — The system shall compute verdicts at line granularity and shall
collapse a verified span even when an unverified span sits within the same diff
hunk.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestPartialCollapseWithinASingleHunk`

**AC2** — The system shall assign `unclaimed` to any changed line no claim
covers, and shall never clear it.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestUnclaimedChangeNeverClears`

**AC3** — Where a line is covered by both a clearing claim and one needing a
human, the system shall treat it as needing a human.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestPartialCollapseWithinASingleHunk`

**AC4** — The system shall exclude its own claim log from the changeset it
analyses.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestClaimLogIsExcludedFromTheChangeset`
**Found in live demo, not by the suite.**

**AC5** — The system shall exclude blank lines and bare delimiters from the
reading list, and shall treat comments as reviewable.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestNonSemanticLinesAreExcludedButCommentsAreNot`
Comments cost ratio deliberately: agents write plenty of wrong ones.

**AC6** — The system shall report changed lines, must-read lines and the
resulting reduction percentage.
`checkable: yes` → `ReadingList.summary`

## Non-functional

- **The ratio is a commercial number.** Anything inflating the denominator is a
  commercial bug, not a cosmetic one. AC4 and AC5 exist because the first live
  demo read 20% collapsed and was mostly blank lines and self-audit noise; the
  same changeset then read 37.5%.

## Out of scope

- A user interface. GitHub check-run annotations put the list where review
  already happens.

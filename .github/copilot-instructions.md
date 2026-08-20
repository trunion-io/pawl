# Instructions for automated review

The review rules for this repository live in
[`.github/skills/code-review/SKILL.md`](./skills/code-review/SKILL.md). **Read it
before commenting on a change.** It is the single source; this file is a pointer,
so that the two cannot drift.

Two things worth carrying even if that file is not loaded:

**This repository exists to refuse assertions made without the evidence to
support them.** A comment, document or spec claiming a property the code does not
deliver is a finding, even when the code itself is correct.

**Several things here look like defects and are deliberate** — the hand-rolled
TOML subset, everything living under `internal/`, comments counting as reviewable
lines, a skipped test treated as absent. The skill lists them. Reporting one is a
finding against the review, because a confident wrong finding costs attention on
every pull request until the output stops being read.

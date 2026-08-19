# PAWL-012 — Configuration

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/config` (does not exist)
**Extends:** [PAWL-006](./PAWL-006-policy-gate.md) (delivered, immutable)

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |

## Relationship to PAWL-006

PAWL-006 is delivered and is not modified by this spec. It defines the policy
pack: the thresholds a changeset must clear, read from `.pawl/policy.toml`, owned
by the client (C-5).

This spec defines a **separate** mechanism for invocation settings. The boundary
between them is the point of the spec, not an implementation detail — see AC4,
which exists to keep PAWL-006's guarantee intact.

## Context

Every pawl invocation currently passes its full configuration as flags:

```bash
pawl verify --base origin/main --junit junit.xml --coverage coverage.xml \
  --typecheck typecheck.json --strip-prefix /build/
```

Repeated across four commands, in every CI job, and on every `pawl claim` from a
harness hook, this is friction in exactly the place C-6 cares about: the kit has
to install into a client repo in under a day, and a tool that needs six flags
memorised at every call site does not.

The harness case is the sharper one. Under PAWL-008, a hook firing on every edit
would repeat `--harness claude-code --model … --session …` on every single
claim, all of which are properties of the environment rather than of the claim.

There is already a config file — `.pawl/policy.toml` — and the tempting move is
to put everything in it. That would be a mistake, and AC4 is the reason.

## Acceptance criteria

**AC1** — Where `.pawl/config.json` exists, the system shall read invocation
defaults from it.
`checkable: yes` (once built)

**AC2** — The system shall resolve each setting in the order: command-line flag,
then environment variable, then config file, then built-in default.
`checkable: yes` (once built)

**AC3** — The system shall read environment variables named `PAWL_<SETTING>`.
`checkable: yes` (once built)

**AC4** — The system shall not read any gate threshold from the config file or
from the environment.
`checkable: yes` (once built) — **this is the load-bearing criterion.** If a
threshold could be set by an environment variable, any job able to set one could
weaken the gate without touching a reviewed file, and C-5 would be decorative.
Thresholds come only from `.pawl/policy.toml`, where a change to them appears in
a diff.

**AC5** — If the config file contains a key the system does not recognise, then
the system shall exit with an error naming that key.
`checkable: yes` (once built) — a silently ignored typo means believing you
configured something you did not.

**AC6** — Where no config file and no environment variables are present, the
system shall behave exactly as it does today.
`checkable: yes` (once built) — the existing suite passing unchanged is the
check.

**AC7** — The system shall, on request, report every resolved setting together
with the source it came from.
`checkable: yes` (once built) — four-level precedence is not debuggable by
reading the code at 2am in someone else's repository.

**AC8** — The system shall read configuration only from within the repository
under inspection.
`checkable: yes` (once built) — see the out-of-scope note on user-level config.

## Non-functional

- **Format: JSON, not TOML.** Two formats in one tool needs justifying, and the
  justification is that they are different kinds of file. `policy.toml` is
  human-authored, comment-heavy and reviewed — TOML earns its place because the
  comments explaining each threshold are load-bearing. `config.json` is plumbing,
  frequently machine-written (a harness hook, a future `pawl init`), and needs no
  comments. JSON is complete in the standard library; the TOML reader is a
  hand-rolled subset and a known liability, and widening its surface is the wrong
  direction.
- **Precedence must be boring.** Flag beats environment beats file beats default,
  with no exceptions, no merging of arrays across levels, no partial overrides. A
  clever precedence rule is one nobody can predict under pressure.
- **No new dependencies.** `encoding/json` covers this entirely.

## Out of scope

- **Gate thresholds.** PAWL-006, and AC4 above.
- **User-level or machine-level config** (`~/.pawl/`, `/etc/pawl/`).
  Deliberately excluded, permanently. Configuration outside the repository is
  invisible to review and makes a run irreproducible from a checkout: two
  engineers on the same commit would get different reading lists and neither
  could see why. AC8 enforces this.
- **Secrets.** pawl reads no credentials and makes no network calls. If that ever
  changes it needs its own spec, and the answer will not be a file in the repo.
- **Per-command config sections.** One flat set of settings. Sectioning by
  command invites the same setting meaning different things in different
  sections.
- **Config for the claim log location.** `.pawl/claims.jsonl` is fixed. CI has to
  find it without being told, and a relocatable claim log is a claim log that can
  be quietly pointed somewhere empty.

# PAWL-010 — Operator documentation

**Status:** DRAFTED · **Module:** `docs/`, `internal/cli`

> **Process note.** This spec was written *after* the documentation it describes.
> The reasoning at the time — "documentation is not behaviour, so it does not
> need a criterion" — was not a process shortcut but a **category error**.
>
> For a CLI tool the documentation is an *output*, exactly as the binary is.
> `pawl --help` is program output. `docs/reference.md` documents the same
> contract in another medium. When the two disagree, that is a defect in the
> delivered product, not an untidy repository — and it is a defect the user hits
> before they hit any bug in the resolver.
>
> The criteria below are written on that understanding, which is why several of
> them are mechanically checkable where a "docs are prose" framing would have
> left them as review judgement. Treat the existing `docs/` tree as unverified
> against them.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Support) | *unsigned* |

## Context

pawl gates merges in a repository the factory does not own, operated by
engineers the factory will eventually leave. When the pod goes, the
documentation is what remains — the codebase is a leave-behind artefact and so
is everything written about it.

`README.md` had grown to 216 lines serving two audiences badly: a landing page
for someone deciding whether to adopt pawl, and a manual for someone already
living with it. `AGENTS.md` and `_spec/` serve a third audience — whoever is
building pawl — and had been absorbing operator concerns by default.

The failure this prevents is specific and expensive: a gate blocks a release at
2am and the on-call engineer has nothing that tells them whether pawl is broken
or their changeset is.

The failure the **drift criteria** prevent is quieter and more common: a flag is
renamed, `--help` updates because it is generated from the code, the
documentation does not, and the client's runbook is wrong in a way nobody
notices until it matters.

## Acceptance criteria

**AC1** — The documentation shall cover, as separately addressable pages,
installation, configuration, CI integration, command reference, and attestation.
`checkable: yes` — file existence under `docs/`.

**AC2** — Every relative link in the documentation shall resolve to a file that
exists.
`checkable: yes` — link check over `**/*.md`.

**AC3** — Every flag the CLI accepts shall appear in the command reference, and
every flag the command reference documents shall be accepted by the CLI.
`checkable: yes` — diff the flag set parsed from `--help` against the flags named
in `docs/reference.md`. This is the criterion that makes documentation a build
artifact rather than prose.

**AC4** — Every exit code the binary can return shall be documented, and each
documented code shall state whether it reports a verdict about the changeset or a
failure of pawl itself.
`checkable: yes` — the distinction between exit 1 and exit 2 is load-bearing in
CI, and conflating them turns a blocked merge into a broken pipeline.

**AC5** — Where the documentation describes behaviour that is specified but not
implemented, it shall mark that behaviour as not implemented and link to the spec
that defines it.
`checkable: partially` — the marking is greppable; that it is applied everywhere
is a review judgement.

**AC6** — The documentation shall render from plain markdown with no site
generator, and shall not require a build step.
`checkable: yes` — no generator config, no dependency, no build target.

**AC7** — The documentation shall address operators, and shall not duplicate the
design rationale held in `AGENTS.md` and `_spec/`.
`checkable: no` — an audience judgement, and the criterion most likely to erode,
because rationale is the easiest thing to paste into a user-facing page.

## Non-functional

- **Documentation is delivered surface.** It versions with the binary and drifts
  from it the way any two coupled artifacts drift. AC3 and AC4 exist so that
  drift is caught by a check rather than by a client.
- **Leave-behind.** Written for the client engineer who inherits pawl, not the
  pod that installed it. Assume no context and no access to whoever set it up.
- **No generator, for now.** A docs site is a dependency and a build step, and
  pawl's whole argument is that it has neither. Plain markdown renders on GitHub
  today and can be fed to any generator later without restructuring. Choosing one
  is a separate decision and needs its own spec.
- **Honesty over completeness.** Documenting an unimplemented feature as though
  it works is worse than omitting it — a client planning around it discovers the
  gap at the least convenient moment.

## Out of scope

- **A published documentation site.** Hosting, theming and a generator are a
  separate unit of work. AC6 keeps that choice cheap.
- **`--help` text content.** What the CLI prints is specified where the commands
  are. This spec constrains only that the two agree (AC3).
- **API documentation.** Everything except `cmd/pawl` is under `internal/` and is
  not a public surface. Promoting packages out is a Phase 4 decision.
- **Tutorials and worked engagements.** Reference and operation only. Narrative
  material about running a factory engagement is commercial collateral, not
  product documentation.
- **Translations.**

# PAWL-019 — Harness installation

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/cli`, `internal/harness`
**Related:** [PAWL-016](./PAWL-016-edit-time-accounting-hook.md) built the hook
this replaces the delivery of; [PAWL-008](./PAWL-008-harness-hooks.md) AC7 wants
more harnesses than one.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

The hook built in PAWL-016 does not load. Its configuration lives in
`<repo>/.claude/settings.json`, which Claude Code reads only when that
repository is the project root. Anyone working on pawl from a parent directory —
which is how this repository is actually used — gets no hook at all, silently.
Demonstrated: a `Write` produced no injected context, while running the script
by hand against the same payload produced exactly what it should.

Two further problems come with the current shape, and both are worth fixing in
the same change rather than carrying forward:

**A shell script is the wrong artifact.** `hooks/claude-code/pending.sh`
requires `jq`, which sits badly beside a tool whose distribution argument is
that it has no dependencies. It also needs bash, so it does not run on Windows.
And any copy installed into a user's home goes stale the moment pawl is
upgraded, with no signal that it has.

**Installation should not be a manual step.** Asking a client to hand-merge JSON
into their editor configuration is the kind of friction C-6 exists to refuse.

## Decision — pawl is the hook

The hook command becomes `pawl hook claude-code`. pawl reads the harness's JSON
payload on stdin and writes the harness's response on stdout.

This removes the script, the `jq` dependency and the staleness together: the
hook is always exactly the pawl that is installed, and it is testable in Go
rather than only by hand.

### Installation

**AC1** — The system shall provide a command that installs its hook
configuration into the user's harness settings.
`checkable: yes` (once built)

**AC2** — The system shall merge into existing settings and shall preserve every
key and array entry it did not add.
`checkable: yes` (once built) — **the criterion that matters most.** These are
the user's settings, not pawl's, and they will already contain hooks,
permissions and preferences that pawl knows nothing about. A tool that
clobbers an editor configuration will be uninstalled immediately and deserve it.

**AC3** — Running the install twice shall leave the settings identical to
running it once.
`checkable: yes` (once built) — a hooks array that grows an entry per invocation
is the obvious way to get this wrong.

**AC4** — The system shall write a backup of the settings file before modifying
it, and shall report where.
`checkable: yes` (once built)

**AC5** — The system shall offer to report the exact change without applying it.
`checkable: yes` (once built) — nobody should have to trust a tool's description
of what it is about to do to their configuration.

**AC6** — The system shall provide the inverse operation, removing only what it
added.
`checkable: yes` (once built) — an install with no uninstall is a change the
user cannot reverse without reading pawl's source.

### Behaviour once installed

**AC7** — Where the edited file is not inside a repository containing a `.pawl`
directory, the hook shall produce no output and exit zero.
`checkable: yes` (once built) — user-level settings apply to *every* project.
A hook that speaks in repositories not using pawl is noise, and one that runs
work there is a tax on every edit the user makes anywhere.

**AC8** — The hook shall determine the repository from the edited file's
location, not from its own.
`checkable: yes` (once built) — the current script derives the repository from
where it sits, which is why it can only work when copied into the repository it
serves.

**AC9** — The hook shall not require any interpreter, shell, or external command
beyond pawl itself.
`checkable: yes` (once built) — no `jq`, no bash. The distribution argument is
that pawl brings nothing with it; a hook that needs a JSON processor installed
quietly gives that up.

**AC10a** — Where the hook is invoked with a terminal on standard input, it
shall print usage and exit non-zero rather than waiting for input.
`checkable: yes` (once built) — found by running it at a prompt, where it hung
with no output and no indication why. A hook entry point is not an interactive
command, but "not for interactive use" in a usage string does not help someone
who has already run it and is watching a cursor blink. Silence is correct for a
harness and wrong for a human, and the two are distinguishable.

**AC10** — Where anything fails, the hook shall exit zero and produce no output.
`checkable: yes` (once built) — carried forward from PAWL-016 AC9 and now more
important, because a user-level hook fires on every edit in every project.

## Non-functional

- **This is an adapter, not a harness.** The build/buy position is explicit that
  pawl does not build a harness. Teaching pawl one harness's hook protocol is a
  few lines of JSON handling at an integration boundary; it does not put pawl in
  the business of running agents. If the adapter ever grows logic beyond
  translating a payload, that is the signal it has drifted.
- **User settings are a shared resource.** Every criterion above about merging,
  backing up, dry-running and uninstalling exists because pawl is a guest in a
  file it does not own. Getting this wrong is worse than not shipping it.
- **Project settings remain valid.** A team that prefers the configuration
  committed alongside their repository should still be able to do that. The
  install writes to the user's settings because that is what fixes the loading
  problem, not because project-level configuration is wrong.

## Open question — the Bash gap

Found while testing, and **not resolved by this spec**: the matcher is
`Edit|Write|MultiEdit`. An agent that edits through shell commands — `sed`, a
heredoc, a script — matches none of them and bypasses accounting entirely, with
no error and no prompt. The gate catches it eventually at PR time, which is
precisely the C-2 backfill situation the hook exists to prevent.

Adding `Bash` to the matcher is not a fix on its own: the payload then carries a
command rather than a file path, so the hook has nothing to report on and would
have to fall back to scanning the whole tree, which is both slower and noisier.

This needs deciding before pawl is used with a shell-driven agent, and it is
its own unit of work.

## Out of scope

- **Harnesses other than Claude Code.** PAWL-008 AC7 wants Codex at minimum, and
  the subcommand shape (`pawl hook <harness>`, `pawl setup <harness>`) is chosen
  to make adding one cheap. Adding them is separate work.
- **Agent Plugins packaging.** Still the intended distribution for PAWL-008 AC7;
  this is the path for people who have not adopted plugins.
- **Deciding the Bash matcher question.** Recorded above, deliberately not
  answered here.

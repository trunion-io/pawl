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
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstallPreservesEverythingItDidNotAdd`

**AC2** — The system shall merge into existing settings and shall preserve every
key and array entry it did not add.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstallPreservesEverythingItDidNotAdd` — **the criterion that matters most.** These are
the user's settings, not pawl's, and they will already contain hooks,
permissions and preferences that pawl knows nothing about. A tool that
clobbers an editor configuration will be uninstalled immediately and deserve it.

**AC3** — Running the install twice shall leave the settings identical to
running it once.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstallIsIdempotent` — a hooks array that grows an entry per invocation
is the obvious way to get this wrong.

**AC4** — The system shall write a backup of the settings file before modifying
it, and shall report where.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstallBacksUpFirst`

**AC5** — The system shall offer to report the exact change without applying it.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstallReportsTheChangeWithoutApplyingIt` — nobody should have to trust a tool's description
of what it is about to do to their configuration.

**AC6** — The system shall provide the inverse operation, removing only what it
added.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestUninstallRemovesOnlyOurs` — an install with no uninstall is a change the
user cannot reverse without reading pawl's source.

### Behaviour once installed

**AC7** — Where the edited file is not inside a repository containing a `.pawl`
directory, the hook shall produce no output and exit zero.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestHookIsSilentOutsideAPawlRepo` — user-level settings apply to *every* project.
A hook that speaks in repositories not using pawl is noise, and one that runs
work there is a tax on every edit the user makes anywhere.

**AC8** — The hook shall determine the repository from the edited file's
location, not from its own.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestHookFindsTheRepoFromTheEditedFile` — the current script derives the repository from
where it sits, which is why it can only work when copied into the repository it
serves.

**AC9** — The hook shall not require any interpreter, shell, or external command
beyond pawl itself.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstalledHookNeedsNoInterpreter` — no `jq`, no bash. The distribution argument is
that pawl brings nothing with it; a hook that needs a JSON processor installed
quietly gives that up.

### The hook is a normal command

**AC11** — The system shall resolve what to report in this order: a path given
as an argument, then a payload on standard input, then the working tree.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestHookPrefersAnExplicitPath` — every invocation does something useful. The
first version blocked forever when run at a prompt, and the first fix made it
refuse outright; both treated "no input" as an error when it is simply the case
with the most obvious default.

**AC12** — Where standard input is a terminal, the system shall not wait for
input.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestHookRefusesATerminal` — a person running the command gets an answer, not
a blinking cursor.

**AC13** — Where standard input is not a terminal and carries no usable payload,
the system shall produce no output and exit zero.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestNonInteractiveWithNoPayloadStaysSilent` — a pipe with nothing readable on it is a harness
call that went wrong, and AC10 governs that. The distinction between a person
and a harness is observable, so the tool observes it rather than making the
person guess.

**AC16** — The system shall accept the directory to install into, defaulting to
the user's home directory when none is given.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstallHonoursAnExplicitDirectory` — the default is what fixes the loading problem,
but a team that wants the configuration committed beside their repository should
not have to hand-edit JSON to get it. This is also what makes the install
testable against a temporary directory rather than against whoever is running
the suite.

**AC17** — The system shall not present the harness entry point among its
general commands, and shall state that its output follows the harness rather
than being a pawl interface.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestHarnessEntryPointIsNotAGeneralCommand` — the shape of what it reads and writes is decided
by a harness, not by pawl. Listing it beside `claim` and `gate` invites someone
to script against it, and the next change to a harness protocol then breaks
them. It is a diagnostic and an integration point, and the stable answer to
"what is unaccounted?" is `pawl pending`.

### An installation that does not work must not look like one that does

**AC18** — The system shall install an absolute path to the running binary
rather than a bare command name.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstallWritesAnAbsolutePath` — a bare `pawl` resolves only if pawl is on the
PATH the harness happens to hand its hooks, which is not the PATH of a login
shell and not the one a direnv-scoped install provides. pawl knows exactly where
it is at install time; guessing that the harness will find it later is the kind
of assumption that fails silently.

**AC19** — When installing, the system shall run the command it is about to
install and refuse to report success if it does not work.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestCheckReportsABrokenInstallation` — an install that writes correct JSON naming a
binary that cannot be found has done nothing, and said it succeeded.

**AC20** — The system shall provide a way to check an existing installation,
reporting whether the configuration is present *and* whether the command it
names actually runs.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestCheckReportsABrokenInstallation` — this is the diagnostic that was missing. AC10
requires the hook to stay silent on failure so it can never break an edit loop,
and the cost of that is a broken installation being indistinguishable from a
working one with nothing to say. Something has to be able to tell them apart,
and it must not be the hook itself.

**AC21** — Where an entry pawl installed names a command different from the one
it would install now, the system shall replace it.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstallRepairsAnOutdatedEntry` — otherwise idempotency becomes a trap: an
existing broken entry is recognised as ours, skipped as "already installed", and
never repaired. It also means moving or upgrading the binary silently leaves a
configuration pointing at the old path. AC3 still holds — installing twice from
the same binary changes nothing.

### The configuration ships in the binary

**AC14** — The system shall hold the harness configuration it installs as a
single definition compiled into the binary.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstalledConfigIsTheEmbeddedOne` — assembling the JSON at the point of use spreads
the shape of a working configuration across code, documentation and whatever a
user pasted from a README. One definition, shipped with the binary that
implements it, cannot drift from itself.

**AC15** — The system shall derive the marker it uses for idempotency and
uninstall from that same definition.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestInstalledConfigIsTheEmbeddedOne` — a separately declared constant is a second
source of truth for "which entry is ours", and the failure when they disagree is
an uninstall that leaves the hook in place.

**AC10** — Where anything fails, the hook shall exit zero and produce no output.
`checkable: yes` → `test:trunion.io/pawl/internal/e2e.TestHookSurvivesGarbage` — carried forward from PAWL-016 AC9 and now more
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

**Resolved in [PAWL-020](./PAWL-020-turn-boundary-accounting.md)**, which binds
accounting to the turn boundary rather than to a list of editing tools. Recorded
here as found: the matcher is
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

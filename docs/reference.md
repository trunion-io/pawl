# Reference

## Commands

| Command | Runs where | Does what |
|---|---|---|
| `pawl claim` | Developer machine, from a harness hook | Records one claim against a span of source |
| `pawl ack` | Developer machine, from a harness hook | Accounts for a changed span that carries nothing to assume |
| `pawl pending` | Developer machine, from a harness hook | Lists changed spans in the working tree with no record yet |
| `pawl verify` | CI | Resolves claims against evidence, prints the reading list |
| `pawl attest` | CI | Emits the in-toto Statement for signing |
| `pawl gate` | CI | Evaluates the policy pack, exits 1 on violation |
| `pawl prune` | After a merge | Removes record files an attestation already embeds |
| `pawl sample` | CI, after a gate passes | Selects this changeset for calibration review |
| `pawl review` | Reviewer, by hand | Two-phase review of a sampled changeset |
| `pawl calibrate` | Anywhere | Reports the false-clear rate |
| `pawl setup` | Once, per machine | Installs pawl's hook into your harness settings |
| `pawl hook` | Called by the harness | Hook entry point; not for interactive use |
| `pawl version` | Anywhere | Prints version and platform |

`pawl <command> -h` lists the flags for any command.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. For `gate`, the changeset passed the policy |
| `1` | `gate` only — a policy violation. The reading list still printed |
| `2` | Usage error, or pawl could not do its job (bad flags, missing evidence file, git failure) |

The distinction matters in CI: `1` is a verdict about your changeset, `2` means
pawl itself did not run properly. Do not treat them the same.

## `pawl claim`

```bash
pawl claim "<text>" --path <file> --lines <a-b> [options]
```

The claim text comes first. It is what a human will read when the span reaches
their reading list, so write it as a statement about the world, not about the
code — *"exp is unix seconds in the same clock domain as now"*, not *"added
expiry check"*. The diff already shows what you did.

| Flag | Default | Notes |
|---|---|---|
| `--path` | required | File the claim is about, relative to the repo root |
| `--lines` | required | `40-58`, or `40` for a single line |
| `--kind` | `assumption` | See claim kinds below |
| `--verified-by` | none | `type:ref`, repeatable |
| `--role` | `agent` | `agent`, `expert` or `client` |
| `--harness` | none | e.g. `claude-code` |
| `--model` | none | Model identifier |
| `--identity` | none | Human identity, for `expert`/`client` roles |
| `--session` | none | Groups claims from one working session |
| `--ticket` | none | e.g. `PROJ-1234` |
| `--repo` | `.` | Repository root |

Each claim is written to its own file under `.pawl/claims/`. Records are written
once and never modified: a claim edited afterwards to match the final code is not
evidence of anything.

The span is read from the **working tree**, not from git, because at the moment
of the edit the change has not been committed. That is the point.

## `pawl ack`

```bash
pawl ack --path <file> --lines <a-b> [options]
```

Records that you changed a span and there was **nothing to assume** about it — a
mechanical edit, a generated file, a test that is itself the evidence for a claim
elsewhere.

| Flag | Default | Notes |
|---|---|---|
| `--path` | required | File, relative to the repo root |
| `--lines` | required | `40-58`, or `40` for a single line |
| `--role` | `agent` | `agent`, `expert` or `client` |
| `--harness` | none | e.g. `claude-code` |
| `--model` | none | Model identifier |
| `--identity` | none | Human identity, for `expert`/`client` roles |
| `--session` | none | Groups records from one working session |
| `--repo` | `.` | Repository root |
| `--auto` | off | Apply the rules in `.pawl/policy.toml` instead of a single span |
| `--dry-run` | off | With `--auto`: report what the rules match, record nothing |

**There is no text argument, deliberately.** Accounting for a trivial span must
cost you nothing to write, or it will not happen. If you find yourself wanting to
explain an acknowledgement, that is the signal it should have been a claim.

Acknowledgements are written to `.pawl/acks/` — a **separate directory** from
claims, so that nothing reading claims can encounter one. They never appear in
the claim count shown to a client, and never in the attestation as claims.

### What an acknowledgement does and does not do

- It **collapses** the span, so it does not reach a human.
- It is **not** `clear`. A cleared span was evidenced; an acknowledged span was
  waved through, and the two stay distinguishable everywhere.
- It **does not** rescue an overlapping claim that needs a human. A claim always
  outranks it.
- It **stops accounting** for its span if the code changes underneath it, exactly
  as a drifted claim does. The span reverts to needing a human.
- It **is sampled**. Acknowledged spans go into the calibration pool, so waving
  something through that mattered shows up later as a false clear.

`pawl verify` reports the acknowledgement ratio — of the changed code that
carried any record at all, the fraction waved through rather than reasoned about.
A ratio climbing toward 100% means claiming has become box-ticking.

## `pawl pending`

```bash
pawl pending [--repo .] [--json] [<file>...]
```

Lists changed spans in the **working tree** that carry neither a claim nor an
acknowledgement. This is the edit-time question — *"is what I just changed
accounted for?"* — as opposed to `verify`'s PR-time question.

| Flag | Default | Notes |
|---|---|---|
| `--repo` | `.` | Repository root |
| `--json` | off | Machine-readable, for a harness hook |
| `--quiet` | off | Print nothing, exit 0 |
| `--once` | off | Stay silent if this exact pending set was already surfaced |

Positional arguments restrict the report to those files.

Three things distinguish it from `verify`:

- It works against **uncommitted** changes, including files git does not yet
  track. A brand new file is entirely pending.
- It needs **no evidence files**. The tests for an edit made thirty seconds ago
  have not run yet, and waiting on them would make the command useless at the
  moment it is wanted.
- It **always exits 0**, even on failure. It is called from an edit loop, and a
  tool that breaks your agent when it misbehaves deserves to be uninstalled.

### The Claude Code hook

```bash
pawl setup claude              # install
pawl setup claude --dry-run    # show the resulting settings, write nothing
pawl setup claude --uninstall  # remove pawl's hook, leave everything else
```

| Flag | Default | Notes |
|---|---|---|
| `--dir` | your home directory | Install into `<dir>/.claude/settings.json` |
| `--dry-run` | off | Print the resulting settings; write nothing |
| `--uninstall` | off | Remove pawl's hook only |

By default this writes to `~/.claude/settings.json` — **user-level, not
project-level**. Use `--dir <path>` to install into a specific directory
instead, which is how you get the configuration committed beside a repository if
your team prefers that.
Project settings load only when that project is the root, so a hook configured
in one repository is silently absent whenever you work from a parent directory.
User settings apply everywhere, which is what makes the hook actually fire.

The hook is `pawl hook claude-code`: pawl itself, reading the harness payload on
stdin. There is no script to install and nothing to go stale when pawl is
upgraded, and it needs no `jq`, no bash and no interpreter.

Four things it does to your settings, in order:

1. **Merges.** Every key, hook and permission it did not add is preserved.
2. **Backs up first**, and tells you where.
3. **Refuses** to touch a settings file it cannot parse, rather than rewriting it.
4. **Does nothing** on a second run.

It reformats the file, because the JSON is re-encoded — content is preserved,
key order is not. Check the backup if you want the original layout.

Open `/hooks` once, or restart, for the harness to pick it up.

### `pawl hook claude-code`

The entry point the settings file points at, and a normal command you can run
yourself. It reports unaccounted spans, resolving what to look at in this order:

```bash
pawl hook claude-code src/auth.go     # 1. an explicit path wins
… | pawl hook claude-code             # 2. a harness payload on stdin
pawl hook claude-code                 # 3. otherwise the whole working tree
```

| Flag | Default | Notes |
|---|---|---|
| `--repo` | `.` | Repository root, when falling back to the working tree |

Running it bare at a prompt gives you the working tree — it does not wait for
input. A **pipe** carrying nothing usable is treated differently: that is a
harness call that went wrong, and it stays silent rather than scanning the tree
on every edit.

It also stays quiet when the same set of spans was already surfaced, so editing
one file repeatedly does not repeat the same message.

### The configuration ships in the binary

`pawl setup claude` installs a configuration compiled into pawl itself, not one
assembled at the point of use or copied out of this page. The marker used for
idempotency and uninstall is read from that same definition, so "which entry is
ours" cannot disagree with what was installed.

The hook **informs; it does not enforce**. Enforcement is already the gate's job
— `max_unclaimed_lines` blocks the merge. What only a hook can do is supply the
span *at the moment of the edit*, so the answer is evidence rather than a
reconstruction from a finished diff. That distinction is C-2.

It stays silent in any repository with no `.pawl` directory. User settings apply
to every project, and a repository not using pawl should hear nothing.

**Known gap:** the matcher is `Edit|Write|MultiEdit`. An agent that edits through
shell commands — `sed`, a heredoc, a script — matches none of them and bypasses
accounting entirely, with no error. See the open question on
[`PAWL-019`](../_spec/PAWL-019-harness-installation.md).

## Where records are stored

| Path | What |
|---|---|
| `.pawl/claims/<id>.json` | One file per claim |
| `.pawl/acks/<id>.json` | One file per acknowledgement |
| `.pawl/.cache/` | Machine-local working state. **Ignore this in git** |

**One file per record, deliberately.** The earlier layout used two shared
append-only files, and two branches each recording a claim conflicted on merge —
every second merge, and in a merge queue every PR against every other. Record
ids are unique, so per-record files mean git never has to merge anything.

**Commit `.pawl/claims/` and `.pawl/acks/`.** CI reads them from your checkout;
a repository that ignores them has CI find no records and mark every changed
line unclaimed. They are already excluded from the changed-line count, so
committing them does not affect your ratio.

**Ignore `.pawl/.cache/`.** It is per-clone working state and never influences a
verdict — deleting it changes nothing except what you are told and when.

Record files are **written once and never modified**. Re-recording an existing
id is refused rather than overwriting: an amended record is not evidence of
anything.

If you still have `.pawl/claims.jsonl` from an earlier version, it is read
alongside the new layout and left alone. There is no migration to run.

## `pawl prune`

```bash
pawl prune --attested trail.intoto.json [--repo .] [--dry-run]
```

Removes the record files that an attestation already embeds.

| Flag | Default | Notes |
|---|---|---|
| `--attested` | required | The attestation whose records to remove |
| `--repo` | `.` | Repository root |
| `--dry-run` | off | Report what would be removed, remove nothing |

Records are working state for an unmerged changeset, not a permanent archive.
The signed attestation contains every one of them and git history keeps them
regardless, so pruning after a merge keeps `.pawl/` holding only in-flight work.

Pruning only what a trail provably names is what makes it safe — it will never
remove a record that is not already preserved somewhere durable. Legacy
`claims.jsonl` entries are skipped, because removing one line would rewrite a
shared append-only file.

Running it at all is your choice; some teams will want the working records kept
in the tree.

## Automatic acknowledgement

`pawl ack --auto` applies deterministic rules from `.pawl/policy.toml` and
records an acknowledgement for every pending span they match.

```toml
[accounting]
auto_acknowledge_paths = ["vendor/", "*.pb.go", "go.sum"]
auto_acknowledge_formatting_only = true
```

Trailing slash matches a path prefix; anything else is matched as a glob.
`auto_acknowledge_formatting_only` acknowledges a file whose
whitespace-normalised content is unchanged — provable rather than heuristic,
because it reuses the same normalisation as claim fingerprints.

Three things bound what this can do:

- **A rule can only ever acknowledge.** It can say there was nothing to assume;
  it can never say *what* was assumed, because it does not know. Attempting to
  record a rule-produced claim is an error.
- **Rule-produced records say so.** Each carries `origin: rule` and the name of
  the rule, so a rule that turns out to be wrong is traceable to every record it
  made.
- **They are sampled like any other acknowledgement.** A bad rule surfaces as
  false clears attributed to it by name. The automation is measured, not
  trusted.

Rules live in your policy file for the same reason the thresholds do: they
decide what escapes human attention, so they are yours, and a change to them
belongs in a reviewed diff. They cannot be set from the environment.

## Calibration

If pawl is doing its job, humans read a small fraction of hunks — so there is no
ongoing signal on whether the trail is *accurate*. Left alone, the trail becomes
ritual, agents learn to emit whatever passes the checker, and it surfaces during
an incident.

Calibration forces a full human read on a randomly selected **cleared** changeset
and records whether the trail was faithful to what the diff actually did.

### `pawl sample`

```bash
pawl sample --base origin/main --junit junit.xml --rate 0.05
```

| Flag | Default | Notes |
|---|---|---|
| `--rate` | `0.05` | Fraction of cleared changesets to sample |
| `--force` | off | Sample regardless of the rate |

Selection is **derived from the changeset's tree hash**, not from fresh
randomness. The same changeset always gets the same answer, so re-running is
idempotent rather than another attempt at not being sampled.

Acknowledged spans are sampled alongside cleared ones. An acknowledgement
asserts there was nothing to assume, and an assertion nobody checks is exactly
what this tool exists to refuse.

### `pawl review` — two phases, and the order is enforced

| Flag | Default | Notes |
|---|---|---|
| `--list` | off | Samples awaiting review |
| `--span` | none | The span to judge, as `path:start-end` |
| `--verdict` | none | `correct` or `false_clear` |
| `--claim` | none | Phase 2: the claim to attribute a cause to |
| `--cause` | none | `claim_false`, `claim_incomplete`, `anchor_wrong`, `evidence_hollow` |
| `--reviewer` | none | Who reviewed it |

**Phase 1 is blind.** You see the spans and the code; you do not see what was
claimed. Answer only: did anything here need a human?

```bash
pawl review <id>                                     # spans, no claims
pawl review <id> --span src/a.go:4-6 --verdict correct --reviewer rich
```

**Phase 2 unlocks only when every span has a verdict.** Attempting it early is
refused, not discouraged — a claim is a plausible story about the code, and
having read it first makes it very hard to conclude the code needed attention
anyway.

```bash
pawl review <id> --span src/a.go:4-6 --claim abc123 --cause evidence_hollow
```

### The causes, and who owns them

| Cause | Meaning | Owner |
|---|---|---|
| `claim_false` | The claim asserts something untrue | Agent |
| `claim_incomplete` | True, but did not address what needed review | Agent |
| `anchor_wrong` | Bound to a span it does not describe | **pawl defect** |
| `evidence_hollow` | The cited check passes but does not exercise the claim | **Tool blind spot** |

Separating them is what makes the number actionable. "Improve the agents" and
"fix the anchoring" are different projects, and one rate cannot tell them apart.

`evidence_hollow` is the one only a human read can ever find: pawl can check that
a check exists and passed, never that it is *meaningful*.

### `pawl calibrate`

```bash
pawl calibrate [--since 2026-01-01] [--json]
```

| Flag | Default | Notes |
|---|---|---|
| `--since` | none | Only samples on or after this date (`YYYY-MM-DD`) |
| `--json` | off | Machine-readable report |
| `--repo` | `.` | Repository root |

Reports the false-clear rate, broken down by author role and by cause.

**Only reviewed spans count.** An unreviewed span is evidence that nobody
looked, not that clearing was right — counting it as correct would let the number
improve by sampling more and reviewing less.

The role breakdown is the handover curve: the rate on client-authored changesets
falling over time is the leading indicator that an engagement is finishable.

If the corpus spans multiple pawl versions the report says so. Verdicts change
between versions, so a rate mixing verifiers needs qualifying.

## Claim kinds

| kind | meaning | escalates? |
|---|---|---|
| `assumption` | Taken as true without proof at the point of writing | if unverified |
| `rejected_alternative` | A path considered and not taken | if unverified |
| `undetermined` | Could not establish it, proceeded anyway | **always** |
| `constraint` | A requirement traced to a spec criterion | if unverified |

`rejected_alternative` is the one nobody can reconstruct afterwards — the diff
contains no trace of the path not taken — and it is the hardest habit to build.

`undetermined` always reaches a human regardless of test results. Reaching for it
is correct behaviour, not failure.

## Evidence types

Used as `--verified-by <type>:<ref>`.

| type | ref is | Resolved against |
|---|---|---|
| `test` | A test node id | junit XML: does it exist, did it pass |
| `coverage` | ignored | Cobertura: is every line in the span hit |
| `typecheck` | A path, or empty for the claim's own path | Typecheck report |
| `policy` | A named rule | OPA decision log |
| `spec` | A criterion id, e.g. `PAWL-003-AC2` | Signed spec — only `checkable` criteria count |

**An asserted check that does not exist does not clear the span.** Absent is not
the same as failed, and neither one clears. This is the single most important
behaviour in the tool: an agent asserting `test:TestExpiry` is asserting that a
check exists, and pawl decides whether that is true.

## Anchor statuses

Claims bind to a content fingerprint, not a line number, and are relocated
against the delivered tree.

| status | meaning |
|---|---|
| `anchored` | Found at the recorded location |
| `relocated` | Found elsewhere in the file; line numbers moved under it |
| `drifted` | Not found. The claim no longer describes delivered code |
| `orphaned` | The file is gone |

`drifted` and `orphaned` claims **never clear**, whatever checks they assert. A
stale claim clearing a span on a passing test is the difference between a trail
and a ritual.

Whitespace is normalised per line, so a reformat preserves anchors. Identifiers
are not normalised, so a rename shows as drift. That is intended — a rename is a
real change — but it is noisy on large refactors.

## Span verdicts

| verdict | meaning |
|---|---|
| `clear` | Claimed and mechanically covered. Collapsed |
| `acknowledged` | Accounted for by `pawl ack`, not evidenced. Collapsed, and sampled |
| `unverified` | Claimed, but at least one claim over it needs a human |
| `unclaimed` | Changed code with no record at all. Always readable |

Verdicts are computed **per line**, not per hunk. A hunk frequently holds one
span a verified claim covers and another nobody claimed; sending the whole hunk
to a human because of the second would make "the minimum set a human must read"
false.

Where a line is covered by both a clearing claim and one needing a human, it
needs a human. Wrongly collapsing a line costs an unreviewed defect; wrongly
expanding one costs a few seconds of reading.

## Evidence file flags

Shared by `verify`, `attest` and `gate`:

| Flag | Notes |
|---|---|
| `--base` | Base ref for the changeset. Default `origin/main` |
| `--junit` | junit XML. Repeatable |
| `--coverage` | Cobertura XML. Repeatable |
| `--typecheck` | Typecheck report, JSON |
| `--policy-results` | OPA decision log, JSON |
| `--spec` | Signed spec attestation, JSON |
| `--strip-prefix` | Prefix to strip from coverage paths |

Coverage tools frequently report paths differently from git. `--strip-prefix` is
the escape hatch; if your coverage shows `/build/src/auth.go` and git knows it as
`src/auth.go`, strip `/build/`.

## Output flags

| Flag | Commands | Notes |
|---|---|---|
| `--json` | `verify`, `gate` | Machine-readable output. On `verify` the full reading list; on `gate` the decision and its violations |
| `--annotations` | `verify` | Writes GitHub check-annotation JSON to the given path — one entry per span that reached a human |
| `--out` | `attest` | Writes the in-toto Statement to the given path instead of stdout |
| `--policy-pack` | `attest` | Free-form identifier recorded in the predicate, e.g. `pawl@0.1.0`. Useful for recording which thresholds were in force |

`--json` does not change any verdict, and `gate --json` still exits 1 on a
violation. Parse the exit code, not the output, when deciding whether to block.

Until the attestation records the tool that produced it (see
[`PAWL-011`](../_spec/PAWL-011-tool-provenance.md)), `--policy-pack` is the
closest thing to provenance in the predicate — passing the pawl version through
it is a reasonable stopgap.

## What is excluded from the diff

`.pawl/` is excluded from changed-line counting. The claim log is part of the
changeset on disk but is not code anyone reviews, and counting it would inflate
the denominator and put the trail on its own reading list.

Blank lines and bare delimiters (`}`, `)`, `];` and friends) are not counted as
reviewable. Comments **are** counted — a wrong comment is a defect, and agents
write plenty of them.

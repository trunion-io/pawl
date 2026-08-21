# PAWL-018 — Record storage and merge behaviour

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/claimlog`
**Extends:** [PAWL-001](./PAWL-001-claim-capture.md) —
changes where records live, not what a record is.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (SRE) | *unsigned* |

## Context

Records live in two append-only JSONL files, `.pawl/claims.jsonl` and
`.pawl/acknowledgements.jsonl`. Both must be committed — CI reads them from the
checkout, and a repository that ignores them has CI find no records and mark
every changed line unclaimed.

Committing them in that shape does not survive ordinary use. Demonstrated
against real git: two branches each recording one claim, merged in sequence.

```
Auto-merging .pawl/claims.jsonl
CONFLICT (content): Merge conflict in .pawl/claims.jsonl
```

Every branch appends at the same end of the same file, so **every second merge
conflicts**. A merge queue makes it worse rather than better: each queued PR
conflicts with every other queued PR that recorded a record, and the queue
serialises into a sequence of manual resolutions of a file nobody should ever
have to hand-edit.

Hand-resolving an append-only evidence log is also the exact moment its
guarantees are worth least. A human picking sides in a conflicted claim log can
silently drop records, and nothing downstream would know.

This is not a novel problem. It is why
[changesets](https://github.com/changesets/changesets) writes `.changeset/*.md`
and towncrier writes newsfragments: appending to one shared file conflicts on
every pull request, and the established answer is to stop sharing the file.

## Decision — one file per record

**AC1** — The system shall store each claim and each acknowledgement in its own
file.
`checkable: yes` (once built)

**AC2** — The system shall name each record file from that record's identifier.
`checkable: yes` (once built) — identifiers are already unique, so two branches
cannot produce the same filename and git never has to merge anything.

**AC3** — When two branches that each recorded records are merged, the merge
shall not conflict and shall retain every record from both.
`checkable: yes` (once built) — testable against real git, per C-9: build two
branches in `t.TempDir()`, record on each, merge, assert clean and assert the
union. This is the criterion the spec exists for; if it ever fails the format is
wrong again.

**AC4** — The system shall treat a written record file as immutable and shall
never modify one in place.
`checkable: yes` (once built) — append-only was the property of the JSONL log
worth keeping. Write-once files hold it more strongly than appending did: there
is no edit path at all, and an amended record is visible in a diff as a
modification to a file that should never see one.

**AC5** — The system shall read a record set from every file in the record
directory, independent of order.
`checkable: yes` (once built) — filesystem ordering is not a guarantee, and no
behaviour may depend on it.

**AC6** — The system shall read the legacy JSONL logs where present, in addition
to per-record files.
`checkable: yes` (once built) — this repository already has 28 committed records
in the old format, and a client mid-adoption will too. Silently ignoring them
would lose evidence, which is the one thing this component may never do.

**AC8** — The system shall provide a command that copies records from a legacy
log into the per-record layout without altering them, and shall remove the log
only once every record it held is present in the new layout.
`checkable: yes` (once built)

> **This reverses a decision recorded above.** AC6 and the non-functional note
> said the legacy log is read and left alone, on the grounds that rewriting a
> shared append-only file is the edit write-once storage exists to prevent.
>
> That reasoning does not apply to migration. Copying a record into a new file
> does not alter the record — the id, timestamp, text and fingerprint are
> identical, and it is the *container* that changes. What write-once forbids is
> editing a record's content, which this does not do.
>
> The cost of not migrating is two read paths and two layouts in every
> repository, indefinitely, plus a directory that looks like a mistake. Removing
> the log only after verifying every record survived is what makes it safe:
> losing evidence is the one thing this component may never do, so the check is
> not optional and the removal is not unconditional.

## Lifecycle

**AC7** — The system shall provide a means of removing the record files for a
changeset that has been attested.
`checkable: yes` (once built)

Records are **working state for an unmerged changeset**, not a permanent
archive. The signed attestation embeds every one of them and is the durable
artifact; git history keeps them regardless. Pruning after attestation keeps
`.pawl/` holding only in-flight work rather than accumulating every record ever
made.

Pruning is offered, not imposed — whether to run it is the client's call, and
some will want the working records retained in the tree.

## Non-functional

- **No `.gitattributes` dependency.** `merge=union` also resolves the conflict
  and was tested working, but it is a textual trick that holds only because
  JSONL lines happen to be independent, and it depends on a file that can be
  dropped in a migration without anyone noticing until conflicts return. Conflict
  freedom should be a property of the layout, not of configuration.
- **The alternatives to committing were considered and do not survive.** A CI
  artifact has no path from the developer machine to the runner; a service
  violates C-6; commit trailers attach records at commit time, which is later
  than edit time and therefore weakens C-2, and history rewriting would mutate
  an append-only log; git notes are neither fetched nor pushed by default.
  Records must be in the tree, so only the layout is available to change.
- **File count is bounded by the prune step**, not by the format. Without
  pruning a busy repository accumulates thousands of small files — tolerable for
  git, untidy for a human.

## Open question — granularity

The decision above is **provisional and recorded as such at Rich's request**;
the instinct that there is a better shape has not been resolved, only deferred.

One file per record is the safest of three granularities, not obviously the best:

| Granularity | Files | Conflicts |
|---|---|---|
| One shared file | 1 | **every second merge** — the current failure |
| One file per session | few | none across branches |
| One file per record | many | none, by construction |

**Per session** is the option worth revisiting. `session` is already a field on
every record, sessions map closely to branches in practice, and it would cut the
file count by an order of magnitude while still giving each branch its own file
to write. It is weaker in one respect: a session file is appended to rather than
written once, so two agents sharing a session identifier would conflict where
per-record never can.

Revisit if the file count becomes the thing people complain about. Do not
revisit by weakening AC3.

## Out of scope

- **What a record contains.** PAWL-001, delivered and unchanged by this.
- **Storage of attestations.** They are emitted to a path the caller names and
  are not kept in `.pawl/` at all.
- **Compaction of historical records into an archive format.** Only removal is
  specified (AC7); anything cleverer needs a reason first.

# PAWL-035 — Telemetry

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/telemetry`, `internal/cli`
**Related:** [PAWL-007](./PAWL-007-calibration-sampler.md) (built),
[PAWL-012](./PAWL-012-configuration.md) (drafted), constitution **C-6**.
Nothing here supersedes anything.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |
| NFR (Legal) | *unsigned* |

## Context

Nobody knows what this tool does in anger. `AGENTS.md` says the missing thing is
a corpus and that it comes from sustained real use; there is currently no way to
learn anything from that use. How often `verify` runs, whether `attest` succeeds,
which gate rule fires, what fails and how often — none of it is observable
outside the machine it happened on.

pawl **emits**; it does not receive. Records go to an OTLP endpoint, which
defaults to a trunion service so that overall usage can be reviewed, can be
repointed at the client's own collector, and can be turned off. C-6 stays intact
for the tool itself: nothing pawl requires to work is operated by anyone, and an
unreachable endpoint changes no verdict.

Two constraints follow, and neither is a preference.

**Anonymity has to be structural.** "No project or user information" is a claim,
and a claim nobody can check is what this repository exists to refuse. A record
type that *can* carry a path will eventually carry one — through an error
message, a field added in a hurry, a debug value left in. The only version that
holds is a record shape in which a path cannot be expressed.

**OTLP, without the OpenTelemetry SDK.** OTLP defines a JSON encoding over HTTP
as well as protobuf over gRPC, and the JSON form is `encoding/json` and
`net/http` — both standard library. The Go SDK is not: it brings a module graph,
and decision 6 in `AGENTS.md` says a tool asking a security-conscious client to
audit a dependency tree before it can verify anything undermines its own
argument.

This reaches receivers that accept OTLP/HTTP with JSON. It does **not** reach
every OTLP endpoint: a receiver may expose gRPC only, or HTTP with
binary-protobuf only, and neither will accept this. A client in that position
puts a collector in front, which is the ordinary deployment anyway. Claiming
otherwise would be the overstatement the accompanying claim already records.

> **A correction this spec depends on.** `docs/install.md` states that pawl makes
> no outbound network calls. That is already untrue: `internal/selfmanage` holds
> an HTTP client and fetches release checksums for `pawl install verify` and
> `upgrade`. A container built to that guidance without `ca-certificates` fails
> on those commands today. Fixing it is a separate change and a precondition for
> discussing this honestly, because the first question a client asks is what the
> binary already connects to.

## Emission

**AC1** — The system shall emit each invocation as one OTLP log record, in an
`ExportLogsServiceRequest`, JSON-encoded, by HTTP POST to the receiver's
`/v1/logs` path.
`checkable: yes` (once built) — against that request schema, and by a receiver
accepting it.
>
> The signal has to be named. OTLP's logs and metrics are different request
> messages on different paths, and "OTLP" alone selects neither. An invocation is
> a discrete event with typed attributes, which is a log record; aggregate
> counters would be metrics and are out of scope below.

**AC2** — The system shall implement OTLP using the standard library only, and
`go.mod` shall gain no `require` entry.
`checkable: yes` (once built) — `check-deps` over `go.mod`. Decision 6 is the
argument the distribution rests on, and a telemetry feature is not a reason to
spend it.

**AC3** — The system shall emit to a default endpoint where none is configured,
and shall accept configuration replacing that endpoint.
`checkable: yes` (once built) — repointing matters as much as the default. A
client who wants the data and not the destination can send it to their own
collector, which is a better answer to a security review than asking them to
trust ours.

**AC16** — The system shall state, in `docs/install.md`, `README.md` and
`SECURITY.md`, that it emits telemetry by default, what is emitted, where it goes
and how to disable it.
`checkable: yes` (once built) — PAWL-010 binds documentation to behaviour, and
this is the behaviour most likely to be discovered rather than read. A client
finding an outbound connection with `strace` before finding it in the README is
the one outcome this feature cannot survive.

**AC17** — The system shall print a notice to standard error, naming the endpoint
and the configuration key that disables it, on the first invocation that would
emit for a given tool version and endpoint, and shall record that it has done so
in the user's state directory.
`checkable: yes` (once built) — scoped and persisted, because "once" alone is not
implementable. Per version and endpoint, so an upgrade or a repointed collector
says so again — both are changes a client should be told about. Per user rather
than per repository, so it is not repeated for every checkout. If the state
cannot be written the notice prints, since a notice too often is a smaller
failure than one never seen.

**AC18** — The system shall spend no more than two seconds in total on emission,
measured across name resolution, connection, write and read.
`checkable: yes` (once built) — a number, because "bounded" is satisfied by
thirty seconds and thirty seconds on a developer's critical path is a broken
tool. Two seconds is a guess at the right order of magnitude and is stated so it
can be argued with; a blackholed collector costs that once per invocation and
nothing else.

**AC4** — The system shall provide configuration that disables telemetry
entirely, and shall then collect and emit nothing.
`checkable: yes` (once built) — PAWL-012 owns the configuration mechanism.

**AC5** — Where emission fails, the system shall not fail the operation being
measured, and shall not retry beyond the invocation.
`checkable: yes` (once built) — a gate that fails because a collector was down
would be a gate whose verdict depends on something irrelevant to the changeset.

## What is recorded

**AC6** — The system shall define telemetry records as typed fields only, with no
field capable of carrying free text.
`checkable: yes` (once built) — over the record type. This is the criterion the
rest depends on: a schema with no free-text field cannot leak a path, a branch
name or a claim, whatever a future contributor intends.

**AC7** — The system shall record, per invocation: the subcommand, the tool
version, the operating system and architecture, the exit outcome as an
enumeration, and the wall-clock duration.
`checkable: yes` (once built)

**AC8** — Where `verify` runs, the system shall record the counts of changed
spans, claims, acknowledgements and must-read spans, each as a bucket rather than
an exact value.
`checkable: yes` (once built)

> Exact counts identify. A changeset of exactly 1,247 changed lines with 33
> claims is a fingerprint, and a handful of those reconstructs a repository's
> shape without any field naming it. Buckets keep the distribution and lose the
> identity.

**AC9** — Where the gate runs, the system shall record which rules fired, as an
enumeration of exactly: `changeset_size`, `must_read_ratio`, `unclaimed_lines`,
`undetermined_claims`, `sensitive_path_needs_named_check`.
`checkable: yes` (once built) — enumerated here rather than by reference. PAWL-006
defines the gate's criteria and names no rule identifiers at all; the strings live
in `internal/policy/policy.go`, and `undetermined_claims` appears nowhere in
PAWL-006. Citing that spec for this enum would have pointed at a contract that
does not contain it.

**AC10** — Where `attest` runs, the system shall record whether the attestation
was produced, as an enumerated outcome.
`checkable: yes` (once built) — the attestation only. PAWL-005 AC5 is delivered
and says the system shall not sign: signing is `cosign attest-blob` with a CI
OIDC token, in a later step pawl does not observe. An earlier draft recorded
whether signing succeeded, which pawl cannot know and which this spec has no
standing to change, since it supersedes nothing.

**AC11** — Where an operation fails, the system shall record an enumerated error
kind and shall not record the error message.
`checkable: yes` (once built) — messages carry paths. This repository's own
diagnostics name files deliberately, which is right for an operator and wrong for
a record leaving the machine.

## What is never recorded

**AC12** — The system shall not record a repository name, remote URL, branch name,
file path, claim identifier, ticket reference, claim text, commit message,
hostname, username or any value derived from them.
`checkable: yes` (once built) — enforced by AC6's shape rather than a blocklist,
which is why AC6 comes first. A blocklist is a list of the leaks someone thought
of.
>
> An earlier draft banned "identifier" unqualified, which contradicted AC9: rule
> identifiers are fixed strings of ours and must be recorded. The ban is on
> identifiers of the client's things, not of ours.

**AC13** — The system shall not record a value that is stable across invocations
and unique to an installation, including any generated installation identifier.
`checkable: yes` (once built) — a random identifier is not anonymous. It is
pseudonymous: it links every record from one client into a profile, and adding
one is the easiest way to turn telemetry into tracking.

**AC14** — The system shall populate exactly these OTLP resource attributes and no
others: `service.name` as the constant `pawl`, `service.version` as the tool
version, `os.type`, and `host.arch`.
`checkable: yes` (once built) — an enumerated allowlist, because "populate
explicitly" names nothing and is satisfied by an empty resource or by anything at
all.
>
> An earlier draft justified this by claiming the OpenTelemetry Go SDK populates
> `host.name`, `process.owner`, `process.command_line` and `service.instance.id`
> without being asked. That is not established: the SDK's default resource uses
> service and SDK detectors, host and process detectors are opted into, and
> semantic conventions define what an attribute *means* rather than causing it to
> be collected. The assertion was made without checking and is removed. The
> allowlist stands on its own — whatever an implementation would otherwise
> attach, only these four leave.

**AC15** — The system shall provide a flag that runs an invocation normally,
prints the record that invocation produced to standard output at its end, and
emits nothing.
`checkable: yes` (once built) — a client must be able to read exactly what would
leave, and "anonymised" is a claim about something they can open.
>
> A preview *before* the run cannot exist: AC7 records the exit outcome and the
> wall-clock duration, and neither is knowable until the invocation finishes. So
> it is a dry run rather than a preview — the work happens, the record is built
> and shown, and the emission is skipped. The dry run itself emits nothing, which
> would otherwise be the obvious way to defeat the point of it.

## Non-functional

- **This buys less than a corpus.** Telemetry says how often the gate fired and
  which rule; it cannot say whether a cleared span should have been read, which
  is what PAWL-007 samples and what the false-clear rate needs. Both are wanted
  and only one is this.
- **Every field is a commitment.** A field that ships is a field a client's
  security review reads, and removing one later does not unsay it. Fewer fields
  answering a real question beat a schema that might be useful.
- **The client can own the destination.** Emitting OTLP rather than a private
  protocol means a client can point pawl at their own collector and keep the data
  entirely. That is a stronger position to sell from than asking them to trust
  where it goes, and it is the mitigation that makes a default endpoint arguable.
- **Default-on is a decision taken with the trade understood.** It was raised
  once and settled: usage cannot be reviewed from installations that never opt
  in, and a tool nobody can measure cannot be improved against real use.
  AC16, AC17 and the structural anonymity criteria are what that decision is paid
  for with — disclosure in three places, a first-run notice, and a record shape in
  which identifying data cannot be expressed.

## Open decisions

**DECISION-1 — bucket boundaries.** AC8 requires buckets and does not say which.
Too coarse says nothing; too fine reconstructs the changeset. The boundaries
should be chosen against real distributions, and there are none — which is the
problem this spec exists to start solving.

## Out of scope

- **The receiving service.** pawl emits; what trunion runs to receive is its own
  unit of work, with its own retention, access and jurisdiction questions. None of
  it is required for pawl to work, and an unreachable endpoint changes no verdict.
- **Traces and spans in the OpenTelemetry sense.** This emits metrics and events
  about invocations. Instrumenting pawl's internals for distributed tracing is a
  different feature with a different cost.
- **Sampling quality.** PAWL-007 measures whether the tool is *right*. This
  measures whether it is *used*, and they are not substitutes.
- **Correcting `docs/install.md`'s claim about network calls.** Named above
  because this spec cannot be discussed honestly without it; it is its own change.

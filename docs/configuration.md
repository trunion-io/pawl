# Configuration

pawl reads from three places. They are deliberately separate, and the separation
is a governance boundary rather than a convenience.

| | Where | Who owns it | What it controls |
|---|---|---|---|
| **Policy** | `.pawl/policy.toml` | **You** | The thresholds a changeset must clear |
| **Invocation** | Flags, and *(planned)* `.pawl/config.json` + `PAWL_*` | Whoever runs pawl | Where the evidence files are, which base ref |
| **Claim log** | `.pawl/claims.jsonl` | Written by pawl | Append-only record of claims |

The important line is between the first two. Policy is a governance artifact:
commit it, review changes to it like any other change, because a change to it
changes what can merge. Invocation config is plumbing — where `junit.xml` lives
is not a decision anyone needs to review.

## Policy — `.pawl/policy.toml`

**The thresholds are yours, not your supplier's.** A supplier who writes both
the gate and the bar it clears has built theatre. pawl ships a defensible
default; the team that owns the service sets the number.

```toml
[gate]

# Comprehension has a hard ceiling regardless of trail quality. A 3,000 line
# changeset is unreadable even fully annotated. Start generous, ratchet down
# once the team is decomposing work habitually.
max_changed_lines = 400

# Fraction of changed lines that may reach a human before the changeset is
# judged not worth reviewing incrementally. Above this, decompose or improve
# the trail — either beats a reviewer skimming.
max_must_read_ratio = 0.35

# Changed code with no claim over it at all. Zero is the right default:
# silence from the agent is not coverage.
max_unclaimed_lines = 0

# An agent that could not establish something and proceeded anyway always
# escalates, whatever the tests say.
block_on_undetermined = true

# Paths where implicit coverage is not sufficient and a claim must cite a
# named check. Auth, billing, migrations, anything with a blast radius.
sensitive_paths = [
    "src/auth/",
    "src/billing/",
    "migrations/",
]
```

Every key is optional; omitted keys take the defaults above. If the file is
absent entirely, pawl uses all defaults.

### Violations

| Rule | Fires when |
|---|---|
| `changeset_size` | `changed_lines` exceeds `max_changed_lines` |
| `must_read_ratio` | must-read fraction exceeds `max_must_read_ratio` |
| `unclaimed_lines` | unclaimed lines exceed `max_unclaimed_lines` |
| `undetermined_claims` | any `undetermined` claim exists and `block_on_undetermined` |
| `sensitive_path_needs_named_check` | a claim on a sensitive path asserts no check |

Any violation means `pawl gate` exits 1. The reading list still prints, and
`pawl attest` still produces a trail — a failed gate produces evidence, which is
the point of it.

### A note on the TOML reader

pawl parses a deliberate **subset** of TOML: comments, one level of table,
scalars, and arrays of strings. No nested tables, arrays of tables, inline
tables, multi-line strings or dates.

This exists so that pawl carries no third-party dependencies. It **rejects what
it cannot parse rather than guessing**, so an unsupported construct is a loud
error, never a silently wrong threshold.

## Invocation settings

Today these are command-line flags only. See [Reference](./reference.md) for the
full list.

```bash
pawl verify \
  --base origin/main \
  --junit junit.xml \
  --coverage coverage.xml \
  --strip-prefix /build/
```

### Config file and environment variables — planned, not built

> Specified in [`PAWL-012`](../_spec/PAWL-012-configuration.md). **Not
> implemented.** Flags are the only mechanism today.

Repeating the same six flags on every invocation, in every CI job and every
harness hook, is the friction this is meant to remove. The intended shape:

```jsonc
// .pawl/config.json
{
  "base": "origin/main",
  "junit": ["junit.xml"],
  "coverage": ["coverage.xml"],
  "strip_prefix": "/build/"
}
```

```bash
export PAWL_HARNESS=claude-code      # set once by the harness hook
export PAWL_SESSION=$SESSION_ID
```

Resolution order will be **flag → environment → config file → default**, and
`pawl config` will print each resolved value with the source it came from.

Two constraints worth knowing in advance, both in the spec:

- **Gate thresholds will never be readable from config or the environment.**
  They come only from `.pawl/policy.toml`. Otherwise a CI environment variable
  could quietly weaken the gate, which would defeat the point of the client
  owning the thresholds.
- **There will be no user-level config.** Nothing outside the repository, so a
  run is reproducible from a checkout and a reviewer can see everything that
  shaped it.

## What pawl writes

Only `.pawl/claims.jsonl`, and only on `pawl claim`. Everything else is stdout or
a path you named with `--out` / `--annotations`.

pawl starts no services, opens no network connections, and writes nothing
outside your repository.

## Excluding pawl's own files

`.pawl/` is excluded from changed-line counting automatically. You do not need
to configure this, and you should not add it to `.gitignore` — the claim log is
part of the changeset and needs to be committed for CI to read it.

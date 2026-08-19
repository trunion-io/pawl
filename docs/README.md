# pawl documentation

Written for the people who have to **live with pawl in their repo** — client
engineers, SREs, whoever gets paged when a gate blocks a release.

If you are an agent working *on* pawl, you want [`../AGENTS.md`](../AGENTS.md)
and [`../_spec/`](../_spec) instead. Different audience, different documents:
these explain how to use and operate pawl, those explain why it is built the way
it is.

| Page | For |
|---|---|
| [Install](./install.md) | Getting the binary, verifying it, pinning it in CI |
| [Configuration](./configuration.md) | Policy thresholds, and what pawl reads from where |
| [CI integration](./ci.md) | A worked GitHub Actions job, plus GitLab and Jenkins |
| [Reference](./reference.md) | Commands, flags, claim kinds, evidence types, exit codes |
| [Attestation](./attestation.md) | What is in the predicate and how to verify a trail |

## The 60-second version

pawl records what an agent **assumed** while it was writing code, at the moment
it wrote it, and then decides which of those assumptions a human still has to
check before the change merges.

```bash
# During the work, from a harness hook
pawl claim "token.exp is unix seconds, same clock domain as now" \
  --path src/auth.go --lines 44-58 --verified-by test:TestExpiry

# In CI
pawl verify --base origin/main --junit junit.xml
```

```
8 changed lines, 5 need a human (37.5% collapsed)
4 claims, 2 unresolved, 0 unclaimed lines

READING LIST
  ? src/auth.go:8-10  [unverified]
      assumption: Refresh is a passthrough until rotation lands
        - test not found in results: TestRotation
```

Everything else is detail on those two commands.

## Three things worth knowing before you start

**pawl does not approve anything.** It produces a reading list and an evidence
bundle. Your promotion process consumes them. Being demonstrably *not* the actor
that promoted a change matters for liability, and it is why pawl will never hold
your deploy.

**You own the thresholds.** `.pawl/policy.toml` lives in your repo and is yours
to set. A supplier who writes both the gate and the bar it clears has built
theatre. See [Configuration](./configuration.md).

**Silence is never coverage.** Changed code that carries no claim always reaches
a human. There is no configuration that turns that off, because the failure mode
that destroys trust in a tool like this is an agent editing quietly and the gate
going green.

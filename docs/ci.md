# CI integration

pawl consumes what your pipeline already emits — junit XML and Cobertura
coverage — so integrating it does not mean changing how you test. It reads
language-neutral artifacts and does not care what your repo is written in.

## The shape of the job

Four things, in this order:

1. **Install and verify pawl** — pinned version, signature checked
2. **Run your existing tests**, emitting junit and coverage. `continue-on-error`:
   a failing test is *evidence*, not a reason to abort the trail
3. **Verify and attest** — produce the reading list and the signed trail
4. **Gate, last** — so the trail is published even when the gate blocks

Step 4 goes last for a reason. A failed gate still produces evidence, and that
evidence is the most useful artifact of the whole run — it is the thing that
tells you *why* it failed.

## GitHub Actions

A complete worked job is in
[`examples/pawl-gate.yml`](../examples/pawl-gate.yml). The parts that matter:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0        # the gate diffs against the base ref
```

**`fetch-depth: 0` is not optional.** The default shallow clone has no merge
base, so pawl cannot compute a diff. This is the single most common integration
failure.

```yaml
permissions:
  contents: read
  id-token: write         # cosign keyless
  checks: write           # reading-list annotations
```

`id-token: write` is what makes keyless signing work. Without it there is no
OIDC token and `cosign attest-blob` cannot sign.

```yaml
env:
  PAWL_VERSION: "0.1.0"   # pin exactly
```

## Annotations

```bash
pawl verify --base origin/main --junit junit.xml --annotations annotations.json
```

Writes GitHub check-annotation JSON: one entry per span that reached a human,
with the claim text and the reason it did not clear.

This is deliberately the only UI pawl has. Check-run annotations put the reading
list where review already happens, in the diff, next to the lines. A dashboard
would be somewhere people have to remember to go.

## GitLab CI

```yaml
pawl-gate:
  image: ghcr.io/trunion/pawl:0.1.0
  variables:
    GIT_DEPTH: 0          # same requirement as fetch-depth: 0
  script:
    - pawl verify --base origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME --junit junit.xml
    - pawl attest --base origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME --junit junit.xml --out trail.json
    - pawl gate --base origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME --junit junit.xml
  artifacts:
    when: always          # publish the trail even when the gate blocks
    paths: [trail.json]
```

Keyless signing needs an OIDC token; GitLab issues these as `id_tokens`. Without
one, produce the trail unsigned and sign it elsewhere — an unsigned trail is
still evidence, it just is not attributable.

## Jenkins and everything else

pawl is a binary that reads files and exits with a code. There is nothing
CI-specific in it:

```bash
pawl gate --base "origin/${CHANGE_TARGET}" --junit junit.xml
```

Exit `1` is a policy violation. Exit `2` means pawl could not run — bad flags, a
missing evidence file, a git failure. **Do not treat them the same:** one is a
verdict about the changeset, the other is a broken pipeline.

## Getting evidence out of common tools

pawl needs junit XML and, optionally, Cobertura coverage.

| Ecosystem | junit | coverage |
|---|---|---|
| Go | `gotestsum --junitfile junit.xml` | `go test -coverprofile` + `gocover-cobertura` |
| Python | `pytest --junitxml=junit.xml` | `pytest --cov --cov-report=xml` |
| Node | `jest --reporters=jest-junit` | `jest --coverage --coverageReporters=cobertura` |
| Java | Surefire, by default | JaCoCo `cobertura` report |
| .NET | `dotnet test --logger junit` | Coverlet `--format cobertura` |

### Test node ids have to match

The `--verified-by test:<ref>` in a claim must match what your junit reports. If
a claim cites `TestExpiry` and junit records
`trunion.io/pawl/internal/e2e.TestExpiry`, pawl reports the test as **not found**
— and not-found never clears a span.

pawl normalises the common pytest shapes (dotted module path and node-id form)
automatically. For everything else, cite exactly what appears in the XML. When a
claim is not clearing and you expected it to, `pawl verify` prints the reason:

```
- test not found in results: TestRotation
```

### Coverage paths have to match

Coverage tools frequently report paths differently from git — `/build/src/auth.go`
against git's `src/auth.go`. Use `--strip-prefix /build/`.

## Cost

One binary download and a few seconds of work per PR. pawl runs no services,
opens no network connections of its own, and stores nothing.

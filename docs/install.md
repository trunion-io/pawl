# Install

pawl is **one static binary**. No runtime, no interpreter, no dependency tree.

That matters more here than it would for most tools. pawl's product is signed
evidence about your changeset; a supply-chain assurance tool that arrives with a
supply chain of its own is arguing against itself. `go.mod` carries no `require`
block, so there is nothing transitive to review.

- ~3MB, statically linked, no libc requirement
- linux, macOS and Windows, on amd64 and arm64
- Reproducible: rebuild from source and the checksum matches

## From source

Works today, and needs only a Go toolchain.

```bash
make build          # ./bin/pawl
make check          # fmt + vet + the end-to-end suite
./bin/pawl version
```

No modules are downloaded. The one network cost is the pinned Go toolchain in
`go.mod`, fetched once and cached.

## Released binaries

> **Not yet published.** The distribution below is designed and the build side
> is wired (`make dist`), but there is no tagged release yet. Until there is,
> build from source.

| Your stack | Install |
|---|---|
| Anything | Download from GitHub Releases, check `SHA256SUMS`, put it on `PATH` |
| macOS | `brew install trunion-io/tap/pawl` |
| TypeScript | `npx @trunion/pawl` |
| Python | `uv tool install trunion-pawl` |
| CI, containerised | `ghcr.io/trunion/pawl:<version>` — `scratch` image, ~5MB |

The binary is the artifact in every row. Package managers are only delivery
channels, so you install pawl the way you install everything else; none of them
need to know what it is written in.

The npm package uses per-platform `optionalDependencies` with `os`/`cpu` fields
rather than a `postinstall` script, because many organisations block install
scripts outright.

## Verifying what you got

Release binaries are signed with **cosign keyless** off the CI OIDC token. There
is no key, so there is no key to have leaked — the signature binds to the
release workflow identity.

```bash
cosign verify-blob \
  --certificate-identity-regexp 'https://github.com/trunion-io/pawl/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle pawl-0.1.0-linux-amd64.sigstore.json \
  pawl-0.1.0-linux-amd64
```

This is the same tool and the same flow pawl asks you to use for `pawl attest`,
so verifying pawl itself exercises the muscle you are building anyway.

### Reproducing the build

```bash
git checkout v0.1.0
make dist VERSION=0.1.0
sha256sum -c SHA256SUMS
```

`-trimpath` and `CGO_ENABLED=0` are what make this deterministic. If your
checksum does not match a published one, that is a finding — report it.

## Pinning in CI

**Pin by digest, not by tag.** pawl decides whether a pull request can merge, so
the version that runs is part of your merge criteria. Treat an unpinned pawl the
way you would treat an unpinned linter with the power to block a release.

```yaml
env:
  PAWL_VERSION: "0.1.0"
```

Any change to gate behaviour is a **major** version bump. Patch and minor
releases will not change whether a given changeset passes.

## Building a container

The binary is static, so the image is the binary:

```dockerfile
FROM scratch
COPY pawl /pawl
ENTRYPOINT ["/pawl"]
```

You need `ca-certificates` in the image only if you add features that make
outbound network calls. pawl currently makes none.

## Licence

pawl is **source-available, not open source**. The source is public so you can
read it and rebuild it; running it is granted by a commercial agreement, not by
downloading it. `LICENSE.txt` ships with every release and states the terms.

Verification is carved out explicitly and needs no agreement: build the source to
check a published binary against it, verify signatures and checksums, and publish
anything you find. See [../SECURITY.md](../SECURITY.md).

`THIRD-PARTY-NOTICES.txt` ships alongside it. pawl declares no module
dependencies, but the binary statically links the Go runtime and standard
library, which is BSD-3-Clause and requires its notice be reproduced in binary
distributions. Zero dependencies is not zero attribution — it is why that file is
one entry long rather than hundreds.

Both files are listed in `SHA256SUMS` and signed, so the terms you received are
as tamper-evident as the binary you received.

## Uninstalling

Delete the binary. pawl writes only to `.pawl/` inside your repo and starts no
services, so there is nothing else to clean up.

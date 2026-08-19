# PAWL-024 — Licensing and source availability

**Status:** DRAFTED, NOT BUILT · **Module:** repository root, `Makefile`,
`.github/workflows/release.yml`
**Extends:** [PAWL-013](./PAWL-013-versioning-and-release.md) (delivered,
immutable) — that spec defines how a release is produced and verified. This one
defines what legal artifacts accompany it and under what terms the repository is
published. Nothing in PAWL-013 is altered.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Legal) | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

Two questions were open and are now decided: whether the repository is public,
and under what licence the binary is distributed. They interact, so they are
settled together.

**The repository is public; the licence is proprietary.** Source-available:
anyone may read the source and rebuild a tag, nobody acquires a right to use,
modify or redistribute the result outside a commercial agreement.

This shape was chosen over a private repository because GitHub release assets
inherit repository visibility. A private repository serves `404` to anonymous
requests for `releases/download/...`, which would have made three delivered
behaviours permanently inert:

| Behaviour | Under a private repository |
|---|---|
| `install.sh` (PAWL-013 AC14) | cannot fetch anything |
| `pawl install verify` (PAWL-023 AC6) | reports `UNCHECKED` forever |
| `pawl install upgrade` (PAWL-023) | cannot resolve a version |

`UNCHECKED` reported forever is the worse outcome of the two. PAWL-023 was
written so that a check which could not run is never reported as success; a
distribution topology that guarantees it can never run would leave the tool
permanently unable to answer the question it asks everyone else.

A public repository also preserves PAWL-013's non-functional requirement — *the
client is the auditor* — in full. AC11 says a third party must be able to rebuild
a tag and match the published checksum. Under private source that claim could not
have been honoured, and a supply-chain assurance tool whose own provenance rests
on trust rather than evidence argues against itself. Reading the source to verify
a build is permitted; running the result is what the commercial agreement grants.

## Repository

**AC1** — The system shall publish the pawl source repository publicly.
`checkable: yes` — the repository's visibility is observable.

**AC2** — The system shall grant no right to use, modify or redistribute the
software by virtue of publishing its source.
`checkable: partially` — that a licence file stating this is present and shipped
is checkable; the legal effect is not a property of the tree.

> Public source is not open source, and the distinction must survive contact with
> a reader who assumes otherwise. GitHub's terms already permit any user to view
> and fork a public repository regardless of licence, so the licence file governs
> *use*, not *access*, and should not claim otherwise.

## Licence artifacts

**AC3** — The system shall include in the repository a licence file stating the
terms under which the software may be used.
`checkable: yes` (once built)

**AC4** — The system shall include a third-party notices file reproducing the
copyright notice and licence conditions of every work whose licence requires
reproduction in binary distributions.
`checkable: yes` (once built)

> This is an obligation the project currently fails, and the failure is easy to
> miss precisely because of a property the project is proud of: `go.mod` carries
> no `require` block, so the instinct is that nothing is owed. Every pawl binary
> nevertheless links the Go runtime and standard library statically, and Go is
> BSD-3-Clause, whose second condition requires reproducing the copyright notice
> **in binary distributions**. Zero dependencies is not zero attribution.
>
> `docs/install.md` states there is "nothing transitive to review". That remains
> true of *dependencies* and is the reason the notice file is one entry long
> rather than hundreds — but one entry is not none.

**AC5** — When publishing a release, the system shall include the licence file
and the third-party notices file among the release assets.
`checkable: yes` (once built) — a notice that ships only in a repository is not
reproduced in the binary distribution, which is what the condition asks for.

**AC6** — The system shall cover the licence file and the third-party notices
file by the release checksum file.
`checkable: yes` (once built) — the terms a client received should be as
tamper-evident as the binary they received.

## Integrity of the checksum file

**AC7** — The system shall sign every published release asset that is not itself
a signature, including the checksum file.
`checkable: yes` (once built)

> **This closes a defect in PAWL-013 AC8, which requires "a signature for each
> artifact".** The release workflow signs only files matching `pawl-*`, so
> `SHA256SUMS` — the one file `install.sh` treats as authoritative — is published
> unsigned.
>
> The consequence is narrow but real: `install.sh` verifies a downloaded binary
> against `SHA256SUMS` fetched over HTTPS, so its trust chain terminates at
> transport security and GitHub's control of the asset, not at the release
> identity. Anyone able to replace release assets could replace the binary and
> the checksum file together and the installer would report success. Signing the
> checksum file gives a client a path to verify the root of that chain against
> the workflow identity rather than against the transport.
>
> Recorded here as an extension rather than an edit: PAWL-013 is delivered and
> immutable, and the criterion it states is correct — it is the implementation
> that is narrower than the criterion.

## Non-functional

- **The licence must not contradict the product's argument.** pawl asks clients
  to verify rather than trust. Terms that forbade reproducing a build, or
  publishing a discrepancy found while doing so, would make the tool's own pitch
  unavailable to the people it is aimed at. Verification and reporting are
  explicitly permitted.
- **One entry is a feature worth keeping.** The notices file is short because the
  dependency count is zero. If it ever grows, that is a signal about the
  dependency policy, not a licensing chore.
- **Public source raises the cost of a mistaken commit.** Anything ever committed
  to a public repository must be assumed disclosed permanently, including in
  history. This is a precondition of AC1, not a consequence to manage afterwards.

## Out of scope

- **The commercial agreement itself** — pricing, term, support obligations. The
  licence file points at it; it is not drafted here.
- **Contributor licensing.** No external contributions are accepted yet; a CLA or
  DCO decision belongs with that.
- **Per-file copyright headers.** Considered and not required: the licence file
  and notices file discharge the obligations above, and headers in every source
  file are noise this repository does not need.
- **Trademark.** The name is not addressed by a copyright licence.
- **Licensing of other trunion products.** This spec binds pawl only.

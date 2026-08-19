# Security

pawl decides whether changesets can merge, and publishes signed binaries that run
inside other people's pipelines. Both make it worth attacking, and both make a
private reporting route more useful to you than a public one.

## Reporting a vulnerability

**Please report privately, not as a public issue.**

- Preferred: [open a private advisory](https://github.com/trunion-io/pawl/security/advisories/new)
- Or email **security@trunion.io**

Useful in a report: the version (`pawl version`), what you did, what happened,
and what you expected. A reproducer matters more than a severity rating.

### What to expect

| | |
|---|---|
| Acknowledgement | within 3 working days |
| Initial assessment | within 10 working days |
| Fix or a stated plan | agreed with you, based on severity |
| Credit | offered by default; tell us if you would rather not be named |

We will tell you when a fix ships and in which version. If we disagree that a
report is a vulnerability we will say so, and say why.

## Research is explicitly permitted

`LICENSE.txt` is a proprietary licence, and proprietary licences have a history
of being used to discourage exactly this. Clause 2 of ours does the opposite: it
permits anyone, without charge or prior permission, to read the source, rebuild
it to check a published binary against it, verify signatures and checksums, and
**publish what they find, including any discrepancy or vulnerability**.

That is deliberate. This tool asks its users to verify rather than trust, and
terms that made verifying pawl itself impossible would contradict the product.
Nothing in the licence should be read as restricting good-faith security research
or its publication. If anything appears to, that is a drafting bug — report it
and we will fix it.

## Scope

**In scope**

- The pawl binary and this repository's source
- The release pipeline and published artifacts: checksums, signatures, `install.sh`
- The self-management path — `pawl install verify`, `pawl install upgrade`
- Any case where pawl reports a **pass, ok, or verified** result for something it
  did not actually check. This is the class of bug the tool exists to prevent, so
  we treat it as a security issue in pawl even where it is not exploitable — a
  gate that silently approves is indistinguishable from a gate that was bypassed.
- A panic or hang reachable from evidence input, which denies the pipeline pawl
  was installed to protect

**Out of scope**

- Findings in a client's own pipeline that pawl merely reported
- Vulnerabilities in Go itself — report those to the Go project; tell us if pawl
  ships an affected toolchain
- Results from a modified build, unless the modification is the finding
- Missing hardening with no demonstrated impact

## Verifying what you were given

Every release artifact is signed with cosign keyless, bound to the release
workflow's identity — there is no key, so there is no key to have leaked. The
checksum file is signed too, so the chain does not end at transport security.
Builds are reproducible: rebuild the tag and the checksum matches.

See [docs/install.md](docs/install.md) for the commands. **A checksum that does
not match a published one is a finding — report it.**

# PAWL-026 — Policy input validation

**Status:** DRAFTED, NOT BUILT · **Module:** `internal/policy`
**Extends:** [PAWL-006](./PAWL-006-policy-gate.md) — that
spec defines what the thresholds mean and what the gate does with them. This one
defines what happens when the file does not contain a usable threshold. Nothing
in PAWL-006 is altered.

## Stakeholders

| Role | Signature |
|---|---|
| Product | *unsigned* |
| NFR (Security) | *unsigned* |

## Context

Static analysis, running for the first time under PAWL-025 AC6, reported two
high-severity findings in `internal/policy`:

```
internal/policy/policy.go:94   p.MaxChangedLines  = int(v)
internal/policy/policy.go:103  p.MaxUnclaimedLines = int(v)
```

Both convert an `int64` parsed from the policy file to `int` with no range check.
On every platform pawl currently ships — linux, darwin and windows on amd64 and
arm64 — `int` is 64 bits and the conversion cannot truncate, so **this is not
exploitable today**. It is worth fixing anyway, for a reason the severity rating
does not capture.

These values are gate thresholds. C-5 makes the client the owner of them, and
PAWL-006 AC2 and AC4 fail a changeset by comparing against them. A threshold that
silently becomes a different number than the one written in the file is the
failure this product exists to prevent, aimed at itself: the operator reads
`max_unclaimed_lines = 2147483648` in their own repository, and the gate enforces
something else without saying so. `SECURITY.md` scopes that class in explicitly,
and treats it as a security issue whether or not it is reachable.

There is also a gap the scanner did not report, and it is the larger one: **no
criterion says what an invalid threshold does at all.** A negative value, a
string where a number belongs, or a key spelled wrong are all currently accepted
in silence — a misspelled key leaves the built-in default in force while the
operator believes they have configured something. That is C-3's shape, one level
up: silence about a threshold is not evidence that the threshold was applied.

## Validation

**AC1** — Where a threshold in the policy file cannot be represented exactly as
the type the gate uses, the system shall reject the policy and shall not fall
back to a default.
`checkable: yes` (once built) — closes the reported findings by construction
rather than by relying on `int` being 64 bits, which is a property of the current
build targets and not of the language.

**AC2** — Where a threshold is negative, the system shall reject the policy.
`checkable: yes` (once built) — no threshold in PAWL-006 has a meaningful
negative value, and accepting one invites it to be read as "unlimited" by a
comparison that was never written for it.

**AC3** — Where a value in the policy file has a type the threshold cannot
accept, the system shall reject the policy and shall name the key.
`checkable: yes` (once built)

**AC4** — When rejecting a policy, the system shall report the key, the offending
value and the file, and shall exit non-zero.
`checkable: yes` (once built) — an operator holding a rejected policy needs to
know which line to edit; "invalid policy" is not an actionable diagnostic.

**AC5** — The system shall not silently ignore an unrecognised key in the policy
file.
`checkable: partially` — that an unrecognised key produces a diagnostic is
checkable. Whether it should be fatal or a warning is a judgement, and this spec
takes the weaker position deliberately: a typo that leaves a default silently in
force is the same failure as a truncated value, but rejecting outright would
break forward compatibility with policy files written for a later pawl.

> **This is the criterion most likely to be got wrong later.** The temptation on
> a future version bump is to accept unknown keys silently so that old binaries
> tolerate new files. That trade is available, but it must be made by choosing a
> versioning story for the policy schema, not by removing the diagnostic.

**AC6** — The system shall reject a policy no less strictly than it would have
applied it.
`checkable: partially` — the rule behind AC1–AC3: refusing to run is a safe
outcome, and enforcing an unintended threshold is not. Where those conflict,
refuse.

## Non-functional

- **Rejection must fail closed.** A rejected policy means the gate does not run,
  which must never be reported as a pass. C-3 applies directly: no evidence is
  not coverage, and a gate that could not read its own thresholds has produced
  no verdict.
- **The findings are kept, not suppressed.** PAWL-025's non-functional section
  says an acceptable finding is suppressed explicitly with a reason in the tree.
  These are not being suppressed, because a bounds check is cheaper than the
  argument for why one is unnecessary — and the argument depends on a build-target
  property that a future `GOARCH=386` would quietly invalidate.

## Out of scope

- **The meaning of each threshold.** PAWL-006, delivered and unchanged.
- **Configuration precedence, environment variables, and file discovery.**
  PAWL-012, drafted.
- **Versioning of the policy schema.** Named as the correct way to resolve AC5's
  tension, and not decided here.
- **Validation of the calibration or accounting tables.** Same reasoning applies
  and they deserve the same treatment; this spec is scoped to the gate thresholds
  the findings landed on.

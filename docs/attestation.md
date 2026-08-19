# Attestation

`pawl attest` emits an [in-toto Statement v1](https://in-toto.io/Statement/v1)
describing what was assumed while a changeset was written, and what happened when
those assumptions were checked.

## The subject is the tree, not a build

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [{
    "name": "git@github.com:acme/service.git",
    "digest": {
      "gitTree": "e0845caa150e2a091739fb08d98cf146d8cb75c0",
      "gitCommit": "ef76e0e1539865a761f7dc559214f3edabc97564"
    }
  }],
  "predicateType": "https://trunion.io/attestations/assumption-trail/v0.1",
  "predicate": { }
}
```

SLSA v1.2 promoted the Source track to approved and deliberately leaves source
provenance attestations undefined, up to whoever implements them. This predicate
occupies that slot.

Binding to an image digest would be the wrong anchor. In agentic delivery the
**changeset is the deliverable** and the build is downstream of it; an
attestation about the image cannot tell you anything about what the agent
assumed while writing the code that went into it.

The predicate type URL describes the **artifact**, not the tool. It survived a
`factory-kit` → `pawl` rename and a Python → Go rewrite unchanged, deliberately.
A consumer parsing these should key on the URL, not on anything about pawl.

## What is in the predicate

| Field | What |
|---|---|
| `schemaVersion` | Predicate schema version |
| `generatedAt` | RFC3339 timestamp |
| `base`, `commit` | The changeset boundaries |
| `ticket`, `policyPack` | Free-form provenance, from flags |
| `summary` | Changed lines, must-read lines, collapse ratio, claim counts |
| `claims[]` | Every claim: text, kind, recorded and anchored ranges, anchor status, coverage status, coverage detail, asserted checks, author role, harness, model, verdict |
| `readingList[]` | The spans that reached a human, with the claims responsible |
| `authorRoleBreakdown` | Claims and escalations per author role |

`authorRoleBreakdown` is there for calibration rather than for the gate. The
ratio of clean clears on client-authored changesets over time is the leading
indicator that an engagement is finishable.

## Reading a trail

The summary is what goes in front of a client:

```json
"summary": {
  "changed_lines": 5,
  "must_read_lines": 2,
  "reduction_pct": 60,
  "claims": 2,
  "claims_needing_human": 1,
  "spans": 2,
  "unclaimed_lines": 0
}
```

`unclaimed_lines` is the number to watch. It counts changed code that carried no
claim at all, and it is the honest measure of whether claiming is actually
happening. A trail with a beautiful collapse ratio and a high unclaimed count is
measuring a small annotated corner of a large unannotated change.

Each claim records **why** it landed where it did:

```json
{
  "id": "23dd1682a293",
  "kind": "assumption",
  "text": "Refresh is a passthrough until rotation lands",
  "anchor": "anchored",
  "coverage": "unverified",
  "coverage_detail": ["test not found in results: TestRotation"],
  "needs_human": true
}
```

`coverage_detail` is the audit trail. An assertion that a check existed, and
pawl's finding that it did not, are both recorded — which is more useful than
either a pass or a fail on its own.

## Signing

pawl **does not sign**. That is deliberate and will not change.

```bash
cosign attest-blob \
  --predicate assumption-trail.intoto.json \
  --type https://trunion.io/attestations/assumption-trail/v0.1 \
  --yes --bundle assumption-trail.sigstore.json \
  assumption-trail.intoto.json
```

Keyless signing off a CI OIDC token means there is no key custody to negotiate
with a security team, and no key to rotate or leak. Reimplementing signing would
be a liability rather than a feature.

## Verifying a trail

```bash
cosign verify-blob-attestation \
  --certificate-identity-regexp 'https://github.com/acme/service/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --type https://trunion.io/attestations/assumption-trail/v0.1 \
  --bundle assumption-trail.sigstore.json \
  assumption-trail.intoto.json
```

Then check the `gitTree` digest in the subject against the tree you are about to
promote. A valid signature over a trail for a *different* tree proves nothing
about the one in front of you.

## Storage

pawl does not store attestations. It writes a file; what happens next is yours.

[Archivista](https://github.com/in-toto/archivista) is purpose-built for in-toto
attestation storage and builds a queryable graph from predicate types and
subjects. [GUAC](https://guac.sh) sits at the aggregation layer above it. Either
is the right home for these once you have enough to query — neither belongs on
day one, and pawl will not grow into one.

## Known gap

The predicate does **not currently record which pawl produced it** — there is no
tool version or binary digest in it. An auditor holding a signed trail cannot
tell whether a permissive old verifier or the current one cleared those lines.

For a provenance tool this is the most significant gap in the schema, and it is
being specified before a first client. Until then, record the pawl version
alongside the trail yourself — the CI example passes it as `--policy-pack`.

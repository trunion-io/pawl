# Repository setup

The GitHub-side configuration this repository expects, recorded as files so it
can be reviewed in a diff and reapplied from scratch.

`_setup` rather than `setup`, for the same reasons as [`_spec`](../../_spec):
it sorts above the source directories, it marks itself as not-source, and it
stays out of tooling globs.

These are **not applied automatically.** Applying them changes who can merge what,
so it is a deliberate act with a human behind it.

## Why these live in the tree

PAWL-025 records three criteria as `checkable: partially` — AC7 (secret scanning),
AC10 (checks required before merge) and AC11 (no force-push or deletion) — on the
grounds that they are repository settings rather than properties of the tree.
That reasoning is sound and these files do not overturn it: GitHub remains the
enforcement point, and nothing here proves what is live.

What they do change is that the *intended* configuration is now reviewable,
diffable, and recoverable. A setting that exists only in a web form is one nobody
can review and nobody can restore.

## Applying

Order matters. The ruleset requires status checks and a pull request, so applying
it to an empty repository blocks the first push of `main`.

```bash
# 1. Repository settings
gh api -X PATCH repos/trunion-io/pawl --input .github/_setup/repo.json

# 2. Push main and at least one branch first, then the ruleset
gh api -X PUT repos/trunion-io/pawl/rulesets/<id> \
  --input .github/_setup/ruleset-main.json

# On a fresh repository there is no id yet, so create rather than update:
gh api -X POST repos/trunion-io/pawl/rulesets \
  --input .github/_setup/ruleset-main.json
```

Actions settings live on their own endpoints and are recorded in
`actions.json`, which is a record rather than a single request body:

```bash
gh api -X PUT repos/trunion-io/pawl/actions/permissions \
  -F enabled=true -f allowed_actions=all
gh api -X PUT repos/trunion-io/pawl/actions/permissions/workflow \
  -f default_workflow_permissions=read -F can_approve_pull_request_reviews=false
```

`default_workflow_permissions: read` is load-bearing: PAWL-025 AC3 requires each
job to hold the minimum token it needs, and a read-only default is what makes the
`permissions:` block in each workflow an increase from a safe floor rather than a
decrease from a permissive one.

`can_approve_pull_request_reviews: false` keeps `GITHUB_TOKEN` from approving
pull requests. Nothing here should be able to satisfy a review requirement by
running.

`fork-pr-contributor-approval` is set to `all_external_contributors`, the
strictest of the three policies: a workflow proposed from a fork runs only after
a maintainer approves it, every time, not just on a contributor's first pull
request.

This matters more here than the setting's name suggests. PAWL-025 AC4 refuses any
trigger that hands a fork's code access to repository secrets, and this is the
other half of that argument — AC4 governs what the workflows declare, and this
governs whether an unreviewed fork gets to run them at all. The release job holds
`id-token: write` and can sign artifacts as pawl, so "runs automatically on a
proposal from a stranger" is not a posture this repository can hold.

```bash
gh api -X PUT repos/trunion-io/pawl/actions/permissions/fork-pr-contributor-approval \
  -f approval_policy=all_external_contributors
```

Two settings have no representation in `repo.json` because they are separate
endpoints:

```bash
gh api -X PUT repos/trunion-io/pawl/automated-security-fixes
gh api -X PUT repos/trunion-io/pawl/private-vulnerability-reporting
```

The second is what makes the reporting route in [`SECURITY.md`](../../SECURITY.md)
resolve. Without it that link is a dead end, which is worse than not offering one.

## Choices in here that are load-bearing

**Squash only.** `allow_merge_commit` and `allow_rebase_merge` are false so the
repository offers what the ruleset permits. They were both true, which put buttons
in front of a maintainer that the ruleset would then refuse — a UI that disagrees
with the rules teaches people to distrust the rules.

**`squash_merge_commit_title: PR_TITLE`.** The default, `COMMIT_OR_PR_TITLE`, uses
the branch commit's subject when a pull request has exactly one commit and the
pull request title otherwise. That makes the subject reaching `main` depend on how
many commits a branch happens to have. PAWL-027 derives the version from what
reaches `main`, so the input to versioning must not be decided by a coincidence.

**`squash_merge_commit_message: COMMIT_MESSAGES`.** This is what carries the
individual commit bodies into the squashed commit, and with them the
`Verdict-Affecting:` trailers that PAWL-027 AC3 reads. `PR_BODY` would discard
them, and the version computation would silently stop seeing declarations that
were correctly made.

**No bypass beyond organisation admin.** The ruleset originally granted bypass to
the admin repository role and to deploy keys as well. A repository role is an
everyday identity rather than a deliberate one, and a deploy key is a credential
that cannot exercise judgement at all. One break-glass route held by a person is
a different thing from rules that are advisory.

## What is not here

- **Secret values.** Nothing readable, and there are none.
- **Proof of what is live.** These files say what is intended. Confirm the live
  state with `gh api`, and confirm protection actually holds by attempting a push
  and being refused — a readback tells you what the API stored, not what it
  enforces.

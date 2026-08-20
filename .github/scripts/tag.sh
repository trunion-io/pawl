#!/bin/sh
# Create and push an annotated tag (PAWL-027).
#
# The single place a tag is written. Two workflows needed this and only one
# configured a tagger, so every release candidate computed the right version and
# then failed at the write — an annotated tag records a tagger and a runner has
# no identity.
#
# Centralising it turns an invariant that could only be approximated into one
# that can be checked exactly: `make check-tagger` asserts no workflow invokes
# `git tag` directly, which a grep can establish, where "the tagging job also
# sets an identity somewhere in the file" could not.
#
# usage: tag.sh [--retry-same-commit] <tag> <commit-ish> <message...>
set -eu

retry_same_commit=0
if [ "${1:-}" = "--retry-same-commit" ]; then
  retry_same_commit=1
  shift
fi

[ "$#" -ge 3 ] || { echo "usage: tag.sh [--retry-same-commit] <tag> <commit-ish> <message...>" >&2; exit 2; }
tag=$1; target=$2; shift 2

# An existing tag is fatal by default.
#
# PAWL-027 AC15 refuses to release where the computed version already exists as a
# tag. nextversion checks that, but a tag can appear between its check and this
# write — and an unconditional "already exists, nothing to do" would let the
# release continue to build, sign and publish as though it had tagged this
# commit, when the existing tag may point somewhere else entirely. That is the
# same failure as the ones already found in nextversion: fails open, and fails
# open toward publishing.
#
# --retry-same-commit exists for release candidates, where the workflow can
# legitimately run twice for one commit. It permits an existing tag only when it
# already points at the commit being tagged; a candidate tag on a different
# commit is a real problem and still fatal.
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null 2>&1; then
  existing=$(git rev-parse "refs/tags/$tag^{commit}")
  wanted=$(git rev-parse "$target^{commit}")

  if [ "$retry_same_commit" = 1 ] && [ "$existing" = "$wanted" ]; then
    echo "tag.sh: $tag already points at $(git rev-parse --short "$wanted"); nothing to do"
    exit 0
  fi

  echo "tag.sh: $tag already exists" >&2
  echo "        existing: $(git rev-parse --short "$existing")" >&2
  echo "        wanted:   $(git rev-parse --short "$wanted")" >&2
  exit 1
fi

# Both are required. Git refuses with "empty ident name" if either is missing,
# and it refuses at the tag write rather than at configuration time.
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

git tag -a "$tag" "$target" -m "$*"
git push origin "$tag"
echo "tag.sh: tagged $tag at $(git rev-parse --short "$target")"

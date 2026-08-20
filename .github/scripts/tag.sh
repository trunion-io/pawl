#!/bin/sh
# Create and push an annotated tag (PAWL-027).
#
# The single place a tag is written. Two workflows needed this and only one
# configured a tagger, so every release candidate computed the right version and
# then failed at the write — an annotated tag records a tagger and a runner has
# no identity.
#
# Centralising it means one place to test rather than one claim to approximate.
# `make check-tagger` lints for workflows writing a tag directly, but it is a
# grep and misses what a grep must: indirection like `g=git; $g tag`, and a
# command split across source lines by YAML folding. It cannot establish that
# every workflow uses this script.
#
# What is established is this script's behaviour, by TestTagScript* in
# internal/e2e — real git, a real bare remote, identity unset.
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

  # Same commit is not sufficient: a lightweight tag points at the right commit
  # while carrying no annotation and no tagger, which is the defect this script
  # exists to prevent. Taking the no-op path there would leave it in place and
  # report success.
  kind=$(git cat-file -t "refs/tags/$tag" 2>/dev/null || echo unknown)

  if [ "$retry_same_commit" = 1 ] && [ "$existing" = "$wanted" ] && [ "$kind" = tag ]; then
    echo "tag.sh: $tag already points at $(git rev-parse --short "$wanted"); nothing to do"
    exit 0
  fi

  if [ "$retry_same_commit" = 1 ] && [ "$existing" = "$wanted" ] && [ "$kind" != tag ]; then
    echo "tag.sh: $tag exists at the right commit but is $kind, not an annotated tag" >&2
    echo "        refusing to treat it as a completed candidate" >&2
    exit 1
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

# One -m per argument: git renders each as its own paragraph, which is what both
# workflows had before this script existed. "$*" joined the subject and every
# body line into a single line.
n=$#
i=0
while [ "$i" -lt "$n" ]; do
  m=$1; shift
  set -- "$@" -m "$m"
  i=$((i + 1))
done
git tag -a "$tag" "$target" "$@"
git push origin "$tag"
echo "tag.sh: tagged $tag at $(git rev-parse --short "$target")"

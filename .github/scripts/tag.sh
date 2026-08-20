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
# usage: tag.sh <tag> <commit-ish> <message...>
set -eu

[ "$#" -ge 3 ] || { echo "usage: tag.sh <tag> <commit-ish> <message...>" >&2; exit 2; }
tag=$1; target=$2; shift 2

if git rev-parse -q --verify "refs/tags/$tag" >/dev/null 2>&1; then
  echo "tag.sh: $tag already exists; nothing to do"
  exit 0
fi

# Both are required. Git refuses with "empty ident name" if either is missing,
# and it refuses at the tag write rather than at configuration time.
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

git tag -a "$tag" "$target" -m "$*"
git push origin "$tag"
echo "tag.sh: tagged $tag at $(git rev-parse --short "$target")"

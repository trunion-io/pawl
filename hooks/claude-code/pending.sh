#!/usr/bin/env bash
#
# pawl edit-time accounting hook for Claude Code (PAWL-016).
#
# Fires PostToolUse on Edit|Write|MultiEdit. Reports the spans the agent just
# changed that carry neither a claim nor an acknowledgement, at the moment the
# reasoning behind them is still in context.
#
# It does not enforce. Enforcement is the gate's job — `max_unclaimed_lines`
# already blocks a merge on unaccounted code. What only a hook can do is supply
# the span *now*, so the agent's answer is evidence rather than a reconstruction
# from a finished diff. That distinction is C-2, and it is the whole point.
#
# Every failure path exits 0 (AC9). A tool that breaks a client's agent when it
# malfunctions gets uninstalled the first time it does, and would deserve to be.

set -uo pipefail

emit_nothing() { exit 0; }

payload=$(cat) || emit_nothing
command -v jq >/dev/null 2>&1 || emit_nothing

file=$(printf '%s' "$payload" | jq -r '.tool_input.file_path // .tool_response.filePath // empty' 2>/dev/null) || emit_nothing
[ -n "$file" ] || emit_nothing

# hooks/claude-code/pending.sh -> repo root
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." 2>/dev/null && pwd) || emit_nothing

# Only speak about files inside this repository.
case "$file" in
  "$repo"/*) rel="${file#"$repo"/}" ;;
  /*)        emit_nothing ;;
  *)         rel="$file" ;;
esac

# Prefer the local build, so a dev iterating on pawl gets the pawl they just
# built rather than whatever is installed globally.
if [ -x "$repo/bin/pawl" ]; then
  pawl="$repo/bin/pawl"
elif command -v pawl >/dev/null 2>&1; then
  pawl="pawl"
else
  emit_nothing
fi

spans=$(cd "$repo" && "$pawl" pending --json --repo . "$rel" 2>/dev/null) || emit_nothing
[ -n "$spans" ] || emit_nothing

count=$(printf '%s' "$spans" | jq 'length' 2>/dev/null) || emit_nothing
[ "$count" -gt 0 ] 2>/dev/null || emit_nothing

lines=$(printf '%s' "$spans" | jq -r '[.[] | .end_line - .start_line + 1] | add' 2>/dev/null)

# Cap the enumerated ranges. This fires on every edit, so the injected context
# is a recurring cost paid out of the agent's own budget — the thing AC3 was
# written to bound. Six is enough to act on; the count above carries the rest.
readonly MAX_RANGES=6
ranges=$(printf '%s' "$spans" | jq -r --argjson n "$MAX_RANGES" \
  '[.[range(0; ([length, $n] | min))] | "\(.path):\(.start_line)-\(.end_line)"] | join(", ")' 2>/dev/null)
if [ "$count" -gt "$MAX_RANGES" ]; then
  ranges="${ranges}, and $((count - MAX_RANGES)) more (pawl pending)"
fi

# `spec:` cannot resolve until the spec tool exists (PAWL-009), so a claim citing
# it would be permanently unverified. AC10: do not suggest it.
context=$(cat <<EOF
pawl: ${count} span(s), ${lines} line(s) in ${rel} carry no claim or acknowledgement yet.

  ${ranges}

Account for them now, while the reasoning is still in context — recording this
later against a finished diff is reconstruction, which is what C-2 forbids.

  pawl claim "<what you assumed>" --path <file> --lines <a-b> [--verified-by test:<TestName>]
  pawl ack --path <file> --lines <a-b>     # nothing to assume here

Use ack freely for mechanical edits; it is cheap and it is measured. Use claim
when you assumed something, rejected an alternative, or could not establish
something and proceeded anyway.
EOF
)

jq -cn --arg ctx "$context" \
  '{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: $ctx}}'

#!/bin/sh
# SessionEnd hook: prepends "<session id> | <summary>" to alligator/history.txt.
# The nested `claude -p` fires this hook too: ALLIGATOR_SUMMARY guards the recursion.
[ -n "$ALLIGATOR_SUMMARY" ] && exit 0

input=$(cat)
session_id=$(echo "$input" | jq -r .session_id)
transcript=$(echo "$input" | jq -r .transcript_path)
history="${CLAUDE_PROJECT_DIR:-.}/alligator/history.txt"

# Summarize only the user prompts, in background: the hook must not block the exit
(
  prompts=$(jq -r 'select(.type=="user" and (.message.content|type)=="string") | .message.content' "$transcript" | tail -c 50000)
  [ -z "$prompts" ] && exit 0
  summary=$(echo "$prompts" | ALLIGATOR_SUMMARY=1 claude -p --model haiku --no-session-persistence \
    "These are the user prompts of a coding session. Summarize what was worked on in 20-30 words, one line, no preamble." \
    | tr '\n' ' ')
  [ -z "$summary" ] && exit 0
  mkdir -p "$(dirname "$history")"
  { echo "$session_id | $summary"; cat "$history" 2>/dev/null || true; } > "$history.tmp" && mv "$history.tmp" "$history"
) >/dev/null 2>&1 &

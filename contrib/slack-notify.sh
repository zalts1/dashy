#!/bin/sh
# board -> Slack sink. Reads board's notification JSON on stdin.
#
# One-time setup:
#   security add-generic-password -s board-slack-webhook -a "$USER" \
#     -w 'https://hooks.slack.com/services/XXX/YYY/ZZZ'
#
# Only pushes when the keyboard has been idle past BOARD_AWAY_SECONDS. The point
# is being told about an agent you *aren't* watching; pinging Slack while you sit
# at the terminal is how a notification channel earns a mute.
#
# Never fails: a broken webhook must not break the agent that triggered it.

AWAY_SECONDS="${BOARD_AWAY_SECONDS:-300}"
payload=$(cat)

idle=$(ioreg -c IOHIDSystem 2>/dev/null | awk '/HIDIdleTime/ {print int($NF/1000000000); exit}')
[ -n "$idle" ] || idle=0

if [ -z "${BOARD_SLACK_DRYRUN:-}" ] && [ "$idle" -lt "$AWAY_SECONDS" ]; then
	exit 0
fi

text=$(printf '%s' "$payload" | jq -r '
	(if .state == "needs input" then ":bell:" else ":white_check_mark:" end) as $icon
	| "\($icon) *\(.state)* — *\(.label)*  `\(.workspace)`"
	  + (if (.message // "") == "" then "" else "\n> \(.message)" end)
' 2>/dev/null)
[ -n "$text" ] || exit 0

body=$(jq -n --arg t "$text" '{text: $t}' 2>/dev/null)
[ -n "$body" ] || exit 0

if [ -n "${BOARD_SLACK_DRYRUN:-}" ]; then
	printf '%s\n' "$body"
	exit 0
fi

url=$(security find-generic-password -s board-slack-webhook -w 2>/dev/null)
[ -n "$url" ] || exit 0

curl -sS -m 10 -X POST -H 'Content-type: application/json' --data "$body" "$url" >/dev/null 2>&1
exit 0

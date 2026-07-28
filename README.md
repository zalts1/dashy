# board

One screen showing every live Claude Code session running under cmux.

```
$ board
STATE        LABEL                                        WORKSPACE           IDLE
blocked → ⚠  merge app#1497 before branching              DLP PHISHING       3h00m
blocked →    migrate auth handlers to v2                  AUTH SVC              9m
done ⚠       Review template-runner PR #541               RAW EVIDENCE       2h48m
running      build board: cmux fleet status CLI           KILLSWITCH            0m

22 sessions · 2 blocked · 1 running · 20 quiet >45m
```

## Commands

    board                     print the screen
    board label "<text>"      label the current session (no text clears it)
    board install-hooks       merge Stop + Notification hooks into ~/.claude/settings.json

## States

| state | meaning | derived from |
|---|---|---|
| `blocked →` | genuinely needs an answer | newest cmux Feed event is `question`, `permissionRequest`, or `exitPlan` |
| `running` | working | cmux `agentLifecycle: running` |
| `done` | finished its turn, unnoticed | everything else |

`⚠` marks a quiet session past the idle threshold (default 45m).

**Why `done` and not `waiting`:** cmux's `needsInput` lifecycle fires ~60 seconds
after *any* finished turn, so it means "sitting at the prompt", not "asked you
something" — on a typical fleet it covers most sessions. `blocked` is derived from
unresolved Feed cards instead, so it stays rare and worth reacting to.

**Known gap:** the event scan reads the last 8MB of `~/.cmuxterm/workstream.jsonl`
(~2 days). A session blocked longer ago shows as `done ⚠` rather than `blocked`.

## Config — `~/.board.json`

```json
{
  "config": {
    "idle_threshold_minutes": 45,
    "notify_cmd": "curl -sS -d @- https://ntfy.sh/my-topic"
  },
  "labels": { "<cmux surface id>": "<label>" }
}
```

`notify_cmd` runs via `sh -c` on every `Stop` and `Notification` hook and receives
this JSON on stdin. Empty (the default) means notifications are off.

```json
{
  "event": "Notification",
  "state": "needs input",
  "label": "migrate auth handlers to v2",
  "workspace": "AUTH SVC",
  "surface_id": "D39C7A0C-…",
  "cwd": "/Users/you/work/repo",
  "message": "Claude needs your permission to use Bash",
  "text": "needs input — migrate auth handlers to v2 [AUTH SVC]: Claude needs your permission to use Bash"
}
```

Use `.text` for a ready-made one-liner; it always carries the label and workspace
name. Sink failures are swallowed so a broken webhook never blocks an agent.

## Uninstall

Remove the two `board notify` entries from `~/.claude/settings.json` (a timestamped
`.board-bak-*` copy is written before the first change), then delete `~/.board.json`
and the binary. Labels live only in `~/.board.json`; nothing else is written.

## Slack

`contrib/slack-notify.sh` (installed as `~/.local/bin/board-slack-notify`) posts to
a Slack incoming webhook. Store the URL in the Keychain rather than in
`~/.board.json` — it is a credential:

    security add-generic-password -s board-slack-webhook -a "$USER" \
      -w 'https://hooks.slack.com/services/XXX/YYY/ZZZ'

It only posts when the keyboard has been idle past `BOARD_AWAY_SECONDS` (default
300), so it pings you about agents you are *not* watching. Without that gate a
22-session fleet would post on every turn end and get muted within a week.

Test it without sending anything:

    echo '{"state":"needs input","label":"demo","workspace":"WS","message":"hi"}' \
      | BOARD_SLACK_DRYRUN=1 ~/.local/bin/board-slack-notify

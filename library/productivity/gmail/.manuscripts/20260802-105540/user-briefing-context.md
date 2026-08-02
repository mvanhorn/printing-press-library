# USER_BRIEFING_CONTEXT (captured mid-research, 2026-08-02)

User request: "I want to add the ability to the CLI to schedule emails"

Interpretation: scheduled email sending (schedule-send). The Gmail API has no
native schedule-send endpoint, so this must be built as a local feature:
- SQLite-backed schedule queue (compose now, send later)
- schedule send --at / --in, schedule list, schedule cancel, schedule edit
- schedule run (process due items; cron/launchd-friendly) + --watch daemon mode
- Idempotent sends (record sent items, never double-send)

This is shipping-scope user vision: include in brief ## User Vision, feed to
novel-features subagent Pass 2 source (e), surface at Phase Gate 1.5.

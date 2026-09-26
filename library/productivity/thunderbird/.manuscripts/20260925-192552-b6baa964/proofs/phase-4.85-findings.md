# Phase 4.85 agentic output review — WARN (3 warnings, non-blocking)
Live-check sampled an empty store; review re-ran the 5 novel commands against the real synced store (53,957 messages). Real addresses redacted.

1. awaiting-reply --days 14: 25 threads, 19 are automated notifications from a single DevOps notification sender (no human waiting). Fix: exclude automated/bulk senders (Auto-Submitted, Precedence: bulk, List-Id/List-Unsubscribe, noreply/notification patterns, one-directional high-volume senders).
2. newsletters --since 90d --min 3: only 4 low-volume groups; high-volume one-directional notification senders (hundreds–thousands received, 0 sent) are missing because classification is header-only. Fix: classify one-directional high-volume / Auto-Submitted senders as bulk, shared with awaiting-reply.
3. contacts top: display names keep literal surrounding quotes from headers. Fix: unquote RFC 5322 display names.

Correct: largest --attachments ordering; filters audit hit counts and target folders; contacts top ranking.
Disposition: fix in Phase 4.95 autofix round (1-3 file changes).

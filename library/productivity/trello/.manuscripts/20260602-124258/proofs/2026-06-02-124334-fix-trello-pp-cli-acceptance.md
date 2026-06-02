# Trello CLI Live Acceptance (Full Dogfood)

Authenticated as the test account (identity redacted). Synced a multi-board
account into local SQLite (boards, lists, cards, members, actions, checklists).

## Results (all PASS against real account)
- trello-sync: all open boards mirrored
- overdue: cross-board overdue sweep returned results
- workload: per-member load across all boards (identities redacted)
- velocity: weekly completion counts over 8 weeks
- cycletime: median/p90 over completed cards
- bottleneck: clogged lists ranked by count + age
- blocked: 0 (valid empty; account uses no blocked labels)
- churn: backward-move detection returned results
- checklist-progress: cards below 100% completion
- stale: cards inactive >365 days

## Bugs found & fixed live (would have shipped broken)
1. DUAL-AUTH GAP: generator wired only the key query param, never token.
   Trello needs both -> every authed call returned "invalid token".
2. NO BULK-LIST SYNC: Trello has no top-level collection endpoint; generic
   sync pulled 0 records. Fixed with a hand-authored trello-sync traversal.
3. stale missed Trello cards: framework field list lacked dateLastActivity.
4. Write commands required --key/--token flags instead of env fallback.

## Known-untestable edge endpoints (not defects)
batch, search (x2), and sessions/socket cannot be auto-probed by the generic
live matrix (special inputs / websocket-only). Verified working manually.

Gate: PASS (core CRUD + 8 novel analytics verified live)

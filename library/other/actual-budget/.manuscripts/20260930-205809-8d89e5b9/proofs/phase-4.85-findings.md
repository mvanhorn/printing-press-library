# Phase 4.85 — Agentic output review (actual-budget-pp-cli)

Status: WARN (5 warnings, 0 blockers). Reviewed against a real-data mirror (read-only;
313 txns / 121 payees / 2026-07..09). No personal values recorded here.

| # | Check | Finding | Disposition |
|---|-------|---------|-------------|
| 1 | output-semantically-matches-query-intent | `audit payees` clustered "Transfer to X" with "Transfer from X" (0.925) with a ready merge_command | FIXED: opposing direction words (to/from, in/out, credit/debit, deposit/withdrawal, sent/received) never cluster; real mirror shows 0 such clusters |
| 2 | result-ordering-ranking-makes-sense | whole-word prefix ("Store" vs "Store Fuel") scored 1.0, ranking above real typo variants | FIXED: prefix match scores 0.9; 1.0 reserved for names identical after normalization |
| 3 | result-ordering-ranking-makes-sense | `ledger duplicates --days 7` returned 28 pairs, 24 of them weekly recurring charges | FIXED: a pair that repeats at the same interval before/after is labelled "looks recurring" and loses 0.3 confidence; real mirror now 6 pairs at --days 7 |
| 4 | no-obvious-format-bugs | `changes since` showed raw FK UUIDs, YYYYMMDD ints and raw_synced_data despite "names resolved" | FIXED: payee/account/category/group ids → names (id kept as `<col>_id`), dates ISO, raw_synced_data dropped; help updated; TestChangesSinceResolvesNames |
| 5 | aggregation-shows-all-sources | scorecard live-check samples ran against an empty store (no mirror selected), and `ledger sql` sample failed the query-token check | RETRO (press): live-check should seed mirror-backed commands and exempt raw-SQL commands from outputMentionsQuery |

Correctly empty on this budget: audit rules / audit schedules / templates status (budget has no
rules, schedules or #template notes) and changes since 7d (no recent edits).

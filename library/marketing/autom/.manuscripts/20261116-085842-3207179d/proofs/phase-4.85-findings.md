# Phase 4.85 output review

Status: WARN (partial coverage). Run without an API key: engines compare, geo compare and news watch failed with HTTP 401, so query relevance, source coverage and ranking were not assessed; they are covered by the user's live dogfood.

Findings (both fixed in-session):
1. visibility JSON reported avg_position/best_position 0 for a domain never found. Now null, with a note when nothing is tracked; test added.
2. credits reported meta.source "live" when the balance call failed. Now "local" when only the ledger answered.

# Phase 4.95 — Local Code Review Findings (hand-coded files)

Reviewer: pr-review-toolkit:code-reviewer over the 7 hand-authored Go files.
All findings autofixed in-place (no design tradeoffs). 0 routed to retro.

| # | Sev | File | Finding | Fix |
|---|-----|------|---------|-----|
| 1 | Med | makerworld_novel.go, tags.go | rows.Scan errors silently `continue`, shrinking the analyzed corpus → wrong rankings | Return scan errors (`return nil, fmt.Errorf(...)`); keep NULL/parse-fail skips |
| 2 | Med | movers.go, designers_deltas.go | constant `"unknown"` sync-timestamp fallback collides under INSERT OR IGNORE → deltas could never accumulate | Monotonic `time.Now().UTC().Format(RFC3339Nano)` fallback |
| 3 | Med | discover.go | enrichment fetch failure silently drops a matching design + exhausts the cap | Track `enrichFailures`, emit a stderr warning; cap still bounds work |
| 4 | Low | discover.go (+movers/tags/deltas) | negative `--limit` panics `make(..., flagLimit)` | Reject `--limit < 0` with usageErr |
| 5 | Low | download.go | truncated body (200 + short read) reported as success | Compare bytes written against `resp.ContentLength`, error + cleanup on mismatch |

Clean: favorites.go, aggregateTags/matchAllTags, computeMovers/aggregateDesignerDeltas, all SQL parameterized, all rows/files/bodies closed, download token correctly stripped on cross-host CDN redirect.

Post-fix: build OK, vet clean, all unit tests pass, movers/discover re-verified live.

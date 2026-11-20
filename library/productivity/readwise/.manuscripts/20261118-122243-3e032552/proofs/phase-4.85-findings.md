# Phase 4.85 agentic output review (Wave B: warnings only)
Status: WARN. 10 findings, 0 blocking. It ran in a sandbox HOME against a copy of the store (2026-08-29 data). The scorecard live-check review_status is pass (7 passed; watch and webhook skipped as mutating).

Checked and plausible: ask "habit" (relevant), digest (5 highlights, matching the store; they are CDR smoke fixtures), search "stoicism" (correct empty), dedupe 618/635 (consistent; 5 docs appear in two clusters), triage 3529 (consistent with 2025 bulk import).

| # | Command | Finding | Class |
|---|---|---|---|
| 1 | connections --library-wide | The near-duplicate URL check never fires: connections.go:140 loader selects no URL. The top pairs are the same page saved twice, and only the site domain links them as "author" | correctness bug |
| 2 | triage stale --zero-highlights | Uses stale books.data.num_highlights. Two "orphans" have 4 and 1 local highlights, which disagrees with digest | correctness bug |
| 3 | read-next | 33-way tie at score 35, so top-N is arbitrary and unstable. `--prefer` silently no-ops when nothing matches | ranking |
| 4 | backlog | `--sort priority` equals `--sort saved`, and the score column disagrees with the sort | ranking |
| 5 | backlog/read-next | 15% of docs have unknown length, silently estimated at 1 minute, so budget worklists fill with empty stubs | format/honesty |
| 6 | backlog | With no --location, counts archived and unknown-location docs as unread debt | aggregation |
| 7 | dedupe | suggested_keep ties go to the tracking-param URL. Login and 403 interstitial titles cluster together | quality |
| 8 | triage/dedupe/backlog | Unbounded output (about 740 KB / 680 KB), no totals, no --limit | agent bounding |
| 9 | backlog/read-next | --db-path help shows two defaults and leaks the absolute HOME; search names the flag --db | cosmetic |
| 10 | search (framework) | A successful local search still prints a stderr notice and `meta.reason: no_search_endpoint` | template → retro candidate |

Raw outputs are at scratchpad/orv/ (private: real titles).

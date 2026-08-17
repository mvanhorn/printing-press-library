# Scrape Creators Absorb Manifest (reprint, press 4.30.1)

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | All 175 endpoint calls | Official ScrapeCreators CLI (endpoint mirror) | (generated endpoint) every resource/endpoint from the official OpenAPI spec fetched 2026-08-08 (175 paths; 12 new since June incl. IG replies, IG search, Apple Music, Snapchat Spotlight; /v1/tiktok/hashtags/popular removed) | typed flags, --json/--select/--compact, --dry-run, exit codes, credit fields preserved |
| 2 | Cursor pagination --all | prior PP print (patch scrape-creators-pagination-cursor-wiring) | (behavior in scrape-creators-pp-cli feed commands) --all follows per-platform response cursors, 100-page cap | official CLI has no pagination automation |
| 3 | Agent-honest compact envelopes | prior PP print (patch scrape-creators-compact-payload-array) | (behavior in scrape-creators-pp-cli comments commands) compact never strips the sole payload array | official CLI has no compact mode |
| 4 | MCP server for agents | Official ScrapeCreators CLI (MCP mode) | (generated) scrape-creators-pp-mcp, runtime Cobra-tree mirror, read-only hints | plus local-store tools the official MCP lacks |
| 5 | Workflow methodology + operational tables | Official scrapecreators skills repo (13 curl workflows) | (behavior in SKILL.md) per-endpoint credit costs, known limitations, cursor cheatsheet, child_comment_count caveat | executable against one binary instead of hand-rolled curl |
| 6 | Local sync + FTS5 search | prior PP print | (generated) sync/search/analytics/tail + transcripts FTS5; spec declares sync-eligible resources + ID keys (fixes "no extractable ID field" warnings) | neither official tool has persistence |
| 7 | Cross-platform presence matrix | prior PP print novel | scrape-creators-pp-cli creator find | preserved via regen-merge |
| 8 | Multi-creator comparison | prior PP print novel | scrape-creators-pp-cli creator compare | preserved via regen-merge |
| 9 | Engagement spike detector | prior PP print novel | scrape-creators-pp-cli content spikes | preserved via regen-merge |
| 10 | Transcript full-text search | prior PP print novel | scrape-creators-pp-cli transcripts search | preserved via regen-merge |
| 11 | Trend triangulation | prior PP print novel | scrape-creators-pp-cli trends triangulate | NOTE: /v1/tiktok/hashtags/popular removed from spec — verify fan-out list on regen |
| 12 | Follower growth tracker | prior PP print novel | scrape-creators-pp-cli creator track | preserved via regen-merge |
| 13 | Brand ad campaign monitor | prior PP print novel | scrape-creators-pp-cli ads monitor | preserved via regen-merge |
| 14 | Credit burn projection | prior PP print novel | scrape-creators-pp-cli account budget | preserved via regen-merge; pairs with new account estimate |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Cost-aware thread completion | comments thread <post-url> | 10/10 | hand-code | Routes between include_replies (15 cr flat) and per-comment replies (1 cr each) on economics the API doesn't expose; reports route + credits in the envelope | Use this command to fetch one post's complete comment threads with cost-aware routing. Do NOT use it for bulk multi-post pulls; use 'comments sweep' instead. Do NOT use it to audit already-synced posts; use 'comments coverage' instead. |
| 2 | Comment corpus FTS | comments search <query> | 8/10 | hand-code | FTS5 over synced comments+replies — no API search over comments exists at all | Use this command to search already-synced comment text offline. Do NOT use it to fetch new comments from the API; use 'comments thread' or 'comments sweep' instead. For transcripts, use 'transcripts search'. |
| 3 | Reply-gap audit | comments coverage [handle] | 9/10 | hand-code | Joins API-reported counts vs stored rows per post — ground truth where child_comment_count is measured unreliable (1 false negative / 5 threads) | Use this command to find synced posts with incomplete comment threads. Do NOT use it to fetch the missing replies; use 'comments thread' on the flagged posts. |
| 4 | Pre-flight credit estimator | account estimate | 9/10 | hand-code | Static per-endpoint credit table × planned calls vs live credits_remaining; non-zero exit if over budget — the go/no-go gate agents need before sweeps | Use this command to project the credit cost of a planned run before making it. Do NOT use it for historical burn rate or days-remaining runway; use 'account budget' instead. |
| 5 | Bulk comment sweep | comments sweep <handle> | 8/10 | hand-code | Posts→comments fan-out with --max-credits circuit breaker; the one-command version of the documented 950-post ritual | Use this command for bulk multi-post comment pulls with a credit budget. Do NOT use it for one post's complete threads; use 'comments thread' instead. |
| 6 | UGC tag watcher | creator tagged <handle> | 5/10 | hand-code | Snapshot+diff memory on the new tagged-posts endpoint (ads monitor shape) | none |

### Killed candidates (audit)
hotspots (ranks by the broken field → coverage), questions (degenerate WHERE → search), apple-music compound (no persona), snapchat monitor (no persona → tagged), audience overlap (unbuildable), credit ledger (duplicates analytics → estimate).

### Reprint verdicts (prior 8)
All keep, 6–9/10, zero drops: creator find 9, creator compare 7, content spikes 6, transcripts search 9, trends triangulate 6 (verify fan-out list: hashtags/popular endpoint removed), creator track 6, ads monitor 9, account budget 8.

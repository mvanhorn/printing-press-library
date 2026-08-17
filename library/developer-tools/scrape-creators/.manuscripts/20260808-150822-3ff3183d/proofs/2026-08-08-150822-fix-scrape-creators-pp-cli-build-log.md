# scrape-creators-pp-cli Build Log — reprint run 20260808-150822-3ff3183d

Manifest transcendence rows: 14 planned, 14 built. (6 new hand-coded + 8 prior novels ported from the published 4.27 print and re-verified under 4.30.1.)

## What was built
- Regenerated from the official OpenAPI spec fetched 2026-08-08 (175 paths; 12 endpoints new since the June print incl. Instagram comment replies, Instagram search, Apple Music, Snapchat Spotlight; /v1/tiktok/hashtags/popular removed upstream).
- Press 4.30.1 surfaces: manifest schema 2, code-orchestration MCP (auto Cloudflare pattern, 178 tools), learn loop seeded with platform aliases, canonical env var SCRAPECREATORS_API_KEY.
- 6 new novel commands (hand-code): comments thread (cost-aware routing, 15cr flat vs 1cr/comment), comments coverage (reply-gap audit vs unreliable child_comment_count), comments search (FTS5 comment corpus), comments sweep (budget-gated bulk pull), account estimate (pre-flight credit gate, exit 7 over budget), creator tagged (snapshot+diff on the new tagged-posts endpoint).
- 8 prior novels ported: creator find/compare/track, content spikes, transcripts search, trends triangulate, ads monitor, account budget — plus their store extension (snapshot tables) and a new comment-corpus store extension (sc_comments + FTS5 + sc_post_meta + sc_tagged_snapshots).
- Phase 4.95 local code review (3-lens agent): 2 errors fixed (coverage would have recorded stored-count as reported-count, making the audit vacuous — now parses per-post comment_count / payload totals), 6 warnings fixed (forced-flat probe waste, tie-boundary wording, silent store-open failure, error-envelope-as-empty, disappeared→left_latest_page rename, helper dedup via sanitizeFetchErr/warnFetchFailures).
- Phase 4.8/4.9 doc audit: teach example missing --resource-type fixed (template-shape, retro candidate), child_comment_count caveat added to SKILL capability text, transcript platform list opened (9 resource types), vendor "comprehensive" adjectives dropped.

## Intentionally deferred
- Per-platform fixture values for all 175 endpoints' live happy-paths (see acceptance report: full live matrix is fixture-bound, not CLI-bound).

## Generator limitations found (retro candidates)
- SKILL template's teach one-call example omits required --resource-type.
- generate --force preserved first-attempt learn_init.go as drift after a spec edit between two generates in the same working dir.

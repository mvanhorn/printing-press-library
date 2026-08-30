# Scrape Creators CLI Brief (reprint, press 4.30.1)

## API Identity
- Domain: public social-media scraping across 28+ platforms (TikTok, Instagram, YouTube, X, LinkedIn, Facebook, Reddit, plus long-tail: Truth Social, Rumble, Snapchat, Pinterest, SoundCloud, Spotify podcasts, GitHub profiles, Apple Music as of Aug 2026). One API key, per-call credit pricing.
- Users: creator-economy analysts, social-media managers running client accounts, growth/ads people monitoring competitor ad libraries, agents (LLM) doing comment/transcript mining pipelines.
- Data profile: high-volume read-only JSON; profiles, posts, comments (threaded), transcripts, ad-library entries; cursor pagination everywhere with per-platform cursor names (next_max_id / cursor / continuationToken / after / has_more); credit metering fields (credits_charged, credits_remaining) on every response.

## Reachability Risk
- None. Official hosted API (api.scrapecreators.com) behind an API key; probe via any GET returns 400/401 JSON without a key, 200 with. No bot-protection between client and API (the API does the scraping on its side).

## Users
Concrete user types, grounded in real usage of the prior print (June–August 2026):

1. **The comment-mining analyst.** Runs a client's social presence (bar/restaurant/local brand). Pulls comments from hundreds of posts per sweep (~950 IG posts in one documented week), mines questions/complaints/sentiment for content ideas. Pain: replies (the answers!) were unreachable until the Aug 2026 replies endpoint; still can't tell cheaply WHICH comments have threads worth chasing (`child_comment_count` is unreliable: 1 false negative per 5 real threads measured).
2. **The cross-platform creator scout.** Given one handle, needs to know where a creator actually lives and how big they are on each platform (the prior print's `creator find` probes 12 platforms). Weekly ritual: qualify collab candidates.
3. **The ad-library watcher.** Monitors a brand's live ads across Facebook/TikTok/Google/LinkedIn ad libraries, wants the diff (what's new, what disappeared) not the dump. The prior print's `ads monitor` does snapshot+diff.
4. **The LLM agent (via pp-scrape-creators skill).** Calls everything with `--agent`; needs honest envelopes (the compact bug that stripped `comments` burned real credits with success:true), credit-cost awareness before expensive calls, and cursor-following `--all` that actually walks pages.

## Top Workflows
1. **Comment mining with replies (NEW):** post URL → comments → replies per thread. Two routes with different economics: `include_replies=true` on /v2/instagram/post/comments (15 credits flat, complete, ~7s) vs per-comment /v1/instagram/post/comment/replies (1 credit each; ~16 credits for an average thread set). The CLI should encode the cost decision, not just the endpoints.
2. **Creator presence matrix:** one handle → 12 platforms, follower counts side-by-side (`creator find`).
3. **Transcript corpus + offline search:** sync transcripts across YouTube/TikTok/IG/FB/LinkedIn/Rumble, FTS5 search offline (`transcripts search`).
4. **Ad monitoring with diff:** `ads monitor` per brand, rerun → new/disappeared ads.
5. **Trend triangulation:** hashtag/topic snapshot across platforms in one call (`trends triangulate`).
6. **Credit budgeting:** balance + daily usage → burn rate and days remaining (`account budget`); agents need this to gate expensive sweeps.

## Table Stakes
- Full endpoint coverage: the fresh official spec is 175 paths (June print had 164; 12 added, 1 removed — /v1/tiktok/hashtags/popular is gone). New families: Apple Music (4), Instagram search/popular/tagged-posts (3), IG comment replies (1), Snapchat spotlight +comments (2), Facebook group (1), TikTok collection videos (1).
- Official CLI (adrianhorning): 110+ endpoint commands, no store, no cross-platform features. Official skills repo: 13 curl-based workflow docs (outlier-post-finder, comment-mining, etc.) — methodology layer, credit-cost tables, known-limitations tables worth absorbing into SKILL.md.
- Cursor pagination with --all on every feed command (prior amend, patch-guarded); typed 403 hints; per-response compact that never strips the sole payload array (patch-guarded).

## Data Layer
- Primary entities: profiles, posts, comments (+replies as child rows keyed by parent comment_id), transcripts, ads, follower snapshots (creator track), trend snapshots.
- Sync cursor: per-platform response cursors (see patch scrape-creators-pagination-cursor-wiring).
- FTS/search: transcripts FTS5 (exists, keep); comments FTS is the natural extension now that replies complete the threads.
- Known pipeline gap to fix in regen: "items returned but not cached locally (no extractable ID field)" warnings on comments commands — spec should declare sync-eligible resources and their ID keys (data_pipeline_integrity was 7/10).

## User Vision
Reprint under press 4.30.1, publicly promised in PR #1624's comments after publish validate refused the 4.27 tree (manifest schema 1, phase5 marker without source fingerprint, 4.27 MCP surface). Fold in Adrian's two Instagram endpoints (released Jul 31 on the user's report; thread documented: API reported 22,327 comments, returned 15,548 — the ~7k gap was replies). SKILL.md gets operational tables in the official-skills style: per-endpoint credit costs, known limitations, cursor pagination cheatsheet, and the measured caveat that child_comment_count is unreliable as a thread filter (1 false negative in 5 real threads) — for completeness include_replies (15 cr) beats per-comment calls (~16 cr). The user drives this CLI daily through the pp-scrape-creators skill in --agent mode; agent-envelope honesty and credit-cost awareness are first-class requirements.

## Product Thesis
- Name: Scrape Creators CLI (scrape-creators-pp-cli)
- Why it should exist: the only tool where all 175 endpoints, a local SQLite/FTS5 corpus, and cross-platform joins live behind one binary — the official CLI mirrors endpoints, the official skills describe workflows in curl, and neither remembers anything between runs.

## Build Priorities
1. Regenerate from the fresh official spec (175 paths) under 4.30.1: schema-2 manifest, current MCP surface, fingerprinted phase5.
2. Comment-thread completeness: replies endpoints wired with the cost-aware guidance; comments+replies land in the store with real IDs.
3. Preserve the 8 prior novel features (subagent re-scores; all shipped and documented) and the patch-guarded behaviors (compact payload, cursor --all, typed 404s, honest descriptions, release-ledger version, x/sys floor).
4. SKILL.md operational tables (credits, limitations, cursors) absorbed from the official skills repo style.

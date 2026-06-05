# DataForSEO CLI Brief

## API Identity
- **Domain:** SEO data platform. 437 paths / 554 operations across 12 product families: SERP, Keywords Data, DataForSEO Labs, Backlinks, On-Page, Domain Analytics, Merchant, Business Data, App Data, Content Analysis, AI Optimization, Appendix.
- **Users:** SEO agencies, in-house SEO teams, content/affiliate operators, AI-search optimization ("LLMO/AEO"), competitive intelligence shops. Paid B2B — min deposit $50.
- **Data profile:** Each API call returns task wrappers with cost, status, and result arrays. Two execution modes: Live (sync, ~3-4× cost) and Standard (queued task_post → tasks_ready → task_get, ~5min, cheapest). Sandbox mirror at `sandbox.dataforseo.com` returns dummy payloads free of charge.
- **Auth:** HTTP Basic only. `Authorization: Basic base64(login:password)`. Credentials from app.dataforseo.com/api-access. No OAuth, no IP allowlist.
- **Server pair:** Production + sandbox. Wire `--sandbox` flag to flip host.

## Reachability Risk
**None.** Paid B2B API behind Basic Auth — no Cloudflare/captcha/anti-bot. GitHub search for `"dataforseo" "403"` returned zero hits against DataForSEO infrastructure. Phase 1.9 reachability probe passed (200 OK against `/v3/appendix/user_data`, balance $50.32 confirmed live).

## Top Workflows
1. **Keyword volume + ideas** — `keywords_data/google_ads/search_volume/live` (Joey's existing use case via ContentBot) + `dataforseo_labs/google/keyword_ideas` for expansion. Bulk: 1000 keywords/request.
2. **SERP scrape (Google organic + AI overviews)** — `serp/google/organic/live/advanced` is the flagship; SERP API has 76 Google endpoints alone. Use for rank tracking, competitor SERP mapping, "what ranks for X today."
3. **Backlink profile audit** — `backlinks/summary` + `backlinks/referring_domains` + `backlinks/anchors`. Replaces $400+/mo Ahrefs queries for one-off audits.
4. **On-page audit** — `on_page/instant_pages` (single URL, fast) or `on_page/task_post` (full site crawl). Powers free-SEO-audit lead magnets.
5. **AI Optimization / brand visibility** — newer family (36 paths). Track brand mentions across ChatGPT, Claude, Gemini, Perplexity. Joey's adjacent interest: `ai-seo` skill, FTM/FSM brand visibility in LLM answers.

## Data Layer
- **Primary entities:** Tasks (task_id → status, cost, retry-from-id), Keyword volume/ideas results, SERP results (per-position), Backlinks (domain-level), On-Page audit findings, Brand visibility mentions.
- **Sync cursor:** Tasks have created/finished timestamps; SERP `id_list` endpoint returns all completed tasks within a time window — natural cursor.
- **FTS/search:** Keywords (volume, intent, difficulty), SERP results (URLs, snippets), backlinks (anchor text), brand mentions (LLM response excerpts).

## Codebase Intelligence
- **Source:** Official OpenAPI repo: github.com/dataforseo/OpenApiDocumentation (single 4.2MB YAML, MIT-style permissive, kept current). Official Python/TypeScript/Java/C# clients in the same org.
- **Auth:** HTTP Basic, single security scheme `basicAuth`, applied globally.
- **Data model:** Every endpoint returns a tasks-wrapper envelope: `{status_code, status_message, cost, tasks: [{id, status_code, status_message, cost, result: [...]}]}`. Top-level vs task-level status codes diverge (top 20000 + task 40501 is the "partial fail" signal — see User Vision below).
- **Rate limiting:** 2000 req/min default. Tight per-endpoint exceptions: Live Google Ads 12/min, `/appendix/user_data` 6/min, `/appendix/status` and `/appendix/errors` 10/min, `tasks_ready` 20/min.
- **Architecture:** Live mode (sync, 1 task/call) vs Standard mode (async, 100 tasks/call, ~$0.0006 vs $0.002 — 3.3× cheaper). 4 priority tiers in Standard. Pingback/postback callbacks for "no-poll" Standard but most users self-host poll loops.

## User Vision (from Joey's memory)
- **Existing wire:** `seo_engine/scoring/serp.py` already uses the Google Ads volume endpoint via `_clean_for_dfs` + bulk POST. ContentBot env vars `DATAFORSEO_LOGIN` / `DATAFORSEO_PASSWORD`. Balance $50.32 on file 2026-05-11.
- **Pre-cleaner gotcha (from `feedback_dataforseo_keyword_cleaning.md`):** One bad keyword (>10 words, punctuation, special chars) fails the WHOLE batch with `task_status_code 40501` while top-level returns `20000` (looks like success). The CLI MUST pre-clean and surface task-level errors loudly.
- **Likely killer workflows:** keyword volume/ideas for SEO blog engine pipeline; SERP scraping for the 200+ FTM/FSM landing pages; backlink audit for the FSM citation submission campaign; brand visibility in AI answers (ties to `ai-seo` skill).

## Product Thesis
- **Name:** `dataforseo-pp-cli`
- **Why it should exist:** The official MCP exposes 49 tools through stdio; community wrappers (AgriciDaniel/claude-seo 6.4k stars, zubair-trabzada/dataforseo-claude, Skobyn MCP) are stateless. **No tool today offers: (a) local SQLite-backed result store with offline FTS search, (b) auto-routing Live↔Standard based on batch size, (c) keyword pre-cleaner that prevents the 40501 batch-fail trap, (d) cost-estimate-before-call, (e) a unified poll-loop wrapper for Standard mode.** All five are agent-native value-adds that compose better than the official MCP can offer.

## Build Priorities
1. **Endpoint mirror across all 12 families** — auto-generated from official OpenAPI. Use spec's natural product taxonomy (`serp`, `keywords-data`, `dataforseo-labs`, `backlinks`, `on-page`, `domain-analytics`, `merchant`, `business-data`, `app-data`, `content-analysis`, `ai-optimization`, `appendix`).
2. **Mode auto-routing transcendence:** `auto-mode` flag that picks Live for batch≤5 and Standard (with managed poll loop) for batch>5. Warn on Live + large input.
3. **Keyword pre-cleaner transcendence:** `clean-keywords` filter applied automatically to `/keywords_data/google_ads/search_volume/live` and `dataforseo_labs/google/keyword_ideas` — drops keywords >10 words, strips punctuation, reports rejects.
4. **Cost estimator transcendence:** `cost-estimate` subcommand that calls the pricing endpoints OR uses a static table to predict spend BEFORE the live call. Critical for users burned by Live-mode cost spikes (top community complaint).
5. **Local SQLite store transcendence:** sync + offline FTS over keywords/SERPs/backlinks. Enables `dataforseo-pp-cli search "tree service"` to scan past results offline without re-billing.
6. **Standard-mode poll bundler transcendence:** `task-bundle` command that `task_post`s N tasks, polls `tasks_ready`, hydrates `task_get`, and returns the merged result — one command for the full Standard lifecycle.

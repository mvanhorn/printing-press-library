# DataForSEO CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | All 554 endpoints CRUD-mirror across 12 product families | Official Python/TS/Java/C# clients | endpoint-mirror generator emits 437 paths × method as Cobra subcommands grouped by product family (`serp`, `keywords-data`, `dataforseo-labs`, `backlinks`, `on-page`, `domain-analytics`, `merchant`, `business-data`, `app-data`, `content-analysis`, `ai-optimization`, `appendix`) | Single static binary, no Python runtime, `--json --select --csv --compact --dry-run --agent --rate-limit --timeout --profile` on every command |
| 2 | 49 MCP tools across 9 modules | dataforseo/mcp-server-typescript (official, 196★) | Cobratree mirror auto-emits MCP tools from every Cobra command at MCP server start | Single source of truth; tools stay in sync with CLI without separate maintenance |
| 3 | Slash commands per product (`/seo dataforseo serp\|keywords\|backlinks`) | AgriciDaniel/claude-seo (6.4k★) | Same product-grouped commands but as a single binary that works outside of Claude Code skills | Offline-usable, scriptable, agent-native; not coupled to one IDE |
| 4 | JSON-in/JSON-out batch interface | Skobyn/dataforseo-mcp-server | `--json` on every command + `--csv` for spreadsheet pipelines + `--select` for field filtering | Standard pp scaffolding; agent-friendly by default |
| 5 | CSV auto-export with timestamps | nikhilbhansali/dataforseo-skill-claude (29★) | `--csv` flag emits CSV to stdout; pipe to `tee report-$(date +%Y%m%d).csv` | Composable; works with any spreadsheet/notebook |
| 6 | Auto-generated MCP tools from OpenAPI | pawneetdev/dataforseo-mcp | endpoint-mirror generator works from the same official OpenAPI yaml | Curated (large-surface MCP enrichment hides raw endpoint tools behind `dataforseo_search` + `dataforseo_execute`) |
| 7 | AI brand visibility (ChatGPT/Claude/Gemini/Perplexity) | aimonk2025/dataforseo-ai-mcp-server | endpoint group `ai-optimization` from the OpenAPI spec (36 paths: gemini, chat_gpt, llm_mentions, claude, perplexity, ai_keyword_data) | Full AI Optimization family coverage, not just brand-mention subset |
| 8 | Sandbox mode | docs (built-in) | `--sandbox` global flag flips host from `api.dataforseo.com` to `sandbox.dataforseo.com` | Cost-free testing of any command before live; default for dogfood/CI |
| 9 | Per-command rate limiting | n/a (no tool does this well) | `cliutil.AdaptiveLimiter` per endpoint family (Live Google Ads 12/min, `user_data` 6/min, `tasks_ready` 20/min, default 2000/min) | Avoids HTTP 429 silently corrupting downstream queries |
| 10 | HTTP Basic auth from env vars | Official clients | `DATAFORSEO_LOGIN` + `DATAFORSEO_PASSWORD` env vars (matches Joey's ContentBot wiring) + `auth doctor` reachability check | Drop-in compatible with existing ContentBot/Railway env |

### Not absorbed (intentional)

- **PDF report generation** (zubair-trabzada/dataforseo-claude) — agency-deliverable shape; out of CLI lane.
- **Workflow orchestration** (n8n / Zapier / Make) — those are runtimes, not CLIs.
- **Telegram/Slack digest delivery** — external service; Joey pipes survivor #7's output into his FTMAlertBot manually.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | How It Works | Evidence |
|---|---------|---------|------:|---------|--------------|----------|
| 1 | Keyword pre-cleaner with 40501 surfacer | `keywords clean <file>` + auto-middleware on `volume` / `keyword-ideas` (`--fail-on-task-error` default-on) | 10/10 | A | Local filter (>10 words, punctuation, special chars) before `/keywords_data/google_ads/search_volume/live`; parses task-level status codes from response envelope and exits non-zero on any task 40501 even if top-level is 20000 | Brief User Vision + `feedback_dataforseo_keyword_cleaning.md` memory; existing `_clean_for_dfs` regex in `seo_engine/scoring/serp.py` |
| 2 | Auto-mode router (Live↔Standard) | `--auto-mode` flag on batch endpoints (volume, keyword-ideas, SERP) | 10/10 | A, D | Batch size ≤5 → Live endpoint; >5 → Standard `task_post` + managed poll loop. Warns when user forces Live on a large batch. | Brief Build Priority #2; 3.3× cost differential in API spec; no competitor MCP routes automatically |
| 3 | Cost estimator pre-flight | `cost estimate <subcommand> <args>` + `--confirm-over $N` gate | 10/10 | D, A | Static price table (Live vs Standard per endpoint family) × planned input size parsed from args; no API call needed | Brief Build Priority #4; "top community complaint" per G2/NextGrowth/Toksta reviews; no competitor offers pre-call gating |
| 4 | Standard-mode task bundler with local ledger | `task bundle <endpoint> --in batch.json`; `task ls`; `task get <id>` | 9/10 | A | Single command runs `task_post` → polls `tasks_ready` respecting 20/min limit → `task_get` for each ready id → merges results. Task IDs persist to local SQLite so user resumes after Ctrl-C or sleep. | Brief Build Priority #6; `tasks_ready` polling tax documented in DataForSEO collection best-practices |
| 5 | Local SQLite store + offline FTS | `search "<query>"` scans synced keywords / SERPs / backlinks / AI-mentions | 8/10 | A, B, C, D | Every result-returning API call upserts into local SQLite tables (`keywords`, `serp_results`, `backlinks`, `ai_mentions`) with FTS5 indexes over snippet/url/anchor/answer-excerpt; offline `search` runs FTS5 MATCH without re-billing | Brief Build Priority #5; rubric's canonical transcendence pattern; powers features 6-9 |
| 6 | Volume delta tracker | `keywords delta --since 7d` | 9/10 | A | Joins current `volume` API response against last stored value per keyword in local SQLite; outputs movers sorted by absolute delta | Cross-entity join only possible with local store; Joey's 2-3×/week ContentBot volume-hydration ritual |
| 7 | Rank + SERP-feature delta over URL list | `rank track --sitemap <url>` (`--features` for AI Overview / featured-snippet diff) | 9/10 | B | Parses sitemap, infers target keyword per URL from page title (or accepts `--map url,keyword`), calls `serp/google/organic/live/advanced` per keyword, diffs position + SERP features against last run in local SQLite, flags movers and feature gains/losses | Brief Top Workflow #2; 200+ FTM + 228 FSM pages per memory; 76 Google SERP endpoints incl. AI Overview |
| 8 | AI-visibility delta in LLM answers | `ai-visibility track --brand "<brand>" --keywords keywords.txt` | 9/10 | C | Calls AI Optimization family endpoints for each keyword, stores per-LLM mentions/excerpts in SQLite, diffs week-over-week presence + mention count per LLM | Brief Top Workflow #5; AI Optimization family (36 paths); Joey's `ai-seo` skill memory entry; no competitor diffs week-over-week |
| 9 | Backlinks-since-last-run | `backlinks new --domain <domain>` | 8/10 | D | Snapshots `backlinks/summary` + `referring_domains` + `anchors` into SQLite; on each run diffs against the prior snapshot and surfaces newly-acquired referring domains with anchor text | Brief Top Workflow #3; FSM citation-submission workstream per `project_fsm_directory_progress.md`; replaces $400+/mo Ahrefs |

### Stubs / deferred / out of shipping scope
- None. All 9 transcendence rows are shipping-scope. No CF-gating, no paid-tier requirements, no LLM dependency.

### Dropped candidates (audit trail)
- **PDF audit reports** — agency deliverable, not CLI's lane.
- **Competitor SERP overlap** — borderline weekly-use; not documented in Joey's rituals.
- **LLM-powered rank-loss explainer** — rubric LLM-dependency kill; mechanical version is `rank track --features`.
- **Slack/Telegram digest delivery** — rubric external-service kill; Joey pipes survivor #7 into his FTMAlertBot manually.

See `2026-05-11-221728-novel-features-brainstorm.md` for full subagent audit trail (customer model, all 18 candidates, killed-with-reason).

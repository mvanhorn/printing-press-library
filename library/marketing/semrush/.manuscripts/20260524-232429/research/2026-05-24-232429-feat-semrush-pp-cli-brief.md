# Semrush CLI Brief

## API Identity
- **Domain:** SEO / SEM intelligence — keyword research, domain analytics, competitor analysis, backlinks, Site Audit, position tracking.
- **Users:** SEO consultants, content strategists, growth/PPC marketers, technical SEO engineers, growth analysts, BI engineers feeding Semrush data into warehouses.
- **Data profile:** Huge, deeply hierarchical. ~75 documented endpoints across two surfaces (v3 Analytics CSV + v3 Projects JSON). High-cardinality entities: domains, keywords, URLs, backlinks, ad copies, project campaigns, audit snapshots, audit issues. Historical SEO data back to 2012, monthly granularity. ~140 country databases (`us`, `uk`, `de`, ...) plus mobile-* and -ext variants.

## Reachability Risk
- **None / Low.** Live probe on 2026-05-24 against v3 query-param auth succeeded: balance check returned `6340`, then a `domain_rank` query for `apple.com` returned the documented CSV (`Domain;Rank;Organic Keywords` followed by `apple.com;16;47380349`). Balance dropped to 6330 (10 units consumed, exactly as documented for 1-row `domain_rank`). No rate limits hit, no auth complications. The CLI will ship against a known-reachable API.
- Standard API balance endpoint (`https://www.semrush.com/users/countapiunits.html?key=KEY`) is free and works as a real-time credit gauge — usable for a `doctor` health check and pre-flight cost gating without spending a unit.

## Top Workflows
1. **Domain SEO audit at a glance.** "How is `<domain>` performing in organic + paid? Who are its top competitors? What keywords drive the most traffic?" — Domain Overview + Organic Keywords + Competitors + Domain Pages, often filtered by region.
2. **Keyword research and qualification.** "Find related keywords for `<seed>` with volume > N, KD < M, transactional intent, US database. Show top SERP for the winners." — Related Keywords / Phrase questions / Broad Match + Keyword Difficulty + Keyword Overview, layered with `display_filter` syntax.
3. **Keyword gap mining between competitors.** "What keywords does `mycompetitor.com` rank for that we don't?" — Domain vs Domain + Competitors in Organic Search, with set-difference logic the API can't return natively (it requires joining two responses).
4. **Backlink intelligence.** "Who links to `<domain>`? Which authority-score 70+ domains link to a competitor but not me? When did this referring domain first appear?" — Backlinks + Referring Domains + Authority Score profile + Historical backlinks, with cross-domain joins.
5. **Site Audit + Position Tracking automation.** "Trigger an audit, wait for the snapshot, pull issue counts by severity. Track keyword ranking drift per region/device over time." — Site Audit (`run audit`, snapshots, detailed report for issue) + Position Tracking (organic positions, visibility index, landing pages).

## Codebase Intelligence
- **Local docs** at `/Users/charlesg77/Desktop/SEMRush/semrush-documentation/` are a clean, hand-scraped, fully indexed copy of the official developer portal (developer.semrush.com). 35 markdown files, ~26K lines, byte-identical to the upstream. README.md is the canonical index. **This is the primary spec source** — Phase 2 will author an internal YAML spec from these docs, not an OpenAPI conversion.
- **No official OpenAPI spec** published by Semrush. They publish docs only, plus an MCP server (`https://mcp.semrush.com/v1/mcp`, OAuth-or-Apikey-header).
- **No official Semrush GitHub repos** for API client, SDK, or CLI. The org publishes the Intergalactic design system, zenrpc, purr, and infra tooling — nothing for API consumers.
- **Two relevant community competitors as of early 2026** (see absorb manifest for full breakdown):
  - `mrkooblu/semrush-mcp` — TypeScript, 77 MCP tools + thin CLI, ephemeral in-memory cache, no persistence.
  - `osodevops/semrush-cli` — Rust, structured-JSON-first CLI, SHA256 disk cache, TOML batch recipes, exit-code discipline, shell completions. Explicitly "agent-friendly."
- **Older / less maintained**: silktide/semrush-api (PHP), ithinkdancan/node-semrush, arambert/semrush, DigitalRockers/semrush, scriptburn/semrush-api.

## Auth
- **Single env var**: `SEMRUSH_API_KEY`, passed as `?key=<value>` query parameter on every v3 endpoint (Analytics + Projects). The same key is the only credential — no OAuth, no Bearer token, no v4 endpoints in scope.
- **Auth check at doctor**: hit the free balance endpoint; expect 200 + integer body. Any non-200 or "0" body is reported as a typed `auth_invalid` or `auth_no_units` error.
- **Cardinal Rule**: the literal API key value MUST NEVER appear in any project artifact (code, docs, README, examples, HARs, archived proofs). Env-var names and "your-key-here" placeholders are fine. See user memory `feedback-secret-handling`.

## Data Layer
- **Primary entities** (each a SQLite table with the canonical query-key shape; resources table holds raw `data` JSON for full agent access):
  - `domains` (root_domain, database, rk, organic_keywords, organic_traffic, organic_cost, adwords_*, sh, sv) — Domain Overview row
  - `keywords` (phrase, database, nq, kd, cp, co, intents, td) — Keyword Overview row
  - `keyword_positions` (domain, phrase, database, po, pp, pd, ur, snapshot_date) — Domain Organic / Paid keyword tracking
  - `backlinks` (target_url, source_url, anchor, first_seen, last_seen, nofollow, page_ascore, target_ascore) — Backlinks row
  - `referring_domains` (target_domain, domain, ip, domain_ascore, backlinks_num, first_seen)
  - `competitors` (target_domain, competitor_domain, database, common_keywords, similarity, kind=organic/paid/pla)
  - `audit_snapshots` (project_id, snapshot_id, taken_at, total_errors, total_warnings, total_notices)
  - `audit_issues` (project_id, snapshot_id, issue_id, severity, count)
  - `audit_pages` (project_id, snapshot_id, page_id, url, issues_count)
  - `tracking_positions` (project_id, campaign_id, phrase, position, url, snapshot_date)
  - `projects` (project_id, name, domain, created_at, tools_enabled)
  - `ad_copies` (target, phrase, title, description, visible_url, ad_id, first_seen)
- **Cursor / sync model:** per `(resource_type, query_signature)`, store `last_synced_at` and `next_offset`. The 4 M-row `display_limit` ceiling means pagination is essential for big-domain queries.
- **FTS5**: full-text index over `keywords.phrase`, `domains.root_domain`, `audit_pages.url`, `backlinks.anchor`, `ad_copies.title+description`. This is what makes "search anything I've ever pulled" possible.
- **History semantics:** every Analytics row carries a Semrush `Ts` column (UNIX timestamp). Persist that; do not store-by-date alone. `display_date=YYYYMM15` allows time-travel, which we want to snapshot into the local store.

## Codebase Intelligence (auth specifics from competitor source review)
- mrkooblu/semrush-mcp source uses `process.env.SEMRUSH_API_KEY` and constructs `key=<value>` on every URL — confirming the canonical env-var name we should use. Both competitors agree on `SEMRUSH_API_KEY`.
- osodevops/semrush-cli has a 3-tier credential precedence: `--api-key` flag > `SEMRUSH_API_KEY` env > `~/.config/semrush/config.toml`. Worth matching; the Printing Press's emitted `config` package already does this pattern.

## User Vision
- User explicitly chose the "Just the tested core" scope: v3 Analytics + v3 Projects only, no Trends, no v4. Their reasoning was credit budget (6,330 remaining), no Trends API access on their account, and a desire for a clean shippable CLI.
- User plans to publish to the public printing-press library, so the API key value must never land in any artifact.
- User already maintains `dataforseo` and `serpapi-google-local` CLIs in their local library. Phase 1.5/Phase 3 should consider whether any cross-CLI SQLite-store conventions (`resources` table shape, `--source` fan-out flag, `cliutil.FanoutRun`) carry over so the three CLIs feel like siblings.

## Product Thesis
- **Name:** `semrush-pp-cli` (binary: `semrush-pp-cli`, slug: `semrush`).
- **Display name:** Semrush (canonical brand; no all-caps "SEMrush" or "SEMRush" — the company rebranded in 2022).
- **Headline:** "Every Semrush Analytics + Projects feature, plus a real local SQLite store nobody else has, plus offline search and snapshot diffs that turn one-shot API calls into a memory you can query."
- **Why it should exist:**
  - The two top competitors (mrkooblu MCP+CLI, osodevops Rust CLI) have great endpoint coverage but **no real persistence** — every call is one-shot. Once you've spent the credits, the data is in your terminal scrollback. Both lack SQL composability, cross-domain joins, snapshot diffs, drift detection, and a stored history.
  - The official Semrush MCP exists but is a remote service — credentials live with Semrush, output goes through Anthropic, no local store, no offline use.
  - Building on the Printing Press gives us SQLite, FTS5, agent-native JSON envelopes, dogfood-tested commands, auto-MCP, intent orchestration, and shell-friendly exit codes for free. Adding cost-aware planning, drift detection, and cross-domain joins on top is the differentiator.

## Build Priorities
1. **Generate the CLI from a hand-authored YAML spec derived from local docs.** All ~75 endpoints, both v3 surfaces, single API-key query-param auth, CSV response handling for Analytics endpoints (custom decoder), JSON for Projects. Spec also declares the `cache` block (the Printing Press's generator-owned freshness helpers fit cleanly) and `mcp.transport: [stdio, http]` (95 tools — over the threshold; remote agent reach matters for an SEO tool).
2. **Build the absorbed feature set** — every CLI subcommand from mrkooblu and osodevops, including the keyword-gap, batch-keyword, q-auto-detect, and trends-summary patterns. The osodevops `--dry-run` cost estimator is the parity bar for cost transparency.
3. **Build the transcendence layer:** a small set of novel commands that only work because we have SQLite — `drift`, `snapshot`, `gap`, `intersect`, `budget`, `since`, `triage`. See the absorb manifest (next phase) for the final list and the novel-features-subagent output for evidence.

## Risks / Open Questions Heading Into Phase 1.5
- CSV row counts can be enormous (default `display_limit` is 10,000). The CLI must default to a sane lower limit (1,000? 100?) and clearly surface the "I'm clipping your data" behavior. Otherwise a single keyword query against a busy domain can drain credits silently.
- The `display_filter` syntax (`+|Field|Op|Value` URL-encoded, `|`-separated, max 25 clauses) is powerful but unergonomic on the command line. Worth a `--where` shorthand that maps friendly Go-style predicates to the wire format.
- Many endpoints share the same base URL and differ only by `?type=`. This rules out a clean per-endpoint Cobra command tree if we just mirror URLs — we need a resource-then-action shape (`domain organic`, `keyword related`) matching the docs' mental model.
- Site Audit has async crawl semantics — `run audit` triggers, then snapshot becomes available later. The Printing Press's `jobs` template can model this; otherwise a `audit run --wait` flag is the minimum.
- Trends (out of scope) and v4 (out of scope) will need to be **explicitly absent from the README/SKILL** so users don't expect them. Anti-trigger phrases ("manage Map Rank Tracker campaigns", "Listing Management", "traffic summary") in SKILL.md.

## Sources
- Local docs index: `/Users/charlesg77/Desktop/SEMRush/semrush-documentation/README.md`
- mrkooblu/semrush-mcp — github.com/mrkooblu/semrush-mcp
- osodevops/semrush-cli — github.com/osodevops/semrush-cli
- @ilker10/semrush-mcp — npmjs.com/package/@ilker10/semrush-mcp
- Older clients: silktide/semrush-api (PHP), ithinkdancan/node-semrush, arambert/semrush, DigitalRockers/semrush, scriptburn/semrush-api, gshel/semrush-cli
- Official Semrush MCP doc: `basics/semrush-mcp.md` (confirms 3-tool code-orchestration pattern: `semrush_report`, `semrush_report_list`, `semrush_report_schema`)
- Live API probe 2026-05-24-23:08 UTC — balance 6340 → 6330, single 10-unit `domain_rank` query for apple.com returned valid CSV

# TrendHunter CLI Absorb Manifest

## Ecosystem Scan (Step 1.5a)

Tools that target trendhunter.com (Python, Node, MCP servers, Claude plugins):

- **jbal/trendhunter** (Python Click CLI, PyPI). 2 stars. Last commit 2024-09-18 (~18 months stale). 5 subcommands. PowerPoint export. Token-bucket rate limiter. No JSON, no SQLite, no offline search, no MCP. This is the only direct competitor.
- No MCP server exists for TrendHunter.
- No Go CLI exists.
- No npm package exists.
- No Claude plugin / skill exists.
- General trend MCPs (trendsmcp/trends-agent-claude, rugvedp/Trends-MCP) cover Google Trends, YouTube, TikTok - not TrendHunter.
- "Trend analysis" Claude skill on mcpmarket.com is a generic framework, not TrendHunter-bound.

Wide-open lane. Everything jbal does, we match in absorb. Everything jbal cannot do becomes transcendence.

## DeepWiki Notes

No useful target. jbal repo is too small for a wiki extract; the entire codebase fits in 5 minutes of reading. Auth/transport/data-model notes already captured in the research brief.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Fetch global trending ideas | jbal `trends` | `latest` + `sync` | JSON, --select, --csv, --compact; SQLite-persisted; offline replay |
| 2 | Browse by category | jbal `categories` | `category <name>` | JSON, --json, --sync flag persists; supports multi-category fan-out |
| 3 | Full-text search | jbal `search` | `search <query>` | Local FTS5 first, --live to merge upstream results, --snippets for context |
| 4 | List-based content (toplists) | jbal `lists` | `toplists` + `popular` + `scoreboard` | Per-list parsing, JSON, ranked output, --top N |
| 5 | Combined query | jbal `assortment` | `digest` and `brief` | Tagged provenance for each row, structured single payload, agent-ready markdown emit |
| 6 | Image download | jbal `-s` resize | `trend show --download-images <dir>` | Stream, no PIL dependency, dedup by content hash |
| 7 | Concurrent fetch | jbal `-c` workers | Default concurrent sync + --workers | adaptive limiter, no monkey-patches |
| 8 | Rate limiter | jbal token bucket | `cliutil.AdaptiveLimiter` | typed `*cliutil.RateLimitError` on 429, no silent empty results |
| 9 | HTTP proxy | jbal `-y` | `--proxy` flag + HTTPS_PROXY env | Standard env-var fallback |
| 10 | Output format options | jbal console/PPTX | JSON, table, CSV, markdown, --select dotted paths | Agent-native plumbing across every command |

## Transcendence (only possible with our approach)

(Survivors from the auto-brainstorm subagent. Themes: corpus-diff, extraction, corpus-analytics, agent-native, graph.)

| # | Feature | Command | Why Only We Can Do This | Score | Group |
|---|---------|---------|-------------------------|-------|-------|
| 1 | digest | `trendhunter digest --since 7d --category eco` | Local corpus diff: new vs repeat slugs + top keywords over a window. Requires persisted first_seen; site has no week view. | 9 | corpus-diff |
| 2 | watch-category | `trendhunter watch --category gadgets --notify-new` | Per-category RSS returns empty upstream; we synthesize it from synced category pages + local dedup. | 9 | corpus-diff |
| 3 | faq | `trendhunter faq --slug ai-clone --format json` | Extracts FAQPage JSON-LD into structured Q&A. No prior tool reads it. Agent-ready trend summary. | 9 | extraction |
| 4 | cluster | `trendhunter cluster --window 30d --min-count 3` | FTS5 co-occurrence + rising-vs-prior-window delta over keywords + body text. Pure local-corpus feature. | 8 | corpus-analytics |
| 5 | authors | `trendhunter authors --top 20 --since 30d` | Site /scoreboard is lifetime; we compute time-windowed publish velocity from first_seen + author column. | 8 | corpus-analytics |
| 6 | megatrend-map | `trendhunter megatrend-map --slug ai-venting` | Joins /megatrends index, related-trend graph, and keyword overlap. None exposed as a single view on the site. | 8 | graph |
| 7 | brief | `trendhunter brief --category ai --top 10 --format markdown` | Bundles ranked trends + FAQ Q&A + keywords in one structured payload tuned for LLM ingestion. | 8 | agent-native |
| 8 | inbox | `trendhunter inbox` | Per-machine cursor table - shows trends new since the user's last `inbox` call. Stateful local feature the website can't offer. | 7 | corpus-diff |
| 9 | scout | `trendhunter scout --category kitchen --business "We sell smart ovens" --top 10` | Pulls top trends in a category, scores each by relevance to a business profile (default: keyword overlap + TF-IDF; `--llm` routes through a local LLM via `codex exec` or `claude --print` if available). Emits a shape designed for handoff to downstream research tools (one trend per line, slug + keywords + score). User-requested addition during Phase 1.5 brainstorm. | 9 | agent-native |

All 9 are shipping-scope. No stubs.

## User Vision Notes

Added during Phase 1.5 brainstorm:

- **Business-relevance use case**: "If I'm an oven company I want to know about kitchen/appliance trends... give it a category and have it poll the top trends relative to that category and then run something like last30days on top of it." -> `scout` command above. Output format designed for pipe-friendly handoff.
- **AI-powered scoring as killer feature**: `scout --llm` routes scoring through a local LLM (probe for `codex` or `claude` on PATH, prefer the one already used by this run). Fallback: deterministic keyword/TF-IDF scoring when no LLM is installed.
- **Do NOT depend on /last30days**: it's a downstream consumer that may or may not be present. The CLI's job is to emit a clean JSON/newline format that's easy to pipe; the user wires up the next stage.

## Spec-Derived Commands (Phase 2 generator output)

These come straight from `trendhunter-spec.yaml` and get scaffolded by `printing-press generate`. They are the raw HTTP-shaped surface, useful for debugging and for the MCP endpoint-mirror:

- `rss latest` - raw RSS body
- `trends get <slug>` - raw HTML
- `search query --search <q>` - raw search results HTML
- `category list <name>` - raw category HTML
- `popular get` - raw popular HTML
- `scoreboard get` - raw scoreboard HTML
- `sitemap get` - raw sitemap XML
- `megatrends list` - raw megatrends HTML
- `reports list` - raw reports HTML

Hand-written sugar commands (`latest`, `category`, `popular`, `scoreboard`, `search`, `trend show`, `trend faq`) wrap these with parsing and structured output.

## Build Order

1. **Phase 2 generate** - lay down the spec-shaped scaffold (10 absorbed feats already mapped).
2. **Phase 3 Priority 0 (foundation)** - SQLite store with trends/categories/authors/reports/sync_run tables, FTS5 over trends, sitemap-driven catalog table.
3. **Phase 3 Priority 1 (absorb)** - the 10 absorbed rows above, full implementation.
4. **Phase 3 Priority 2 (transcend)** - the 8 survivor rows above, full implementation.
5. **Phase 3 Priority 3 (polish)** - flag descriptions, README, MCP annotations.

No paid auth anywhere. Single-source CLI. stdlib HTTP transport.

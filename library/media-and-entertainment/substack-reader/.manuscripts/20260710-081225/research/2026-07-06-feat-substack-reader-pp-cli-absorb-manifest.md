# Substack Reader — absorb manifest

Absorb everything the ecosystem does (keyless/public posture), then transcend with the local corpus + SQL layer nobody has. Status: ✅ built · ◐ scaffolded/stub · ○ planned.

## Absorbed (match or beat every existing tool)

| # | Feature | Best source | Our implementation | Added value | Status |
|---|---|---|---|---|---|
| 1 | List a publication's posts | substack_api `get_posts` / sbstck-dl `list` | `archive <pub>` (also persists) | offline, SQLite-backed | ✅ |
| 2 | Fetch a single post's content | substack_api `Post.get_content` | `read <pub>/<slug>` | full-vs-preview detector, entitlement-aware | ◐ (FetchPost ready) |
| 3 | Per-publication search | substack_api `search_posts` (archive?search=) | framework `search` + live `archive?search=` | offline FTS across ALL archived pubs | ◐ |
| 4 | Full archive download | sbstck-dl `download`, Substack2Markdown | `archive --limit` | metadata + JSON in one store | ✅ |
| 5 | Discover publications | substack_api search/categories | `publications search`, `categories` | generated typed endpoints | ✅ (generated) |
| 6 | Podcast/RSS awareness | RSS-based tools | `feed` (planned) | private-RSS Tier-1 path | ○ |
| 7 | Paid content via own session | substack_api cookies, sbstck-dl --cookie | Tier-1 cookie/RSS in `read` | entitlement-bound, never-required, custom-domain redirect fix | ○ (design done) |
| 8 | Author profile / recommendations | substack_api `get_authors`/recommendations | (generated endpoints, later) | — | ○ |
| 9 | Comments | (few tools) | `/api/v1/post/{id}/comments` | optional | ○ |
| 10 | Rate-limited fetching | sbstck-dl `--rate` | `cliutil.AdaptiveLimiter` @2rps | adaptive backoff on 429 | ✅ (in client) |
| 11 | Multi-format output | substack_downloader | `--json`/`--select`/`--csv` (framework) | agent-native | ✅ (framework) |
| 12 | Agent/MCP surface | dkyazzentwatwa substack_mcp (13 tools) | MCP cobratree mirror of every command | + local corpus tools the live-per-call MCP lacks | ◐ (chassis) |

## Transcendence (only our local-corpus approach enables)

| # | Feature | Command | Buildability | Why only we can do this | Status |
|---|---|---|---|---|---|
| 1 | Local publication corpus | `archive` | hand-code | persist → offline read/search/SQL; every other tool is live-per-call | ✅ |
| 2 | Offline full-text search across all archived pubs | framework `search` | spec-emits | one local FTS index spans pubs; Substack's search is server-side, single-pub | ◐ (needs default-DB sync + fix) |
| 3 | Entitlement-aware read | `read` | hand-code | one command unifies keyless free + user's Tier-1, with an honest full-vs-preview signal | ○ |
| 4 | Cross-publication digest | `digest --since` | hand-code | time-window across the whole local corpus; no single endpoint aggregates pubs | ○ |
| 5 | Author comparison | `author-compare` | hand-code | local join across two archived pubs | ○ |
| 6 | SQL over the corpus | framework `sql` | spec-emits | arbitrary analytics on a real local DB (audience mix, cadence, longest posts) | ◐ (fix research.json) |

## Scope guardrail (the NOT-list)
- **Read-only.** No publishing, scheduling, subscriber/payment management for a pub you own.
- **No bulk crawling/redistribution.** On-demand, personal, self-throttled. Substack ToS prohibits crawl/scrape/spider + redistribution.
- **Tier-1 = your own entitlement only.** The user's own cookie/private-RSS reads only what they already pay for; never required, never redistributed. (Same posture as Medium Reader.)
- **Auth posture:** keyless default (Tier 0); optional own-session Tier 1. No third-party API key.

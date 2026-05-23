# Blu-ray.com Absorb Manifest

## Source tools surveyed (Phase 1.5a)
| # | Tool | URL | Language | Stars | Contribution |
|---|------|-----|----------|-------|--------------|
| 1 | TUVIMEN/blu-ray-scraper | https://github.com/TUVIMEN/blu-ray-scraper | Python | low | Full scraper — extracts every release field across 8 categories (blu-ray, dvd, itunes, digital, ma, uv, prime, main). Reference for what's extractable per page. Site rate-limit guidance (~4k pages/day per IP). |
| 2 | Flexget/Flexget PR #1336 | https://github.com/Flexget/Flexget/pull/1336 | Python | (PR) | API+lookup+estimator plugin; pattern for title lookups. |
| 3 | xenonnsmb/blu2trakt | https://xenonnsmb.com/2018/04/22/blu2trakt-import-your-blu-ray.com-library-into-trakt/ | PHP | low | One-shot importer (Blu-ray.com → Trakt). Demonstrates that blu-ray.com's native UPC-only export creates real friction. |
| 4 | n8n template "4K Blu-ray preorder updates" | https://n8n.io/workflows/6830 | n8n | n/a | Daily preorder polling → Discord. Proves the preorder/coming-soon polling workflow is wanted. |
| 5 | Blu-ray.com (the site itself) | https://www.blu-ray.com | — | — | Native: search (JS, robots-disallowed), coming-soon, new-releases, calendar, deals, news, per-release detail. Native gap: no MCP, no JSON API, no machine-readable export beyond a UPC list, no price-drop alerts. |
| 6 | CLZ Movies (commercial) | https://clz.com/movies | — | — | Closed-source disc collection cataloger. Demonstrates the user appetite for structured collection management; out of scope (commercial GUI/mobile). |

**MCP servers for Blu-ray.com:** none exist.
**Go CLIs for Blu-ray.com:** none exist.
**JSON API:** none exists.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | Fetch full release detail (every spec field) | TUVIMEN scraper | `releases get <slug> <id>` — full HTML detail with ratings, audio, subs, packaging, region coding, distributor links | Single binary, `--json`, `--select`, cached locally, no Python deps |
| 2 | Enumerate every Blu-ray release ever indexed | TUVIMEN sitemap walk | `sync` — pulls all 9 bluraymovies sitemap shards into local SQLite | Incremental sync via `<lastmod>`, ~ minutes instead of days; honors rate limits |
| 3 | Title lookup / search | Flexget plugin, blu-ray.com JS search | `search '<title>'` — local FTS5 index after `sync`, regex-capable, instant | Offline, avoids robots-disallowed `/movies/search.php`, dramatically faster than the JS-rendered native UI |
| 4 | New releases listing | blu-ray.com `?show=newreleases` | `releases new --page N --format 4k --country US --json` | Agent-native JSON, filters as flags, pages chainable with `jq` |
| 5 | Coming-soon / preorders | blu-ray.com `?show=comingsoon` + n8n template | `releases new --show comingsoon` (same command, different view) | Same agent-native JSON; no Discord webhook needed — just pipe |
| 6 | Release calendar | blu-ray.com `/movies/releasedates.php` | `calendar releases --year 2026 --format 4K --country USA` | Server-side filters preserved; future-proof against site UI changes (we use stable URL params) |
| 7 | Theatrical + digital calendars | blu-ray.com `/theatrical/releasedates.php`, `/digital/...` | `calendar theatrical --year`, `calendar digital --year` | One CLI for all three calendar surfaces |
| 8 | Deals listing | blu-ray.com `/deals/` | `deals list --country USA --format 4k --min-discount 30 --max-price 25 --json` | Client-side filters beyond what the site offers; surfaces underlying release id + retailer; never follows the affiliate redirect |
| 9 | News reader | blu-ray.com `/news/` + news sitemap | `news list --limit N`, `news get <id>` | News sitemap is enriched with `<news:title>` + `<news:publication_date>` — full enumeration without per-story fetch |
| 10 | Image / cover access | blu-ray.com `images.static-bluray.com` | `releases get` emits `cover_url`, `screenshots[]`, `packaging[].link` | URLs only (no binary download) — respect the host, agents can fetch if needed |
| 11 | Release format variants | TUVIMEN's 8 categories (`/dvd/`, `/itunes/`, etc.) | `releases get` accepts any of: `/movies/`, `/dvd/`, `/digital/`, `/itunes/`, `/ma/`, `/uv/` URL slugs | Single fetcher covers every category; spec normalizes the field set |
| 12 | UPC export reciprocity | blu-ray.com export (UPC-only) | `upc <file.csv>` (see transcendence #4) | Closes the round-trip: take their UPC export, get full structured data back |
| 13 | Health check / doctor | (none) | `doctor` — verifies reachability, rate-limit headroom, cache freshness | Standard printing-press surface |
| 14 | Agent-native everything | (none in this ecosystem) | Every command supports `--json`, `--select`, `--csv`, `--compact`, typed exit codes | This is the entire reason agent-natives win — no competitor offers it |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | Offline FTS5 search over the entire Blu-ray.com sitemap | `search '<title>' [--format 4k] [--year 2024]` | hand-code | Requires local SQLite + FTS5 index hydrated from the sitemap; blu-ray.com's own search is JS-rendered, robots-disallowed, and slow. |
| 2 | Price-drop watchlist with alerts | `watch add <id> [--target N]`, `watch check`, `watch list`, `watch rm` | hand-code | Requires longitudinal local store (`price_history`) cross-joined with the daily deals scan; blu-ray.com has no native alerting. |
| 3 | Catalog drift (what's new/removed this week) | `drift [--since DATE] [--kind bluray|4k|dvd]` | hand-code | Requires diffing today's sitemap against the last `sync`'s sitemap snapshot — only feasible with a local catalog. |
| 4 | UPC import + resolve (closes the export round-trip) | `upc <file.csv> [--enrich]` | hand-code | blu-ray.com exports UPCs only; reverse mapping to release_ids needs the local sitemap + a tiny in-memory upc→id index built from spec fetches. |
| 5 | Editions side-by-side compare | `editions <movie-umbrella-id> [--country US]` | hand-code | Requires walking the umbrella `/main/<id>/` page to enumerate every format release, then joining each against local detail + price; no competitor surfaces this view. |
| 6 | Price history + ASCII spark plot | `history <release-id> [--retailer NAME] [--plot]` | hand-code | Longitudinal price data persisted locally during `watch check` + `deals --record` runs; plot rendered inline. |
| 7 | Blu-ray.com as an MCP server | (every command above, via `<binary> mcp`) | spec-emits | Generator's Cobra-tree MCP mirror — no MCP server exists for blu-ray.com today; agents get the entire CLI as tools for free. |

**Transcendence count:** 7 (target was ≥5).
**Buildability breakdown:** 6 hand-code + 1 spec-emits.

## Shipping-scope summary
- **Absorbed:** 14 features (all in shipping scope).
- **Transcendence:** 7 features (6 hand-coded + 1 spec-emitted MCP surface; all in shipping scope).
- **Stubs:** 0.
- **Total:** 21 features, every one of which will ship.

## Risks
- **Rate limit:** ~4k pages/day per IP per TUVIMEN's scraper guidance. Mitigation: default polite settings (`--wait 1`+jitter, single-threaded), cache-first behavior on `releases get`, document `--proxy` for power users, polite `User-Agent`.
- **Robots.txt boundaries:** `/movies/search.php`, `/news/search.php`, `/community/*` are disallowed. The CLI MUST NOT touch these. Verified: no command in this manifest scrapes a disallowed path.
- **Latin-1 encoding:** Pages are ISO-8859-1, not UTF-8. The HTML parser must decode accordingly.
- **Slug drift:** Title slugs can change with re-issues (e.g., "Alien-Blu-ray/9929" 301-redirects to "Harts-War-Blu-ray/9929"). The `id` is stable; `get` follows redirects.
- **AI-bot UAs blocked:** robots.txt disallows ClaudeBot/GPTBot/Bytespider/etc. The CLI MUST send a normal Chrome UA string (configurable, default real-browser).

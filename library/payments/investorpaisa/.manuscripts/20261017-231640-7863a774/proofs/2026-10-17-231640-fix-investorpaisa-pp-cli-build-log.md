Manifest transcendence rows: 8 planned, 8 built. Phase 3 passes: all 8 ship.

# Build log — investorpaisa-pp-cli

## Built (hand-written, internal/cli)
- ip_data.go — shared data layer: ipLoader (live / local / auto with local fallback; auth failures never masked by stale data), SQLite `ip_snapshots` table (one row per kind+as-of date), snapshot recorder wired into `sync`, typed /api/live view, signal parsing, stats helpers. Snapshot fetches bypass the HTTP response cache so history is never stale.
- ip_expr.go — screener expression language (tokenizer + precedence parser + tree evaluator; and/or/not, comparisons, + - * /, quoted strings, field aliases, unknown-field validation). Never executes user text.
- screen.go (expr over 48 metrics, --sector/--sort/--asc/--limit/--saved), quote.go (screener+live+news+watchlists, sector percentile), movers.go (6 kinds, RS join), sectors.go (windows 1d–1y, --trend rank change), changes.go (snapshot self-join over 6 conditions), brief.go (indices, breadth, sector leaders/laggards, signals, watchlists, headlines), breadth.go (per-snapshot A/D, %>50/200DMA, 52w highs/lows, trend), history.go (parses /chart/<SYM> RSC flight payload; split/bonus adjusted; cached in snapshots; --raw/--years/--tail).
- sync.go: one hook after the learn refresh calls ipRecordSnapshots (preserved on regen).

## Tests
- ip_expr_test.go (16 expression cases + error/validation), ip_novel_test.go (rankMovers, rankSectors, diffSnapshots, breadthOf, buildQuote, summarizeWatchlist, chart-HTML parse/adjust/errors), ip_e2e_test.go (fake local site via httptest: all 8 commands, two-day changes/breadth/sectors trend, local-mode never touches network, 401 surfaces as auth error, empty local store hint, sync stores snapshots). Test credentials are a dummy env value against a loopback server.

## Absorbed (generated endpoints)
live, universe, screener, market overview, news by-symbol/feed, videos, watchlists list/create/delete/set-items, bookmarks list, me/layout, screens, health, auth login --chrome.

## Intentionally deferred / limitations
- Watchlist rename (PATCH), bookmark add/remove (POST/DELETE with body) and layout/theme PUT were not exercised on the live site; the spec has no PATCH method support. Not in the approved manifest.
- /api/screener/rows ignores `?sector=` server-side; `screen --sector` filters locally.
- `history` depends on the chart-page payload format; it errors clearly (with an auth hint) if the format changes or the session expired.
- `changes`/`breadth`/`sectors --trend` need >=2 stored daily snapshots; with one they return an empty, exit-0 result with a note.
- Generator limitation: dogfood `config inconsistency` heuristic reports garbage fields for cookie-auth CLIs (see shipcheck).

# Wolt CLI Absorb Manifest

## Scope
Browse-only consumer CLI built on Wolt's unauthenticated endpoints + page-embedded SSR data. No login, no cart, no orders.

## Source tools surveyed
- **what-to-eat** (Valaraucoo, Python, GH) — `ls`/`random`/`configure`, filter by tag/sort/limit, weighted random picker, profile-based location
- **wolt-cli** (setevik, Python, GH) — order history reports — **auth-gated, out of scope** for this CLI
- **wolt-restaurants-scraper MCP** (Apify) — bulk city → venues scrape (Apify Actor wrapped as MCP)
- **ikurcubic/wolt-api** (Go) — low-level Items/Inventory partner-API library; not directly useful for browse
- **OzTamir gist** — documented community endpoints (reference, not a tool)

## Absorbed Features

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List cities Wolt operates in | (no tool — only gist) | `cities list` → `/v1/cities` | Pre-sync to SQLite; FTS over country/timezone; `--country FI` filter |
| 2 | List venues near a lat/lon | what-to-eat `ls` | `venues list --lat --lon` → `/v1/pages/restaurants` | SQLite snapshot; offline replay; `--open`/`--cuisine`/`--max-eta` filters; JSON/CSV/select |
| 3 | Search venues by query | what-to-eat (via `ls --query`) | `search venues` → POST `/v1/pages/search` | Multi-city search via stored profiles; FTS dedupe |
| 4 | Search items across venues | (no tool) | `search items` → POST `/v1/pages/search` body `target:items` | Returns dish + venue + price together; sort by price |
| 5 | Get venue details (ETA, fees, hours, status) | (no tool — partial in what-to-eat) | `venues get <slug>` → `/order-xp/web/v1/venue/slug/{slug}/dynamic/` + LD+JSON fallback | Both ETA snapshot and full structured metadata; merges live dynamic with SSR LD+JSON |
| 6 | Filter venues by cuisine tag | what-to-eat `--tag` | `venues list --cuisine pizza` | Stored cuisine vocabulary from synced data; multi-tag AND/OR |
| 7 | Sort by rating / ETA / fee | what-to-eat `--sort --ordering` | `venues list --sort rating\|eta\|fee` | Combined with `--limit`; agent-friendly |
| 8 | Random venue picker | what-to-eat `random` | `venues random` | Same plus `--seed` for reproducibility, `--weighted` (rating × eta) |
| 9 | Location profiles (saved lat/lon) | what-to-eat `configure` | `profile add/use/list/remove` | Stored in SQLite; `--profile home` everywhere |
| 10 | Bulk-scrape a city's venues | Apify scraper MCP | `sync --city helsinki` | Local SQLite, incremental; no Apify dependency |
| 11 | Price-tier badges (€/€€/€€€) | what-to-eat | Surface `priceRange` from venue payload | `--json` exposes raw; table shows badges |
| 12 | Show venue address + phone | (no tool) | Surface from LD+JSON | One-shot fetch; cached locally |
| 13 | Open venue in browser | (no tool) | `venues open <slug>` (print-by-default, `--launch` to open) | Verify-friendly side-effect command |
| 14 | Venue menu items | (none — gist documents but live-broken) | `menu show <slug>` (best-effort) | Tries `/v4/.../menu/data`; on empty body, returns actionable error pointing to known-gap doc |
| 15 | Order tracking by share link | (none) | `track <share-link>` (best-effort) | Extracts order id from `wolt.com/en/track/<id>`; attempts known patterns, reports honestly on failure |

## Transcendence Features

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Cross-city venue comparison | `venues compare --slugs <a>,<b>` | 8/10 | Requires per-venue dynamic snapshots from different cities joined locally; one curl per slug aggregated client-side |
| 2 | ETA / fee drift over time | `venues drift <slug> --days 7` | 7/10 | Requires periodic snapshots of /dynamic stored in SQLite — no API call gives historical data |
| 3 | "What's open right now and fast" | `venues now --max-eta 25 --cuisine X` | 7/10 | Joins live status + ETA + cuisine filter; `--lat/--lon` and stored profiles; agents can pipe directly |
| 4 | Cuisine bottleneck (where are waits longest right now?) | `cuisine bottleneck --city helsinki` | 6/10 | Aggregates ETA distribution per cuisine across all synced venues; only possible with a local store |
| 5 | "Doctor" reachability + endpoint health | `doctor` | 6/10 | Probes each of the 5 working endpoints, surfaces which return useful bytes; helps users distinguish "Wolt is down" from "my CLI is broken" |
| 6 | Multi-city profile bundle | `profile bundle add --city helsinki,tel-aviv,stockholm` | 6/10 | Trip-planner workflow nobody else covers; one command to fan out searches |
| 7 | Offline replay of last sync | `venues list --offline` | 5/10 | SQLite-backed; lets agents reason without burning live calls |

## Stubs / known gaps
- `menu show` (#14) ships **as a working stub** — the documented endpoint returns empty bodies via CloudFront. The command attempts it, formats the empty response honestly, and includes a `--debug` flag that prints the raw request for the user to file an upstream issue. Reason: endpoint discovery needs a longer browser-sniff (forms with selected items / page-context streaming) than fit the time budget.
- `track <share-link>` (#15) ships **as a working stub** — no live order was available to browser-sniff. The command parses the share URL, attempts the most likely endpoint paths, and falls back to printing the order id with a guide for inspecting the response in DevTools.

Both are explicitly marked in `--help`, in README's Known Gaps, and in SKILL.md.

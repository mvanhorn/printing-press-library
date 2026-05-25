# MercadoLibre CLI Absorb Manifest

## Source Tools Surveyed

| Tool | Type | Stars | Last Updated | Coverage |
|------|------|-------|--------------|----------|
| mercadolibre/ml-api-node-sdk | Official Node SDK | 154 | 2020-03 | Auth + raw resource fetch. **ARCHIVED.** No CLI surface. |
| mercadolibre/python-sdk | Official Python SDK | 99 | 2019-06 | Auth + GET/POST/PUT helpers. **ARCHIVED.** Library only. |
| newaeonweb/mercadolibre | Community PHP SDK | 38 | 2022-11 | OAuth + items + categories + users. Maintained but PHP-only. |
| Various per-language wrappers | Mixed | <30 each | varied | Thin wrappers over the REST API. None ship a CLI. None offer agent-friendly output (JSON, --compact, SQLite mirror). None handle the 2024 marketplace-search closure. |

**There is no published CLI for MercadoLibre. There is no MCP server for MercadoLibre.** This is a greenfield niche.

## Features Absorbed (table stakes)

These are common SDK features all alternatives provide — this CLI inherits them as the floor:

- OAuth 2.0 Authorization Code flow (client_id + client_secret + redirect_uri → code → access_token + refresh_token).
- Bearer token sent on every request via `Authorization: Bearer <token>` header.
- Resource endpoint coverage: users, items, categories, sites, products (catalog), questions.
- JSON response parsing.

## Features Built (novelty)

These go beyond what the alternatives provide:

1. **Cross-platform CLI** (Linux, macOS, Windows × amd64/arm64) via goreleaser. No alternative ships a binary.
2. **Token-efficient agent output** (`--agent` expands to `--json --compact --no-input --no-color --yes`). No alternative considers agent ergonomics.
3. **Local SQLite mirror** (`sync` command) for offline / analytics workflows. No alternative offers offline mode.
4. **Cobra help tree at every level** + `which` command for capability lookup. No alternative has discovery affordances for agents.
5. **`agent-skills.io` `SKILL.md`** that any compliant agent (Claude Code, Codex, Gemini CLI, Cursor) auto-discovers. None of the alternatives ship a skill manifest.
6. **Public-path auth omission** (`isPublicPath()` in `internal/client/client.go`) — works for unauthenticated users AND for users with expired tokens on `/classified_locations/*`. No SDK handles ML's post-2024 endpoint mix correctly.
7. **Honest API-gating awareness:** uses `/products/search` (catalog, OAuth-accessible) instead of `/sites/{site}/search` (marketplace, certification-gated). Documented in README "Caveats".

## Novel Features Planned (wired as hidden stubs in v0.1.x)

These ship as hidden experimental commands with `[stub] coming in v0.2.x` messages. Per-command implementation is the roadmap for v0.2.x:

- `watch` — poll-and-diff catalog search at interval; emit JSON on new product appearances. Cron-friendly resale-opportunity radar.
- `compare` — token-bag fingerprint dedup of catalog search results to collapse near-duplicate listings.
- `ml-analytics` — local SQLite analytics over synced catalog (price percentiles, IQR outlier trim, top sellers).

## What This CLI Does NOT Cover (out of scope for v0.1.x)

- **Marketplace listing search** (`/sites/{site}/search`) — ML closed it to non-certified apps in 2024. Future versions could add `--certify` once the user has been through ML's certification process.
- **Sales / orders** (`/orders/search`) — requires the `read orders` scope not requested by default. Documented in README "Caveats" for advanced users who want to regenerate from the spec adding the orders resource.
- **MCP companion binary distribution** — generated but not published to GitHub Releases in v0.1.x (CLI binary is the primary distribution). Future versions may add MCP binary releases.
- **Browser-based scraping** for endpoints not exposed in the REST API (e.g., historical sold prices, seller analytics dashboards). No public ML endpoint exists; would require headless browser approach, deferred.

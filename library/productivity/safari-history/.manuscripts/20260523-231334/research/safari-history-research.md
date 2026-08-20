# safari-history — Research Brief

## Source

Local Safari history SQLite database at `~/Library/Safari/History.db` (macOS).
No remote API, no OpenAPI spec — the "API" is the on-disk schema. Reading it
requires macOS **Full Disk Access**. All access is read-only against a snapshot
copy; nothing leaves the machine.

## Why a CLI (not an API wrapper)

Safari's history DB is locked while Safari runs, uses a CoreData epoch, and
splits page/visit data across two tables. A local-first CLI removes that
friction and exposes the data as both a human CLI and read-only MCP tools.

## Schema findings

- **Timestamps** are `REAL` seconds since the CoreData/Cocoa epoch
  (2001-01-01 UTC); convert with `seconds + 978307200` to Unix seconds. (Chrome,
  by contrast, uses microseconds since 1601.)
- **Two tables:** `history_items` (url, `domain_expansion`, visit_count — but
  **no title**) and `history_visits` (FK `history_item`, `visit_time` REAL,
  **title lives here, per-visit**, `redirect_source`/`redirect_destination` for
  navigation chains, `origin`).
- **No transition bitmask.** Safari has `origin`/`generation`/`attributes`
  instead of Chrome's `transition`, so there is no clean typed-vs-link signal.
- **~1 year retention** (vs Chrome's ~90 days) — wider default windows are
  appropriate.
- **Schema version** lives in a `metadata` table (vs Chrome's `meta`).
- **Locked DB:** Safari holds a lock while running; the CLI copies the DB to a
  snapshot before reading and builds an FTS5 index over url/title.

## Capability gating (vs Chrome)

Safari does **not** store several datasets Chrome does, so the shared Source
interface reports `Capabilities{Journeys:false, SearchTerms:false,
Downloads:false}`:
- **No keyword search terms** → `searches` returns an honest "not available".
- **No downloads table** (Safari downloads live in `Downloads.plist`) →
  `downloads` not available.
- **No Journeys/clusters** → `journeys` not available; `topic` falls back to
  full-text matches only.

This validated the source-adapter boundary: Safari implemented the same
interface as Chrome without touching the store/output/cli/mcp layers.

## Cross-device (sync)

`history_visits.origin` distinguishes local (this device) from synced (other
devices), but unlike Chrome there is no per-device identifier — so partitioning
is binary (this/synced). Surfaced as the `devices` command and a
`--device all|this|synced` filter; `device-N` returns an honest "not available".

## Categorization division of labor

The `domains` static category map is coarse and Safari has no Journeys clusters,
so high-quality topic clustering is left to the agent reading `--json`
titles/URLs. Documented in SKILL.md / README.md.

## Scope / non-goals

- **macOS only** (Safari does not exist off macOS; needs Full Disk Access).
- **Active/open tabs are out of scope** — not in the history DB.
- No writes to Safari's DB; the snapshot is disposable.

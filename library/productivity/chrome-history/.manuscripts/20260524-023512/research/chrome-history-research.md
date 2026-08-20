# chrome-history — Research Brief

## Source

Local Chrome history SQLite database at
`~/Library/Application Support/Google/Chrome/Default/History` (macOS). No remote
API, no OpenAPI spec — the "API" is the on-disk schema. All access is read-only
against a snapshot copy; nothing leaves the machine.

## Why a CLI (not an API wrapper)

Chrome's history DB is rich but awkward to query directly: it is locked while
Chrome runs, uses a non-obvious timestamp epoch, and spreads useful signal
across several tables. A local-first CLI removes that friction and exposes the
data both as a human CLI and as read-only MCP tools for agents.

## Schema findings

- **Timestamps** are microseconds since 1601-01-01 UTC (WebKit/Chrome epoch);
  convert with `micros/1e6 - 11644473600` to Unix seconds.
- **Core tables:** `urls` (url, title, visit_count, typed_count, last_visit_time),
  `visits` (visit_time, from_visit, transition, visit_duration), and
  `keyword_search_terms` (search-engine queries) and `downloads`.
- **`transition & 0xff`** yields the core transition type (link, typed, reload,
  etc.); the high bits are qualifiers. `typed` visits are a strong signal of
  intentional navigation (used by `rabbitholes`).
- **`visit_duration`** is 0 for the large majority of rows, so dwell time must be
  estimated from inter-visit gaps rather than read directly (see `dwell`).
- **Locked DB:** Chrome holds a lock while running; the CLI copies the DB to a
  snapshot before reading and builds an FTS5 index over url/title/search terms.

## Cross-device (sync)

History includes activity synced from other signed-in devices. `visit_source`
(0=synced, 1=local/browsed, 2=extension, 3-5=imported) plus
`originator_cache_guid` (a per-device identifier) let the CLI partition activity
into this-device vs other-devices and enumerate distinct origins. Surfaced as
the `devices` command and a `--device all|this|synced|device-N` filter on reads.
This is a standard browser-forensics technique, applied here read-only.

## Use cases (drove the feature set)

- Recall ("what was that page I saw last week?") → `search`, `visited`, `topic`
- Self-analysis ("how do I spend my browsing time?") → `report`, `heatmap`,
  `dwell`, `profile`
- Session reconstruction → `timeline`, `rabbitholes`, `graph`
- Topic clustering → `journeys` (Chrome's own clusters) + agent inference over
  titles (the CLI provides data + coarse hints; agents do real clustering)

## Categorization division of labor

The static `domains` category map is coarse (~70% "Other" for niche domains) and
Chrome's `journeys` clusters are noisy. The CLI therefore positions `domains` and
`journeys` as hints; high-quality topic clustering is left to the agent reading
`--json` titles/URLs. This is documented in SKILL.md and README.md.

## Scope / non-goals

- **macOS only** for now (Chrome path + Full-Disk-Access model are macOS-specific;
  Linux/Windows use different paths and permission models).
- **Active/open tabs are out of scope** — they are not in the history DB.
- No writes to Chrome's DB; the snapshot is disposable.

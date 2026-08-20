# AnkiWeb Absorb Manifest

## Landscape note
Every existing Anki tool (AnkiConnect, apy, anki-cli, the many anki-mcp-servers) wraps the **desktop** app via the AnkiConnect add-on on `localhost:8765`. **None targets ankiweb.net.** So there is no competing tool to "absorb" on this surface — the absorbed features ARE the AnkiWeb website's own capabilities, matched and beaten with offline FTS, agent-native JSON, and a local SQLite catalog.

## Absorbed (match or beat the AnkiWeb website)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search shared decks by keyword | ankiweb.net/shared/decks | `shared search <term>` (protobuf decode) | regex, --json, --select, scriptable, typed exits |
| 2 | Shared deck detail + reviews | ankiweb.net/shared/info | `shared info <id>` | structured fields, --json |
| 3 | Download deck .apkg | AnkiWeb Download button | `shared download <id>` **(stub — token gap)** | scriptable, --dry-run; honest error until token minting is solved |
| 4 | List my synced decks + stats | ankiweb.net/decks | `decks list` (cookie auth) | --json, offline cache |
| 5 | Offline catalog snapshot | (none — SPA has no offline) | `sync` + FTS5 `search` | offline, SQL-composable |

## Transcendence (only possible with our local store + offline approach)
| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|-------------------------|-------|
| 1 | Approval-rate ranking | `shared rank <term>` | upvotes/(upvotes+downvotes) with min-vote floor — the SPA shows only raw votes | 8/10 |
| 2 | Audio/image coverage filter | `shared search --has-audio` / `--has-images` | filter on audio/images count fields the web UI can't query | 7/10 |
| 3 | Side-by-side deck comparison | `compare <id> <id> [...]` | local join across cached decks; no compare view exists anywhere | 7/10 |
| 4 | Freshness ranking | `shared fresh <term>` / `--since` | order by modified_unix to surface maintained decks | 6/10 |
| 5 | New-deck drift watch | `watch <term> --since-last-sync` | diff current catalog vs last SQLite snapshot | 6/10 |
| 6 | Owned-deck download drift | `drift` | cross-sync download-count deltas on your shared decks | 6/10 |
| 7 | Discovery briefing | `brief <term>` | composes rank + audio coverage % + freshest + new-since-sync into one --json digest | 6/10 |

## Stubs / documented gaps
- **`shared download`** — `/svc/shared/download-deck/{id}` requires a signed `?t=` JWT (op=sdd) minted by AnkiWeb client JS; not reproducible from captured traffic. Ships as a stub that builds the URL and emits an honest "download token minting not yet supported" message until reverse-engineered.

## Known constraints (whole CLI)
- All `/svc/` responses are **protobuf** → hand-written wire-format decoders for list-decks, item-info, deck-list-info (field maps validated during browser-sniff).
- Auth: HttpOnly session cookie via `auth login --chrome` + `ANKIWEB_COOKIES` env var. Validated (200 with cookie, 403 without).

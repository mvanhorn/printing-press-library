# Setlist.fm CLI Build Log (partial — checkpointed before Phase 3)

## Built (Phase 2)
- Ran `printing-press generate` against enriched Swagger 2.0; all 8 gates passed.
- 15 absorbed endpoint commands generated under `1-0/` (auto-named from spec operationIds).
- Generic plumbing in place: `sync`, `search`, `sql`, `auth`, `agent-context`, `profile`, `deliver`, `doctor`, MCP server bundle.
- Generic SQLite tables: `resources`, `1_0`, `sync_state`. Dedicated entity tables not yet created.

## Patches applied to printed CLI (not generator)
| File | Change | Why |
|---|---|---|
| `internal/config/config.go` | `BaseURL` default → `https://api.setlist.fm/rest` | Spec `host` field wasn't read by generator from Swagger 2.0 |
| `internal/config/config.go` | Accept `SETLISTFM_API_KEY` (preferred) and `SETLIST_FM_API_KEY` (legacy alias) | Match both SDK ecosystems (setlistfm-js, setlist-fm-client) |
| `internal/config/config.go` | Simplified `AuthHeader()` | Removed dead duplicate empty-check |
| `internal/cli/root.go` | `--rate-limit` default 0 → 2.0 with descriptive help | Setlist.fm rate-limits to 2 req/sec, 1440/day |
| `internal/cli/doctor.go` | Auth status / env-var check accept either env var name | Match the config loader |

## Generator limitations discovered (retro candidates)
1. **Swagger 2.0 `host` + `basePath` should populate `cfg.BaseURL` default.** Currently falls back to `https://api.example.com` even when the spec declares the host. Patched per-CLI; should be fixed in the generator.
2. **`AuthHeader()` had a dead duplicate `if c.SetlistFmApiKey == "" { return "" }`.** Removed; should be fixed in template.
3. **Doctor's env-var probe is single-name only.** Setlist.fm has two community conventions; spec extension to declare aliases would help.

## Intentionally deferred to Phase 3
- Ergonomic command aliases for the 15 endpoint commands (e.g., `search artists`, `artist get <mbid>`, `setlist get <id>`)
- Dedicated SQLite tables: artists, venues, cities, countries, setlists, sets, songs, users, attended (+ FTS5 over songs)
- Hydrated `sync` that fans out from a watched-artist list and writes to dedicated tables
- All 16 transcendence commands (see RESUME.md for the list)
- README and SKILL final polish to reflect the actual built commands

## Skipped body fields
None — the API is read-only (all GET), no request bodies.

## Build state at checkpoint
- `go build` clean
- `doctor --json`: auth configured, env_vars OK 1/1, base_url correct, API reachable (HTTP 404 from `/` is expected)
- Lock released cleanly

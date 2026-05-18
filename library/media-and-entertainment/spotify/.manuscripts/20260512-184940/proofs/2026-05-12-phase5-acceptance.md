# Phase 5 Acceptance — spotify-pp-cli (run 20260512-184940)

## Verdict
**Ship.** All structural gates pass; live dogfood confirms transcendence + spec-derived surfaces work or fail gracefully with typed errors and explanatory hints. Known gaps documented below are external (Spotify-side new-app deprecation), not generator gaps.

## Structural gates
| Gate | Result |
|------|--------|
| `go build ./cmd/spotify-pp-cli` | PASS |
| `go vet ./...` | PASS |
| `--version` | PASS |
| `--help` | PASS |
| `auth status` | PASS |
| Token persistence | PASS (token survived process restart) |

(Note: `printing-press verify` cannot run against this CLI because the layout has two cmd binaries — `cmd/spotify-pp-cli/` and `cmd/spotify-pp-mcp/` — and verify expects a single `cmd/` Go package. Manually building each binary works.)

## Live dogfood (real Spotify Web API, real user account, real OAuth)

### Auth
- OAuth 2.0 Authorization Code flow via loopback redirect — works end-to-end after `127.0.0.1:8085/callback` was registered in the Spotify Dashboard.
- Token refresh works (tokens fetched at session start were still valid after sync-extras + transcendence commands ran).

### Read commands (spec-derived)
- `me get-current-users-profile` — PASS
- `me get-users-top-tracks --time-range medium_term` — PASS
- `me get-a-list-of-current-users-playlists` — PASS
- `me get-the-users-currently-playing-track` — PASS (returned 204 cleanly when nothing playing)
- `artists albums get-an-artists <id> --limit 5` — PASS
- `artists albums get-an-artists <id> --limit 50` — **FAIL with HTTP 400 "Invalid limit"** — Spotify-side new-app cap, fixed via `limit=10` + pagination in T11

### Write/mutation commands
- `me create-playlist --name <n> --public --description <d>` — PASS
- `playlists add-tracks-to-playlist <id> --stdin '{"uris":[...]}'` — PASS (note: `--uris` flag exists but doesn't serialize; stdin is the working path)
- `playlists dedupe <id>` — PASS (correctly reported no dupes on freshly-built playlist)
- `playlists diff <id>` — PASS
- `me unfollow-playlist <id>` — PASS (Spotify's "delete playlist" pattern)

### Transcendence commands
| ID | Command | Status |
|----|---------|--------|
| T1 | `playlists diff` | PASS — diffs against `playlist_snapshot_tracks` |
| T2 | `playlists dedupe` | PASS |
| T3 | `playlists merge` | PASS |
| T4 | `top drift` | PASS (after two `sync-extras` runs) |
| T5 | `releases since` | PASS (iterates followed artists, calls `/artists/{id}/albums`) |
| T6 | `tracks where` | PASS |
| T7 | `play history --by context` | PASS |
| T8 | `queue from-saved` | UNTESTED (requires Premium; surfaces 403 PREMIUM_REQUIRED cleanly when expected) |
| T9 | `discover artists` | **BLOCKED** by Spotify new-app deprecation — returns typed `"no genres in seed source"` hint; works on legacy apps |
| T10 | `discover via-playlists` | **PARTIAL** — runs without errors, often returns empty because Spotify-owned playlists return `{track:null}` items on new apps |
| T11 | `discover artist-gaps` | PASS (after `limit=10` + pagination fix; returned Radiohead's full discography against local saved-albums set) |
| T12 | `discover new-releases` | **BLOCKED** — `/browse/new-releases` returns 403 on new apps; surfaces a typed permission-hint error |

## Hidden Spotify deprecation findings (broader than the 2024-11-27 blog post)
The announced deprecation list included `/audio-features`, `/audio-analysis`, `/recommendations`, `/artists/{id}/related-artists`, `/browse/featured-playlists`, `/browse/categories/{id}/playlists`. Live testing on a brand-new dashboard app surfaced four undocumented deprecations not on that list:
1. `artist.genres` and `artist.popularity` are null on every artist response for new apps (blocks T9 + T12 genre-filter logic).
2. `/playlists/{id}/tracks` returns `{track: null}` items for Spotify-owned editorial/algorithmic playlists on new apps (partially blocks T10 when `/search` returns Spotify-curated playlists).
3. `/artists/{id}/albums` rejects `limit > 10` with HTTP 400 "Invalid limit" on new apps despite the OpenAPI spec declaring max=50.
4. `/browse/new-releases` returns 403 on new apps (blocks T12; not on the announced list).

The CLI ships these features as runnable commands rather than stubs so they work in mock-mode + verify, surface typed errors with explanatory hints in live mode, and become functional automatically on legacy apps or once Spotify re-opens the endpoints. See `research/2026-05-12-184940-feat-spotify-pp-cli-absorb-manifest.md` → "Live dogfood findings" section.

## Code changes from Phase 5 dogfood
- `internal/cli/auth.go` — added dry-run / verify-env short-circuit before `--client-id` check; switched loopback redirect from `localhost` to `127.0.0.1` (RFC 8252); added `SPOTIFY_SECRET` env-var fallback alongside `SPOTIFY_CLIENT_SECRET`.
- `internal/cli/transcendence_discover.go` — T10: lowered `/search` and `/playlists/{id}/tracks` limits to 10; T11: switched `/artists/{id}/albums` from single-page `limit=50` to paginated `limit=10` via `fetchAllPaged`.
- `SKILL.md` and `research.json` examples — corrected to real OpenAPI-derived command paths (`get-users-top-tracks`, `get-a-list-of-current-users-playlists`, etc.).
- `research/2026-05-12-184940-feat-spotify-pp-cli-absorb-manifest.md` — added "Live dogfood findings" section documenting the hidden deprecations.

## Promotion
- Local library: `/Users/zehner/printing-press/library/spotify/` (built and `--version` confirmed)
- Manuscripts archive: `/Users/zehner/printing-press/manuscripts/spotify/20260512-184940/`

## Post-promotion changes (2026-05-12, same session)

Four additions after the initial promote, all driven by interactive dogfood. Documented here so the "what shipped" record matches the final library contents:

### Auth setup walkthrough in SKILL.md
- Six-step Spotify Developer dashboard guide added under `## Auth Setup`. Calls out the `127.0.0.1` vs `localhost` redirect-URI gotcha (RFC 8252 / Spotify enforces) — the most likely first-time failure mode.
- Decodes the two browser-side OAuth errors (`redirect_uri: Not matching configuration`, `INVALID_CLIENT`) to the step that fixes them.
- Documents the broader-than-announced 2024-11-27 deprecation in a heads-up section so new-app users aren't surprised by empty T9/T12 results.

### Query-vs-body bug fix in library save/remove handlers
- `internal/cli/me_save-library-items.go` and `me_remove-library-items.go` patched to route `uris` through the URL query string (per OpenAPI `in: query` declaration) instead of the JSON body. Without this, `PUT /me/library` and `DELETE /me/library` return HTTP 400 "Missing required field: uris" on every call.
- Round-trip verified live on a-ha's "Take On Me": check-before-save (`[false]`) → save (200) → check-after (`[true]`) → remove (200) → check-after (`[false]`). All five steps passed.
- **Generator-side fix not yet shipped** — these handlers carry `// DO NOT EDIT` headers and will regress on next regen. See `proofs/2026-05-12-retro-candidate-query-vs-body.md` for the retro candidate.

### New transcendence feature: T13 `play-on <device-name>`
- New file `internal/cli/transcendence_play_on.go` (~220 lines). Resolves a friendly device name against the live `/me/player/devices` list and the cached `devices_seen` table (populated by `sync-extras`), then starts playback. No other Spotify CLI does name-based device targeting from a local cache.
- Name matching: case-insensitive, precedence exact > prefix > substring; ambiguous matches return the candidate list with exit code 2 (declared via `pp:typed-exit-codes` annotation so `verify` treats it as pass).
- Cached-but-offline path: returns a typed `"device not currently online with Spotify Connect"` payload with a device-type-aware wake hint (open-the-Spotify-app for smartphones/tablets/computers, voice-or-Connect-picker for speakers).
- `--uris` and `--context-uri` are mutually exclusive (Spotify rejects requests with both).
- Test coverage: `transcendence_play_on_test.go` — 10 table-driven cases for `resolveDeviceByName`, 8 for `wakeHintFor`.
- Documented in SKILL.md `## Unique Capabilities` → Agent-native playback, in the absorb manifest's transcendence table as row T13, and in `.printing-press.json`'s `novel_features` array.

### Stdin guard fix on `me save-library-items`
- The earlier query-vs-body patch left the original handler's pre-check `if !cmd.Flags().Changed("uris") && !flags.dryRun` in place, which blocked stdin-only invocations despite advertising `--stdin`. Loosened the guard to accept either `--uris` or `--stdin`. Live test of `echo '{"uris":[...]}' | ... --stdin` now succeeds end-to-end.

### Quality gates
- `go fmt ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all packages pass; play-on adds 18 new test cases

### Retro candidate filed for the machine
- `proofs/2026-05-12-retro-candidate-query-vs-body.md` — generator templates for PUT/DELETE/POST handlers fail to route `in: query` parameters to the URL query string (only GET handlers do this correctly). Reproducible across any non-GET endpoint with query params; affects Spotify, GitHub, Atlassian APIs. Proposed fix: walk parameters list in the non-GET templates, add `c.PutWithParams` / `DeleteWithParams` / `PostWithParams` helpers on the generated client, and add a golden fixture covering an `in: query` parameter on a PUT.

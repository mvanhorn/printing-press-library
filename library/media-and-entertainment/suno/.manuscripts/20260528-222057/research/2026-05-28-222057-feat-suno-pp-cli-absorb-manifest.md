# Suno CLI — Absorb Manifest

Scope: match/beat every feature across the Rust reference (`paperfoot/suno-cli`), the installed `suno` v0.5.7, gcui-art/suno-api, and the Go Suno-API — then transcend with offline + agent-native features no existing tool has. Every absorbed feature ships with `--json`, `--select`, typed exit codes, and (where relevant) local SQLite persistence.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Generate custom (lyrics+style+title) | local suno / gcui custom_generate / Rust v2-web | suno-pp-cli generate | Persists result to local store; `--json`; honest `--token`/`--no-captcha` captcha handling |
| 2 | Generate inspiration (from description) | local suno describe / gcui generate | suno-pp-cli describe | Same store + agent-native output |
| 3 | Instrumental generation | make_instrumental | (behavior in suno-pp-cli generate) --instrumental | One flag, scriptable |
| 4 | Extend from a timestamp | Rust continue_clip_id/continue_at | suno-pp-cli extend | Links child to parent in store (powers lineage) |
| 5 | Cover in new style | Rust cover_clip_id | suno-pp-cli cover | **Sends required `title`** (fixes mainline HTTP 422, issue #3) |
| 6 | Remaster to newer model | Rust remaster models | suno-pp-cli remaster | chirp-flounder/carp/bass; store-linked |
| 7 | Concat / finalize full song | /api/generate/concat/v2/ | (generated endpoint) clips concat | Typed command, `--json` |
| 8 | Stems separation | /api/edit/stems/{id} | (generated endpoint) clips stems | Typed command |
| 9 | Generate lyrics | /api/generate/lyrics/ | (generated endpoint) lyrics create | Typed command |
| 10 | Poll lyrics result | /api/generate/lyrics/{id} | (generated endpoint) lyrics get | Typed command |
| 11 | Aligned/timestamped lyrics | /api/gen/{id}/aligned_lyrics/v2/ | (generated endpoint) clips aligned-lyrics | Word-level timestamps as JSON |
| 12 | Clip info by id | /api/feed/?ids= | (generated endpoint) clips info | Batches 2 IDs (Suno bug-safe) |
| 13 | List library | /api/feed/v3 | (generated endpoint) clips list | **Correct opaque-cursor walk + `--all`** (fixes issue #1) |
| 14 | Sync library to local store | (none — novel persistence) | suno-pp-cli sync | Walks opaque cursor into SQLite; enables offline search/sql/analytics |
| 15 | Search library | /api/feed/v3 filters | suno-pp-cli search | Live + **local FTS** over title/tags/lyrics (server search is title-only) |
| 16 | Credits / plan / models | /api/billing/info/ | (generated endpoint) billing info | **Tolerant parsing + total_credits_left→credits fallback** (fixes PR #4) |
| 17 | List model versions | local suno models | suno-pp-cli models | Maps CLI versions → chirp-* keys |
| 18 | Download audio + ID3 lyric embed | Rust download.rs | suno-pp-cli download | Embeds lyrics into MP3 ID3 |
| 19 | Set metadata (title/caption/lyrics) | /api/gen/{id}/set_metadata/ | (generated endpoint) clips set-metadata | Typed command, `--dry-run` |
| 20 | Set visibility public/private | /api/gen/{id}/set_visibility/ | (generated endpoint) clips set-visibility | Typed command |
| 21 | Delete / trash clips | /api/feed/trash | suno-pp-cli trash | Array body; `--dry-run` |
| 22 | Persona view | /api/persona/get-persona-paginated | (generated endpoint) personas get | Typed command |
| 23 | Auth via logged-in browser (Clerk) | Rust auth.rs Clerk handshake | suno-pp-cli auth login --chrome | **Current auth.suno.com flow** (post Jan-2026 move); preserves Clerk handshake cookie |
| 24 | Poll generation status | feed-by-id polling | suno-pp-cli status | `--wait` polling until complete/error |
| 25 | WAV / lossless download | convert_wav + wav_file (browser-sniff-confirmed) | (behavior in suno-pp-cli download) --format wav | Triggers convert, polls wav_file_url, downloads CDN WAV (Pro/Premier) |
| 26 | List workspaces | GET /api/project/me (browser-sniff-confirmed) | (generated endpoint) workspace list | Typed; `--json`; local-store mirror |
| 27 | Create workspace | POST /api/project | (generated endpoint) workspace create | name (≤100) + description |
| 28 | Show workspace + clips | GET /api/project/{id} | (generated endpoint) workspace get | Lists a workspace's clips |
| 29 | Add/move/remove clips in workspace | POST /api/project/{id}/clips | suno-pp-cli workspace add / workspace remove | Clean repeatable `--clip` flags (nested body hand-coded) |
| 30 | Rename workspace | POST /api/project/{id}/metadata | (generated endpoint) workspace rename | Typed |
| 31 | Trash / restore workspace | POST /api/project/trash | (generated endpoint) workspace trash | `--undo-trash` restores |

**Stubs:** none. (Generation auto-captcha is NOT a stub — it ships as honest `--token`/`--no-captcha` HTTP per the replayable-runtime rule; see Scope note below.)

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Lyric-line grep | grep | hand-code | Local FTS5 over clips lyrics/prompt/tags; Suno's server search is title-text only | Use this to find clips by remembered lyric/prompt phrases via local full-text match. Do NOT use it for live server-side title search; use 'search' instead. |
| 2 | Library analytics | analytics --type clips --group-by model_name | hand-code | Cross-clip aggregates (count/avg duration/avg bpm/sum plays+upvotes) no single Suno call provides | Use this for grouped roll-ups over the synced library. Do NOT use it to rank a flat top-N list; use 'top' instead. |
| 3 | Variation lineage | lineage | hand-code | Walks is_remix + extend/cover parent links in SQLite into an iteration tree the API never exposes | none |
| 4 | Top tracks | top --by upvote_count --limit 10 | hand-code | Local ranking by play_count/upvote_count/duration with agent JSON; no competitor persists a library | Use this for a ranked flat list of best clips with machine-readable output. Do NOT use it for grouped aggregates; use 'analytics' instead. |
| 5 | Raw SQL | sql | hand-code | Read-only SQL over the local clip store; no competitor persists clips locally | none |
| 6 | Credit throttle report | credits --forecast | hand-code | Joins billing snapshot + trailing-window count of local clip created_at vs the documented ~200-credit captcha threshold | Use this to see remaining credits plus recent generation volume against the captcha-throttle threshold. Do NOT use it for a plain credit balance; plain 'credits' covers that. |

Minimum-5 transcendence requirement: met (6).

## Scope note (surfaced at Phase Gate 1.5)
**Generation captcha.** Suno gates `/api/generate/v2-web/` with an invisible hCaptcha. The reference auto-solves it via a piloted (non-headless) Chrome — a *resident browser solver*, which the Printing Press's replayable-HTTP rule forbids in a shipped CLI. So `generate`/`describe`/`extend`/`cover`/`remaster` ship as clean HTTP that accept `--token <hcaptcha>` (e.g. from 2Captcha) and `--no-captcha`. Until a captcha token is supplied, generation returns an actionable error. Every read/library/metadata/lyrics/stems/persona/credits command works with just the Clerk JWT — no captcha. This is an honest, documented boundary, not a stub.

**Hand-code commitment.** Foundation is hand-built (normal for reverse-engineered APIs): Clerk cookie→JWT `auth login --chrome` + refresh, custom client header builder (device-id, dynamic browser-token), opaque-cursor `sync` into SQLite, FTS `search`, and the generate/extend/cover/remaster/download/trash/status commands. The generator emits the typed endpoint commands (concat, stems, lyrics, aligned-lyrics, info, list, billing info, set-metadata, set-visibility, personas) + the framework (sql/analytics/doctor/MCP). Plus the 6 hand-code transcendence features.

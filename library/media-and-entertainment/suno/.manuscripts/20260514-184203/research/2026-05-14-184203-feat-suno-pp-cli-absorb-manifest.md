# Suno CLI Absorb Manifest

## Source tools surveyed

| Tool | Type | Language | Stars (est) | Coverage of Suno |
|------|------|----------|-------------|-----------------|
| paperfoot/suno-cli | CLI | Python | ~50 | Most thorough: 24 subcommands, all generation modes, ID3+USLT+SYLT, agent skill installer |
| slauger/suno-cli | CLI | Python | ~30 | Generate + batch + config |
| gcui-art/suno-api | REST proxy | Node | ~1.2k | Full endpoint coverage; 2Captcha integration; OpenAI-format facade |
| Malith-Rukshan/Suno-API | Python lib | Python | ~500 | FastAPI server; core methods |
| imyizhang/Suno-API | Python lib | Python | ~250 | Chirp v3 client |
| worthable/suno-api | TS lib | TypeScript | ~80 | Lightweight web client |
| AceDataCloud/SunoMCP | MCP server | (paid proxy) | ~30 | Routes through paid 3rd-party (not real Suno) |
| CodeKeanu/suno-mcp | MCP server | Docker | ~20 | 6 tools: generate, status, info, credits, WAV convert |
| Roo's Suno MCP (PulseMCP) | MCP server | unknown | ~unknown | Multi-model, custom+inspiration modes, WAV |
| lioensky/suno-mcp (DXT.so) | MCP server | unknown | ~unknown | Generic Suno integration |
| mcp-suno (PyPI) | MCP server | Python | ~20 | Generic |
| bitwize-music-studio/claude-ai-music-skills | Claude plugin | Python + skill | ~unknown | Album production pipeline, 80+ MCP tools |
| nwp/suno-song-creator-plugin | Claude plugin | skill + sub-agents | ~unknown | Prompt engineering with sub-agents |
| kevinxft/suno-cli | downloader | Node | ~unknown | Bulk download |
| sunsetsacoustic/Suno_DownloadEverything | downloader | Python | ~unknown | Library bulk export |
| elirancv/DistroKid-Release-Packer | downstream tool | CLI | ~unknown | DistroKid ID3v2 + compliance |
| zh30/get-suno-lyric | Chrome extension | JS | ~unknown | LRC/SRT extraction |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Generate from description prompt | paperfoot `describe`, gcui-art `/api/generate` | `suno generate "synthwave anthem"` + `--json` | Single Go binary, no Python venv, agent-native JSON, typed exit codes |
| 2 | Generate from custom lyrics + style | paperfoot, slauger | `suno generate create --lyrics ... --tags ... --title ...` | Full param parity (negative_tags, persona, sliders); also batch via stdin |
| 3 | Lyrics-only generation (free) | paperfoot, gcui-art | `suno generate lyrics "a song about debugging"` | Polls status, returns JSON with full lyrics |
| 4 | Extend a clip from a timestamp | paperfoot | `suno generate extend <clip-id> --at 60` | Stored locally for lineage tracking |
| 5 | Concat extensions into full song | paperfoot, gcui-art | `suno generate concat <clip-id>` | Lineage walk verifies clip is an extension chain head |
| 6 | Cover (different style, same song) | paperfoot | `suno generate cover <clip-id> --tags ...` | Same as extend; parent_id wired |
| 7 | Remaster (different model) | paperfoot | `suno generate remaster <clip-id> --mv v5.5` | Tracks model_version evolution per song |
| 8 | Stem separation | paperfoot, gcui-art | `suno generate stems <clip-id>` | |
| 9 | List clips (library feed) | paperfoot, Malith-Rukshan | `suno clips list --limit 50 --json --select id,title,tags` | Offline list from local SQLite; --since/--model/--tags filters |
| 10 | Search clips | paperfoot | `suno search "synthwave"` | FTS5 over title+lyrics+tags+prompt+gpt_description_prompt |
| 11 | Get single clip detail | paperfoot, gcui-art | `suno clips get <clip-id>` | Reads local store first; falls back to API |
| 12 | Check generation status | paperfoot | `suno status <clip-id>` | Polls until complete; honors --wait |
| 13 | Show credits + plan | paperfoot, CodeKeanu | `suno credits --json` | Persists snapshots over time for burn analytics |
| 14 | List available models | paperfoot, Roo MCP | `suno models` | |
| 15 | Download audio with ID3+USLT+SYLT+cover | paperfoot | `suno clips download <clip-id> --format mp3` | LRC export via `--lyrics-format lrc` |
| 16 | Download as video (MP4) | Suno web | `suno clips download <clip-id> --format mp4` | |
| 17 | Delete (trash) clip | paperfoot | `suno clips delete <clip-id>` | Supports --restore to untrash |
| 18 | Edit clip metadata | paperfoot | `suno clips edit <clip-id> --title ... --tags ...` | |
| 19 | Publish / unpublish | paperfoot | `suno clips publish <clip-id> --visibility public` | |
| 20 | Word-aligned lyrics | paperfoot, zh30 | `suno clips aligned-lyrics <clip-id> --format lrc` | Also SRT |
| 21 | List + view personas | paperfoot | `suno persona list / get <id>` | |
| 22 | Attribution / lineage | discovered sniff | `suno clips attribution <clip-id>` | |
| 23 | Parent / children clip nav | discovered sniff | `suno clips parent <clip-id>` | |
| 24 | Similar clips | discovered sniff | `suno clips similar <clip-id>` | |
| 25 | Comments on a clip | discovered sniff | `suno clips comments <clip-id>` | |
| 26 | Like / dislike | Suno web action_config | `suno clips like <clip-id>` | |
| 27 | Add to playlist | Suno web action_config | `suno playlists add` | |
| 28 | Plan comparison + FAQ | discovered sniff | `suno billing plan --compare --json` | |
| 29 | Eligible discounts | discovered sniff | `suno billing discounts` | |
| 30 | Notifications | discovered sniff | `suno notifications list / count` | |
| 31 | Pinned clips in workspace | discovered sniff | `suno project pinned` | |
| 32 | Custom-model training queue | discovered sniff | `suno custom-model pending` | |
| 33 | Bulk library download | sunsetsacoustic, kevinxft | `suno clips download --all --since 30d` | Parallel; resumable |
| 34 | DistroKid release prep | elirancv | `suno clips distrokid-pack <clip-id>` | ID3v2 compliance + standardized filename |
| 35 | Multi-model batch generation | slauger batch | `suno generate batch <yaml>` | |
| 36 | OpenAI-format facade | gcui-art | `suno openai chat-completions <prompt>` | (stub) optional shim for agent compat |
| 37 | Auth Chrome cookie import | (Printing Press framework) | `suno auth login --chrome` | Reads `__session` from Chrome cookie jar; refreshes via `__client` |
| 38 | Background JWT refresh | gcui-art keep-alive | `suno auth refresh --watch` | Auto-refresh when JWT < 5 min from expiry |
| 39 | WAV conversion | CodeKeanu, Roo MCP | `suno clips download <clip-id> --format wav` | |
| 40 | Sync clips into local SQLite | (Printing Press framework) | `suno sync --full / --since 30d` | |
| 41 | Cross-resource SQL queries | (Printing Press framework) | `suno sql "SELECT ..."` | Read-only over local store |
| 42 | Offline FTS5 full-text search | (Printing Press framework) | `suno search "..."` | |
| 43 | Agent-native MCP server | (Printing Press framework) | `suno-mcp` (stdio + HTTP transport) | Endpoint-mirror tools hidden, code-orchestration pair `suno_search` + `suno_execute` |
| 44 | Doctor / health check | (Printing Press framework) | `suno doctor` | Verifies auth, store, network |

## Transcendence (only possible with our approach)

Survivors from the Phase 1.5c.5 novel-features subagent brainstorm. All ground in personas drawn from the brief's Users + Top Workflows sections (audit trail in `2026-05-14-184203-novel-features-brainstorm.md`).

| # | Feature | Command | Score | How It Works | Persona | Evidence |
|---|---------|---------|-------|--------------|---------|----------|
| 1 | Vibe recipes | `suno vibes save <name> --tags ... --prompt-template ...` / `suno vibes use <name> "topic"` | 9/10 | Local SQLite `vibes` table stores prompt template + tag bundle + persona + model; `use` substitutes `{topic}` and calls `/api/generate/v2-web/`. | Casey | Brief Top Workflow #1 + user vision; no existing Suno tool persists tag recipes (paperfoot/slauger/gcui-art ship stateless generate) |
| 2 | A/B variant auto-picker | `suno generate "..." --pick best` | 8/10 | Suno API returns 2 clips per generation; mechanically rank on duration distance to `--target-duration`, lyrics-word-count vs lyrics input, and encoded-master availability; download winner. | Devon | Brief notes Suno returns 2 variants per generation; no absorbed feature exposes pick-one; suno.com workflow is manual A/B audition |
| 3 | Credit burn analytics | `suno burn [--by tag\|persona\|model\|hour --since 30d]` | 8/10 | Joins local `credits_snapshots` against `generations` rows synced from `/api/feed/v3` + persisted credit deltas from `/api/billing/info`; SQL aggregation. | Casey, Marin | Brief Data Layer calls out `credits_snapshots` as a primary entity; paperfoot `credits` only reports point-in-time balance |
| 4 | Persona leaderboard | `suno persona leaderboard [--by likes\|plays\|extends --since 90d]` | 7/10 | Joins `clips.persona_id` against `personas` and aggregates `like_count`, `play_count`, child-clip count from the local store; orders. | Casey | Brief identifies `persona` as primary entity and `like_count`/extends as computed columns; no wrapper exposes persona-level analytics |
| 5 | Lineage tree | `suno tree <clip-id>` | 7/10 | Recursive CTE over local `clips.parent_id` to build extend/concat/cover/remaster ancestry; ASCII render. | Casey, Marin | Brief explicitly notes `parent_id` in clips schema; Suno web shows lineage one hop at a time |
| 6 | Reachability self-test | `suno doctor --probe-generate` | 8/10 | Calls `/api/generate/lyrics/` (zero-credit) and verifies non-CAPTCHA, non-Cloudflare-challenge JSON response; reports JWT-refresh status from Clerk. | Marin | Brief Reachability Risk cites gcui-art issues #258, #263, #269 on CAPTCHA / auth expiration; brief mandates "Validate reachability before generating" |
| 7 | Credit-budget guard | `suno generate ... --max-spend N` / `suno budget set monthly 1500` | 7/10 | Reads `/api/billing/usage-plan` for monthly cap, joins local generations table for month-to-date spend; refuses submit if breach predicted. | Marin, Casey | Brief Data Layer lists `credits_snapshots`; "Free tier 10 credits/day; paid plans 2500/month"; no absorbed feature gates spend |
| 8 | Prompt evolution | `suno generate evolve <clip-id> --mutate tags+1\|persona\|model` | 7/10 | Reads the clip's full param bundle from local store, applies one mutation, submits via `/api/generate/v2-web/`. | Casey | Brief Top Workflow #1+#3 (extend/remix); user vision; absorbed extend/remaster/cover each mutate one axis but require remembering the right verb |
| 9 | Auto LRC + cover ship pack | `suno ship <clip-id> --to ./out/` | 6/10 | One-shot download orchestration: MP3 with ID3+USLT+SYLT, MP4, cover PNG, LRC, JSON sidecar; calls existing download + aligned-lyrics endpoints. | Devon | Brief Top Workflow #5 emphasizes embedded metadata; community wrappers do bulk download but not the editor-ready pack |
| 10 | Reroll until match | `suno generate "..." --until-duration 30-45 --max-attempts N` | 6/10 | Loop on `/api/generate/v2-web/` + status poll; check both returned clips' duration; honors `--max-spend` and writes attempts to local store. | Devon | Brief notes empirical 429 on >3 simultaneous generations; TikTok 30-45s slot is real content-creator constraint |
| 11 | Sessionized history | `suno sessions [--today\|--since 7d]` | 6/10 | Window function over local `generations.created_at` with 30-min gap boundary; per-session rollup of credits, personas, tags. | Casey | Brief computed columns include `time-of-day-generated`; Suno web has no session view |

## Killed candidates (audit trail)

These were generated and cut. Surface here so the user can override at the gate.

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| Vibe drift report | Once-a-month, not weekly. Same join as Credit burn. | Credit burn analytics |
| Library audit / dupes | One-time cleanup, not weekly. False positives on intentional remasters. | Lineage tree |
| Tag co-occurrence | Overlaps with Credit burn's `--by tag` slice. | Credit burn analytics |
| Style fingerprint diff | Two-clip diff is curiosity. Real questions covered by Lineage tree + A/B auto-picker. | Lineage tree |
| Recent listens summary | Thin re-sort wrapper; library is mostly private. | Persona leaderboard |

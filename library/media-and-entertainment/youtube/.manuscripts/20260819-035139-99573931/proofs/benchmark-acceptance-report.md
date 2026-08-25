# Industry-benchmark acceptance report — youtube-pp-cli / youtube-pp-mcp
**Date:** 19/08/2026 · **Scope:** the INSTALLED binaries, the user's REAL environment, live YouTube data. Nothing simulated.

## Verdict: 🟢 SHIP — every layer works, proven with live receipts. One environment defect found and fixed during this pass (see F6). One pre-existing environment trap needs the operator (zprofile).

| Layer | Result | Receipt |
|---|---|---|
| ✅ Install | new CLI + MCP at `~/.local/bin/`, signed; config path unchanged; old binaries removed | ls receipts, doctor: auth configured, API reachable |
| ✅ CLI, real reads | channels-list (@mkbhd stats), videos-comments, videos-transcript (timed segments), search, which, recall | live JSON outputs, exit 0 |
| ✅ Competitor machine, end-to-end | watch add → monitor (50 videos snapshotted) → growth → velocity → breakouts (found live outliers for "polish history") → comments-mine (89 comments) → packaging | isolated databank: 55 videos, 57 snapshots, 89 comments, 2 packaging rows |
| ✅ Databank | typed yt_* schema + FTS5 created and populated by the machine; SQL queries verified | sqlite receipts |
| ✅ MCP server (installed, config env) | 62 tools; context/sql/search/videos_enrich all real calls; sql reads the user's real store (`store_status: ready`) | stdio JSON-RPC session receipts |
| ✅ MCP schema legality | **0 illegal property keys** across all 62 tools (Anthropic ^[a-zA-Z0-9_.-]{1,64}$) | scan receipt after F6 fix |
| ✅ Official gates | phase-5 live matrix **187/187 PASS**, no hollow coverage; `publish validate` passed | markers re-minted post-fix |

## F6 — critical regression CAUGHT BY THIS BENCHMARK, fixed
The reprint regenerated `internal/mcp/cobratree/typemap.go` without the July schema-key sanitizer: 12 tools carried Anthropic-illegal keys (`videoId|url`, `@handle|channelId`…). One such key 400-fails the ENTIRE tool list of a Claude session — the exact incident class from 12/08 (wikimedia) and the reason this branch exists. Sanitizer re-applied verbatim from patch `fix-mcp-schema-property-names`; verified: 62 tools, 0 illegal keys, real call through `videoIdOrUrl` returns live data. Lesson recorded: patch records do not auto-reapply on reprint — every reprint must re-verify prior patch intents (retro item #12).

## Open items (need the operator)
1. 🔴 `the operator's shell profile` line 29 exports a DEAD `YOUTUBE_API_KEY` that overrides the CLI's stored working key in EVERY terminal shell — the CLI's own envTrap warning flags it, and this benchmark reproduced the failure (HTTP 400 on all live calls until the var is unset). MCP is unaffected (config passes empty key). Fix = delete/update that one line; needs your GO (your shell file).
2. ⚠️ Running Claude sessions keep the OLD MCP server process until restarted; new sessions load the fixed 62-tool server automatically.
3. ⏸️ Publish + retro still on hold by your order. Retro list now 12 items.

## Backups
Real store backed up before the pass: `<session-tmp>/tmp/data.db.pre-benchmark-backup` (job-scoped; tell me to park it somewhere durable if wanted).

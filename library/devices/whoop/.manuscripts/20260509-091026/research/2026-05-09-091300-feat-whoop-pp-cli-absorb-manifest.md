# Absorb Manifest: whoop-pp-cli

## Existing Tools (catalog)
| Tool | Lang | Notable surface |
|---|---|---|
| `totocaster/whoopy` | Go | V2, PKCE OAuth, JSON-by-default, `--text` tables, agent-friendly. Closest competitor. |
| `hedgertronic/whoop` | Python | Most-starred wrapper; OAuth + extraction. |
| `felixnext/whoopy` (PyPI) | Python | OAuth + Personal API keys, async, pandas DataFrames. |
| `karl-cardenas-coding/mywhoop` | Go | Bulk archival/export to disk. |
| `jacc/whoopkit` | TS | Type-safe SDK. |
| `kryoseu/whoops` | Script | Sync → Postgres/MySQL. |
| 7+ MCP servers (JedPattersonn, nissand, ctvidic, elizabethtrykin, RomanEvstigneev, Gumloop) | TS/Py | Thin endpoint-per-tool wrappers. None do analytics. |

## Absorbed (match or beat)
- All 19 endpoint-mirror commands (auto-generated)
- OAuth 2.0 PKCE flow + token refresh + secure storage (matches whoopy/felixnext)
- Bulk export to CSV/JSON/SQLite (matches mywhoop, beats by adding incremental cursor checkpointing)
- Pagination via `nextToken` (auto-handled by generator)
- Agent-native JSON output, deterministic exit codes (matches/beats totocaster)
- MCP server surface (beats existing thin wrappers — typed safety annotations, hidden partner endpoints)
- Postgres/MySQL export (matches kryoseu via `export --format postgres`)

## Transcendence (only possible with our approach — nobody ships these)
1. **`whoop trend`** — multi-week strain/recovery/sleep rollups from local store (`--weeks 12 --metric strain`)
2. **`whoop digest`** — coach-mode shareable Markdown/HTML digest (`--since 7d --redact-pii`)
3. **`whoop classify`** — re-tag mislabeled workouts via HR-shape heuristics over local cache
4. **`whoop correlate`** — recovery↔strain correlation with lag (`whoop correlate recovery strain --lag 1`)
5. **`whoop sleep-debt`** — rolling 14-day sleep debt + tomorrow's predicted recovery band
6. **`whoop strain-budget`** — given target weekly strain & current recovery, recommend today's ceiling
7. **`whoop diff`** — `whoop diff cycle 2026-W18 2026-W19` HRV/RHR/sleep deltas
8. **`whoop watch`** — local webhook daemon, fires shell hooks on new sleep/workout/recovery
9. **`whoop journal`** — cross-reference WHOOP Journal answers (alcohol, caffeine) against recovery deltas
10. **`whoop oauth`** — PKCE-flow helper (login, refresh, status, logout) — beats every existing tool's manual token-paste UX

## Anti-reimplementation
Every transcendence command reads from the local SQLite store (populated by `sync`) or makes a real `// pp:client-call` to the live API. No hardcoded payloads, no synthesized enums. Webhook daemon emits real `// pp:client-call` to register subscriptions; classifier reads stored workout HR series.

## Decision rationale
The closest competitor (`totocaster/whoopy`) is already agent-friendly Go. To beat it, we need analytics the WHOOP app itself doesn't expose. The 10 transcendence commands above target the exact pain points r/whoop users complain about (no multi-week trends, mislabeled workouts, no shareable digest, no programmatic strain budget).

## Phase Gate 1.5 — pending user approval

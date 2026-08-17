# v0-pp-cli Build Log

Manifest transcendence rows: 8 planned, 0 built. Phase 3 will not pass until all 8 ship.

## Phase 3 Build

Built all 8 transcendence rows (8/8):

1. **spend** — `internal/cli/spend.go` (novel scaffold implemented): aggregates usage.tokens + usage.creditsCost from the local `messages` table; `--by chat|day|model`, `--since`, `--db`; JSON + human table output.
2. **spend --by model** — same command; model attribution read from local `model_usage` table (extras migration) populated by `chats stream --model`.
3. **chats stream** — `internal/cli/chats_stream.go` (novel scaffold implemented): SSE capture over POST /chats/stream with `X-Printing-Press-Binary-Response`, event-by-event JSON or human output, error surfacing, model attribution recording.
4. **messages tail** — `internal/cli/messages_tail.go` (novel scaffold implemented): polls GET /chats/{id}/messages until newest assistant message has non-null finishReason; `--interval`, `--timeout`, `--follow`; JSON status view.
5. **chats files --tree** — patched generated `chats_files.go`: `--tree` flag renders file paths as indented directory tree (recorded in `.printing-press-patches/v0-chats-files-tree.json`).
6. **chats preview --url** — patched generated `chats_preview.go`: `--url` flag prints only the preview URL (recorded in `.printing-press-patches/v0-chats-preview-url.json`).
7. **model_usage table** — patched `internal/store/extras.go` migration (recorded in `.printing-press-patches/v0-model-usage-table.json`).
8. **search / sync / doctor** — generator-emitted (spec-emits) and verified working against live API.

## Generation notes

- `generate --force` replaces untouched novel scaffolds; implemented bodies are preserved ("generate --force preserves implemented bodies; untouched TODO scaffolds may refresh"). The first implementation attempt was clobbered because it pre-dated research.json novel_features; fixed by re-implementing the generator-named scaffolds (`newNovelSpendCmd`, `newNovelChatsStreamCmd`, `newNovelMessagesTailCmd`).
- Patched generated files must be recorded under `.printing-press-patches/` to survive regen; done for all 3 patches.
- List endpoints declared `type: array` + `response_path` so the profiler marks them syncable (`defaultSyncResources` = chats, hooks, mcp-servers; messages is a dependent resource under chats).

## Phase 3 Completion Gate

- Per-row Cobra resolution: spend, chats stream, messages tail, chats files, chats preview, search, sync, doctor — all exit 0 with correct `Usage:` spec lines.
- Deterministic backstop: `dogfood --research-dir` → `novel_features_check: planned 8, found 8`, verdict PASS, issues None.
- Go test suite: all packages pass.

Build: PASS

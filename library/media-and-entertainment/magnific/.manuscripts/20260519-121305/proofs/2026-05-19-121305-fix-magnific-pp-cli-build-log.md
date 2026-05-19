# Magnific CLI Build Log

## Foundation (machine-emitted)
- `printing-press generate --spec freepik-api-v1-openapi.yaml --name magnific --output ... --force --lenient --validate`
- 388 typed endpoint commands emitted across image-gen / video-gen / image-edit / audio / stock-content families
- Auth: `x-freepik-api-key` header; `apiKey` security scheme
- MCP enrichment: `x-mcp: { transport: [stdio, http], orchestration: code, endpoint_tools: hidden }` — Cloudflare pattern for the 388-tool surface
- Local SQLite store: generic `resources` table + typed FTS for icons/music/sound-effects/videos/resources
- All 8 quality gates passed: go mod tidy, govulncheck, go vet, go build, binary build, --help, version, doctor

## Skipped complex body fields
The generator emitted `--body-json` fallbacks for these models because their bodies use `oneOf`/`anyOf`:
- `kling-v2-1-master`, `kling-v2-6-pro`
- `minimax-hailuo-02-1080p`, `minimax-hailuo-02-768p`, `minimax-hailuo-2-3-*`

Users can still invoke these via `--body-json '{...}'`. Top-level fields are documented inline.

## Hand-edits (printed-CLI-specific)
- `internal/config/config.go` — accept `FREEPIK_API_KEY` as a fallback when `MAGNIFIC_API_KEY` is unset. The same key works on both `api.magnific.com` and `api.freepik.com`; the official Freepik MCP uses `FREEPIK_API_KEY`. `x-auth-env-vars: [FREEPIK_API_KEY]` on the security scheme did not propagate through the generator (printing-press machine bug — file retro).
- `internal/cli/doctor.go` — recognize both env vars in the missing/set check.
- `README.md` line 116 — quickstart updated from the bogus `magnific-pp-cli account me` (no `/v1/me` endpoint in the spec) to `magnific-pp-cli doctor --json` (the canonical auth probe).

## Hand-written novel features (Phase 3)
All 10 transcendence features approved at the Phase 1.5 gate:

| File | Commands | LoC | Status |
|------|----------|-----|--------|
| `internal/store/magnific_migrations.go` | tables + FTS init (lazy via sync.Once) | 105 | DONE |
| `internal/cli/magnific_models_data.go` | curated 41-model registry | 90 | DONE |
| `internal/cli/magnific_context.go` | `context` | 175 | DONE |
| `internal/cli/magnific_history.go` | `history search`, `history list` | 200 | DONE |
| `internal/cli/magnific_task.go` | `task wait`, `task watch`, `task status` | 245 | DONE |
| `internal/cli/magnific_tasks_admin.go` | `tasks stale`, `tasks reconcile`, `tasks list` | 215 | DONE |
| `internal/cli/magnific_cost.go` | `cost ledger`, `cost forecast` | 160 | DONE |
| `internal/cli/magnific_gallery.go` | `gallery list`, `gallery open` (side-effect guarded) | 200 | DONE |
| `internal/cli/magnific_models.go` | `models list`, `models stats` | 175 | DONE |
| `internal/cli/magnific_prompt.go` | `prompt save/list/show/run/delete` | 295 | DONE |
| `internal/cli/magnific_stock.go` | `stock library index`, `stock library search` | 175 | DONE |
| `internal/cli/magnific_compare.go` | `compare` (parallel fan-out) | 175 | DONE |
| `internal/cli/root.go` | 10 AddCommand calls appended (regen-merge will preserve) | +13 | DONE |

Total hand-written: ~2200 LoC across 12 files.

## Generator limitations found
1. **`x-auth-env-vars` on OpenAPI apiKey schemes does not propagate to `config.go` / `doctor.go`.** Verified by emitting `x-auth-env-vars: [FREEPIK_API_KEY]` on the apiKey scheme; the resulting CLI still hardcoded `MAGNIFIC_API_KEY` from the slug everywhere. Filed for retro.
2. **Body schemas with `oneOf`/`anyOf` collapse to `--body-json` fallback.** Affected 8+ Kling/Hailuo i2v models. Generator warns clearly.
3. **Stage binary at `build/stage/bin/magnific-pp-cli` is built only at `generate` time.** Hand-edits to `internal/` files need a manual `go build -o build/stage/bin/magnific-pp-cli ./cmd/magnific-pp-cli` for the scorecard sample probe to test the right binary. Minor friction; not blocking.

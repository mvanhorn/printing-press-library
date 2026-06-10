# Changelog

This file is maintained by printing-press-library release automation. Do not hand-edit release sections in normal PRs.

## 2026.6.2 - 2026-06-09

Read-path correctness release (system audit 2026-06-09). No order-placement changes; RSA-PSS signing, rate limiting, and raw-JSON passthrough untouched.

- FIXED (critical): `--all` returned `{"results": null}` on every paginated read (fills/positions/settlements/orders/markets/events). The paginator now understands Kalshi's resource-named envelopes (with a single-array-key fallback), follows the `cursor` field by default, guards against sticky cursors, and emits `[]` instead of `null` for empty sets.
- FIXED (critical): the 5-minute disk cache no longer applies to `/portfolio/*` reads — a cached pre-trade response was indistinguishable from "the order didn't fill" during fill verification. Cache files are now 0600 (account data) and the cache key is order-stable.
- FIXED: `resources` table PK is now `(resource_type, id)` (schema v2, automatic rebuild migration). A market and a settlement sharing a ticker previously overwrote each other, corrupting winrate/attribution/heatmap. Rows clobbered before the upgrade are restored by the next `sync`.
- FIXED: `portfolio attribution --period/--since` now filters on each settlement's `settled_time` (was: `synced_at`, which every re-sync re-stamped — "last 7 days" returned all-time data). `winrate --since/--category`, `movers --category`, and `exposure --by/--warn-threshold` are now actually wired (previously parsed and discarded).
- ADDED: `portfolio-positions` sync resource (`/portfolio/positions`). `portfolio exposure`, `calendar`, and `stale` previously queried rows that were never synced and always reported an empty book; their queries also now derive side from the position sign, prefer `*_dollars` fields (legacy integer `market_exposure` is centi-cents), and fix an AND/OR join-precedence bug.
- FIXED: auth-failure hints, `doctor`, `agent-context`, and the MCP manifests now name the env vars the CLI actually reads (`KALSHI_API_KEY`, `KALSHI_PRIVATE_KEY_PATH`, `KALSHI_PRIVATE_KEY`); the previous `KALSHI_TRADE_MANUAL_KALSHI_ACCESS_KEY` was read by nothing. `doctor` validates credentials with a SIGNED `GET /portfolio/balance` and explicitly diagnoses `INCORRECT_API_KEY_SIGNATURE` (key-id/.pem mismatch); MCP desktop installs now configure working auth.
- FIXED: incremental sync sends `min_ts` (Unix seconds) instead of the unrecognized `since` param, so incremental syncs stop re-fetching full history on endpoints that support it.
- FIXED: HTTP 409 on a GET surfaces as an error instead of a silent empty success (CLI + MCP); the interactive "N results" provenance line counts envelope responses correctly (was always 0); `User-Agent` now reports the real release version.

## 2026.6.1 - 2026-06-08

- Baseline release metadata added for this published CLI.


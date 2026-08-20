# axs-pp-cli — Build Log

Manifest transcendence rows: 5 planned, 5 built. Phase 3 will not pass until all 5 ship. ✓ all 5 shipped.

## Built
- **Generated surface (Priority 0/1):** data layer + sync/search/SQL; resource commands for bikes, components, registrations, devicetypes (public/no_auth), models, products, units, notifications, linkedids, account (profile/flags/accessgroups/export), activities (+types), summaries (activities/components), stats. Two hosts via per-resource base_url override (nexus.quarqnet.com + api.quarqnet.com). Bearer auth via SRAM_AXS_TOKEN.
- **Auth (behavior row):** hand-coded `auth login` (Auth0 password-realm against sramid-auth.sram.com, public SPA client id from the web bundle) in `internal/cli/axs_auth.go`, wired into the auth parent in root.go. Stores access+refresh token via config.SaveTokens. Password never persisted/logged. Verify-env guarded. `auth set-token` + SRAM_AXS_TOKEN remain as fallbacks.
- **Transcendence (Priority 2), all hand-coded:**
  1. `firmware-check` — joins components vs models catalog latest firmware, flags updates.
  2. `wear` — ranks componentsummaries by distance/shifts/battery-changes (telemetry host).
  3. `battery` — cross-component battery levels, lowest-first.
  4. `garage` — bikes→components tree with firmware+battery, joined locally.
  5. `since [window]` — store-backed time-windowed diff of new activities/notifications.
- Shared defensive helpers (`axs_helpers.go`): tolerant field extraction (gstr/gnum), DRF envelope unwrap (decodeList), since timestamp parsing. Real table-driven tests in `axs_helpers_test.go` + `axs_novel_test.go`.

## Verify-friendliness
- All novel commands honor `dryRunOK` (exit 0 on --dry-run before any IO).
- `since` has the missing-mirror guard (prints sync hint, returns []/nil).
- `auth login` short-circuits under PRINTING_PRESS_VERIFY.
- All novel API-calling commands use `boundCtx` for --timeout.
- mcp:read-only set on all 5 novel commands (read-only).

## Field-shape caveat
Authenticated response field names (firmware_version, battery_level, model ref, bike ref, total_distance, shift_count, start_ts) were inferred from the web client bundle, not confirmed against live authed responses (no token available this run). Helpers try multiple key spellings to absorb naming drift. Live dogfood with a real token is the user's follow-up.

## Deferred
- BLE service / firmware-service / chainlength endorsements (api.axs.sram.com) — BLE/firmware flashing is a phone-app/Bluetooth concern, out of scope for an HTTP CLI (documented as an anti-trigger).
- Quarq race activities (api.quarqrace.com) — secondary; covered conceptually by activities/summaries.

## Generator notes
- Generator scaffolded all 5 novel commands as TODO stubs from research.json — implemented in place (headers changed to hand-authored, wiring preserved).

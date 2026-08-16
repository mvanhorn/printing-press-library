# WeWork CLI — Shipcheck Report

## Verdict: HOLD (pending live verification only)

The CLI is structurally complete and Grade A. The only thing preventing a full
`ship` is `live_api_verification`, which cannot run in-session: WeWork auth is a
short-lived auth0 Bearer token held in the browser, and raw token material was
deliberately not extracted into the shell. This is a "hold pending user live
verification," not a defect hold.

## Shipcheck legs
| Leg | Result |
|---|---|
| verify | PASS |
| validate-narrative | PASS (7/7 commands resolve + full examples) |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | HOLD — 91/100 Grade A; only live_api_verification unverified |

## Scorecard highlights (91/100, Grade A)
- Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native,
  MCP Quality, MCP Remote Transport, Local Cache, Workflows: 10/10
- Auth Protocol 10/10, Path Validity 10/10, Sync Correctness 10/10, Type Fidelity 5/5
- Weak: Insight 6/10, Cache Freshness 5/10, Data Pipeline Integrity 7/10 (polish targets)

## What was built
- **Generated read surface:** `cities` / `wework-yardi list-cities`, `spaces search-desks`,
  `spaces list-locations`, `wework-yardi list-locations-by-geo`, `wework-yardi list-amenities`,
  `common-booking list-bookings`, plus framework `search`, `sync`, `workflow`, `doctor`, etc.
- **Composed auth (generator-wired, no hand-editing):** `Authorization: Bearer` +
  `weworkuuid` + `weworkmembertype`, from `WEWORK_TOKEN` / `WEWORK_UUID` / `WEWORK_MEMBER_TYPE`.
- **Hand-coded novel command `desks`:** resolves a city name to its market geo, derives the
  bounding box the desk-search API requires, calls get-spaces, then supports `--sort credits|price`,
  `--available-only`, and `--limit`. This is the headline city-name desk finder.
- **Friendly top-level aliases:** `cities`, `bookings`.
- **book / cancel:** present under `common-booking` with explicit "real charge / unverified endpoint"
  warnings. Endpoint paths (`reserveworkspace`, `user-booking-cancellation`) were inferred from the
  app JS bundle + confirm modal, NOT verified against a live booking (no charge was ever placed).

## Live verification status
The underlying READ endpoints were live-verified during discovery against the real
authenticated session:
- `get-affiliate-cities` → 842 cities with market geo (200)
- `get-spaces` (Austin, Aug 18) → real bookable desks with credits/price/seat availability (200)
- `upcoming-bookings` → valid envelope (200, empty for this account)
The compiled binary was not run against live auth in-session (token not extracted).
User can complete live verification by exporting a fresh token from the browser.

## Dep fix applied
- Bumped `golang.org/x/text` v0.38.0 → v0.39.0 to clear GO-2026-5970 (generator's govulncheck gate).

## Remaining work (next-session / polish)
1. Live-verify the compiled binary with a fresh browser token (moves scorecard off HOLD).
2. Verify/repair book & cancel payloads against a real (refundable) booking.
3. Polish targets: Insight (6/10), Cache Freshness (5/10), Data Pipeline Integrity (7/10).

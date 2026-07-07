# Sutra Fitness Partner API CLI Brief

## API Identity
- **Domain:** Boutique fitness / wellness studio management ("studio OS"). Sutra Fitness, Inc. is the legal entity behind the product now branded **Arketa** (`sutra.fit` → `arketa.co`; help at `sutrapro.com`). The Partner API is still served under `sutra-prod` infra and documented under the `sutrafitness` GitHub org. Same company; product = Arketa, API = "Sutra Partner API."
- **Users:** Studio owners and front-desk staff who run class-based fitness businesses on the platform and want programmatic access to *their own* data.
- **Base URL:** `https://us-central1-sutra-prod.cloudfunctions.net/partnerApi/v0` (Firebase Cloud Functions, single region, production-only, no sandbox).
- **Auth:** API key — `X-API-Key: {key}` header (raw) OR `Authorization: Bearer {key}`. Every path is scoped under `/{partnerId}/`, so a partner ID is required for all calls (config value, not a secret).
- **Rate limit:** 25 req/sec (per Arketa help doc).
- **Pagination:** cursor-based — `limit` (default 10, max 100) + `start_after` (ID cursor); responses carry `pagination.nextStartAfterId` + `hasMore`.
- **Incremental sync:** every list endpoint accepts `updated_at_min`/`updated_at_max` (ISO 8601) and `ids` (comma-separated batch). Classes additionally accept `start_date`/`end_date` + `location_id`.
- **Data profile:** 7 related entities — Location, Room (with spot_map), Class, Client, Purchase, Referral, Reservation. Highly relational; ideal for a local store.
- **Support posture:** "provided as-is, no API support," v0.1.0 (pre-1.0; breaking changes plausible).

## Reachability Risk
- **None (current).** Live spec returns 200; base endpoint returns a clean documented `401 UNAUTHORIZED` envelope without a key (probed). No GitHub issues, deprecation notices, or 403/broken reports exist anywhere — the API has essentially zero public community footprint.
- Residual *operational* risk only: single Firebase region, "as-is" no support, pre-1.0 versioning.
- Probe-safe endpoint used: `GET /{partnerId}/locations` → 401 (expected, no key provided per user choice).

## Endpoint Inventory (12 operations, the full surface)
| Path | Method | Op | R/W |
|---|---|---|---|
| `/{partnerId}/locations` | GET | List locations | R |
| `/{partnerId}/locations/{locationId}/rooms` | GET | List rooms | R |
| `/{partnerId}/classes` | GET | List classes (`location_id`,`start_date`,`end_date`) | R |
| `/{partnerId}/classes/{classId}` | GET | Get class | R |
| `/{partnerId}/classes/{classId}/reservations` | GET | List class reservations | R |
| `/{partnerId}/classes/{classId}/reservations` | POST | Create reservation (existing/new client) | **W** |
| `/{partnerId}/classes/{classId}/reservations/{reservationId}` | PUT | Cancel reservation (late-cancel/refund flags) | **W** |
| `/{partnerId}/classes/{classId}/reservations/{reservationId}/check-in` | POST | Check in reservation | **W** |
| `/{partnerId}/clients` | GET | List clients | R |
| `/{partnerId}/clients/{clientId}` | GET | Get client | R |
| `/{partnerId}/purchases` | GET | List purchases | R |
| `/{partnerId}/referrals` | GET | List referrals | R |

**Critical: the API has NO reporting/aggregation endpoints. It is pure CRUD-list.** Every metric a studio owner wants is local-join-only.

## Top Workflows
1. **`sync` → local SQLite mirror.** Pull all 6 list resources incrementally via `updated_at_min`/`start_after` cursors. The operator owns a full, queryable copy offline. Answers the #1 industry pain (data ownership/export) and unlocks all analytics. No competitor offers this.
2. **Roster + check-in at the door.** `roster <class>` (booked clients + spot names), `check-in <reservation>`, `book`, `cancel` — the daily front-desk loop. Exercises the 3 write endpoints.
3. **Client export for marketing.** `clients export --csv` filtered (active / lapsed / expiring / by location), respecting `removed`. The export competitors make painful and "cagey."
4. **Expiring / low-balance watchlist.** `expiring --within 7d`, `--low-credits` over `purchases.end_date`+`remaining_uses`+`status`. Turns "which memberships expire this week" into one command.
5. **Capacity / fill scan.** `utilization --start --end` computing `total_booked/max_capacity` per class/instructor/time-slot — the buried metric, surfaced.

## Table Stakes (synthesized from Mindbody / Momence / Pike13 / Glofox)
- Class schedule pull (date-range, by location)
- Roster export (who's booked for a class/day)
- Client list export (CSV, for email/SMS) — #1 export request
- Attendance / check-in (mark attended; pull history)
- No-show tracking (usually buried in vendor "Attendance" report)
- Revenue / purchase reporting (by date/type/location; prior-period comparison is the explicit Capterra ask)
- Membership/plan status (active, expiring, credits remaining)
- Capacity / utilization (booked vs max — data exists, rarely surfaced cleanly)

## Data Layer
- **Primary entities:** locations, rooms (with embedded spots), classes, clients, purchases, referrals, reservations.
- **Sync cursor:** `updated_at` per entity → drive `updated_at_min`; classes also windowed by `start_date`/`end_date`.
- **FTS/search:** clients (name/email/phone), classes (name/instructor/description), locations (name/address).
- **Load-bearing fields for analytics:** `reservations.status` (BOOKED|CANCELED|CHECKED_IN|**NO_SHOW**), `reservations.checked_in`, `classes.instructor_name`, `classes.max_capacity`/`total_booked`, `classes.start_time`, `purchases.type`/`status`/`end_date`/`remaining_uses`/`price`, `clients.created_at`/`removed`, `referrals.referred_user_id`/`referring_user_id`.

## Codebase Intelligence
- No SDK, CLI, MCP, wrapper, or example code exists for this API anywhere (GitHub `sutrafitness` org has only unrelated boilerplate repos). This CLI would be the first and only tool of any kind. Greenfield.

## Competitive Landscape
- **No studio-OS vendor ships a CLI.** Mindbody has the most mature *library* ecosystem (TS/Python/Ruby wrappers) but no CLI; reporting is "canned-only" CSV/PDF. Momence has the richest competitor API (sessions, memberships, check-in, async Reporting API). Pike13 has a separate Reporting API. Walla has no public API (Zapier only).
- **Differentiator:** we ship the one thing none of them do — a local-first, offline-queryable, agent-native data mirror with reports the platforms bury.

## Product Thesis
- **Name:** `sutra-fitness` (binary `sutra-fitness-pp-cli`); prose display "Sutra".
- **Why it should exist:** The Sutra/Arketa Partner API is pure CRUD with zero analytics. Studio operators are stuck with canned vendor reports they can't customize and a client list the vendor is "cagey" about exporting. This CLI syncs your studio's full dataset into a local SQLite database you own, then answers the questions the dashboard buries — no-show rate per instructor, capacity utilization across the schedule, memberships expiring this week, membership churn, referral-funnel conversion, client LTV, revenue by type with prior-period comparison — all offline, all scriptable, all agent-native. Plus the daily front-desk loop (roster, book, cancel, check-in) in one composable binary.

## Build Priorities
1. **Data layer + sync** for all 7 entities with cursor pagination + incremental `updated_at` sync. This is the foundation that makes everything else possible.
2. **Absorbed table-stakes:** list/get every resource, schedule pull, roster, client/purchase export (CSV/JSON/--select), the 3 write ops (book/cancel/check-in), search.
3. **Transcendence analytics** (local joins, none available from any single endpoint): instructor scorecard, no-show rate, capacity utilization, expiring/low-balance watchlist, churn/at-risk, client LTV, revenue-by-type with prior-period comparison, referral funnel, first-visit follow-up.

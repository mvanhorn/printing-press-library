# Plan Clone / Fill Implementation Notes

## User Priority

The primary workflow is filling a new Wanderlog plan from a shared/public plan URL such as:

`https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared`

This must supersede lower-value local-analysis features if scope has to be trimmed.

## Verified Source Read Shape

`GET /api/tripPlans/naertjcoixqrgrfc?clientSchemaVersion=2` returns `success:true` and a full `tripPlan` payload. The Morocco sample has:

- title: `Morocco Travel Guide`
- key: `naertjcoixqrgrfc`
- itinerary sections: 8
- day dates: none (this plan carries no start or end date)
- total blocks in this sample: 9
- resources include `geo`, `geos`, `placeMetadata`, `distancesBetweenPlaces`, `currencyRatesUSD`, `hotelDeals`, `sectionRecommendations`, and related travel resources.

Even with this sparse block list, this proves the plan-template copy surface: title, dates, section layout, section modes, headings, colors/icons, notes, budget, and journal skeleton.

## Commands

### `plan preview`

Read-only command. Inputs: `--source-url` or `--source-key`, optional `--client-schema-version` default 2. Outputs title, key, start/end dates, section count, day dates, block counts by section, place count, note/checklist/hotel/expense counts when present, and clone warnings.

### `plan clone`

Creates a new target trip and fills it from a shared/public source plan.

Inputs:

- `--source-url` or `--source-key`
- `--destination` optional override; otherwise use source `resources.geos[0]` / `resources.geo`
- `--start-date` optional override; otherwise use source start date or first dayPlan date
- `--end-date` optional override; otherwise use source end date or last dayPlan date
- `--title` optional override; otherwise use source title plus optional suffix
- `--privacy` default `private`
- `--dry-run` to preview without mutation
- `--apply` to actually create/fill the trip

Flow:

1. Fetch source via `/api/tripPlans/{key}?clientSchemaVersion=2` without auth.
2. Determine destination geo id from source resources or geo autocomplete fallback.
3. Create target trip via `POST /api/tripPlans` using `WANDERLOG_COOKIE`.
4. Fetch target trip with `/api/tripPlans/{targetKey}?clientSchemaVersion=2&registerView=true`.
5. Connect ShareDB websocket: `wss://wanderlog.com/api/tripPlans/wsOverall/{targetKey}?clientSchemaVersion=2` with `Cookie`, `Origin`, and `User-Agent` headers.
6. Subscribe to `TripPlans/{targetKey}` and submit JSON0 ops.
7. Fill sections/itinerary from sanitized source snapshot.

### `plan fill`

Fills an existing target trip from a source plan. Inputs: source URL/key, `--target-key`, `--mode replace-sections|append-missing-days` default `replace-sections`, `--dry-run`, `--apply`, and `--force` for destructive replacement when the target already has non-empty sections.

## Mutation Evidence

The public MCP source proves these wire details:

- ShareDB URL: `/api/tripPlans/wsOverall/{tripKey}?clientSchemaVersion=2`
- Handshake frame: `{a:"hs", id:null, protocol:1, protocolMinor:2}`
- Subscribe frame: `{a:"s", c:"TripPlans", d:tripKey}`
- Operation frame: `{a:"op", c:"TripPlans", d:tripKey, v, seq, x:{}, op:[...]}`
- Rate-limit rejection can arrive as code `4001`; retrying the same op after backoff is safe because it was rejected before apply.
- Existing tools insert blocks with JSON0 `li` ops under `['itinerary','sections',sectionIndex,'blocks',insertIndex]`.
- Date updates are built from `li`/`ld` section ops plus top-level `startDate`, `endDate`, and `days` `od`/`oi` replacements.

## Clone Safety Rules

- Default to `--dry-run`; require `--apply` for real writes.
- Require `WANDERLOG_COOKIE` for create/fill.
- Refuse `plan fill --mode replace-sections` when target sections contain blocks unless `--force` is passed.
- Regenerate section and block IDs when copying into a target trip.
- Preserve source place payloads, text rich-text ops, headings, section modes, colors, icons, dates, budget, and journal when possible.
- For copied blocks with `addedBy`, rewrite `addedBy.userId` to the authenticated user id when available.
- Do not copy source trip key, author, comments, likes, distinction, or view/social metadata into the target trip.
- Emit a structured clone report: source key, target key, sections copied, day sections copied, blocks copied, warnings, and skipped fields.

## Shipping Priority

Build `plan preview`, `plan clone --dry-run`, and `plan fill --dry-run` first. Then implement real `--apply` behind cookie auth and ShareDB. If a verified live cookie/fixture is unavailable, dogfood must clearly report that apply-mode live verification was skipped; the dry-run/planning surface still ships as verified behavior.

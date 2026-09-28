# Jalan CLI

**Find Japan stays and understand the exact room and plan.**

Read-only public accommodation discovery with source evidence, explicit price units and canonical booking links.

## Authentication

Public accommodation commands need no credentials or paid service. Legacy API registration is closed and the credential-gated API is excluded.

## Build locally

Requires Go 1.26.6 or newer. From this checkout:

```bash
go build -o bin/jalan-pp-cli ./cmd/jalan-pp-cli
./bin/jalan-pp-cli stay --help
```

Use `./bin/jalan-pp-cli` directly. Examples written as `jalan-pp-cli` assume this local binary directory is already on PATH. Source builds require no global installation or shared configuration changes.

## Quick Start

```bash
# Check the local command setup.
jalan-pp-cli doctor --dry-run

# Resolve the source region.
jalan-pp-cli stay locations --query Hakone

# Fetch a small fresh shortlist.
jalan-pp-cli stay search --destination Hakone --check-in 2026-11-10 --adults 2 --limit 3

```

## Agent Usage

Accommodation commands return compact JSON by default. Results live in `.results`; shared `.meta` describes the query, observation time, source, cache state and request count. `.pagination` reports coverage and continuation. Missing facts remain unknown. Diagnostics use stderr.

```bash
./bin/jalan-pp-cli stay search --destination Hakone --check-in 2026-11-10 --limit 3 --agent --select id,name_ja,url,price
./bin/jalan-pp-cli stay offers 385995 --check-in 2026-11-10 --adults 2 --limit 3
./bin/jalan-pp-cli stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2
```

Property, plan and room IDs are separate strings; retain leading zeroes. An offer identifies all three. Search summaries do not contain every room/plan fact: retrieve `stay plan` for cancellation, fees and restrictions. Evidence retains short Japanese source text; `source_ref` resolves through response-level `.meta.sources` to its source URL.

Bath facts distinguish in-room baths, outdoor baths, private-use/reservable facilities, and hot-spring water. A property's onsen does not establish that its room bath contains hot-spring water. Read the evidence and explicit negative statements.

Prices retain their source basis. The base quote is separate from conditional coupons and earned points. Extra taxes/fees may be payable locally; a quote with unresolved extras is not a final checkout total. Compare only equal occupancy, stay length, currency and price basis.

## Cookbook

Inspect facilities and access before retrieving offers:

```bash
./bin/jalan-pp-cli stay property 371898
./bin/jalan-pp-cli stay offers 371898 --check-in 2026-11-10 --adults 2 --limit 3
```

Search two rooms with the same family in each room:

```bash
./bin/jalan-pp-cli stay search --destination Hakone --check-in 2026-11-10 --rooms 2 --adults 2 --children-elementary 1 --limit 3
```

The other per-room child flags are `--children-meals-bed`, `--children-meals`, `--children-bed`, and `--children-neither` for infant meal/bedding categories.

Compare a small set of dates under the same party:

```bash
./bin/jalan-pp-cli stay compare 385995 --dates 2026-11-10,2026-11-11 --adults 2 --limit 3
```

Compare exact plan/room pairs on one date:

```bash
./bin/jalan-pp-cli stay compare 385995 --check-in 2026-11-10 --plans 03912759:0576806,03806855:0546600 --adults 2
```

For compact comparisons, select fields inside each offer with `--select check_in,results.property_id,results.plan_id,results.room_id,results.price`. Keep the full `price` object to retain currency, basis, occupancy, taxes and discount conditions. Alternative identity, query, freshness, coverage and failures remain available.

These comparisons cover fetched offers, not every possible offer or a guaranteed cheapest stay. Failed alternatives remain visible and never become zero-price rows. Partial results keep exit code 8; explicit file/webhook sinks still receive the complete response. MCP preserves the bounded partial payload and marks the tool result as an error. Booking handoff URLs lead to Jalan, where current terms and availability must be checked.

## Freshness and bounds

Inventory defaults to fresh requests. `--max-age 2m` explicitly permits cache reuse; `--refresh` forces a new observation. A cache hit retains its original observation timestamp. The CLI flag `--cache-dir` isolates cached public observations. MCP tools use the server-configured cache location; callers cannot override it. No stale cache fallback is presented as live availability. Fresh requests are separate observations and may reorder results. For repeatable slices within one native source page, use `--max-age 5m` with the same query; there is no atomic snapshot across native pages.

Exact adult counts are 1–8 per room; Jalan’s 9-or-more bucket is rejected. Each child category is 0–5, rooms 1–10, and nights 1–9. The 365-day date window is a CLI bound, not a promise of released inventory.

Defaults and hard limits are discoverable in `stay --help` and each command's help. Search/detail retrieval is separate; result limits, pagination, comparison width, request bodies, retries and total time are bounded. `--timeout` limits the whole command. Field selection reduces stdout without dropping shared freshness or coverage metadata.

## Health Check

```bash
./bin/jalan-pp-cli doctor --dry-run
./bin/jalan-pp-cli stay capabilities
./bin/jalan-pp-cli stay search --help
```

`--dry-run` checks command shape without fetching inventory. It adds top-level `dry_run` and `action` markers while retaining the response envelope. A successful health check alone does not verify a dated offer.

## Troubleshooting

- Unknown destination: use `stay locations --query ...` or a source area code. English aliases are explicit; arbitrary translation is unsupported.
- No matching inventory: inspect dates, party, filters and coverage. An empty bounded result is not evidence that an entire destination is sold out.
- Access/rate-limit/timeout/parse error: inspect stderr and retry later or inspect the canonical Jalan URL. These failures are distinct from no matches.
- Missing detail: inspect the evidence and source URL. An absent attribute is unknown, not false.

## Coverage and verification

The implementation targets public Japanese accommodation pages, including ryokan, onsen stays and hotels. It preserves child meal/bedding categories with identical occupancy per room. Heterogeneous room parties, booking/payment/cancellation actions, coupon redemption, account benefits and other travel providers are outside scope.

The [legacy Jalan API](https://www.jalan.net/jw/jwp0000/jww0001.do) requires an existing key; new registration closed on 2020-02-25. Its documentation being online does not prove credentialed operation. It is excluded from this CLI. The newer Jalan-hosted Korean-widget MCP search is also excluded because its verified schema lacks room count, children and pagination.

See [verification evidence](docs/verification.md) for live cases, deterministic-only cases, efficiency measurements and remaining limits. Source markup can change; parsing failures should be reported explicitly rather than silently producing empty data.

## Development

```bash
go test -count=1 ./...
go vet ./...
```

Built with [Printing Press](https://github.com/mvanhorn/cli-printing-press). [ngs/yadosearch-api](https://github.com/ngs/yadosearch-api) informed the separate property/plan model and legacy API limitations; runtime data comes directly from Jalan. Catalog installation becomes available after the publication PR merges and the library registry updates.

## Unique Features

### Accommodation decisions
- **`stay plan`** — Inspect separate bath facts without inferring room facilities from property amenities.


  ```bash
  jalan-pp-cli stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2 --agent
  ```
- **`stay plan`** — Separate quoted cash price, conditional coupons, earned points and extra fees.


  ```bash
  jalan-pp-cli stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2 --agent
  ```
- **`stay compare`** — Compare a few dates or exact plans under equal party conditions.


  ```bash
  jalan-pp-cli stay compare 385995 --dates 2026-11-10,2026-11-11 --adults 2 --limit 2 --agent
  ```
- **`stay property`** — Preserve Japanese facts, explicit unknowns and source evidence.


  ```bash
  jalan-pp-cli stay property 371898 --agent
  ```
- **`stay locations`** — Resolve supported Japanese and English destination aliases explicitly.


  ```bash
  jalan-pp-cli stay locations --query Hakone --agent
  ```

## Recipes

### Bounded shortlist

```bash
jalan-pp-cli stay search --destination Hakone --check-in 2026-11-10 --adults 2 --limit 3 --agent --select id,name_ja,url,price
```

Find a small dated shortlist while retaining source and coverage metadata.

### Exact offer terms

```bash
jalan-pp-cli stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2
```

Inspect this plan/room pair with Japanese source evidence and explicit price scope.

### Date alternatives

```bash
jalan-pp-cli stay compare 385995 --dates 2026-11-10,2026-11-11 --adults 2 --limit 3
```

Compare only the bounded offers retrieved for the same party.

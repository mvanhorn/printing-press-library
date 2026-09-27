# Architecture contract — approved public Jalan CLI

Root owns this contract, scope, research and acceptance. Workers may refine internals within ownership but coordinate exported changes directly with root. All implementation targets the staged working/jalan-pp-cli module, never workspace root or shared config.

## Ownership
- Parser worker: `internal/jalan/model.go`, `parse*.go`, `internal/jalan/testdata/` and parser tests. Japanese HTML → facts only. No network.
- Service worker: `internal/jalan/client*.go`, `query*.go`, `cache*.go`, `locations*.go` and corresponding tests. HTTP, cache, request budget, query validation, response envelopes and comparisons. No parser edits.
- CLI worker: `internal/cli/stay*.go`, minimal registration/help changes in `root.go`, CLI tests, `scripts/live-e2e.*`, `workflow_verify.yaml`. Commands and observable E2E tests. No parser/client edits. Root writes README/SKILL and acceptance proof.

## Shared parser API (parser worker implements first)
Package `internal/jalan`. Input is decoded UTF-8 HTML. Export:
- `ParseSearch(doc, sourceURL string) (SearchPage, error)`
- `ParseOffers(doc, sourceURL, propertyID string) (OfferPage, error)`
- `ParseProperty(doc, sourceURL, propertyID string) (Property, error)`
- `ParsePlan(doc, sourceURL, propertyID, planID, roomID string) (Plan, error)`

Types (additional fields allowed; preserve these names/types):
- `Evidence {Field string; Text string; URL string}`
- `BathFacts {InRoom *bool; Outdoor *bool; PrivateReservable *bool; HotSpring *bool; Evidence []Evidence}`
- `Price {Amount *int64; Currency string; Basis string; TaxInclusion string; ServiceChargeInclusion string; ExtraFees string; ConditionalDiscount string; Points string; Evidence []Evidence}`. Amount = observed base quote, never coupon-reduced or points-adjusted. Basis explicit `per_person_per_night`, `per_room_per_night`, `whole_stay`, `unknown`. Final payable remains unknown when extras unresolved; avoid computing speculative fee totals.
- `Property {ID string; NameJa string; URL string; LodgingType string; Address string; Access string; Description string; Amenities []string; Baths BathFacts; ReviewCategories map[string]any; Price Price; Evidence []Evidence}`. Unknown scalars should serialize null (pointers or explicit custom marshal if needed); Japanese labels retained in reviews.
- `Offer {PropertyID string; PlanID string; RoomID string; PlanName string; RoomName string; URL string; Meals string; Smoking string; Baths BathFacts; Price Price; Availability string; Evidence []Evidence}`. Go fields string; JSON unknown represented explicitly using "unknown" for enums. Source-linked room names may be unknown in listing until plan detail.
- `Plan {Offer embedded; CheckIn string; CheckOut string; Cancellation string; Fees string; Payment string; BookingDeadline string; RoomDescription string; Description string; OccupancyText string}`. Raw Japanese restrictions retained. No inferred room hot spring from a property onsen label.
- `SearchPage {Items []Property; Total *int; HasNext bool; NoResults bool; Warnings []string}`
- `OfferPage {Items []Offer; Total *int; HasNext bool; NoResults bool; Warnings []string}`
All collection fields initialize empty arrays/maps. Detect actual source empty/no-inventory messages. Wrong-page, parse failure, unavailable detail are errors, not empty success. Ordinary zero search matches mean `no_matches`, never broad `sold_out`. Preserve duplicate room/plan relationships, de-duplicate only exact property/plan/room tuple.

## Service API (service worker implements)
Same package. Export:
- `Query {Destination string; AreaCode string; CheckIn string; Nights int; Rooms int; Adults int; Children [5]int; Meals string; LodgingType string; Onsen bool; OutdoorBath bool; PrivateBath bool; RoomOutdoorBath bool; NonSmoking bool; Page int; Limit int}`
- `PlanRef {PlanID string; RoomID string}`
- `Options {Timeout time.Duration; MaxAge time.Duration; Refresh bool; CacheDir string}`
- `NewClient(options Options) *Client`
- `(*Client).Search(ctx context.Context, q Query) (Response,error)`
- `(*Client).Property(ctx context.Context, propertyID string) (Response,error)`
- `(*Client).Offers(ctx context.Context, propertyID string, q Query) (Response,error)`
- `(*Client).Plan(ctx context.Context, propertyID, planID, roomID string, q Query) (Response,error)`
- `(*Client).Compare(ctx context.Context, propertyID string, q Query, dates []string, plans []PlanRef) (Response,error)`
- `Locations(query string) (Response,error)` and `Capabilities() Response`.
- `Response {Meta map[string]any; Results []any; Pagination map[string]any; FetchFailures []map[string]any}` with JSON snake_case. Page metadata includes page, requested limit, source page size/returned count/total when known, has_more and next action. Meta carries source, URL(s), observed_at, timezone Asia/Tokyo, query, cache status/age, upstream_requests, elapsed_ms, coverage/status/warnings. Keep meta across field selection.

Search/Offers q defaults: 5 results (max 30), page 1. Source page size 30 (verify). Paginate without skipping unreturned items: map logical CLI page/limit into source offset or use opaque continuation cursor if upstream offset rounds (coordinate with CLI worker). Each invocation bounded max 2 source pages. Date required for inventory; undated search only if explicit support (otherwise actionable usage error). Nights 1..9, rooms/adults 1..10 with source limits validated. Children five categories: elementary, infant meals+bed, infant meals, infant bed, infant neither. Same per-room occupancy only; verify `roomCrack` source encoding. Calendar validate in Asia/Tokyo; past dates fail. Max five date/plan alternatives; serial or concurrency<=2; request timeout <=20s and command timeout <=60s. At most one retry on transient failures, honor Retry-After within command budget. Use generated `cliutil.AdaptiveLimiter` and typed rate-limit errors (skill requirement). Limit response bodies (e.g. 4MiB).

Cache is URL/query exact and versioned: inventory fresh by default MaxAge=0, opt-in reuse up to 5min; `--refresh` forces network. Property detail may use explicit max-age too, keeping zero default simplest. Preserve original observed_at for cache hits. Cache public parsed observations or HTTP body only, never headers/cookies. Set private permissions. Normal command writes only its cache directory, no global tool config.

Compare: dates searches property Offers once each and summarizes bounded observed offers, labeling not exhaustive/no cheapest guarantee. Exact plans mode calls Plan per supplied pair under one date. Failures retained with nonzero partial indicator; never rank unknown price as zero. Sort only comparable price basis; no speculative multiplication. Partial response returns data + failures and CLI distinct nonzero partial exit if feasible. No reservation/coupon claim/launch actions.

Locations: explicit curated Japanese+English aliases + source area IDs and canonical URLs; source-linked. At least Hakone, Tokyo, Kyoto, Beppu, Kusatsu, Kinosaki, Yufuin where source IDs verified. Support direct `--area-code` for other large areas. Never equate English keyword matches with translated area search. Ambiguous/unknown destination fails with next action.

## CLI commands
`stay search --destination Hakone --check-in 2026-11-10 --nights 1 --adults 2 --rooms 1 --limit 5`
`stay locations --query Hakone`
`stay property 385995`
`stay offers 385995 --check-in 2026-11-10 --adults 2`
`stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2`
`stay compare 385995 --dates 2026-11-10,2026-11-11 --adults 2 --limit 3`
`stay compare 385995 --check-in 2026-11-10 --plans 03912759:0576806,03806855:0546600 --adults 2`
`stay capabilities`

Domain commands output compact JSON by default (also support generated --json/--agent flags); single `{meta,results,pagination,fetch_failures}` envelope. `--select` selects item dotted fields but keeps shared correctness/freshness meta. Ensure JSON escapes no HTML unnecessarily. Shared child flags descriptive, avoid ambiguous --children. Support --max-age duration, --refresh, --cache-dir override. Errors structured on stderr, stdout empty for total failures; partial results stdout + diagnostic stderr. Distinct usage/unsupported, access, rate limit, parse failure and partial states. No raw HTML stdout. Honor root --timeout with boundCtx and no I/O under --dry-run. Source request bounds in help, realistic `pp:happy-args`, `mcp:read-only` and `pp:data-source=live` annotations. Keep generic generated utilities compilable; core help highlights stay group.

## Live evidence locations
Run discovery includes cp932 raw files and decoded UTF8 .html: dated, filtered, page2-http, property-385995, property-371898, offers-385995, plan, keyword-ja/en. Exact sampled IDs and plan URL are in plan-url.txt. Current source date queried 2026-11-10. Native names/filter contract in browser-filters.json. Do not commit full pages, personal reviews or advertising scripts; create focused sanitized fixtures. Root is still verifying multiple-room child query encoding and destination IDs and will send updates.

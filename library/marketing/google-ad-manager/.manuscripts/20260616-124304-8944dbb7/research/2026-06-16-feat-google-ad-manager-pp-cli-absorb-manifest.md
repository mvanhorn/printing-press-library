# Google Ad Manager CLI — Absorb Manifest

**Source spec:** Ad Manager REST v1 (Beta), official Google discovery doc → OpenAPI 3 (97 paths, 183 methods, 305 schemas, 49 resource groups).
**Competition:** No modern REST GAM CLI and no GAM MCP server exist. Closest tools are the GAM web UI, Google's official SOAP client libraries (`googleads-python-lib`), and one archived DFP-era SOAP CLI (`publica-project/dfp-api`). Most ecosystem CLI/MCP tooling is *Google Ads* (advertiser side) — wrong product.

## Absorbed — match/beat the full REST surface (generated endpoint commands)

Every REST resource group is emitted as typed endpoint commands (list/get + REST-supported writes), each gaining `--json`/`--select`/`--agent`, `--dry-run`, typed exit codes, and local-store persistence. Network code is supplied once (env `GOOGLE_AD_MANAGER_NETWORK_CODE` or root `--network`) and auto-built into the `networks/{code}/…` parent.

| # | Domain (resources) | Methods absorbed | Our Implementation | Added value over UI/SOAP-libs |
|---|---|---|---|---|
| 1 | **Inventory** — adUnits, adSpots, placements, sites, adUnitSizes, applications | list/get/create/patch/batch* | (generated endpoint) inventory resources | Offline mirror + FTS; hierarchy reassembly; no `networks/{code}` boilerplate |
| 2 | **Custom targeting** — customTargetingKeys, customTargetingValues | list/get/create/patch/batch* | (generated endpoint) targeting resources | Instant local key→value lookup; reverse index |
| 3 | **Targeting dimensions** — geoTargets, audienceSegments, browsers, browserLanguages, deviceCapabilities/Categories/Manufacturers, mobileDevices/Carriers/Submodels, operatingSystems(+Versions), bandwidthGroups, linkedDevices | get/list (read) | (generated endpoint) dimension lookups | Cached reference tables, searchable offline |
| 4 | **Orders & line items** — orders, lineItems | get/list (read-only in REST) | (generated endpoint) order/lineItem read | Cross-entity joins (order→lineItem→targeting→adUnit) |
| 5 | **Reporting** — reports, operations | create/get/list/patch/run, results:fetchRows, operations poll/cancel | (generated endpoint) report CRUD + (behavior in `report run`) async orchestration | One-shot async run→fetch; local row cache; offline re-query |
| 6 | **PMP** — privateAuctions, privateAuctionDeals, programmaticBuyers | create/get/list/patch (read+write) | (generated endpoint) PMP resources | Deal/buyer visibility in one place |
| 7 | **Content & CMS** — content, contentBundles, contentLabels, cmsMetadataKeys/Values, taxonomyCategories, entitySignalsMappings | get/list/batch* | (generated endpoint) content resources | Searchable offline catalog |
| 8 | **Org & reference** — companies, contacts, labels, teams, roles, users, customFields, creativeTemplates, richMediaAdsCompanies | get/list/create/patch/batch* | (generated endpoint) org resources | Resolve IDs→names locally; FTS |
| 9 | **Live streaming** — liveStreamEvents(+ByAssetKey/ByCustomAssetKey).adBreaks, webProperties.adReviewCenterAds | create/get/list/patch/delete/search/batch | (generated endpoint) live-stream + ad-review | Scriptable ad-break + ad-review ops |
| 10 | **Earnings** — mcmEarnings | fetch | (generated endpoint) mcmEarnings fetch | MCM earnings pull without UI |
| 11 | **Network** — networks | get/list | (generated endpoint) networks list/get | Discover accessible network codes for auth setup |

*batch\* = the resource's `batchCreate`/`batchUpdate`/`batchActivate`/`batchDeactivate`/`batchArchive` variants where the REST API offers them.*

**Framework features absorbed automatically (every printed CLI):** offline `sync`, FTS `search`, `sql`, `doctor`, `--json`/`--select`/`--compact`/`--csv`/`--agent`, `--dry-run`, typed exit codes, MCP server mirror.

## Transcendence — build what nobody else has (all hand-code)

Minimum 5 required; 11 shipping (all scored ≥6/10 by the novel-features pass). All `hand-code` — the generator emits none of these.

| # | Feature | Command | Buildability | Score | Why only we can do this | Long Description |
|---|---|---|---|---|---|---|
| 1 | One-Shot Async Report | `report run` | hand-code | 10 | Chains create→run→operation-poll→paginated fetch into one call; raw API forces 4 round-trips, UI makes you babysit a spinner | Use for actual GAM revenue/delivery numbers. Builds, runs, polls, and fetches in one step. |
| 2 | Saved Report Rerun | `report rerun` | hand-code | 9 | Re-fetches a stored definition then re-runs the full async cycle; API re-runs but you re-poll/re-paginate by hand | Use when the user names a standing report they run repeatedly. |
| 3 | Daily Report Diff | `report watch` | hand-code | 9 | Retains prior run rows in SQLite and diffs offline; neither API nor UI keeps your last result | Use for "what changed/dropped overnight" without eyeballing two exports. |
| 4 | Cross-Entity Offline Search | `search` | hand-code | 9 | Local FTS spans all entity types at once; REST forces per-resource list, UI has no global search | Use as the first move on a fuzzy name/partial ID before you know the entity type. Do NOT use for live-only data; run `sync` first. |
| 5 | Order Expansion Graph | `order graph` | hand-code | 8 | Joins orders+lineItems+targeting+adUnits in one local pass; API needs many calls across 4 resources | Use to hand an agent the full "what this campaign touches" object. |
| 6 | Targeting Reverse Lookup | `targeting where` | hand-code | 8 | Reverse index over mirrored line-item targeting; API has no who-targets-this endpoint, UI can't answer it | Use to assess blast radius before changing/retiring a targeting value. |
| 7 | Ad-Unit Tree View | `adunits tree` | hand-code | 7 | Reassembles parent/child hierarchy locally in one render; API returns flat pages needing client stitching | Use to orient in the inventory hierarchy before reasoning about placements/subtree revenue. |
| 8 | Inventory Orphan Audit | `inventory orphans` | hand-code | 7 | Offline join of the ad-unit tree against placements/sites in one pass; impossible in a single API call | Use during inventory cleanups to list misconfigured/dead units. |
| 9 | Unused Targeting Sweep | `targeting unused` | hand-code | 6 | Set difference between all keys/values and everything referenced in targeting; API exposes no usage count | Use when auditing taxonomy bloat or prepping a safe key/value retirement. |
| 10 | Local Resync Diff | `since` | hand-code | 6 | Diffs two local snapshots; GAM exposes no per-entity change feed | Use to catch what another team changed between two points in time. |
| 11 | Line-Item Pacing Join | `lineitem pace` | hand-code | 6 | Fuses read-only line-item goals with async delivery-report rows in the local store; API keeps them in separate subsystems | Use to triage which campaigns are at delivery risk without the delivery UI. |

**Hand-code commitment:** 11 features (each ~50–150 LoC + `root.go` wiring). Generated endpoint surface (domains 1–11 above) is auto-emitted.

## Stubs / explicit exclusions
- **No stubs.** Everything listed ships fully.
- **Out of scope (SOAP-only, user-confirmed):** forecasting/availability, line-item/order trafficking *writes*, creative creation + LineItemCreativeAssociation. Not listed above; not stubbed; documented as a boundary in README/SKILL.

## Risks to flag at the gate
- **Beta API:** REST v1 surface/shapes can shift; reprint picks up parity later.
- **OAuth2 setup:** needs a Google access token (`GOOGLE_AD_MANAGER_ACCESS_TOKEN`) + network code + Cloud project with the API enabled. Tokens expire ~hourly.
- **Network-code ergonomics** (domains use `networks/{code}/…` parents) is hand-wired in Phase 3 so users never type raw parents.

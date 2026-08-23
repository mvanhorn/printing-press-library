# Absorb Manifest

## Sources

- `shaikhspeare/wanderlog-mcp` (`wanderlog-mcp` npm v0.3.1): MCP server and TypeScript source proving Wanderlog cookie auth, public guide/place APIs, trip list/get/create, and ShareDB mutation transport.
- `@zaw_ye/wanderlog_mcp`: secondary npm-discovered Wanderlog MCP package with overlapping MCP intent.
- `danilden1/Wanderlog-to-KML`: Python exporter from saved Wanderlog HTML to KML, including per-date splits.
- `devsuhh/wanderlog_importer`: Chrome extension importing Google Maps saved places into a Wanderlog trip section and auditing matched/missing/name-mismatch rows.
- Wanderlog web/browser-sniff capture: public destination, shared guide, place card, category-list, comments, distinction, and session endpoints.
- Public Morocco shared plan: `https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared`, verified via `/api/tripPlans/naertjcoixqrgrfc?clientSchemaVersion=2`.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search destinations/geos | Wanderlog web + wanderlog-mcp source | (generated endpoint) geos autocomplete | Scriptable geo resolution for later guide/place workflows |
| 2 | List guide-rich destinations | wanderlog-mcp source | (generated endpoint) geos good-guides | Helps agents find destinations with strong public guide coverage |
| 3 | Search public guides by destination | wanderlog-mcp search_guides | (generated endpoint) guides list-for-geo | Gives JSON output instead of MCP-only prose |
| 4 | Get public guide/shared plan by key | wanderlog-mcp get_guide + live probes | (generated endpoint) guides get | Fetches full tripPlan/resources with clientSchemaVersion=2, including the public Morocco plan |
| 5 | Get shared itinerary page | Wanderlog shared view browser-sniff | (generated endpoint) pages shared-view | HTML fallback for public shared views |
| 6 | Get guide comments | Browser-sniff capture | (generated endpoint) guides comments | Adds social sidecar metadata to guide exports |
| 7 | Get guide distinction metadata | Browser-sniff capture | (generated endpoint) guides distinction | Preserves public distinction badge/status data |
| 8 | Search places | wanderlog-mcp source | (generated endpoint) places autocomplete | Exposes Wanderlog place autocomplete request envelope |
| 9 | Get place details | wanderlog-mcp source + live probe | (generated endpoint) places details | Fetches structured place detail JSON |
| 10 | Get place card details | Browser-sniff capture | (generated endpoint) places card | Includes rich card data seen in web UI |
| 11 | Read destination category list | Browser-sniff capture | (generated endpoint) place-lists geo-category | Large top attractions/restaurants payload for mining |
| 12 | Read explore page | Wanderlog web/browser-sniff | (generated endpoint) pages explore | HTML fallback for destination discovery |
| 13 | Read category list page HTML | Wanderlog web/browser-sniff | (generated endpoint) pages geo-category-page | HTML fallback with links/metadata |
| 14 | Read anonymous session preferences | Browser-sniff capture | (generated endpoint) session get | Captures locale/currency/distance defaults |
| 15 | Authenticated current user | wanderlog-mcp source | (generated endpoint) account me | Cookie-backed account check via WANDERLOG_COOKIE |
| 16 | List personal trips | wanderlog-mcp list_trips | (generated endpoint) trips home | Cookie-backed trip inventory |
| 17 | Get personal trip | wanderlog-mcp get_trip | (generated endpoint) trips get | Cookie-backed full trip/resources read |
| 18 | Create trip | wanderlog-mcp create_trip | (generated endpoint) trips create | Cookie-backed POST needed by `plan clone` |
| 19 | Delete trip | wanderlog-mcp deleteTrip source | (generated endpoint) trips delete | Cookie-backed cleanup command for disposable fixture testing |
| 20 | Get trip URL | wanderlog-mcp get_trip_url | (behavior in wanderlog-pp-cli guides get/trips get) output source URL fields | Avoids a separate thin command |
| 21 | Preview copyable shared plan contents | User priority + Morocco probe | wanderlog-pp-cli plan preview | Shows sections, dates, blocks, resources, and clone warnings before mutation |
| 22 | Clone shared/public plan into a new trip | User priority + REST create + ShareDB source | wanderlog-pp-cli plan clone | Creates a new cookie-backed trip and fills it from the source plan template |
| 23 | Fill an existing trip from a shared/public plan | User priority + ShareDB source | wanderlog-pp-cli plan fill | Applies a source plan template to an existing target with dry-run/force safeguards |
| 24 | Add place to trip | wanderlog-mcp add_place | (behavior in wanderlog-pp-cli plan fill/clone) copied place blocks when present; standalone `trip edit add-place` remains stubbed | Most important place-writing need is covered by plan fill/clone first |
| 25 | Add note/edit note/remove note | wanderlog-mcp note tools | (behavior in wanderlog-pp-cli plan fill/clone) copied note/rich-text blocks when present; standalone `trip edit note` remains stubbed | Preserves copied plan notes without promising a general editor first |
| 26 | Add hotel/checklist/expense | wanderlog-mcp hotel/checklist/expense tools | (behavior in wanderlog-pp-cli plan fill/clone) copied supported blocks when present; standalone add commands remain stubbed | Keeps fill-new-plan priority ahead of individual mutators |
| 27 | Annotate/remove place/update trip dates/rename day | wanderlog-mcp mutation tools | (stub - standalone editing requires broader ShareDB mutation coverage) trip edit annotate/remove-place/update-dates/rename-day | Explicitly captures lower-priority mutation parity |
| 28 | Export saved Wanderlog HTML to KML | Wanderlog-to-KML | wanderlog-pp-cli export kml | Adds CLI-native JSON/KML/CSV path and agent output modes |
| 29 | Split KML by travel date | Wanderlog-to-KML | (behavior in wanderlog-pp-cli export kml) --split | Matches existing exporter while keeping combined output available |
| 30 | Import Google Maps saved places to Wanderlog section | wanderlog_importer | (stub - full import mutates Wanderlog through UI/ShareDB and needs authenticated fixture) import google-maps --apply | Reframed as parse/audit-first CLI workflow |
| 31 | Reconcile Google Maps export vs Wanderlog section | wanderlog_importer | wanderlog-pp-cli import google-maps --audit | Keeps the high-value audit without requiring a browser extension |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Shared plan clone/fill | `plan clone --source-url <url> --apply` | 10/10 | hand-code | Fetches a public/shared source plan via `/api/tripPlans/{key}?clientSchemaVersion=2`, creates a cookie-backed target trip via `POST /api/tripPlans`, then fills sections/blocks through ShareDB JSON0 ops with dry-run and apply modes. | User said this is most important; Morocco source plan verified; MCP source proves create-trip and ShareDB mutation transport. | Use this command to create a new Wanderlog trip from a shared/public plan. Do NOT use it to modify an existing trip; use `plan fill` instead. |
| 2 | Existing trip fill from template | `plan fill --source-url <url> --target-key <trip_key> --apply` | 9/10 | hand-code | Fetches the source plan and target trip, previews destructive changes, then applies sanitized section/block JSON0 operations to the target trip through ShareDB. | Directly serves the user's fill-new-plan workflow and covers source plans with places/notes when present; MCP source proves section/block mutation patterns. | Use this command to fill an existing Wanderlog target trip from a shared/public source. Do NOT use it to create a new target trip; use `plan clone` instead. |
| 3 | Clone preview and copyability report | `plan preview --source-url <url>` | 8/10 | hand-code | Reads the source plan and emits title, dates, section counts, block counts, resource coverage, and clone warnings without credentials or mutations. | Required safeguard for the public Morocco example and for dry-run dogfood when no authenticated fixture is available. | Use this command to inspect what a shared plan contains before cloning/filling. Do NOT use it to write a trip; use `plan clone` or `plan fill` with `--apply`. |
| 4 | Guide consensus map | `guides consensus --geo <geo_id>` | 8/10 | hand-code | Uses local SQLite rows synced from public guides, place details/card data, and geo category lists to count repeated place appearances and emit ranked places with no external dependencies. | Weekly use for destination research; cross-guide/category join; evidence from destination guide ritual, place research ritual, guide-rich destinations, public guides, category lists, Wanderlog-to-KML/importer reconciliation patterns. | Use this command to rank destination-wide public-guide consensus. Do NOT use it to copy a plan; use `plan clone` or `plan fill` instead. |
| 5 | Agent planning bundle | `plan bundle --geo <geo_id> --days <n>` | 7/10 | hand-code | Uses local geos, guide summaries, guide tripPlans, category lists, place details/card data, and consensus scores to emit compact deterministic planning JSON with no external dependencies. | Useful after source-plan cloning when an agent needs compact context; evidence from agentic workflow builder user and public guide mining resources. | none |
| 6 | Itinerary compare | `itinerary compare --left <guide_or_trip_key> --right <guide_or_trip_key>` | 7/10 | hand-code | Uses local trip plan/resource records from public guide reads and cookie-backed trip reads to compute deterministic day, section, place, note, and ordering diffs with no external dependencies. | Useful for verifying clone/fill output against the source; evidence from shared itinerary export ritual, authenticated personal trip ritual, MCP trip reads, shared view browser-sniff. | Use this command to compare two known Wanderlog itineraries. Do NOT use it for destination-wide recommendation mining; use `guides consensus` instead. |

## Lower-Priority Novel Candidates Trimmed Behind Clone/Fill

| Feature | Reason | Replacement / status |
|---|---|---|
| Trip load audit | Useful, but less important than copying/filling a plan. | Defer unless clone/fill lands cleanly. |
| Trip readiness audit | Useful for handoff, but secondary to making a target plan exist. | Defer unless clone/fill lands cleanly. |
| Place canonicalizer | Useful for imports/content mining, but not essential to plan fill. | Defer unless clone/fill lands cleanly. |
| Destination dossier | Vague all-in-one output with weaker weekly action than a compact planning bundle. | Killed. |
| Batch general trip editor | Requires broad ShareDB editor surface. | Keep narrow plan clone/fill first. |

## Explicit Stubs Requiring Approval

- Standalone broad trip editing commands outside plan clone/fill remain stubs: `trip edit annotate`, `trip edit remove-place`, `trip edit update-dates`, `trip edit rename-day`, and other arbitrary editor operations.
- `import google-maps --apply` remains deferred because full application mutates Wanderlog through UI/ShareDB behavior; the shippable value is parse/audit first.
- `plan clone --apply` and `plan fill --apply` are shipping scope, but live apply verification requires `WANDERLOG_COOKIE` and an approved disposable target. Without that, dry-run/preview behavior can be dogfooded and apply mode must be reported as credential-gated.

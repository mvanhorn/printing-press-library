# Mariana Tek CLI Absorb Manifest

## Source tools surveyed
| Tool | Type | Stars | Features absorbed |
|------|------|-------|-------------------|
| Mariana Tek Customer API v1.0.0 | Official OpenAPI 3.0.3 spec (81 endpoints) | — | every endpoint |
| `bigkraig/marianatek` | Go community client | low | ClassSessions, Locations, PaymentOptions, Reservations, Users (5 services, ~5 yrs old, Barry's-tenant default) |
| `Bitlancer/mariana_api-gem` | Ruby community gem | low | REST endpoint coverage |
| `@mariana-tek/anemone` | Official JS Embedded Apps SDK | — | not relevant (admin embedded apps, not consumer booking) |
| `bfitzsimmons/marianatek_movies` | Django demo | — | confirms public API works against demo tenant |
| Apify scraper `alizarin_refrigerator-owner/mariana-tek-api-boutique-fitness-studio-data` | Public studio-data scraper | — | confirms location/region endpoints are public + cacheable |
| MCP servers | — | — | **none exist for Mariana Tek — we'd ship the first** |

## Absorbed (match or beat everything that exists)

Every Customer API endpoint mapped to a typed Cobra command with `--json`/`--select`/`--csv`/`--dry-run`/SQLite cache where applicable. The generator handles the bulk of these from the OpenAPI spec.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List class sessions | Customer API `GET /classes` + filters | `marianatek classes list` + offline SQLite + FTS | works offline, multi-tenant, agent-friendly JSON |
| 2 | Single class detail | `GET /classes/{id}` | `marianatek classes get <id>` | --json, --select |
| 3 | Class payment options | `GET /classes/{id}/payment_options` | `marianatek classes payment-options <id>` | typed, --json |
| 4 | Locations list + single | `GET /locations[/{id}]` | `marianatek locations list/get` | cached + FTS |
| 5 | Regions list + single | `GET /regions[/{id}]` | `marianatek regions list/get` | cached |
| 6 | Schedule filters | `GET /{locations,regions}/{id}/schedule_filters` | `marianatek schedule filters` | powers tab-completion |
| 7 | Per-location schedule | `GET /locations/{id}/schedule` | `marianatek locations schedule <id>` | typed |
| 8 | Per-region schedule | `GET /regions/{id}/schedule` | `marianatek regions schedule <id>` | typed |
| 9 | Account profile CRUD | `/me/account` GET/PATCH/POST/DELETE | `marianatek account [get\|update\|create\|delete]` | typed |
| 10 | Account achievements | `GET /me/achievements` | `marianatek account achievements` | typed |
| 11 | Communications preferences | `POST /me/account/update_communications_preferences` | `marianatek account comms set <type> <value>` | typed |
| 12 | Profile image upload | `POST /me/account/upload_profile_image` | `marianatek account upload-image <file>` | typed |
| 13 | Credit cards CRUD | `/me/credit_cards` GET/POST/PATCH/GET/DELETE | `marianatek cards [list\|add\|update\|get\|delete]` | typed |
| 14 | Gift card redeem | `POST /me/credit_cards/redeem_gift_card` | `marianatek cards redeem <code>` | typed, --dry-run |
| 15 | Credits list + single | `GET /me/credits[/{id}]` | `marianatek credits list/get` | typed, expiry view |
| 16 | Memberships list + single | `GET /me/memberships[/{id}]` | `marianatek memberships list/get` | typed |
| 17 | Reservations CRUD | `/me/reservations` POST/GET/GET/DELETE | `marianatek book` + `marianatek reservations [list\|get\|cancel]` | typed, --dry-run, --auto |
| 18 | Cancel penalty preview | `GET /me/reservations/{id}/cancel_penalty` | `marianatek reservations cancel-penalty <id>` | typed |
| 19 | Check in | `POST /me/reservations/{id}/check_in` | `marianatek reservations check-in <id>` | typed |
| 20 | Swap spot | `POST /me/reservations/{id}/swap_spot` | `marianatek reservations swap <id> <target>` | typed |
| 21 | Buy pages | `GET /buy_pages/{id}` | `marianatek buy-pages get <id>` | typed |
| 22 | Add-ons buy pages + sub-resources | `GET /add_ons_buy_pages/...` | `marianatek add-ons [buy-pages\|sections\|listings]` | typed |
| 23 | Cart CRUD + checkout + discount | `/me/cart`, `/me/cart/line_items`, `/me/cart/discount_code`, `/me/cart/checkout` | `marianatek cart [show\|add\|set\|discount\|checkout\|clear]` | typed, --dry-run |
| 24 | Add-ons cart full | `/me/add_ons_cart/...` | `marianatek add-ons cart [...]` | typed |
| 25 | Orders list/single/cancel | `/me/orders` GET/GET/DELETE | `marianatek orders [list\|get\|cancel]` | typed |
| 26 | Metrics: class_count | `GET /me/metrics/class_count` | `marianatek metrics count` | typed |
| 27 | Metrics: longest weekly streak | `GET /me/metrics/longest_weekly_streak` | `marianatek metrics streak` | typed |
| 28 | Metrics: most popular day | `GET /me/metrics/most_popular_day` | `marianatek metrics day` | typed |
| 29 | Metrics: top instructors | `GET /me/metrics/top_instructors` | `marianatek metrics instructors` | typed |
| 30 | Metrics: top time of day | `GET /me/metrics/top_time_of_day` | `marianatek metrics time-of-day` | typed |
| 31 | Metrics: total minutes | `GET /me/metrics/total_minutes` | `marianatek metrics minutes` | typed |
| 32 | Appointments categories | `GET /appointments/categories` | `marianatek appointments categories` | typed |
| 33 | Appointment services payment options | `GET /appointments/services/.../payment_options` | `marianatek appointments services payment-options` | typed |
| 34 | Appointment services schedule filters | `GET /appointments/services/.../schedule_filters` | `marianatek appointments services filters` | typed |
| 35 | Appointment services slots | `GET /appointments/services/.../slots` | `marianatek appointments services slots` | typed |
| 36 | Feedback CSAT show + submit | `/feedback/show_csat`, `/feedback/submit_csat` | `marianatek feedback [show\|submit]` | typed |
| 37 | Brand config | `GET /config` | `marianatek config` | typed |
| 38 | Legal documents | `GET /legal` | `marianatek legal` | typed |
| 39 | Theme | `GET /theme` | `marianatek theme` | typed |
| 40 | App version metadata | `/app_version_metadatas[/{id}]` | `marianatek app-versions` | typed |
| 41 | Countries lookup | `/countries[/{iso}]` | `marianatek countries` | cached locally |
| 42 | OAuth Authorization Code login | `/o/authorize` + `/o/token` | `marianatek login --tenant <slug>` | interactive browser, refresh-token-to-disk |
| 43 | OAuth refresh | `/o/token` (refresh_token grant) | automatic in HTTP client | transparent retry on 401 |
| 44 | Logout / revoke | (local file delete) | `marianatek logout --tenant <slug>` | clean revoke |
| 45 | Tenant context switch | n/a (CLI-only) | `marianatek tenants list` / `--tenant` flag | multi-tenant config |

(Note: rows 1–41 are spec-derived and will be generated by `printing-press`; rows 42–45 are auth/CLI plumbing the generator scaffolds plus we wire.)

## Transcendence (only possible with our approach)

These are the 8 survivors from the novel-features subagent's three-pass brainstorm + adversarial cut. All ≥ 5/10. Personas: **Maya** (multi-tenant regular), **Jordan** (cancellation hunter), **Sam** (agentic scheduler).

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|------------------------|
| T1 | Watch for cancellation | `marianatek watch <class-id\|--filter k=v,...> --interval 60s [--auto-book]` | 10/10 | Jordan, Sam | API has no waitlist signal at consumer scope. We synthesize one via interval poll of `/classes/{id}` and `/classes/{id}/payment_options`; emits NDJSON on spot-open; optional auto-book closes the loop. |
| T2 | Multi-tenant unified schedule | `marianatek schedule --any-tenant --type vinyasa --before 07:00 --window 7d` | 10/10 | Maya, Sam | Mariana Tek API is single-tenant by construction. Only the local SQLite store (one row per tenant per class session) can produce a cross-tenant joined view. |
| T3 | Regulars / personal affinity | `marianatek regulars [--by instructor\|type\|time\|day\|location] [--top 5]` | 8/10 | Maya, Jordan | `/me/metrics/top_instructors` returns one dimension; `regulars` joins reservations + classes + instructors + locations and groups by any dimension or combination. |
| T4 | Expiring credits + memberships | `marianatek expiring --within 30d` | 8/10 | Maya | API exposes `expires_at` per credit/membership but never aggregates. `expiring` joins balance + expiry + candidate-session-cost to surface "use it or lose it" with concrete recommendations. |
| T5 | Cross-tenant calendar conflicts | `marianatek conflicts <date> [--ics <path>] [--buffer 30m]` | 7/10 | Maya, Sam | API has no cross-tenant view at all. Reads reservations across all logged-in tenants, optionally an exported ICS, flags overlaps + insufficient buffer including travel-time between locations. |
| T6 | FTS5 catalog search | `marianatek search "vinyasa soho morning"` | 8/10 | Maya, Sam | API filters are structured-field-only. SQLite FTS5 over name + instructor + location + region + class type in one BM25-ranked query (powers "earliest vinyasa at any location under 45 min" agentic queries). |
| T7 | Book-regular compound | `marianatek book-regular --slot "tue-7am-vinyasa" [--auto]` | 8/10 | Maya | Compound of T3 regulars + T6 search + reservation create. No single Mariana Tek surface lets a user say "the usual Tuesday slot" — it's always three clicks. |
| T8 | Multi-tenant doctor | `marianatek doctor` | 6/10 | Sam | Per-tenant token expiry, last sync, row counts, one-class-probe for reachability. Mariana Tek itself can't see your tokens or your local cache. |

## Stubs
None. All 8 transcendence features are shipping-scope.

## Risk / known dependencies
- **Iframe auth flow** may use OAuth2 password grant against `/o/token` (with `grant_type=password`) rather than the documented `authorization_code` flow. The HAR capture will confirm. If true, we offer both: `marianatek login --tenant <slug>` (browser flow) AND `marianatek login --tenant <slug> --email --password` (password grant) for headless / CI use.
- **Waitlist endpoints** are documented as deprecated in Customer API v1.0.0. If the iframe widget surfaces a working waitlist endpoint, we absorb it and use it as a precision-improvement on `watch` mode (real waitlist signal vs polling synthetic).
- **JSON:API envelope** (`application/vnd.api+json`): generator's JSON:API handling needs to be confirmed during dogfood. Pagination uses `meta.pagination` + `links.next` — page-based.
- **Multi-tenant config**: each tenant gets its own `~/.config/marianatek/<tenant>.token.json` (0600). Tenant resolution: `--tenant` flag > `MARIANATEK_TENANT` env > default in `~/.config/marianatek/config.toml` > error.

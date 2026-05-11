# Goose CLI Absorb Manifest

## Scope notes
- **Read-only v1.** No mutation commands ship (no POST/PUT/DELETE) until the user explicitly requests writes. Cognito tokens are scoped admin-level, so the API permits writes; we just don't expose them. The Annotations contract per command sets `mcp:read-only: true`.
- **No competing CLI/MCP/SDK exists for goose.pet.** No public docs, no community wrappers. The "absorb" surface is the web app's admin functionality — what a logged-in admin can already do — and our value-add is agent-native plumbing, offline FTS over local store, and cross-entity transcendence features that the web UI cannot answer in one step.

## Absorbed (match the web app's admin surface)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Today's dashboard (arrivals/departures/here) | Web app / Dashboard | `goose dashboard --date today` — composite endpoint wrapper | `--json --select`, offline-friendly, agent-pipeable |
| 2 | Booking/reservation list | Web app / Bookings | `goose bookings list --service boarding --from gte_2026-05-10` | `--json`, includes-aware, local store cache |
| 3 | Booking detail | Web app / Invoice page | `goose bookings get <invoiceId>` | All includes in one shot, `--select` field projection |
| 4 | Customer search | Web app / Customers | `goose customers search <query>` — hits search-api.goose.pet | Local FTS fallback when offline |
| 5 | Customer detail | Web app / Customer page | `goose customers get <userId>` | All includes (pets, vouchers, agreements, payments, notes) |
| 6 | Customer outstanding balance | Web app | `goose customers balance <userId>` | Bulk batch lookup, `--json` |
| 7 | Customer vouchers (credits) | Web app | `goose customers vouchers <userId> --type OFFER\|CASH` | Filter by used/expired, agent output |
| 8 | Customer notes | Web app | `goose customers notes <userId> --limit N` | Sort, JSON, paged |
| 9 | Customer payment methods | Web app | `goose customers payment-methods <userId>` | Returns v1 + v2 endpoints |
| 10 | Pet detail | Web app | `goose pets get <petId>` | Activities, vaccinations, instructions in one call |
| 11 | Pet search | Web app | `goose pets search <name>` | FTS across all pets |
| 12 | Service-type catalog | Web app | `goose services list` | Lists boarding/daycare/grooming/etc with serviceType joined |
| 13 | Species/breeds catalog | Web app | `goose species list` | Useful for new-customer onboarding scripts |
| 14 | Grooming schedule | Web app / Scheduler | `goose schedule --date today --service grooming` | Resource assignments, availability windows |
| 15 | Staff/resources list | Web app | `goose staff --subtype GROOMING_STAFF` | Status filters |
| 16 | Resource availability | Web app | `goose availability --start --end --type --subtype` | Daterange query, JSON output |
| 17 | Booking configurations | Web app | `goose booking-confs --service <id>` | Operational config for a service type |
| 18 | Reports catalog | Web app / Reports landing | `goose reports list` | Lists all 50+ report types by category |
| 19 | Direct CSV export (one) | Web app / Export buttons | `goose reports export <slug> --date <YYYY-MM-DD>` | Replays each Data Export (sales, customer-activity, agreements, feeding-medication, vaccinations, etc.) |
| 20 | Explo dashboard URL | Web app / Report page | `goose reports open <slug> [--launch]` | Mints Explo embed token via soar-api, prints/opens URL |
| 21 | Report cards (Pawgress) | Web app / Report Cards | `goose report-cards --date YYYY-MM-DD` | Daily pet progress reports |
| 22 | Conversations | Web app / Messages | `goose conversations list` | Goose-native messaging (separate from embedded Intercom) |
| 23 | Contracts | Web app | `goose contracts list --type SERVICE_AGREEMENT` | Reference data |
| 24 | Restrictions (price/occupancy) | Web app / Revenue Mgmt | `goose restrictions list --type PRICE --target OFFER` | Revenue management policies |
| 25 | Auth (Cognito JWT bearer) | Web app signin | Generator emits Bearer scaffolding (env var `GOOSE_ACCESS_TOKEN`) — login flow itself is transcendence #1 below | — |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This | Persona |
|---|---------|---------|-------|------------------------|---------|
| 1 | Cognito-from-Chrome auth bootstrap | `goose auth login --chrome` | 9/10 | Reads `CognitoIdentityServiceProvider.<clientId>.<email>.refreshToken` from app.goose.pet localStorage; calls Cognito `InitiateAuth REFRESH_TOKEN_AUTH` for fresh accessToken; no manual JWT paste each hour | All personas |
| 2 | Composite daily roster with cross-warnings | `goose today [--date YYYY-MM-DD]` | 9/10 | Single `dashboard/invoices?visitDate=` call with all includes; mechanically surfaces vaccine-expired, agreement-missing, balance-due flags from the returned join graph; web app spreads across 5 screens | Owner, Front-Desk |
| 3 | Customer one-shot search→detail | `goose customer <name>` | 8/10 | Stitches search-api + admin API + balance endpoint; web app makes you click each section sequentially | Off-Hours, Owner |
| 4 | Pet one-shot lookup | `goose pet <name>` | 7/10 | FTS over local `location_pet_profiles` (or pet search via search-api), then resolves with tags+vaccinations+instructions+activities | Front-Desk, Off-Hours |
| 5 | Vaccinations expiring × visit window | `goose vaccines expiring --within 30d [--by-visit]` | 8/10 | Local SQL: `location_pet_profiles` JOIN vaccinations [LEFT JOIN upcoming `invoices` when `--by-visit`]. The documented Expiring-Vaccinations report doesn't intersect with bookings | Owner, Front-Desk |
| 6 | Churn list with voucher overlay | `goose churn --not-booked-since 60d [--has-voucher]` | 7/10 | Local SQL: `location_user_profiles` LEFT JOIN MAX(`invoices.period.startDate`) per user; optional JOIN `vouchers WHERE atLeastOneAvailable=true`. No API aggregation endpoint exists | Analyst |
| 7 | Bulk-export week of CSV reports | `goose reports run-all --week <YYYY-WW>` | 7/10 | Fan-out over 16 documented `reports/<slug>?date=...` CSV endpoints, writes timestamped files to `./reports/<week>/`. Beats 16 sequential web-app click-export cycles | Analyst |
| 8 | Daily-prep alerts panel | `goose alerts daily` | 7/10 | Joins fresh `dashboard/invoices` includes + bulk `outstanding-balance` + local voucher window: incoming pets with expired vaccines, customers without active agreement, checkouts with non-zero balance, vouchers within 7d of expiry tied to today's customers | Front-Desk |

## Killed candidates

| # | Feature | Kill reason | Closest survivor |
|---|---------|-------------|-----------------|
| K1 | AI summary of the day's roster | LLM-dependency rule | `today --json \| <user's LLM>` |
| K2 | Sentiment scoring of customer notes | LLM-dependency rule | — |
| K3 | Auto-text-blast for vaccines expiring | External service (Twilio) not in spec; write side-effect | `vaccines expiring --json \| <user's tool>` |
| K4 | Waitlist optimizer | Scope creep + unverifiable | `restrictions list` (absorbed) |
| K5 | Plain `feeding today` printable board | Subsumed by `alerts daily` + `reports export feeding-medication-export` | `alerts daily`, `reports export` |
| K6 | `balance-due --checkout today` | Subsumed by `alerts daily` | `alerts daily` |
| K7 | `vouchers expiring` | Subsumed by `alerts daily` (operational) + `churn` (analyst) | `alerts daily`, `churn` |
| K8 | `agreements unsigned --checkin tomorrow` | Subsumed by `alerts daily` | `alerts daily` |
| K9 | `notes grep` | Framework `sql` command covers it | `sql 'SELECT ... FROM notes WHERE body LIKE ...'` |
| K10 | `pawgress drafts pending` | Verifiability risk; no documented "draft" state | — |

## Authorship audit trail
Full Pass-1/Pass-2/Pass-3 brainstorm: `2026-05-10-205901-novel-features-brainstorm.md`.

# Human-Goat CLI Absorb Manifest

Combo CLI over two human-labor networks. Absorb target = the TaskRabbit web app surface
(sniffed live) + the working Magic bash CLI. Approved at Phase Gate 1.5 (2026-07-03).

## Absorbed (match the app + Magic)
| # | Feature | Source | Our command | Disposition |
|---|---------|--------|-------------|-------------|
| 1 | Browse categories/templates | TR metro_task_template | human-goat-pp-cli categories | (generated endpoint) |
| 2 | Resolve account metro | TR bootstrap | (behavior in human-goat-pp-cli doctor) | |
| 3 | List bookings | TR page.tasks.list | (behavior in human-goat-pp-cli tasks list) | hand-code (tRPC adapter) |
| 4 | Booking detail | TR page.tasks.list item | (behavior in human-goat-pp-cli tasks get) | hand-code |
| 5 | Favorite Taskers | TR poster_rabbit_relationships/favorite | human-goat-pp-cli taskers favorites | (generated endpoint) |
| 6 | Past Taskers | TR poster_rabbit_relationships/past | human-goat-pp-cli taskers past | (generated endpoint) |
| 7 | Tasker suggestions | TR dashboard/tasker_suggestions | human-goat-pp-cli taskers suggestions | (generated endpoint) |
| 8 | Account profile | TR account.json | human-goat-pp-cli account | (generated endpoint) |
| 9 | Search Taskers + prices | TR page.book.recommendations | human-goat-pp-cli search | hand-code (tRPC adapter) |
| 10 | Availability slots | TR page.book.schedule | (behavior in human-goat-pp-cli availability) | hand-code |
| 11 | Confirm/all-in quote | TR page.book.confirm | (behavior in human-goat-pp-cli book quote) | hand-code |
| 12 | Hire/commit | TR commit mutation | human-goat-pp-cli hire | hand-code, GATED (real round-trip) |
| 13 | Cancel | TR page.tasks.cancelTask | human-goat-pp-cli cancel | hand-code, GATED |
| 14 | Reschedule | TR mutation | (behavior in human-goat-pp-cli reschedule) | hand-code, GATED |
| 15 | Rescue/re-match | TR page.tasks.rescueRecommendation | (behavior in human-goat-pp-cli rescue) | hand-code |
| 16 | Invoices/history | TR invoices | human-goat-pp-cli invoices | (generated endpoint) |
| 17 | Cookie auth from Chrome | TR session | human-goat-pp-cli auth login --chrome | (generated) |
| 18 | Create remote request | Magic POST /request | (behavior in human-goat-pp-cli send) | hand-code (Magic adapter) |
| 19 | Phone-call errand | Magic POST /request | (behavior in human-goat-pp-cli call) | hand-code |
| 20 | Request status/answer | Magic GET /request/{id} | (behavior in human-goat-pp-cli track) | hand-code |
| 21 | Reply into conversation | Magic POST /conversation | (behavior in human-goat-pp-cli reply) | hand-code |

## Transcendence (approved at gate, all hand-code)
| # | Feature | Command | Buildability | Score | Why only we can do it |
|---|---------|---------|--------------|-------|------------------------|
| 1 | Honest all-in ranking | best | hand-code | 10/10 | Fold the empirical +34% fee markup into a ranked shortlist the app never shows |
| 2 | Cross-source spend analytics | spend | hand-code | 9/10 | Local SQL over bookings+invoices+magic tasks; true all-in effective $/hr |
| 3 | Cross-source dispatch | dispatch | hand-code | 9/10 | Task-shape routing across two human networks; --via override |
| 4 | Unified in-flight inbox | status | hand-code | 7/10 | Local join of TR bookings + Magic requests on the common Task model |
| 5 | Qualifying-opening watch | watch | hand-code | 7/10 | Poll availability with a rating/all-in-price predicate; agent-shaped single match |

Dropped at gate: compare (folded into best), rebook (folded into hire+watch).

## Build division (why some is hand-code)
- **Generator produced:** the `goat` scaffold, cookie auth (`auth login --chrome`), config, SQLite
  store, `doctor`, framework (sync/search), MCP tree, README/SKILL, and the TaskRabbit REST
  `/api/v3` read commands (categories, taskers, account, invoices). Verified: builds, vets,
  govulncheck, doctor all pass.
- **Hand-code (the combo pattern + transcendence):** the TaskRabbit tRPC funnel and Magic second
  source cannot live in one generator spec (two base URLs, two auth models, non-REST tRPC envelope),
  so they are `internal/source/{taskrabbit,magic}/` adapters — exactly how every combo CLI in the
  library (flightgoat, contact-goat, prediction-goat) is built. The 5 transcendence commands are
  hand-code by definition (the generator never emits transcendence). All-in math lives in
  `internal/pricing`.

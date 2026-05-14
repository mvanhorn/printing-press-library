# Cal.com Absorb Manifest

Bar: bcharleson/calcom-cli (61 tools, 14 resource groups) + raw OpenAPI surface (82 paths, 121 operations).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|--------------------|-------------|
| 1 | bookings list / get / create / cancel / confirm / decline / reschedule / mark-absence / reassign / update-location | bcharleson + OpenAPI | Generated typed Cobra commands | --json --select --csv --compact, --dry-run, typed exit codes, offline-cached search |
| 2 | booking attendees CRUD + add-guests | OpenAPI (bcharleson partial) | Generated | Full add/remove/list parity |
| 3 | booking recordings / transcripts / references / conferencing-sessions / calendar-links | OpenAPI (bcharleson lacks) | Generated | New surface bcharleson never absorbed |
| 4 | event-types list / get / create / update / delete | bcharleson + OpenAPI | Generated | Generated parity |
| 5 | event-type webhooks CRUD | OpenAPI (bcharleson lacks) | Generated | Sub-resource bcharleson missed |
| 6 | event-type private links CRUD | OpenAPI (bcharleson lacks) | Generated | Sub-resource bcharleson missed |
| 7 | event-type destination + selected calendars | OpenAPI | Generated | Sub-resource bcharleson missed |
| 8 | schedules list / get / default / create / update / delete | bcharleson + OpenAPI | Generated | Generated parity |
| 9 | slots query / reserve / get-reserved / update-reserved / delete-reserved | bcharleson + OpenAPI | Generated | Generated parity |
| 10 | calendars list / busy-times / check / save-credentials | bcharleson + OpenAPI | Generated | Generated parity |
| 11 | unified calendars (multi-provider) events CRUD + freebusy | OpenAPI (bcharleson lacks) | Generated | New multi-provider surface |
| 12 | calendars ICS feed save / check | OpenAPI | Generated | ICS feed support |
| 13 | webhooks CRUD (top-level) | bcharleson + OpenAPI | Generated | Generated parity |
| 14 | OAuth-clients webhooks CRUD (deprecated platform path) | OpenAPI | Generated | Spec coverage |
| 15 | OAuth-clients managed users CRUD + force-refresh | OpenAPI | Generated | Spec coverage |
| 16 | OAuth-clients CRUD (deprecated platform path) | OpenAPI | Generated | Spec coverage |
| 17 | profile (me) read / update | bcharleson + OpenAPI | Generated | Generated parity |
| 18 | OOO list / create / update / delete | bcharleson + OpenAPI | Generated | Generated parity |
| 19 | teams list / get / create / update / delete | bcharleson + OpenAPI | Generated | Generated parity |
| 20 | conferencing list / default / set-default / connect / disconnect / oauth callback | bcharleson + OpenAPI | Generated | Generated parity |
| 21 | destination calendars update | bcharleson + OpenAPI | Generated | Generated parity |
| 22 | selected calendars add / delete | bcharleson + OpenAPI | Generated | Generated parity |
| 23 | Stripe check / connect / save | bcharleson + OpenAPI | Generated | Generated parity |
| 24 | verified emails request-code / verify / list / get | OpenAPI (bcharleson lacks) | Generated | New surface |
| 25 | verified phones request-code / verify / list / get | OpenAPI (bcharleson lacks) | Generated | New surface |
| 26 | api-keys refresh | OpenAPI | Generated | Spec coverage |
| 27 | OAuth2 token + client lookup | OpenAPI | Generated | Spec coverage |
| 28 | auth login / logout / status / doctor | Press framework | Auto-emit | Framework table stakes |
| 29 | sync (incremental) / reconcile / SQL composable over local SQLite | Press framework | Auto-emit | Offline, no rate-limit, jq/sql-pipeable |
| 30 | search (FTS over local) | Press framework | Auto-emit | Offline full-text |
| 31 | MCP server (every Cobra command exposed) | Press framework | Auto-emit | Single-binary CLI + MCP |
| 32 | --json / --select (dotted) / --csv / --compact / --quiet / --dry-run / typed exit codes | Press framework | Auto-emit | Agent-native |
| 33 | AdaptiveLimiter (X-RateLimit-Remaining/Reset aware) | Press framework | Auto-emit | Beats bcharleson exponential backoff with cooperative limiter |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Conflict scan across linked calendars | `conflicts --window 7d` | 9/10 | Joins local calendars/busy multi-provider rows against bookings; emits overlap with offending calendar+title | Brief thesis names "conflict detection"; bcharleson lacks; Sam pain |
| 2 | No-show risk by attendee | `no-show-risk --since 30d` | 8/10 | Local aggregation over bookings grouped by attendee email; cancel_rate, no_show_rate, total_count | Brief names "who's about to no-show"; no API endpoint; Devi+Priya |
| 3 | Attendee history | `attendee <email> [--summary]` | 8/10 | Joins bookings × attendees × event_types filtered by email; --summary = GROUP BY first/last/counts | Brief Build Pri #3; Marco; no API endpoint |
| 4 | Schedule gap finder | `gaps --min 30m --window 7d` | 7/10 | Walks schedules availability windows minus bookings time-ranges; emits contiguous gaps >= min | Brief Build Pri #3; Marco/Sam ritual |
| 5 | Reschedule replay | `reschedule-history <uid>` | 7/10 | Recursive walk of rescheduledFromUid pointers; ordered timeline | Brief Build Pri #3; Priya debug |
| 6 | Cancel sweep | `cancel-sweep --status PENDING --older-than 48h [--apply]` | 7/10 | Local SQL pre-filter; --apply loops /bookings/{uid}/cancel; --dry-run default | Brief Build Pri #3; Devi Wed |
| 7 | Host load report | `host-load --week 2026-W20` | 6/10 | Local GROUP BY host_user_id with counts/hours/cancel-rate/no-show-rate | Devi VP report; no API aggregation |
| 8 | Load-by-day | `load-day <date>` | 5/10 | /bookings windowed call + upsert + delta line vs prior sync | Brief Build Pri #3; Priya debug |

No stubs. All transcendence rows ship fully implemented.

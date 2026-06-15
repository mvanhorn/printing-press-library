# Sutra Fitness CLI — Absorb Manifest

**Landscape:** Greenfield. No CLI, SDK, MCP, or wrapper exists for the Sutra/Arketa
Partner API. Competitors (Mindbody, Momence, Pike13, Glofox, Walla) ship dashboards
and library wrappers, never CLIs. The absorbed set matches every table-stakes feature
those platforms offer; the transcendence set ships what none of them do — local-join
analytics over a data mirror the operator owns.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List locations | (all platforms) | (generated endpoint) locations list | Offline, `--json`/`--select`/`--csv`, SQL-composable |
| 2 | List location rooms (+ spot map) | (all platforms) | (generated endpoint) rooms list | Offline, spot_map preserved in store |
| 3 | Class schedule pull (date+location filters) | Mindbody/Momence/Pike13 | (generated endpoint) classes list | Date-range + location filter, offline, agent-native |
| 4 | Get single class | (all platforms) | (generated endpoint) classes get | `--select` field narrowing |
| 5 | Class roster (list reservations) | Mindbody/Momence/Pike13 | (generated endpoint) reservations list | Offline, scriptable, pipes to jq |
| 6 | Book reservation (existing/new client) | Mindbody/Momence | (generated endpoint) reservations create | `--dry-run`, stdin body, idempotent-friendly |
| 7 | Cancel reservation (late-cancel/refund) | Mindbody/Momence | (generated endpoint) reservations cancel | `--dry-run`, typed exit codes |
| 8 | Check in reservation | Mindbody/Momence/Pike13 | (generated endpoint) check-in | Door-ready, `--dry-run` |
| 9 | List clients | (all platforms) | (generated endpoint) clients list | The export competitors make painful — offline + CSV |
| 10 | Get single client | (all platforms) | (generated endpoint) clients get | `--select` |
| 11 | List purchases (memberships/packs) | Momence/Pike13 | (generated endpoint) purchases list | Offline, filterable, feeds analytics |
| 12 | List referrals | (rare in competitors) | (generated endpoint) referrals list | Offline, feeds referral-funnel analytic |
| 13 | Local SQLite data mirror | (none — unique) | (behavior in sutra-fitness-pp-cli sync) incremental updated_at cursor + start_after pagination | Operator owns full queryable copy offline |
| 14 | Offline full-text search | (none — unique) | sutra-fitness-pp-cli search | Search clients/classes/locations offline |
| 15 | Raw SQL over local store | (none — unique) | sutra-fitness-pp-cli sql | Arbitrary SELECT analytics |
| 16 | Generic group-by analytics | Mindbody canned reports | (generated framework) analytics | Single-resource group-by aggregation |
| 17 | CSV / JSON / field-select export | Mindbody/Momence CSV export | (behavior in output flags) --json/--csv/--select/--compact | Owned, scriptable, no vendor lock |

## Transcendence (local joins — impossible from any single Sutra endpoint)

The Sutra API has ZERO reporting/aggregation endpoints. Every row below is a
hand-written Cobra command that reads the local SQLite mirror and joins across
tables. All tagged `hand-code` (the endpoint mirrors above are the only `spec-emits`
surface, and they are absorbed, not transcendence). `// pp:data-source local`;
each calls the sync-hint helper before returning.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Instructor scorecard | scorecard | hand-code | Join classes × reservations aggregating fill/no-show/check-in rates per instructor_name; no Sutra endpoint returns per-instructor aggregates | Use this command to rank instructor performance (fill, no-show, check-in rates). Do NOT use it for per-slot capacity — use 'utilization'. No-show is one column here; to pivot no-shows by class or client use 'no-shows'. |
| 2 | No-show rate | no-shows | hand-code | Aggregate reservations.status=NO_SHOW over BOOKED grouped by instructor/class/client; vendor buries this in a fixed Attendance report | Use this command to surface no-show rates by instructor, class, or client. For a full instructor ranking use 'scorecard'; for renewal-risk clients use 'churn'. |
| 3 | Capacity utilization | utilization | hand-code | total_booked/max_capacity per class/instructor/timeslot/location over a window; segmentation/aggregation is local-only | Use this command for fill ratio across many classes or slots. For one class's attendees use the reservations list; for teacher performance use 'scorecard'. |
| 4 | Expiring / low-balance watchlist | expiring | hand-code | Filter purchases by end_date window OR low remaining_uses AND status=ACTIVE, joined to client contact info; ranking + contact join is local | Use this command for deterministic renewal outreach (date/credit threshold, act now). For behavioral lapse signals use 'churn'. |
| 5 | Churn / at-risk clients | churn | hand-code | Join clients × reservations × purchases: non-removed clients with no recent CHECKED_IN and/or EXPIRED plan via mechanical recency threshold | Use this command for behavioral at-risk clients (lapsed attendance or expired plan). For hard date/credit expiry use 'expiring'. |
| 6 | Revenue by type + prior-period | revenue | hand-code | Sum purchases.price by type/location for a window AND delta vs prior equal window — the explicit Capterra "prior-period comparison" gap | none |
| 7 | Referral funnel conversion | referral-funnel | hand-code | Walk referrals → clients.created_at → first purchase/check-in to count conversion and rank referrers; three-table funnel | none |
| 8 | Client lifetime value | ltv | hand-code | Sum purchases.price per client with tenure from created_at, ranked; multi-table aggregation | Use this command for per-client lifetime spend ranking. For period totals by plan type use 'revenue'. |

**Cut at brainstorm (re-addable at the gate):** roster-with-spot-map, first-visit
follow-up, daily door-brief — strong but trimmed to hold an 8-feature target;
`mark-no-shows` killed as unbuildable (no NO_SHOW write op).

## Hand-code commitment
- **8 transcendence features, all hand-code** (~50-150 LoC each + root.go wiring).
- **0 transcendence spec-emits.** Absorbed endpoint commands (rows 1-12) are generator-emitted.
- They share one local-store-read + aggregate + JSON-envelope/table pattern, so build cost amortizes.

## No stubs
- No row ships as a stub. `mark-no-shows` was killed (not stubbed) because the API has no NO_SHOW write transition.

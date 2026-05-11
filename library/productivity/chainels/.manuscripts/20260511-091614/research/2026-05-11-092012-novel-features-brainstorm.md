# Chainels Novel-Features Brainstorm (Phase 1.5c.5 subagent output)

## Customer model

**Maya — Senior Community Manager, mixed-use retail + residential portfolio (12 communities, ~3,400 members).**

*Today:* Maya lives in the Chainels web UI across a dozen tabs — one per community. She copies turnover numbers into a spreadsheet on the 5th of each month, scrolls timelines for unresolved issues, and exports CSVs by hand. She has no way to ask "which 47 of my tenants haven't filed July turnover yet" without opening each community.

*Weekly ritual:* Monday morning she sweeps open issues across all 12 communities, chases the laggards on periodic reports, posts a building-wide timeline message about lift maintenance, and checks bookings for the upcoming weekend's shared-space conflicts. Friday she pulls a renewals-this-quarter list and an alarm-recipient audit before the on-call handoff.

*Frustration:* She cannot grep across communities. Stale issues hide in low-traffic communities. Turnover variance is eyeballed, not computed. Read receipts on a broadcast require opening each message individually.

**Devi — Integrator at a Yardi/Entrata implementation partner, syncing leases + units for 4 Chainels customers.**

*Today:* Devi writes one-off Postman collections per customer. She pages through `/companies`, `/agreements`, `/agreement-items`, and `/accounts` to reconcile against the property-management system. Every sync is bespoke; nothing is idempotent.

*Weekly ritual:* Tuesday she runs a delta sync per customer (agreements changed since last Friday), Wednesday she reconciles member roles against the source-of-truth HRIS, Thursday she diffs alarm-recipient lists against the building's emergency roster, Friday she ships a report.

*Frustration:* No cursor primitive — she rebuilds "since X" logic in every script. No cross-resource join — agreement → accounts → roles requires three round-trips per row. No machine-readable diff between two snapshots.

**Rashid — Retail tenant ops lead, files turnover for 19 store locations across 6 Chainels communities.**

*Today:* On the 5th of each month a calendar reminder makes him open six Chainels logins and key in last month's gross sales per location. He keeps a Google Sheet of "did I submit yet" because the web UI only shows one community at a time.

*Weekly ritual:* Monthly, not weekly — but the monthly cycle dominates: gather POS exports, file turnover per location, respond to any landlord follow-up via Chainels message, then close. Mid-month he checks discounts/offers from each landlord.

*Frustration:* No "submit all pending turnover from this CSV" path. No "show me my last 12 months' submissions" view. No variance check against his own history.

**Ines — Chainels platform SRE, on-call for alarm-config drift and auth-related tenant reports.**

*Today:* When a tenant says "the alarm didn't fire" she logs into the demo server, copies prod alarm config, diffs by eye. Service-account rotation is manual.

*Weekly ritual:* Weekly she audits service accounts (which still have `send.alarm` scope?), spot-checks alarm recipients vs the building's emergency list, and rotates one or two long-lived tokens.

*Frustration:* No diff tool for alarm config. No "who has what scope" rollup. No way to fire a test alarm against the demo server from a script.

## Survivors (>= 5/10)

| # | Feature | Command | Score | Persona | How It Works | Evidence |
|---|---------|---------|-------|---------|--------------|----------|
| 1 | Cross-community FTS | `search "<query>"` | 9/10 | Maya | FTS5 over synced messages/issues/agreements in local SQLite; one query spans every community | Brief Data Layer note |
| 2 | Issue assignee load | `issues load` | 8/10 | Maya | Local groupby on synced issues by assignee + age buckets | Brief P2 list; workflow #2 |
| 3 | Stale issue digest | `issues stale` | 7/10 | Maya | Local query: issues with no state change for N days, across communities | Brief P2 |
| 4 | Turnover variance | `turnover variance` | 8/10 | Maya, Rashid | Local arithmetic: per-tenant variance vs trailing-N median | Brief P2; workflow #3 |
| 5 | Turnover laggards | `turnover pending` | 8/10 | Maya | Set-difference: expected submitters minus actuals for a period | Workflow #3 |
| 6 | Agreement renewals | `agreements renewals` | 7/10 | Maya, Devi | Local filter on agreements by end-of-term window | Brief P2; workflow #5 |
| 7 | Member-load audit | `members audit` | 7/10 | Maya, Devi | Join accounts + entity roles; per-account role counts, flag dups/orphans | Brief P2 |
| 8 | Alarm recipient diff | `alarms diff` | 6/10 | Ines, Maya | Diff over two alarm-config snapshots | Brief P2 |
| 9 | Since-sync changed | `changed` | 7/10 | Devi | Union of `updated_at >= ts` across resources | Brief Data Layer + P2 |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Broadcast read-receipt rollup | Brief does not confirm read-receipt fields on message resource; risk of synthesizing data | #1 cross-community FTS |
| Bulk turnover submit from CSV | Write-path with CSV mapping is application-shaped, not command-shaped | #5 turnover laggards |
| Booking utilization | Narrower persona; not in weekly ritual | #2 issue assignee load |
| Booking conflicts | Verifiability hard without planted bad fixture | #9 since-sync changed |
| Service-account scope audit | Only Ines's job, not weekly for larger personas | #7 member-load audit |
| Agreement parties join | Subsumed by member-load audit's join graph | #7 member-load audit |
| Community digest | Dashboard-shaped; covered by `changed --since` + survivors | #9 since-sync changed |

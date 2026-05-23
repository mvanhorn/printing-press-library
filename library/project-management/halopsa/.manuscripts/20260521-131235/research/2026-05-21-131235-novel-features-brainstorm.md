# HaloPSA Novel Features Brainstorm (subagent audit trail)

## Customer model

**Persona 1: Maria, MSP Tier-2 dispatcher / shift lead**

*Today (without this CLI):* Maria runs a HaloPSA console with five tabs open — Open Tickets (filtered to her team), one ticket detail per active escalation, the agent roster page, and the SLA breach board. She refreshes the queue every 10-15 minutes through the day. When a P1 lands she manually scans agent utilization by clicking each tech, eyeballs ticket counts, and reassigns. She cannot answer "which of my open tickets touched a client whose contract burn-down is over budget this month" without exporting two CSVs and joining them in Excel.

*Weekly ritual:* Monday morning queue cleanse — find every ticket stale > 7 days in "Awaiting Customer Reply," nudge with templated action or close, then rebalance load across the four techs on shift. Friday afternoon SLA review — pull breach risk for the next 24 hours and pre-emptively reassign.

*Frustration:* The HaloPSA UI cannot show "agent load + stale tickets + SLA risk" in one view. She bounces between three saved views and a spreadsheet. Bulk-closing 30 stale tickets requires 30 clicks each (or a saved view + multi-select that frequently times out).

**Persona 2: Devon, MSP technician on the floor**

*Today (without this CLI):* Devon lives inside one ticket at a time. To service a client call he keeps the ticket open, a second tab on the client's asset list, a third on the client's contract, and a fourth on the KB. He copies the asset tag into the ticket by hand, searches KB by re-typing keywords, and writes time entries at end-of-day from memory because logging them per-ticket interrupts the call.

*Weekly ritual:* Daily — own 8-15 tickets, touch each twice, log time, attach the right KB article when a known fix applies. Friday — reconcile his timesheet against the tickets he actually worked.

*Frustration:* Switching context between ticket, asset, contract, KB is the entire job and is entirely manual. Time entry reconciliation at week's end is "what did I forget" archaeology — he knows tickets he touched that have no time on them but finding them takes 20 minutes of clicking.

**Persona 3: Priya, MSP ops/admin (runs billing prep + workflow audits)**

*Today (without this CLI):* Priya pulls reports from Halo Reports into CSV, opens them in Excel, builds pivot tables to answer "billable hours per client this week," "which contracts are over their monthly hour bank," and "which workflows fired most this month." She also reviews ticket rules quarterly — clicks through each rule's UI to see what it does because there's no export.

*Weekly ritual:* Monday — generate the prior-week billing prep export per client. Mid-week — investigate any client whose contract hours are pacing over. Quarterly — audit ticket rules and workflows.

*Frustration:* Halo Reports gets her 70% of the way there but she always needs one more join — billable time × contract bank × client × agent. The native reports don't let her cross those entities cleanly. Ticket-rule audits are screen-by-screen UI clicks; she'd kill for a single text dump.

**Persona 4: Sam, Internal IT lead on HaloITSM (same API, different tenant)**

*Today (without this CLI):* Sam runs a 200-employee internal helpdesk. He's the only admin. He uses the HaloITSM UI for triage and a homegrown PowerShell script to nightly-dump tickets-by-status to a SharePoint folder for leadership. His script breaks every time Halo changes a field.

*Weekly ritual:* Daily queue check, weekly leadership report (open vs closed by category, SLA hit rate), monthly review of automation rules.

*Frustration:* The PowerShell script. He wants `halopsa sync && halopsa sql "..."` and to be done with the bespoke ETL.

## Candidates (pre-cut)

(See full subagent response above; 18 candidates generated; C10/C13/C14/C15/C17 pre-cut.)

## Survivors and kills

### Survivors (final transcendence list — all >= 6/10)

| # | Feature | Command | Score | How It Works | Persona | Evidence |
|---|---------|---------|-------|-------------|---------|----------|
| 1 | Triage queue | `triage [--team T] [--agent A]` | 10/10 | Joins local tickets × agents × statuses in SQLite to render per-agent load, stale count, 24h breach count in one table | Maria | Brief Top Workflow #1, Build Priority #3 |
| 2 | Bulk age-out closer | `tickets age-out --status "X" --stale-days N [--apply]` | 9/10 | Local SQLite filter on status_id + lastactiondate; --apply issues batch close + templated action | Maria | Brief Top Workflow #2; absorb #6/#10 |
| 3 | SLA breach radar | `sla breaching --within 24h` | 9/10 | Selects local tickets.targetdate BETWEEN now AND now+N, joins agent+client, sorts by time-to-breach | Maria | Build Priority #3; spec field targetdate |
| 4 | Agent workload | `agents load [--team T]` | 8/10 | Joins tickets × actions × time_entries × agents for open/touched-this-week/billable/oldest | Maria, Priya | Build Priority #3 |
| 5 | Client desk card | `clients card <id-or-name>` | 10/10 | 6-table cross-join: clients + sites + tickets + contracts + assets + kb_links | Devon, Priya | Brief Top Workflow #3; Build Priority #3 |
| 6 | Asset ticket history | `assets history <tag-or-id>` | 7/10 | Filters tickets by asset_id chronologically with agent + time per ticket | Devon | Brief Top Workflow #3; CMDB pattern |
| 7 | KB suggest for ticket | `kb suggest --ticket <id>` | 9/10 | FTS5 against kb_articles using ticket.summary + details + last action body | Devon | Brief Top Workflow #5; absorb #30-32 |
| 8 | Time gap finder | `time gaps --agent me --week current` | 8/10 | Set-diff: tickets actioned by agent this week MINUS tickets with time_entries this week | Devon | Brief Top Workflow #4; absorb #44-45 |
| 9 | Contract burn-down | `contracts burn [--client X]` | 9/10 | Sum time_entries.hours WHERE billable AND client AND period vs contracts.hours_bank | Priya | Brief Top Workflow #4; absorb #24 |
| 10 | Ticket rules dump | `rules dump [--workflow W]` | 6/10 | Reads ticket_rules + workflows + statuses, prints each rule's conditions→actions as text | Priya | Brief Top Workflow #6; absorb #33-40 |
| 11 | Changed-since | `tickets changed-since <when> [--mine]` | 9/10 | Queries local store for tickets/actions lastupdated >= when, groups by ticket | Maria, Sam | Brief Data Layer (cursor); Sam persona |
| 12 | Standup digest | `standup --team T --since yesterday` | 7/10 | Aggregates closed/reopened/time-logged/top-client per agent for window | Maria | Maria daily standup; Build Priority #3 |
| 13 | Multi-client overlay | `clients overlay --metric open_tickets --top 10` | 8/10 | Group-by + rank on clients joined to chosen metric (tickets/time/contracts) | Priya | Brief Data Layer (MSP shape); Priya persona |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Billable hours export | Thin wrapper over `time list --billable --csv` (already absorbed) | Contract burn-down |
| Recurring-pattern finder | NLP-shaped; mechanical fallback brittle on ticket text | Asset history |
| Doctor strict | Already absorbed (#57/#63) | — |
| Action templates apply | Halo has canned text (#47); thin local rewrap | Bulk age-out |
| Tickets watch mode | Scope creep to long-running TUI/daemon | Changed-since |

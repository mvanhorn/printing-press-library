# HubSpot CLI Brief

## API Identity
- **Domain:** HubSpot Sales Hub — CRM + Engagements + Pipelines + Lists. Scope deliberately restricted to Sales (no CMS, no Marketing, no Conversations/Inbox) per the user's stated use.
- **Users:** Sales-ops, AEs/SDRs, CRM admins, and AI agents driving prospect nurture (Damien's "Titans" nurture campaign and meeting prep are the immediate consumers).
- **Data profile:** Object graph — Contacts ↔ Companies ↔ Deals ↔ Owners, with Engagements (Calls/Emails/Meetings/Notes/Tasks) attached via v4 Associations. Custom Properties on every object. Lists (v3) for segmentation. Bulk Imports for CSV-driven mutations. All objects have `lastmodifieddate` / `hs_lastmodifieddate` and stable IDs — clean candidates for a local SQLite mirror with delta sync. Every standard CRM read endpoint accepts `propertiesWithHistory=<comma-list>` and returns per-property snapshots `{value, timestamp, sourceType, sourceId}` — the foundation for the property-history wedge added in this reprint.

## Reachability Risk
- **None.** Probed `GET /crm/v3/owners?limit=1` with the working private-app token → HTTP 200 in 0.27s. Anonymous probe → 401 (expected). GitHub spec collection at `HubSpot/HubSpot-public-api-spec-collection` → HTTP 200. No recent GitHub-issue chatter about 403s or rate-limit cliffs.
- **API versioning churn (2026-03):** HubSpot rolled out date-based versioning paths (`/crm/objects/2026-03/<type>`) alongside the existing `/crm/v3/<type>` surface. **v3 paths continue to work; the end-of-life date for v3 has not been announced.** We stay on v3 for this reprint to match the GitHub spec collection (still v3-shaped). One v1 surface DID sunset: Contact Lists v1 → HTTP 404 after 2026-04-30. We use Lists v3 (already in the absorb manifest); no migration work needed.

## Spec Sources
Canonical: `https://github.com/HubSpot/HubSpot-public-api-spec-collection`. Per-object JSON specs under `PublicApiSpecs/CRM/<Object>/Rollouts/<num>/v3/<slug>.json`. No aggregated index spec — Printing Press's multi-`--spec` merge will compose the surface from these:

| Object | Spec URL |
|---|---|
| Contacts | `…/Contacts/Rollouts/107729/v3/contacts.json` |
| Companies | `…/Companies/Rollouts/424/v3/companies.json` |
| Deals | `…/Deals/Rollouts/424/v3/deals.json` |
| Leads | `…/Leads/Rollouts/424/v3/leads.json` |
| Line Items | `…/Line%20Items/Rollouts/424/v3/lineItems.json` |
| Products | `…/Products/Rollouts/424/v3/products.json` |
| Quotes | `…/Quotes/Rollouts/424/v3/quotes.json` |
| Calls | `…/Calls/Rollouts/424/v3/calls.json` |
| Emails | `…/Emails/Rollouts/424/v3/emails.json` |
| Meetings | `…/Meetings/Rollouts/424/v3/meetings.json` |
| Notes | `…/Notes/Rollouts/424/v3/notes.json` |
| Tasks | `…/Tasks/Rollouts/424/v3/tasks.json` |
| Crm Owners | `…/Crm%20Owners/Rollouts/146888/v3/crmOwners.json` |
| Pipelines | `…/Pipelines/Rollouts/145896/v3/pipelines.json` |
| Associations (v4) | `…/Associations/Rollouts/130902/v4/associations.json` |
| Lists | `…/Lists/Rollouts/144891/v3/lists.json` |
| Imports | `…/Imports/Rollouts/144903/v3/imports.json` |
| Properties | `…/Properties/Rollouts/145899/v3/properties.json` |
| Objects | `…/Objects/Rollouts/424/v3/objects.json` |

**Out of scope for v1** (per user directive + tier reality): Tickets (Service Hub), Tickets-adjacent feedback, all Commerce Hub (Carts/Orders/Invoices/Payments/Contracts/Subscriptions/etc.), all CMS, all Marketing, all Conversations. Sales-Hub-Enterprise features (Forecasts, GoalTargets, DealSplits) deferred to v1.1.

## Users

Concrete personas the CLI must serve. Each has a distinct cadence, a primary loop, and a clear set of commands they reach for.

- **Sales rep (AE / SDR).** Lives in HubSpot daily, runs prospect outreach, logs calls and meetings. Needs the daily "who do I call today" answer, fast outcome logging, and a personal stale-prospect list. Primary commands: `nurture queue`, `nurture-mine`, `tasks mine`, `stale contacts --owner me`, `engagements log call`, `meetings list --owner me`. Token-light agent surface matters — they'll often invoke the CLI through an agent inside an IDE or chat client, not the binary directly.
- **Sales-ops lead.** Owns the CRM hygiene, the weekly pipeline review deck, and the CSV bulk-update flow that fixes the rep mistakes from the prior week. Primary commands: `owner-load`, `pipeline-health`, `contacts bulk-update --from-csv`, `properties list`, `deals stage --to`, `since 7d`. Cares more about correctness than speed — pre-flight validation is the killer feature for them.
- **RevOps analyst.** Owns reporting and ad-hoc questions from the GM. Lives in `sql` and `search`. Needs `meetings ever-had` and `meetings status-report` for the monthly customer-driven reports that HubSpot's standard search API cannot answer. Primary commands: `sql`, `search`, `meetings status-report`, `deals top`, `notes signals`, `since 30d --types deals,engagements`.
- **AI agent.** Acting on behalf of any of the above. Reads through MCP, calls every command with `--agent --json --select` to keep context tight, expects deterministic exit codes and a typed manifest. Reaches for `nurture queue --agent`, `meetings history <id> --agent`, `engagements of contact:<id> --agent`, `since 24h --agent`. The only persona for whom `mcp:read-only: true` annotations are load-bearing safety; for the others they're invisible.

## Top Workflows

1. **Daily nurture list — sales rep.** `nurture queue --owner me --top 20 --agent` produces today's prioritized contacts with stale-days × deal $ × stage probability columns. The agent layer in `/nurture today` consumes this directly; the rep can read the same output in their morning standup.
2. **Weekly pipeline review — sales-ops lead.** `owner-load --pipeline default --csv` + `pipeline-health default --idle-days 14 --json` produce the Monday morning deck. One run replaces the Deal Owner report + Sheets pivot the web UI requires.
3. **Stale-deal detection — sales rep + sales-ops.** `stale deals --days 21 --owner me` and `stale contacts --days 30` find what's gone cold without any live API quota burn after one sync.
4. **Meeting outcome reporting — RevOps analyst (NEW; customer-driven).** `meetings status-report --status scheduled --month 2026-04` answers "every meeting EVER marked Scheduled in April, even if it later flipped to No Show or Completed." Backed by sync-time `propertiesWithHistory` capture; HubSpot's standard search API physically cannot do this.
5. **CSV bulk update — sales-ops lead.** `contacts bulk-update --from-csv people.csv --map email=Email,lifecyclestage=Stage --dry-run` validates against the local properties schema, then dispatches valid rows through batch endpoints. Pre-flight error report instead of HubSpot Imports' silent row drops.
6. **Monthly attribution rollup — RevOps analyst.** `deals top --top 20 --pipeline default --json` + `notes signals --pipeline default --since 30d --json` feed a signal-weighted top-deals view for the monthly Compounding/Servosity revenue review.
7. **Cross-object timeline view — sales rep + AI agent.** `engagements of contact:<id> --since 30d --json` and `engagements of deal:<id>` return the unified call/email/meeting/note/task timeline in one query — replaces N+1 round-trips to HubSpot's per-engagement endpoints.

## Data Layer
- **Primary entities:** `contacts`, `companies`, `deals`, `leads`, `line_items`, `products`, `quotes`, `owners`, `pipelines`, `pipeline_stages`, `lists`, `properties`, `associations`, plus engagement tables `calls`, `emails`, `meetings`, `notes`, `tasks`.
- **NEW (this reprint): `hubspot_property_history`** — single shared snapshot table with shape `(object_type TEXT, object_id TEXT, property TEXT, value TEXT, timestamp DATETIME, source_type TEXT, source_id TEXT, PRIMARY KEY(object_type, object_id, property, timestamp))`. Populated by sync paths that pass `--with-history <props>`; the composite PK makes re-sync idempotent. One table covers meetings + deals + contacts + companies; meetings commands ship in this reprint, the per-object `ever-had` commands for the other three are a follow-up PR.
- **Sync cursor:** `hs_lastmodifieddate` per object via the v3 `/crm/v3/objects/{type}/search` endpoint with `filterGroups[].filters[]` on modified time + cursor token. Lists API uses v3 list IDs + `updatedAt`. When `--with-history` is set, sync switches the per-object read leg to GET (since search doesn't accept `propertiesWithHistory`) and writes both the current property snapshot AND the full history block to `hubspot_property_history` in the same transaction.
- **FTS/search:** SQLite FTS5 over `contacts(firstname, lastname, email, company)`, `companies(name, domain, description)`, `deals(dealname, description)`, `notes(hs_note_body)`. Per-object FTS plus a global `resources_fts`.
- **Graph table:** `associations(from_type, from_id, to_type, to_id, label, association_type_id)` — the only way to answer "which engagements touched deal X" without N+1 round trips.

## Codebase Intelligence
- **Official Node SDK** (`@hubspot/api-client`) parity target: modules under `crm.{contacts,companies,deals,tickets,lineItems,products,quotes,leads,objects}.{basicApi,searchApi,batchApi,associationsApi}` + `engagements.{calls,emails,meetings,notes,tasks}.*` + `associations.v4.*` + `owners.ownersApi` + `properties.{coreApi,groupsApi,batchApi}` + `pipelines.{pipelinesApi,pipelineStagesApi}` + `lists.listsApi` + `imports.coreApi`. Generic escape: `client.apiRequest()`.
- **Auth:** `Authorization: Bearer pat-…`. Env-var convention across the ecosystem is `HUBSPOT_ACCESS_TOKEN` (shinzo, peakmojo, dipankar all use it). We MUST adopt it to avoid friction.
- **Rate limiting:** 110 req / 10s for OAuth apps; Private Apps share daily quota with documented 429s. Batch endpoints accept up to 100 records/call — the documented escape hatch.
- **Architecture insight:** HubSpot's v3 search API is the only path with `hs_lastmodifieddate` cursor filtering. Plain `GET /crm/v3/objects/{type}` paginates by `after` but doesn't support modified-after filters. Sync MUST use search for delta detection. The `propertiesWithHistory` parameter is supported only on the GET surface, not on `/search`, so the with-history sync path uses search to find changed IDs then GETs each one for the history block. This is the price of the wedge; it's bounded because `--with-history` is opt-in per sync invocation.

## User Vision

This reprint targets a library re-submission. The prior CLI was removed from mvanhorn/printing-press-library on 2026-05-07 (PR #269, no documented reason). A separate fresh attempt (PR #549, 2026-05-23) was closed for two concrete reviewer blockers — committed binary in repo, and a Closed-Lost probability bug in pipeline-health.

Add ONE customer-driven feature beyond the prior novel set: **property change history for meetings** (plumbed to extend to deals/contacts/companies). A Servosity customer needs a monthly report of meetings that were EVER in outcome "Scheduled" — even after they flip to "No Show" or "Completed". HubSpot's standard search/filter API cannot answer this; only the property-history surface (`?propertiesWithHistory=...` parameter on standard reads) can.

The wedge:
- Sync accepts `--with-history <props>` and persists the `propertiesWithHistory` block into a NEW generic `hubspot_property_history(object_type, object_id, property, value, timestamp, source, source_id)` table (composite PK for idempotent re-sync).
- Three novel meetings commands read from that table: `meetings history <id>`, `meetings ever-had --property <p> --value <v> --from <date> --to <date>`, and `meetings status-report --status scheduled --month YYYY-MM`. All `mcp:read-only: true`.
- Plumb `--with-history` into deals/contacts/companies sync paths too (same shared table), defer their `<object> ever-had` commands to a follow-up PR.

Quality constraints from the prior PR #549 failure:
- No committed binary in the final artifact.
- Pipeline-health / weighting math must use real stage probabilities (Closed Lost = 0, not 0.5).
- The hand-authored skill at `~/.claude/skills/hubspot-pp-cli/SKILL.md` (8.6KB) carries discovery copy worth preserving in the printed SKILL.md.

## Product Thesis
- **Name:** `hubspot-pp-cli` (slug: `hubspot`). README banner: "Every HubSpot Sales Hub feature, plus offline cross-object queries, property-change-history reporting, and an agent-native data layer no other HubSpot tool has."
- **Why it should exist:**
  1. **No name collision with the official `hs` CLI** — HubSpot's own `hubspot-cli` is CMS/developer-platform only. Searching "hubspot cli" lands on a tool that cannot list a contact. The Sales/CRM CLI niche is structurally vacant.
  2. **Local SQLite mirror with `since=` deltas** — every incumbent MCP requires a live API call per query. `peakmojo` has minimal vector caching only. We are the only tool that answers "what changed since I last looked" instantly + offline.
  3. **Property-change-history reporting (NEW)** — every incumbent CLI/MCP exposes the current property value; none retain snapshots across syncs in a queryable shape. `meetings ever-had` and `meetings status-report` answer customer questions no other tool can.
  4. **Cross-object joins** — owner load, deal-engagement coverage, stale-prospect detection require joining 3+ tables. SDKs expose modules in isolation; the web UI shows it but is unscriptable. We make these one-shot CLI queries.
  5. **Bulk-update-from-CSV with schema validation** — sales-ops's daily pain. No incumbent has it.

## Build Priorities
1. **P0 (foundation):** Auth + config + client + 17-spec generation + SQLite store + sync command + FTS search + global `--json` / `--select` / `--csv`.
2. **P1 (absorb):** Per-object CRUD (basic + batch + search + associations) for all 17 spec resources — match `@hubspot/api-client` module surface 1:1 (this is the absorb-the-incumbents priority). Plus `owners list`, `pipelines list`, `properties list/show`, `lists members`.
3. **P2 (transcend, the killer features):**
   - **Property change history (NEW wedge):** `hubspot_property_history` table; `sync --with-history <props>` capture (meetings + deals + contacts + companies); `meetings history <id>`, `meetings ever-had`, `meetings status-report` read commands.
   - `stale` — contacts/deals with no engagement in N days, cross-table.
   - `owner-load` — open deals per rep per stage with $ totals.
   - `pipeline-health` — stuck deals + slip risk per stage. **Closed Lost probability MUST be 0, not 0.5** (PR #549 review fix).
   - `engagements of <contact|deal>` — unified engagement timeline via v4 associations.
   - `since <duration>` — what changed across all synced objects.
   - `contacts bulk-update --from-csv` — schema-validated batch property writes.
   - `deals stage <id> --to <stage> --note <text>` — atomic stage move + engagement log.
   - `nurture-mine` and `nurture queue` — the agent-loop priority list.
   - `notes signals` and `deals top` — signal-weighted ranking from real pipeline-review patterns.
4. **P3 (polish):** auth `doctor` with token-scope inspection; `auth login` setup wizard; rate-limit-aware adaptive client; reseat MCP tools for the largest pieces; agent recipes for the AI Group's nurture loop; carry forward 8.6KB of discovery copy from `~/.claude/skills/hubspot-pp-cli/SKILL.md` into the printed SKILL.md (generic-ified).

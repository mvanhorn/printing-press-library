# EU TED CLI Brief

## API Identity
- Domain: EU public procurement — TED (Tenders Electronic Daily), the official EU journal supplement for public procurement notices. Operated by the Publications Office of the EU.
- Users: Procurement officers, business development / bid managers, policy researchers, public spend analysts, compliance teams, government agencies, data journalists, eProcurement platform integrators
- Data profile: ~676,000 notices/year, €815B annual EU public spend, notices from EU27 + EEA, full data from 2007 onward. Single POST search endpoint with 1,830 queryable fields, expert query language, no auth required.

## Reachability Risk
- **None** — API is live, public, no auth required. Confirmed working: `https://api.ted.europa.eu/v3/notices/search` returns 200 with JSON. No reported 403/blocked issues in community repos.

## Top Workflows
1. **Sector + Country + Deadline sweep**: Find open contracts in a country/sector closing within N days — the core use case for business development teams scanning for bid opportunities.
2. **Award intelligence**: Who won contracts for CPV 72 (IT) in Germany last quarter? Maps buyer-winner relationships for competitor intelligence and market sizing.
3. **Buyer profiling**: What has a specific contracting authority published in the last 2 years? Procurement history to inform bid strategy and relationship mapping.
4. **Bulk sync for analytics**: Pull all notices matching a saved query into local SQLite, then run SQL aggregations for spend analysis, trend reports, and dashboards.
5. **Daily new-notice monitoring**: Watch specific keywords/CPV codes/countries and surface new notices since last check — the "morning digest" workflow for bid managers.

## Table Stakes
- Competing SaaS tools (TenderMetric, TenderAlerts, Tenderlake, Tenderbase, Spend Network): saved searches, email alerts, CPV filtering, deadline visibility, CSV export
- fbuchner/ted-mcp: structured search, multilingual field resolution, winner-contract pairing
- tap-eu-ted (Singer tap): full incremental sync, state-tracked replication, 15K+ notice pagination

## Data Layer
- Primary entities: Notice (publication_number, notice_type, publication_date, deadline, buyer_name, buyer_country, estimated_value, cpv_code, procedure_type, url)
- Secondary: CPVCode (code, description, section, division, group, class), BuyerProfile (buyer_name, buyer_country, notice_count, total_value), Award (winner_name, winner_country, contract_value)
- Sync cursor: `publication_date` + `publication_number` (incremental by date)
- FTS/search: Full-text on title, buyer name, CPV description; SQLite FTS5

## Codebase Intelligence
- Source: fbuchner/ted-mcp source code analysis
- Auth: None required
- Data model: Notice has `notices[]` array; each notice is a map of field-name → value(s). Multilingual fields return `{"eng": [...], "deu": [...]}` — must resolve to preferred language.
- Rate limiting: 30s timeout documented; partial results flagged with `timedOut: true`. No published rate limit policy.
- Architecture: Single POST endpoint with expert query DSL. Two pagination modes: PAGE_NUMBER (up to 15K) and ITERATION (unlimited via opaque token). Field selection is explicit — caller must request fields by name.

## Product Thesis
- Name: eu-ted-pp-cli
- Why it should exist: TED's web UI is slow and limited. Existing SaaS tools cost €500-2000/month. The API is free and powerful but requires expert query knowledge and custom pagination code. A CLI gives any developer, researcher, or bid manager a composable, scriptable, offline-capable tool for the world's largest public procurement dataset — for free, with SQLite, jq-compatible output, and novel features no existing tool has.

## Build Priorities
1. **`notices search`** — expert query + pagination + field selection + JSON/table output (core command)
2. **`notices get`** — lookup single notice by publication-number
3. **`sync`** — pull notices matching a query into local SQLite, incremental by date
4. **`cpv`** — CPV code lookup (code → description, description → code, list section/division/group)
5. **`alerts`** — define saved queries + notify on new matches (file-based, runs as daemon or via cron)
6. **`deadline`** — pre-configured search scoped to notices with submission deadlines within N days
7. **`awards`** — pre-configured search for contract award notices (CANs) with winner data
8. **`buyer`** — profile a contracting authority: history, top CPV codes, spend volume

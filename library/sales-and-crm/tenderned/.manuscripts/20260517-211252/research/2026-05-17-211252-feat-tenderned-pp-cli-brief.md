# TenderNed CLI Brief

## API Identity
- **Domain:** Dutch national public procurement. TenderNed is the central platform run by PIANOo (Ministerie van EZK) where every Dutch contracting authority publishes tender notices. All above-threshold notices are forwarded to EU TED; sub-threshold notices appear only on TenderNed.
- **Users:** Dutch suppliers monitoring tender opportunities, market analysts tracking procurement trends, journalists/researchers, public-sector compliance officers, government bodies tracking their own publications, B2B lead-gen platforms.
- **Data profile:** 143,717 publications in the live archive (May 2026). Each publication carries rich structured metadata: title, description, contracting authority, CPV codes (multiple), NUTS region codes, procedure type, contract type (works/supplies/services), publication & closing dates, status flags, eForms code (EF05/EF16/EF17/EF20/EF25/EF29/EF30/EFE3 ...). Attached documents (bestek, PvE, evaluation criteria, Q&A) are downloadable. License: CC-0 public domain.

## Reachability Risk
- **None.** Live HTTP probe of `/publicaties?page=0&size=2` returned 200 with valid JSON. Filters tested: search keyword, publicatieType, sluitings/publicatie date ranges. License confirmed CC-0 on data.overheid.nl.

## API Surface (discovered)
**Two-part API.** TNS publication webservice (JSON, no auth) covers everything except the full eForms XML payload, which lives behind Basic Auth.

| Endpoint | Auth | Purpose |
|---|---|---|
| `GET /publicaties` | none | List/search/filter publications (paginated). Filters: `search`, `publicatieType`, `cpvCodes` (8-digit-dash form), `typeOpdracht`, `procedure`, `nationaalOfEuropees`, `publicatieDatumVanaf/Tot`, `sluitingsDatumVanaf/Tot`, `aanbestedendeDienstId`, `typeAanbestedendeDienst`, `aardVanDeOpdracht`. |
| `GET /publicaties/{id}` | none | Full structured metadata for one publication |
| `GET /publicaties/{id}/documenten` | none | List attached documents (bestek/PvE/etc.) with size + virus indicator |
| `GET /publicaties/{id}/documenten/{docId}/content` | none | Download single document (binary) |
| `GET /publicaties/{id}/documenten/zip` | none | Download all docs as zip |
| `GET /aanbestedendediensten` | none | List contracting authorities (2,699 total) |
| `GET /aanbestedendediensten/{id}` | none | One contracting authority detail |
| `GET /rss/laatste-publicatie.rss` | none | Atom feed of newest publications |
| `GET /publicaties/{id}/public-xml` | basicAuth | Full eForms XML (the only authenticated endpoint) |

Base URL: `https://www.tenderned.nl/papi/tenderned-rs-tns/v2`. Auth required: HTTP Basic, request creds via functioneelbeheer@tenderned.nl.

## Top Workflows
1. **Discover new opportunities by keyword + CPV + date** — `tenderned search "spoor" --cpv 45000000-7 --since 2026-04-01 --status open` returns matching open tenders; supplier bidding pipeline.
2. **Track one contracting authority** — `tenderned buyer dossier "Gemeente Rotterdam"` returns spending cadence, top CPVs, recent awards. Used by suppliers planning relationship work.
3. **Below-threshold long-tail discovery** — `tenderned search --national --max-value 200000 --cpv 63712700` finds national-only notices that never reach TED. This is the entire reason TenderNed beats TED for NL.
4. **Document download for bid prep** — `tenderned docs download 270869` pulls all attached tender documents into a local folder for analysis.
5. **Cross-reference with EU TED** — `tenderned ted-link 270869` returns the matching TED publication number so users can pivot to the `eu-tenders` CLI for EU-wide comparison.

## Table Stakes (incumbent feature parity)
The only competing tools are paid scrapers (Apify) and the official TenderNed web UI. Both offer:
- Full-text search across title/description
- CPV-code filter (single or multi)
- Date-range filter (publication + closing)
- Publication type filter (open/awarded/PIN/modification)
- Contract-type filter (services/supplies/works)
- Procedure filter (open/restricted/negotiated)
- Pagination with deep-page access
- Document download
- Per-buyer drill-down

## Data Layer
- **Primary entities:** publications, documents, contracting authorities (aanbestedendediensten)
- **Sync cursor:** `publicatieDatum` descending — every publication carries a publication timestamp, so an incremental sync queries `publicatieDatumVanaf=<last_seen>` and walks forward.
- **FTS/search:** FTS5 over title (`aanbestedingNaam`), description (`opdrachtBeschrijving`), trefwoord1, trefwoord2, buyer name (`opdrachtgeverNaam`).
- **Computed columns:** indexed `cpvCodes[*].code`, `nutsCodes[*].code`, `procedureCode`, `typeOpdrachtCode`, `aankondigingCode`, `isGegund`, `nationaalOfEuropeesCode`, `publicatieDatum`, `sluitingsDatum`. Lets the CLI run rich offline queries.

## Codebase Intelligence
- No SDK exists. The only public code is `tenderned-analyse/code-example-tn-xml-api` — a Python example demonstrating Basic-auth retrieval of one XML doc via the `requests` library.
- Greenfield: no CLI, no MCP server, no MCP host integration, no Go module exists. This CLI would be the first.

## Source Priority
- Single source (TenderNed). No combo CLI gates.

## Product Thesis
- **Name:** tenderned (binary: `tenderned-pp-cli`)
- **Why it should exist:** Every Dutch supplier monitoring tenders today does one of three things: (a) opens tenderned.nl in a browser and clicks through search filters, (b) pays for Mercell/Negometrix/Apify, or (c) writes ad-hoc Python against the JSON endpoint. None of those give you a local SQLite snapshot, agent-native JSON output, FTS-indexed offline search, or composable CLI commands. The above-threshold subset reaches `eu-tenders` (via TED), but the sub-threshold long tail — €40k–€200k contracts that make up the majority of municipal procurement — only exists on TenderNed.

## Build Priorities
1. **Foundation:** Internal YAML spec (TenderNed has no full OpenAPI), local SQLite store for publications + documents + aanbestedende diensten with FTS, sync command with incremental cursor.
2. **Absorb:** Mirror every filter the web UI offers (search, CPV, type, procedure, dates, buyer, national/EU); list/get/download for publications and documents; aanbestedende dienst lookup; XML fetch (auth'd).
3. **Transcend:** Compound features only the local store enables — buyer dossier, CPV-drift over time, closing-deadline heatmap, sub-threshold lead-gen (filter for sub-EU-threshold national notices in a CPV range), TED cross-reference via the TED publication number embedded in eForms XML, watch (alert on new matches for a saved query), RSS-to-SQLite live ingest.

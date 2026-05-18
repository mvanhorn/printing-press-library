# Miami-Dade Clerk Official Records CLI Brief

**Generated:** 2026-05-17
**Source:** Empirical recon performed earlier this session via Playwright MCP against the live portal at `https://onlineservices.miamidadeclerk.gov/officialrecords`. No OpenAPI spec exists for this portal; the API was reverse-engineered.

## API Identity

- **Domain:** Florida public-records search (Miami-Dade County government). Recorded documents: deeds, mortgages, satisfactions, assignments, lis pendens, judgments, federal tax liens, lien releases, court papers, easements, plats, restrictions.
- **Users:** Title insurers, foreclosure investors, real-estate attorneys, due-diligence researchers, journalists, and county residents tracking liens on their own property. Primary persona for this CLI: **foreclosure investor** (BlueStone Capital and similar) who needs the full lien chain before bidding at auction.
- **Data profile:** ~50 years of recorded documents (1967–present, 5/11/2026 cutoff per portal banner). Estimated ~10M records county-wide. Per-search 500-row cap. Records keyed by `cfn_master_id` (Clerk File master ID).

## Reachability Risk

- **Medium-Low.** The portal is publicly accessible without authentication, protected by reCAPTCHA Enterprise v3 (invisible scoring). The site banner notes "exceeding reCAPTCHA Enterprise free quota" but tokens still validate successfully — appears to be a Google billing notice, not a functional block.
- Empirical evidence: 7 successful searches executed during recon (3 Property/Condo, 4 Name) — no challenges fired, no rate-limit responses.
- Per-token use is single-use. Each API call requires a fresh `x-recaptcha-token` header obtained from a freshly-loaded page. This drives the runtime architecture.
- `probe-reachability` will likely return `browser_clearance_http` (Phase 1.9). The printed CLI will need Surf + Chrome cookie capture for live calls.

## Top Workflows

1. **Search by property address → get every deed ever recorded.** Property/Condo mode returns only deeds (DEE/QCD/DAM/DM/ODE). Empirical: `"5600 W 13 AVE"` → 14 records, all deeds, 1995-2022. Address normalization rules: strip ordinals (170th → 170), uppercase, condo unit from PA legal_description (UNIT B312).
2. **Search by owner name → get every mortgage / lis pendens / lien / satisfaction.** Name + DocType mode returns name-indexed docs. Empirical: `"HERRIOTT NATHANIEL"` → 55 records across 30+ doc types. `"DEOD INVESTMENT LLC"` → 15 records: 1 DEED + 1 MOR + 1 ASG + 9 LIE + 1 LIS + 2 DCP.
3. **Lien chain reconstruction for a folio.** Compose Property/Condo (get deeds → extract chain of title) + per-owner Name searches (get mortgages/sats/liens) → fuzzy-match back to target property via subdivision + plat_book/page + block_no signature. This is the killer feature; no existing tool does it.
4. **Owner portfolio scan.** Single Name search returns every property an owner controls plus every mortgage/lien they've taken. Lead-gen + risk signal.
5. **Surviving-lien calculator.** Read the chain, filter to liens NOT released (no matching SAT/REL by `linK_DOCTYPE`), compute Max Safe Bid. Critical for tax deed buyers — Federal Tax Liens (FTL) survive TD foreclosure.

## Table Stakes

This portal has **no public API documentation, no SDK wrappers, no MCP servers, no community CLIs**. (Confirmed by absence in Phase 1.5a search; county clerk portals don't have ecosystems.)

The "table stakes" baseline therefore reduces to: **the portal's own web UI**, which offers 5 search modes (Name+DocType, Clerk's File #, Book/Page, Legal Description, Property/Condo) and a viewer that displays scanned PDFs. Our CLI must:
- Match all 5 search modes (3 implemented in MVP: Name+DocType, Property/Condo, Book/Page)
- Match the portal's filter set (doc type code, date range)
- Match the result table (53 fields per record)
- Provide the document viewer URL for each record
- Beat the portal at: pagination beyond 500 rows (auto-narrow by doc type or year window), local cache (never re-query the same instrument), structured JSON output, batch lookups (multiple folios in one invocation), agent-native flags (--json, --select, --csv, --quiet).

## Data Layer

- **Primary entities:** `recording` (one row per `cfn_master_id`, normalized — multi-party API rows merged into one record with a `parties` jsonb array).
- **Sync cursor:** `cfn_master_id` (monotonically increasing). Resume sync by fetching records with `cfn_master_id > last_seen`. No native cursor in the API; we get this from the response and persist locally.
- **FTS/search:**
  - `recordings_fts` (FTS5 over `first_party + second_party + legal_description + subdivision_name + address + case_number`) — supports fast offline party-name and legal-description search.
  - `recordings_by_folio` (covering index on `folio_number` where not null) — fast property lookups.
  - `recordings_by_signature` (covering index on `subdivision_name + plat_book + plat_page + block_no` where folio_number is null) — fast composite property-link search for name-indexed records.

## Codebase Intelligence

No GitHub repos exist for this portal. DeepWiki, source-code analysis, and SDK reading not applicable.

The closest analog is `reteps/redfin-pp-cli` (in spirit) — a CLI for an undocumented website with anti-bot protection. That CLI ships browser-clearance HTTP transport via Surf. We will mirror that pattern.

## User Vision (briefing context, 2026-05-17)

From the user (BlueStone Capital founder) at session start:
- "Use the empirical recon I just did" — Recon captured: two-step qs-token API flow, 53-field response schema, 30+ doc-type codes, dual-index behavior (Property mode = deeds only; Name mode = everything else), address normalization rules, 4-layer fallback design for properties not in the address index, composite property-link signature for name-search records lacking folios.
- Implicit vision (from broader project context): the CLI is **Phase 1 of a larger integration**. The final shape of the tool is determined by what flows through the v3 frontend's Title tab + AI tab + pipeline card badges. The CLI must produce JSON that's directly consumable by a downstream Supabase ingest pipeline (every record has a stable `cfn_master_id`, a viewer URL, money in cents, dates ISO-8601).

## Source Priority

Single-source CLI. No combo-CLI priority concerns.

## Product Thesis

- **Name:** `miami-dade-clerk-pp-cli`
- **Why it should exist:** Foreclosure underwriting today sees the active judgment + opening bid, nothing else. Title companies charge $150-300 per O&E report. This CLI gives you the full chain of title + every active lien + every prior foreclosure for $0 of marginal cost per lookup, in under 30 seconds, in JSON, ready to feed an AI underwriter. It is the only tool in existence that pivots property → owners → bounded name searches → filtered-back-to-property timeline.

## Build Priorities

1. **Priority 0 (data layer):** SQLite store with `recordings` table + FTS5 + indexes. Sync command.
2. **Priority 1 (absorbed primitives):** `search-property`, `search-name`, `search-book-page`, plus `doctor`, `health`, `version`.
3. **Priority 2 (transcendence):** `lien-chain` (the killer), `owner-portfolio`, `surviving-liens`, `chain-of-title`.
4. **Priority 3 (polish):** PDF download (`download <cfn_master_id>`), batch invocations (`--folio-list folios.csv`), agent-native `--select` for all commands.

## Build constraints

- Public records, no auth → no API key gate work
- reCAPTCHA Enterprise v3 → runtime needs `auth login --chrome` style cookie capture (decided in Phase 1.9 via `probe-reachability`)
- Single-use tokens → must reload page (or capture fresh token) between calls; throttle ~10 req/min
- 500-row hard cap per response → auto-narrow by doc_type_code or 5-year window when hitting cap
- Money in dollars in the API, cents in our DB and JSON output (always `* 100`)
- Folio in clerk = 12 digits (leading zero dropped); pad to 13 before joining our DB

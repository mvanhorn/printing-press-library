## Customer model

### Environmental due-diligence consultant screening properties and acquisition targets

**Today (without this CLI):** The consultant searches ECHO by address or facility name, opens detailed-facility reports, follows program identifiers across media-specific pages, and copies findings into a diligence workbook. They cannot easily prove why two similar facility names were treated as the same—or different—regulated site.

**Weekly ritual:** For each active transaction, they assemble a defensible facility dossier covering programs, inspections, violations, enforcement, penalties, and pollutant records.

**Frustration:** Facility matching is probabilistic, while the final report must preserve source identifiers and explain every match.

### Portfolio environmental-risk reviewer at a lender or insurer

**Today (without this CLI):** The reviewer screens spreadsheets of collateral or insured facilities, repeats corporate searches, and compares current results with saved exports. Large portfolios push them toward EPA bulk downloads, but those files still require local crosswalking and prioritization.

**Weekly ritual:** They rescreen the portfolio and identify facilities with changed compliance status, new enforcement, inspections, or penalties.

**Frustration:** ECHO provides records, but not an identifier-preserving weekly change queue for a portfolio.

### Supply-chain compliance analyst monitoring parent companies

**Today (without this CLI):** The analyst uses corporate screening and facility searches, then reconciles parent-company matches with local supplier records. Similar names and incomplete corporate relationships force manual review.

**Weekly ritual:** They review facilities associated with monitored companies and investigate new significant noncompliance or enforcement activity.

**Frustration:** They cannot quickly connect parent-company matches, facility identifiers, program records, and recent enforcement into an auditable timeline.

### Community environmental journalist checking nearby facilities

**Today (without this CLI):** The journalist searches around an address or coordinates, opens multiple detailed reports, and manually compares inspections, violations, enforcement cases, and pollutant data. A geographic result list does not explain which records make one nearby facility more newsworthy than another.

**Weekly ritual:** They scan communities on their beat for newly concerning compliance or enforcement developments.

**Frustration:** Turning a radius search into a sourced explanation requires many tabs and manual joins.

## Candidates (pre-cut)

| # | Feature | Command | Description | Persona served | Source | Long Description | Inline rubric verdict |
|---|---------|---------|-------------|----------------|--------|------------------|-----------------------|
| 1 | Explainable facility resolution | `resolve facility "<query>" --explain-match` | Ranks candidate facilities while preserving FRS/program identifiers, matched fields, conflicts, and query provenance. | Due-diligence consultant | (a) persona-driven; (b) service-specific identifiers | none | **Keep:** Mechanical identity comparison using ECHO results and local aliases; no LLM, external service, or fabricated response. |
| 2 | Facility dossier change set | `dossier diff <facility-id> --since 30d` | Reports newly observed inspections, violations, cases, penalties, program-status changes, and pollutant records for one facility. | Due-diligence consultant | (a) persona-driven; (c) cross-entity local query | Use this command for changes to one facility dossier. Do NOT use this command for changes across a facility list; use `watch --portfolio` instead. | **Keep:** Local snapshot diff is verifiable and builds on synced ECHO records rather than reimplementing an endpoint. |
| 3 | Portfolio compliance watch | `watch --portfolio portfolio.csv --since 7d` | Crosswalks portfolio rows to preserved facility identities and emits a review queue of material record changes with match rationale. | Lender or insurer reviewer | (a) persona-driven; (c) cross-entity local query; (e) User Vision | Use this command for changes across a portfolio of facilities. Do NOT use this command for a single-facility change history; use `dossier diff` instead. | **Keep:** One-shot local comparison; no background process or opaque score. |
| 4 | Explained nearby scan | `nearby explain --latitude <lat> --longitude <lon> --radius <miles>` | Ranks nearby facilities by transparent concern drivers and returns the underlying record IDs for each driver. | Community environmental journalist | (a) persona-driven; (c) cross-entity local query; (e) User Vision | none | **Keep:** Mechanical categorization of linked violations, enforcement, penalties, inspections, and pollutant records; avoids an unverifiable composite risk score. |
| 5 | Effluent exceedance trend | `effluent trend <facility-id> --since 1y` | Joins effluent measurements or loading records to facility/program identity and reports exceedance counts, pollutants, periods, and source IDs. | Due-diligence consultant; journalist | (b) service-specific effluent/loading content; (c) cross-entity local query | none | **Keep:** Uses documented EPA effluent/loading data and local history; no semantic interpretation or external dependency. |
| 6 | Enforcement chain timeline | `enforcement timeline <facility-id> --since 5y` | Orders evaluations, violations, enforcement cases, and penalties into a sourced facility timeline. | Supply-chain analyst; due-diligence consultant | (b) service-specific enforcement content; (c) cross-entity local query | Use this command for the enforcement sequence behind one facility. Do NOT use this command for every kind of dossier change; use `dossier diff` instead. | **Keep:** Deterministic timestamp/identifier join over real ECHO records. |
| 7 | Compliance coverage audit | `dossier gaps <facility-id>` | Lists expected dossier categories that are absent or stale in the local store. | Due-diligence consultant | (c) cross-entity local query; (f) Codebase Intelligence | none | **Keep for cut review:** Buildable locally, but absence may mean either no record or incomplete retrieval, limiting weekly decision value. |
| 8 | Bulk-download planner | `bulk plan --portfolio portfolio.csv` | Recommends which EPA bulk datasets to download for a large portfolio. | Lender or insurer reviewer | (b) service-specific bulk workflow | none | **Kill now:** Mostly static advice, used during setup rather than weekly, and the brief does not define a callable bulk-download surface. |
| 9 | Composite environmental risk score | `risk score <facility-id>` | Produces a single numeric score from violations, enforcement, penalties, and pollutant data. | Lender or insurer reviewer | (a) persona-driven | none | **Kill now:** Weighting is unverifiable without domain policy and conflicts with the product thesis against opaque risk scores. |
| 10 | Live compliance dashboard | `dashboard --portfolio portfolio.csv` | Runs a persistent interactive portfolio-monitoring interface. | Lender or insurer reviewer | (a) persona-driven | none | **Kill now:** Requires a TUI or persistent process and exceeds one-command scope. |
| 11 | Enforcement news enrichment | `enforcement news <facility-id>` | Joins ECHO cases with press coverage and company news. | Journalist | (b) service-specific enforcement content | none | **Kill now:** Requires scraping or an external news service not present in the brief. |
| 12 | Media-specific search shortcuts | `facility search-air "<query>"` | Adds friendly aliases for existing media-specific facility-search endpoints. | All personas | (b) service-specific media families | none | **Kill now:** Thin endpoint renaming already covered by media-specific facility search table stakes. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | Persona served | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|----------------|--------------|----------|------------------|
| 1 | Explainable facility resolution | `resolve facility "<query>" --explain-match` | 10/10 | hand-code | Due-diligence consultant | This uses all-media/media-specific facility-search results plus locally stored aliases, coordinates, FRS IDs, program IDs, and query provenance to compute ranked matches and field-level match rationale with no external dependencies. | The brief’s Reachability Risk says matching is probabilistic and identifiers/match rationale must be preserved; Data Layer requires facility identities, aliases, coordinates, program IDs, and query provenance; Facility Dossier is the top workflow. | none |
| 2 | Facility dossier change set | `dossier diff <facility-id> --since 30d` | 10/10 | hand-code | Due-diligence consultant | This uses timestamped local facility, compliance, evaluation, enforcement, penalty, and pollutant records to compute identifier-preserving additions and status changes with no external dependencies. | Top Workflows names both Facility Dossier and Compliance Watch; Data Layer specifies snapshot timestamps and compliance/evaluation/enforcement records; the EPA detailed-facility-report source supports the dossier surface. | Use this command for changes to one facility dossier. Do NOT use this command for changes across a facility list; use `watch --portfolio` instead. |
| 3 | Portfolio compliance watch | `watch --portfolio portfolio.csv --since 7d` | 10/10 | hand-code | Lender or insurer reviewer | This uses locally stored facility aliases, parent-company matches, source identifiers, and timestamped compliance/enforcement snapshots to compute a portfolio-wide change queue with no external dependencies. | Portfolio Screen and Compliance Watch are named workflows; User Vision explicitly requests `watch --portfolio`; Reachability Risk requires preserved identifiers and match rationale. | Use this command for changes across a portfolio of facilities. Do NOT use this command for a single-facility change history; use `dossier diff` instead. |
| 4 | Explained nearby scan | `nearby explain --latitude <lat> --longitude <lon> --radius <miles>` | 9/10 | hand-code | Community environmental journalist | This uses geospatial facility-search results joined locally to violations, enforcement, penalties, inspections, and pollutant records to compute transparent concern-driver categories and cite their source IDs with no external dependencies. | Nearby Scan is a named workflow and asks which records drive concern; User Vision explicitly prioritizes `nearby`; Data Layer provides coordinates and linked compliance/enforcement records. | none |
| 5 | Effluent exceedance trend | `effluent trend <facility-id> --since 1y` | 8/10 | hand-code | Due-diligence consultant; journalist | This uses EPA effluent-chart or pollutant-loading results joined to local facility/program identifiers and snapshots to compute pollutant-level exceedance counts and periods with no external dependencies. | API Identity identifies effluent charts and pollutant-loading tools as distinctive ECHO families; Facility Dossier explicitly includes pollutant data; Data Layer preserves program identifiers and snapshot timestamps. | none |
| 6 | Enforcement chain timeline | `enforcement timeline <facility-id> --since 5y` | 9/10 | hand-code | Supply-chain compliance analyst; due-diligence consultant | This uses local evaluation, violation, enforcement-case, and penalty records keyed by preserved facility/program identifiers to compute a chronological enforcement chain with no external dependencies. | API Identity includes enforcement-case search and detailed reports; Facility Dossier requires inspections, violations, enforcement, and penalties; Data Layer stores compliance/evaluation/enforcement records. | Use this command for the enforcement sequence behind one facility. Do NOT use this command for every kind of dossier change; use `dossier diff` instead. |

Adversarial checks:

- **Explainable facility resolution:** The consultant runs identity resolution for active screenings each week. It is not a search-endpoint wrapper because its leverage comes from local alias history, cross-program identifiers, conflict detection, and provenance. Its transcendence proof is the local crosswalk and agent-readable match explanation. The closest killed candidate is media-specific search shortcuts, which merely rename existing endpoints. Buildability is `hand-code`. Its Long Description is `none`, so no sibling redirect can become stale.
- **Facility dossier change set:** The consultant reruns dossier comparisons weekly during active transactions. It is not a DFR wrapper because it compares historized records across multiple entity types. Its transcendence proof is the local temporal join. The closest killed candidate is compliance coverage audit, whose missing-data output is too ambiguous to drive the same weekly decision. Buildability is `hand-code`. Both redirects name surviving commands exactly: `dossier diff` and `watch --portfolio`.
- **Portfolio compliance watch:** The portfolio reviewer performs this rescreen weekly. It is not a corporate-screen wrapper because it joins portfolio rows, identity rationale, and changes across stored snapshots. Its transcendence proof is the multi-facility local change queue. The closest killed candidate is the live compliance dashboard, which turns the same need into an oversized persistent application. Buildability is `hand-code`. Its redirect to surviving `dossier diff` remains valid.
- **Explained nearby scan:** The journalist runs nearby checks weekly across their beat. It is not a geospatial-search wrapper because it joins results to several record families and exposes concrete concern drivers without collapsing them into a score. Its transcendence proof is the cross-entity local join and agent-shaped evidence output. The closest killed candidate is composite environmental risk score, which obscures the evidence this command preserves. Buildability is `hand-code`. Its Long Description is `none`.
- **Effluent exceedance trend:** Consultants and journalists examining industrial facilities can use it weekly during active monitoring. It is not an effluent-chart wrapper because it joins multiple periods to stable facility/program identities and computes a local trend. Its transcendence proof is the historized pollutant/program join. The closest killed candidate is enforcement news enrichment, which seeks contextual insight through an unavailable external source. Buildability is `hand-code`. Its Long Description is `none`.
- **Enforcement chain timeline:** Supply-chain analysts and consultants reviewing flagged facilities use it weekly. It is not an enforcement-search wrapper because it orders evaluations, violations, cases, and penalties across separate stored entities. Its transcendence proof is the identifier-preserving local chronology. The closest killed candidate is media-specific search shortcuts, which adds no cross-entity leverage. Buildability is `hand-code`. Its redirects reference the surviving `dossier diff` command exactly.

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Compliance coverage audit | Missing local categories cannot reliably distinguish “no EPA record” from incomplete retrieval, so the output is ambiguous and less likely to drive a weekly decision. | Facility dossier change set |
| Bulk-download planner | It is setup-time static guidance rather than a weekly workflow, and the supplied API surface does not establish a callable bulk-download implementation. | Portfolio compliance watch |
| Composite environmental risk score | Its weights would be unverifiable and opaque, directly conflicting with the evidence-first product thesis. | Explained nearby scan |
| Live compliance dashboard | A persistent TUI or service exceeds the command-sized scope; the weekly queue is already served by a one-shot portfolio watch. | Portfolio compliance watch |
| Enforcement news enrichment | It depends on scraping or an external news API absent from the brief. | Enforcement chain timeline |
| Media-specific search shortcuts | They are thin aliases over table-stakes media-specific facility-search endpoints and add no local or cross-entity leverage. | Explainable facility resolution |

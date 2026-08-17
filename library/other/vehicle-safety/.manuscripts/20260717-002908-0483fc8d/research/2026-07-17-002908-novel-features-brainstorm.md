## Customer model

### Used-car buyer comparing exact model years

**Today (without this CLI):** During an active search, they repeatedly move among VIN-decoder results, NHTSA recall and complaint pages, crash-rating pages, and listing tabs. They manually reconcile model naming and cannot easily distinguish model-wide campaigns from VIN-specific unrepaired recalls.

**Weekly ritual:** Several times each week while shopping, they shortlist two vehicles, decode their VINs, compare exact model years, and decide whether either deserves a paid inspection.

**Frustration:** The evidence is fragmented, and raw complaint counts invite misleading comparisons because production or exposure denominators are absent.

### Fleet safety manager reviewing a garage

**Today (without this CLI):** They maintain a spreadsheet of fleet VINs, rerun VIN or model lookups, and manually compare current results with prior exports. They cannot reliably identify only newly observed campaigns, remedy changes, or discrepancies between model-wide and VIN-specific recall coverage.

**Weekly ritual:** Once a week, they recheck the garage, triage newly observed safety issues, and produce an auditable exception list for maintenance staff.

**Frustration:** NHTSA provides current records, but the manager must construct change history and garage-wide prioritization independently while respecting VIN lookup rate limits.

### Independent mechanic preparing safety briefings

**Today (without this CLI):** Before service appointments, they open VIN-decoder, recall, complaint, rating, investigation, and manufacturer-communication results separately. They copy findings into a customer note and explain by hand which records are VIN-specific versus model-wide.

**Weekly ritual:** Several times per week, they prepare a concise, source-backed safety briefing for incoming vehicles before inspection or repair authorization.

**Frustration:** Building a defensible briefing takes too many lookups, and the distinction between an unrepaired VIN recall, a general campaign, a complaint pattern, and a manufacturer bulletin is easy to blur.

### Automotive safety reporter or product-liability researcher

**Today (without this CLI):** They download or query complaints, search investigations, recalls, and manufacturer communications separately, then maintain spreadsheets to align dates and components. They cannot quickly reproduce how a complaint signal evolved into agency or manufacturer action.

**Weekly ritual:** Each week, they review emerging defect claims, update timelines for active stories or cases, and preserve the underlying records supporting each claim.

**Frustration:** Cross-record chronology and component linkage are laborious, while narrative clustering would require subjective or external NLP that weakens reproducibility.

## Candidates (pre-cut)

1. **Safety dossier**
   - **Command:** `dossier`
   - **Description:** Produce one source-attributed report joining identity, VIN-specific recall status when available, model-wide recalls, complaints, ratings, investigations, and communications, with explicit scope caveats.
   - **Persona served:** Independent mechanic; used-car buyer
   - **Source:** (a) persona-driven, (c) cross-entity local queries, (e) user briefing, (f) DeepWiki
   - **Long Description:** Use this command for a comprehensive report about one vehicle or exact model year. Do NOT use this command to compare two vehicles; use `compare` instead.
   - **Verdict:** Keep. Mechanical joins and agent-shaped output need no LLM or external service; an `auto` data-source strategy can use live endpoints with local fallback and preserve source timestamps.

2. **Garage recall watch**
   - **Command:** `watch`
   - **Description:** Compare the latest synced observations for saved vehicles with prior snapshots and report newly observed campaigns, changed remedies, and resolved or missing records.
   - **Persona served:** Fleet safety manager
   - **Source:** (a) persona-driven, (c) cross-entity local queries, (e) user briefing
   - **Long Description:** none
   - **Verdict:** Keep. This is a bounded local change query, not a persistent monitor; it uses a `local` data-source strategy, rejects live-only execution, and relies on first/last-seen history.

3. **Defect signal timeline**
   - **Command:** `signals`
   - **Description:** Build a monthly component-level timeline of raw complaint volume alongside investigations, recalls, and communications without labeling counts as rates.
   - **Persona served:** Automotive safety reporter or product-liability researcher
   - **Source:** (a) persona-driven, (b) service-specific content patterns, (c) cross-entity local queries, (e) user briefing
   - **Long Description:** Use this command for a component-level chronology of complaint and agency/manufacturer signals. Do NOT use this command for a complete single-vehicle report; use `dossier` instead.
   - **Verdict:** Keep after mechanical reframing. It groups structured dates and component fields only; it does not summarize narratives, infer causation, or calculate incidence without a denominator.

4. **Honest model comparison**
   - **Command:** `compare`
   - **Description:** Compare two normalized model years across recall breadth, complaint mix, crash ratings, investigations, and communications while labeling complaint totals as raw reports.
   - **Persona served:** Used-car buyer; automotive safety reporter
   - **Source:** (a) persona-driven, (c) cross-entity local queries, (e) user briefing, (f) DeepWiki
   - **Long Description:** Use this command to compare two exact vehicle configurations or model years. Do NOT use this command for a deep report about one vehicle; use `dossier` instead.
   - **Verdict:** Keep. It uses normalized vehicle identities and cross-table output, avoids a synthetic risk score, and explicitly surfaces missing denominators.

5. **VIN/model recall coverage reconciliation**
   - **Command:** `recall-coverage`
   - **Description:** Contrast VIN-specific unrepaired recall results with model-wide campaigns and explain records present on only one surface.
   - **Persona served:** Fleet safety manager; independent mechanic
   - **Source:** (a) persona-driven, (b) service-specific content patterns, (c) cross-entity local queries
   - **Long Description:** Use this command to reconcile VIN-specific open-recall coverage with model-wide campaign records. Do NOT use this command for a comprehensive vehicle report; use `dossier` instead.
   - **Verdict:** Keep. The feature directly addresses the brief’s coverage warning and makes no claim that a model campaign proves an individual VIN has an unrepaired recall.

6. **Complaint-to-bulletin bridge**
   - **Command:** `bulletin-bridge`
   - **Description:** Match structured complaint components and dates to manufacturer communications for the same normalized vehicle and show auditable candidate relationships.
   - **Persona served:** Automotive safety reporter or product-liability researcher; independent mechanic
   - **Source:** (b) service-specific content patterns, (c) cross-entity local queries, (f) DeepWiki
   - **Long Description:** Use this command to inspect complaint and manufacturer-communication overlap for one vehicle family or component. Do NOT use this command for the broader complaint-to-agency chronology; use `signals` instead.
   - **Verdict:** Keep after limiting matching to structured vehicle, component, and date fields. It must label results as co-occurrence, not causation, and perform no semantic narrative analysis.

7. **Remedy-change audit**
   - **Command:** `remedy-diff`
   - **Description:** Show how remedy text and campaign status changed between two locally stored observations.
   - **Persona served:** Fleet safety manager
   - **Source:** (a) persona-driven, (c) cross-entity local queries
   - **Long Description:** none
   - **Verdict:** Keep through initial checks: local snapshots make it verifiable and dependency-free, but its scope substantially overlaps `watch`.

8. **Customer service briefing**
   - **Command:** `service-brief`
   - **Description:** Format selected safety findings into a plain-language, printable mechanic-to-customer handout.
   - **Persona served:** Independent mechanic
   - **Source:** (a) persona-driven
   - **Long Description:** none
   - **Verdict:** Keep through initial checks only as deterministic formatting; no generated prose or LLM summarization is allowed. Its evidence and query surface duplicate `dossier`.

9. **Complaint momentum ranking**
   - **Command:** `complaint-momentum`
   - **Description:** Rank vehicle/component groups by recent raw-report growth versus an earlier local window.
   - **Persona served:** Automotive safety reporter or product-liability researcher
   - **Source:** (a) persona-driven, (c) cross-entity local queries
   - **Long Description:** none
   - **Verdict:** Keep through initial checks if output includes minimum-count filters and denominator warnings, but it is a narrower and easier-to-misread slice of `signals`.

10. **Safety evidence bundle**
    - **Command:** `evidence-bundle`
    - **Description:** Export source records, query parameters, retrieval timestamps, and checksums into a reproducible case directory.
    - **Persona served:** Automotive safety reporter or product-liability researcher
    - **Source:** (a) persona-driven, (f) DeepWiki
    - **Long Description:** none
    - **Verdict:** Keep through initial checks because it is mechanical and auditable, but packaging directories and manifests risks exceeding the one-command feature scope and duplicates agent-shaped dossier output.

11. **Narrative defect clusters**
    - **Command:** `cluster-narratives`
    - **Description:** Semantically group complaint narratives and generate labels for emerging failure modes.
    - **Persona served:** Automotive safety reporter or product-liability researcher
    - **Source:** (a) persona-driven
    - **Long Description:** none
    - **Verdict:** Kill immediately. Semantic grouping and label generation require NLP/LLM behavior not supported by the API or deterministic local fields.

12. **Garage risk score**
    - **Command:** `risk-score`
    - **Description:** Assign each vehicle a single numeric safety-risk score from complaints, recalls, ratings, and investigations.
    - **Persona served:** Fleet safety manager; used-car buyer
    - **Source:** (a) persona-driven
    - **Long Description:** none
    - **Verdict:** Kill immediately. Missing production, exposure, severity, and reporting-bias denominators make a composite score unverifiable and potentially misleading.

13. **Bulk VIN sweep**
    - **Command:** `vin-sweep`
    - **Description:** Decode and check thousands of VINs in one invocation.
    - **Persona served:** Fleet safety manager
    - **Source:** (a) persona-driven
    - **Long Description:** none
    - **Verdict:** Kill immediately. NHTSA explicitly says its API is not for bulk VIN lookup, and a high-volume sweep conflicts with rate-control requirements.

14. **Campaign family map**
    - **Command:** `campaign-map`
    - **Description:** Expand a campaign into all locally observed makes, models, years, complaint components, investigations, and communications.
    - **Persona served:** Automotive safety reporter or product-liability researcher
    - **Source:** (b) service-specific content patterns, (c) cross-entity local queries
    - **Long Description:** none
    - **Verdict:** Keep through initial checks. It is mechanically buildable from local records, but its investigative value is already covered by `signals` plus campaign filtering.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona served | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|----------------|--------------|--------------|----------|------------------|
| 1 | Safety dossier | `dossier` | 10/10 | Independent mechanic; used-car buyer | hand-code | This uses vPIC identity data plus NHTSA recall, complaint, rating, investigation, and communication endpoints or their synced local records to compute one source-attributed report with no external dependencies. | The brief names “Pre-purchase dossier” as Top Workflow 1; User Vision prioritizes `dossier`; Codebase Intelligence identifies reproducible dossiers as the gap versus SaferCar-style single-surface experiences. | Use this command for a comprehensive report about one vehicle or exact model year. Do NOT use this command to compare two vehicles; use `compare` instead. |
| 2 | Garage recall watch | `watch` | 10/10 | Fleet safety manager | hand-code | This uses locally synced campaign observations and first/last-seen timestamps to compute newly observed campaigns, remedy changes, and disappeared records with no external dependencies. | “Garage recall watch” is Top Workflow 2; the Data Layer explicitly requires historical snapshots and first/last-seen timestamps; User Vision and Build Priority 1 prioritize saved-garage watch. | none |
| 3 | Defect signal timeline | `signals` | 10/10 | Automotive safety reporter or product-liability researcher | hand-code | This uses synced complaint component/month fields joined to investigation, recall, and communication dates to compute an auditable timeline with no external dependencies. | “Emerging defect scan” is Top Workflow 3; the Data Layer calls for complaint component/time fields and linked safety records; User Vision and Build Priority 2 prioritize a complaint-to-investigation/recall timeline. | Use this command for a component-level chronology of complaint and agency/manufacturer signals. Do NOT use this command for a complete single-vehicle report; use `dossier` instead. |
| 4 | Honest model comparison | `compare` | 10/10 | Used-car buyer; automotive safety reporter | hand-code | This uses normalized year/make/model identities and synced recalls, complaints, ratings, investigations, and communications to compute a side-by-side comparison with explicit denominator caveats and no external dependencies. | “Model comparison” is Top Workflow 4; Reachability Risk warns that complaint counts lack exposure denominators; User Vision and Build Priority 3 explicitly prioritize `compare` with caveats. | Use this command to compare two exact vehicle configurations or model years. Do NOT use this command for a deep report about one vehicle; use `dossier` instead. |
| 5 | VIN/model recall coverage reconciliation | `recall-coverage` | 9/10 | Fleet safety manager; independent mechanic | hand-code | This uses the VIN-specific recall response and model-wide campaign records to compute matched and one-sided recall findings with no external dependencies. | Reachability Risk states that manufacturer-supplied VIN open-recall coverage may differ from general model recall data; the Product Thesis requires distinguishing model-wide campaigns from VIN-specific unrepaired recalls; Top Workflows 1 and 2 require both views. | Use this command to reconcile VIN-specific open-recall coverage with model-wide campaign records. Do NOT use this command for a comprehensive vehicle report; use `dossier` instead. |
| 6 | Complaint-to-bulletin bridge | `bulletin-bridge` | 8/10 | Automotive safety reporter or product-liability researcher; independent mechanic | hand-code | This uses locally synced normalized vehicle identities, structured complaint components and dates, and manufacturer communication metadata to compute labeled co-occurrence candidates with no external dependencies. | The API Identity includes both owner complaints and manufacturer communications; Top Workflow 3 calls for component/time clustering against safety actions; Codebase Intelligence identifies cross-record local relationships as the CLI opportunity. | Use this command to inspect complaint and manufacturer-communication overlap for one vehicle family or component. Do NOT use this command for the broader complaint-to-agency chronology; use `signals` instead. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Remedy-change audit | Although weekly, locally verifiable, and hand-code buildable, its complete useful scope is already a filter within `watch`, so a separate command would fragment the fleet ritual. | `watch` |
| Customer service briefing | Deterministic formatting is feasible, but the mechanic can select plain or agent-shaped output from the richer dossier; a separate command adds presentation scope without new data leverage. | `dossier` |
| Complaint momentum ranking | Raw growth is easy to misread without exposure denominators and is better presented as one explicitly caveated view inside the broader chronology. | `signals` |
| Safety evidence bundle | Reproducibility matters, but directory packaging and manifest management push beyond a compact command while `dossier` already emits attributed, timestamped, machine-readable evidence. | `dossier` |
| Narrative defect clusters | Semantic narrative grouping requires NLP/LLM classification and cannot be deterministically verified from the documented API fields. | `signals` |
| Garage risk score | A composite score would imply comparability unsupported by absent production and exposure denominators, making the output unverifiable and misleading. | `compare` |
| Bulk VIN sweep | NHTSA explicitly discourages bulk VIN lookup, so the command conflicts with the service’s traffic-control expectations and cannot ship responsibly. | `watch` |
| Campaign family map | The local joins are feasible, but reporters can obtain the same campaign-centered chronology by filtering `signals`; the separate surface would not add weekly leverage. | `signals` |

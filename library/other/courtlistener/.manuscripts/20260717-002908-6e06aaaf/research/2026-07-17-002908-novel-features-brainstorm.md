## Customer model

### Litigation associate checking an active docket before standup

**Today (without this CLI):** Each morning they reopen CourtListener and PACER-related tabs, inspect the docket, scan recent entries, check whether documents are actually available through RECAP, and paste changes into an internal case note. They cannot generate one reproducible chronology that distinguishes entry metadata from locally available documents.

**Weekly ritual:** Every weekday, and especially before the weekly team meeting, they review active dockets for new filings, changed parties or counsel, and documents requiring attorney attention.

**Frustration:** Docket metadata, entries, parties, counsel, and document availability are separate surfaces, while incomplete RECAP coverage makes silence or a missing PDF ambiguous.

### Legal journalist tracking a beat

**Today (without this CLI):** They rerun saved searches for companies, judges, topics, and cases, compare result pages with notes from the prior day, and open several dockets to understand what changed. They cannot easily separate genuinely new filings from previously seen results or preserve an auditable trail for a story.

**Weekly ritual:** Several times each week, they scan watched searches and dockets, follow recurring parties and lawyers, and build context around judges and newly published opinions.

**Frustration:** Change detection and cross-case relationship tracing are manual, and legal-research products can obscure whether a document is present, missing from RECAP, or merely represented by metadata.

### Nonprofit legal-data researcher maintaining a reproducible corpus

**Today (without this CLI):** They write pagination and retry scripts, export API responses, normalize canonical URLs and IDs, and join dockets, parties, attorneys, opinions, and judges in notebooks. They repeatedly audit missing documents and worry that rolling throttles or incomplete RECAP coverage silently bias a dataset.

**Weekly ritual:** Each week, they refresh a defined corpus, inspect additions and coverage gaps, and publish a reproducible record of source queries, timestamps, and missingness.

**Frustration:** The API exposes the records, but defensible local joins, refresh cursors, and explicit coverage accounting must be rebuilt for every project.

### In-house counsel monitoring company litigation

**Today (without this CLI):** They maintain keyword alerts, spreadsheets of related matters, and lists of outside counsel, then ask staff to verify whether a new hit is actually connected to the company. They cannot quickly see recurring parties, opposing counsel, and representation patterns across dockets.

**Weekly ritual:** At least weekly, they review new litigation mentions, update matter lists, and assess which internal or outside lawyers need the underlying filing metadata or available documents.

**Frustration:** Entity names vary across cases, representation changes over time, and a search hit does not provide a reliable cross-docket relationship map.

## Candidates (pre-cut)

1. **Chronological docket brief**
   - **Command:** `docket`
   - **Description:** Assemble docket metadata, parties, counsel, entries, available RECAP documents, and source links into one chronological, coverage-caveated report.
   - **Persona served:** Litigation associate; legal journalist
   - **Source:** (a) persona-driven, (b) service-specific content patterns, (c) cross-entity local queries, (e) user briefing, (f) DeepWiki
   - **Long Description:** Use this command for a comprehensive chronology of one docket. Do NOT use this command to report changes across saved searches or multiple dockets; use `new-filings` instead.
   - **Verdict:** Keep. An `auto` data-source strategy can join live API records or synced local data, and the report can mechanically distinguish metadata-only entries from available documents without NLP.

2. **New-filing watch report**
   - **Command:** `new-filings`
   - **Description:** Compare watched query or docket cursors with first-seen local records and report only newly observed entries, documents, opinions, or dockets.
   - **Persona served:** Litigation associate; legal journalist; in-house counsel
   - **Source:** (a) persona-driven, (c) cross-entity local queries, (e) user briefing, (f) DeepWiki
   - **Long Description:** Use this command to report newly observed results across saved watches. Do NOT use this command for the complete history of one docket; use `docket` instead.
   - **Verdict:** Keep. This is bounded local change detection rather than a background monitor; it uses a `local` data-source strategy, rejects `--data-source live`, and relies on persisted cursors and first-seen timestamps.

3. **Cross-docket party map**
   - **Command:** `party`
   - **Description:** Map a normalized party name to related dockets, co-parties, counsel, courts, and first/last observed activity with canonical source links.
   - **Persona served:** In-house counsel; legal journalist
   - **Source:** (a) persona-driven, (c) cross-entity local queries, (e) user briefing
   - **Long Description:** Use this command to trace where a party appears and who represents it. Do NOT use this command to analyze an attorney or firm's recurring representations; use `counsel` instead.
   - **Verdict:** Keep. Structured party, attorney, and docket relationships make the map mechanical and auditable; name variants must remain visible rather than being semantically merged.

4. **Recurring counsel map**
   - **Command:** `counsel`
   - **Description:** Show an attorney or firm's observed representations across dockets, parties, courts, case types, and time.
   - **Persona served:** In-house counsel; legal journalist
   - **Source:** (a) persona-driven, (c) cross-entity local queries, (e) user briefing
   - **Long Description:** Use this command to inspect an attorney or firm's recurring representations. Do NOT use this command to trace all cases involving a litigant; use `party` instead.
   - **Verdict:** Keep. It uses explicit attorney-party-docket records and reports observed representation only, avoiding inferred affiliations or success claims.

5. **Non-predictive judge context**
   - **Command:** `judge`
   - **Description:** Combine judge metadata with authored or participated opinions, courts, dates, and case types while explicitly refusing outcome prediction.
   - **Persona served:** Legal journalist; nonprofit legal-data researcher
   - **Source:** (a) persona-driven, (b) service-specific content patterns, (c) cross-entity local queries, (e) user briefing
   - **Long Description:** none
   - **Verdict:** Keep after limiting output to descriptive counts and linked records. It must not calculate win rates, ideological scores, or causal claims from selection-biased CourtListener data.

6. **RECAP availability audit**
   - **Command:** `recap-gaps`
   - **Description:** Inventory docket entries with available documents, metadata-only document records, unavailable documents, and ambiguous coverage states.
   - **Persona served:** Litigation associate; nonprofit legal-data researcher
   - **Source:** (a) persona-driven, (b) service-specific content patterns, (c) cross-entity local queries
   - **Long Description:** Use this command to audit document availability and missingness across synced RECAP records. Do NOT use this command to read a complete case chronology; use `docket` instead.
   - **Verdict:** Keep. A `local` data-source strategy can compute explicit availability categories from stored flags and canonical records without fetching PACER documents or implying complete federal coverage.

7. **Docket entry diff**
   - **Command:** `docket-diff`
   - **Description:** Compare two local docket snapshots and show added entries, changed descriptions, and document-availability transitions.
   - **Persona served:** Litigation associate
   - **Source:** (a) persona-driven, (c) cross-entity local queries
   - **Long Description:** none
   - **Verdict:** Keep through initial checks because snapshots make it verifiable, but its useful weekly behavior overlaps `new-filings` and `docket`.

8. **Counsel concentration report**
   - **Command:** `counsel-concentration`
   - **Description:** Rank counsel by observed appearances for a party, court, or case type.
   - **Persona served:** In-house counsel; legal journalist
   - **Source:** (a) persona-driven, (c) cross-entity local queries
   - **Long Description:** none
   - **Verdict:** Keep through initial checks if described as observed counts rather than market share, but it is a filter and grouping within `counsel` rather than a distinct ritual.

9. **Oral-argument packet**
   - **Command:** `argument-packet`
   - **Description:** Join an oral-argument record to its docket, participating judges, opinions, and available source links.
   - **Persona served:** Legal journalist; litigation associate
   - **Source:** (b) service-specific content patterns, (c) cross-entity local queries
   - **Long Description:** none
   - **Verdict:** Keep through initial checks because the API supplies the component records, but the brief does not establish weekly demand distinct from the general docket chronology.

10. **Dataset provenance bundle**
    - **Command:** `dataset-manifest`
    - **Description:** Write query parameters, canonical URLs, retrieval timestamps, pagination cursors, and record counts for a local research corpus.
    - **Persona served:** Nonprofit legal-data researcher
    - **Source:** (a) persona-driven, (f) DeepWiki
    - **Long Description:** none
    - **Verdict:** Keep through initial checks as deterministic local metadata, but it risks becoming a packaging system rather than a focused command and is not itself CourtListener-specific data leverage.

11. **Opinion narrative summarizer**
    - **Command:** `opinion-summary`
    - **Description:** Generate a plain-language holding and rationale from opinion text.
    - **Persona served:** Litigation associate; legal journalist
    - **Source:** (a) persona-driven
    - **Long Description:** none
    - **Verdict:** Kill immediately. Reliable legal summarization requires an LLM or external NLP service and cannot be verified mechanically from the API response.

12. **Judge outcome predictor**
    - **Command:** `judge-predict`
    - **Description:** Predict case outcomes from a judge's prior opinions and case history.
    - **Persona served:** Litigation associate; in-house counsel
    - **Source:** (a) persona-driven
    - **Long Description:** none
    - **Verdict:** Kill immediately. The brief expressly requires non-predictive judge context, and incomplete, selection-biased records cannot support a verifiable causal forecast.

13. **Automatic PACER document fetcher**
    - **Command:** `fetch-missing`
    - **Description:** Purchase or retrieve every document missing from RECAP.
    - **Persona served:** Litigation associate; nonprofit legal-data researcher
    - **Source:** (a) persona-driven
    - **Long Description:** none
    - **Verdict:** Kill immediately. It requires PACER access, possible fees, and an external acquisition workflow not provided by the CourtListener API surface in the brief.

14. **Global litigation graph**
    - **Command:** `litigation-graph`
    - **Description:** Build and interactively explore a graph of every party, lawyer, judge, docket, opinion, and citation in the local corpus.
    - **Persona served:** Legal journalist; nonprofit legal-data researcher
    - **Source:** (c) cross-entity local queries, (e) user briefing
    - **Long Description:** none
    - **Verdict:** Kill immediately after descoping analysis. An interactive global graph is application-scale, likely exceeds 200 lines, and requires visualization infrastructure; its useful one-command slices are already represented by `party` and `counsel`.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona served | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|----------------|--------------|--------------|----------|------------------|
| 1 | Chronological docket brief | `docket` | 10/10 | Litigation associate; legal journalist | hand-code | This uses CourtListener docket, party, attorney, docket-entry, and RECAP-document endpoints or their synced local records to compute one chronological report with availability flags and no external dependencies. | “Docket brief” is Top Workflow 1; User Vision prioritizes `docket`; Codebase Intelligence identifies composable local timelines with explicit CourtListener links and coverage caveats as the CLI opportunity. | Use this command for a comprehensive chronology of one docket. Do NOT use this command to report changes across saved searches or multiple dockets; use `new-filings` instead. |
| 2 | New-filing watch report | `new-filings` | 10/10 | Litigation associate; legal journalist; in-house counsel | hand-code | This uses watched query cursors plus local first-seen timestamps for dockets, entries, documents, and opinions to compute newly observed records with no external dependencies. | “New-filing watch” is Top Workflow 2; the Data Layer requires watched query cursors and first/last-seen timestamps; User Vision and Build Priority 1 prioritize `watch` and `new-filings`. | Use this command to report newly observed results across saved watches. Do NOT use this command for the complete history of one docket; use `docket` instead. |
| 3 | Cross-docket party map | `party` | 9/10 | In-house counsel; legal journalist | hand-code | This uses locally synced party, attorney, docket, and court relationships to compute the cases, co-parties, and counsel attached to an exact or explicitly selected party identity with no external dependencies. | “Party/counsel map” is Top Workflow 3; User Vision prioritizes `party`; Build Priority 2 calls for party/counsel relationship mapping. | Use this command to trace where a party appears and who represents it. Do NOT use this command to analyze an attorney or firm's recurring representations; use `counsel` instead. |
| 4 | Recurring counsel map | `counsel` | 9/10 | In-house counsel; legal journalist | hand-code | This uses locally synced attorney, party, docket, court, and date records to compute observed representations across matters with no external dependencies. | The in-house-counsel persona monitors companies and outside counsel; “Party/counsel map” is Top Workflow 3; User Vision and Build Priority 2 explicitly prioritize `counsel` relationship mapping. | Use this command to inspect an attorney or firm's recurring representations. Do NOT use this command to trace all cases involving a litigant; use `party` instead. |
| 5 | Non-predictive judge context | `judge` | 9/10 | Legal journalist; nonprofit legal-data researcher | hand-code | This uses judge metadata joined to authored or participated opinions, courts, dates, and case types to compute descriptive context with linked source records and no external dependencies. | “Judge context” is Top Workflow 4 and explicitly rejects causal prediction; User Vision prioritizes `judge`; Product Thesis requires scrupulous non-predictive use. | none |
| 6 | RECAP availability audit | `recap-gaps` | 9/10 | Litigation associate; nonprofit legal-data researcher | hand-code | This uses local docket-entry, RECAP-document, and availability-flag records to compute available, metadata-only, unavailable, and ambiguous coverage categories with no external dependencies. | Reachability Risk says RECAP coverage is incomplete and document availability or cost varies; the Data Layer requires document availability flags; Product Thesis requires explicit coverage and availability caveats. | Use this command to audit document availability and missingness across synced RECAP records. Do NOT use this command to read a complete case chronology; use `docket` instead. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Docket entry diff | Its snapshot comparison is buildable and useful, but the weekly change ritual is fully served by `new-filings`, while complete chronology belongs in `docket`. | `new-filings` |
| Counsel concentration report | Ranking observed appearances is a useful view, but it is only a grouping option within the broader recurring-representation map and does not justify a separate command. | `counsel` |
| Oral-argument packet | The join is feasible, but research did not establish weekly demand separate from the docket brief, so it falls below the pain and evidence bar. | `docket` |
| Dataset provenance bundle | Deterministic provenance is useful but becomes generic corpus-packaging infrastructure rather than a CourtListener-specific weekly insight command. | `recap-gaps` |
| Opinion narrative summarizer | Legal summarization requires an LLM or external NLP dependency and cannot be mechanically verified from the API records. | `docket` |
| Judge outcome predictor | It conflicts with the brief's non-predictive requirement and would infer outcomes from incomplete, selection-biased data. | `judge` |
| Automatic PACER document fetcher | It requires external PACER credentials, possible purchases, and acquisition behavior outside the documented CourtListener API scope. | `recap-gaps` |
| Global litigation graph | An interactive all-entity graph is application-scale; the bounded, weekly-useful relationship slices survive as `party` and `counsel`. | `party` |

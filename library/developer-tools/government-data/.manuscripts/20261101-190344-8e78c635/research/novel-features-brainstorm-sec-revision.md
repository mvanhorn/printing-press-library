## Customer model

### California water-policy analyst
**Today (without this CLI):** The brief describes an analyst opening California and national catalog searches, applying publisher and format filters, and inspecting dataset resources before each weekly report. Catalog metadata and actual agency files live in different places.

**Weekly ritual:** Repeat a water-topic sweep and select a defensible set of machine-readable sources for the week's analysis.

**Frustration:** A federal search can list the same agency resource as the state catalog; different record identifiers obscure that overlap, while missing metadata makes intake decisions harder.

### Civic-data journalist
**Today (without this CLI):** The brief describes a reporter revisiting known datasets and checking whether their metadata or download links changed. Comparing browser tabs or exported metadata requires keeping the previous state independently.

**Weekly ritual:** Revisit the sources behind an active story and record changes that require follow-up with the publishing agency.

**Frustration:** A current dataset page does not explain how its title, license, publisher, or resource links differ from the previously used version.

### Research data engineer
**Today (without this CLI):** The brief describes an engineer maintaining repeatable discovery and download pipelines. ckanapi supplies metadata exports, but the two catalog contracts and pagination models differ.

**Weekly ritual:** Refresh a bounded research topic, review candidate resources, and preserve an explicit intake record before downstream processing.

**Frustration:** A dataset ID alone is insufficient to reproduce the chosen file, and an exported catalog record does not make selection provenance or metadata omissions easy to inspect.

The SEC revision adds direct company lookup, full-text filing search, submissions including bounded older-history traversal, document listing/download, and company facts/concepts to the baseline. The existing journalist and research-engineer personas plausibly use these read-only surfaces, but the brief supplies no evidence of a new weekly investment-analysis ritual. No new finance persona is invented. Company facts remain filing-provenanced facts rather than canonical financial statements. Original catalog personas and weekly rituals remain valid and retain equal catalog prominence.

## Candidates (pre-cut)

Personas and rituals above are workflow inferences explicitly identified in the brief, not claims of user interviews. Sources: (a) persona-driven, (b) service-specific metadata/resource pattern, (c) cross-entity local query, (e) user briefing. Current-run research.json supplies five planned, unshipped prior features for source (d); each is re-scored below. There is no Codebase Intelligence section, so source (f) does not apply. SEC source (e) adds requests for raw filing and financial-fact access, already included in the baseline. Baseline search, detail, resource listing, download, local sync/search, and export are already absorbed.

| # | Candidate / command | Description | Persona | Source | Inline verdict | Long Description |
|---|---|---|---|---|---|---|
| 1 | Shared resource evidence / `overlap` | Show resource URLs occurring in saved records from both catalogs, retaining every source record. | Analyst | a,b,c,e | Keep; exact URL join is testable and does not assert semantic dataset identity. | Use this command to find shared resource URLs across saved catalog records. Do NOT use it to compare records over time; use 'changes' instead. |
| 2 | Semantic duplicate detector / `duplicates` | Infer whether differently named datasets describe the same underlying measurements. | Analyst | a | Kill; requires semantic inference or unsupported domain-specific matching. | none |
| 3 | Saved metadata changes / `changes` | Compare two explicit saved snapshots and report added, removed and changed fields and resource URLs. | Journalist | a,b,c | Keep; bounded local diff, never claims that an absent query result proves deletion. | Use this command to compare two saved metadata snapshots over time. Do NOT use it to identify cross-catalog resource overlap; use 'overlap' instead. |
| 4 | Always-on disappearance monitor / `monitor` | Notify whenever a source disappears. | Journalist | a | Kill; background process and query absence cannot establish source disappearance. | none |
| 5 | Selection provenance lock / `intake lock` | Write an explicit selected-resource manifest with catalog ID, URL, metadata fingerprint and capture time. | Engineer | a,b,e | Keep; local deterministic artifact, stronger than generic raw metadata export. | Use this command to preserve the selected resources and their catalog provenance. Do NOT use it to diagnose missing metadata; use 'intake audit' instead. |
| 6 | Mirror everything / `mirror` | Download all matching agency resources and archive every historical version. | Engineer | a | Kill; unbounded files, storage and historical claims exceed a small command. | none |
| 7 | Metadata intake audit / `intake audit` | Report missing title, publisher, license, modified date, resource URL and format on saved records. | Engineer / Analyst | a,b,c | Keep; explicit field-presence findings, not a legal judgment or overall data-quality score. | Use this command to find missing metadata before selecting resources. Do NOT use it to preserve a selected intake manifest; use 'intake lock' instead. |
| 8 | Trust score / `trust` | Rank datasets by inferred authority, freshness and quality. | Analyst | a | Kill; unsupported quality conclusions and measurement freshness cannot be derived from catalog recency. | none |
| 9 | Saved format coverage / `coverage` | Show the source-by-format matrix and missing-format records within an explicit saved topic scope. | Analyst | a,b,c | Keep; local cross-source comparison with captured counts, query, limits and timestamps, not catalog-wide completeness. | Use this command to compare format availability within a saved topic result set. Do NOT use it to inspect individual metadata omissions; use 'intake audit' instead. |
| 10 | Agency performance league table / `rank-agencies` | Compare agencies by dataset update speed and publication quality. | Journalist | a | Kill; local bounded samples cannot support agency-wide rankings and metadata dates are not observation dates. | none |

| 11 | Cross-catalog-to-company linker / `company-context` | Infer which catalog datasets concern an EDGAR issuer. | Journalist | e,c | Kill; no reliable CIK/company relationship in generic catalog metadata and no research-backed weekly ritual. | none |
| 12 | Latest comparable financials / `financials latest` | Select a single comparable value across units, periods, forms and amendments. | Engineer | e,b | Kill; adds interpretation absent from the user request and risks false comparability; baseline facts/concept preserves all upstream provenance. | none |
| 13 | Filing provenance lock / `edgar lock` | Preserve accession, document and fact selections in a manifest. | Engineer | e,b | Kill as a separate novel command; baseline filing/fact output already retains accession and units, and extending catalog intake lock to heterogeneous SEC fact selection adds unrequested persistence scope. | none |

## Survivors and kills

### Survivors

Revision verdict: retain the five planned catalog features without expanding them to SEC records. The SEC additions remain baseline read-only commands, not a reason to manufacture investment analytics. All survivors are `hand-code`; none is an endpoint rename. Local read commands declare `// pp:data-source local`, reject live-only use, call generated unsynced/stale hint helpers, and drain/check/close parent SQLite rows before follow-up queries. `intake lock` uses local snapshots plus explicit resource selection and writes an output artifact; transaction writes must not invoke pooled Upsert inside an open custom write transaction. Baseline sync must retain snapshot membership and metadata payloads rather than only overwrite the current record. If a command lacks the required local evidence, it returns an actionable error, not an empty assertion about the live catalog.

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Persona served | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Shared resource evidence | `overlap` | 8/10 (fit 3, pain 2, feasibility 2, backing 1) | hand-code | This uses saved dataset-to-resource rows from both catalogs to join exact resource URL strings and emit all contributing catalog IDs, with no external dependencies. | Brief Top Workflows #4 explicitly calls for identical agency download URL detection; Data.gov publishing guide explains harvested agency resource metadata. | California water-policy analyst | Use this command to find shared resource URLs across saved catalog records. Do NOT use it to compare records over time; use 'changes' instead. |
| 2 | Saved metadata changes | `changes` | 8/10 (fit 3, pain 2, feasibility 1, backing 2) | hand-code | This uses two explicit local metadata snapshots to compute field-level and resource-URL changes and snapshot-membership differences, with no external dependencies. | Brief weekly source refresh and reporter verification rituals; Data.gov user guide/publishing model describes ongoing harvested metadata updates; ckanapi exports make old metadata preservation feasible. | Civic-data journalist | Use this command to compare two saved metadata snapshots over time. Do NOT use it to identify cross-catalog resource overlap; use 'overlap' instead. |
| 3 | Selection provenance lock | `intake lock` | 8/10 (fit 3, pain 2, feasibility 2, backing 1) | hand-code | This uses explicitly selected saved dataset/resource rows to write a deterministic manifest containing catalog IDs, exact resource URLs, metadata SHA-256 values, selection scope and capture times, with no external dependencies. | Brief research-engineer repeatable intake ritual and metadata-versus-file distinction; California and federal metadata provide resource URLs and identifiers. | Research data engineer | Use this command to preserve the selected resources and their catalog provenance. Do NOT use it to diagnose missing metadata; use 'intake audit' instead. |
| 4 | Metadata intake audit | `intake audit` | 8/10 (fit 3, pain 2, feasibility 2, backing 1) | hand-code | This uses saved raw metadata and normalized resource rows to compute explicit missing-field findings for title, publisher, license, modified date, URL and format, with no external dependencies. | Brief intake workflow inspects license, dates and formats; CKAN and federal catalog responses expose these metadata fields or their absence. | Research data engineer / California water-policy analyst | Use this command to find missing metadata before selecting resources. Do NOT use it to preserve a selected intake manifest; use 'intake lock' instead. |
| 5 | Saved format coverage | `coverage` | 7/10 (fit 2, pain 2, feasibility 2, backing 1) | hand-code | This uses explicit saved topic-result membership joined to resource formats and catalog identity to compute a source-by-format matrix plus records without declared formats, with no external dependencies. | Brief weekly analyst topic sweep and equal-catalog vision; California browser capture demonstrates format filters and resource-format metadata. | California water-policy analyst | Use this command to compare format availability within a saved topic result set. Do NOT use it to inspect individual metadata omissions; use 'intake audit' instead. |

| Survivor | Weekly use | Wrapper versus leverage | Transcendence proof | Closest killed sibling and reason | Buildability proof check | Long-description validity |
|---|---|---|---|---|---|---|
| overlap | Analyst's weekly sweep checks repeated source URLs before shortlisting. | No single endpoint joins both catalogs. | Cross-source local join with provenance, exact-string evidence only. | duplicates: semantic identity is unsupported; exact URL evidence is defensible. | Uses the existing metadata/resource collections; custom Go join/output required. | changes survives with this exact name. |
| changes | Reporter revisits sources behind the active story weekly. | Neither current detail API provides the user's previous saved metadata. | Local historical snapshots and structured field diffs. | monitor: constant surveillance and disappearance claims exceed evidence. | Requires bounded snapshot persistence in addition to existing metadata; custom Go diff required. | overlap survives with this exact name. |
| intake lock | Engineer records the chosen weekly intake before downstream processing. | Raw export does not identify the explicitly selected resource set or reproducible metadata fingerprints. | Agent-shaped deterministic provenance artifact joining dataset and resource selection. | mirror: copying every file is unbounded; a selected intake manifest is bounded. | Reads saved selections and computes hashes; custom Go artifact writer required. | intake audit survives with this exact name. |
| intake audit | Engineer checks newly refreshed intake candidates weekly. | No endpoint computes a cross-catalog missing-field checklist. | Local join of catalog metadata and resource metadata with explicit finding codes. | trust: quality ranking exceeds field-presence evidence. | Known-field presence checks and record/resource association suffice; custom Go findings required. | intake lock survives with this exact name. |
| coverage | Analyst compares machine-readable format availability in each weekly saved sweep. | Individual portal facets do not compare two saved result sets with shared scope provenance. | Cross-source local topic membership/resource-format matrix. | rank-agencies: format coverage is descriptive within the sample; agency performance ranking is unsupported. | Local scoped membership and resources suffice; custom Go matrix output required. | intake audit survives with this exact name. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Semantic duplicate detector | It would infer dataset identity beyond exact metadata evidence and needs semantic or domain rules absent from this brief. | overlap |
| Always-on disappearance monitor | It requires persistent monitoring and would overstate what absence from bounded search results proves. | changes |
| Mirror everything | It introduces unbounded file downloads/storage and historical guarantees beyond a bounded intake command. | intake lock |
| Trust score | Catalog metadata does not substantiate authority, data quality or observation freshness scores. | intake audit |
| Agency performance league table | A saved topic sample cannot support agency-wide performance conclusions. | coverage |
| Cross-catalog-to-company linker | Generic catalog metadata does not establish issuer identity and the brief supplies no named recurring linkage workflow. | overlap |
| Latest comparable financials | Collapsing SEC facts across units, periods and amendments would add unsupported interpretation beyond raw financial-fact access. | intake audit |
| Filing provenance lock | Separate command duplicates provenance already required in baseline EDGAR outputs while adding unsupported heterogeneous persistence scope. | intake lock |


## Reprint verdicts

This is a scope-revision reconciliation of planned current-run features, not a claim that any feature has shipped. Existing score dimensions were re-evaluated against the original concrete personas and the revised SEC baseline. SEC introduces no contrary evidence against catalog workflow utility; it also supplies no extra research-backing points for these catalog-only features.

| Prior planned feature | Verdict | Revised score | Re-evaluation |
|---|---|---|---|
| Shared resource evidence / overlap | keep | 8/10 (3,2,2,1) | Exact URL overlap still answers the named weekly catalog verification workflow; restrict it to saved catalog resources, with no inferred company linkage. |
| Saved metadata changes / changes | keep | 8/10 (3,2,1,2) | Weekly journalist source refresh remains valid; compare explicit catalog snapshots only, with no filing-amendment interpretation. |
| Selection provenance lock / intake lock | keep | 8/10 (3,2,2,1) | Repeatable catalog intake remains supported; do not silently broaden the manifest to XBRL fact selections. |
| Metadata intake audit / intake audit | keep | 8/10 (3,2,2,1) | Existing catalog field-presence checks remain useful; do not apply license/format checklists to SEC facts or infer financial quality. |
| Saved format coverage / coverage | keep | 7/10 (2,2,2,1) | The bounded saved topic matrix remains valid for the catalog analyst; SEC is a separate direct source and excluded from this catalog comparison. |

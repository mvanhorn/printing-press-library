## Customer model

The concrete personas and rituals below come from the brief's Users and Top Workflows. “Weekly” refers to repeated active itinerary planning, not an assertion that every traveler uses Carstay year-round. Prior research is `none`, as discovered by the builder's prescribed snippet. The brief has no `## User Vision` or `## Codebase Intelligence` section; those optional candidate-source branches do not apply.

**Mika, a self-drive campervan traveler choosing the next designated overnight stop.**

Today (without this CLI): Mika searches several Carstay station pages for the next route segment, opens descriptions and facilities in separate tabs, and checks whether the parking space and amenities suit the rented vehicle. A price on a list card does not answer the total-cost or explicit-acceptance question.

Weekly ritual: During active trip planning and travel, Mika repeats stop selection for the next few days, comparing power, toilets, bathing, parking dimensions, and source rules near planned coordinates.

Frustration: The same dimensions and facility requirements must be compared manually across many pages, while missing fields look deceptively like negative answers.

**Alex, an English-speaking planner expanding a Japan shortlist.**

Today (without this CLI): Alex starts on the English station page, then opens Japanese pages after noticing the smaller result set, preserving original station names and checking whether an English name actually exists.

Weekly ritual: While preparing the itinerary, Alex revisits regional candidates and checks which Japanese-only options the English discovery path would miss.

Frustration: English approval, translated field presence, and total Japan coverage are conflated, so a convenient language filter can silently shrink the shortlist.

**An itinerary agent assembling a bounded decision table for a traveler.**

Today (without this CLI): The agent collects IDs, names, facilities, notes, source URLs, starting prices, and observations from separate pages, then manually normalizes them before providing a booking handoff.

Weekly ritual: For each active itinerary revision, the agent reduces a regional result set to a small comparison table, evaluates explicit constraints, and flags the unanswered booking questions.

Frustration: It is difficult to produce compact, mechanically auditable evidence without quietly turning starting prices, false-plus-note facilities, or dated search results into stronger claims.

## Candidates (pre-cut)

The already absorbed `spots find`, `spots show`, and `spots handoff` are excluded from this candidate pool. Candidate scores use Domain Fit/User Pain/Build Feasibility/Research Backing, respectively; source captures from the same provider count as one research source rather than independent corroboration.

| # | Name | Command | Description | Persona served | Source | Kill/keep verdict | Long Description |
|---|------|---------|-------------|----------------|--------|-------------------|------------------|
| C1 | Evidence comparison | `spots compare ID ID [ID...]` | Join a capped shortlist's public details into a consistent price/facility/dimension/unknown matrix with source references. | Mika; itinerary agent | (a) persona-driven; (c) cross-station public detail join | Keep: bounded custom output exceeds a single endpoint; score 3/2/2/1=8. | Use this command for contrasting several stations. Use 'spots fit' for a vehicle or facility requirement decision, and 'spots audit' for unresolved evidence. |
| C2 | Explicit constraint assessment | `spots fit ID [ID...] --length-m N --width-m N --height-m N --require electricity,restroom` | Compare numeric parking-space dimensions and tri-state facility flags to explicit inputs; emit evidence and confirmation outcomes. | Mika; itinerary agent | (a) persona-driven; (b) parking dimensions and facility notifications | Keep: mechanical decisions only; vehicle acceptance remains unknown; score 3/2/2/1=8. | Use this command for explicit dimensional and facility constraints. Use 'spots compare' for a side-by-side shortlist, and 'spots audit' for missing or qualified source evidence. |
| C3 | Radial nearby ranking | `spots near --lat N --lon N --radius-km N --limit N` | Rank overnight-only directory records by Haversine distance from an explicit itinerary coordinate with stable ID ties. | Mika; itinerary agent | (a) persona-driven; (b) public station coordinates | Keep: local geodesic computation plus overnight filtering provides a useful ranking the observed endpoint does not return; score 3/2/2/1=8. | none |
| C4 | Language coverage audit | `spots coverage [--prefecture TEXT]` | Report overnight Japanese corpus coverage, English approval, actual translated-field presence, and IDs missed by each language criterion. | Alex; itinerary agent | (a) persona-driven; (b) partial English approval and translation fields | Keep: aggregate independent language signals; no machine translation; score 3/2/2/1=8. | none |
| C5 | Booking evidence audit | `spots audit ID [ID...] [--check-in DATE --check-out DATE]` | Emit unresolved fields, false flags accompanied by notifications, price basis, observation age, and dated-search claim limits for a bounded shortlist. | Itinerary agent; Mika | (a) persona-driven; (b) source ambiguity patterns | Keep: deterministic evidence matrix; score 3/2/2/1=8. | Use this command to inspect unresolved or qualified booking evidence. Use 'spots compare' for station contrasts, and 'spots fit' for explicit requirements. |
| C6 | Cheapest guaranteed winner | `spots cheapest --check-in DATE --check-out DATE` | Pick the lowest complete total for the stay. | Mika | (a) persona-driven | Kill: starting price and priceForSort do not establish a dated total, fees, or options. | none |
| C7 | Guaranteed vacancy | `spots available --check-in DATE --check-out DATE` | Label remaining spaces and bookable dates from recurring slots. | Mika; itinerary agent | (b) provider calendar content | Kill: server calendar/quote semantics are not verified; candidate search is not guaranteed inventory. | none |
| C8 | Any-vehicle acceptance badge | `spots accepted --vehicle campervan` | Certify that a station accepts any vehicle in a broad category. | Mika | (a) persona-driven | Kill: parking dimensions and vehicle access cannot establish unrestricted vehicle acceptance. | none |
| C9 | Translate all rules | `spots translate ID --language en` | Generate an English summary of Japanese rules. | Alex | (a) persona-driven | Kill: requires LLM/NLP not present in the public contract; preserve original fields instead. | none |
| C10 | Road-route optimizer | `spots roadtrip --from PLACE --to PLACE` | Optimize driving detours, tolls, and overnight sequence. | Mika | (a) persona-driven | Kill: road routing and place resolution require an external service and broader application scope. | none |
| C11 | Review mood scorer | `spots vibe ID` | Rank peaceful or family-friendly places by review sentiment. | Mika | (b) review content | Kill: semantic analysis is not deterministic, and the raw review payload carries unnecessary personal/payment fields. | none |
| C12 | Personal vacancy watcher | `spots watch ID --dates DATE,DATE` | Persist a background monitor and alert when a booking opens. | Mika | (a) persona-driven | Kill: unverified vacancy plus background scheduling exceeds the bounded public read-only CLI scope. | none |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona served | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|----------------|--------------|--------------|----------|------------------|
| 1 | Evidence comparison | `spots compare ID ID [ID...]` | 8/10 (3/2/2/1) | Mika; itinerary agent | hand-code | This uses the public station-detail endpoint for at most a small documented ID limit to compute a consistent price/facility/dimension matrix and evidence gaps with no external dependencies. | Brief Top Workflows3; directory dimensions all absent; Suzuka detail has 4m breadth/8m length; facility absence is common. | Use this command for contrasting several stations. Use 'spots fit' for a vehicle or facility requirement decision, and 'spots audit' for unresolved evidence. |
| 2 | Explicit constraint assessment | `spots fit ID [ID...] --length-m N --width-m N --height-m N --require electricity,restroom` | 8/10 (3/2/2/1) | Mika; itinerary agent | hand-code | This uses whitelisted public station details to compare positive finite input dimensions against parking-space meters and explicit facility flags, emitting unknown or confirmation results without external dependencies. | Brief Top Workflows4; bundle70 labels parking-space dimensions in meters; false-plus-notification facilities contradict categorical absence. | Use this command for explicit dimensional and facility constraints. Use 'spots compare' for a side-by-side shortlist, and 'spots audit' for missing or qualified source evidence. |
| 3 | Radial nearby ranking | `spots near --lat N --lon N --radius-km N --limit N` | 8/10 (3/2/2/1) | Mika; itinerary agent | hand-code | This uses the public Japanese station directory's longitude/latitude pairs to compute straight-line kilometers and filter/rank overnight rows around one supplied coordinate with no external dependencies. | Brief Top Workflows1; public directory has397 rows including54 activities; Suzuka coordinates are longitude136.46485525/latitude34.96122466. | none |
| 4 | Language coverage audit | `spots coverage [--prefecture TEXT]` | 8/10 (3/2/2/1) | Alex; itinerary agent | hand-code | This uses the public Japanese directory to compute overnight counts and separately measure approvedEn, actual nameEn presence, and their gaps with no external dependencies. | Brief Users English-speaking planner; observed397 Japanese/103 English corpus; source overnight subset343 with75 approvedEn and271 missing nameEn. | none |
| 5 | Booking evidence audit | `spots audit ID [ID...] [--check-in DATE --check-out DATE]` | 8/10 (3/2/2/1) | Itinerary agent; Mika | hand-code | This uses public station details and, when dates are supplied, provider date-filtered search observations to compute an explicit unresolved-evidence checklist with no external dependencies. | Brief Build Priorities unknowns; Suzuka shower false plus200JPY notification despite numeric price0; dated route total134/page20 with starting price2200. | Use this command to inspect unresolved or qualified booking evidence. Use 'spots compare' for station contrasts, and 'spots fit' for explicit requirements. |

**Adversarial answers, required for each survivor:**

1. Evidence comparison: The named traveler and itinerary agent use this at least weekly during active planning to revise stop choices. It is not an endpoint rename: several detail records are aligned, unknowns retained, and comparable evidence emitted. Its power comes from a cross-station join and agent-shaped output. Closest killed sibling C6 pretended an incomplete starting price could identify a complete-cost winner. `hand-code` is required for the custom matrix and Cobra wiring. Its Long Description references only surviving `spots fit` and `spots audit`.
2. Explicit constraint assessment: The named campervan traveler runs this at least weekly during active planning whenever another stop enters the shortlist. It is not an endpoint rename: caller constraints drive numeric and tri-state comparisons. Its power comes from parking-space content patterns and evidence-shaped decisions; a dimensional pass is explicitly not vehicle acceptance. Closest killed sibling C8 made a category acceptance claim unsupported by dimensions. `hand-code` is required for input validation and comparison logic. Its Long Description references only surviving `spots compare` and `spots audit`.
3. Radial nearby ranking: The named traveler and itinerary agent run this at least weekly during active planning for the next stop coordinate. It is not an endpoint rename: the CLI computes geodesic rankings absent from the observed directory response. Its power comes from Carstay's station coordinates plus activity exclusion and bounded agent output. Closest killed sibling C10 required road routing; this survivor labels results as straight-line radial proximity, with no route or drive-time claim. `hand-code` is required for Haversine, coordinate validation, deterministic ties, and Cobra wiring. Its Long Description is `none`; no sibling redirect is needed.
4. Language coverage audit: The named English planner runs this at least weekly during active itinerary revision to check which regional options the chosen language view omitted. It is not an endpoint rename: the CLI separately counts approval and field presence, including their intersection and missing IDs. Its power comes from Carstay's asymmetric locale content and compact audit output. Closest killed sibling C9 attempted to fill translation gaps with an LLM; this survivor exposes gaps without fabricating translations. `hand-code` is required for aggregate coverage and subset logic. Its Long Description is `none`; no sibling redirect is needed.
5. Booking evidence audit: The itinerary agent and traveler run this at least weekly during active planning before treating a revised shortlist as ready for handoff. It is not an endpoint rename: it checks a defined field set, detects false-plus-notification mechanically, reports price basis and observation age, and limits dated claims. Its power comes from service-specific missing/qualified evidence patterns and agent-shaped output. Closest killed sibling C7 converted date filtering into inventory certainty; C12 further added an unsupported monitor. `hand-code` is required for the evidence checks and optional candidate-search join. Its Long Description references only surviving `spots compare` and `spots fit`.

All five features can share small public-fetch and whitelist helpers; each should fit a bounded hand-written command rather than a new application. Use exactly one `// pp:data-source live` directive if reading only public HTTP. If existing generated cache/store support is reused, choose `auto` or `local` accurately and honor mode rejection and freshness/sync hint rules. Use generated client/config seams and check HTTP status before decoding; a custom wrapper invisible to dogfood needs the documented `// pp:client-call` marker only for a real public API call. No auth environment reads are needed.

If local SQLite is used for any comparison or observation cache, drain and close parent rows before follow-up queries, check rows.Err, and never call pooled store upserts inside an open custom write transaction. Keep upstream observations separate from derived rankings and decisions; empty collections remain arrays.

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|--------------------------|
| C6 Cheapest guaranteed winner | The source exposes starting/sort prices rather than complete dated totals, so the winner would be fabricated. | `spots compare` |
| C7 Guaranteed vacancy | Recurring slots and provider date filtering do not establish exact remaining inventory. | `spots audit` |
| C8 Any-vehicle acceptance badge | Parking space and generic access evidence do not certify acceptance of every vehicle. | `spots fit` |
| C9 Translate all rules | The promised output requires an LLM and would replace original evidence with generated language. | `spots coverage` |
| C10 Road-route optimizer | Road distance, tolls, place resolution, and route optimization require external infrastructure beyond this public source. | `spots near` |
| C11 Review mood scorer | Sentiment classification is not a deterministic contract feature and requires unnecessarily sensitive raw review processing. | `spots compare` |
| C12 Personal vacancy watcher | Background monitoring expands scope around an inventory state the public contract has not verified. | `spots audit` |

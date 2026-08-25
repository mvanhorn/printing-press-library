## Customer model

**Maya, the multi-tab trip builder**

Today, Maya opens Wanderlog destination pages, several public guides, Google Maps tabs, and a half-finished personal itinerary. She copies promising places by hand, then loses track of which places appeared in multiple guides versus which came from one random list.

Weekly ritual, she picks a destination, resolves the Wanderlog geo, browses guide-rich trips, scans top attractions/restaurants, and turns repeated recommendations into a rough day-by-day plan.

Frustration, Wanderlog exposes the raw material but not the consensus. Maya cannot quickly answer "which places keep showing up across guides and category lists, and which ones are already in my trip?"

**Jon, the client itinerary planner**

Today, Jon receives shared Wanderlog itineraries from clients or collaborators and manually compares them against his working trip copy. He exports bits to documents, checks day density by eye, and tries not to miss deleted or renamed stops.

Weekly ritual, he pulls shared itineraries into structured formats, reconciles them with personal or client trips, and prepares polished handoff notes.

Frustration, the expensive part is comparison. Wanderlog can show one itinerary, but it does not give Jon a mechanical diff of days, sections, places, missing coordinates, notes, or changed ordering.

**Priya, the travel-content miner**

Today, Priya researches destinations by opening public guides, explore pages, category lists, place cards, comments, and distinction metadata. She saves promising places into spreadsheets before writing destination content.

Weekly ritual, she mines several destinations for high-signal recommendations, looking for places that are repeated across guides, appear in curated category lists, and have rich metadata.

Frustration, each Wanderlog surface is useful alone, but the reusable insight lives across surfaces. She needs a local cross-guide index, not another endpoint wrapper.

**Nico, the agentic planning builder**

Today, Nico wires natural-language planning agents to individual Wanderlog commands: geo autocomplete, guide fetch, place search, place details, trip reads, and exports. He then has to manually decide which payloads are compact enough for an agent context.

Weekly ritual, he builds scripts that turn a destination and trip length into agent-readable planning context, then optionally checks the result against a cookie-backed trip.

Frustration, raw Wanderlog JSON is too wide and fragmented. The CLI needs a deterministic planning bundle that selects the right local records without requiring an LLM inside the CLI.

## Candidates (pre-cut)

1. **Guide consensus map**  
   Command: `guides consensus --geo <geo_id>`  
   Description: Rank places by repeated appearance across public guides, category lists, and place details for a destination.  
   Persona served: Maya, Priya  
   Source label: (b) service-specific content patterns, (c) cross-entity local queries  
   Long Description: `Use this command to rank destination-wide public-guide consensus. Do NOT use it to compare two specific itineraries; use 'itinerary compare' instead.`  
   Kill/keep: Keep. No LLM, no external service, buildable from synced guide/place/category local data.

2. **Itinerary compare**  
   Command: `itinerary compare --left <guide_or_trip_key> --right <guide_or_trip_key>`  
   Description: Diff two Wanderlog trip plans by day, section, place identity, order, notes, and missing metadata.  
   Persona served: Jon, Maya  
   Source label: (a) persona-driven, (c) cross-entity local queries  
   Long Description: `Use this command to compare two known Wanderlog itineraries. Do NOT use it for destination-wide recommendation mining; use 'guides consensus' instead.`  
   Kill/keep: Keep. Local deterministic diff, no ShareDB mutation, no NLP.

3. **Trip load audit**  
   Command: `trip load --trip-key <trip_key>`  
   Description: Show day-by-day stop counts, missing coordinates, note density, and section imbalance for a trip.  
   Persona served: Maya, Jon  
   Source label: (a) persona-driven, (c) cross-entity local queries  
   Long Description: none  
   Kill/keep: Keep. Not a thin endpoint wrapper because it computes cross-day itinerary load from local trip resources.

4. **Agent planning bundle**  
   Command: `plan bundle --geo <geo_id> --days <n>`  
   Description: Emit compact agent-ready JSON from synced geos, guides, category lists, place details, and consensus signals.  
   Persona served: Nico, Maya  
   Source label: (a) persona-driven, (c) cross-entity local queries  
   Long Description: none  
   Kill/keep: Keep. Mechanical selection and shaping only; no embedded LLM or summarization.

5. **Place canonicalizer**  
   Command: `places canonicalize --scope <trip|geo> --id <id>`  
   Description: Detect duplicate or near-duplicate Wanderlog places across trip stops, guide resources, and category lists using ids, names, addresses, and coordinates.  
   Persona served: Maya, Priya, Jon  
   Source label: (c) cross-entity local queries  
   Long Description: `Use this command to find duplicate place records inside one synced scope. Do NOT use it to compare itinerary membership; use 'itinerary compare' instead.`  
   Kill/keep: Keep. Mechanical identity reconciliation; no third-party geocoder.

6. **Trip readiness audit**  
   Command: `trip readiness --trip-key <trip_key>`  
   Description: Check a personal trip for missing coordinates, empty days, unassigned places, sparse notes, checklist gaps, expense gaps, and export blockers.  
   Persona served: Jon, Maya  
   Source label: (a) persona-driven, (c) cross-entity local queries  
   Long Description: none  
   Kill/keep: Keep. Uses cookie-backed trip read data already in scope; no mutations.

7. **Destination dossier**  
   Command: `destination dossier --geo <geo_id>`  
   Description: Produce one destination-level JSON dossier from guides, category lists, comments, distinctions, and top place cards.  
   Persona served: Priya, Nico  
   Source label: (b) service-specific content patterns  
   Long Description: none  
   Kill/keep: Cut in Pass 3. Too close to `plan bundle`, with weaker weekly use and a vaguer output contract.

8. **Place-list coverage radar**  
   Command: `places coverage --geo <geo_id>`  
   Description: Compare public guides against top attractions/restaurants category lists to show under-covered but high-ranking places.  
   Persona served: Priya  
   Source label: (b) service-specific content patterns, (c) cross-entity local queries  
   Long Description: none  
   Kill/keep: Cut in Pass 3. Useful, but mostly a narrower view of `guides consensus`.

9. **Client handoff pack**  
   Command: `handoff pack --trip-key <trip_key>`  
   Description: Emit Markdown, CSV, JSON, and KML together with comments and distinction metadata for client delivery.  
   Persona served: Jon  
   Source label: (a) persona-driven  
   Long Description: none  
   Kill/keep: Cut in Pass 3. Drifts into packaging already covered by export paths; weaker transcendence than itinerary comparison.

10. **Cookie auth doctor**  
    Command: `auth doctor`  
    Description: Validate `WANDERLOG_COOKIE`, show account identity, trip visibility, and likely expired-session errors.  
    Persona served: Nico, Jon  
    Source label: (a) persona-driven  
    Long Description: none  
    Kill/keep: Cut. Useful setup helper, but not a weekly differentiated feature and mostly wraps account/trips reads.

11. **Guide theme summarizer**  
    Command: `guides themes --geo <geo_id>`  
    Description: Summarize recurring food, neighborhood, and attraction themes across guides.  
    Persona served: Priya  
    Source label: (b) service-specific content patterns  
    Long Description: none  
    Kill/keep: Cut. Fails LLM dependency unless reduced to mechanical counts, which is already covered by `guides consensus`.

12. **Batch trip editor apply**  
    Command: `trip edit apply --file ops.json`  
    Description: Apply a batch of place/note/date mutations to a Wanderlog trip.  
    Persona served: Maya, Jon  
    Source label: (a) persona-driven  
    Long Description: none  
    Kill/keep: Cut. Requires unverified ShareDB websocket JSON0 mutations, explicitly outside the generated endpoint spec unless separately approved.

13. **Offline map pack**  
    Command: `offline pack --trip-key <trip_key>`  
    Description: Generate KML, CSV, and Markdown variants optimized for offline map tools.  
    Persona served: Maps/offline-export traveler  
    Source label: (a) persona-driven  
    Long Description: none  
    Kill/keep: Cut. Existing absorbed export KML/CSV/agent modes already cover the core value.

14. **Comments and distinction leaderboard**  
    Command: `guides social-rank --geo <geo_id>`  
    Description: Rank guides by comments, likes, distinction metadata, and place richness.  
    Persona served: Priya  
    Source label: (b) service-specific content patterns  
    Long Description: none  
    Kill/keep: Cut in Pass 3. Interesting, but weaker weekly pain than ranking places directly with `guides consensus`.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Guide consensus map | `guides consensus --geo <geo_id>` | 8/10 | hand-code | This uses local SQLite rows synced from public guides, place details/card data, and geo category lists to count repeated place appearances and emit ranked places with no external dependencies; source file should use `// pp:data-source local` and sync/stale hints for guide/place/place_list resources. | Weekly use: Maya and Priya run it during destination research; wrapper vs leverage: cross-guide/category join, not one endpoint; transcendence: local consensus query; sibling kill: `places coverage` is narrower; evidence: brief destination guide ritual, place research ritual, guide-rich destinations, public guides, category lists, Wanderlog-to-KML/importer reconciliation patterns. | Use this command to rank destination-wide public-guide consensus. Do NOT use it to compare two specific itineraries; use `itinerary compare` instead. |
| 2 | Itinerary compare | `itinerary compare --left <guide_or_trip_key> --right <guide_or_trip_key>` | 7/10 | hand-code | This uses local SQLite trip plan/resource records from public guide reads and cookie-backed trip reads to compute deterministic day, section, place, note, and ordering diffs with no external dependencies; source file should use `// pp:data-source local` and sync/stale hints for trip/guide resources. | Weekly use: Jon runs it for client shared itinerary reconciliation; wrapper vs leverage: structural diff across two trips; transcendence: local cross-entity comparison; sibling kill: `handoff pack` is mostly packaging; evidence: shared itinerary export ritual, authenticated personal trip ritual, MCP trip reads, shared view browser-sniff. | Use this command to compare two known Wanderlog itineraries. Do NOT use it for destination-wide recommendation mining; use `guides consensus` instead. |
| 3 | Trip load audit | `trip load --trip-key <trip_key>` | 7/10 | hand-code | This uses local trip days, sections, stops, notes, coordinates, and resource metadata to compute per-day load and imbalance metrics with no external dependencies; source file should use `// pp:data-source local` and sync/stale hints for trip resources. | Weekly use: Maya and Jon run it while shaping day-by-day plans; wrapper vs leverage: computed itinerary metrics; transcendence: service-specific trip structure; sibling kill: `trip readiness` covers broader completion, this survives for load-specific planning; evidence: brief day-by-day itinerary workflow, shared itinerary export ritual, data layer days/blocks/stops/places/routes. | none |
| 4 | Agent planning bundle | `plan bundle --geo <geo_id> --days <n>` | 8/10 | hand-code | This uses local geos, guide summaries, guide tripPlans, category lists, place details/card data, and consensus scores to emit compact deterministic planning JSON with no external dependencies; source file should use `// pp:data-source local` and sync/stale hints across local resources. | Weekly use: Nico runs it as the bridge from destination to planning-agent context; wrapper vs leverage: agent-shaped multi-resource bundle; transcendence: compact local synthesis; sibling kill: `destination dossier` is less actionable; evidence: agentic workflow builder user, product thesis agent-ready trip planning, public guide mining, session/geos/guides/places resources. | none |
| 5 | Place canonicalizer | `places canonicalize --scope <trip|geo> --id <id>` | 6/10 | hand-code | This uses local place ids, names, addresses, coordinates, trip stops, guide resources, and category-list entries to flag duplicate or conflicting place records with no external dependencies; source file should use `// pp:data-source local` and sync/stale hints for the selected scope. | Weekly use: Maya and Priya run it when reconciling places before exporting or writing; wrapper vs leverage: identity reconciliation across resources; transcendence: local duplicate detection; sibling kill: `offline pack` does not solve data quality; evidence: import reconciliation ritual, Wanderlog importer matched/missing/name-mismatch audit, place search/details/card resources. | Use this command to find duplicate place records inside one synced scope. Do NOT use it to compare itinerary membership; use `itinerary compare` instead. |
| 6 | Trip readiness audit | `trip readiness --trip-key <trip_key>` | 7/10 | hand-code | This uses local cookie-backed trip resources, stops, notes, coordinates, checklists, expenses, hotels, and export-relevant fields to emit a readiness report with no external dependencies; source file should use `// pp:data-source local` and sync/stale hints for trip resources. | Weekly use: Jon runs it before client handoff and Maya runs it before offline export; wrapper vs leverage: multi-section quality audit; transcendence: service-specific readiness checks; sibling kill: `auth doctor` only validates setup; evidence: data layer includes budgets/checklists/expenses/hotel deals, maps/offline-export traveler, shared itinerary export ritual. | none |

### Killed candidates

| feature | kill reason | closest-surviving-sibling |
|---|---|---|
| Destination dossier | Vague "all-in-one" output with weaker weekly action than a compact planning bundle. | Agent planning bundle |
| Place-list coverage radar | Useful but too narrow; guide/category under-coverage is a subview of consensus ranking. | Guide consensus map |
| Client handoff pack | Mostly bundles existing export formats and risks duplicating absorbed KML/CSV/Markdown export behavior. | Itinerary compare |
| Cookie auth doctor | Setup utility, not a weekly differentiating feature; mostly wraps account/trips reads. | Trip readiness audit |
| Guide theme summarizer | Requires NLP-style summarization; mechanical counts are already handled by consensus. | Guide consensus map |
| Batch trip editor apply | Requires unverified ShareDB websocket mutations and authenticated fixture validation outside the current spec. | Itinerary compare |
| Offline map pack | Core value already absorbed by export KML/CSV paths and split-by-date behavior. | Trip readiness audit |
| Comments and distinction leaderboard | Social ranking is weaker than place-level recommendation mining and likely less frequently used. | Guide consensus map |

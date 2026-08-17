# iNaturalist CLI Brief

## API Identity

- Domain: iNaturalist is a community biodiversity-observation platform used to
  discover wildlife, record observations, and identify organisms.
- Users: field naturalists checking recent local biodiversity; educators and
  families designing real-world nature activities; observers tracking whether
  their own observations have gained community identifications; researchers
  working with bounded, rate-conscious observation queries.
- Data profile: observations, taxa, identifications, places, projects, users,
  and derived species counts. Observation records can carry geoprivacy states
  and authenticated responses can contain private location fields.

## Reachability Risk

- Low for public read operations. `GET https://api.inaturalist.org/v1/swagger.json`
  returned 200 on 2026-07-24 and declares iNaturalist API v1.3.0 with 86 paths.
- The human-facing recommended-practices page returned 403 to raw curl during
  research; the official API and Swagger endpoint remained reachable. This is
  documentation-host reachability evidence, not an API block.
- API policy: use the newer API, make bounded filtered requests, keep around
  one request per second and below roughly 10k requests/day; use exports or
  GBIF for bulk data.
- Privacy: official schema documents `geoprivacy`, `obscured`,
  `private_location`, `private_geojson`, `private_place_guess`, and
  `private_place_ids`; authenticated GETs may contain hidden coordinates.

## Top Workflows

1. Find interesting, recent wildlife around a supplied place or radius without
   converting a public observation into a location-disclosure tool.
2. Build a factual, balanced nature scavenger hunt from taxa actually observed
   in a supplied area and time window.
3. Check a named observer's current identification state and identify which of
   their observations changed after a previous local sync.
4. Search, inspect, filter, and safely sync the full public iNaturalist API
   surface: observations, taxa, identifications, places, projects, and users.
5. Use scriptable, agent-native output while respecting public API limits.

## Table Stakes

- Official v1 Swagger: observation and identification search/details/stats;
  taxa, places, projects, users, controlled terms, and authenticated CRUD.
- `cvsouth/inaturalist-mcp`: observations, species counts, taxa, places,
  projects, similar species, and universal search.
- `ufo2243/inaturalist-mcp`: public observation/taxon/place/project/user
  lookups; histogram, observers/identifiers, quality and identification
  statistics; identification search and nearby-species conveniences.
- `tamnd/inaturalist-cli`: portable plain-HTTPS command-line retrieval and
  pipe-friendly table/JSON/CSV/TSV output.

## Data Layer

- Primary entities: observations, taxa, identifications, places, projects,
  users, and only privacy-redacted compound snapshots.
- Sync cursor: bounded observation updates by ID/time. Store only public,
  privacy-safe derived fields for novel-feature histories; never persist API
  `private_*` fields or coordinate geometry for compound commands.
- FTS/search: taxon names, common names, observation species guesses, and
  non-private observer-facing metadata.

## Product Thesis

- Name: iNaturalist CLI
- Why it should exist: existing wrappers expose excellent point lookups and
  counts, while this CLI provides a rate-conscious local workflow for nearby
  field briefings, factual scavenger hunts, and honest identification progress
  without weakening iNaturalist location privacy.

## User Vision

- “What interesting wildlife has been spotted nearby?”
- “Create a nature scavenger hunt.”
- “Check whether my observations got identified.”

## Build Priorities

1. Preserve official API privacy states and never surface private or inferred
   locations in command output, local store, docs, examples, or MCP tools.
2. Generate the complete official API surface with safe mutation defaults,
   bounded/rate-conscious reads, local SQLite state, JSON/agent output, and
   runtime MCP parity.
3. Implement all five approved compound commands exactly as approved.

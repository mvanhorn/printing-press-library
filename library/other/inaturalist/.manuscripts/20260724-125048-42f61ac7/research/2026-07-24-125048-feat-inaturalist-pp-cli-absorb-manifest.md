# iNaturalist Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Full official v1 public read and authenticated write surface (86 API paths) | Official Swagger v1.3.0 | (generated endpoint) official Swagger resources/endpoints | Cobra, JSON/agent output, local sync/search, dry-run protections, and MCP parity. |
| 2 | Observation, species-count, taxon, place, project, similar-species, and universal search | cvsouth/inaturalist-mcp | (generated endpoint) observations/taxa/places/projects/search | Full official endpoint coverage plus local state and privacy-safe compound workflows. |
| 3 | Observation/taxon/place/project/user detail; histogram, observer/identifier, quality and identification stats; controlled terms | ufo2243/inaturalist-mcp | (generated endpoint) observations/identifications/taxa/places/projects/users/controlled-terms | Full v1 parity, structured output, and bounded synchronization. |
| 4 | Pipe-friendly plain HTTPS retrieval with table/JSON/CSV/TSV output | tamnd/inaturalist-cli | (behavior in inaturalist-pp-cli output flags) typed resource commands with `--json`, `--agent`, `--select`, `--csv` | Typed API contract, local SQL/FTS, and agent-native MCP surface. |
| 5 | Public taxa lookup | cssnr/inaturalist-mcp | (generated endpoint) taxa autocomplete/get | Includes the complete public API and taxon-backed field workflows. |

## Transcendence (approved shipping scope)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Nearby highlights | `nearby highlights` | 10/10 | hand-code | Uses bounded `GET /observations/species_counts` and `GET /observations` results to explain a transparent recent/biodiversity ranking; compound output omits all coordinates and preserves `geoprivacy`/`obscured`. | User vision; cvsouth `get_species_counts`; ufo `get_recent_species_nearby`; official Observation Search and Species Counts. | Use this command for an explanation-backed wildlife briefing near an explicit supplied area. Do NOT use it to recover observation locations or interpret obscured records; use ordinary observation lookup only for the API's returned public metadata. |
| 2 | Nature scavenger hunt | `hunt create` | 9/10 | hand-code | Uses real local species-count results to print a balanced taxonomy-aware checklist with source taxon IDs/names and no observation locations. | User vision; official taxa/species-count endpoints; nearby-species MCP workflows. | none |
| 3 | Current identification status | `observations id-status` | 10/10 | hand-code | Uses bounded observation and identification fields to classify a named observer's observations as identified, needs-ID, disagreement, or no-taxon without emitting coordinates or private-place data. | User vision; official Observation Search and Identification Search/Categories; ufo identification/quality tools. | Use this command for the current state of an observer's observations. Do NOT use it for historical change detection; use `observations id-changes` after a privacy-safe sync. |
| 4 | Identification changes since sync | `observations id-changes` | 9/10 | hand-code | Compares live public identification state with earlier privacy-redacted local snapshots to report newly community-identified, changed, withdrawn, and still-needing-ID observations; it never fabricates history. | User vision; official `current`, `previous_observation_taxon`, and community-taxon schema; existing MCP tools only offer point-in-time summaries. | Use this command for changes since a prior sync. Do NOT use it as a current-only summary; use `observations id-status` instead. |
| 5 | Seasonal shift briefing | `nearby seasonal-shift` | 8/10 | hand-code | Compares two explicit bounded date windows through real species-count/observation results and explains taxa newly appearing, returning, or materially changing; output contains no coordinates. | Official date-filtered Observation Search, Species Counts, Histogram; ufo histogram/nearby-species tools; recurring field-visit workflow. | Use this command to compare two field windows for an explicit area. Do NOT use it to locate records or predict undisclosed observations; use `nearby highlights` for a single-window briefing. |

## Exclusions and safety constraints

- Never expose, persist, infer, reconstruct, de-obscure, or derive from
  `private_location`, `private_geojson`, `private_place_guess`,
  `private_place_ids`, or coordinates from obscured/private records.
- Compound commands omit coordinates entirely. They preserve the upstream
  `geoprivacy` and `obscured` status as a label where relevant.
- Do not scrape or bulk-download media; do not exceed documented query limits.
- All remote mutations retain Printing Press dry-run and explicit-confirmation
  semantics. The five approved novel commands are read-only.

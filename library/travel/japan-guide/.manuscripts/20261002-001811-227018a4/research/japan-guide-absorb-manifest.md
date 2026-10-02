# Absorb manifest
User delegated focused source scope and routine gate decisions. No community wrapper contributed features. Novel feature brainstorming performed directly under the shared brief's exactly-one-reviewer delegation constraint, which overrides generic extra subagents.

### Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Destination discovery | Japan Guide directory | japan-guide-pp-cli guide destinations | Region, keyword, paging, compact JSON |
| 2 | Attraction discovery | Destination HTML | japan-guide-pp-cli guide attractions | Interest filtering; side-trip and event identity |
| 3 | Visit information | Attraction HTML | japan-guide-pp-cli guide inspect | Facility schedules, qualifiers, unknowns, update dates |
| 4 | Interest directory | Source interest HTML | japan-guide-pp-cli guide interests | Stable source link IDs |
| 5 | Source itineraries | Source suggestions | japan-guide-pp-cli guide itineraries | Index or destination, bounded list |
| 6 | Source itinerary stops | Source itinerary HTML | japan-guide-pp-cli guide itinerary | Bounded factual day/stop labels and links |

### Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Small-list planning comparison | guide compare | hand-code | Bounded source visit facts side by side, per-item partial errors | none |
| 2 | Scoped schedules | guide inspect | hand-code | Preserve museum vs garden vs shrine fact identity | none |
| 3 | Explicit opening uncertainty | guide inspect | hand-code | Prevent schedules being treated as live open state | none |
| 4 | Offline facts with freshness | guide inspect | hand-code | Compact local cache, source and fetched timestamps, explicit cache provenance | none |
| 5 | Resource budgets | guide destinations | hand-code | One directory fetch, compact page results, request/byte/timing metrics | none |

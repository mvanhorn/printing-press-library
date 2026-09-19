# Haven Hot Chicken absorb manifest

Scope: public consumer menu/location reads only. No Haven-specific competing CLI/SDK/MCP was found in GitHub, npm, PyPI or plugin searches. Official website and Thanx public interface supplied the contract. User delegated Codex workflow adaptation and ordinary implementation judgment; no external provider used.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Locations and details | Haven/Thanx | (generated endpoint) locations | Structured public data |
| 2 | Menus and categories | Haven/Thanx | (generated endpoint) categories | Structured item prices and availability |
| 3 | Item modifier constraints | Haven/Thanx | (generated endpoint) modifiers | Required choices and option prices |
| 4 | Save location menus | Observed public API | haven-hot-chicken-pp-cli haven refresh | Complete timestamped SQLite observations |
| 5 | Read/search saved menu | Official menu UI | haven-hot-chicken-pp-cli haven menu | Offline query and bounded output |
| 6 | Read saved locations | Official directory | haven-hot-chicken-pp-cli haven locations | Offline location lookup |

## Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Location price comparison | haven compare | hand-code | Join location-scoped menu observations without switching storefronts. | Compare the same named menu item across saved location menus. |
| 2 | Base-price meal subtotal | haven subtotal | hand-code | Combine saved prices with quantities without creating a remote cart. | Estimate a meal base-price subtotal from explicit item IDs and quantities. |
| 3 | Saved menu changes | haven changes | hand-code | Retain observation history the current-menu API does not expose. | Show additions, removals, price and availability changes between two complete saved menus. |
| 4 | Shared availability | haven common | hand-code | Intersect location-specific availability with observation timestamps. | Find items marked available in every selected saved location menu. |
| 5 | Nearby locations | haven nearby | hand-code | Calculate distances locally without sending coordinates to a service. | Rank saved shops by straight-line distance from supplied coordinates. |

Five hand-authored features, no stubs. Names are exact normalized string matches, not inferred product equivalence. Cached timestamps are always included. Nearby is straight-line distance; subtotal excludes options, tax, tips and fees. Changes requires two complete observations of the same location.

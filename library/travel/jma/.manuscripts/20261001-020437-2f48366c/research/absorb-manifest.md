# Focused JMA manifest (user preauthorized)

## Absorbed
| Feature | Source | Our Implementation | Value |
|---|---|---|---|
| Forecast today / week | JMA frontend; japan-weather-mcp | jma-pp-cli forecast get | Filter by resolved source district and temperature station, six-hour probabilities, confidence |
| Cities / stations | JMA catalogs; jma-data-mcp | jma-pp-cli areas search | Source names and hierarchy, bounded pagination |
| Temperature station lookup | JMA catalogs; jma-data-mcp | jma-pp-cli stations search | Explicit forecast reference station, parent district |
| Warnings/advisories | JMA r8 frontend | jma-pp-cli warnings get | Product IDs, hazard IDs, new severity, lifecycle, source wording |
| Typhoon information | JMA map frontend | jma-pp-cli typhoons get | Analysis/estimate/forecast, units and uncertain circles |

## Transcendence (focused improvements; user prohibits separate ideation agents)
| Feature | Command | Buildability | Acceptance |
|---|---|---|---|
| Explicit source resolution | areas resolve | hand-code | Reject ambiguous names, accept canonical IDs; forecast district and office returned |
| Warning completeness | warnings get | hand-code | Per product/municipality records; unknown/empty/failure never no-warning |
| Coherent typhoon detail | typhoons get | hand-code | Require matching issue times and valid-time joins across both documents |
| Bounded lazy discovery | typhoons list | hand-code | List one request, fetch detail only by selected ID |
| Auditable inventory refresh | inventory refresh | hand-code | Validate and atomically replace source catalog; timestamp, offline reads and provenance |

No stubs. No personal safety clearance. River/coastal-zone warning supplements explicitly partial; source table and properties preserved in --detail. Out-of-scope incumbent features: observations/history/geocoding, other providers. SQLite/sync/query framework retained only if generated baseline requires it; domain reads use bounded source caches and inventory, avoiding broad sync of transient weather.

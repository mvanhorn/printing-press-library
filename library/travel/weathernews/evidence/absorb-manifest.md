# Weathernews absorb manifest
User authorization: explicit focused read-only scope and ordinary gates preapproved.

## Absorbed
| Feature | Best source | Our Implementation | Value |
|---|---|---|---|
| Location search | Weathernews first-party search / weathernews-python | weathernews-pp-cli places resolve | exact Japanese identity, ambiguity preserved |
| Hourly/daily forecast | Public homepage client / Python wrapper/gist | weathernews-pp-cli weather forecast | bounded units/time/provenance, no third-party geocoder |
| Observation | Public forecast response | (behavior in weathernews-pp-cli weather forecast) separate observation | measured vs predicted preserved |

## Focused travel features
| Feature | Our Implementation | Buildability |
|---|---|---|
| Seasonal inventories | weathernews-pp-cli season search | hand-code |
| Seasonal detail evidence | weathernews-pp-cli season show | hand-code |
| Transparent criteria comparison | weathernews-pp-cli weather compare | hand-code |
| Seasonal date evidence comparison | weathernews-pp-cli season compare | hand-code |
| Inventory refresh and compact provenance | (behavior in weathernews-pp-cli season search) refresh/projection/pagination | hand-code |

Commercial MCP endpoints and raw news/user-report ingestion excluded by user focused scope. No stubs in shipping scope. Dates absent or unavailable are product states, not placeholders. Builder performs brainstorm and output audit due exactly-one-reviewer limit.

# CAROL Bike CLI Absorb Manifest

## Shipping Scope
- Seven browser-observed CAROL Rider API operations, all GET-only.
- Standard generated bearer authentication and rider-scoped configuration.
- Standard generated local SQLite sync, search, and SQL surfaces.

## Absorbed Features
| Feature | Source | Shipping Command |
|---|---|---|
| Latest ride | CAROL dashboard | `ride get-latest` |
| REHIT ride history | CAROL dashboard | `ride list-rehit-rides` |
| Rider totals and frequency | CAROL dashboard | `stats get-rider-stats`, `stats get-ride-count`, `stats get-rides-per-week` |
| Recent ride calendar | CAROL dashboard | `stats get-ride-calendar` |
| Rider trends | CAROL dashboard | `trends get-rider-trends` |

## Transcendence Feature
| Name | Command | Implementation | Acceptance |
|---|---|---|---|
| Private ride mirror | `sync` | Generator-provided SQLite sync/store; no hand-written command | `sync --full --max-pages 1 --dry-run --json` exits 0; full live dogfood validates the real read-only workflow. |

## Excluded
- Account, subscription, checkout, bike-control, and workout mutation endpoints.
- The rejected custom `summary` command.
- Any credential, rider identifier, HAR response body, or private workout payload.

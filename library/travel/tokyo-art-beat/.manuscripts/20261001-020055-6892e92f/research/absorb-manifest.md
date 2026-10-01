# Focused approved absorb manifest

Approval: user preauthorized sensible public read-only scope, sole builder and routine gates. No account writes or premium coupon redemption. Website app is the only feature source; no dedicated SDK/MCP found.

### Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Bilingual event discovery | Tokyo Art Beat public website | tokyo-art-beat-pp-cli events search | area/category/artist/venue filters, concise cards |
| 2 | Bilingual event detail | Tokyo Art Beat public website | tokyo-art-beat-pp-cli events detail | edition source ID/URL, event fields and venue identity |
| 3 | Venue discovery/detail | Tokyo Art Beat public website | tokyo-art-beat-pp-cli venues search | location and source admission/hours kept separate |
| 4 | Area/category/type IDs | Tokyo Art Beat public website | tokyo-art-beat-pp-cli catalogs areas | bounded finite catalogs with Japanese names |
| 5 | Venue exhibition schedule | Tokyo Art Beat public website | tokyo-art-beat-pp-cli venues events | events linked to exact venue ID |

### Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Trip window overlap | events search | hand-code | inclusive full-year date span; not an open-day guarantee | none |
| 2 | Starting/ending windows | events search | hand-code | --relation starts/ends around trip dates | none |
| 3 | Day closure assessment | events detail | hand-code | --on with conservative unknowns for exceptions and holidays | none |
| 4 | Nearby shortlist | nearby | hand-code | bounded candidate venues and explainable straight-line distance | none |
| 5 | Fresh compact agent results | events search | hand-code | --fields, --fresh and --offline with structured completeness | none |

Scope: all rows implemented. Unknown closures, ticket availability, unavailable last-admission fields remain explicit; no stubs. Sole-builder user instruction supersedes brainstorming delegation.

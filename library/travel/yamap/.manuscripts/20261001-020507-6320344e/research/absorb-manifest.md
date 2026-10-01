# Authorized YAMAP focused manifest
The user preauthorized focused read-only scope and ordinary gates. No stubs. Novel-feature brainstorming performed by sole builder as explicitly required by user, overriding skill's separate brainstorm-worker requirement. Scope excludes exporter account backups/photos/GPX and all mutation/membership operations.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Mountain search/detail | YAMAP website | yamap-pp-cli mountains search | Japanese names, IDs, prefectures, lazy detail |
| 2 | Reference route search/detail | YAMAP website | yamap-pp-cli routes search | Correct name wire filter, planned semantics |
| 3 | Activity search/detail | YAMAP website and yamap-export | yamap-pp-cli reports search | Recorded semantics, compact units, lazy detail |
| 4 | Map search/detail | YAMAP website | yamap-pp-cli maps search | Bounds, map coverage and canonical links |
| 5 | Mountain-specific courses | YAMAP first-party API | yamap-pp-cli mountains routes | Actual relationship instead of ignored query param |
| 6 | Mountain-specific logs | YAMAP first-party API | yamap-pp-cli mountains reports | Mountain relationship vs broad text hit |
| 7 | Explicit cache refresh/projection | Focused agent workflow | (behavior in yamap-pp-cli reports search) --refresh and --select | Small bounded reads with fetch timestamps |

## Transcendence
| # | Feature | Command | Buildability | Why Useful | Long Description |
|---|---------|---------|--------------|------------|------------------|
| 1 | Activity-date recent evidence | reports recent | hand-code | Bounded scan, old uploaded trips not counted recent | Recent only within scanned candidate pages, not complete inventory |
| 2 | Contributor observation excerpts | reports observations | hand-code | Explicit observation source/date, no closure claims | Report text is contributor evidence, not authoritative advice |
| 3 | Planned versus recorded metric comparison | routes compare | hand-code | Preserve different time definitions and no route-equivalence claim | Numeric comparison does not verify track overlap |
| 4 | Map area coverage facts | maps coverage | hand-code | Bounds/version/deprecated status and no offline navigation claim | Map area coverage does not establish trail coverage or open status |
| 5 | Cached inventory diagnosis | inventory status | hand-code | Explicit counts/freshness/partial cache, no silent refresh | Exact-request cache, not comprehensive offline inventory |

## Candidate cut
Rejected safety scoring (reports cannot establish safety); rejected inferred closures (false flags do not prove open); rejected automated AI text advice (source AI description is not authoritative); rejected full inventory refresh (unbounded requests); rejected GPX/photos bulk export (membership/scope). Five surviving features score >=5 for this user's workflow, with explicit caveats.

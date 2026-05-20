# Absorb Manifest — nasa-images-pp-cli

## Absorbed (match or beat everything that exists)

| #  | Feature                                  | Best Source                            | Our Implementation                                | Added Value                                            |
|----|------------------------------------------|----------------------------------------|---------------------------------------------------|--------------------------------------------------------|
| 1  | Search by free text (`q`)                | every wrapper                          | Generator's typed `search list` command           | --json + --select + --csv free                         |
| 2  | Filter by media_type (image/video/audio) | every wrapper                          | Generator-emitted query param                     | typed enum hint in MCP schema                          |
| 3  | Filter by year range (year_start/year_end) | every wrapper                        | Generator-emitted                                 | —                                                      |
| 4  | Filter by NASA center                    | every wrapper                          | Generator-emitted (HQ/JSC/KSC/GSFC/JPL/MSFC/ARC/LRC/AFRC/GRC/SSC) | enumerated in description                   |
| 5  | Filter by photographer                   | AJFunk, peteretelej                    | Generator-emitted                                 | —                                                      |
| 6  | Filter by secondary_creator              | AJFunk, peteretelej                    | Generator-emitted                                 | —                                                      |
| 7  | Filter by keywords                       | every wrapper                          | Generator-emitted                                 | —                                                      |
| 8  | Filter by location                       | AJFunk, peteretelej                    | Generator-emitted                                 | —                                                      |
| 9  | Filter by description / description_508  | (none)                                 | Generator-emitted                                 | first wrapper to expose 508                            |
| 10 | Filter by title                          | (none)                                 | Generator-emitted                                 | —                                                      |
| 11 | Filter by nasa_id (exact match)          | every wrapper                          | Generator-emitted                                 | —                                                      |
| 12 | Pagination via page + page_size          | every wrapper                          | Generator-emitted; `--all` to follow next-links   | —                                                      |
| 13 | Get asset rendition manifest             | AJFunk, peteretelej                    | Generator-emitted typed `assets get`              | typed result; classified by variant kind               |
| 14 | Get metadata location                    | AJFunk, peteretelej                    | Generator-emitted typed `metadata get`            | —                                                      |
| 15 | Get captions location                    | AJFunk, peteretelej                    | Generator-emitted typed `captions get`            | —                                                      |
| 16 | Get album contents (/album/{name})       | nasa-images-cli only                   | Generator-emitted typed `albums get`              | first Go CLI with this endpoint                        |
| 17 | JSON output everywhere                   | every wrapper                          | --json on every command                           | --select dotted paths for free                         |
| 18 | MCP server                               | peteretelej, jezweb, ProgramComputer   | Generator's cobratree-walked MCP                  | typed schemas, all 5 endpoints; assets-best + captions-fetch are net-new |
| 19 | Rate-limit handling                      | peteretelej (header-aware)             | `cliutil.AdaptiveLimiter`                          | shared with other Printing Press CLIs                  |
| 20 | URL scheme normalization (http→https)    | (none)                                 | Generator's client upgrades upstream `http://` URLs | every wrapper today returns `http://` from responses |
| 21 | --dry-run on every command               | (none)                                 | Generator's --dry-run                             | —                                                      |
| 22 | doctor / health check                    | (none)                                 | Generator's `doctor`                              | —                                                      |
| 23 | Offline SQL access to the cache          | (none)                                 | Generator's `sql` over the local store            | —                                                      |
| 24 | --select dotted paths                    | (none)                                 | Generator-emitted                                 | —                                                      |
| 25 | --csv / --compact / --quiet              | (none)                                 | Generator-emitted                                 | —                                                      |

## Transcendence (only possible with our approach)

| #  | Feature                                  | Command                                            | Score | Buildability | Why Only We Can Do This |
|----|------------------------------------------|----------------------------------------------------|-------|--------------|-------------------------|
| 1  | Resumable album bulk download            | `download album <name> --variant orig --resume`    | 9/10  | hand-code    | Calls `/album/{name}` paginated, then `/asset/{id}` per item, then byte-ranged GETs against `images-assets.nasa.gov`; writes a `downloads` SQLite progress row per file so re-runs skip completed and byte-range-resume in-flight files. No wrapper has byte-range resume; only one wrapper covers the album endpoint at all. |
| 2  | Caption text fetch (not URL)             | `captions fetch <nasa_id> --format srt\|vtt\|text`  | 8/10  | hand-code    | Every existing wrapper stops at `/captions/{id}` and returns the location URL. We follow the indirection, GET the .srt/.vtt body, and offer a `--format text` mode that strips cue numbers/timecodes — the actual transcript text agents and editors need. |
| 3  | Metadata sidecar follow + noise filter   | `metadata fetch <nasa_id>`                         | 7/10  | hand-code    | Every wrapper returns the location URL from `/metadata/{id}`. We follow it, fetch the sidecar, flatten `AVAIL:*` + EXIF fields, and drop the leak fields (`SourceFile`, `File:Directory`, `AVAIL:Owner` — the curator's login). |
| 4  | Deterministic "best variant" picker      | `assets best <nasa_id> --max-bytes 5MB --prefer orig,large,medium` | 9/10 | hand-code | Parses the asset manifest, classifies each href as orig/large/medium/small/thumb by filename suffix, applies caller preference order with optional byte-ceiling HEAD probe, prints exactly one URL. Agents stop burning tokens parsing prose to pick a JPG size. |
| 5  | Local chronological FTS search           | `search local --sort date-desc --q "perseverance"` | 8/10  | hand-code    | FTS5 over title/description/description_508/keywords/album columns in the local mirror, then `ORDER BY date_created DESC`. NASA's upstream search is keyword-only with no chronological sort (open issue nasa/api-docs#187, no maintainer response). |
| 6  | Center profile aggregation               | `center profile JPL`                               | 6/10  | hand-code    | Local SQL over `assets` + `keywords` + photographer: counts by media_type, year-bucket histogram, top-10 keywords, top-5 photographers for a NASA center. Brief enumerates 11 centers each with a distinct content profile; no wrapper exposes this view. |
| 7  | Unused-in-album anti-join                | `unused-in Apollo-at-50`                           | 7/10  | hand-code    | LEFT JOIN of `album_members(nasa_id)` against the local `downloads(nasa_id)` table from the `download` command; prints nasa_ids in the album not yet downloaded locally. Falls out of the downloads ledger that resumable download already requires. |
| 8  | Topic timeline histogram                 | `timeline --q "perseverance" --bucket month`       | 6/10  | hand-code    | Local `GROUP BY strftime('%Y-%m', date_created)` over FTS-matched rows; prints a month-bucket count to answer "when did this topic get coverage?" — unanswerable today because the upstream API has no chronological surface. |
| 9  | Citation string generator                | `citation <nasa_id> --style apa\|mla\|chicago`     | 5/10  | hand-code    | Pure string template over cached metadata row (or fetches sidecar on miss): photographer + date_created + center + nasa_id + URL. Journalists and educators hand-assemble this today from metadata.json. |

**Hand-code count:** 9 of 9 transcendence rows. None are spec-emitted because all of them either (a) write new SQLite tables (`downloads`, `album_members`), (b) post-process API responses (caption text, metadata flattening, variant classification), or (c) aggregate over the local cache (center profile, timeline, unused-in, citation, local search).

**Stub list:** None. Every transcendence row is committed as full shipping scope.

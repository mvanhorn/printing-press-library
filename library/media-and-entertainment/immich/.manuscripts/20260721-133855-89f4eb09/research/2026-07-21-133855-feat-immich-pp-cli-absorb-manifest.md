# Immich Ecosystem Absorb Manifest

## Approval

Sol absorb approval: 2026-07-21; 16 shipping rows; exact 8-feature set below. The approved rows, including the explicit source-adapter requirements in rows 2/3/4/5/8, are binding shipping requirements. Exact implementation and publication remain subject to independent Sol gatekeeping.

## Absorbed capabilities

| # | Incumbent | Capability | Our implementation | Evidence | Scope |
|---|---|---|---|---|---|
| 1 | Official CLI | API-key login/logout and server info | `immich-pp-cli auth`, `server ping`, `server version` | Official CLI docs | shipping |
| 2 | Official CLI | Upload folders/files recursively with ignores, album routing, dry-run, bounded concurrency, JSON outcome and duplicate-aware upload | `immich-pp-cli import folder <path> --recursive --ignore <glob> --album-name <name> --album-by-folder --dry-run --concurrency N --json`; it walks only permitted files, performs a real `POST /assets/bulk-upload-check` before each candidate, and only performs the real multipart `POST /assets` upload when needed. | Official CLI docs | shipping |
| 3 | Official CLI | Watch-style ongoing upload workflow | `immich-pp-cli import watch <path> --for 30m --poll 10s --stable-for 5s --recursive --ignore <glob> --album-name <name> --concurrency N --json`; it polls a bounded duration, waits for file size/mtime stabilization, honors context cancellation, and routes each eligible file through the same real duplicate check/upload implementation as `import folder`. | Official CLI docs | shipping |
| 4 | Immich-Go | Folder, archive/ZIP, Google Photos Takeout, iCloud, and server-to-server import pathways | `import folder`, `import archive <zip>`, `import takeout <dir-or-zip>`, `import icloud <export-dir>`, and `import immich --source-url URL --source-api-key-env VAR`; each is a source adapter, not an endpoint alias. Archive adapters enumerate real file entries; Takeout reads its JSON sidecars for title/date/album metadata; iCloud reads export CSV metadata when present; server migration downloads source originals and uploads through the destination duplicate-check flow. | immich-go README | shipping |
| 5 | Immich-Go | Preserve albums, tags, metadata, locations and source organization | `import takeout` maps source album memberships/descriptions/dates/locations when a Takeout sidecar supplies them; `import icloud` maps CSV date/album metadata when supplied; `import immich` gets source assets/albums/tags and writes them through destination asset metadata, albums, and tags APIs. Every unsupported source field is reported in JSON as `unmapped_metadata`, never silently claimed preserved. | immich-go README | shipping |
| 6 | Immich-Go | Duplicate detection and cautious file cleanup | generated duplicate group/list/resolve/dismiss plus novel `duplicates plan`/`duplicates apply` | immich-go README + official duplicate docs | shipping |
| 7 | Immich-Go | Burst and RAW+JPEG grouping | generated stacks create/search/get/edit and asset-stack endpoints plus novel `stacks review` | immich-go README | shipping |
| 8 | Immich-Go | Large-library safe import controls and exclusion patterns | `import folder`/`import watch`/archive adapters implement repeated `--ignore` glob filters, default junk-file exclusions, `--include-hidden`, `--max-files`, `--concurrency`, cancellation, and structured skipped/error/duplicate counts. Files are selected before any API request, so filesystem filtering is real rather than delegated to raw endpoint pagination. | immich-go README | shipping |
| 9 | ImmichMCP | Asset browse/get/update/delete, EXIF/OCR/download/thumbnail and stats | generated `/assets` and `/assets/{id}` command mirrors | ImmichMCP README tool list | shipping |
| 10 | ImmichMCP | Smart, metadata, OCR, explore, place, city and random search | generated `/search/*` command mirrors | ImmichMCP README tool list | shipping |
| 11 | ImmichMCP | Album create/manage/share/statistics | generated `/albums*` command mirrors plus novel `album event` | ImmichMCP README tool list | shipping |
| 12 | ImmichMCP | People list/get/update/merge/assets | generated `/people*` command mirrors plus novel `people july` | ImmichMCP README tool list | shipping |
| 13 | ImmichMCP | Tag CRUD and asset tagging | generated `/tags*` command mirrors | ImmichMCP README tool list | shipping |
| 14 | ImmichMCP | Shared links and social activities | generated `/shared-links*` and `/activities*` command mirrors | ImmichMCP README tool list | shipping |
| 15 | ImmichMCP | Connectivity/capability health | generated `/server/*`, `/jobs`, `/queues/*` command mirrors plus novel `library health` | ImmichMCP README tool list | shipping |
| 16 | ImmichMCP gap | Native memories, stacks, partners and duplicate utilities | generated native endpoint mirrors plus novel memory/stack/partner/duplicate rituals | ImmichMCP tool list vs official spec | shipping |

## Transcendence features

Exactly eight features are approved; all are hand-code and use real Immich v3 endpoints.

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Shared event album | `album event` | 9/10 | hand-code | Calls Smart Search, creates/updates an album, adds the matched asset IDs, and optionally shares it with users through the Albums API. | User priority; official album/search API; neither incumbent offers an event-to-shared-album safety workflow. | Use this to create a reviewable event album from explicit filters. Do NOT use it to upload folders; use the asset upload command. |
| 2 | Duplicate resolution plan | `duplicates plan` | 9/10 | hand-code | Calls `GET /duplicates`, scores only native group metadata, and emits a non-mutating keeper/trash proposal with explicit group IDs. | Official duplicate utility preselection rules; user priority; Immich-Go detects duplicates but does not expose this agent-safe plan. | Use this to preview native duplicate cleanup. Do NOT use it to apply a resolution; use `duplicates apply`. |
| 3 | Duplicate resolution apply | `duplicates apply` | 8/10 | hand-code | Re-fetches requested duplicate groups, requires `--apply`, and calls `POST /duplicates/resolve` only after reporting each selected keeper and trash set. | Official duplicate utility; user priority. | Use this only after reviewing `duplicates plan`; it changes the library. |
| 4 | Recurring-month people finder | `people july` | 8/10 | hand-code | Resolves two people through `/search/person` then uses `/search/metadata` with explicit person/date filters across a bounded range of years. | User priority; official people/search API. | Use this for a recurring month across years. Do NOT use it for a broad semantic query; use Smart Search. |
| 5 | Memory review queue | `memories review` | 7/10 | hand-code | Calls `/memories` and `/memories/statistics`, returning bounded dated memories and asset counts for a chosen window. | Official memories API; ImmichMCP omission. | none |
| 6 | Favorites and archive review | `library review` | 7/10 | hand-code | Calls `/search/metadata` with favorite/archive filters and returns a bounded actionable count/list without mutating assets. | User priority; official asset search API. | none |
| 7 | Stack hygiene review | `stacks review` | 7/10 | hand-code | Calls `/stacks` and each selected `/stacks/{id}`, reports empty/singleton/large stack conditions without changing assets. | Immich-Go stacking; official stacks API; MCP gap. | none |
| 8 | Partner and job health | `library health` | 7/10 | hand-code | Calls `/partners`, `/jobs`, and queue job APIs to return explicit sharing and worker-pressure facts. | Partner-sharing docs; official jobs/partners API; MCP gap. | none |

## Kill check

All eight features use only the configured Immich API, its generated client, and explicit command input. No candidate uses an LLM, external service, hardcoded response payload, background daemon, or unverified destructive action.

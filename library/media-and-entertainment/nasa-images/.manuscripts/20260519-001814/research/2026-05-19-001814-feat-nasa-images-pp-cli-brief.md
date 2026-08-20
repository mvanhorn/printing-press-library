# NASA Image and Video Library CLI Brief

## API Identity
- **Domain:** Public-domain media catalog — every image, video, and audio asset NASA centers (HQ, JSC, KSC, GSFC, JPL, MSFC, ARC, LRC, AFRC, GRC, SSC) have curated into the AVAIL index. ~140k assets covering Apollo, Shuttle, Mars Rovers, Webb/Hubble, Earth science, history. Free for any use.
- **Users:** Educators, journalists, designers, hobbyists who need NASA media for a deck/post/video; agentic workflows ("find the Apollo 11 Buzz Aldrin bootprint at large size"); developers building tools or visualizations on top of NASA's photo/video archive.
- **Data profile:** Items have `nasa_id`, `title`, `description`, `description_508`, `media_type` (image|video|audio), `date_created`, `center`, `keywords[]`, `album[]`, optional `photographer`/`secondary_creator`/`location`. Each asset expands to 5–30+ rendition files (image sizes, mp4 variants, audio bitrates, captions, metadata.json sidecar with `AVAIL:*` and EXIF fields).

## Reachability Risk
- **None.** `probe-reachability` returned `mode: standard_http`, confidence 0.95, 200 from stdlib HTTP. No CDN challenge, no auth, no rate limiting headers observed. CloudFront sits in front but is friendly (`Cache-Control: public, max-age=300, s-maxage=600`). No GitHub issues report 403s/429s/blockage on the wrappers (only one issue reports v1.1 schema drift, which is cosmetic).

## Top Workflows
1. **Search and download.** "Find me Mars rover images from 2020–2022, give me the original res JPGs." Today: open https://images.nasa.gov, search, click-through, right-click-save per file. With CLI: `search --q "Mars rover" --year-start 2020 --year-end 2022 --media-type image --json | jq` then `download --nasa-id …` or `download --query …`.
2. **Bulk album archive.** "Archive the full Apollo-at-50 album to local disk." Today: no good option; the NASA UI has no bulk-download button. The Python `nasa-images-cli` does it but restarts every failed file from byte zero. We can beat that with byte-range resume.
3. **Caption / transcript extraction.** "Pull the transcript from the Webb deployment livestream video." Today: every wrapper returns the captions *URL* but not the file content. With CLI: `captions get <nasa_id>` fetches and prints the `.srt`/`.vtt` text directly.
4. **Album / collection browse.** "What's in the Mars-Perseverance album?" Today: only NASA's web UI exposes albums; no Go wrapper supports `/album/{name}`. With CLI: `album get Mars-Perseverance --page 1 --json`.
5. **Offline FTS search over synced cache.** "I keep searching for Apollo 11 things — let me sync the metadata locally and grep without hitting the API." Today: nobody does this. With CLI: `sync --q "apollo 11"` then `search "bootprint" --local` (FTS5 over title/description/keywords).
6. **Agent media discovery.** "Find me a high-resolution NASA image of Saturn's rings I can use." A Claude agent needs structured JSON, a typed MCP surface, and small token footprint. The generator gives us this for free; the contribution is making `nasa_id → variants` deterministic so an agent can compose `search → asset → download` without parsing prose.

## Table Stakes
- **Full endpoint coverage:** `/search`, `/asset/{nasa_id}`, `/metadata/{nasa_id}`, `/captions/{nasa_id}`, `/album/{album_name}`. No competing tool covers all five.
- **Full search param surface:** `q`, `center`, `description`, `description_508`, `keywords`, `location`, `media_type`, `nasa_id`, `photographer`, `secondary_creator`, `title`, `year_start`, `year_end`, `page`, `page_size`.
- **Pagination follow:** auto-paginate when `--all` is set, follow `links[rel=next]`.
- **JSON output:** every command (`--json`).
- **Asset rendition manifest:** parse the response into a clean list (orig, large, medium, small, thumb for images; ~mp4 variants and posters for video; ~mp3/m4a for audio).
- **Metadata fetch:** follow the indirection `metadata` → `metadata.json` and surface the `AVAIL:*` + ExifTool fields as a flat object.
- **Captions fetch:** follow `captions` → `.srt/.vtt` and surface the text content.
- **MCP surface:** every user-facing command becomes an MCP tool (generator handles).

## Data Layer
- **Primary entities:**
  - `assets` — one row per `nasa_id` with the search-result `data` block flattened (title, description, description_508, media_type, date_created, center, photographer, location, secondary_creator).
  - `keywords` — many-to-many `nasa_id ↔ keyword`.
  - `albums` — many-to-many `nasa_id ↔ album_name`.
  - `renditions` — one row per file URL discovered from `/asset/{nasa_id}` (nasa_id, variant_kind, format, href).
  - `metadata` — sidecar JSON keyed by nasa_id (`AVAIL:*` + ExifTool fields), opportunistically populated when a user runs `metadata get`.
- **Sync cursor:** the API has no `since` or chronological sort, so `sync` is query-driven: caller provides `--q`, `--year-start`/`--year-end`, `--center`, etc., and we walk pages with `page_size=100` until `links[next]` is absent. Track `last_synced_at` and `last_query` per cursor row so re-runs can either widen or skip cached pages.
- **FTS/search:** FTS5 over `title`, `description`, `description_508`, `keywords` (joined), and `album` (joined). Highly valuable because the upstream search is keyword-only with no chronological/popular sort.

## Codebase Intelligence
- DeepWiki skipped: NASA's AVAIL backend is closed-source. No GitHub repo to wiki-mine. Knowledge comes from the v1.22.0 PDF docs + live API probes.
- Auth: none. The host `images-api.nasa.gov` is separate from `api.nasa.gov` — passing `api_key=…` is silently ignored.
- Data model: confirmed Collection+JSON envelope; pagination via `collection.links[rel=prev|next]` with `metadata.total_hits`; asset/metadata/captions use a small indirection where the response carries only a `location` URL pointing to a static `images-assets.nasa.gov` file.
- Quirks worth a comment in code (not stored in memory): `http://` echoed in responses even when called over HTTPS (cosmetic, treat as https://); `page_size` has no enforced max but should be ≤100 for stability; album names are exactly case-sensitive; `/album/` ignores `page_size`; metadata.json leaks `SourceFile` / `AVAIL:Owner` internal paths (treat as noise); audio assets typically have no thumbnail link in search results.

## Source Priority
- Single source. No combo CLI; no inversion risk.

## Product Thesis
- **Name (slug):** `nasa-images` → binary `nasa-images-pp-cli`. Confirmed by user (`nasa-images-pp-cli`, not `images-nasa-pp-cli`).
- **Display name:** NASA Image and Video Library.
- **Headline:** Every endpoint the NASA Image and Video Library exposes, in Go, with a local SQLite mirror, FTS search, resumable bulk download, caption text extraction, and an agent-native MCP surface. Nothing else in the ecosystem covers all five endpoints, and no Go tool covers any of them well.
- **Why it should exist:** The competitive bar is shockingly low. The strongest direct competitor on GitHub has 1 star, no offline cache, and covers 2 endpoints. The best-known Go option (peteretelej/nasa, 17⭐) ships NIVL as one of six API surfaces and skips the `/album` endpoint entirely. No Go CLI is dedicated to images-api.nasa.gov. Agents currently have to compose 3+ API calls and parse prose to download a single NASA image at a given resolution — we make that one command.

## Build Priorities
1. **Foundation (P0):** Internal YAML spec covering all 5 endpoints, with `auth.type: none`, the full search parameter set, and clear param descriptions. Generator emits Go client + Cobra command tree + MCP server.
2. **Absorbed (P1):** Match every feature every existing wrapper offers — full search filters, asset/metadata/captions/album retrieval, JSON output, pagination, MCP tool exposure.
3. **Transcend (P2):** SQLite mirror with FTS5; resumable bulk download with byte-range resume and variant selection; caption text fetch (not just URL); chronological/popular sort over the local cache; "best variant" selector; metadata indirection auto-follow; captions indirection auto-follow.
4. **Polish (P3):** Realistic CLI examples (Apollo, Mars, Webb), agent-native flag descriptions, README cookbook entries for each top workflow.

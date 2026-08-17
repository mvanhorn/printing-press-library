# printgoat CLI Brief (Printables + Thingiverse + Cults3D combo)

## API Identity
- Domain: cross-site discovery, metadata, and file download for 3D-printable model files (STL/3MF/G-code) across three community platforms: Printables.com (Prusa), Thingiverse (UltiMaker/MakerBot), Cults3D.
- Users: hobbyist makers, 3D-print farm operators, people building physical objects who don't want to manually check 3 sites per search.
- Data profile: read-heavy (search, browse, metadata, download). Very little useful write surface for a discovery tool (likes/follows/collections exist per-site but are secondary). Local data layer is mostly a download-history/dedup cache, not a full mirror.

## Reachability Risk

### Printables — Low-Medium, unofficial but empirically solid
- **No official API.** GraphQL endpoint `https://api.printables.com/graphql/` is undocumented by Prusa (Prusa Forum thread confirms "considering it, not shipped").
- **Confirmed live and working anonymously** (research agent completed a full search → file-list → signed-download-URL → actual file fetch chain with zero auth, zero cookies, on the public-domain Benchy model).
- Cloudflare sits in front of the API (`server: cloudflare` observed) but did not challenge a single low-volume request with a browser UA. Community tool `GhostTypes/printables-cli-api` uses `cloudscraper` for the HTML site specifically, implying Cloudflare *does* challenge HTML scraping at least sometimes — GraphQL POSTs appear to fare better.
- **Schema drift is real**: at least 2 incompatible signatures for the `getDownloadLink` mutation exist in circulation across community repos from different dates; field additions (price/premium/club fields) were noted mid-2024 by one wrapper author's own code comments.
- Community reverse-engineering is extensive (~10 small repos) but fragmented and some are unverified/wrong (one library's Printables provider uses a query shape that doesn't match the confirmed-working schema).
- No official ToS sanction for scraping; every community repo self-describes as "unofficial, not endorsed."
- Probe-safe endpoint used: `POST https://api.printables.com/graphql/` with the `searchPrints2` query (read-only, anonymous, empirically confirmed 200 with real data during research).

### Thingiverse — Low, official but fragile
- **Official OAuth2 API**, live OpenAPI 3.0 spec directly fetchable at `https://www.thingiverse.com/swagger/docs/openapi.yaml` (confirmed resolvable, all `$ref`s load).
- API License Agreement last revised **Feb 19, 2026** — actively maintained, not abandoned.
- Reachability risk is about *friction and rate limits*, not shutdown: self-serve app registration UI has a years-long-documented bug (button invisible until hover); rate limit is a strict **300 requests / 5-minute window**, and one 2025 GitHub issue reports Thingiverse **invalidating the token entirely** (not just 429) when the limit is hit hard, causing a maintainer to disable the integration as "too expensive."
- All-or-nothing auth — **no anonymous/read-only mode exists**; every endpoint requires a Bearer token.
- Known open bug: NSFW-tagged Things don't appear via API even for permitted accounts (manyfold3d/manyfold#4686, open).
- Its own hosted Swagger UI is broken ("No operations defined in spec!") even though the underlying YAML is fine — signals under-maintained docs surface, not under-maintained API.

### Cults3D — Low for search/metadata, N/A (by design) for download
- **Official, actively-maintained GraphQL API** at `https://cults3d.com/graphql` (single endpoint, single POST method), with a changelog entry as recent as **July 2026**. Self-service key generation at `cults3d.com/en/api/keys`, no approval/waitlist.
- Auth: **HTTP Basic Auth**, username = Cults3D handle, password = generated API key.
- **The API explicitly and permanently refuses to serve other users' 3D files** — direct quote from Cults3D's own docs: *"the API will not give you access to the 3D files for other users... for legal reasons."* This applies to free AND paid models alike. This is why every community "downloader" (e.g. `CultsDL`, 13★) works around it via a logged-in session cookie instead of the API — and that scraping path sits in direct tension with Cults3D's own ToS ("It is... prohibited to practice scraping, download or copy digital content... without... authorization").
- Net effect: Cults3D is fully viable for search/browse/metadata/categories/collections via the sanctioned API, but **download must be scoped out or explicitly flagged as unsupported/best-effort** for Cults3D specifically — this is a permanent product constraint, not a bug to fix later.

## Top Workflows
1. **Unified search** — one query hits Printables + Thingiverse + Cults3D in parallel, returns a single ranked, deduplicated result list labeled by source (this is the entire reason the combo exists).
2. **Model detail + full file listing** for any single result, regardless of source, in one normalized shape.
3. **Reliable bulk download** of a model's files (or an entire pack/zip where the site supports it) with resume/retry — the #1 validated pain point across every research angle (Reddit threads, the dominant "fix broken download-all" theme across every Thingiverse browser extension, Cults3D having zero download-assist tooling at all).
4. **Cross-site duplicate detection** — many designers publish the same model to 2-3 of these sites simultaneously; surfacing "this is the same design, available on Printables (free) and Cults3D ($4)" is a novel, high-value feature nobody in the ecosystem does today.
5. **Local download history / "have I got this already"** — dedup against previously-downloaded models before re-downloading, and flag re-encountered search results.

## Table Stakes
- Keyword search with sort (popularity/rating/newest/relevance) and category/tag filters (all 3 sites support some form of this; Thingiverse and Cults3D users specifically complain about weak native search/pagination).
- Model detail: description, images, license, tags, rating, download/like counts, author.
- File listing per model with format (STL/3MF/G-code/other) and size.
- Bulk-download-all as a single operation (the recurring "fix the broken zip button" theme across Thingiverse extensions).
- Browse by user profile, collection, and category.
- Config persistence for API keys and default download directory (thingfinder does this).
- Retry/backoff handling for rate limits (thingfinder does this for 429s).
- Interactive result picker (thingfinder's `-i` mode).

## Data Layer
- Primary entities: `models` (unified: id, source, source_id, name, slug, url, description, images, license, category, tags, rating, likes_count, downloads_count, files_count, is_paid, price, author_handle, published_at), `files` (model_id, source, name, format, size_bytes, download_url_or_ttl_link), `authors` (per-source handle/profile), `downloads` (local history: model_id, source, downloaded_at, local_path, file hashes — powers dedup).
- Sync cursor: not a full-mirror sync tool (search is always live against 3 upstreams) — the local store is a **download-history + search-result cache**, not a synced replica. `sync` in the generated sense applies to refreshing cached metadata for models the user has already touched, not bulk-mirroring any site's catalog.
- FTS/search: SQLite FTS5 over the local download-history/cache table for instant "have I searched/downloaded this before" queries offline; live search always hits the 3 upstream APIs (no upstream provides a bulk export to mirror).

## Codebase Intelligence
- Source: GitHub research on `nukleas/thingfinder` (closest direct competitor — Node.js CLI+library+MCP server covering Thingiverse+Printables+Thangs, dormant since Apr 2026) and `pikalover6/3dfetch` (Node.js library, 16-source provider abstraction, no CLI). Deeper source-level analysis of these two (auth patterns, actual query shapes used) is scheduled for Step 1.5a.5/1.5a.6.
- Auth patterns observed in the wild: Printables needs no headers for read paths; Thingiverse needs `Authorization: Bearer <token>`; Cults3D needs `Authorization: Basic base64(handle:api_key)`.
- Architecture insight: no competitor pre-builds an index — everyone hits live upstream APIs at query time. Our local SQLite layer differentiates on **download history/dedup**, not on being a faster mirror.

## Source Priority
- Confirmed via Multi-Source Priority Gate (`source-priority.json`): **Printables > Thingiverse > Cults3D**.
- Primary: **Printables** — no official spec (GraphQL reverse-engineered from live traffic + community schemas), auth: free/anonymous for search+download.
- Secondary: **Thingiverse** — official OpenAPI 3.0 spec available directly from the vendor, auth: free but requires OAuth2 app registration (self-serve).
- Tertiary: **Cults3D** — official GraphQL API (self-serve key), auth: free registration, but **download is out of scope by upstream design** — this source is search/metadata/browse only.
- **Economics:** All three sources are free to use programmatically (no paid API tiers). No tier-routing needed.
- **Inversion risk:** Printables has *no* official spec while Thingiverse ships a clean OpenAPI YAML. There is a real risk that spec-completeness bias could invert the priority (Thingiverse ends up with more auto-generated typed endpoint commands than Printables). Must actively counter this in Phase 2 by hand-authoring/enriching a first-class internal YAML spec for Printables from the confirmed-working GraphQL queries so its command surface is not thinner than Thingiverse's, and by giving Printables the headline search command and top-of-README billing regardless of which source has more generated endpoints.

## Product Thesis
- Name: **printgoat** (working name; may be refined during Phase 1.5e narrative authoring if research surfaces a better brand fit).
- Why it should exist: no existing tool covers all three sites reliably from a single static binary. The closest analog (`thingfinder`, Node.js) is dormant, misses Cults3D entirely, and has zero GitHub traction. Bulk-download reliability — the single most repeated complaint across Reddit threads and the entire Thingiverse browser-extension ecosystem ("fix the broken download-all button") — remains unsolved by anyone. No Go tool exists for Printables or Cults3D at all (the one Go Thingiverse wrapper is unmaintained since 2019). Cross-site duplicate detection (same model published to multiple platforms, sometimes free on one and paid on another) is a feature nobody has built.

## Build Priorities
1. Unified parallel search across all 3 sources with relevance ranking, dedup, and source labeling.
2. Normalized model detail + file listing per source, with format-aware download.
3. Reliable bulk download (resume/retry, zip-pack support where available) + local SQLite download-history for dedup ("already downloaded this").
4. Cross-site duplicate/same-model detection (transcendence feature — the headline differentiator).
5. Category/user/collection browsing per source.

## Reachability Gate (Phase 1.9)
- **Printables**: PASS. Live-tested during research: `POST https://api.printables.com/graphql/` with `searchPrints2` query returned HTTP 200 with real data, anonymously. Probe-safe endpoint: the same anonymous search query (read-only, no auth, no state change).
- **Thingiverse**: PASS. Live-tested with the user-provided app token: `GET https://api.thingiverse.com/users/me` with `Authorization: Bearer <token>` returned HTTP 200 with a real account payload. Confirms both reachability and that the provided token is valid.
- **Cults3D**: PASS. `POST https://cults3d.com/graphql` without Basic Auth returned HTTP 401 "HTTP Basic: Access denied" — expected behavior per the official docs ("all calls use HTTP Basic Auth"), consistent with the reachability matrix's "401 (no key provided) -> PASS" row. Full authenticated verification deferred to Phase 5 pending the user's Cults3D handle (API key alone is insufficient for Basic Auth; username also required).

# Placeit CLI Brief

## API Identity
- Domain: Browser-based "smart template" generator (Envato-owned). Mockups, design templates, logo maker, video maker, gaming/Twitch assets.
- Users: Etsy/POD apparel sellers (dominant), Twitch/YouTube streamers, small-business solopreneurs, social-media managers/freelancers.
- Data profile: ~164,580 published catalog "stages" (templates) indexed in Algolia. Four categories: Mockups (67k), Design Templates (63k), Logos (26k), Videos (7.6k). Rich tag taxonomy (device/model/gender/age/ethnicity/color/bundle tags) + a 152-entry Industries taxonomy.

## Reachability Risk
- **Primary surface (Algolia search): None.** Verified live: `POST https://KSLVR81FGG-dsn.algolia.net/1/indexes/Stage_production/query` with the public search-only key returns HTTP 200 + 43,064 hits for "t-shirt". Separate `*.algolia.net` host, no Cloudflare, public client key (deliberately embedded in placeit.net's JS bundle — same class as a Stripe *publishable* key).
- **HTML page reads (stage detail pages): Low/Medium.** placeit.net is Cloudflare-fronted (`__cf_bm` set every response). A real browser-UA `curl` returns 200; WebFetch IP gets 403. UA/IP-reputation based, not a hard JS challenge.
- **Authenticated download/render: High effort.** Envato SSO session (cookie), Cloudflare-protected, and `template_type: blender` implies server-side 3D render pipeline. Not obviously replayable from raw HTTP — pending Phase 1.7 capture.
- Probe-safe endpoint used: `POST /1/indexes/Stage_production/query` (Algolia, read-only search).

## Top Workflows
1. **POD apparel mockup:** search `t-shirt` mockups → filter by `device_tags:T-Shirt` + Front/Flat-Lay/Crew-Neck → open stage → upload artwork → render → download PNG.
2. **Logo maker:** search industry/style → pick logo template → edit name/tagline/icon/palette → download static + animated.
3. **Twitch streamer kit:** browse gaming templates → overlay/panel/emote/webcam-frame as a matched set → download (transparent + bg versions).
4. **Social batch:** Instagram/Facebook story+post templates → swap text/brand colors → export sized variants.
5. **Video promo:** intro/promo template → edit text/logo/colors → render MP4.

## Table Stakes
- Keyword search; filter by category + type/device + attribute tags (gender/age/ethnicity/color).
- Sort by newest / best-selling / free.
- Template detail + deep link + thumbnail URLs.
- Customize text/color/upload-art; render/preview; download (PNG/JPG/MP4/transparent).
- Brand kit (saved colors/fonts); favorites/saved designs; "my downloads" library.
- Industry taxonomy browse.

## Data Layer
- Primary entities: `stage` (template) — id, name, category_name, template_type, stage_link, thumbnails (+w/h), tag arrays. `industry` (taxonomy, 152). Optional: `category`, `facet` distributions.
- Sync cursor: Algolia is fully queryable; sync = paginated crawl of `Stage_production` (and replicas) into SQLite. No incremental cursor exposed, but `..._replica_newest` gives recency ordering for delta sync.
- FTS/search: local FTS5 over name + tags for offline search; mirrors Algolia hit shape.

## Source Priority
- Single source (placeit.net via Algolia + stage HTML). Not a combo CLI.

## Auth model
- Browse/search/customize = **free, ungated**. Algolia needs no auth.
- Download/export = **subscription-gated** (Placeit Unlimited, ~$14.95/mo or ~$89.95/yr). Login = **Envato SSO** (`account.envato.com/sign_in?to=placeit`). No OAuth/API-key path for end users — cookie/session only.
- CLI implication: `search`/`browse`/`template`/`categories`/`industries` need no auth. `download`/`account`/`my-designs` need a logged-in Envato session (Phase 1.7 capture target).

## User Vision
- User is logged into Placeit/Envato in Chrome and approved authenticated browser-sniff. Wants the CLI to leverage their session where it adds value (download/render/account discovery).

## Codebase Intelligence
- Stack: Next.js (`__NEXT_DATA__`, buildId), Algolia search (App `KSLVR81FGG`, index `Stage_production`), Cloudinary + AWS CDN images, Plasmic marketing pages.
- Algolia replicas: `Stage_production_replica_newest`, `_replica_best_selling`, `_replica_free`. Taxonomy index `Industries_production`.
- Facets: category_name, device_tags, stage_tags, model_tags, gender_tags, age_tags, ethnicity_tags, color_tags, bundle_tags.
- No existing Placeit API wrapper/SDK/scraper on GitHub/npm/PyPI — this CLI is net-new.

## Product Thesis
- Name: **placeit-pp-cli** ("Placeit catalog, in your terminal")
- Why it should exist: Placeit has 164k templates behind a heavy web UI with no API, no bulk search, no offline catalog, no scriptable workflow. POD sellers and streamers who batch-produce assets have no way to query the catalog programmatically. This CLI turns the entire Algolia-indexed catalog into a fast, offline-cacheable, agent-native search/browse tool with deep links and thumbnail URLs — something no existing tool (and not even Placeit's own UI) offers.

## Build Priorities
1. **Algolia client + data layer** — `search`, `browse`, paginated `sync` of `Stage_production` into SQLite, FTS5.
2. **Catalog commands** — `search <query>` (filters: --category, --type, --tag, --sort newest|best-selling|free, --limit, --page), `template <id|slug>` (detail + deep link + thumbnails), `categories`, `facets`, `industries`.
3. **Transcendence** (local-store-powered, what neither competitors nor Placeit's UI offer) — cross-facet analytics, matched-set builders (Twitch kit), bulk export of deep links/thumbnails for listing pipelines, saved searches/watchlists, gap finders. (Final list from Phase 1.5 subagent + gate.)
4. **Authenticated surface** (pending Phase 1.7) — `account` (subscription status), `my-designs`/`downloads` if replayable; `download`/`open` for stages (likely browser-session-gated; ship honestly).

# American Reindustrialization CLI Brief

## API Identity
- **Domain:** Curated directory of US-based companies driving reindustrialization (manufacturing, robotics, advanced materials, supply chains, defense tech) plus an aggregated jobs board for those companies and a news feed (empty as of capture).
- **Surface:** Undocumented JSON REST API at `https://americanreindustrialization.com/api/*`, fronted by Cloudflare. No OpenAPI/Swagger published. Backend serves a React/Vite SPA that the public reaches at the bare domain.
- **Auth:** Cookie session for user-side endpoints (`/api/profile`, `/api/jobs/my-applications`, company-dashboard, admin). All directory read endpoints (companies, categories, tags, jobs list/sub-aggregates, news) work fully unauthenticated.
- **Scale:** 96 companies, 44 categories, 87 tags, 501 job listings. Small-to-medium curated dataset that comfortably fits a local SQLite store.
- **robots.txt:** Cloudflare Content-Signal markers — `search=yes, ai-train=no`. Read access explicitly allowed for ClaudeBot, GPTBot, PerplexityBot, Applebot. CLI hits JSON API, not HTML, so no scraping concerns.

## Users
1. **VC / investor analyst tracking reindustrialization deal flow.** Tracks which companies are operating in onshoring / manufacturing / robotics / supply-chain. Wants funding-stage signals, competitive density per sub-sector, and "new companies added since last week" snapshots. Today: keeps a browser tab open on the site, eyeballs the company list weekly, copies fields into a spreadsheet.
2. **Job seeker pivoting into industrial tech.** Engineer or operator scanning for hands-on roles at hardware/manufacturing companies. Wants to filter by work_mode (remote/hybrid/onsite), salary range, experience level, geography. Today: clicks through the jobs board paginated 20-at-a-time, opens each role in a new tab.
3. **Industry researcher / journalist writing about American manufacturing revival.** Needs to pull lists for stories — "robotics startups in the Midwest," "companies in advanced materials founded since 2020," "the new manufacturing tech ecosystem in Texas." Today: copy-pastes from the site into a Google Doc, no good way to slice/export.
4. **Founder / strategist scanning the competitive landscape.** Wants the breakdown — how many companies in their sector, how the directory clusters geographically, which categories are growing. Today: scrolls the directory, manually tallies, no analytics view exists on the site.
5. **AI agents helping any of the above.** Agents querying the directory programmatically need fast JSON output, server-side filters, local SQL for arbitrary slices, and a small enough surface to enumerate cheaply.

## Reachability Risk
- **None.** Direct HTTPS to `/api/companies`, `/api/categories`, `/api/tags`, `/api/jobs`, `/api/news` all returned 200 JSON. Cloudflare in front but standard `http.Client` works — no challenge, no clearance cookie required. No 429s during probing. Vendor uses standard JSON (not protected), no anti-bot.
- **No reverse-engineering risk:** the SPA fetches this API in normal browser flows; we're using the same public surface.

## Top Workflows
1. **Filter & search the company directory** — by category, state, sector, employee range, funding stage, founded year, free-text query.
2. **Browse jobs** — by company, work_mode, experience_level, salary band, state, posted-since.
3. **Inspect a company profile** — full description, products/services, contact emails, jobs_count, categories, tags.
4. **Ecosystem analytics** — counts per category, counts per tag, geographic distribution, sector clustering, funding-stage breakdown.
5. **Track changes over time** — new companies, new jobs since last sync; updated profiles; companies that gained/lost jobs.

## Table Stakes (read endpoints to mirror)
- `GET /api/companies` (paginated, default 20, max ?) — list with filters: `state`, `category` (working); other filters: silently ignored unless we discover otherwise (`tag`, `tag_slug`, `category_slug`, `primary_sector` returned all 96).
- `GET /api/companies/{slug}` — full company by slug (unwrapped, not `{data:…}`).
- `GET /api/companies/search?q=<term>` — search returns array (no pagination wrapper).
- `GET /api/categories` — full list (44, returned as bare array).
- `GET /api/categories/{slug}` — single category (unwrapped).
- `GET /api/categories/counts` — map of `category_id → company_count`.
- `GET /api/categories/search?q=<term>`.
- `GET /api/tags` — full list (87).
- `GET /api/tags/{slug}` — single tag.
- `GET /api/tags/counts` — map of `tag_id → company_count`.
- `GET /api/tags/search?q=<term>`.
- `GET /api/jobs` (paginated, 501 total) — filters: `work_mode`, `experience_level` working; `state` silently ignored.
- `GET /api/jobs/{slug}` — single job.
- `GET /api/jobs/companies` — autocomplete list of `{id, name}`.
- `GET /api/jobs/titles?q=<term>` — autocomplete strings array.
- `GET /api/jobs/categories`, `GET /api/jobs/tags` — autocomplete-shape lists.
- `GET /api/news` — list (empty at capture; shape TBD when populated).

## Data Layer
- **Primary entities:** `companies` (rich, ~30 fields), `jobs` (~25 fields), `categories` (hierarchical via parent_id), `tags` (typed, `tag_type`). Plus join tables: `company_categories`, `company_tags` (already embedded in company responses).
- **Sync cursor:** `updated_at` on companies and `posted_at` on jobs. Track per-resource last-sync timestamp.
- **FTS targets:** company name + tagline + short_description + full_description + products_services; job title + description + qualifications.
- **Local SQL value:** the directory is small enough (~96 companies, 501 jobs, 44 categories, 87 tags) that a one-shot full sync fits in <1MB. After that, every analytics question runs offline as a SQL query.

## Codebase Intelligence
Skipped — no GitHub repos exist for this API. The site is a closed-source SPA, the backend has no public source.

## Why Would Someone Install This CLI?
1. **Analytics the site doesn't expose.** Sector breakdowns, geographic clustering, employee-range distribution, jobs-per-category, "top hiring companies" — the site shows lists, the CLI shows insights.
2. **Composable filters across resources.** "Robotics tag + funding stage Series A + has-remote-jobs + HQ in Texas" — none of these compose on the website; all are easy in SQL on a local store.
3. **Watch for changes.** Daily/weekly sync + diff. "What companies were added this week? Which roles opened? Which companies gained 5+ jobs?"
4. **Agent-native output.** Agents want JSON, `--select` field-pruning, `--csv` for analysis, `--limit` to bound responses. The website returns ~30 fields per company at all times.
5. **Offline-after-sync.** Once synced, every query runs without hitting the API.

## Product Thesis
- **Name:** `american-reindustrialization-pp-cli` (suggested slug: `american-reindustrialization`).
- **Why it should exist:** the public directory has a clean read API but no analytics, no programmatic access path, no way to slice/export, no diff-over-time. A CLI with local SQLite, FTS search, and a handful of analytical commands turns it from "browseable list" into "queryable knowledge base for the reindustrialization economy."

## Build Priorities
1. **Priority 0 (foundation):** Sync companies/jobs/categories/tags into local SQLite, build FTS indexes, expose `sync`, `search`, `sql`, `stale`.
2. **Priority 1 (absorbed read):** Mirror every public read endpoint listed under Table Stakes — list, search, detail, counts, autocomplete.
3. **Priority 2 (transcendence — to be brainstormed by Phase 1.5c.5 subagent):** Analytics, cross-entity joins, geographic clustering, ecosystem snapshots, watch/diff, agent-native composed queries.

## What Is Out of Scope (decided in Phase 1.6 — user picked public read-only)
- `/api/profile`, `/api/jobs/my-applications`, `/company-dashboard` (cookie session auth required, narrow value for a directory CLI).
- `/api/admin/*` (admin-only).
- Job application submission (each job's `apply_url` points off-site).
- Company submission (`/submit` is a contact form, not an API).

# here.now CLI Brief

## API Identity
- **Domain:** Agent-first publishing platform. Publish static Sites to live URLs instantly; store private files in cloud Drives. Spec self-describes: "here.now lets agents publish static Sites to live URLs and store private files in cloud Drives."
- **Users:** AI agents and developers who want to ship HTML/JS/static content to a live URL in one call, plus durable per-agent file storage and lightweight per-site data collections (form submissions, structured records).
- **Data profile:** Sites (slug, url, title, version, visibility), Site versions, Drives (id, name, fileCount, sizeBytes), Drive files (path, size, contentType, checksum), Site Data records (collection/recordId/data), Domains, Handles (subdomain), Links (short links), Variables (proxy-route service vars).
- **API surface:** OpenAPI 3.1.0, base `https://here.now`, 56 operations, 75 schemas, 9 tags. Auth = `Authorization: Bearer <API_KEY>` (`bearerAuth`, HTTP bearer, format "API key").

## Reachability Risk
- **None.** `GET https://here.now/openapi.json` → 200, valid 3.1.0 spec (107KB). Base host resolves (66.33.22.63) and serves the API. The publish endpoint additionally accepts **anonymous** requests (no key), so even key-less smoke tests work.

## Free vs Paid (the load-bearing product constraint)
- **Majority of users are FREE, not paid** (user directive). The CLI must make free flows first-class and degrade gracefully on paid endpoints.
- **Analytics is paid.** Tag description: "Paid-plan Site and account analytics rollups." `GET /api/v1/analytics` and `GET /api/v1/publishes/{slug}/analytics` will 402/403 for free users. CLI must catch the paid-gate response and print a friendly "requires a paid plan" message + typed exit code, never a raw 402 stack.
- **Anonymous publish is FREE and key-less.** `POST /api/v1/publish` has `security: [{bearerAuth:[]}, {}]` — the empty scheme means auth optional. An anonymous publish returns a `claimToken`; `POST /api/v1/publish/{slug}/claim` attaches it to an account later. This is the single best free-user onboarding: publish with zero signup, get a live URL, claim when ready.
- **Other anonymous-capable endpoints** (security includes `{}`): `getSite`, all Site Data record ops (these power public form submission on published sites), `readDriveFile` (with a share token).
- Plan quota limits (site count, drive storage, custom domains) are **not encoded in the spec** — they surface as runtime errors. CLI handles them at the error layer.

## Top Workflows
1. **Publish a local directory to a live URL.** Walk a dir → inline small text files (`files[]` with `{path, content}`) → presign-upload large/binary assets (`uploads[]` → PUT to presigned `uploadUrls[]` → `finalize`) → print live `url`. The raw API forces the agent to hand-build the files array and orchestrate uploads; the CLI collapses it to `publish ./dir`.
2. **Anonymous publish then claim.** `publish --anon ./dir` (no key) → store `claimToken` in a local vault → later `claim <slug>` with the stored token after setting a key.
3. **Use a Drive like a filesystem.** Push/pull/sync a local dir ↔ a Drive (`drive push`, `drive pull`, `drive sync`), read/write/move/delete files, list with sizes/checksums. Publish directly from a Drive (`publish from-drive`).
4. **Manage Site Data collections.** List/create/get/patch/delete records in a site's built-in collections (form submissions, content records). Search across them offline.
5. **Wire up domains / handle / short links.** Attach a custom domain, set a subdomain handle, create short links.
6. **Account hygiene.** Profile + username, profile-listed sites, service variables, support requests, account analytics (paid).

## Table Stakes (absorb every endpoint)
All 56 operations across Sites, Site Data, Drives, Domains, Profiles, Variables, Analytics, Support, Auth. Every endpoint becomes a typed command. No competitor CLI exists, so "table stakes" = full, correct coverage of the official API, beaten with offline store + `--json`/`--select`/`--dry-run`/typed exit codes.

## Data Layer
- **Primary entities to persist/sync:** Sites, SiteVersions, Drives, DriveFiles, SiteDataRecords, Domains, Links, Handles, Variables.
- **Sync cursor:** `updatedAt` on Sites/DriveFiles; full-list pull for the smaller collections (drives, domains, links, variables).
- **FTS/search:** Sites (slug/title/url), DriveFiles (path/contentType), SiteDataRecords (flattened data JSON). Offline search across all three is a key differentiator since the API only offers `publishes/search` (sites by query) and no cross-entity search.

## Competitive Landscape
- **No official CLI** (user confirmed). **No discoverable community CLI, MCP server, SDK, or code** referencing here.now's API as of this run (web search + `gh search repos`/`gh search code` for `here.now`, `herenow cli`, `HERE_NOW_API_KEY`, `/api/v1/publish` all empty). here.now is brand-new (API v0.1.0).
- here.now ships a **hosted agent skill** (referenced on the site; page is JS-rendered and not statically fetchable). It is the closest "competitor" — a prompt-level skill, not a binary. Our CLI beats it with: offline mirror, directory-aware publish orchestration, drive-as-filesystem sync, cross-entity offline search, anonymous-claim vault, typed agent-native output.

## User Vision
- API key provided for dogfooding (stored in session dir, never in artifacts).
- **Explicit directive:** "we also need to work and test with the free plan since majority of people will be free users not paid." → free-plan-graceful behavior is shipping scope, not polish: anonymous publish must work key-less; paid endpoints (analytics) must fail soft with clear messaging; `doctor` must report plan-aware status; quickstart must not require a paid key.

## Product Thesis
- **Name:** here.now CLI (`here-now-pp-cli`).
- **Why it should exist:** here.now's API is powerful but low-level — publishing a real site means hand-building a files array, presigning uploads, finalizing, and tracking claim tokens by hand. There is no tool that turns "publish this folder" into one command, mirrors your Drives offline, or searches everything you've shipped. The CLI is the missing layer: `publish ./site` → live URL, `drive sync`, offline search, and a free-user path that needs no account to start.

## Build Priorities
1. **P0 — data layer + sync + offline search** across Sites, Drives, DriveFiles, SiteDataRecords, Domains, Links, Variables.
2. **P1 — absorb all 56 endpoints** as typed commands with `--json`/`--select`/`--dry-run`/typed exit codes; free-plan-graceful error handling for analytics and quota gates.
3. **P2 — transcendence:** directory-aware `publish ./dir` (inline+presigned orchestration + finalize), `drive push/pull/sync` (rsync-style), anonymous publish + local claim-token vault, cross-entity offline `search`, `doctor` with plan-aware diagnostics.

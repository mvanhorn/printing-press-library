# Dub CLI Brief

## API Identity
- **Domain:** Modern link attribution platform — short links, conversion tracking, affiliate/partner programs. Open source (AGPL-3.0), self-hostable, vendor-maintained at dubinc/dub. Marketed as "the modern Bitly."
- **Users:** Marketers running attribution campaigns, growth engineers automating UTM/short-link generation, DevRel teams shipping referral/partner programs, partner-program operators (now also bounties operators), founders running affiliate marketing for SaaS.
- **Data profile:** Links + analytics aggregates dominate. Event stream (`/events`) for real-time. Long-tail resources (partners, applications, commissions, bounties, bounty submissions, customers, payouts) only matter if conversion-tracking / partner program is enabled. Many small entities (tags, folders, domains).

## Reachability Risk
- **None.** dubinc/dub repo is highly active. Spec ships nightly via Speakeasy from `dubinc/dub-ts`. Tested live at `https://api.dub.co` with the user's workspace key — HTTP 200 on `/links`, real workspace data ("compound-engineering" link). The only `403` issue (#1072) is for self-hosters configuring PlanetScale, not API consumers.

## Top Workflows
1. **Bulk campaign link creation** — UTM-tagged variants per channel/audience using `POST /links/bulk` with `--stdin-jsonl` and `--dry-run`. Organize with tags + folders. Highest-frequency batch operation for marketers.
2. **Top-link analytics rollup** — `/analytics?groupBy=top_links&interval=30d` + `/events` stream. Identify winners and tail. Pull weekly/monthly into reporting.
3. **Conversion attribution tracking** — server-side `/track/lead` + `/track/sale` (+ `/track/open`) tied to customers for revenue attribution. Native Stripe/Shopify integrations send events here.
4. **Partner program ops** — recruit, approve/reject applications, ban/deactivate, set commission rates, run bounties (Performance / Submission / Social Metrics), process payouts. Surface area expanded since 2026-04: now 9 partner endpoints + 3 bounty-submission endpoints + `/partners/analytics`.
5. **Domain migration + branded QR** — vanity domain registration, status polling, per-link QR generation with logo overlay (`/qr` returns image, base64-friendly).
6. **(NEW) Bounty submission triage** — review partner-submitted proof for blog posts / social posts / videos, approve or reject. Manual today; CLI-friendly batch operation.

## Table Stakes
- Create / update / delete short links (custom slugs, password, expiration, geo targeting, device targeting, A/B variants, prefix, externalId).
- Bulk operations on links (POST/PATCH/DELETE bulk endpoints).
- List + filter links (by domain, tag, folder, search, date range; pagination via `page`/`pageSize`, max 100).
- Get link analytics (clicks, leads, sales aggregated by interval / dimension).
- Tag and folder management (CRUD).
- QR code generation (with optional logo, color).
- Domain CRUD + verification + DNS status polling.
- Workspace-scoped Bearer auth, `DUB_API_KEY` env var (Speakeasy convention).

## Codebase Intelligence
- **Spec source (authoritative):** `https://raw.githubusercontent.com/dubinc/dub-ts/main/.speakeasy/out.openapi.yaml` — 361 KB, OpenAPI 3.0.3, **39 paths / 53 operations**. Generated nightly by Speakeasy. JSON inside a `.yaml` file; the parser auto-detects.
- **Auth:** Single security scheme `token` (`type: http, scheme: bearer, x-speakeasy-example: DUB_API_KEY`). Header: `Authorization: Bearer dub_xxx`. Workspace-scoped — the key implicitly chooses the workspace.
- **Base URL:** `https://api.dub.co`
- **Resource groups (op count):** links 11 (CRUD + bulk + count + upsert + info), partners 9 (list/create/get/update/applications-list/approve/reject/ban/deactivate, plus partner-links 3 and partner-analytics 1), domains 6, bounties 3 (submissions list/approve/reject), commissions 3 (list, update one, bulk update), customers 3 (list, get, update, delete), tags 4 (list/create/update/delete), folders 4 (list/create/update/delete), track 3 (lead/sale/open), analytics 1, events 1, payouts 1, qr 1, tokens 1 (embed/referrals).
- **Verb quirks:** `/links/upsert` and `/partners/links/upsert` are **PUT**. `/commissions/bulk` is **PATCH** (not POST).
- **Rate limits:** Free 60/min → Enterprise 3000/min. Analytics endpoints have stricter per-second caps (Pro 2/s, Advanced 8/s). Headers: `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After` on 429s.
- **Pagination:** `page` (default 1), `pageSize` (max 100), `sortBy`, `sortOrder`. Cursor-based for events/analytics.
- **Architecture:** Workspace-scoped API keys. No public `/workspaces` endpoint (created via dashboard). Returned 401 on probe — confirms the workspace-scoped key model.

## What Changed Since The Prior Print (April 2026, v2.3.9)
- **Official Dub CLI now exists** — `dub-cli` on npm at v0.0.12 (issue #506 closed). Covers 6 commands: `login`, `config`, `domains`, `shorten`, `links`, `help`. Tiny surface — ~3-4 of 53 operations. Our `dub-pp-cli` still wins by ~10× on coverage and entirely on agent-native + transcendence.
- **Bounty submissions endpoints** (`/bounties/{bountyId}/submissions`, `/approve`, `/reject`) — fully shipped after the prior print landed. Three Performance / Submission / Social Metrics bounty types are GA (per the dub.co blog).
- **`/partners/analytics`** — new dedicated partner-attributed analytics endpoint, separate from `/analytics`.
- **`/partners/ban`** and **`/partners/deactivate`** — distinct paths now (prior spec had different shape for ban; deactivate is brand new).
- **`/customers/{id}` now supports PATCH + DELETE** (prior was read-only).
- **`/tokens/embed/referrals`** — single endpoint (prior was `/tokens/embed/referrals/links`).
- These spec deltas justify rebuild (and the v2.3.9 → v3.6.0 binary delta justifies the redo on its own).

## User Vision
- User explicitly approved a **full research and redo** despite a prior print existing. They want a fresh manifest, with the prior 14 novel features as **input** to a reconcile step (keep / reframe / drop / add).
- **Full dogfood is authorized** — including writes (link creation/update/delete), real link generation, click-through to drive stats, and analytics queries. API key provided live.
- Implication: Phase 5 must be the full mechanical test matrix, not Quick Check. We can mutate the live workspace; we should still tag created links with an obvious test marker (`pp-test-` prefix) and clean up.

## Product Thesis
- **Name:** `dub-pp-cli`
- **Why it should exist (sharper than the prior print):**
  1. The official `dub-cli` shipped as a 6-command MVP — link shortening + workspace config. It does **not** cover analytics, bulk operations, partner programs, bounties, conversion tracking, customers, commissions, or payouts. We absorb everything plus give agents a single typed surface.
  2. The single MCP (gitmaxd) still wraps 4 of 53. No replacement.
  3. Power-user marketing workflows are batch-shaped (campaign-wide UTM, top-link rollups, partner audits, bounty submission review) — a CLI with `--stdin`, `--dry-run`, and `--json` wins decisively over click-through dashboards.
  4. Local SQLite store enables transcendence the API can't: dead-link detection (analytics × links), drift detection (analytics history), commission audit (partners × commissions × bounties × payouts), bounty submission triage queue, link/UTM bulk rewrite with diff preview.
  5. Agents authoring marketing automation need a single binary with typed exit codes (0/2/3/4/5/7/10), `--select`, and offline search. Today they shell out to curl + jq.

## Build Priorities
1. **Foundation (P0):** Local SQLite store for `links`, `tags`, `folders`, `domains`, `partners`, `commissions`, `bounties`, `customers`, `analytics_buckets`, `events`. Cursor-based sync. FTS5 search across links + customers + partners.
2. **Absorb (P1):** All 53 spec operations as commands with `--json`, `--dry-run`, `--select`, agent-native exit codes. Auth via `DUB_API_KEY` (Speakeasy convention, supports `DUB_TOKEN` alias for back-compat with prior print).
3. **Transcendence (P2 — built by hand, see absorb manifest for full list):**
   - Dead-link detection (zero clicks N days).
   - Drift detection (week-over-week click drop > threshold).
   - Slug-collision lint.
   - Bulk URL/UTM rewrite with diff (dry-run).
   - Commission audit reconciliation.
   - Workspace health doctor (rate-limit headroom, expired-but-active links, stalled domains, bounty submissions awaiting review).
   - Time-windowed change feed (`since 24h`).
   - Top-performer rollup with tag/folder joins.
   - **NEW:** Bounty submission triage queue (groups partner-submitted proof by status, age, bounty type).
   - **NEW:** Partner attribution rollup joining `/partners/analytics` with local `partners` × `commissions` for per-partner ROI.
4. **Polish:** Bulk mutations gated by `--yes` above N items. QR generation supports `--out file.png` and base64 stdout. Doctor reports rate-limit headers + workspace ID derived from API echo. README headlines the official CLI's coverage gap.

## Key Risks / Watchpoints
- **Workspace-scoped key isolation** — every test against a real workspace mutates real state. Tag created links with `pp-test-` and `--ttl 1h` (or auto-archive in Phase 5 cleanup) to avoid polluting the user's workspace.
- **Rate-limit awareness on analytics** — per-second caps (Pro 2/s) bite hard on tight loops. Adaptive backoff is a must for `sync` and `analytics` commands.
- **`PUT /links/upsert` and `PATCH /commissions/bulk` quirks** — verify the generator emits these correctly.
- **`/qr` returns binary** — generator must not try to JSON-parse the response; route to `--out file.png` or base64 stdout.

## Sources
- Official spec: [dubinc/dub-ts speakeasy out.openapi.yaml](https://github.com/dubinc/dub-ts/blob/main/.speakeasy/out.openapi.yaml)
- Official CLI docs: [dub.co/docs/sdks/cli](https://dub.co/docs/sdks/cli)
- Original CLI request issue (closed): [dubinc/dub#506](https://github.com/dubinc/dub/issues/506)
- Bounties product line: [dub.co/blog/introducing-bounties](https://dub.co/blog/introducing-bounties), [dub.co/blog/social-metrics-bounties](https://dub.co/blog/social-metrics-bounties)
- Affiliate program scale: [dub.co/blog/10m-payouts](https://dub.co/blog/10m-payouts)
- Rate limits: [dub.co/docs/api-reference/rate-limits](https://dub.co/docs/api-reference/rate-limits)
- Stale community CLI: [sujjeee/dubco](https://github.com/sujjeee/dubco)
- Single existing MCP: [gitmaxd/dubco-mcp-server-npm](https://github.com/gitmaxd/dubco-mcp-server-npm)

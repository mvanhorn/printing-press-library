# Skool CLI Brief

## API Identity
- **Domain:** Community / course platform (skool.com). Members consume posts, classroom courses, calendar events; admins moderate; creators monetize paid communities.
- **Users:** Community owners (Cadence/BTD shape — paid ~$99/mo audiences), engaged members, admins of multi-community portfolios.
- **Data profile:** Posts (forum, threaded comments), classroom courses + lessons (Mux video URLs, attachments, polls), calendar events, members (level, points, streaks, last-active), notifications, chat messages, leaderboard (computed). All keyed to a community slug.

## Reachability Risk
- **Low — once auth + UA are set.** CloudFront returns 403 to bare curl. Direct HTTP works once `auth_token` cookie + realistic `User-Agent` are sent (confirmed across all 5 working community wrappers). No TLS-impersonation library needed.
- `probe-reachability` returned `browser_clearance_http` only because it doesn't send the auth cookie. With cookie + UA, runtime is `standard_http`.
- **Two hosts:** `www.skool.com` (reads, via Next.js `/_next/data/<buildId>/*.json` routes) and `api2.skool.com` (writes, REST). Same cookie works for both.
- **Gotchas:** `buildId` rotates on Skool deploys — refresh from `/` HTML when reads fail with mismatched route. AWS ALB stickiness cookie (`AWSALBCORS`) should be echoed back on subsequent requests.

## Top Workflows
1. **Sync community offline** — pull all posts/comments/members/courses/events into local SQLite for SQL queries, FTS, exports.
2. **Auto-DM new members beyond Skool's 1-message native AutoDM** — drip sequences, conditional copy.
3. **Leaderboard / churn analytics** — point velocity, level moves, members at risk (drop-off detection); week-over-week deltas the native UI can't show.
4. **Export posts/comments to markdown** — for backups, search, LLM ingestion.
5. **Classroom export** — courses, lessons, Mux video URLs, attachments, polls.
6. **Cross-community ops** — manage 2+ communities you own from one CLI.
7. **Calendar event sync** — push to Google Cal / iCal (no clean OSS solution today).

## Table Stakes (from competitors)
- Posts: list, get, create, update, delete, comment.
- Members: list, search, pending list, approve/reject/ban.
- Courses & lessons: list, get, full export with attachments.
- Notifications: list, mark-read.
- Calendar events: list, create.
- Chat: list channels, get history (read-only minimum).
- `community info`, `me`, `doctor`.
- Markdown ↔ TipTap conversion for write paths (Skool's editor uses TipTap JSON).

## Data Layer
- **Primary entities:** community, post, comment, member, course, lesson, calendar_event, notification, badge_level, chat_message.
- **Sync cursor:** per-entity `updated_at` watermark; posts and comments support pagination via cursor in `_next/data` routes.
- **FTS:** posts (title + body), comments (body), members (name + bio), lessons (title + body). Cross-entity FTS via `resources_fts`.
- **Snapshot tables:** member_points_snapshot (daily) → enables leaderboard delta queries the native UI can't do.

## Codebase Intelligence
- Source: cristiantala's dev.to writeup, louiewoof2026/skool-mcp source, FlowExtract scraper-pro README.
- **Auth:** `auth_token` JWT cookie, ~1-year expiry, `httpOnly`. No bearer, no CSRF. Required header: `User-Agent` (any realistic value).
- **Data model:** REST-ish on `api2.skool.com` for writes; Next.js data routes for reads. Markdown→TipTap JSON conversion needed for post bodies.
- **Rate limiting:** No published numbers. Community wrappers default to ~250–1000ms delay between writes. No 429 examples in any issue.
- **Architecture:** the `buildId` rotation is the only real trap. Wrappers handle it by refetching `https://www.skool.com/` and parsing `__NEXT_DATA__`.

## Auth Tier
- Single tier: cookie session (`auth_token` JWT). Tier 3 in user's auth strategy (high-stakes — full account access). Manual session config only; never plaintext to env.
- Config file: `~/.config/skool-pp-cli/config.toml` with `auth_token = "<JWT>"` (mirrors substack-pp-cli pattern).

## Source Priority
- Single source — Skool. No combo CLI. Skip Multi-Source Priority Gate.

## Product Thesis
- **Name:** `skool-pp-cli`
- **Display name:** Skool
- **Headline:** Every Skool community feature, plus a local SQLite mirror, FTS, and cross-community ops no other Skool tool ships.
- **Why it should exist:** The 5 existing Skool wrappers are stateless scrapers or one-shot MCPs. None persist data, none compose across communities, none let an agent run `skool members at-risk --weeks 4 --json` in one call. Owners running paid communities need offline analytics + automated workflows that survive `buildId` rotation, cookie expiry, and CloudFront friction without each script reimplementing the auth dance.

## Build Priorities
1. **Foundation** — config (TOML cookie), client (auth_token + UA + buildId resolver + AWSALBCORS echo), store (SQLite + FTS5 + snapshot table), sync command.
2. **Absorb (match every existing tool)** — 14 louiewoof MCP tools + cristiantala's full surface (posts CRUD, members CRUD+moderate, classroom export, courses/lessons, calendar, chat read, notifications, community info, me, doctor).
3. **Transcend (only we can do)** — at-risk members (point-velocity drop), leaderboard delta over N weeks, post velocity rank, classroom export to markdown bundle, cross-community SQL view, since-time digest, calendar to ics, churn-cohort report.

## Anti-triggers (what this CLI is NOT)
- Not a Skool admin replacement for billing/Stripe — Skool doesn't expose Stripe to community wrappers.
- Not a chat bot — read-only chat.
- Not for skool.com (the school education site) — wrong product.

## Risks / Gaps to flag at Phase 1.5
- **Markdown ↔ TipTap conversion** for write paths is non-trivial; cristiantala has it working — we'll use a Go markdown→TipTap JSON encoder. May ship v1 with markdown-only-paragraphs and defer rich blocks.
- **Calendar write** has no proven OSS implementation — will start read-only, mark create as v0.2 stub if browser-sniff doesn't surface the endpoint.
- **Chat write** intentionally out of scope.

# yt-studio CLI Brief

Source pre-work: `HANDOFF_pp-yt-studio.md` (in repo) and approved design spec at
`/Users/kamibot/.openclaw/docs/superpowers/specs/2026-05-14-pp-yt-studio-design.md`.
Scope is locked. This brief synthesizes those plus targeted research to drive the build.

## API Identity
- Domain: YouTube creator analytics for Kami's PoE 2 / Last Epoch channel and a
  competitor watchlist. Replaces the qualitative audience-obsession framework with
  per-video measurable evidence (retention as belief-shift signal, CTR as hook signal,
  competitor titles as niche-reward signal).
- Surfaces (one provider, two surfaces):
  1. **YouTube Data API v3** (OAuth `youtube.readonly`) — channel/video/playlist/search.
  2. **YouTube Analytics API v2** (OAuth `yt-analytics.readonly`) — retention curves
     via `elapsedVideoTimeRatio` + `audienceWatchRatio`, demographics, traffic
     sources, and (as of January 15, 2026) thumbnail impressions + CTR per video.
  3. **YouTube Studio web (Innertube XHR)** — real-time analytics (last 48h) and
     thumbnail A/B variant breakdowns that the public Analytics API does not expose.
     Captured via cookie session from `login` command.
- Users: solo creator with one channel + a curated competitor watchlist (size 5–20).
- Data profile: per-channel timelines, per-video time-series (daily metrics + retention
  curves), categorical breakdowns (demographics, traffic source).

## Important Finding — Public API now covers most "Studio-only" data
Research surfaced that on **2026-01-15** Google added `videoThumbnailImpressionsClickRate`
and related metrics to the YouTube Analytics API. Retention curves were already public
via `elapsedVideoTimeRatio`. **Today (2026-05-14) is 4 months after that change.**

What this means for the build:
- ~95% of the design spec's "Studio sniff required" data is now reachable via
  OAuth-authenticated public APIs (Data API + Analytics API).
- Studio sniff (cookie session) uniquely retains: **real-time analytics** (last 48h)
  and **A/B thumbnail variant breakdowns**.
- For v1, we still implement the Studio session capture in `login` (per locked scope),
  but the Analytics-API path is the primary surface. `retention <video_id>`,
  `ctr-decay <video_id>`, demographics commands all run on the public API.
- Quota implication: Analytics API doesn't burn the Data API 10K/day quota.
  `search.list` is the only hot quota item; the design spec's
  `--data-source auto` + `--concurrency 2` caps are still correct.

Scope stays locked (hybrid + read-only). Architecture leans on the public APIs first.

## Reachability Risk
- **Low.** Both APIs are stable Google services. Phase 1.9 will probe
  `youtube.googleapis.com` (unauthenticated 401 expected, which passes the gate).
- Studio Innertube is reverse-engineered but extremely stable in shape; the bigger
  risk is session expiry, which `login` handles by typed exit 4.

## Top Workflows (the killers)
1. **Framework-audit a published video.** Pull retention curve + script lines from
   `~/.openclaw/workspace/data/`. Cross-check whether Signal / Belief Shift / Action
   CTA structure aligns with retention drops. Exit with a structured verdict.
2. **Cohort retention compare.** "How do my Rework-series videos retain vs my Build-Guide
   videos?" — `retention-cohort --pattern "Rework"` averages curves across matching titles.
3. **Title pattern mining.** What words in my titles correlate with above-median CTR?
   Token-level analysis on the local store: `title-patterns --winners --losers`.
4. **Competitor benchmark.** Where does my CTR / retention / upload cadence sit vs the
   five biggest PoE-2 / Last-Epoch creators? `vs-watchlist --metric ctr,retention,upload-cadence`.
5. **Idea gap.** What topics did the watchlist cover in the last 14 days that we haven't?
   `idea-gap` — joins `search.list` snapshots against own-channel video corpus.

## Table Stakes (absorbed from competing tools)
Research surveyed all known YouTube CLIs and MCP servers (see Absorb Manifest).
None of them touch creator-side analytics. The "table stakes" they collectively own:
- Channel info (`channel info --self`)
- Video metadata (`video get <id>`, `videos list --channel`)
- Playlist metadata + items
- Public stats (views, likes, comments)
- Search videos (`search "<q>"`)
- Trending videos by region
- Comments list (read-only)
- Video categories
- Captions list / transcript fetch (defer to v1 polish — uses yt-dlp pattern)

The novel CLI must match all of these AND beat them with offline SQLite, normalized
multi-channel schema, FTS5 search, and `--json --compact` agent contract.

## Data Layer (from design spec, refined)
- SQLite at `~/.openclaw/state/yt-studio/yt-studio.db` (separate from
  `content-ops-library.db`; `ATTACH`-able via `sql` command).
- Primary entities: `channels`, `videos`, `video_metrics_daily`, `retention_curves`,
  `thumbnail_impressions`, `demographics`, `search_idea_gap`, `script_videos`.
- FTS5: `videos_fts(title, description)` (and `script_videos_fts(signal_line, …)`
  for framework-audit grep).
- Sync cursor: `channels.last_synced_at`; per-video `video_metrics_daily.day` watermark.
- Cross-DB: `sql --attach ~/.openclaw/workspace/data/content-ops-library.db` for
  joins against existing publishing-pipeline tables.

## Codebase Intelligence
- Reference SDK: `google.golang.org/api/youtube/v3` (Google official Go client,
  maintenance mode). Provides typed structs for every Data API resource. The CLI's
  generated client can wrap it for the Data API surface.
- For Analytics API: `google.golang.org/api/youtubeanalytics/v2` — same project.
- For Studio Innertube: hand-rolled client. The internal endpoint shape is
  `https://studio.youtube.com/youtubei/v1/analytics/...` with `Cookie:` + `X-Goog-AuthUser`
  + an Innertube context blob. Session cookies harvested by `login`.
- Auth: composed pattern (OAuth refresh for Data + Analytics; cookie jar for Studio).
  The press's internal-YAML spec supports `oauth2` auth type; the cookie layer is
  hand-built in Phase 3 alongside the `login` command. Token storage:
  - OAuth tokens: `~/.openclaw/state/yt-studio/oauth.json` mode 600.
  - Studio session: `~/.openclaw/state/yt-studio/studio-session.json` mode 600.
- Rate limiting: typed exit 7 + retry hint + backoff with jitter. Daily quota
  logged to stderr in `--quiet` mode per the design spec.

## Source Priority
Single provider (YouTube). Not a combo CLI in the multi-source-priority sense.
Surfaces ranked by reliance:
- Primary: **YouTube Data API v3** (OAuth) — channel/video/playlist/search.
- Primary: **YouTube Analytics API v2** (OAuth) — retention, demographics, CTR.
- Secondary: **YouTube Studio Innertube** (cookie session) — real-time + A/B variants only.

## User Vision (Kami, from HANDOFF + design spec)
- Replace qualitative audience-obsession discipline with measurable per-video evidence.
- Framework-audit is the killer; binds retention to script lines via `content-registry.md`.
- KamiOps Dashboard will shell out to `yt-studio-pp-cli` for analytics; CLI is
  post-publish only (not publishing pipeline).
- Read-only v1; writes deferred to polish.

## Product Thesis
- Name: `pp-yt-studio` (binary: `yt-studio-pp-cli`).
- Headline: "Every YouTube creator metric that matters, offline-queryable, with
  framework-audit binding to your scripts. The CLI that turns retention curves into
  belief-shift evidence."
- Why it should exist: every existing YouTube CLI/MCP is a downloader or transcript
  fetcher. None handle creator analytics, none have a local store, none bind to scripts.
  Framework-audit is unique — no other tool joins retention drops to script structure.
- Differentiators vs the ecosystem:
  - Local SQLite + FTS5 (vs incumbents' stateless API calls).
  - Multi-channel normalized schema (own + watchlist).
  - Public Analytics API surfaced via Cobra commands (no other CLI does this).
  - Script binding via `content-registry.md` (genuinely novel — no precedent).
  - `--json --compact` agent contract for downstream Dashboard integration.

## Build Priorities
1. **Foundation:** Data layer (all 8 tables), `sync --full` / `sync --since`, FTS5,
   `sql` and `search` commands.
2. **Absorb:** Match every operation in the competing MCPs (14 tools from
   danny/youtube-mcp-server, etc.). Add `--json`, `--compact`, `--select`, `--dry-run`,
   typed exit codes.
3. **Transcend (hero commands from HANDOFF):**
   - `retention <video_id>` (JSON + ASCII sparkline + 3 sharpest drops auto-annotated)
   - `retention-cohort --pattern "<regex>" --days <N>`
   - `ctr-decay <video_id>` (first-72h vs day-30)
   - `vs-watchlist --metric ctr,retention,upload-cadence`
   - `title-patterns --winners --losers`
   - `idea-gap` (14-day competitor topic delta)
   - `framework-audit <video_id>` (Signal/BeliefShift/CTA × retention join) — THE KILLER
   - `watchlist suggest/add/remove/list`
   - `script-link <video_id> <script_path>` (manual fallback for framework-audit)
   - `login` (OAuth + Studio cookie capture, interactive only)
4. **Polish:** Real-time Studio sniff (deferred from v1 if it complicates auth
   review); writes; comment moderation; live streaming analytics — all out of scope.

## Out of scope v1 (from design spec, restated)
- Write operations (titles, descriptions, thumbnails, playlists)
- Live streaming analytics
- Community posts / Shorts analytics
- Monetization (RPM, CPM, AdSense)
- Comment moderation
- Captions transcription (defer; use existing yt-dlp-based tools for now)

## Sources
- HANDOFF_pp-yt-studio.md (this repo)
- /Users/kamibot/.openclaw/docs/superpowers/specs/2026-05-14-pp-yt-studio-design.md
- /Users/kamibot/.openclaw/workspace/data/content-registry.md (binding format)
- developers.google.com/youtube/analytics/dimensions (elapsedVideoTimeRatio)
- developers.google.com/youtube/analytics/metrics (audienceWatchRatio, thumbnail CTR added 2026-01-15)
- developers.google.com/youtube/v3 (Data API)
- pkg.go.dev/google.golang.org/api/youtube/v3 (Go SDK)
- github.com/dannySubsense/youtube-mcp-server (14-tool MCP for absorb)
- github.com/anaisbetts/mcp-youtube, adhikasp/mcp-youtube, nattyraz/youtube-mcp (transcript MCPs)
- github.com/danvega/youtube-cli (Spring Shell channel stats reference)

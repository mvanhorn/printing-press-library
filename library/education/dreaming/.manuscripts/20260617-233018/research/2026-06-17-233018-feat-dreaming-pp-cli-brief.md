# Dreaming CLI Brief

## API Identity
- **Domain:** Comprehensible-input language learning (Dreaming Spanish + Dreaming French). TIME Best Inventions 2025. App at `app.dreaming.com`.
- **Users:** Dedicated Spanish/French learners following the hours-based "roadmap to fluency." Power users obsessively track cumulative input hours and pace toward level milestones.
- **Data profile:** Per-user time-series (daily watched seconds, external-input log entries, total hours, streak, daily goal, level), plus a large video catalog (~1k free / 3k–7k+ premium) with rich per-video metadata.
- **Backend:** Undocumented **Netlify Functions** at `https://app.dreaming.com/.netlify/functions/` (current host; legacy `https://www.dreamingspanish.com/.netlify/functions/`). Plain HTTPS JSON — `probe-reachability` confirms `standard_http`, no bot protection, no clearance cookie.
- **Auth:** `bearer_token` — `Authorization: Bearer <token>`. Token lives in browser `localStorage` key `token`; also mintable via `POST /signIn {email,password}`. All endpoints 401 without it. Scope = full account (email/name; excludes payment).

## Reachability Risk
- **MEDIUM-HIGH.** Undocumented private backend, no stability contract. A domain migration already broke the ecosystem once (`www.dreamingspanish.com` → `app.dreaming.com`, added `language` param — HarryPeach issue #10). Token is scraped from localStorage and expires (no third-party refresh flow).
- **Mitigation in CLI design:** make base host + `language` configurable (default `app.dreaming.com`, `es`); accept token via env/flag; treat 401/403 (expired token) and 3xx/404 (host moved) as the two named failure modes with actionable `doctor` guidance.
- Endpoint *shapes* are stable across ~5–10 independent codebases over years; only host + auth are the real risk vectors.

## Internal API Surface (extracted from community source, host = app.dreaming.com/.netlify/functions)
| Path | Method | Notes |
|---|---|---|
| `/user?timezone=<tz>` | GET | `{user:{email, watchTime(sec), externalTimeSeconds, dailyGoalSeconds, Subscription:{status,currentPeriodEnd,cancelAtDateEnd}}}` |
| `/dayWatchedTime?date=<YYYY-MM-DD>` | GET | array of `{date, timeSeconds}` (per-day on-platform watch time) |
| `/externalTime?language=es\|fr` | GET / POST / DELETE | GET `{externalTimes:[{type,description,timeSeconds,date,_id}]}`; POST `{date,description,timeSeconds,type:"watching"}`; DELETE by id |
| `/videos?language=es\|fr` | GET | `{Videos:[{_id,title,level,dialect,guide,subscription,...}]}` full catalog |
| `/playlist` | GET | watched list `[{videoId,addedDate}]` |
| `/video?id=<24hex>` | GET | single video metadata + stream |
| `/inspectExternalVideo?videoUrl=<encoded>` | GET | resolve a YouTube URL for external logging |
| `/signIn` | POST | `{email,password}` → token |
- Thumbnail CDN: `d36f3pr6g3yfev.cloudfront.net`. Video id = 24-hex. `language` query param selects es/fr catalog + external log.

## The Roadmap (domain core — load-bearing for transcendence features)
- Single master metric: **cumulative input HOURS** (on-platform watchTime + manual externalTime).
- Level → cumulative-hours map (client-side convention, stable across tools): **L1=0, L2=50, L3=150, L4=300, L5=600, L6=1000, L7=1500**.
  - 300h: speaking becomes optional. 600h: speaking recommended + reading introduced. 1500h: "native-like for practical purposes." (Halve if coming from a related Romance language.)
- Video difficulty: 4 tiers (Superbeginner/Beginner/Intermediate/Advanced) **plus a numeric 1–100 difficulty rating** (finer-grained). Catalog also indexed by guide, dialect/accent (~9 regions), topic/tags (18+), series, sound-type (podcast-friendly), free/premium.

## Top Workflows
1. **Log external input hours** — most frequent recurring manual action (podcast/Netflix/YouTube time, with description). One-at-a-time in the web app; no bulk/CSV.
2. **Check progress toward next level** — hours-to-next-milestone, projected level dates vs daily goal.
3. **Find the next unwatched video** at the right level/difficulty (filter level+difficulty, hide watched, sort Easy/Hard/New).
4. **Maintain streak / hit daily goal** — confirm today's minutes counted.
5. **Plan to hit an hours target by a date** — back-solve required daily minutes to reach e.g. 600h/1500h by a deadline (currently mental/spreadsheet math).
6. **Curate by guide / accent / series** — follow specific teachers or a dialect.

## Table Stakes (what every community tool already does)
- Read user stats (total hours, external seconds, daily goal, subscription, streak).
- Per-day watched-time history + export (CSV/Excel).
- External-time log: list, add, delete; bulk re-tag by category.
- Full video catalog list with filters (level, dialect, guide, subscription, exclude-watched) and sorts.
- Watched/unwatched (playlist) state.
- Required-daily-average to hit a target date or target level.
- Milestone/level ETA projection; best days; rolling averages; per-guide/per-tag breakdowns.
- Add a YouTube URL to external hours (resolve via `inspectExternalVideo`).
- Precise (unrounded) hours display.

## Data Layer
- **Primary entities:** `videos` (catalog, large, slow-changing → cache & FTS), `external_time` (log entries), `day_watched_time` (daily series), `user` (stats snapshot), `playlist` (watched ids). Optionally `daily_progress` = merged series of on-platform + external per day.
- **Sync cursor:** date-based for `dayWatchedTime`/`externalTime`; full snapshot for `videos`/`user`.
- **FTS/search:** FTS5 over the video catalog (title, guide, dialect, topic, series) — offline next-video selection that the app can't do.
- This local store is what unlocks every transcendence feature (level ETA, pace planning, frequency/difficulty-sorted next-video, historical trends, what-if planning).

## Codebase Intelligence (community, no official repo)
- Auth: Bearer token from localStorage `token`; `/signIn` mints one. (HarryPeach `bearer_how_to.md`, yt-dlp-dreaming.)
- Endpoint coverage richest in sk82jack/DSToolbox (PowerShell) and HarryPeach/DreamingSpanishStats (Python/Streamlit). yt-dlp-dreaming confirms `_API_BASE=https://app.dreaming.com/.netlify/functions`, `signIn`, `video?id=`.
- Architecture insight: pure serverless function-per-endpoint; no GraphQL; `language` param multiplexes es/fr.

## User Vision
- User pointed at `app.dreaming.com` and chose "the app itself." Interpreted as: a CLI that works against the real Dreaming backend for the logged-in learner. No further feature constraints given; build the GOAT for the Dreaming roadmap.

## Product Thesis
- **Name candidate:** `dreaming` (binary `dreaming-pp-cli`).
- **Thesis:** Every Dreaming community tool's feature — stats, external-time logging, catalog browsing, milestone projection — in one offline-first, agent-native CLI with a local SQLite store, FTS catalog search, and roadmap math no single tool combines. The only CLI for Dreaming, and the only tool that unifies the fragmented extension/script ecosystem.
- **Why install over the web app + extensions:** bulk/scriptable external-hours logging (CSV import), real "hit X hours by date" planner, frequency/difficulty-sorted next-video picker offline, exportable history, and a Claude-native MCP surface no other tool offers.

## Build Priorities
1. **P0 data layer + sync + FTS** for videos, external_time, day_watched_time, user, playlist.
2. **P1 absorb** every community-tool feature: user/stats, dayWatchedTime list+export, externalTime CRUD, video catalog list/filter/sort, playlist (watched), inspectExternalVideo→add, signIn/auth, precise time, required-daily-average, milestone ETAs, best days, rolling averages, per-guide/per-tag breakdowns.
3. **P2 transcend:** roadmap planner ("hit 600h by 2026-12-31 → N min/day"), frequency/difficulty-sorted `next` picker, level-ETA `forecast`, `since`/`tail` time-window stats, CSV bulk import of external hours, account/language migration, streak intelligence, offline catalog SQL.

## Auth Decision
- `bearer_token`. Env `DREAMING_TOKEN`; `auth login` (email/password → `/signIn` → store token); `auth set-token`/`status`/`logout`. Base host + `language` configurable. No browser runtime.

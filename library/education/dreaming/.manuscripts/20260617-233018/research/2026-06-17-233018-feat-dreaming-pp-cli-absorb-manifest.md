# Dreaming CLI — Absorb Manifest

Binary: `dreaming-pp-cli`. Backend: `https://app.dreaming.com/.netlify/functions` (configurable host + `language=es|fr`). Auth: Bearer token.

## Absorbed (match or beat every feature that exists in any Dreaming tool)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | User stats (total hours, external secs, daily goal, subscription) | DSToolbox Get-DSUser / HarryPeach | `user` / `stats` | offline cache, `--json`/`--select`, precise hours |
| 2 | Per-day watched-time series (date-range) | DSToolbox Get-DSDayWatchedTime | `days [--from --to]` | SQLite series, offline |
| 3 | Export day-watched + external time (CSV) | DSToolbox Export-* / ds-insights-web | `days export --csv`, `external export --csv` | agent-native, scriptable |
| 4 | List external-time entries (filter type/date) | DSToolbox Get-DSExternalTime | `external list` | FTS on description |
| 5 | Add external-time entry | importer / ds-logger / DreamingSync | `external add --minutes --desc --type` | `--dry-run`, idempotent |
| 6 | Delete external-time entry by id | TikeDev importer-enhanced | `external rm <id>` | `--dry-run`, confirm |
| 7 | Bulk re-tag external entries by category prefix | DSToolbox Update-DSExternalTimeDescription | `external retag <match> <category>` | `--dry-run` preview |
| 8 | Add a YouTube URL to external hours | importer / ds-logger / Insights | `external add-youtube <url>` | resolves via `/inspectExternalVideo`, batch |
| 9 | Video catalog list + filters (level/dialect/guide/subscription/exclude-watched) | DSToolbox Get-DSVideoList/Stats | `videos list` | FTS offline, SQL composable |
| 10 | Video sort (Random/New/Old/Easy/Hard/Short/Long) | app native | `videos list --sort` | offline |
| 11 | Watched/unwatched (playlist) state | hide-watched extensions | `playlist` / `--unwatched` flag | local join |
| 12 | Required daily average to target date or level | DSToolbox Get-DSGoalRequiredAverage | `plan --target <h|level> --by <date>` | back-solve, both axes |
| 13 | Milestone/level ETA projection | HarryPeach expected_milestones | folded into `roadmap` / `forecast` | per-level calendar dates |
| 14 | Best days (top N) | HarryPeach best_days | `stats best-days [--top N]` | offline |
| 15 | Rolling averages (7/30-day) | ds-insights-web | `stats [--window]` | offline |
| 16 | Per-guide / per-tag breakdowns | ds-insights-web / Insights | `stats by-guide`, `stats by-tag` | local joins |
| 17 | Streak (current/longest) | ds-insights-web | `streak` | merged series |
| 18 | Content split (on-platform vs external sources) | ds-insights-web | `stats sources` | offline |
| 19 | Precise (unrounded) hours display | kfound / brianlund precise-time | default precise output everywhere | — |
| 20 | Projected future growth predictions | HarryPeach projected_growth | `forecast` | configurable pace |
| 21 | Account/language migration (es↔fr) | brianlund migrate_dsdf | `migrate --to <lang/account>` (stub — needs second account; see Status) | `--dry-run` |
| 22 | Single video metadata lookup by id | yt-dlp-dreaming | `videos get <id>` | typed JSON (sources, subtitles, duration, guides, tags) |
| 23 | Auth via bearer token / email-password signIn | all tools | `auth login` / `set-token` / `status` / `logout` | env `DREAMING_TOKEN` |
| 24 | Resolve/download a video stream | yt-dlp-dreaming | `download <id>` | prints signed HLS (`sources.bunny`) + YouTube id + ready `ffmpeg` command by default; `--exec`/`--out` downloads when ffmpeg/yt-dlp on PATH. Verify-safe side-effect (print-by-default, honors `PRINTING_PRESS_VERIFY`). No DRM. |
| 25 | Bulk transcript sync (build the corpus) | (new — enables #8) | `transcript sync [--level --guide --all]` | rate-limited bulk `GET /video?id=` over catalog ids, caches cue-level transcripts in SQLite; scoped by default, `--all` for full corpus. |

**Status column:** Only #21 (`migrate`) ships as a `(stub — requires a second Dreaming account/credentials to target; emits honest "needs --to-token" guidance)`. All other rows ship fully.

## Transcendence (only possible with our approach — from novel-features subagent, scores ≥5/10)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Roadmap-aware next-video picker | `dreaming next --level auto --limit 10` | 9 | Joins user(cumulative hours → derived level) × cached catalog − watched playlist, sorted by 1–100 difficulty. Offline. The app filters but never says "next video for where *you* are." |
| 2 | FTS5 offline catalog search | `dreaming search "cocina argentina" --unwatched` | 8 | Full-text over title/guide/dialect/topic/series. The web app has no real search; runs offline. |
| 3 | CSV bulk external-hours import | `dreaming external import backlog.csv` | 9 | CSV → batched `POST /externalTime`. The platform is one-entry-at-a-time with no bulk path — the single most-requested missing workflow. |
| 4 | Difficulty-progression / readiness analysis | `dreaming diet --window 90d` | 7 | Joins watched playlist × per-video 1–100 difficulty × daily series to show if consumed difficulty is trending up. No tool touches the numeric rating of watched content. |
| 5 | Unified fluency-ladder roadmap | `dreaming roadmap` | 6 | Renders L1–L7 with CI advisories (speaking-optional@300, recommended+reading@600, native@1500, Romance-halving) + per-level calendar ETAs in one view. Tools only project a single next milestone. |
| 6 | Raw offline SQL over the local store | `dreaming sql "SELECT ..."` | 5 | Agent-shaped queryable offline mirror of catalog/series. No Dreaming tool exposes the data as a scriptable store. (Framework-provided.) |
| 7 | Caption/transcript fetch (single video) | `dreaming transcript <id>` | 8 | `GET /video?id=` returns the full inline WEBVTT (`video.subtitles`); we clean it to plain text + cache cue-level (with timestamps) in SQLite. No tool reads captions via the API (the one extension does fragile Shaka stream-interception). **(User-requested.)** |
| 8 | Corpus concordance / KWIC search over ALL transcripts (words, phrases, regex, verb-tense presets) | `dreaming concordance "<query>" [--tense imperfect-subjunctive] [--level intermediate] [--guide X] [--context 6]` | 9 | Searches the entire cached transcript corpus and returns each hit IN CONTEXT with video title/level/guide + cue timestamp (jump to the moment). `--tense` presets = es morphology regex (imperfect/preterite/future/conditional/subjunctive-imperfect/present-subjunctive/gerund/compound). Joins transcript_cues × videos so you can scope to comprehensible content at your level. No language tool offers grammar-in-context search over this curated CI corpus. **(User-requested — headline feature.)** |

Caption/transcript source (evidence): `GET /video?id=<24hex>` (bearer) → `video.subtitles` = full WEBVTT string; cleanup = strip `WEBVTT...\n\n` header, `HH:MM:SS.mmm --> HH:MM:SS.mmm` cue ranges, whole-line cue numbers, collapse newlines. Fallback: parse `sources.bunny` HLS master subtitle track. `/video` response fields: `video.{title,description,duration,publishedAt,tags[],guides[],episodeNumber, subtitles, sources:{bunny(HLS m3u8, URL-signed),youtube(id)}}`. Thumbnail synthesized: `https://d36f3pr6g3yfev.cloudfront.net/<id>.jpg`. No DRM.

## NOT absorbed (deliberately out of scope)
- Book/reading tracker (cameron-adrian enhancer) — separate domain (Open Library + Google Sheets), not the Dreaming API. Out of scope.
- Discord nickname sync (DSToolbox) — needs Discord API + token; niche, off-domain. Out of scope.

(VTT/transcript extraction is now ABSORBED + transcended via `transcript` — the API exposes captions inline, so no Shaka interception needed.)

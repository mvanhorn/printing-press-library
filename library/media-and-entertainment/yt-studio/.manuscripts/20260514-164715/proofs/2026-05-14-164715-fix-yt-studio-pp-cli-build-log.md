# yt-studio-pp-cli Build Log

## What was built

### Generator output (Phase 2)
- All 8 quality gates passed: `go mod tidy`, `govulncheck`, `go vet`, `go build`,
  binary build, `--help`, `version`, `doctor`.
- 8 resources spec'd: `channels`, `videos`, `playlists`, `playlistItems`,
  `discover` (renamed from `search` to avoid framework collision), `captions`,
  `categories`, `comments`. Each got a `list`, `get` (where applicable),
  promoted shortcut, MCP endpoint mirror.
- Auth: OAuth2 authorization_code flow with `access_type=offline` for refresh
  tokens. Google Cloud Console KeyURL surfaced in setup.

### Phase 3 hand-written (in `$CLI_WORK_DIR/internal/`)

Packages added:
- **`internal/ytstore/`** — Schema migration (10 typed tables: yt_channels,
  yt_videos, yt_videos_fts, yt_video_metrics_daily, yt_retention_curves,
  yt_demographics, yt_thumbnail_impressions, yt_search_idea_gap,
  yt_script_videos, yt_watchlist, yt_quota_log), upsert helpers, idea-gap
  detection, watchlist management, quota accounting. Includes 3 table-driven
  tests for the heuristic (significantTokens, coveredBy).
- **`internal/ytanalytics/`** — YouTube Analytics API v2 client. Separate
  package because the base URL differs (`youtubeanalytics.googleapis.com`).
  Wraps `reports.query` with typed Report/ColumnHeader/QueryParams,
  RetentionCurve helper that pulls 100-bucket `audienceWatchRatio` by
  `elapsedVideoTimeRatio`, VideoDailyMetrics helper that pulls
  views/watchTime/CTR/thumbnail-impressions in one shot. Typed Error
  (KindAuth/KindRateLimit/KindOther) maps to CLI exit codes. 5 unit tests.
- **`internal/ytstudio/`** — Studio Innertube client backed by a stored
  browser session. `session.go` persists `~/.openclaw/state/yt-studio/studio-session.json`
  mode 600; `client.go` computes the SAPISIDHASH Authorization header
  (`sha1(timestamp + ' ' + SAPISID + ' ' + origin)`), wraps Innertube
  context, classifies HTTP responses (auth/rate-limit/drift), and exposes
  `CheckHealth` for `sniff-doctor`. 3 unit tests.

Novel commands (in `internal/cli/`):
- `login` — wrapper that prints OAuth steps and walks the user through
  Studio cookie capture (paste cookies → session file written). Honors
  `PRINTING_PRESS_VERIFY=1` short-circuit per Phase 3 side-effect contract.
- `retention <video_id>` — reads from store by default; `--fresh` forces
  live Analytics API call. ASCII sparkline + 3 sharpest drops auto-annotated.
- `retention-cohort` — regex pattern match across own-channel videos; averages
  the curves; reports cohort_size, avg_retention_pct, worst_bucket, drops.
- `ctr-decay <video_id>` — store-only join of days 1-3 vs day-30 CTR;
  verdicts: steady, moderate-decay, fast-decay (>20%), rising.
- `vs-watchlist --metric ctr,retention,upload-cadence` — store-only
  aggregations across own + watchlist channels in a sliding period window.
- `title-patterns --winners --losers` — token-level CTR median analysis;
  surfaces lift_pct vs channel median; `--min-n` threshold.
- `idea-gap` — competitor videos NOT covered by own corpus (two-token
  significance heuristic; tunable `--days`).
- `framework-audit <video_id>` — **the killer**. Looks up script via
  binding table OR content-registry.md OR explicit `--script`; extracts
  Signal/Belief-Shift/CTA via heading detection; joins with latest retention
  curve; emits verdict (pass/partial/fail/no-binding/no-script-file) with
  drops and a recommendation. Honors agent contract (`--json`, `--select`).
- `script-link <video_id> <script_path>` — fallback binding command;
  auto-parses framework lines from the script unless `--no-auto-parse`.
- `watchlist` — `list`, `add`, `remove`, `suggest` (live or local; live
  burns 100 quota units logged to yt_quota_log).
- `quota` — today's Data API quota usage with by-endpoint breakdown.
- `sniff-doctor` — Studio session health probe; typed exit 4 on auth
  failure, exit 5 on schema drift.

All commands wired in `internal/cli/root.go`. Help walks, `--dry-run` probes,
`PRINTING_PRESS_VERIFY=1` short-circuit all behaving correctly.

### Tests added
- `internal/cli/yt_helpers_test.go` — 7 tests (sparkline rendering, drop
  detection, tokenization, parsePeriodDays, oneLine).
- `internal/cli/content_registry_test.go` — 5 tests (registry parse,
  framework line extraction, registry file probe, path expand).
- `internal/ytstore/ytstore_test.go` — 3 tests (significantTokens,
  coveredBy with two-token rule, empty-title corner case).
- `internal/ytanalytics/ytanalytics_test.go` — 5 tests (numeric coercion,
  scope enumeration, truncate helper).
- `internal/ytstudio/ytstudio_test.go` — 3 tests (cookie header sort,
  SAPISID fallback, client name default).
- `go test ./...` → all green.

## What was intentionally deferred (per locked scope + design spec)

- **Write operations** — Out of v1. Polish pass adds them per design spec.
- **Live streaming analytics** — Out of v1.
- **Community posts / Shorts analytics** — Out of v1.
- **Monetization data (RPM/CPM/AdSense)** — Out of v1.
- **Comment moderation** — Out of v1.
- **Caption transcripts** — Stubbed via `captions list`; full transcript
  via yt-dlp pattern is polish work.
- **Real-time Studio analytics** — Studio Innertube layer is wired and
  `sniff-doctor` can verify session health, but the v1 hero commands all
  read from the OAuth Analytics API. Studio-only data (real-time 48h,
  thumbnail A/B variants) is reserved for polish.

## Skipped body fields / generator limitations
- None. Spec parsed cleanly after one rename (`search` → `discover` to
  avoid colliding with the press's built-in `search` framework command).

## Phase 3 Completion Gate

Every transcendence row in the absorb manifest resolves to a real Cobra
leaf:

| Manifest row | Resolves to |
|---|---|
| `retention <video_id>` | `yt-studio-pp-cli retention --help` exit 0, leaf path correct |
| `retention-cohort` | leaf correct |
| `ctr-decay <video_id>` | leaf correct |
| `vs-watchlist` | leaf correct |
| `title-patterns` | leaf correct |
| `idea-gap` | leaf correct |
| `framework-audit <video_id>` | leaf correct (killer command verified) |
| `script-link <video_id> <path>` | leaf correct |
| `watchlist suggest/add/remove/list` | parent + 4 children all wired |
| `sync` | inherited from press framework |
| `login` | leaf correct |
| `quota` | leaf correct |
| `sniff-doctor` | leaf correct |

Generator built clean against the spec. The novel-features-built dogfood
sync will run in Phase 4.

## Reachability note
- Data API: 403 unauthenticated (expected); proper OAuth flow gated by user-
  side OAuth client creation. Phase 1.9 gate PASS.
- Analytics API: 401 unauthenticated (expected). Phase 1.9 gate PASS.
- Studio Innertube: depends on user-captured cookies; `sniff-doctor`
  verifies at user runtime. No live testing in this generation run.

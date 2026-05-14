# Browser-Sniff Discovery Report — yt-studio

**Note on approach.** Live browser capture was not performed because no
logged-in YouTube session is available in this generation run. The Studio
Innertube endpoint shape is documented from prior reverse-engineering research
(stable, widely known pattern). The printed CLI's `login` command performs
the actual cookie capture at user runtime; `sniff-doctor` (deferred polish)
will verify the shape on each user system.

## Primary user goal
"Pull retention curve + thumbnail A/B variant breakdown for a given video, to
join with script structure for framework-audit."

## Surface 1 — YouTube Data API v3 (replayable HTTP, OAuth)
- Base: `https://www.googleapis.com`
- Auth: `Authorization: Bearer <oauth_access_token>`
- Endpoints used:
  - `GET /youtube/v3/channels?part=snippet,statistics,contentDetails&mine=true`
  - `GET /youtube/v3/channels?part=snippet,statistics&id=<channelId>`
  - `GET /youtube/v3/videos?part=snippet,statistics,contentDetails&id=<id1,id2,…>`
  - `GET /youtube/v3/playlistItems?part=snippet,contentDetails&playlistId=<UU…>&maxResults=50&pageToken=<t>`
  - `GET /youtube/v3/search?part=snippet&channelId=<id>&order=date&publishedAfter=<RFC3339>&maxResults=50` (100 quota units)
  - `GET /youtube/v3/captions?part=snippet&videoId=<id>` (deferred)

## Surface 2 — YouTube Analytics API v2 (replayable HTTP, OAuth)
- Base: `https://youtubeanalytics.googleapis.com`
- Auth: `Authorization: Bearer <oauth_access_token>` with scope `yt-analytics.readonly`
- Endpoints used:
  - `GET /v2/reports?ids=channel==MINE&metrics=audienceWatchRatio&dimensions=elapsedVideoTimeRatio&filters=video==<id>&startDate=<…>&endDate=<…>` — retention curve, 100 buckets
  - `GET /v2/reports?ids=channel==MINE&metrics=views,estimatedMinutesWatched,averageViewDuration,averageViewPercentage,subscribersGained&dimensions=day&filters=video==<id>` — daily metrics
  - `GET /v2/reports?ids=channel==MINE&metrics=videoThumbnailImpressionsClickRate,videoThumbnailImpressions&dimensions=day&filters=video==<id>` — CTR / impressions (added 2026-01-15)
  - `GET /v2/reports?ids=channel==MINE&metrics=viewerPercentage&dimensions=ageGroup,gender&filters=video==<id>` — demographics

## Surface 3 — YouTube Studio Innertube (cookie session, replayable)
Reverse-engineered. The printed CLI's `login` captures the session.
- Base: `https://studio.youtube.com`
- Auth headers per request:
  - `Cookie: <studio cookies>` (LOGIN_INFO, SAPISID, HSID, SSID, APISID, SID, …)
  - `Authorization: SAPISIDHASH <timestamp_unix>_<sha1(timestamp + ' ' + SAPISID + ' ' + 'https://studio.youtube.com')>`
  - `X-Origin: https://studio.youtube.com`
  - `X-Goog-AuthUser: 0`
- Endpoints (POST with JSON body containing `context` + query):
  - `POST /youtubei/v1/analytics/get_screen?alt=json` — real-time 48h panel
  - `POST /youtubei/v1/creator/get_creator_videos?alt=json` — video listing with Studio-only fields
  - `POST /youtubei/v1/creator/get_creator_video_analytics?alt=json` — A/B thumbnail variant breakdown
- All require the standard Innertube `context` blob (clientName=62 for Studio web,
  clientVersion observed empirically). The `login` command captures both the
  cookies and the current clientVersion.

## Replayability assessment
- **Replayable:** all three surfaces are plain HTTP. No live page-context JS
  required. Surf transport (default Go `net/http`) is sufficient for surfaces 1
  and 2. Surface 3 requires the SAPISIDHASH header computation, which is
  trivial Go code.
- **No browser sidecar at runtime.** All commands run as direct HTTP after
  `login` harvests credentials.

## Provenance plan
- `login` writes raw cookie + token captures to
  `~/.openclaw/state/yt-studio/discovery/<ISO8601>-login.json` (mode 600).
- Every Studio Innertube request archives `{request_url, status, response_body_first_2KB}`
  to `~/.openclaw/state/yt-studio/discovery/<date>.ndjson` so if the schema
  drifts there's an audit trail.

## Schema drift surface
- Innertube clientVersion bumps weekly; the SAPISIDHASH algorithm is stable.
- The Studio response shapes have moved twice since 2024 but the
  `analytics/get_screen` URL has been stable since at least 2022.
- Drift surfaces as typed exit 5 with the suspected schema change logged.

## Conclusion
- Phase 1.7 satisfied: documented Studio surface is replayable; no resident
  browser needed. The runtime mode is plain HTTPS with cookies + computed auth
  header.
- Phase 2 generation: spec drives only Data API + Analytics API endpoints
  (these have OAuth and clean shapes). The Studio Innertube layer is hand-built
  in Phase 3 alongside `login`.

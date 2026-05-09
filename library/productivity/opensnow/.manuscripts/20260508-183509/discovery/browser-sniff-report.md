# OpenSnow API Browser-Sniff Report

## Summary
Successfully accessed API documentation at blizzard.opensnow.com/opensnow-api/index.html
using browser-use CLI mode. The docs page requires a real browser (returns 403 to direct HTTP).

## Endpoints Discovered
1. `GET /forecast/{point}` — Forecast for any lng,lat (level 5)
2. `GET /forecast/detail/{id_or_slug}` — Named location forecast (level 10)
3. `GET /forecast/snow-detail/{id_or_slug}` — Day+night snow forecast (level 10)
4. `GET /snow-report/{id_or_slug}` — Resort snow report (level 10)
5. `GET /daily-reads/{id_or_slug}/content` — Daily Snow posts (level 10)

## Authentication
- API key via query parameter `api_key`
- Partnership-only access
- Access levels: 5 (forecast by point) and 10 (all named-location endpoints)
- Optional `X-OpenSnow-Token` header for Daily Snow (user token from /user/login)

## Response Schemas
All endpoints return rich JSON with full field documentation captured.
Key response structures: ForecastPeriod, SemiDailyPeriod, SnowReport, DailyRead.
Conditions enum: -1 to 17 with labels (Snow, Rain, Sunny, etc.)

## Replayability
All endpoints are standard REST GET requests with query-string auth.
Fully replayable via direct HTTP — no browser runtime needed for the CLI.

## Spec Output
Written to: research/opensnow-spec.yaml (581 lines, OpenAPI 3.0.3)

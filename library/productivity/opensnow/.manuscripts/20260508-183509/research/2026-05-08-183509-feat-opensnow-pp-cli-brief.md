# OpenSnow CLI Brief

## API Identity
- Domain: Snow/ski weather forecasting and resort conditions
- Users: Skiers, snowboarders, backcountry enthusiasts, resort operators, trip planners
- Data profile: Time-series forecasts, point-in-time snow reports, editorial content (Daily Snow posts), resort operational status
- Base URL: api.opensnow.com
- Auth: API key via query string (`api_key`), partnership-only access (email enterprise@opensnow.com)
- Access levels: 5 (basic forecasts, 6hr hourly) and 10 (detailed snow, 24hr hourly, daily reads)

## Reachability Risk
- Low — No GitHub issues about blocked access. API is partnership-gated (key required) but functional for authorized users. The website itself blocks direct HTTP (403 on all pages), but the API endpoints at api.opensnow.com are separate infrastructure.

## Known Endpoints (from research)
1. `GET /forecast/{location}` — High-level forecast summary for any point on earth. Current conditions, next 5 days, configurable hourly chunks (6h at level 5, 24h at level 10)
2. `GET /forecast/snow-detail/{location}` — 5-day snow forecast with detailed data at named OpenSnow locations (level 10)
3. `GET /snow-report/{resort}` — Most recent snow report as reported by the resort
4. `GET /daily-reads/{location}/content` — Written daily snow post with full content for any publicly available Daily Snow

## Top Workflows
1. **Morning powder check** — Check overnight snowfall at favorite resorts, compare across mountains, decide where to ski today
2. **Trip planning** — Compare 5-day forecasts across multiple resorts to pick the best destination for a multi-day trip
3. **Storm tracking** — Monitor incoming weather systems, track snowfall accumulation predictions over time
4. **Daily Snow reading** — Read expert-written daily forecasts for specific regions (the editorial layer that differentiates OpenSnow)
5. **Resort status check** — Is the resort open? How many lifts/runs? What's the base depth?

## Table Stakes (from competitor analysis)
- Resort snow depth (base, mid, summit) — Weather Company, OnTheSnow
- Snowfall totals (24h, 48h, 72h) — Weather Company, OnTheSnow
- Lift count and status — OnTheSnow, SkiAPI
- Trail/run counts and status — OnTheSnow
- Surface conditions — Weather Company, OnTheSnow
- Operating status — Weather Company, OnTheSnow
- Nordic/terrain park data — Weather Company, OnTheSnow
- Multi-day forecasts — Weather Unlocked (7-day), OpenSnow (5-day+)
- Geographic lookup — Weather Company (geocode, IATA, ICAO, postal)

## Data Layer
- Primary entities: Locations (resorts, summits, custom points), Forecasts (hourly, daily, 5-day), Snow Reports, Daily Snow Posts
- Sync cursor: Forecasts update continuously throughout the day; snow reports update daily; Daily Snow posts update daily
- FTS/search: Location names, Daily Snow content, resort names

## Product Thesis
- Name: OpenSnow CLI — The powder hunter's terminal companion
- Why it should exist: No CLI exists for OpenSnow. Power users (trip planners, ski journalists, resort aggregators, alert builders) need programmatic access to forecasts and snow data without navigating the website or app. The Daily Snow editorial content is unique — no other service has expert-written regional forecasts. Combining forecast data with local SQLite storage enables historical tracking, cross-resort comparison, and powder day scoring that the app can't do.

## OpenSnow Differentiators (vs competitors)
- PEAKS AI model: ML-enhanced forecasting up to 50% more accurate in mountain terrain
- Daily Snow: Expert-written regional forecasts (editorial, not just data)
- Powder Quality forecast: Optimized snow quality predictions
- Avalanche danger integration
- 15-day extended forecasts
- Custom location support (not just resorts)

## Competitor Landscape
1. **OnTheSnow API** — Detailed resort snow reports (lifts, trails, surface conditions). Well-documented. Requires x-api-key header.
2. **Weather Company Snow API** — Comprehensive ski conditions by geocode/IATA/ICAO. 35+ response fields. Requires apiKey query param.
3. **Weather Unlocked Ski API** — Mountain forecasts at 3 elevations, 3000+ resorts, 7-day forecast. Updated 4x daily.
4. **SkiAPI** — Basic lift status and conditions via RapidAPI.
5. **jwise/snow** — Python scraper for Vail resort snow reports (3 resorts only).
6. **meccaLeccaHi/snow_scraper** — Python scraper that pulls OpenSnow data for visualization ("FreshyFinder").
7. **Synoptic Data API** — Raw weather station data (50,000+ stations) that OpenSnow uses internally.

## Build Priorities
1. Forecast retrieval (summary + detailed snow) with local caching
2. Snow report fetch and comparison across multiple resorts
3. Daily Snow content retrieval and offline reading
4. Favorites management with local SQLite storage
5. Historical tracking and cross-resort powder comparison

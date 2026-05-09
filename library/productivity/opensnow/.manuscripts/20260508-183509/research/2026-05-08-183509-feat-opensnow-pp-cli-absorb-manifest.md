# OpenSnow CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Current conditions at any point | OpenSnow API /forecast/:point | `forecast <lng,lat> --elev <ft>` | Works offline after sync, --json, --compact |
| 2 | 5-day daily forecast | OpenSnow API /forecast/detail | `forecast detail <slug>` | Local SQLite cache, historical comparison |
| 3 | Hourly forecast (24h) | OpenSnow API /forecast/detail | `forecast detail <slug> --hourly` | Tabular output, pipe to jq |
| 4 | Day+night snow forecast | OpenSnow API /forecast/snow-detail | `snow-detail <slug>` | Day vs night split in tables, --json |
| 5 | Snow summary (5-day total) | OpenSnow API /forecast/snow-detail | `snow-detail <slug> --summary` | Range display (min/expected/max) |
| 6 | Resort snow report | OpenSnow API /snow-report | `report <slug>` | Offline cache, historical tracking |
| 7 | Resort operating status | OpenSnow + OnTheSnow | `report <slug> --status` | Status enum labels, --json |
| 8 | Lifts open/total | OpenSnow + OnTheSnow + Weather Co | `report <slug>` | Part of report, with percentage |
| 9 | Runs open/total | OpenSnow + OnTheSnow + Weather Co | `report <slug>` | Part of report, with percentage |
| 10 | Terrain open/total | OpenSnow + Weather Co | `report <slug>` | Acres/hectares, percentage |
| 11 | Snow depth (base) | OpenSnow + OnTheSnow + Weather Co | `report <slug>` | Min/max range when available |
| 12 | New snow (24h/72h/5d/season) | OpenSnow + OnTheSnow + Weather Co | `report <slug>` | All windows in one view |
| 13 | Surface conditions | OpenSnow + OnTheSnow + Weather Co | `report <slug>` | Packed Powder, Machine Groomed, etc |
| 14 | Daily Snow posts | OpenSnow API /daily-reads | `daily-snow <region>` | Full HTML→text rendering, author info |
| 15 | Wind speed and direction | OpenSnow + Weather Unlocked | `forecast detail <slug>` | Cardinal labels, gust speed |
| 16 | Precipitation probability | OpenSnow API | `forecast detail <slug>` | Pop as percentage, precip type labels |
| 17 | Rollup forecasts | OpenSnow API /forecast/:point | `forecast <lng,lat> --rollup 6` | 3/4/6/12 hour rollups |
| 18 | Multi-resort comparison | snow_scraper (FreshyFinder) | `compare <slug1> <slug2> ...` | Side-by-side table, --json |
| 19 | Imperial/metric units | OpenSnow + Weather Co | `--units metric` flag on all commands | Global config or per-command |
| 20 | Weather conditions icons/labels | OpenSnow conditions enum | `forecast detail <slug>` | Readable labels from enum |
| 21 | Resort contact info | Weather Company API | `report <slug> --info` | Website, phone, email |
| 22 | Season dates | Weather Company API | `report <slug>` | Projected open/close dates |

### Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Powder day scorer | `powder-score <slug>` | Combines snow forecast (expected/max), wind, temp, and historical averages in SQLite to rate upcoming days 1-10 for powder quality |
| 2 | Storm tracker | `storm-track <region>` | Correlates rolling hourly forecasts over time to show storm progression — when it starts, peaks, and ends. Only works with cached forecast history |
| 3 | Multi-resort powder ranker | `powder-rank [--region CO,UT,CA]` | Queries all synced locations, scores each by expected snowfall × base depth × percent open, ranks best options for the next 5 days |
| 4 | Historical snowfall trends | `history <slug> [--days 30]` | SQLite accumulates snow report snapshots over time. Shows snowfall trends, season totals vs averages, base depth progression |
| 5 | Favorites dashboard | `dashboard` | One-command view of all favorited locations: current temp, 24h snow, 5-day total, status. Like the OpenSnow app favorites screen but in terminal |
| 6 | Daily Snow digest | `digest [--region all]` | Pulls all Daily Snow posts for favorited regions, strips HTML to clean text, shows summary + full content. Newsletter in your terminal |
| 7 | Overnight snowfall alert check | `overnight <slug>` | Checks the semi-daily forecast for the overnight period (6pm-6am) at your favorites. "Alta: 6-10 inches tonight. Steamboat: 2-4 inches tonight." |
| 8 | Conditions diff | `diff <slug>` | Compares current snow report against the last-synced version. Shows what changed: new snow, lifts opened/closed, status change |
| 9 | Forecast accuracy tracker | `accuracy <slug>` | Compares past forecasts against actual snow reports. "Tuesday forecast said 4-8in, actual was 6in — 75% accuracy" |
| 10 | Season snapshot | `season <slug>` | Full season overview: total snowfall, biggest storm, average base, days open, best powder day. Requires season of cached data |

## Source Tools Analyzed
1. **OpenSnow API** — 5 endpoints, full response schemas captured via browser-sniff
2. **OnTheSnow Partner API** — Resort snow reports with detailed lift/trail/surface data
3. **Weather Company Snow API** — Comprehensive ski conditions with 35+ fields
4. **Weather Unlocked Ski API** — Mountain forecasts at 3 elevations, 3000+ resorts
5. **SkiAPI** — Basic lift status via RapidAPI
6. **jwise/snow** — Python scraper for Vail resort snow reports (3 resorts)
7. **meccaLeccaHi/snow_scraper** — Python scraper pulling OpenSnow data for visualization

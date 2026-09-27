# Source API contract

`NewClient(Config) *Client`, `(*Client).Metrics() Metrics`.

All operations take `context.Context`, return a typed result and `error`:

```
Search(ctx, query, kind string, limit, maxPages int) (SearchResult, error)
Resolve(ctx, target string) (PlaceResult, error)
Daily(ctx, target string) (ForecastResult, error)
Hourly(ctx, target string) (ForecastResult, error)
SeasonalList(ctx, kind, query string, year, limit, maxPages int) (SeasonalListResult, error)
Seasonal(ctx, kind, target string, year int) (SeasonalResult, error)
Mountain(ctx, target string) (MountainResult, error)
```

Kinds: municipality, leisure, mountain; seasonal kinds sakura/kouyou (foliage is accepted as a kouyou alias). Canonical tenki.jp URLs are stable identifiers. Search names must resolve uniquely before weather calls; ambiguity is an error with candidate URLs. Arrays are empty arrays, never null. Status values include ok, no_results, out_of_season, season_ended, year_unavailable, unavailable. Numeric absence is null.

Times are RFC3339 with JST offset and dates YYYY-MM-DD. Hourly Period precipitation describes Start–End (preceding hour); ValidAt is End for temperature and wind. At label 24, Date remains the preceding calendar date while End/ValidAt are the following midnight. Hourly Kind is forecast or estimated_actual. Daily Periods expose only daily fields; wind stays null. Daily Intervals holds six-hour precipitation/weather and Instants holds temperature/humidity/wind, kept separate because 4 intervals coexist with 5 instant labels. Daily day zero is Partial and Kind=mixed, with calendar-day Start/End. WeatherProbabilityFrom identifies the issue-time beginning for today's symbol/probability only; WeatherProbabilityKind=forecast. TemperatureKind=forecast_or_estimated_actual for today's unmarked min/max, or estimated_actual if explicitly marked. No unverified cutoff hours are inferred. Future daily temperatures are forecasts. Daily precipitation amount remains the source daily summary with no post-issue reinterpretation. CoverageStart may begin at source issue time for the remaining outlook, separately from the daily summary's calendar bounds.

Each result carries the destination Place and Source. Place.ForecastReferenceURL/Name identify municipality; Scope municipal/foothill/nearby_model is distinct from ElevationM. Mountain Levels are explicitly nearby_model_guidance with separate ModelInitialAt/ModelSource; SummitForecastAvailable is false.

Source.SourceStale uses explicit CLI age policy: weather publication 2h, seasonal publication 36h, mountain initialization 12h. Raw cache TTL weather/season/model 1h, catalog 7d. AllowStale explicitly permits stale cache on failure/local selection. Source.Freshness is fresh/stale/unknown; unknown source time cannot establish freshness. FreshnessReference/FreshnessAt identify the age basis. Mountain initialization uses model_initial_at and never becomes publication issue_at. ModelInitialRaw preserves source initialization text. Seasonal Spot.ReportDate holds day-only reports; ReportAt is populated only for an explicitly timestamped spot report. Neither inherits the product publication hour. Source publication IssueAt and Spot report evidence remain separate.

Metrics count actual HTTP attempts including redirects, cache hits and bytes of decoded response bodies consumed by the reader, not network transfer bytes. Request pacing is capped at 1 request/second per Client, including redirect hops. Each CLI invocation or MCP command call uses a separate provider client; concurrent clients/invocations do not coordinate and may exceed that rate in aggregate. Config.RateLimit/`--rate-limit` only lowers the per-client ceiling. Callers and agents should serialize source calls; there is no shared or global rate coordinator. Missing fields remain null and unavailable/denied/changed markup fails explicitly.

Config.SearchDirectory selects an explicit readable leisure or seasonal index/region/prefecture directory. Leisure keyword acquisition is currently unavailable, so Search filters only linked destinations within the selected directory (default leisure root). SearchScope, DirectoryURL and Scanned identify this limited scope; no_results never establishes nationwide absence. Kouyou query discovery follows the verified form with search_type=venue and keyword. Ended Sakura discovery scans retained directory links rather than inventing a search endpoint. SeasonalList.Scanned/Truncated/Pages describe bounded coverage; source year, requested year, year relation and update state stay separate.

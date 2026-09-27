# tenki-cli: semantics, scope and architecture research

Research reviewed 2026-09-27 at approximately 23:19 JST. Research only; no implementation is authorized before the user scope gate. Official tenki.jp pages and tenki.jp's linked official help service are the sources below. Examples are research snapshots, not reusable current-weather fixtures. Recommendations are design decisions, distinguished from provider facts.

## Decision summary

Build an agent-facing, tenki.jp-only sightseeing comparison tool around explicit place identity, forecast coverage, source times and user criteria. Weather suitability is conditional on the requested activity window and available evidence; a pleasant daily symbol is insufficient evidence for an hourly excursion, mountain summit, blossom date or trail condition. Start with municipal daily/hourly forecasts and bounded place comparisons. Mountain and seasonal products require distinct schemas before inclusion.

One material acquisition issue must be resolved at the scope gate: current tenki.jp terms, Article 8(6), prohibit obtaining/using website/app information through tools other than browsers or RSS readers. Article 8(7) addresses use beyond private scope without approval. HTTP 200 and an undocumented page do not establish authorization for a Go HTTP scraper. Root owns acquisition discovery; do not represent browser automation or an internal endpoint as an approved workaround. The rest of this brief describes semantics independently of that decision. [Current terms](https://tenki.jp/docs/rule/)

## Verified product semantics

| Product | Official meaning and observed coverage | Consequence |
|---|---|---|
| Municipal / leisure location | Municipal forecasts refer to the vicinity of the municipality's government office. Leisure forecasts use the municipality containing the facility. | Keep attraction identity separate from forecast reference place; do not label this exact-coordinate weather. [Location FAQ](https://tenkijp.tayori.com/q/tenkijp-faq/detail/1137776/) |
| Municipal 1-hour | Today, tomorrow and the day after. Weather, precipitation probability and precipitation refer to the preceding hour; temperature, humidity and wind refer to the displayed instant. | Store period boundaries separately from instantaneous valid times. [Web forecast guide](https://tenkijp.tayori.com/q/tenkijp-faq/detail/1138228/) |
| Daily / 2-week | Six-hour detail through ten days ahead; days 12–14 have daily weather, temperatures, precipitation probability and confidence. Six-hour temperature/humidity/wind are at the interval start. Daily minimum means morning low; maximum means daytime high. Today's weather/probability cover the period after issuance. | Do not interpret daily lows as midnight-to-midnight extrema, daily probability as sightseeing-window probability, or one value as both instantaneous and interval-based. [Web forecast guide](https://tenkijp.tayori.com/q/tenkijp-faq/detail/1138228/) |
| Late outlook confidence | A–E on days 12–14 is provider forecast reliability; A indicates greater reliability and less likelihood of change. | Preserve letters; do not invent percentages or fill absent confidence with A. [Confidence FAQ](https://tenkijp.tayori.com/q/tenkijp-faq/detail/1138224/) |
| Elapsed hours | Grey/monochrome hours are estimates derived from nearby observations, not observations at that municipality. | Mark `estimated_actual`; do not present elapsed entries as future forecasts or station observations. [Past-hour FAQ](https://tenkijp.tayori.com/q/tenkijp-faq/detail/1138202/) |
| Mountain leisure weather | Mountain-region facility pages display foothill representative weather. The mountain index offers foothill forecasts and nearby numerical results. | `scope=mountain_foot`; summit availability must remain false unless separately verified. [Mountain-location FAQ](https://tenkijp.tayori.com/q/tenkijp-faq/detail/1137777/), [Mountain index](https://tenki.jp/mountain/) |
| Sakura | Major-point flowering/full-bloom forecasts, differences from normal/prior year, and spot information are different content types. Full-bloom forecasts are unavailable during some periods. | Missing forecast dates remain null. City flowering must not become a park's bloom observation. [Sakura guide](https://tenkijp.tayori.com/q/tenkijp-faq/detail/1165093/) |
| Foliage | Status comes from reports at spots. Details may contain current status, typical/current-year viewing windows, species, access and municipal weather. | Separate current report, forecast and typical season; retain species/elevation limitations. [Foliage guide](https://tenkijp.tayori.com/q/tenkijp-faq/detail/1165097/) |

### Actual-page checks

- Chiyoda hourly displayed 2026-09-27, 28 and 29, each with 01–24 hour labels. Units shown were °C, %, mm/h and m/s. Earlier hours had missing precipitation probabilities. Thus the future horizon is the remainder of today plus two full calendar days, not a rolling guaranteed 72 hours. At 24:00, the instant is next-day midnight while the precipitation interval still belongs to the preceding hour. [Chiyoda hourly](https://tenki.jp/forecast/3/16/4410/13101/1hour.html)
- Chiyoda's legacy `/10days.html` route displayed September 27–October 10 inclusive: fourteen dates. Six-hour detail extended to October 7 (today + ten days); October 8–10 were daily only, with confidence D, D and E in this snapshot. Route names do not establish horizons. Determine coverage from actual dates and available fields. The current layout includes five instant labels (00/06/12/18/24) alongside four precipitation intervals; do not zip them by index into identical-length records. [Chiyoda two-week page](https://tenki.jp/forecast/3/16/4410/13101/10days.html)
- Fuji's main weather explicitly named Fujinomiya and warned that it was foothill weather. A separate table used a September 27 15:00 **initial** time, eight heights from 300 to 4,400 m and 09:00/15:00 values through September 30. Its note explicitly says these are numerical calculation results rather than weather forecasts. Keep `nearby_model_guidance`, `model_initial_at`, actual height and valid instant. Do not interpolate these into a summit forecast or substitute them for summit weather. [Fuji detail](https://tenki.jp/mountain/famous100/5/25/150.html)
- Sakura's index and Ueno detail explicitly said the 2026 season's updates had ended and 2027 information could change. Ueno retained a typical late-March/early-April window while its weather section continued to update in September. A recent weather issue time does not refresh seasonal facts. [Sakura index](https://tenki.jp/sakura/), [Ueno detail](https://tenki.jp/sakura/3/16/54401.html)
- Foliage's current index was labeled 2026, issued September 27 at 15:00, with reported colour stages. Oze detail separately showed a September 27 status report and 22:00 weather issuance, with a typical late-September–mid-October window. Its photograph was explicitly a peak-season illustration, not current evidence. [Foliage index](https://tenki.jp/kouyou/), [Oze detail](https://tenki.jp/kouyou/3/13/30139.html)

### Update schedules and geographic limits

Official help lists daily, time-series and two-week weather updates every hour; indices every even hour; ordinary AMeDAS measurements every ten minutes and snow depth hourly; previous-day weather around 01:00. These are product schedules, not a guarantee that a fetch has the newest issue. No specific seasonal-report or mountain-model refresh cadence was verified. [Update FAQ, revised 2026-08-14](https://tenkijp.tayori.com/q/tenkijp-faq/detail/1083867/)

The two-week index links regional coverage from Hokkaido through Okinawa, with Hokkaido subregions and prefectural navigation. This establishes national navigation, not an audited count of municipalities or completeness of every leisure category. [Two-week directory](https://tenki.jp/week/)

Use provider directory IDs and canonical URLs, not an invented national geocoder. Return prefecture, municipality and category with search candidates. Ambiguous names require an explicit ID; preserve aliases but never silently select the first fuzzy match. Directory completeness, kana/Latin-name search behaviour, remote-island coverage and unavailable-page behaviour remain discovery items. These are recommendations, not provider guarantees.

## Proposed data contract

Use compact JSON by default, stable names, `--fields` projection and `--detail` for per-period evidence. Summary mode should reduce records and repeated text, not omit the selected period, reference location, source URL, missing-data notices or freshness needed to interpret a result.

Every product result should include:

- `schema_version`, `provider: "tenki.jp"`, product identifier and canonical `source_url`.
- `place`: stable provider ID, displayed Japanese name, prefecture, municipality, category; separate requested attraction and `forecast_reference` with scope. Coordinates/elevation are nullable and must be source-backed.
- `issued_at`: RFC3339 timestamp with `+09:00`, or null. `issued_at_raw` preserves the displayed label. Never replace an unknown issue time with fetch time. Separate `model_initial_at` from publication time; do not conflate initialisation with issuance.
- `fetched_at`: actual retrieval timestamp in JST; `timezone: "Asia/Tokyo"`; `valid_from`/`valid_to` or `valid_at` for each relevant element, plus actual product `coverage`. Date-only season reports keep date precision rather than fabricated midnight timestamps.
- `freshness`: `fresh|stale|unknown`, source age where calculable, cache age/hit, checked-at timestamp and policy/threshold. Distinguish revalidation time from original payload retrieval; local cache reuse must not reset retrieval time.
- Explicit units: `temperature_c`, `humidity_pct`, `precip_probability_pct`, `precip_rate_mm_h` for the hourly display, `precip_total_mm` for declared multi-hour totals, `wind_speed_m_s`, source wind direction text, and altitude metres. Preserve the source label and applicable period in detailed output.
- `kind`: `forecast|estimated_actual|observation|model_guidance|seasonal_report|seasonal_prediction|typical_period`; field-level overrides where a record contains mixed kinds. Absent or unavailable values are JSON null, accompanied by `missing_reason` when known. Zero is retained only when the provider actually supplies zero.
- Provider weather wording and confidence letter preserved verbatim as values, with normalized enums only alongside raw wording. A confidence letter is not a probability of a dry day. Preserve unknown source labels instead of dropping the record.
- Seasonal data: `season_year`, `requested_year`, `update_state` (`active|ended|not_started|unknown`), `reported_on`, predicted dates, typical-period text and species where available. `year_relation` explicitly distinguishes current from previous-season content. Do not silently use retained 2026 information for 2027; explicit historic opt-in still labels it historical.
- `warnings`, `partial`, and product-specific `availability` so one unavailable component does not become a zero-value success. Report evidence separately from the CLI's derived suitability decision.

For displayed day/time stamps lacking year and month, resolve them only against a verified page calendar/issue context in JST. Validate midnight/month/year rollover, conflicting labels and impossible future issue dates. Leave timestamps null with raw text and a parsing notice when resolution is ambiguous.

## Proposed boundaries and comparison rules

Separate four responsibilities: provider catalog/resolution; bounded acquisition and product-specific parsing; semantic normalization/provenance; deterministic selection/comparison. CLI rendering and field projection consume normalized records and must not silently repair source data. Keep mountain, seasonal and weather issue contexts separate even on the same HTML page.

Use explicit criteria, never an unexplained universal sightseeing score. A comparison result should echo the place set, JST dates/time window, thresholds, source periods used, coverage ratio, missing criteria, provider confidence and individual reasons. Proposed criteria include maximum hourly rain rate, maximum interval rain probability, temperature range and maximum wind speed. User thresholds describe preferences, not provider-certified safety boundaries.

Evaluate compatible source periods only. For a 09:00–17:00 hourly request, aggregate the eight complete precipitation hours ending 10:00–17:00. Define endpoint inclusion for instantaneous temperature/wind separately. A six-hour period overlapping an activity window cannot supply a precise hourly decision; expose the coarser evidence and partial coverage. Never sum precipitation probabilities; their maximum can be reported as maximum interval probability, never as the probability of any rain during the whole trip.

Use `meets_criteria`, `fails_criteria` or `insufficient_data` with missing/stale data unable to produce a positive result. Default ordering may use an explicitly documented sequence (complete evidence first, requested criteria outcomes, then user-selected tie-breakers); show ties honestly. Avoid cross-product averages and invented rain/cloud/summit visibility. Out-of-horizon dates return availability plus actual coverage instead of synthetic hourly rows. Provider outing indices can be shown as supporting source values, distinct from the CLI's criteria result.

## Proposed bounded workflows for the scope gate

These are candidate user workflows, not approved commands. Initial suggested cap: ten candidate places and fourteen dates per operation, explicit limits/pagination, no national recursive crawl.

1. **Resolve an outing place:** search an explicit prefecture/category/name, return at most ten provider candidates and their forecast reference municipality. Select by ID; expose missing exact-place forecasts.
2. **Choose a day:** compare a selected municipality across up to fourteen dates. Show daily outlook and confidence first; add 09:00–17:00 detail only for dates with the necessary coverage. Return why each date meets/fails criteria or lacks evidence.
3. **Choose a destination:** compare two to ten explicit places for a fixed JST date/window with the same rain/temperature/wind criteria. Deduplicate a shared municipal forecast while preserving separate attraction identities.
4. **Check a mountain outing:** show the named foothill forecast and separately timestamped altitude model guidance for an explicit mountain. Return summit weather unavailable when that is what the verified public product supports; no ascent-safety or summit recommendation from foothill values.
5. **Choose a seasonal outing:** list a bounded region's sakura/foliage candidates with source year, report date and status; join weather only inside its actual horizon. A current foliage report, historical flowering date and typical-season range remain distinct. Future-year availability is an explicit outcome.

## Cache, error and acceptance recommendations

Acquisition design remains conditional on the scope-gate decision. If approved, use on-demand requests, canonical-URL deduplication, a small request/concurrency budget, context cancellation and timeouts. No background polling or hidden whole-country refresh. Conditional revalidation is useful only if the server actually advertises validators. Do not copy browser credentials or circumvent denials/challenges.

Suggested initial cache policy, explicitly a CLI choice: reuse ordinary forecasts for up to 30 minutes; mark hourly products stale once their source issue age exceeds two hours. Unknown issuance stays unknown. Fail a fresh comparison rather than silently use stale values; allow clearly labeled stale output only by explicit flag. Cache catalog metadata longer (for example seven days), invalidating on missing IDs or structure mismatch. Seasonal status needs its own age rule plus year/update-state checks; ended seasons stay ended even after successful re-fetch. Establish mountain model and seasonal cadence policies only after evidence or an explicit conservative policy choice.

High-value acceptance cases:

- Chiyoda three-calendar-day hourly and fourteen-date daily coverage; hourly 24:00 normalisation and JST month/year rollover.
- Five six-hour instant labels versus four precipitation intervals; per-element time semantics remain aligned.
- Today's past estimates and missing probabilities; missing confidence and all missing numeric values remain null.
- Daily morning low versus a colder night; no false assertion that the daily low is the absolute daily minimum.
- Two products with different issuance times; no shared timestamp laundering or silent mixing.
- Fuji reference municipality plus separate model initialisation, heights and model-guidance label; no foot-as-summit output.
- Ended Sakura 2026 plus fresh September weather; future-year queries cannot produce current-season success.
- Foliage report-date/issue-time/weather-time separation; typical period is not predicted exact dates.
- Ambiguous place names, unavailable/out-of-range product, changed markup, partial comparisons, stale cache and source denial produce structured, actionable outcomes.

## Remaining uncertainties before implementation

1. Authorized acquisition mode and intended distribution/use must be addressed at the user scope gate; no official public forecast API contract was established by this research.
2. Root must validate raw current markup, including dynamic six-hour temperatures and metadata timestamps; web extraction is adequate semantic evidence but not parser evidence.
3. Do not assume the 3-hour page has three-hour precipitation totals: the current official guide's example describes the preceding hour even in its three-hour section. Prefer the verified 1-hour product initially, or resolve this with raw labels before adding 3-hour aggregation.
4. Exact directory completeness, redirects, search semantics, coverage of islands, and stable provider ID formats need bounded discovery.
5. Seasonal current-year spot predictions are optional, not guaranteed; complete historical-year endpoints and 2027 products were not verified. Neither a current title nor a successful response proves active updates.
6. Refresh cadence for mountain model guidance and seasonal reports, issue-stamp timezone declaration, and all error/empty-page variants remain unverified. JST output is the user's required normalization; preserve raw evidence and avoid invented precision.

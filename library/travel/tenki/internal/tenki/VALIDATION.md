# Provider validation

Final fixture/transport command:

```
GOCACHE=<workspace>/.press/go-build go test ./internal/tenki -v
```

PASS: 19 top-level tests and 7 named subtests, 1.100 seconds. The local HTTP server test required socket permission. All assertions inspect typed content or transport/cache behavior rather than only process exit status.

- Daily source snapshot has 14 actual dates (2026-09-27 through 2026-10-10), D/D/E confidence on its final three dates, null late rain amount and null daily wind. Four six-hour intervals coexist with five separate instants per detailed date. Day zero is mixed/partial, calendar bounded; today's weather/probability start at issuance, while temperature provenance remains forecast_or_estimated_actual without an invented provider cutoff.
- Hourly source snapshot has 72 source rows over three dates; elapsed grey entries are estimated_actual, missing probability is null, and hour24 precipitation ends at the next midnight. Constructed calendar variants assert month/year rollover and negative/null temperatures.
- Fuji retains 3,776m destination elevation and its named 富士宮市 foothill reference. Its 64 actual model values span eight levels from300m to4,400m. Missing values remain null. Initialization15:00 is distinct from unknown publication issue; model guidance never becomes summit forecast.
- Active 2026 foliage report publication15:00 stays separate from report_date2026-09-27 and weather23:00. The old2025 sidebar does not change source year. Typical viewing period and four species are retained separately from predictions. Ended2026 Ueno Sakura remains ended despite continuing weather; requested2027 returns year_unavailable. Explicitly constructed active Sakura tests optional predictions and out_of_season status.
- Broad 京都 search retains Tokyo substring ambiguity and canonical deduplication. Exact raw 京都市 search retains 左京区/下京区 identity, original address/postcode, provenance, and bounded/truncated coverage. Search does not assert an exact forecast reference name before resolution. Negative municipality, leisure and seasonal queries never return unrelated navigation items; empty results encode as[] rather than null.
- Actual 金閣寺 page resolves 鹿苑寺 金閣寺 to 京都市北区 via the linked municipal URL. A request-routing test retains destination identity while fetching that reference's daily product, with no redundant per-run destination request.
- Cache tests cover cold/warm metrics, disk reuse, refresh bypass and per-run deduplication, explicit stale selection/fallback, refusal of silent stale fallback, local no-network behavior, future-dated cache rejection and near-limit escaped cache content. HTTP tests cover403/404 denial, typed429, bounded bodies, same-host redirect acceptance, other-host/path denial, per-hop attempt count, rate ceiling and a real local test server.
- Source age policy boundary assertions cover weather2h, seasonal36h and mountain initialization12h; unknown timestamps remain unknown.

`go build ./...` and `go vet ./...` both passed after interface integration. Root owns the final wider verification after subsequent semantic changes and the live workflow run. `workflow_verify.yaml` contains the concrete search→daily→criteria comparison chain, using live mode, no auth and cache under its verification checkout.

Fourteen unmodified raw source fixtures and URL/acquisition metadata are under testdata. Constructed variants are identified in test names/comments. The leisure municipal forecast request test uses the Chiyoda layout solely for routing, not as claimed Kyoto weather.

Remaining source limitations: public HTML is not an official API; acquisition terms limitation was disclosed in approved scope. Leisure keyword acquisition returns403, so readable selected directories and direct canonical URLs are used without a nationwide completeness claim. Ended Sakura has no verified active search endpoint; retained directories and canonical details are supported. Seasonal/model refresh cadence remains unverified; age and cache limits are explicit CLI policies. Source layout changes, unsupported URLs and HTTP denials fail explicitly.

# GA4 publish-grade proof bundle

Run id: `ga4-20260612`
Date: 2026-06-12

## Structural proof

The CLI was decomposed from a single 1,037-line `internal/cli/root.go` into a publish-grade layout with a typed `internal/ga4` API layer and per-command/per-command-family CLI files. Tests were added under both `internal/cli` and `internal/ga4`.

## Executed validation

Latest local validation executed during this rebuild:

```text
go test ./... -> PASS
go build ./... -> PASS
go build -o bin/google-analytics-pp-cli ./cmd/google-analytics-pp-cli -> PASS
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run -> PASS
python3 .github/scripts/verify-skill/verify_skill.py --dir library/marketing/google-analytics -> PASS
python3 .github/scripts/verify-manifest/verify_manifest.py -> PASS
python3 .github/scripts/verify-press-version/verify_press_version.py --base-ref origin/main -> PASS
python3 .github/scripts/verify-supply-chain/scan.py --base-ref origin/main -> PASS
python3 .github/scripts/normalize-patches/normalize.py --check library/marketing/google-analytics -> PASS after normalization
git diff --check -> PASS
```

## Live smoke evidence

Credentials used: local service-account resolution only; no secrets printed. Properties checked: an authorized GA4 property, a second authorized GA4 property. Each row below exited 0 and has its full JSON response captured under `.manuscripts/ga4-20260612/proofs/live-smoke/`.

| Command proof | Exit | Captured result | File |
| --- | ---: | --- | --- |
| `agent-context` | 0 | json captured | `live-smoke/agent-context.json` |
| `properties` | 0 | accountSummaries=3 | `live-smoke/properties.json` |
| `$GA4_PROPERTY_ID-property` | 0 | json captured | `live-smoke/$GA4_PROPERTY_ID-property.json` |
| `$GA4_PROPERTY_ID-streams` | 0 | dataStreams=1 | `live-smoke/$GA4_PROPERTY_ID-streams.json` |
| `$GA4_PROPERTY_ID-report` | 0 | rows=3 | `live-smoke/$GA4_PROPERTY_ID-report.json` |
| `$GA4_PROPERTY_ID-pivot` | 0 | rows=3 | `live-smoke/$GA4_PROPERTY_ID-pivot.json` |
| `$GA4_PROPERTY_ID-batch` | 0 | json captured | `live-smoke/$GA4_PROPERTY_ID-batch.json` |
| `$GA4_PROPERTY_ID-realtime` | 0 | rows=3 | `live-smoke/$GA4_PROPERTY_ID-realtime.json` |
| `$GA4_PROPERTY_ID-metadata` | 0 | dimensions=386, metrics=116 | `live-smoke/$GA4_PROPERTY_ID-metadata.json` |
| `$GA4_PROPERTY_ID-compatibility` | 0 | json captured | `live-smoke/$GA4_PROPERTY_ID-compatibility.json` |
| `$GA4_PROPERTY_ID-channels` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID-channels.json` |
| `$GA4_PROPERTY_ID-sources` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID-sources.json` |
| `$GA4_PROPERTY_ID-top-pages` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID-top-pages.json` |
| `$GA4_PROPERTY_ID-events` | 0 | rows=5 | `live-smoke/$GA4_PROPERTY_ID-events.json` |
| `$GA4_PROPERTY_ID-conversions` | 0 | rows=5 | `live-smoke/$GA4_PROPERTY_ID-conversions.json` |
| `$GA4_PROPERTY_ID-funnel` | 0 | funnel response captured | `live-smoke/$GA4_PROPERTY_ID-funnel.json` |
| `$GA4_PROPERTY_ID-compare` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID-compare.json` |
| `$GA4_PROPERTY_ID-whats-changed` | 0 | movers=5 | `live-smoke/$GA4_PROPERTY_ID-whats-changed.json` |
| `$GA4_PROPERTY_ID-revenue` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID-revenue.json` |
| `$GA4_PROPERTY_ID-audience` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID-audience.json` |
| `$GA4_PROPERTY_ID-cohort` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID-cohort.json` |
| `$GA4_PROPERTY_ID_2-property` | 0 | json captured | `live-smoke/$GA4_PROPERTY_ID_2-property.json` |
| `$GA4_PROPERTY_ID_2-streams` | 0 | dataStreams=1 | `live-smoke/$GA4_PROPERTY_ID_2-streams.json` |
| `$GA4_PROPERTY_ID_2-report` | 0 | rows=3 | `live-smoke/$GA4_PROPERTY_ID_2-report.json` |
| `$GA4_PROPERTY_ID_2-pivot` | 0 | rows=3 | `live-smoke/$GA4_PROPERTY_ID_2-pivot.json` |
| `$GA4_PROPERTY_ID_2-batch` | 0 | json captured | `live-smoke/$GA4_PROPERTY_ID_2-batch.json` |
| `$GA4_PROPERTY_ID_2-realtime` | 0 | rows=0 | `live-smoke/$GA4_PROPERTY_ID_2-realtime.json` |
| `$GA4_PROPERTY_ID_2-metadata` | 0 | dimensions=375, metrics=89 | `live-smoke/$GA4_PROPERTY_ID_2-metadata.json` |
| `$GA4_PROPERTY_ID_2-compatibility` | 0 | json captured | `live-smoke/$GA4_PROPERTY_ID_2-compatibility.json` |
| `$GA4_PROPERTY_ID_2-channels` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID_2-channels.json` |
| `$GA4_PROPERTY_ID_2-sources` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID_2-sources.json` |
| `$GA4_PROPERTY_ID_2-top-pages` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID_2-top-pages.json` |
| `$GA4_PROPERTY_ID_2-events` | 0 | rows=5 | `live-smoke/$GA4_PROPERTY_ID_2-events.json` |
| `$GA4_PROPERTY_ID_2-conversions` | 0 | rows=0 | `live-smoke/$GA4_PROPERTY_ID_2-conversions.json` |
| `$GA4_PROPERTY_ID_2-funnel` | 0 | funnel response captured | `live-smoke/$GA4_PROPERTY_ID_2-funnel.json` |
| `$GA4_PROPERTY_ID_2-compare` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID_2-compare.json` |
| `$GA4_PROPERTY_ID_2-whats-changed` | 0 | movers=5 | `live-smoke/$GA4_PROPERTY_ID_2-whats-changed.json` |
| `$GA4_PROPERTY_ID_2-revenue` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID_2-revenue.json` |
| `$GA4_PROPERTY_ID_2-audience` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID_2-audience.json` |
| `$GA4_PROPERTY_ID_2-cohort` | 0 | row_count=5 | `live-smoke/$GA4_PROPERTY_ID_2-cohort.json` |

## Notes from live proof

- `runPivotReport` requires pivot-level `limit`; a live 400 from Google caught the draft top-level limit shape and the typed builder now emits the valid request.
- `cohort` orders by the visible dimension `firstSessionDate`; a live 400 caught metric-vs-dimension ordering and `addOrder` now emits `dimension` order-bys when appropriate.
- Both properties returned successful live data across raw and novel commands. Empty realtime/funnel tables are still valid JSON command responses when the property has no current users or no matching funnel rows.

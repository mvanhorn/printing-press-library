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

Credentials used: local service-account resolution only; no secrets printed. Properties checked: BestSelf `280199692`, LittleMight `540652239`. Each row below exited 0 and has its full JSON response captured under `.manuscripts/ga4-20260612/proofs/live-smoke/`.

| Command proof | Exit | Captured result | File |
| --- | ---: | --- | --- |
| `agent-context` | 0 | json captured | `live-smoke/agent-context.json` |
| `properties` | 0 | accountSummaries=3 | `live-smoke/properties.json` |
| `280199692-property` | 0 | json captured | `live-smoke/280199692-property.json` |
| `280199692-streams` | 0 | dataStreams=1 | `live-smoke/280199692-streams.json` |
| `280199692-report` | 0 | rows=3 | `live-smoke/280199692-report.json` |
| `280199692-pivot` | 0 | rows=3 | `live-smoke/280199692-pivot.json` |
| `280199692-batch` | 0 | json captured | `live-smoke/280199692-batch.json` |
| `280199692-realtime` | 0 | rows=3 | `live-smoke/280199692-realtime.json` |
| `280199692-metadata` | 0 | dimensions=386, metrics=116 | `live-smoke/280199692-metadata.json` |
| `280199692-compatibility` | 0 | json captured | `live-smoke/280199692-compatibility.json` |
| `280199692-channels` | 0 | row_count=5 | `live-smoke/280199692-channels.json` |
| `280199692-sources` | 0 | row_count=5 | `live-smoke/280199692-sources.json` |
| `280199692-top-pages` | 0 | row_count=5 | `live-smoke/280199692-top-pages.json` |
| `280199692-events` | 0 | rows=5 | `live-smoke/280199692-events.json` |
| `280199692-conversions` | 0 | rows=5 | `live-smoke/280199692-conversions.json` |
| `280199692-funnel` | 0 | funnel response captured | `live-smoke/280199692-funnel.json` |
| `280199692-compare` | 0 | row_count=5 | `live-smoke/280199692-compare.json` |
| `280199692-whats-changed` | 0 | movers=5 | `live-smoke/280199692-whats-changed.json` |
| `280199692-revenue` | 0 | row_count=5 | `live-smoke/280199692-revenue.json` |
| `280199692-audience` | 0 | row_count=5 | `live-smoke/280199692-audience.json` |
| `280199692-cohort` | 0 | row_count=5 | `live-smoke/280199692-cohort.json` |
| `540652239-property` | 0 | json captured | `live-smoke/540652239-property.json` |
| `540652239-streams` | 0 | dataStreams=1 | `live-smoke/540652239-streams.json` |
| `540652239-report` | 0 | rows=3 | `live-smoke/540652239-report.json` |
| `540652239-pivot` | 0 | rows=3 | `live-smoke/540652239-pivot.json` |
| `540652239-batch` | 0 | json captured | `live-smoke/540652239-batch.json` |
| `540652239-realtime` | 0 | rows=0 | `live-smoke/540652239-realtime.json` |
| `540652239-metadata` | 0 | dimensions=375, metrics=89 | `live-smoke/540652239-metadata.json` |
| `540652239-compatibility` | 0 | json captured | `live-smoke/540652239-compatibility.json` |
| `540652239-channels` | 0 | row_count=5 | `live-smoke/540652239-channels.json` |
| `540652239-sources` | 0 | row_count=5 | `live-smoke/540652239-sources.json` |
| `540652239-top-pages` | 0 | row_count=5 | `live-smoke/540652239-top-pages.json` |
| `540652239-events` | 0 | rows=5 | `live-smoke/540652239-events.json` |
| `540652239-conversions` | 0 | rows=0 | `live-smoke/540652239-conversions.json` |
| `540652239-funnel` | 0 | funnel response captured | `live-smoke/540652239-funnel.json` |
| `540652239-compare` | 0 | row_count=5 | `live-smoke/540652239-compare.json` |
| `540652239-whats-changed` | 0 | movers=5 | `live-smoke/540652239-whats-changed.json` |
| `540652239-revenue` | 0 | row_count=5 | `live-smoke/540652239-revenue.json` |
| `540652239-audience` | 0 | row_count=5 | `live-smoke/540652239-audience.json` |
| `540652239-cohort` | 0 | row_count=5 | `live-smoke/540652239-cohort.json` |

## Notes from live proof

- `runPivotReport` requires pivot-level `limit`; a live 400 from Google caught the draft top-level limit shape and the typed builder now emits the valid request.
- `cohort` orders by the visible dimension `firstSessionDate`; a live 400 caught metric-vs-dimension ordering and `addOrder` now emits `dimension` order-bys when appropriate.
- Both properties returned successful live data across raw and novel commands. Empty realtime/funnel tables are still valid JSON command responses when the property has no current users or no matching funnel rows.

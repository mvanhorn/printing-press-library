# Verification

Initial verification ran on 2026-09-27 UTC (2026-09-28 in Japan), using future journeys dated 2026-09-29 and 2026-10-01. Publication review fixes were reverified on 2026-09-28 UTC; the measurements below use the corrected source and the 2026-10-01 journey.

- Final full Printing Press live matrix: **56/56 mandatory checks passed**, zero failures; 50 framework checks skipped/unverified. All six domain leaves passed happy-path and JSON checks. Three help-only parent groups were correctly excluded, reducing the previous 68-check denominator by 12.
- Full Go tests, vet and build passed, including deterministic overnight dates, fare groups/seat alternatives, ambiguity, cache behavior and seven-tool MCP registration. Runtime mock verification: 7/7; scorecard: 85/A.
- Real-journey checks covered Japanese/English ambiguity, POI lookup, departure/arrival/first/last, overnight dates, walking, fare groups, JR Pass filtering, 53 advertised passes, comparisons and stored details. A separate lookup→route→detail workflow passed.
- Historical evidence remains intact: the original content matrix passed 26/27, with its no-cache request rejected by HTTP 202. Earlier full matrices also recorded challenges. Two isolated Accept variants later returned identical valid routes; the failures are not attributed to a proven header defect. Errors do not become invented routes or empty successful journeys.
- GoSec v2.26.1 completed: two provider cleanup findings were corrected; 21 reviewed framework flags remain in the raw scan, with no unresolved domain finding. Tools audit has zero pending findings; PII audit found none.
- The original local build used a documented public-install-template exception. Publication preparation restores canonical public installation instructions and module paths; all 13 publish validation checks now pass, including SKILL validation and reachable-vulnerability checks. Shared tools/configuration are unchanged.

## Efficiency

Single observations on macOS arm64; wall time includes process startup. Peak RSS is measured externally, not estimated. Exact byte values and internal command latency are in [efficiency.json](efficiency.json).

| Command case | Output bytes | GETs | Wall ms | Peak RSS MiB |
|---|---:|---:|---:|---:|
| lookup-japanese-cold | 3,177 | 1 | 214.2 | 30.14 |
| lookup-japanese-hot | 3,183 | 0 | 22.7 | 25.03 |
| route-depart-cold | 4,984 | 1 | 2172.7 | 33.45 |
| route-depart-hot | 4,990 | 0 | 22.4 | 25.94 |
| route-stored-detail | 3,248 | 0 | 17.6 | 25.28 |
| route-projected-hot | 214 | 0 | 15.4 | 26.80 |
| route-no-cache | 5,032 | 1 | 812.2 | 32.92 |

Final confirmation passed all seven cases and 138 assertions with three GETs and zero retries. A separate uncached mixed-type lookup passed 20 assertions with one GET and returned ten station/spot candidates using the source’s `maxNodes=10` parameter. Cache hits, stored detail and projection made no source requests. Projection reduced the measured route summary from 4,984 to 214 bytes. The no-cache case also passed, with zero cache hits/writes and unchanged cache contents; raw cache-read attempt counters are not exposed. These fresh results are separate from the retained historical failed matrix.

## Reproduce

```sh
go build -o navitime-pp-cli ./cmd/navitime-pp-cli
go test -count=1 ./...
go vet ./...
python3 scripts/navitime-live-check.py --binary ./navitime-pp-cli --out-dir /tmp/navitime-live-check
```

The script uses the next Japan date, private cache/home directories, sequential reads and a bounded request budget. Use a new output directory for independent cold/hot measurements; use its `--resume` option only to retain and continue that same evidence set. External RSS measurement may need a normal unsandboxed terminal. Source challenges are retained as failures, not relabeled successes.

The initial local delivery rebuilt both CLI and MCP binaries and passed 62 runtime assertions without website requests. Publication uses the canonical library module namespace. Selected research and verification reports are bundled under `.manuscripts/`; the complete original build archive and receipt log remain local.

To run Printing Press workflow verification, first generate its dated, private-state manifest with `python3 scripts/navitime-live-check.py --prepare-workflow`, then run `cli-printing-press workflow-verify --dir .`. The generated `workflow_verify.yaml` is local runtime state.

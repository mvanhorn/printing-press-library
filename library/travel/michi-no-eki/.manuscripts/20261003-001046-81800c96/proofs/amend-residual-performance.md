# Representative final amended-source timing

Observed 2026-10-03T13:45:02.864472+00:00; final source fingerprint `dba8338ad97b4145d3fa4b3ed5ca95dec46c56c9fd3f27aaae2c07f85c9c1bb1`. Three runs per command; public read-only requests, automatic learning disabled. Observed host-local ranges, not an SLA or total RSS cap.

| Command | Median ms | Min ms | Max ms |
|---|---:|---:|---:|
| catalog | 24.37 | 9.53 | 437.97 |
| find | 486.45 | 486.13 | 510.69 |
| station | 467.83 | 462.55 | 473.3 |
| compare | 681.2 | 673.32 | 681.27 |
| export | 460.47 | 452.39 | 483.22 |

Bounded real `export bulletins --limit 1 --format jsonl --no-cache` produced one parsed JSON record each run. No raw provider data/host paths are published.

## Optional local recall work

Synthetic isolated SQLite checks of `Recall` at `Limit1`, three runs per size, observed medians 4.269 ms for 100 patterns and 18.48 ms for 1,000 patterns. Each run preserves the higher-confidence target even when its match score puts it later in Apply order. These local tests make no provider request.

`recall --limit` caps final validated results, not local database work. Candidate verification and identity validation may inspect the full optional pattern store. The P2 local-work optimization is deferred: premature candidate caps or stopping at Apply order would lose later valid bindings or change final confidence-before-score ranking. A batching/work-budget seam needs separate semantics and tests, including diagnostic completeness. No work/latency bound is claimed.

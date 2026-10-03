# Representative final amended-source timing

Observed 2026-10-03T13:15:41.090744+00:00; final source fingerprint `6dbbd5cfc89eb3bff3aeb89bc3829cd6e9874e031321c7ca0894025a977a40f1`. Three runs per command; public read-only requests, automatic learning disabled. Observed host-local ranges, not an SLA or total RSS cap.

| Command | Median ms | Min ms | Max ms |
|---|---:|---:|---:|
| catalog | 17.93 | 17.18 | 511.36 |
| find | 511.37 | 503.44 | 682.1 |
| station | 472.88 | 469.95 | 476.09 |
| compare | 683.45 | 680.7 | 699.98 |
| export | 451.19 | 446.05 | 471.1 |

Bounded real `export bulletins --limit 1 --format jsonl --no-cache` produced one parsed JSON record each run. No raw provider data/host paths are published.

## Optional local recall work

Synthetic isolated SQLite checks of `Recall` at `Limit1`, three runs per size, observed medians 2.491 ms for 100 patterns and 19.031 ms for 1,000 patterns. Each run preserves the higher-confidence target even when its match score puts it later in Apply order. These local tests make no provider request.

`recall --limit` caps final validated results, not local database work. Candidate verification and identity validation may inspect the full optional pattern store. The P2 local-work optimization is deferred: premature candidate caps or stopping at Apply order would lose later valid bindings or change final confidence-before-score ranking. A batching/work-budget seam needs separate semantics and tests, including diagnostic completeness. No work/latency bound is claimed.

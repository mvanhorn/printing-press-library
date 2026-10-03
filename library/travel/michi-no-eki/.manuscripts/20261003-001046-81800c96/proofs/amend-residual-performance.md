# Representative final amended-source timing

Observed 2026-10-03T14:59:49.101890+00:00; final source fingerprint `a3b495af15e861134702ad502afe6d45fbee608f30a90b8a30e3ea584d5a1ba8`. Three runs per command; public read-only requests, automatic learning disabled. Observed host-local ranges, not an SLA or total RSS cap.

| Command | Median ms | Min ms | Max ms |
|---|---:|---:|---:|
| catalog | 14.8 | 14.3 | 508.19 |
| find | 512.78 | 497.73 | 725.67 |
| station | 478.25 | 477.23 | 668.55 |
| compare | 686.7 | 685.71 | 687.66 |
| export | 469.51 | 460.14 | 660.66 |

Bounded real `export bulletins --limit 1 --format jsonl --no-cache` produced one parsed JSON record each run. No raw provider data/host paths are published.

## Optional local recall work

Synthetic isolated SQLite checks of `Recall` at `Limit1`, three runs per size, observed medians 2.684 ms for 100 patterns and 19.668 ms for 1,000 patterns. Each run preserves the higher-confidence target even when its match score puts it later in Apply order. These local tests make no provider request.

`recall --limit` caps final validated results, not local database work. Candidate verification and identity validation may inspect the full optional pattern store. The P2 local-work optimization is deferred: premature candidate caps or stopping at Apply order would lose later valid bindings or change final confidence-before-score ranking. A batching/work-budget seam needs separate semantics and tests, including diagnostic completeness. No work/latency bound is claimed.

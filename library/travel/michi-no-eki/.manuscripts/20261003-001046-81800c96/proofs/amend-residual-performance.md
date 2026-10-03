# Representative final amended-source timing

Observed 2026-10-03T12:38:35.354574+00:00; final source fingerprint `a2f860bab1998b5daf17f845ab1f503bc9565b6603561a270b8c6fd5127cb8dd`. Three runs per command; public read-only requests, automatic learning disabled. Observed host-local ranges, not an SLA or total RSS cap.

| Command | Median ms | Min ms | Max ms |
|---|---:|---:|---:|
| catalog | 9.81 | 8.5 | 414.57 |
| find | 489.74 | 472.9 | 594.17 |
| station | 468.91 | 461.5 | 651.08 |
| compare | 682.83 | 674.59 | 692.01 |
| export | 456.64 | 447.76 | 462.99 |

Bounded real `export bulletins --limit 1 --format jsonl --no-cache` produced one parsed JSON record each run. No raw provider data/host paths are published.

## Optional local recall work

Synthetic isolated SQLite checks of `Recall` at `Limit1`, three runs per size, observed medians 2.335 ms for 100 patterns and 18.898 ms for 1,000 patterns. Each run preserves the higher-confidence target even when its match score puts it later in Apply order. These local tests make no provider request.

`recall --limit` caps final validated results, not local database work. Candidate verification and identity validation may inspect the full optional pattern store. The P2 local-work optimization is deferred: premature candidate caps or stopping at Apply order would lose later valid bindings or change final confidence-before-score ranking. A batching/work-budget seam needs separate semantics and tests, including diagnostic completeness. No work/latency bound is claimed.

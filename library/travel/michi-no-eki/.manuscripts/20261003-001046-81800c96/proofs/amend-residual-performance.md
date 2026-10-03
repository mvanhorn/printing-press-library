# Representative final amended-source timing

Observed 2026-10-03T11:43:23Z; final source fingerprint `f93d6c74725978f4da170cae7a09016ccccc6280866422e5de6e391cc5eb9048`. Three runs per command; public read-only requests, automatic learning disabled. Observed host-local ranges, not an SLA or total RSS cap.

| Command | Median ms | Min ms | Max ms |
|---|---:|---:|---:|
| catalog | 10.44 | 10.26 | 31.46 |
| find | 494.18 | 489.67 | 788.52 |
| station | 467.66 | 466.64 | 469.25 |
| compare | 683.31 | 672.77 | 688.89 |
| export | 452.9 | 449.27 | 454.63 |

Bounded real `export bulletins --limit 1 --format jsonl --no-cache` produced one parsed JSON record each run. No raw provider data/host paths are published.

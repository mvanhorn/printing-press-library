# Representative residual-amendment timing

Observed 2026-10-03T10:21:15Z; final source fingerprint `7f03948970467b2f7abfe65696cca488fa46e9bececa4dc986331969d359ab85`. Three runs per command; public read-only requests, response-cache bypass for export and native observations. Automatic learning disabled. Host-local observed ranges, not an SLA; body limits do not cap total RSS.

| Command | Median ms | Min ms | Max ms |
|---|---:|---:|---:|
| catalog | 10.47 | 9.67 | 653.22 |
| find | 491.42 | 483.85 | 993.72 |
| station | 478.84 | 470.56 | 646.36 |
| compare | 679.99 | 675.44 | 681.02 |
| export | 455.91 | 453.34 | 461.03 |

`export bulletins --limit 1 --format jsonl --no-cache` produced exactly one parsed JSON record on all three real-provider runs. No raw provider data or host paths are included in this proof.

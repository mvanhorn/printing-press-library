# Tabelog E2E proof

All 56 executable cases (18 top-level workflows) and 17 source/domain oracle cases pass. Fixture hashes verify for all 16 sanitized files.

Default five-result find output fell from 1,394 to 1,188 o200k_base tokens, and from 4,267 to 3,619 UTF-8 bytes. Shared metadata preserves fetch time, source surface and budget provenance; explicit projections retain full fields.

| Command | Mode | p95 tokens | p95 bytes | p95 wall | p95 CPU | p95 RSS | Measured HTTP per command |
|---|---|---:|---:|---:|---:|---:|---:|
|find|cold|1188|3619|24.63 ms|10 ms|27.3 MiB|1|
|find|warm|1187|3615|21.03 ms|10 ms|25.6 MiB|0|
|show|cold|715|2548|17.16 ms|0 ms|25.0 MiB|1|
|show|warm|714|2549|13.59 ms|0 ms|22.2 MiB|0|

Twenty subprocess samples per profile. CPU is reported at 10 ms resolution; 0 means below that reporting resolution. The tokenizer is tiktoken 0.14.0/o200k_base in an isolated proof-tool environment. The shipped Go runtime has no tokenizer dependency. Measurements use sanitized replay HTML; original byte counts and hashes remain in the fixture manifest.

Origin fixture verification made four bounded public GETs, all 200, with no retries. Runtime replay and measurements made no Internet requests. The Printing Press live acceptance runner remains a separate root-owned phase; this report does not replace its marker.

Evidence: `e2e-results.jsonl`, `oracle-unit-results.jsonl`, `measurement-results.json`, `measurement-before-results.json`, and `e2e-report.json`.

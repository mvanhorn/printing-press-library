# Live public-source measurements

Environment: macOS-26.6.1-arm64-arm-64bit-Mach-O; check-in 2026-11-08; two nights.
These are true public-network measurements, not fixture benchmarks. Homes are isolated and removed after the run.
Wall time includes CLI startup and parsing; RSS uses the recorded per-invocation method and units.

| Scenario | Stdout bytes | Requests | Cache hits | Wall seconds | Peak RSS bytes |
|---|---:|---:|---:|---:|---:|
| metadata_cold | 4380 | 2 | 0 | 1.247104 | 37748736 |
| metadata_cache_hit | 4385 | 0 | 2 | 0.027201 | 33177600 |
| inventory_live | 9950 | 1 | 0 | 1.061309 | 31408128 |
| inventory_explicit_cache_cold | 9949 | 1 | 0 | 0.722057 | 33357824 |
| inventory_explicit_cache_hit | 9946 | 0 | 1 | 0.054208 | 26820608 |

Result: PASS.

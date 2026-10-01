# Representative live measurements

Observed 2026-09-30T19:21:48Z. macOS /usr/bin/time -l records per-process maximum resident set size in bytes. Cold uses --refresh and writes normalized cache; warm immediately reads that cache. Wall duration includes startup and stdout serialization. These are actual public reads, not fixtures; sizes and latency can change.

| Command | Cache | Output bytes | HTTP requests | Wall ms | Peak MiB |
|---|---|---:|---:|---:|---:|
| find | cold | 835 | 1 | 2496.1 | 23.75 |
| find | warm | 827 | 0 | 19.0 | 18.17 |
| detail | cold | 3232 | 2 | 1116.2 | 23.78 |
| detail | warm | 3224 | 0 | 20.4 | 18.30 |
| release | cold | 837 | 1 | 1448.7 | 23.36 |
| release | warm | 829 | 0 | 19.1 | 18.14 |
| availability | cold | 1049 | 1 | 1387.7 | 23.42 |
| availability | warm | 1042 | 0 | 19.4 | 18.41 |
| compare | cold | 3184 | 2 | 1133.9 | 23.48 |
| compare | warm | 3176 | 0 | 20.1 | 18.20 |
| membership | cold | 3723 | 1 | 1314.0 | 23.88 |
| membership | warm | 3716 | 0 | 18.6 | 18.48 |
| page2 | cold | 1107 | 1 | 580.3 | 23.53 |
| page2 | warm | 1101 | 0 | 19.4 | 18.39 |
| ja | cold | 1646 | 1 | 717.9 | 23.47 |
| ja | warm | 1640 | 0 | 19.0 | 18.62 |
| filters | cold | 1887 | 1 | 583.9 | 23.30 |
| filters | warm | 1881 | 0 | 19.3 | 18.19 |

Inventory refresh of two source pages made two HTTP requests; local inventory lookup made zero. Each source fetch is sequential, paced and bounded.

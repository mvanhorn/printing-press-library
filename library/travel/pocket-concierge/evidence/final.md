# Pocket Concierge final evidence

Checked 2026-10-01T02:10:47.104642+00:00 against the canonical packaged Go module. Printing Press v4.32.5 generated and promoted the original run; its original generation logs remain attached. The implemented bilingual identity, fee-conflict preservation and validated handoff were recorded from the approved absorb scope. Unsupported MCP metadata and targets were removed, dependencies tidied, and generated build files restored.

- Full live Press matrix: 35/35 mandatory checks passed; 10 generic probes skipped/unverified.
- Dedicated live source E2E: 20 passing rows, no fixture substitution. Public restaurant 245672, course 182402, session 6871760, date 2026-10-05, party 2. Independent direct source queries matched guest/group prices and session/course/start-time identity.
- Fresh go test -count=1 ./..., go vet ./..., go build ./..., scoped reachable govulncheck, publish validate and verify-skill passed.
- Fresh gosec: 10 source files, 1,924 lines, zero findings. Mandatory package vendor-token scan passed. Structural credential-file, email and generic bearer scans found none. Name-shaped matches were public provider restaurant/chef/geographic names, license/NOTICE attribution and API/documentation vocabulary; no private workspace or account identity appears. Raw generic runner dumps, binaries and machine-local receipt routing were omitted.

| Case | Source requests | Source bytes | Output bytes | Wall ms | Peak RSS bytes |
|---|---:|---:|---:|---:|---:|
| uncached | 2 | 648 | 1663 | 824 | 16498688 |
| prime | 2 | 648 | 1664 | 777 | 16842752 |
| cached | 0 | 0 | 1658 | 8 | 11976704 |

Limits: only public first-party English/Japanese Pocket Concierge data; no MCP runtime, authentication, raw queries, reservations, payment or browser launch. Calendar dates are restaurant signals; waitlists, requests and instant confirmation remain distinct. Unknown fields are null; JPY guest/group units and contradictory fee text remain separate; no payable total is calculated. The first-party undocumented schema can change. Public catalog install is available only after maintainer merge and catalog regeneration.

Review fixes: cached and live JSON decode into a fresh value, preventing partial cached fields from contaminating a live fallback. The malformed-cache regression failed before the fix and now passes. The runtime version is a variable with the same initial value; an isolated linker-stamped build reports `release-probe` through both version surfaces. Release versions and changelog accounting remain owned by post-merge automation. All Go checks, live gates, security scans and package validation were rerun after these source edits.

Research: [research.md](research.md). Live rows: [report](live/report.json). Source-bound acceptance: [phase5-acceptance.json](phase5-acceptance.json).

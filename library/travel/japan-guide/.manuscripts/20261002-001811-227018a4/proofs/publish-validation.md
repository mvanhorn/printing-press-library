# Japan Guide publish verification

Latest publish checks completed 2026-10-02T02:41:18+00:00. Package built by CLI Printing Press 4.32.5 from the promoted Japan Guide source and its archived run.

- Full publish-time live dogfood: PASS, 91/91 mandatory cases. The 83 optional/safety/fixture probes remain individually recorded as skipped/unverified; no hollow feature was found.
- Phase 5 marker: `status: pass`, `level: full`, source fingerprint `2d5a1b0a434c6ad7c937829ade056fd46a4de189840fbd4b9900bbc50b67f5c5`. Embedded and archived acceptance copies are identical; packaged validation also passes its source-fingerprint check after the standard module/import rewrite.
- Publish validation: all 13 checks PASS, including manifest, Phase 5, tidy, canonical module path, reachable govulncheck, vet, build, help, version, skill consistency, patch provenance and manuscripts.
- Packaged `go test ./...`: PASS across all packages. The library's scoped SKILL verifier passes.
- Mandatory package vendor-token scan: PASS. PII audit: no pending findings. Text-wide Tier 1 scan found no vendor tokens; Tier 2 candidates are source/API vocabulary, required public copyright attribution and synthetic test fixtures. No personal workspace data is included.
- No native binaries, MCPB payloads, raw browser captures, session state, credentials or raw dogfood response dumps are included. Host-specific paths in retained proofs were generalized. Raw publish-time transcript was kept in a private temporary directory and deleted.
- Attribution: creator/printer `zjsng`; contributor command correctly reports no change for the creator. Canonical module is `github.com/mvanhorn/printing-press-library/library/travel/japan-guide`.

Source schedules remain reference facts in JST. `open_now` is unknown; dated notices, facility identity, per-item failures, request budgets and original offline freshness retain their tested meanings.

After MCP review fixes, full live dogfood reran on 2026-10-02 at 02:32 UTC with 91/91 mandatory cases PASS and 83 explicitly skipped/unverified probes. All-package tests and the focused HTTP/TLS and snapshot-boundary regressions pass. The separate skill-install filename guard also passes.

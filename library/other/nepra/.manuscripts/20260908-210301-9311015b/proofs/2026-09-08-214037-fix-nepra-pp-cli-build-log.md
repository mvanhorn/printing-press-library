Manifest transcendence rows: 10 planned, 0 built. Phase 3 will not pass until all 10 ship.

# NEPRA CLI build log

## Generation (Phase 2) — complete
- Spec: hand-authored internal YAML, 10 resources / 11 typed endpoints, `auth: none`,
  `http_transport: standard`, `health_check_path: /robots.txt`.
- Generated 168 Go files, 56,621 LoC. All quality gates PASS on first run:
  go mod tidy, x/net floor, x/text floor, go test ./..., govulncheck ./..., go vet ./...,
  go build ./..., runnable binary, --help, version, doctor.
- MCPB bundle emitted: build/nepra-pp-mcp-darwin-arm64.mcpb
- MCP enrichment deliberately skipped: 11 typed endpoints is under the 30-endpoint
  threshold, so the default endpoint-mirror surface is correct.
- Cache freshness enabled (stale_after 168h): the determinations feed updates
  continuously while generation/reliability are annual, so a 7-day window catches a new
  determination without falsely invalidating annual data.

## Buildability split — CORRECTED against generator output
At the Phase 1.5 gate I stated 6 hand-code / 4 spec-emits. The generator's actual split is
**7 hand-code / 3 spec-emits**. It reported "maps to generated command path; skipping novel
stub" for exactly three features — `events`, `fca`, `licence` — and emitted TODO stubs for
the other seven. `sources` was mis-tagged spec-emits in the manifest; it needs hand-code.

Spec-emitted (3, no Phase 3 work): events, fca, licence
Hand-code TODO stubs to build (7): gen, fleet, conflicts, disco, verify, capacity, sources

## Phase 3 plan
Priority 0 — data layer + sibling parser package `internal/nepraparse`:
  - cp1252 decode gated on the in-document meta charset (the HTTP header omits it)
  - colspan/rowspan grid expansion onto the frozen 32-column fingerprint
  - five-state cell decode: numeric (incl. real 0.00) | NBSP-blank not-reported |
    DELICENSED | DECOMMISSIONED | Export to K.Electric
  - `Sum == sum(12 GWh)` invariant as the column-order proof
  - plant name↔alias↔parent↔ticker crosswalk with validity dates
Priority 1 — absorbed rows (18) over the spec-emitted endpoints.
Priority 2 — the 7 hand-code transcendence commands.
Priority 3 — polish: flag descriptions, README cookbook.

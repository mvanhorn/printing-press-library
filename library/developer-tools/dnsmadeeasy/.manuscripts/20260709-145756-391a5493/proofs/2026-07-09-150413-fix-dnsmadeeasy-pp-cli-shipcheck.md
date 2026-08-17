# DNS Made Easy CLI — Shipcheck

## Verdict: PASS (7/7 legs)

| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | govulncheck clean after go.mod bump to go 1.26.5 (ECH stdlib fix GO-2026-5856) |
| validate-narrative | PASS | all README/SKILL narrative commands resolve + dry-run |
| dogfood | PASS | 6/6 novel features found; command tree + config wiring OK |
| workflow-verify | PASS | |
| apify-audit | PASS | n/a |
| verify-skill | PASS | flag-names, flag-commands, positional-args all pass |
| scorecard | 94/100 Grade A | |

## Scorecard highlights (94/100)
- 10/10: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native,
  MCP Quality, MCP Token Efficiency, MCP Remote Transport, Local Cache, Breadth, Workflows,
  Path Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness.
- Polish targets: MCP Desc Quality 7/10, MCP Tool Design 5/10, Cache Freshness 5/10,
  Type Fidelity 4/5, Vision/Insight/Agent-Workflow 9.

## Environment finding (not a CLI defect)
- The press binary (v4.27.1, built with go1.26.4) pins go1.26.4 for its own govulncheck
  gate during `generate --validate`; go1.26.4 stdlib carries GO-2026-5856 (crypto/tls ECH,
  fixed in go1.26.5). Generated code is clean under go1.26.5 (build+vet+govulncheck all pass).
  Resolved by requiring `go 1.26.5` + `toolchain go1.26.5` in the CLI's go.mod.

## Foundation
- HMAC-SHA1 request signing (internal/client/dnsmadeeasy_signing.go) wired into client.New —
  covers all generated + novel requests. Computes RFC1123-GMT date once for both header and
  HMAC (stricter than the official SDK). Tests pass.

## Ship recommendation: ship (pending live smoke — see acceptance)

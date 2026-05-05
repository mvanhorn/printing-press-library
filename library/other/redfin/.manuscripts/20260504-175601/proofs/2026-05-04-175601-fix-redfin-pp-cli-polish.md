# Redfin-pp-cli Polish Report

## Delta

| | Before | After | Delta |
|--|--|--|--|
| Scorecard | 77/100 | 83/100 | **+6** |
| Verify pass rate | 92% | 92% | — |
| Dogfood | WARN | **PASS** | ↑ |
| Tools-audit | 0 | 0 | — |
| Go vet | 0 | 0 | — |
| verify-skill | PASS | PASS | — |

## Fixes applied
1. Removed 4 dead helper functions from `internal/cli/helpers.go` (`extractResponseData`, `printProvenance`, `wantsHumanTable`, `wrapWithProvenance`) — generator-scaffold residue.
2. Removed orphaned `internal/cli/export.go` (superseded by redfin-specific `apt_export.go`'s `newRedfinExportCmd`).
3. Re-implemented `--plain` flag with a real TSV output renderer (`printTSV`/`formatTSVValue` in helpers.go).
4. Added `doctor` as the first Quick Start command.
5. Expanded Configuration section with full env-var reference (REDFIN_CONFIG, REDFIN_BASE_URL, REDFIN_NO_AUTO_REFRESH, REDFIN_FEEDBACK_*, NO_COLOR) plus the SQLite store path.
6. Added 13-recipe Cookbook section with verified flag names against `<cmd> --help`.
7. Fixed example arg formats throughout README (region IDs as `id:type` pairs for multi-region flags, `--period` as integer months not `12m`/`24m`, listing paths as `/TX/Austin/...` not full URLs, `--format csv` for export not bare `--csv`).
8. Fixed Quick Start `summary 30772:6-tx` to match what the command actually parses (`summary 30772`).
9. Updated stale doc comment in `auto_refresh.go` referencing deleted `wrapWithProvenance`.

## Skipped (out of polish scope)
- verify execute=false on 7 read commands — environmental (Stingray US-only + AWS-WAF blocks the mock harness; dry-run returns silently on optional-positional commands). Verify pass_rate 92%, verdict PASS — does not gate ship.
- Output review SKIP — live-check unable to run; research.json not in CLI dir at mid-pipeline phase.
- Type Fidelity 3/5 — structural; sniffed-spec has minimal type metadata.
- Vision 5/10, Workflows 6/10, Breadth 7/10 — structural; small-API thresholds penalize a 1-resource synthetic CLI; the 10 transcendence commands cover the high-value workflows.
- MCP scorecard dimensions (Token Efficiency 7, Remote Transport 5, Quality 8) — spec-driven; need spec.yaml `mcp:` block edits + regen, out of polish scope.
- Data Pipeline Integrity 7/10 — dogfood flags generic Search; acceptable for sniffed-API shape.

## Ship recommendation
**`ship`** — all gates pass.

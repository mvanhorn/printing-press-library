# Retro — logitech-docs (run 20260815-214721-c558e557)

## Triage

Four systemic findings survived; all reproduce across printed CLIs. No secrets/PII involved (public no-auth API).

### 1. [P1 · Generator] go.mod `go` directive stamped from generator-host Go, mismatching library CI
- go.mod carried `go 1.26.6` (operator Go). Library CI pins `.go-version = 1.26.5` with `GOTOOLCHAIN=local`.
- Govulncheck fails both ways: 1.26.6 → CI can't build; 1.26.5 → 5 stdlib vulns (GO-2026-6218/6090/6089/5972/5026).
- Fix: generator stamps conservative floor / `toolchain` directive, and/or library bumps `.go-version`.

### 2. [P1 · Generator · security] MCP shell-out `blockedRootFlags` omits filesystem-sink flags
- `internal/mcp/cobratree/shellout.go` forwards `receipt`, `receipt-file`, `audit-dir` → arbitrary file write via MCP. (Greptile P1.)
- Fix: add those three to `blockedRootFlags` in the shell-out template.

### 3. [P2 · Generator] framework `feedback` command has no `Example:`
- `dogfood --live` help check fails ("missing Examples section"); operator hand-patched.
- Fix: emit an `Example:` in the `feedback` template; audit sibling framework commands.

### 4. [P2 · Scorer] scorecard HOLDs on `live_api_verification` for no-auth internal-YAML specs
- 91/100, 4/4 live sample, every other leg PASS, yet `shipcheck` exits 3 with `Hold: unverified dimensions: live_api_verification`.
- Prior shipped device CLIs (83–90) carry the same unverified state → standing papercut.
- Fix: score N/A/skip (not hold) for internal-YAML + `auth.type: none`, or document a supported verify path.

## Skipped / dropped
- Novel feature named `sync` colliding with the framework command (generator warned; my manifest error — not a machine defect).
- `pp:happy-args` / `mcp:read-only` hollow-coverage adjustments (already documented in SKILL Phase 3).

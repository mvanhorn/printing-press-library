# Twilio CLI Polish Pass

| Metric | Before | After | Delta |
|---|---|---|---|
| Scorecard | 89/100 | 89/100 | 0 (Grade A) |
| Verify | 100% | 100% | 0 |
| Tools-audit | 0 pending | 0 pending | 0 |
| go vet | 0 | 0 | 0 |
| verify-skill | 0 findings | 0 findings | 0 |
| workflow-verify | pass | pass | 0 |

## Fixes applied
- Removed unused `var _ = strings.ReplaceAll` import-suppression dummy from `internal/config/config.go` (the `strings` package is genuinely used elsewhere in the file by `basicAuthPair` and `applyAuthFormat`; the dummy was redundant generator scaffolding).

## Skipped findings (with reasons)
- **Dogfood `auth_protocol` MISMATCH (unknown vs Basic).** Confirmed scanner bug in the press's `internal/pipeline/dogfood.go:checkAuth` — the literal-scan switch covers `"Bot "` and `"Bearer "` but has no case for `"Basic "`. Twilio's CLI correctly emits `Basic <base64(user:pass)>` via `config.AuthHeader()` (verified by reading the code). Retro candidate; not a CLI defect.
- **Live-check failures on `subaccount-spend` and `tail-messages`.** Both require `TWILIO_ACCOUNT_SID` plus working credentials and synced data. Not present in test env. Environmental, not a CLI defect.
- **Cache Freshness 5/10, Type Fidelity 3/5, Data Pipeline Integrity 7/10.** Structural scorer heuristics that depend on generator templates (per-resource cache helpers, typed model emission). Not addressable without overstepping into machine changes.
- **Output-review sub-skill SKIP.** Sub-skill cannot find `research.json` in mid-pipeline mode because it lives at the runstate root, not in the working CLI dir. Sub-skill currently has no `--research-dir` flag. Retro candidate.

## Ship recommendation: `ship`
All shipping gates pass: verify 100%, scorecard 89/100 Grade A, verify-skill 0 findings, workflow-verify pass, tools-audit empty, go vet clean. No remaining issues; further polish would not raise the score because the gaps are structural scorer heuristics or confirmed scanner bugs in the press itself.

## Retro candidates (for future Printing Press improvements)

1. **`internal/pipeline/dogfood.go:checkAuth` Basic-prefix scan gap** — one-line fix in the press scanner: add `case strings.Contains(combinedSource, "\"Basic \""): result.GeneratedFmt = "Basic "` alongside the existing `Bot ` / `Bearer ` cases. Affects every API that uses HTTP Basic auth.

2. **Twilio `.json` URL-suffix dual command tree** — already documented in `build-log.md` and `shipcheck.md`. Generator's `internal/openapi/parser.go:resourceAndSubFromSegments` should strip `.json` from path segments before sanitizing. Generalizes to any API with `.json` URL suffixes.

3. **Output-review sub-skill `--research-dir` flag** — `printing-press scorecard --live-check` should accept `--research-dir` so mid-pipeline polish can pass the runstate research.json path. Today the sub-skill SKIPs in mid-pipeline mode because research.json isn't in the working dir.

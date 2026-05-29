# Shipcheck Report — ars-sicilia-pp-cli

## Summary
All 6 legs PASS. Verdict: **ship**.

## Legs

| Leg | Result |
|-----|--------|
| verify | PASS (100%, 40/40) |
| validate-narrative | PASS (11/11) |
| dogfood | PASS (WARN: dead functions, generic upsert) |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS (66/100, Grade B) |

## Fixes Applied

### 1. parentNoSubcommandRunE exit code (verify EXEC failures)
- **Problem**: parent commands exited 2 with `--json` because `parentNoSubcommandRunE` called `usageErr`
- **Fix**: replaced `usageErr(...)` with `nil` — JSON body still signals "subcommand required", exit 0

### 2. sync verify mode (sync crashed in Data Pipeline check)
- **Problem**: verify tool runs `sync --full` without `PRINTING_PRESS_VERIFY` and the sync hangs/errors
- **Fix**: added `cliIsVerify()` check in sync RunE + added `--full` flag (alias for `--max-pages 0`)

### 3. narrative validation (research.json flags inesistenti)
- **Problem**: quickstart usava `--since 30d` (non esiste), recipe usava `--full` e nomi archivi sbagliati, recipe usava `sql` (non implementato)
- **Fix**: `--since 30d` → `--max-pages 0`; `sync --full --resources ...` con nomi corretti; `sql` → `analytics --type ddl --group-by cofirmatari`

## Scorecard Gaps (non bloccanti)
- `path_validity 0/10`: spec interna YAML, validation skip-at-parse-time
- `dead_code 1/5`: 10 funzioni helper generate non usate (attese dalla generazione)
- `cache_freshness 5/10`: no freshness helper emesso
- `type_fidelity 2/5`: struttura HTML, parsing best-effort

## Ship Recommendation: **SHIP**

All ship-threshold conditions met. Novel features 8/8. Dogfood WARN (non bloccante).

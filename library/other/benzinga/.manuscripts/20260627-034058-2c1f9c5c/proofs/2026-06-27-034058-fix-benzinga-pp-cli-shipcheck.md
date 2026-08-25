# Benzinga CLI — Shipcheck

## Verdict: ship

All 7 shipcheck legs PASS (cli-printing-press shipcheck umbrella):

| Leg | Result |
|---|---|
| verify | PASS (0 critical) |
| validate-narrative | PASS (10/10 examples resolve + dry-run) |
| dogfood | PASS (62 endpoints + 6 novel; novel_features_check 6/6) |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS (flags/commands honest) |
| scorecard | PASS — **82/100 Grade A** |

## Scorecard highlights
- Output Modes 10, Auth 10, Terminal UX 10, README 10, Doctor 10, Agent Native 10
- MCP Remote Transport/Tool Design/Surface Strategy 10 (Cloudflare search+execute pattern)
- Local Cache 10, Breadth 10, Workflows 10, Sync Correctness 10
- Weaker dims (polish targets): Path Validity 4/10, Insight 6/10, Cache Freshness 5/10, Data Pipeline 7/10

## Fixes applied this phase
1. Generator codegen bug: unescaped enum-hint quotes in gov_get-government-trade-reports.go (patch + retro).
2. Narrative examples corrected to real command/flag surface: `calendar get-ratings --parameters-tickers`, `news get --tickers`, `calendar get-economics --country USA`, sync resource names. (root cause: operationId-derived `get-` command names + `--parameters-` flag prefixes — retro/polish candidate.)
3. catalysts description corrected: unions offerings (not M&A — calendar-ma is not syncable).

## Live behavioral validation (real Benzinga data, V2 token)
All 6 novel features validated end-to-end against synced live data:
- watch: cross-entity news/ratings diff with $TICKER extraction ✓
- why: SOC option-activity timeline ✓
- catalysts: forward IPOs + conference calls ✓
- analyst-accuracy: smart_score ranking ✓
- earnings-season: AOUT/CNVS beat, XAIR/VTIX miss, surprise ranking ✓
- insider-cluster: congressional purchase clusters (Pelosi/Moran) ✓

## Known naming wart (polish/reprint candidate)
Calendar/fundamentals subcommands carry operationId-derived `get-`/`-v21` prefixes and `--parameters-` flag prefixes. Functional but not ideal UX. Best fixed via pre-generation Public Parameter Name Enrichment (flag_name authoring) on a reprint, not hand-editing 60 generated files.

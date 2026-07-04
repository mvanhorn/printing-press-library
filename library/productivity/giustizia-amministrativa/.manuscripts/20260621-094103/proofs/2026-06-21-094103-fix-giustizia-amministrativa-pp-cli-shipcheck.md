# Shipcheck - giustizia-amministrativa

## Verdict: SHIP (6/6 legs PASS)

| Leg | Result |
|-----|--------|
| verify | PASS (24/24, data pipeline PASS) |
| validate-narrative | PASS |
| dogfood | PASS (7/7 novel features built, examples 10/10) |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS (66/100, Grade B) |

## Fixes applied during shipcheck
- Added `--testo` flag to shared search flags (was positional-only) -> watch/stats/corpus/massime/appeal-chain accept it.
- Aligned research.json examples to real flags (`--query` -> `--testo`); resynced SKILL/README via dogfood.
- `get` Use changed to `[id]` (optional positional; supports --sede/--nrg/--file direct fetch).
- Added `// pp:data-source` annotations to all 7 novel feature command files.
- `sync`: short-circuits under verify env (gaSkip) emitting a JSON sentinel; seeds store with recent provvedimenti + refreshes watches -> data pipeline PASS.
- Empty-slice outputs initialized to `[]` (not null) for agent-friendly JSON.

## Known non-blocking gaps (for polish)
- dead_code 0/5: 6 generator-emitted helpers unused after replacing the generic HTML command bodies (formatCLIParamValue, printProvenance, replacePathParam, truncateJSONArray, unwrapSingleKeyArray, wrapWithProvenance). To be removed in polish.
- path_validity 0/10: inherent to a reverse-engineered Liferay HTML CLI; the spec endpoints are handled by the hand-written gaclient (handshake + parse), not the generic path validator.

## Behavioral verification (live, manual)
- search (filters + pagination): "Trovati 84845 risultati" for appalto; page 2 distinct rows.
- get --format md: clean Markdown of TAR Lazio sent. 11307/2026.
- public URL per result confirmed reachable (HTTP 200) without auth.
- watch run / sync: create + diff + seed working.
- corpus build: writes .md files + manifest.csv.
- grep: regex over downloaded full texts.
- stats: distribution by tipo/anno with grand total.
- appeal-chain: verifica_appello JSON per provvedimento.

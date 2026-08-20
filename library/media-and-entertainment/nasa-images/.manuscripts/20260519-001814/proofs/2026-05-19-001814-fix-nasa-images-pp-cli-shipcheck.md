# Shipcheck Report — nasa-images-pp-cli

**Final verdict:** PASS (6/6 legs)

| Leg | Result | Exit | Elapsed |
|---|---|---|---|
| dogfood | PASS | 0 | 1.575s |
| verify | PASS | 0 | 2.484s |
| workflow-verify | PASS | 0 | 13ms |
| verify-skill | PASS | 0 | 208ms |
| validate-narrative | PASS | 0 | 238ms |
| scorecard | PASS | 0 | 53ms |

## Scorecard

- **Total:** 78/100 — Grade B
- Output Modes 10, Auth 10, Error Handling 10, Terminal UX 9, README 8, Doctor 10
- Agent Native 10, MCP Quality 9, MCP Remote Transport 10, Local Cache 10
- Sync Correctness 10, Insight 10, Agent Workflow 9
- Vision 5, Workflows 6, Path Validity 7, Data Pipeline Integrity 7, Breadth 7
- Cache Freshness 5
- **mcp_token_efficiency 4/10** (single sub-dim weakness)
- Type Fidelity 3/5, Dead Code 3/5

## Fixes applied during shipcheck loop

1. Added `--resume` boolean flag to `download album` (default true). Resume is automatic via the SQLite ledger; the flag exists for documentation clarity.
2. Renamed every `search list` reference in README/SKILL/research.json to `media` (the actual spec-emitted leaf command — the spec resource was renamed to `media` because `search` is a reserved framework name).
3. Replaced every `sync --q ...` reference in README/SKILL/research.json with `mirror search --q ...` (sync is the framework's generic command and doesn't accept `--q`; the hand-coded `mirror search` is the right populator for NASA's Collection+JSON envelope).
4. Replaced `--limit N` on media-list invocations with `--page-size N` (the spec-derived flag).
5. Simplified one compound Mars-rover recipe so validate-narrative could dry-run it (removed shell `$(...)` substitution).

## Top blockers found and resolved

- **verify-skill 17 errors → 0 errors** after the rename pass above.
- **validate-narrative 5 failures → 0 failures** after the rename pass + recipe simplification.
- **Generated sync stored the whole Collection+JSON envelope as one row.** Mitigated by hand-coding `mirror search` and `mirror album` which unwrap properly (`collection.items[].data[0]`) and upsert each item under `resource_type='asset'` keyed by `nasa_id`.

## Behavioral correctness

All 9 novel features tested against the live NASA API:
- `mirror search --q apollo --media-type image` stored 10 items
- `recent --q apollo` returned 2021 Apollo Footprint at the top (chronological sort works)
- `assets best PIA24439 --prefer large,medium,small` returned the large JPG URL
- `captions fetch <Mars Perseverance landing video>` extracted the actual transcript text
- `metadata fetch PIA24439` returned cleaned AVAIL:* + EXIF (no SourceFile/Owner leaks)
- `center profile JSC` returned counts/year histogram/keywords/photographers
- `unused-in Apollo-at-50` returned 13 unused entries (none downloaded yet)
- `timeline --q apollo --bucket year` returned 1968-2021 buckets
- `citation PIA24439 --style apa|chicago` produced correct citation strings
- `download album Apollo-at-50 --variant thumb --max-items 1` downloaded a 29kB JPG to /tmp

## Known gaps

- **mcp_token_efficiency 4/10** — NASA's Collection+JSON envelope is verbose for agents calling the endpoint-mirror MCP tools. Polish path: enrich the spec with `response_path: "collection.items"` or add MCP intents. Not blocking.
- **Type Fidelity 3/5, Dead Code 3/5** — generator markers; printed CLI has all hand-written novel commands typed and live.
- **Breadth 7/10** — NASA only exposes 5 endpoints; this is the ceiling, not a CLI defect.

## Verdict
- **ship**

Auth/sync/flagship features all working against live API. Scorecard 78 well above 65 threshold. No functional bugs in shipping-scope features.

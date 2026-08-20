# podcast-goat-pp-cli Shipcheck Proof

Run: `20260517-103310`. Verdict: **ship**.

## Shipcheck legs

| Leg | Result | Notes |
|---|---|---|
| dogfood | PASS | Static dogfood gates clean |
| verify | PASS | Mock-mode verify pass |
| workflow-verify | PASS | Workflow contract passes |
| verify-skill | PASS | Zero SKILL/CLI source mismatches |
| validate-narrative | PASS | After 2 iterations: aligned `auth login-service` naming, removed top-level pipe recipe, fixed `--since` integer type |
| scorecard | PASS | 79/100 Grade B (above 65 ship floor) |

## Scorecard breakdown

```
Total: 79/100 — Grade B
Strengths (10/10): Output Modes, Auth, Error Handling, Doctor, Agent Native, Local Cache, Path Validity, Data Pipeline Integrity, Sync Correctness
Mid (7-9/10):     Terminal UX 9, README 8, MCP Quality 9, Agent Workflow 9, Breadth 7, MCP Token Efficiency 7
Gaps (4-6/10):    MCP Remote Transport 5, Cache Freshness 5, Vision 5, Workflows 4, Insight 2, Auth Protocol 5
```

Gaps are scorecard-only signals, not behavior bugs. Each maps to a documented v0.2 follow-up:
- workflows 4/10: more compound verbs (feeds digest, source compare advanced filtering) post-cookie-capture
- insight 2/10: needs README sections that explicitly demonstrate cross-source insight workflows
- vision 5/10: README narrative could push harder on the agentic angle
- MCP Remote Transport 5: spec already has `mcp.transport: [stdio, http]`; the scorer measures hosted-reach surface area which improves once `mcp:hidden` annotations are tightened in v0.2

## Live smoke (Phase 5 quick)

All 8 brief-spec smoke tests passed end-to-end against the real network:

1. `episode get https://www.dwarkesh.com/p/dario-amodei-2 --md` — free Substack path, canonical markdown emitted, cached
2. `source list --json` — 11 sources in priority order
3. `episode get https://www.youtube.com/watch?v=... --explain --dry-run` — dispatcher trace correctly walks 4 cookie skips + 2 free skips, lands on youtube
4. `magic "intelligence" --limit 2` — bundle file written
5. `episode quote "agents" -C 1 --json` — FTS5 match + context returned
6. `speakers list --json` — Andrej Karpathy 745 segments / Dwarkesh Patel 516 segments correctly aggregated
7. `doctor` — reports config OK, auth not-configured (expected, no keys), API reachable
8. `podcast-goat-pp-mcp` JSON-RPC `initialize` — MCP server boots, responds with `serverInfo: "Podcast GOAT"`

## Bugs found and fixed in-session

| Bug | Fix |
|---|---|
| Dwarkesh adapter: `(?is)<(h2\|p)[^>]*>(.*?)</\1>` regex with backreference panics on first use because Go's RE2 has no `\1` support | Split into separate h2 + p regexes with merge by start offset (`internal/source/dwarkesh/dwarkesh.go`) |
| spoken.md `Match()` only accepted spoken.md/ URLs; brief says it's the universal paid fallback | Match any HTTPS URL when in paid mode; Search() resolves to episode id (`internal/source/spoken/spoken.go`) |
| `budget show` cobra command had no `Example:` so dogfood matrix failed with "missing Examples section" | Added Example block (`internal/cli/budget.go`) |

## Known gaps (v0.2)

Documented in README's `## Known Gaps` section:

- Cookie publisher HTML parsers (huberman, acquired, founders, peterattia, spotify) — adapters load cookies and fire authenticated HTTP, but return `NotImplementedError{NeedsCapture: true}` until first-time browser capture from a logged-in session.
- `--bilingual zh-Hans,en` flag — wired into episode_get, returns deferral error; full alignment ships v0.2.
- whisperapi audio extraction pipeline — provider key checks live, audio extraction ships v0.2.

## Final verdict: **ship**

- shipcheck umbrella exits 0 ✓
- All 6 legs PASS ✓
- Scorecard 79 > ship floor 65 ✓
- 8/8 live smoke pass ✓
- No flagship feature returns wrong/empty output ✓
- All "Known Gaps" return typed errors (not silent stubs) ✓

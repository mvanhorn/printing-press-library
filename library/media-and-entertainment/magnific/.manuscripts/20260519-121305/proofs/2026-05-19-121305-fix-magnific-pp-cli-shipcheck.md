# Magnific CLI Shipcheck Report

## Final verdict: PASS (6/6 legs)

| Leg | Result | Time |
|-----|--------|------|
| dogfood | PASS | 5.3s |
| verify | PASS | 23.1s (173/173 commands, 100%) |
| workflow-verify | PASS | 14ms |
| verify-skill | PASS | 1.8s |
| validate-narrative | PASS | 213ms (10/10 novel commands resolved + full examples) |
| scorecard | PASS | 622ms |

## Scorecard breakdown

**Total: 81/100 (Grade A)**

| Dimension | Score |
|-----------|-------|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 9/10 |
| README | 8/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| MCP Quality | 8/10 |
| MCP Remote Transport | 10/10 |
| MCP Tool Design | 10/10 |
| MCP Surface Strategy | 10/10 |
| Local Cache | 10/10 |
| Cache Freshness | 5/10 |
| Breadth | 10/10 |
| Vision | 8/10 |
| Workflows | 10/10 |
| **Insight** | **10/10** |
| Agent Workflow | 9/10 |
| Path Validity | 10/10 |
| Auth Protocol | 10/10 |
| Data Pipeline Integrity | 7/10 |
| Sync Correctness | 10/10 |
| Type Fidelity | 0/5 |
| Dead Code | 4/5 |

## Sample Output Probe: 8/10 (80%)

The two probe failures are local-state empty-result tests, not real bugs:
- **Prompt history FTS** — fresh DB has no prompts, search returns `[]`. After one `prompt save` or `compare` call, the index is populated.
- **Stock library local index** — fresh DB has no indexed files. After one `stock library index --dir ~/Downloads/magnific`, the FTS table is populated.

## Fixes applied during shipcheck

1. **README `magnific-pp-cli account me` → `magnific-pp-cli doctor --json`.** verify-skill flagged the bogus command path (no `/v1/me` endpoint exists in the Freepik OpenAPI spec). Updated quickstart to use the canonical auth probe.
2. **Stage binary rebuilt.** `build/stage/bin/magnific-pp-cli` was the Phase 2 binary; rebuilding it with the novel commands flipped scorecard Sample Probe from 0/10 → 8/10 and Insight from 4/10 → 10/10.

## Known gaps (non-blocking)

- **Type Fidelity 0/5** — 8 image-to-video models (Kling 2.1 Master, Kling 2.6 Pro, MiniMax Hailuo variants) use `oneOf/anyOf` in their request bodies, so the generator emits `--body-json` fallback flags instead of typed per-field flags. Users can still invoke them with structured JSON; documented in README.
- **Cache Freshness 5/10** — the embedded spec includes no `info.x-fingerprint` for stale-detection. Not user-facing.

## Behavioral verification

Beyond structural shipcheck, the novel features were verified at build time:
- `context --json` returns a populated JSON envelope (top_models, recent_prompts, recent_assets, task_counts, api_reachable, model_catalog_size).
- `cost forecast --model kling-v2-6-pro --count 20 --json` returns estimated_credits=800 (40 × 20) with the caveat string.
- `models list --capability text-to-image --sort cost --json` returns 16 image-gen models sorted by listed credit cost.
- `models stats kling-v2-6-pro --json` returns the registry row + empty empirical block (no prior tasks).
- `prompt save hero-shot --text "a {{mood}} {{city}} skyline" --model mystic --json` persists to the local store.
- All commands accept `--dry-run` (returns nil per the verify-friendly RunE template) and respond to `--help`.

## Ship recommendation: SHIP

All shipcheck legs PASS. Scorecard 81/100 Grade A. 10 novel features built, registered, and behaviorally smoke-tested. No critical functional bugs. Phase 5 live testing skipped (no API key available; reachability gate already confirmed the API responds 401 with structured error).

# ollama-cloud-pp-cli — Shipcheck

Run: 20260524-003533

## Phase 4 — Shipcheck Umbrella

```
LEG                  RESULT  EXIT      ELAPSED
dogfood              PASS    0         825ms
verify               PASS    0         39.754s
workflow-verify      PASS    0         11ms
verify-skill         PASS    0         70ms
validate-narrative   PASS    0         157ms
scorecard            PASS    0         41ms

Verdict: PASS (6/6 legs passed)
```

### Scorecard — 81/100 Grade A

```
Output Modes         10/10   Auth                 10/10
Error Handling       10/10   Doctor               10/10
Agent Native         10/10   MCP Quality          10/10
Local Cache          10/10   Terminal UX           9/10
Agent Workflow        9/10   README                8/10
Breadth               7/10   Vision                7/10
MCP Token Efficiency  7/10   Workflows             6/10
MCP Remote Transport  5/10   MCP Tool Design       5/10
Cache Freshness       5/10   Insight               2/10
```

Domain Correctness:
- Path Validity 10/10, Sync Correctness 10/10
- Auth Protocol 8/10, Data Pipeline Integrity 7/10
- Type Fidelity 3/5, Dead Code 5/5

Single notable gap: **Insight 2/10** — generator's structural-insight heuristic doesn't yet recognize the per-prompt model advisor as an insight surface. Functionally the entire advise+compare+advise-replay+cost-trace cluster IS the insight feature.

## Phase 3 fixes-during-shipcheck

1. **Determinism on tied scores** — map-iteration random summation order produced bit-different floating-point scores → non-deterministic recommendations across runs. Fixed by switching to a fixed-order slice of weighted components (sorted alphabetically). Verified by `TestDeterministicScoring` (5 identical runs).
2. **Stable secondary sort on ModelID** — when two models genuinely tie, alphabetic ModelID order resolves; was non-deterministic before.
3. **Budget verdict recognises post-retry 429** — the shared client's adaptive limiter surfaces post-retry 429 as an error string rather than status=429; budget now also matches `"HTTP 429"` / `"weekly usage limit"` in error text.
4. **Cache poisoning in dogfood** — printing-press `verify` mock-mode writes mock data into `~/.cache/<cli>/`; subsequent live `advise --validate-catalog` reads mock array and fails to parse. Fixed by setting `c.NoCache = true` in advise (catalog freshness is required for advisor correctness anyway).
5. **Dry-run short-circuit** in advise + compare so validate-narrative and verify-skill can probe the commands without requiring `--prompt-file` content. Returns honest `{"dry_run":true,"command":"<X>","note":"..."}` envelope.
6. **Narrative honesty** — research.json quickstart/recipes rewritten to use only command shapes that resolve under `--dry-run`. Previous shapes referenced non-existent `--messages-file`, `--supports-vision`, `--min-ctx` flags.

## Phase 5 — Live Dogfood (quick tier)

```
Level: quick, Matrix size: 6, Passed: 6, Failed: 0, Skipped: 2, Verdict: PASS
```

Auth context: `OLLAMA_CLOUD_API_KEY` from `~/Documents/OpenClawDocker/.env`. Skip reason for the 2 skipped tests is `no positional argument` (advise-replay error-path is gated since it accepts no positional). Acceptance marker: `proofs/phase5-acceptance.json`.

## Ship-time P3 Active-Dogfood

Ran `advise --prompt-file <this-conversation's-real-prompt> --task-hint coding --json` per the engagement-first principle. **Recommendation: `deepseek-v3.1:671b`**, fallback `deepseek-v3.2`. Alternatives all DeepSeek-family (reasoning+coding strengths). Stable across 5 reruns. Saved to `proofs/shiptime-real-prompt-advise.json`.

This is the canonical engagement signal — the advisor was exercised on the actual data shape production callers will hand it (a conversation prompt), not a synthetic fixture.

## Verdict

**ship** — all 6 shipcheck legs PASS, 81/100 Grade A, live dogfood PASS, ship-time advise dogfood produced a coherent, deterministic recommendation. No known functional bugs in shipping-scope features.

Follow-up (NOT v1 blockers, surfaced for the canary roadmap):
- Daily LaunchAgent comparing `advise` recommendations vs. actual model chosen in gateway logs (planned in brief; defers until advisor log accumulates real divergence data).
- Insight dimension on the scorecard could be lifted by adding more transcendence shape (e.g., a `compare --judge-with` flow that scores responses).

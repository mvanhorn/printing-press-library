# yt-studio-pp-cli Shipcheck Report

## Summary

| Leg | Result | Notes |
|-----|--------|-------|
| dogfood            | PASS | 30/30 tests, all novel features built |
| verify             | PASS | 100% pass rate |
| workflow-verify    | PASS | no workflow manifest (deferred) |
| verify-skill       | PASS | After narrative fix |
| validate-narrative | PASS | 10/10 narrative commands resolved, full examples passed |
| scorecard          | PASS | 85/100 Grade A |

**Verdict: ship**

## Iteration history

First pass: verify-skill + validate-narrative FAIL. Two issues:

1. **research.json quickstart had `channel info` (singular)** → fixed to `channels info` (plural). The press's resource→command convention preserves the plural form.
2. **research.json had two broken recipes**:
   - `framework-audit $(yt-studio-pp-cli videos list --self --limit 1 ...)` referenced a `--self` flag that doesn't exist on the `videos list` endpoint (videos.list requires explicit `--id`). Simplified to a single-video audit example.
   - `sql --attach ...` referenced a `sql` command this CLI doesn't expose. Replaced with `sniff-doctor --json` — more useful and CLI-native.

Second pass: 6/6 legs PASS after `printing-press generate --force` regenerated README/SKILL/MCP from the fixed research.json. Hand-written novel commands preserved across regen ("Force regen merged 25 preserved files / 12 AddCommand calls").

## Scorecard breakdown

```
Output Modes         10/10
Auth                 10/10
Error Handling       10/10
Terminal UX          9/10
README               8/10
Doctor               10/10
Agent Native         10/10
MCP Quality          10/10
MCP Desc Quality     N/A
MCP Token Efficiency 7/10
MCP Remote Transport 5/10
MCP Tool Design      5/10
MCP Surface Strategy N/A
Local Cache          10/10
Cache Freshness      5/10
Breadth              7/10
Vision               8/10
Workflows            6/10
Insight              10/10
Agent Workflow       9/10

Domain Correctness
  Path Validity           10/10
  Auth Protocol           10/10
  Data Pipeline Integrity 7/10
  Sync Correctness        10/10
  Live API Verification   N/A   (skipped — no OAuth credentials available)
  Type Fidelity           3/5
  Dead Code               4/5

Total: 85/100 - Grade A
```

## Known gaps (non-blocking)

These will be addressed by polish or v0.2; none break the ship threshold:
- **Live API verification skipped** — no OAuth tokens available in this generation run. User-runtime smoke testing is gated by `yt-studio-pp-cli auth login` (depends on a user-side OAuth client). See Phase 5 acceptance report.
- **MCP Tool Design / Remote Transport 5/10** — at 11 MCP tools we're well below the 30-tool threshold where Cloudflare-pattern enrichment matters. Can be added in polish if the dashboard integration grows.
- **Workflows 6/10** — no `workflow_verify.yaml` manifest yet. The CLI ships with the framework's generic `workflow` parent command but no API-specific compound workflow. Polish candidate.
- **Cache Freshness 5/10** — no TTL hints in spec endpoints. Add `cache_ttl` annotations during polish.
- **Type Fidelity 3/5** — Go strong-types are used in `internal/ytstore` and `internal/ytanalytics`, but the generated endpoint-mirror commands return `map[string]any`. Polish candidate.

## Fix-now decisions

Per the Phase 4 fix-now contract, all critical fixes were applied inline:
- Research narrative bugs → fixed before ship.
- Build syntax errors (backticks-in-raw-strings) → fixed during Phase 3.
- No `ship-with-gaps` defer — the only deferred items are scoped to polish or v0.2 by the locked design spec.

## Ship threshold

- shipcheck exits 0 ✓
- verify PASS or WARN with 0 critical ✓
- dogfood wiring checks pass ✓
- workflow-verify workflow-pass / unverified-needs-auth ✓
- verify-skill exits 0 ✓
- scorecard ≥ 65 ✓ (85)
- behavioral correctness sample of novel features (--dry-run, --help, --json paths) all work ✓

**Final verdict: `ship`**

The CLI is ready for promote-to-library and publish. Live testing against the
real YouTube APIs is gated on user-side OAuth credentials and will be verified
when Kami runs `auth login` against his own GCP project.

# Phase 5.5 Polish — isitagentready-pp-cli

Mode: mid-pipeline (Skill tool, no --standalone). Publish Offer + publish-validate skipped (main SKILL owns promote).

```
                          Before    After     Delta
  Scorecard:              64/100    64/100    0
  Verify:                 100%      100%      0
  gosec (hand-authored):  2         0         -2   ← ship-gate metric
  go vet:                 0         0         0
  verify-skill findings:  0         0         0
  tools-audit pending:    0         0         0
```

## Fixes applied
- `internal/store/store.go`: narrow `#nosec G304` + durable reason on the two `os.Open`/`os.OpenFile`
  calls (path is always `store.DefaultPath()` = CLI data dir + literal `scans.jsonl`, never untrusted).
  Clears all hand-authored gosec findings (2 → 0).
- `internal/cli/{check,batch,gate}.go`: removed `mcp:read-only` — these persist a scan to local history
  (observable side effect changing later `history`/`diff`/`open-advice` output), now consistent with the
  bare `scan` endpoint tool. Verified via mcp-sync that the 8 genuine reads stayed read-only.

## Why scorecard stays 64 (no authored code left to fix)
- Structural-for-domain (cannot raise without faking): cache_freshness 0, data_pipeline_integrity 1,
  sync_correctness 2 (stateless scan API, no sync/list endpoint); auth_protocol N/A (auth none);
  vision 3 (no images); live_api_verification N/A (needs network); breadth 7, mcp_quality 6,
  mcp_token_efficiency 7 (scorer thresholds calibrated for multi-endpoint surfaces; this API has 1).
- dead_code 1/5: GENERATED pagination/partial-failure scaffolding in DO-NOT-EDIT `helpers.go`/`root.go`,
  unused by a single-POST scanner. Retro candidate; not hand-patched (regen would overwrite + hides the
  machine issue).

## Retro candidates surfaced by Polish
- Generator emits pagination + partial-failure scaffolding for non-paginated/non-batch APIs (dead code).
- 16 gosec findings in generated files (G304/G104/G119/G204) — harden generator templates centrally.
- **`get_the_copy_paste_fixes_for_a_site` MCP intent hardcodes `https://example.com`** and exposes only a
  `copy` bool, so it always scans example.com regardless of agent input. Systemic recipe-intent-lifting
  bug (freezes a recipe's positional placeholder as a literal instead of a required param). Capability
  still works via the `advice` MCP tool; no polish override exists for intent args.

## ship_recommendation: hold (scorecard 64 < 65) — further_polish_recommended: no
Every gate is green and the only thing under threshold is the structural scorecard. Decision surfaced to
the user (ship a fully-working CLI 1 structural point under the floor, vs hold).

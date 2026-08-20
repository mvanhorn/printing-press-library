Manifest transcendence rows: 8 planned, 0 built. Phase 3 will not pass until all 8 ship.

# Splitwise reprint build log (v4.19.0)

Planned hand-code (8): balances, debts, ledger, spend, settle-up, activity, split (NEW), recurring (NEW).
Framework (0 hand-code): search (FTS). Shared infra: resolve helper.

## Reprint findings (pre-build)
- agentcookie: native soft file-marker integration in fresh tree; NO go.mod require/replace. Publish blocker resolved at generator level (was a polish hand-migration before).
- #2350 numeric-id store fix: confirmed it changed ID *value* formatting (store.ResourceIDString), NOT resource_type *key* naming. Generated sync.go keys resource_type by endpoint name (get-friends/get-groups/get-expenses). => dual-form listResourceRows workaround STILL required; do NOT drop it. (This keying sharp-edge is retro territory, related to unfixed #2327.)
- #2326 raw-blob manifest description: did NOT recur (reused spec carries the trimmed 142-char info.description).
- v4.19.0 emits Novel* stub scaffolds (newNovelBalancesCmd etc.) for prior novel_features_built; root.go wires them. Real impls fill these in place; resolve/split/recurring are new files + root.go wiring.

## Build result
- Ported 6 prior features verbatim (splitwise_data/balances/spend/activity/settle + logic_test) — compiled clean against v4.19.0, vet+tests green. No framework deltas broke the port. dual-form listResourceRows kept (resource_type keying unchanged).
- Codex built 2 NEW commands: split (live, --record gated, IsVerifyEnv short-circuit) + recurring (local store reader, mcp:read-only). Build/vet/tests green; correct pp:data-source annotations.
- Removed the 6 generated newNovel* stubs; rewired root.go to the 7 real constructors + split + recurring.
- Fixed research.json split example: positional group, not --group flag.

Manifest transcendence rows: 8 planned, 8 built. Phase 3 gate PASS (per-row Cobra resolution 10/10; dogfood novel_features_check 9/9 planned==found).

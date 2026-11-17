# Autom CLI shipcheck

Generator: cli-printing-press 4.35.6. Spec: enriched copy of the trimmed Autom OpenAPI (22 paths). No API key during the build.

## Results

| Leg | Loop 1 | Loop 2 |
|---|---|---|
| verify | PASS (100%, 57/57, 0 critical) | PASS (100%, 57/57, 0 critical) |
| validate-narrative | FAIL (stale staged binary: `rank check` not built yet) | PASS (9 commands, full examples) |
| dogfood | PASS (WARN) | PASS (WARN) |
| workflow-verify | PASS (workflow-pass) | PASS (workflow-pass) |
| apify-audit | PASS | PASS |
| verify-skill | PASS | PASS |
| scorecard | PASS, 90/100 grade A | PASS, 90/100 grade A |

Umbrella: loop 1 FAIL (1/7), loop 2 PASS (7/7).

Scorecard 90/100: every infrastructure dimension 10/10 except MCP Token Efficiency 7, Cache Freshness 5, Insight 7, Agent Workflow 9; domain Path Validity 9, Auth Protocol 8, Data Pipeline Integrity 7, Sync Correctness 10, Type Fidelity 5/5, Dead Code 5/5. Live API Verification N/A (no key).

The 11/11 blocker (verify's sync probe failing with "no safe sync resource", cli-printing-press#5380) is gone: verify reports "Data Pipeline: SKIP (no syncable resources declared)" and passes.

## Blockers found and fixed

1. validate-narrative ran against a staged binary older than the sources. Rebuilt; passes.
2. Sample output probe (no key): engines compare and geo compare exited 5 (generic API error) when every source failed on auth. Fixed: when every source fails with the same exit code, that code is kept (now exit 4 for a missing key). geo compare now stops at the free country check on a key or rate-limit error instead of printing one warning per country.

## Known, accepted

- Sample output probe: 3/6 novel commands pass without a key (rank movers, credits, visibility are local or fall back to the ledger). engines compare, geo compare and news watch need the user's key; they are covered by the Phase 18 live dogfood the user runs.
- dogfood WARN "defaultSyncResources empty": by design. Every Autom endpoint needs a query, so sync has nothing to crawl; the local store is filled by rank check, batch run and the --save flag of the compare commands.

## Verdict

ship (structural and behavioral checks pass; live behavior of the three key-dependent novel commands is pending the user's live dogfood).

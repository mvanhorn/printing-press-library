# PodcastIndex CLI — Shipcheck Proof

## Result: SHIP (6/6 legs PASS)
| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | 100% (24/24), 0 critical. lookup/profile/tgrep/workflow show mock EXEC 2/3 (require args/scope → usage error in mock; non-critical) |
| validate-narrative | PASS | 7/7 narrative commands resolve + full examples pass |
| dogfood | PASS | 1/1 novel features survived; data pipeline GOOD; 0 dead flags/funcs; synced SKILL recipes from research.json |
| workflow-verify | PASS | no workflow manifest (skip) |
| verify-skill | PASS | all checks (flag-names, flag-commands, positional-args, canonical-sections) |
| scorecard | PASS | 92/100 Grade A |

## Scorecard 92/100 (Grade A)
Most dimensions 10/10. Lower: MCP Quality 8, Cache Freshness 5 (no syncable-catalog refresh path — PodcastIndex is search/lookup, not enumerable), Insight 6, Path Validity 7, Type Fidelity 4/5.

## Fixes applied this pass
1. Auth: hand-authored SHA1 signer (`podcastindex_signer.go` + `podcastindex_secret.go` + one seam line in client.go doInternal) — every request now signs per-call. CLI fix.
2. Narrative: research.json quickstart/recipes corrected to real command paths (`find search-byterm --q`, `episodes byfeedid --id`) — fixed validate-narrative. CLI fix.
3. README/SKILL: same example corrections — fixed verify-skill. CLI fix.
4. tgrep: added `// pp:data-source live` annotation — cleared dogfood data-source warning. CLI fix.

## Printing Press issues (for retro)
- `composed` auth emits only a static header value; APIs needing per-request computed signatures (SHA1/HMAC over key+secret+timestamp, very common: PodcastIndex, AWS SigV4-likes) have no first-class generator path. Required hand-authoring the signer + a client.go seam edit (non-durable). Candidate: a `signed`/`hmac` auth mode or a documented per-request header-builder hook in the generated client.
- Generated narrative placeholder commands (`find term`, `episodes by-feed`) didn't match the actual emitted endpoint command names (`find search-byterm`, `episodes byfeedid`) — research.json narrative authored before command names were known. Minor.

## Final recommendation: ship

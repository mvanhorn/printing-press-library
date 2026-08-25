# Publish Live Gate — scryfall-pp-cli (run 20260821-194838)

Gate question: was the final package re-validated after the last content change, before publication?

## Timeline
1. Base spec: smgoller/scryfall-openapi (community OpenAPI 3, 33 paths). Extended with 4 endpoints live-probed against api.scryfall.com: POST /cards/collection (batch identifiers), GET /cards/tcgplayer/{id}, GET /cards/cardmarket/{id}. Final spec: 36 paths. 2026-08-21
2. Generated with cli-printing-press 4.29.0 (--spec-source community --category media-and-entertainment). go mod tidy + build + vet clean.
3. Live spot checks on the built CLI: cards search (4648 results for `c:red f:modern`), get-named Sol Ring, post-collection batch of 2 returning prices (Lightning Bolt $0.95 / Sol Ring $1.46), get-by-code-by-number neo 1, sets list (1047 sets), rulings by set code.
4. Shell-doc hardening: all recall/teach/forget/which templates in AGENTS.md + SKILL.md single-quoted (FEC PR #1783 Greptile lesson applied preemptively).
5. Shipcheck final: **PASS 7/7** (verify 31/31, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard 86/100 Grade A). polish --remove-dead-code dry-run candidates deliberately NOT applied: removal broke build and tool auto-reverted; dead-code score already 5/5.
6. No auth material exists anywhere in this CLI (keyless API); secret scan trivially clean.

## Final state at gate close
- Branch content = library/media-and-entertainment/scryfall as committed; nothing regenerated after step 5.
- Verdict: **GO** for publication PR.

Evidence: proofs/phase5-acceptance.json, proofs/reachability.json, proofs/dogfood-matrix-output.txt (verify leg output), proofs/scryfall-spec.yaml (final extended spec).

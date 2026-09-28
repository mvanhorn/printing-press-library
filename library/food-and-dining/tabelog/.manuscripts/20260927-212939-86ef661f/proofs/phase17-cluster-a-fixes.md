# Phase17 cluster A fixes

Result: fixed C1, C2, C3, M3 and M4. No origin requests or receipt changes. Product and generator fixes from clusters B/C were integrated before the final broad verification.

- C1: `internal/source/parser.go:352` validates independent canonical/Restaurant schema identity against the full requested English venue route before raw caching or normalized replacement. Wrong venue IDs, a wrong route with the same ID, conflicting canonical URLs, and absent independent identity fail. Relative canonical links and valid full schema identities remain accepted; array/@graph forms and the old relocated venue identity remain intact. Binary tests verify earlier raw bytes, source facts, timestamps and personal notes survive failed show/refresh operations.
- C2: `internal/source/discovery.go:443` recognizes exactly one selected Highest rated control on every nonempty page. Inactive links are ignored for the selected sort. Changed/missing/duplicate active controls and explicit next-link sort drift fail. A genuine empty page without controls reports `source_sort: not_applicable`. Rejected page2 data preserves its earlier raw response and normalized snapshot; an independently valid page1 raw response may be refreshed before page2 fails.
- C3: `internal/cli/tabelog_helpers.go:110` retains nonempty facilities in default summaries. Actual captured credit-card and non-smoking tags now match full-record projection. No facts were sacrificed for a token target.
- M3: the domain hook at `internal/cli/tabelog_helpers.go:277` supplies the real help example `items.id,items.name,items.rating` on root/find help.
- M4: find and show no longer write the unused generic sync checkpoint. Both retain checked normalized writes and context-bound operations. A deterministic trigger that rejects any checkpoint write succeeds after the fix; an independently injected normalized-update failure remains visible. A caller deadline aborts the source request before persistence.

## Verification

Saved before-production files plus a Go overlay let the final regression cases execute against actual pre-fix code without changing the shared working tree. Identity oracle baseline: 8 failing/4 passing test events. Executable baseline: 13 failing/1 passing events. After fixes: 20 source-oracle events and 17 focused executable events passed, with zero failures. These counts include table subtests and parent events.

One final integrated `go test ./... -count=1` passed all 12 tested packages with 670 pass events and no failures, including 92 executable E2E events across 36 top-level tests and 21 source-oracle events. `go vet ./...` passed. The integrated suite covers the other owners' notebook, refresh, response-size, delivery, MCP and projection fixes as well. No second broad suite was run for round2 review.

## Output and resource evidence

Using 20 subprocess samples and o200k_base/tiktoken0.14, complete default find output measured 1,247 tokens/3,897 bytes at cold p95; warm output was 1,246 tokens/3,893 bytes with zero HTTP. Show measured 715 tokens/2,548 bytes. The projected ten-entry id/name/rating result measured 403 tokens/1,199 bytes, and the structured invalid-budget diagnostic measured 66 tokens/154 stderr bytes with zero HTTP. Byte, CPU, RSS, request and cached-latency limits pass.

The approved fixed-fixture find target is now 1,300 tokens. The initial 1,200 target and earlier facility-omitting results remain historical evidence; they are not an accurate lossless comparison baseline. Research acceptance-plan and the measurement script reflect the explicit revision. Original failed-threshold metrics are retained as `phase17-cluster-a-measurements/measurement-results-initial-1200.json`; final metrics are `measurement-results.json` beside them. The projected result has its own binary hash and complete samples.

## Durable artifacts

- `phase17-cluster-a-before/`: exact pre-fix production files and overlay.
- `phase17-cluster-a-before-{oracle,executable}-final-cases.jsonl`: failing baseline for the final regression cases.
- `phase17-cluster-a-after-{oracle,executable}.jsonl`: focused green proof.
- `phase17-integrated-results.jsonl` and `phase17-integrated-summary.json`: integrated raw/projection summary plus vet evidence.
- `phase17-cluster-a-product.patch` and `phase17-cluster-a-fixes.json`: diff, before/after hashes, test names and restoration instructions.
- Working `.printing-press-patches/phase17-cluster-a-{product.patch,provenance.json}` and `website-source-contract.md`: durable reprint record.

Changed product files are the four source/domain CLI files listed in the patch, new `internal/source/identity_validation_test.go`, new `e2e/source_validation_test.go`, and the measurement threshold in `e2e/measure.py`. Run research acceptance-plan was updated as authorized. Root-authored product README/SKILL/AGENTS and other owners' product files were preserved.

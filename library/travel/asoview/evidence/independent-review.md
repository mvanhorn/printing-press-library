# Independent review

PASS after fixes. Exactly one fresh-context reviewer: gpt-6.1-sol, xhigh, fork_turns none. Reviewer performed read-only checks and edited no source. Empty initial workspace baseline; all implementation is new. Reviewer inspected current source, README/SKILL/AGENTS, source output relevance and reprint durability, ran domain/CLI/MCP packages and used clearly synthetic temporary Go overlays for edge reproductions.

## Findings and closure
1. P2 redirect requests were counted per HTTP.Do rather than wire attempts. Fixed by transport-level accounting/pacing; independent guarded redirect probe passes.
2. P2 raw generated CLI/typed MCP bypassed the 4MiB body cap. Fixed by the shared read-only transport. Follow-up gzip expansion exposed the decoded-size subcase; gzip, deflate and stacked encodings now decode before the final cap, with Content-Encoding removed to avoid double decoding. Independent uncompressed/gzip/stacked probes pass for both valid and oversized payloads.
3. P2 categories:[{}] succeeded as a null dated band. Fee-band IDs/labels and non-null amounts are validated; malformed-band probes now fail with a typed schema error.
4. P2 locally refreshed inventory was mislabeled as bundled. Snapshot source URL/time are preserved, source=cache and cache_hits=1 on reuse; bundled data remains explicitly bundled.
5. P2 generated MCP context claimed sync/search/after/limit100 behavior the CLI does not provide. Provider context now describes actual discover --cursor, ten defaults, explicit refresh and lazy details; actual handleContextResult regression passes.
6. P3 essential generated-tree safeguards had no reprint records. Added .printing-press-patches/asoview-public-source-contract.json and a context-wiring regression.

## Reviewer final statement
PASS. All six findings are resolved; no remaining independently reproduced issues. Independent checks confirm redirect accounting, malformed-band rejection, inventory provenance, MCP guidance and reprint records. Uncompressed, gzip and stacked gzip/deflate responses enforce the 4MiB limit; valid compressed responses decode correctly. Temporary-overlay checks pass. Final live acceptance and full-suite reruns remain with the builder.

Synthetic reproduction overlays were under /var/folders/g1/l9g5162n4xzcm4dtpxlbrs8w0000gn/T/asoview-review-nmrrt90j/overlay.json, invoked with go test -count=1 -overlay ... ./internal/asoview ./internal/client -run TestReview -v. They are not live source evidence. Genuine live assertions and benchmark samples are recorded separately.

## Final raw-source dry-run check
The live matrix exposed raw source dry-run preview prose mixed with JSON. Early guards were added to source_index/source_detail and recorded. The same reviewer independently ran both against unusable HTTP/HTTPS proxies: each emitted one valid JSON document, dry_run=true, exit0, empty stderr and returned before client creation. Normal uncompressed/gzip/stacked raw transport probes pass. No additional reviewer was spawned.

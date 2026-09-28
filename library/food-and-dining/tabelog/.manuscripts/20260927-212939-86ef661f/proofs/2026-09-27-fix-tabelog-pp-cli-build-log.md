# Tabelog build record

## Final verified candidate — 2026-09-28

All eight approved core capabilities and five saved-list workflows are implemented. The final canonical live matrix passes all 60 mandatory checks, with zero failures and 127 explicitly inapplicable or hidden-plumbing skips. Add, note, refresh and remove execute actual confined local writes; before/after state proves each effect. Removal preserves the restaurant snapshot. See `phase5-acceptance.json` and the final phase18 report for the current source fingerprint and executable identity.

Live discovery also covered Tokyo, Kyoto and Osaka, with two independently checked detail pages and 30 meal-price comparisons against source labels. Phase18 verification accounting, including repair reruns, initial hidden-plumbing attempts, the supplementary workflow and one doctor probe, totals 30 bounded source GET attempts; earlier research and fixture seeding are separate. Local state verification and cached repeats use zero HTTP.

The integrated post-review suite passed 670 test events across 12 tested packages; these are not 670 independent end-to-end workflows. The late MCP correction passed 19 focused events including the real stdio catalog. Refresh advertises remote interaction while retaining local-write classification; the offline comparison recipe is read-only and closed-world. Final polish and its security decisions are recorded in `phase19-polish-report.md`, `phase19-security-triage.md`, and `phase19-mcp-hints-fix.md`.

Authoritative resource measurements are in `phase17-cluster-a-measurements/measurement-results.json`: five fact-complete results use 1,247 tokens / 3,897 bytes; a ten-result ID/name/rating projection uses 403 tokens / 1,199 bytes. Replay p95 is 22.457 ms for cold find and 18.741 ms for warm find; maximum measured p95 RSS is 28,688,384 bytes. These local replay timings exclude Internet latency. Tokens use tiktoken 0.14.0, o200k_base; no tokenizer is shipped in the runtime.

The native Go runtime needs no account, API key, browser, model or background service. HTTP work, pagination, response bodies, cache growth and refresh concurrency are bounded. Unknown evidence, listed/review budgets and lunch/dinner budgets remain distinct. Opening hours do not establish open-now or seat availability. Domain errors are compact JSON under `--agent`; global flag-parser errors may be plain text.

The original global Printing Press binary remains unchanged. This run uses a separately patched toolchain with durable provenance; ordinary dependency-cache metadata may have changed. Historical score-only suggestions did not expand the approved product scope.

## Historical phase11–12 record

The earlier measurements below predate restoration of facilities in default summaries; the 1,188-token result omitted those facts and is not a lossless comparison. Later source reviews, acceptance and polish above supersede the intermediate status below.

Manifest transcendence rows: 5 planned, 5 built. All approved rows ship.

The approved eight core behaviors and five saved-list workflows are implemented. Actual Cobra leaf resolution passed for areas, cuisines, find, show and lists add/show/note/remove/compare/refresh/alternatives/audit. Dogfood confirms planned=5, found=5 with no missing, skipped or stub features. Proofs: phase11-core-samples/approved-leaves.json and phase11-dogfood-final.json.

Built source behavior: typed geography/station and cuisine resolution; highest-rated native search; source-supported lunch/dinner average-price brackets; bounded native pagination; independent card and detail parsers; distinct displayed/review budgets; nearest-station distance; lifecycle warnings; compact items+meta output and full-record projection. Source failures, unrecognized HTML, effective-constraint drift and response caps fail explicitly. Parsed source responses replace cache only after validation. Listing refreshes never downgrade or redate a saved full-detail snapshot.

Built saved-list behavior: persisted notebooks and notes; offline factual comparison; bounded detail refresh with changes/new evidence and per-ID failures; constrained alternatives with unknown/unmatched counts; missing-evidence/freshness audit. Normalized snapshots replace atomically, notes/membership remain separate, and failed refreshes retain valid saved facts. Storage retention bounds unsaved snapshots at16MiB and preserves list members.

Source implementation files are internal/source, internal/domain, internal/cli/tabelog_{helpers,discovery,raw_suggest}.go. List implementation files are internal/notebook and internal/cli/tabelog_lists{,_refresh,_test}.go. Original scaffold constructors delegate to the real implementations. Generic sync/search/analytics are hidden from the product surface; unsupported raw-store MCP search/sql/context registrations are removed. Optional stdio MCP exposes domain commands and applicable saved-list recipe intents.

Generation passed tidy, tests, vulnerability check, vet, build, help/version/doctor. Runtime uses standard Go HTTP, no browser/TLS transport dependencies, learning disabled, lazy SQLite, no background process. Whole-command contexts honor --timeout; bodies, requests, pages, cache and refresh concurrency are bounded. Private caches strip captured CSRF and third-party asset credentials; artifact sanitation provenance is in source-artifact-sanitization.json.

Validation: CLI/source/notebook/domain and MCP package tests passed; independent real-source parser oracles and executable replay cover exact facts, null/unknown evidence, budget constraints, pagination, relocation and7-digit IDs, invalid input before I/O, source blocks/drift/non2xx/oversize/redirects, cache preservation, notes/list persistence, audit and partial refresh. Final executable replay is recorded in e2e-results.jsonl. Priority1 help/dry-run/JSON samples passed for find/areas/cuisines, including one actual anonymous Tokyo listing request returning5 of20 scanned with0 detail fan-out; catalogue samples use0HTTP.

Actual20-sample measurements (measurement-results.json, o200k_base): find5 cold p95=1188tokens/3619bytes/24.634ms/10msCPU/~27.3MiB RSS; warm p95=21.029ms with0HTTP. Show p95=715tokens/2548bytes/17.164ms; warm13.585ms with0HTTP. All requested token/byte/CPU/RSS/request/warm-latency checks pass. Summary sharing inherits matching fetched_at/source_surface/budget_source from meta without losing facts; explicit --select reads full records.

No approved shipping feature was deferred; no skipped request bodies remain in scope. Inventoried rich TUI, booking availability, bulk crawls, review/menu/image collection and legacy/paid wrappers remain outside the user-approved manifest.

Generator limitations: dogfood still warns about five unused generated helpers and generic defaultSyncResources despite the custom normalized notebook population path. Bundled internalJSON spec is mechanically parsed as OpenAPI and emits harmless no-server/root-mcp warnings even though standardHTTP replay and canonical URLs work. These framework warnings do not represent missing source or saved-list behavior. Dogfood rewrites concise documentation sections, so the latest root-authored README/SKILL are restored after the final mutating gate.

Phase12 product corrections: dry-run with --select now preserves the planned envelope and records the intended selection rather than projecting absent restaurant facts. The actual executable no-I/O regression passes (phase12-projection-dry-run-regression.jsonl). List Use metadata now uses conventional angle-bracket positional labels, allowing static docs checks to find the real note flag and argument counts. Strict narrative11 examples and all five noncanonical skill checks pass. The first umbrella reports score83/A; a publication-only canonical install assumption is being repaired in a run-local Press copy with independent focused tests.

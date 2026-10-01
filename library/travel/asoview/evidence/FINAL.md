# Final evidence — Asoview

Outcome: verified buildable read-only CLI, approved focused scope complete; local promotion completed. No publishing, PR, account credential, purchase, booking, ticket hold or other-provider integration.

## Canonical paths
- Editable project: <local-home>/Projects/Personal/Coding/asoview-cli
- Canonical staging: <local-home>/printing-press/.runstate/asoview-cli-89a1800b/runs/20261001-020600-ab3a746a/working/asoview-pp-cli
- Local library: <local-home>/printing-press/library/asoview
- Manuscripts: <local-home>/printing-press/manuscripts/asoview/20261001-020600-ab3a746a
- Sequencing receipts (disposable, not archived): <local-home>/printing-press/.runstate/asoview-cli-89a1800b/runs/20261001-020600-ab3a746a/pipeline/phase-receipts.jsonl
- Selected Press binary: <local-home>/go/bin/cli-printing-press (4.32.5); Go 1.27.1, darwin/arm64.

## Shipped coverage
Discover/search supports first-party native region/category/date/party filters plus honest bounded local substring relevance, source-matched cards (recommendations excluded), pagination and dotted field projection. Product exposes exact Japanese names/IDs, advertised age bands, inclusions, cancellation variants, validity/sales periods, category/location and canonical URLs. Options lazily exposes dated public bands and units; computed party subtotal is explicitly unconfirmed and excludes checkout-specific adjustments. Availability exposes public dated calendars/slots, quantity checks and source signals; general admission validity never becomes a fabricated reservation. Compare and handoff support bounded review and human completion. Inventory has bundled first-party data and persistent explicit refresh.

Runtime is anonymous, allowlisted, paced sequential HTTP with counted redirects, bounded retries, total context deadlines, encoded/decoded body limits, rooted bounded cache reads and bounded disk cache. Ordinary commands need no browser. CLI and MCP Go sources build. Only www.asoview.com is integrated; Japanese is the verified language surface.

## Verification
- Final canonical Press shipcheck: PASS, all seven legs exit0. Structural verify: 20/20 (mock mode, explicitly not live source correctness). Scorecard: 80/100, grade A.
- Binary-owned full live matrix: 66/66 passed, status pass. 39 skipped/unverified framework rows are retained in dogfood-results.json, not claimed exercised. Business happy-path annotations use real public IDs/dates; mutation/framework fixture branches remain unapproved/skipped.
- Marker source fingerprint: d8fbf1d3c6d12a25fe085af5c2c9759a91b2541c0853a868dbfe17114a67cf8b; written by the Press runner, never hand-authored or edited.
- Public-source semantic runner: 21 live checks PASS at 2026-09-30T20:02:11.736961+00:00. No fixtures used. Exact source terms, cancellations, prices/units, age labels, native filters, relevance/negative query, cursor continuity, general validity, dated band subtotal, provenance, projection and typed failures were asserted. Stored samples redact only a published provider contact; live comparisons use full actual response values.
- Independent fresh-context code/doc/output review: exactly one gpt-6.1-sol/xhigh reviewer, fork_turns none, reused for fixes. Final PASS; six findings and compression subcase closed. Synthetic independent probes are labeled separately in independent-review.md.
- Deterministic domain regressions and full generated suite pass; go vet and go build ./... pass. Test listener restriction was resolved by authorized local-loopback execution; failures were not treated as passes.
- Gosec v2.26.1: zero unresolved provider/custom findings. Rooted bounded cache reads replaced two custom warnings. Twenty generator-framework findings remain separately triaged as retro candidates in security-triage.json; no unrelated upstream writes.
- MCP tools audit: no pending findings (one accurately named generated profile-list Short accepted); description override survives MCP sync. PII gate: public provider term contact in a generated workflow proof is provenance-qualified; stored public samples otherwise redact it. No customer or session data was captured.

## Measured output and runtime
Actual source reads, not fixtures. macOS /usr/bin/time -l measures each process peak RSS. Each cached measurement uses an explicitly primed fresh cache; uncached uses --no-cache. Values are one representative run, affected by source/CDN/network conditions.

| Command | Uncached bytes / requests / ms / peak MiB | Cached bytes / requests / ms / peak MiB |
|---|---|---|
| discovery | 4082 / 1 / 42.0 / 26.7 | 4076 / 0 / 13.9 / 22.3 |
| product | 7180 / 1 / 42.0 / 24.1 | 7175 / 0 / 11.6 / 20.4 |
| availability | 3699 / 3 / 912.8 / 24.1 | 3691 / 0 / 22.7 / 20.2 |
| dated_options | 3783 / 3 / 873.0 / 24.0 | 3775 / 0 / 22.7 / 20.5 |
| comparison | 11913 / 2 / 338.0 / 24.6 | 11905 / 0 / 23.1 / 21.0 |

## Coverage limits
This is an unofficial public website contract. Authenticated account/history/checkout facts are outside scope; no paid account is silently required. General tickets without a public dated band/slot surface report explicit unknown coverage. Legacy zero maximum quantities do not establish a cap; ambiguous windows remain unclassified without an explicit time-ticket schedule ID. Product age limits may conflict with dated band labels; both are preserved. Other first-party languages and exhaustive municipal taxonomy are unverified. Calendar/fee/source schema changes fail predictably instead of becoming empty stock. Source crawler restrictions are recorded; requests are explicit and bounded, with no catalog crawling.

## Evidence index
- research.md and scope.md: public access, costs/auth, selected scope and source contracts.
- live-source-checks.json and live/: assertion summary and normalized public source samples.
- benchmark.json: output size, request counts, cached/uncached latency and per-process peak memory.
- independent-review.md and implementation.diff: independent findings/verification and implementation changes.
- tests.log, vet.log, review-fix-tests.log, gosec-after.json, security-triage.json: local checks and static-analysis triage.
- shipcheck.json, dogfood-results.json and phase5-acceptance.json: Press-owned checks and source-bound live acceptance.
- README.md, SKILL.md, AGENTS.md, .printing-press-patches/: usage, semantics and durable reprint guards.

User scope overrides were explicit: sole builder/no novelty or implementation workers; one independent reviewer reused across review phases; ordinary brief/scope/depth gates preauthorized; global skill/config updates skipped; local source build instead of current registry installation. Canonical Press install text is conditional future-release reference only. User explicitly chose no publishing and no PR; next-steps choice is local completion.

## Promotion closure
Press lock promote returned promoted=true at <local-home>/printing-press/library/asoview; lock released. Staging, library and editable-project Go/module bytes were verified identical. Durable manuscripts were archived at the canonical path above; mutable caches and phase-receipt ledger stay run-scoped. No source change followed the passing live acceptance marker.

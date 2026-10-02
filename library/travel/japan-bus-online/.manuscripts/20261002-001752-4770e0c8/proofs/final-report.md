# Japan Bus Online: final build report

Status: complete, locally promoted; publication was outside the authorized scope. This report supersedes stale maintenance/pending notices in MAX-HANDOFF.md and historical proofs. Run 20261002-001752-4770e0c8.

All five supported public read-only workflows are implemented: route search/list, directional published timetables and stops, dated service inventory, selected-stop Adult/Child party fare/capacity evidence, and operator baggage/boarding/cancellation conditions with canonical booking handoff. Source access is anonymous English HTTPS GET through standard Go HTTP with memory-only cookies, shared pacing, typed rate errors, context/timeouts, bounded bodies and provider-bound HTTPS redirects. The CLI requires no browser at runtime.

## Delivery

- Buildable source: `<source-tree>`
- Promoted isolated library: `<local-library>`
- Host CLI: `<local-library>/build/japan-bus-online-pp-cli`
- Host MCP: `<local-library>/build/japan-bus-online-pp-mcp`
- Verified Darwin arm64 MCP bundle: `<local-library>/build/japan-bus-online-pp-mcp-darwin-arm64.mcpb`
- Current proofs: `<press-home>/.runstate/japan-bus-online-cli-145c42c7/runs/20261002-001752-4770e0c8/proofs`
- Embedded manuscripts: source/library `.manuscripts/20261002-001752-4770e0c8`
- Archived manuscripts: `<manuscript-archive>/japan-bus-online/20261002-001752-4770e0c8`
- Disposable receipt ledger: `<press-home>/.runstate/japan-bus-online-cli-145c42c7/runs/20261002-001752-4770e0c8/pipeline/phase-receipts.jsonl` (kept out of manuscripts)

The actual `cli-printing-press lock promote` completed its source-bound acceptance/PII/patch gates and atomic swap, and released the lock. The promoter refreshed host staged binaries and the MCP bundle. The bundle's manifest and both executables match the delivered host files byte-for-byte. All 161 compared Go/module source files match the reviewed workspace after promotion. No Go behavior changed during closure. The generic promotion manifest writer re-emitted its historical one-endpoint descriptor; the existing independently reviewed artifact refresh restored the actual 23-tool catalog and metadata, including exactly five provider tools. Fresh actual stdio descriptors match the reviewed final catalog in full; all five provider tools retain read-only/non-destructive hints. The complete protected SKILL Install section remains exactly 1159 bytes.

## Verification

- Canonical current-source shipcheck: PASS, all seven binary-owned legs exit 0 (`proofs/shipcheck-current.json`).
- Structural verify: 100% (22/22); canonical-equivalent Steinberger score: 80/100, grade A, with three genuinely evaluated live feature checks passing and zero failing (`scorecard-current.json`). The preserved score of 64 came from omitting verify evidence and is historical diagnostic output.
- Required full live dogfood: PASS, 92 executable checks, zero failures, 84 skips/unverified rows; genuine binary-owned full marker `phase5-acceptance.json`, fingerprint `b617bedaa3091e546a774f795bbf3ee2d48a72bd4a3d976e009db6e2e3b5ff8f`. Skips are explicit, including six optional framework candidate-ID fixture rows; provider coverage passed.
- Final core test/vet and fresh host builds pass; independent same-reviewer overlays and affected tests pass. No additional reviewer was created for receipt closure.
- Sole dedicated fresh-context gpt-6.1-sol MAX reviewer converged at round 3 and independently confirmed later help/protected-section refinements (`evidence/reviewer-round3.md`).
- Tool audit: zero pending findings, four resolved, two individually accepted generator-owned brief framework descriptions.
- Security scan: 39 raw generator/reserved baseline observations, zero introduced provider findings and zero post-triage unresolved findings (`security-post-help-triage.json`).
- Strict source/run PII gate was clean; final promoted/archive audits and secret containment checks are recorded separately.

After native Chrome independently confirmed maintenance had ended, the strict six-sample collector passed actual route, service, both quote pairs and conditions checks. The promoted final CLI was additionally verified against the actual public source at 2026-10-02T01:29:32Z: course 12200160001, direction 0, service 0001, requested/effective service day 2026-10-10, stops 8 to 9. Two adults plus one child: Adult 6100 JPY, Child 3050 JPY, total 15250 JPY; boarding 2026-10-11T01:00:00+09:00, alighting 2026-10-11T06:00:00+09:00; source capacity display 6 treated as a lower bound and max 4 tickets per transaction. Fresh delivery call: 6 upstream GETs, 3533 output bytes, 5144 ms, 27623424 bytes peak RSS. Full collector maxima were 9.2 KB displayed output, six requests and 38.8 MB observed RSS. Earlier restricted sandbox DNS failed locally; the authorized public read-only call succeeded outside that restriction, so this was not a provider access blocker.

## Practical limits

English is the verified anonymous provider surface. Published schedules do not prove dated inventory. Service dates and original route/direction/service/plan/stop identities are preserved, including JST 24+ overnight time semantics. Unit Adult/Child fares support a one-way arithmetic estimate for selected stops; Child is the provider's 6–12 label. Positive numeric seat displays are lower bounds, zero is sold out, and missing/overflow counts are unknown. Transaction limits are independent of capacity. Adjacency, gender-plan feasibility, age/discount/group/roundtrip eligibility and final booking confirmation remain unknown unless the source explicitly supplies them. Inventory and fares are snapshots and may change. Cancellation details can return labeled partial failures. Japanese names are null where absent, rather than invented.

The optional framework learning DB is distinct from provider inventory; provider sync/local-cache reads are unavailable. `workflow-verify` had no workflow manifest; actual primary behavior is covered by the real provider matrix and strict collector. Six blocked optional framework candidate confirm/reject rows were not exercised as candidate lifecycles and remain accurately recorded. All supported agreed provider scope shipped; these do not imply unsupported booking operations.

Native built-in Chrome was used for source discovery and independent capacity/reopening checks. No Playwright/browser fallback was required. No booking, payment, passenger form submission, terms acceptance, account operation, outbound message, GitHub write or publication occurred. Research browser tabs and owned sessions were closed. Only dense public research/proofs and the sniff summary are embedded/archived; no raw HAR, cookie/session state, credential scratch or receipt ledger is included. The closing receipts record Done/local only, honoring the user's existing authority. Actual final status verification confirms receipt sequence 39, phase 21 completed, next `done`; the build lock is released and the library is recognized.

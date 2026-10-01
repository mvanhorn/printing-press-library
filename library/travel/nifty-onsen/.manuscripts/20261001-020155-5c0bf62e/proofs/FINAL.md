# Nifty Onsen final evidence

Result: verified, buildable read-only source completed in this workspace. All approved provider workflows and all 5 planned agent capabilities are implemented. Local promotion destination: `[local library]` (native promotion receipt is `evidence/promotion.json`). No publishing, PRs, purchases, reservations, payments or account changes.

## Canonical paths

- Workspace/source: `[local project]`
- CLI: `[local project]/evidence/bin/nifty-onsen-pp-cli`
- MCP: `[local project]/evidence/bin/nifty-onsen-pp-mcp`
- Run state: `[private runstate]/state.json`
- Receipt ledger: `[private runstate]/pipeline/phase-receipts.jsonl`
- Native live acceptance: `[private runstate]/proofs/phase5-acceptance.json`
- Archived manuscripts: `[local archive]`
- Independent review: `[local project]/evidence/independent-review.md`

## Recovery and scope

Recovered the saved implementation-phase run after confirming its recorded owner PID 49831 was absent. Supported Press lock acquire recovered stale ownership without force. Existing research, receipts, scaffold, model/catalog and release files were preserved; no regeneration/reset, other CLI build or shared/global configuration modification occurred. The unsupported generated tail template remains byte-identical in `internal/cli/tail.go.disabled` and the original saved scaffold; it is outside the active command surface.

Shipped commands: `bath search`, `bath nearby`, `bath show`, `bath coupons`, `bath compare`, `regions`, `filters`. Search/nearby default to source day-use classification; detail/coupon reads are lazy. Search is one organic page (default 10, cap 30); nearby is one map window (cap 20), then local distance filtering; comparison is 2–5 IDs. Output is compact `{meta,results}` agent JSON with field projection, explicit nulls, source IDs/URLs/Japanese names, coverage, freshness and actionable errors.

Domain assertions preserve natural hot spring vs ordinary bath, room vs private bath, source day-use vs stay, Japanese weekday/weekend/holiday/age/fee basis/extras and coupon eligibility. Modern default coupon details include one-person and 24-hour reuse terms. Tattoo, children/accessibility and reservable capacity are unknown without explicit facility evidence. A generic bath/room or family tag does not establish rentable inventory.

## Verification

- Final canonical shipcheck: **PASS**, all seven legs (`evidence/shipcheck.json`). Native mock verify **16/16, 100%, 0 critical**; fixture verification is distinct from source verification.
- Native full live matrix: **54 pass, 0 fail, 41 safe skips**, no hollow feature coverage (`evidence/dogfood-live.json`). Every networked bath command happy path ran; measured native requests: search 1, show 2, coupons 3, nearby 2, compare 4. Skips are unrelated administrative mutations/framework probes and absent optional fixture inputs.
- Content-checking E2E: **24 pass** across Tokyo/Osaka/Hokkaido, pagination, zero-match query, source filters, spa/sento/hotel, fees/hours/policies, public coupon restrictions, nearby order/radius, compare, projection, cache and invalid input (`evidence/e2e/summary.json`).
- Actual MCP stdio: **8 provider tools, 3 live parsed calls**, source-shaped schema and domain output assertions (`evidence/mcp-smoke.json`). MCP mirrors the real provider CLI; generic HTML/SQL/sync inventory handlers are not exposed.
- Fresh `go test -count=1 ./...` and `go vet ./...`: **PASS** (`evidence/tests-final.log`, `evidence/vet-final.log`). Deterministic tests cover consequential parsing, identity, source-only claims, coupon terms, cache states/bounds/throttling, projected stale comparison provenance and forcing live reads in the live harness.
- Exactly **one** independent fresh-context reviewer: `gpt-6.1-sol`, `xhigh`, `fork_turns=none`; final PASS after fixes and follow-ups, with source/proof fingerprints. Builder owned all research, implementation, tests and fixes.
- Scorecard: **80/100**; 5/5 novel live samples pass (`evidence/scorecard.json`). Source limits and inherited scaffold warnings are not disguised as live capability.
- Tools audit: zero pending, one precise generated helper description accepted. PII audit: zero phase-1 findings. Pinned gosec v2.26.1: **zero authored provider findings**, with **22 inherited generated/reserved findings** explicitly triaged; raw scanner exits nonzero for those (`evidence/gosec-final.json`, `evidence/security-and-scaffold-notes.md`). This is not a claim of a wholly clean tree-wide security scan.

## Measured runtime

Single representative sample per workflow/mode. Search/nearby return 5 rows, coupons 3, detail one. Wall latency includes startup; macOS `/usr/bin/time -l` supplies peak RSS; requests count redirects. Token estimates in `evidence/metrics.json` are bytes/4, not a model tokenizer.

| Workflow | Mode | JSON bytes | Requests | Latency ms | Peak RSS MiB |
|---|---|---:|---:|---:|---:|
| search | uncached | 3177 | 1 | 1947.5 | 28.56 |
| search | cached | 3177 | 0 | 14.2 | 18.16 |
| detail | uncached | 5185 | 2 | 1416.0 | 26.59 |
| detail | cached | 5185 | 0 | 18.2 | 18.39 |
| coupons | uncached | 5291 | 3 | 1848.7 | 29.31 |
| coupons | cached | 5291 | 0 | 13.0 | 18.47 |
| nearby | uncached | 4064 | 2 | 835.8 | 24.27 |
| nearby | cached | 4064 | 0 | 14.5 | 18.47 |

Runtime bounds: sequential requests, default pacing 2/s, 20-second HTTP timeout plus command timeout, 4 MiB response cap, one bounded 429 retry, parsed cache 128 entries / 32 MiB. Cache v2 invalidates pre-fix parses. Cookie/query context stays memory-only; raw HTML and ephemeral context captures were removed after inspection.

## Practical limits

This is an unofficial one-provider public website integration. Source classifications, layouts, schedules and offers can change. Nearby coverage is a finite source window, not complete radius inventory. Minimum fees are hints, not payable quotes. Coupons and listings do not establish eligibility, acceptance, bookable sessions or capacity. No current-crowding, bulk review corpus, coupon issuance/redemption or account workflow was verified or offered. Offline results disclose age/staleness; computed catalogs disclose their verification date. Links hand off to source/official information only.

No provider access blocker remains for the approved public workflows. Fixture data, native mock verification and actual live read evidence are identified separately above. Release remains `0.0.0-dev`; release ledger/version stamping was not hand-edited.

## Native completion

The acceptance marker below is generated by Press and bound to the final source; it was never manually edited.
`source_fingerprint`: `2248ab5803ec8768c828bc2b967aa1abf5531886d04aa722073ba71a39fdf48b`

Local promotion and final receipt closure are recorded in `evidence/promotion.json`, `evidence/receipt-20.json` and `evidence/receipt-21.json`. The workspace remains the user-facing source copy; local library promotion and manuscript archiving are not public publication.

Confirmed completion: native local promotion `promoted: true`; workspace/library Go source hashes match; receipt ledger ends at `21-next-steps / completed`; build lock is released. Final marker and review evidence are archived at the canonical paths above.

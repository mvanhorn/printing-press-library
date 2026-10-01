# OMAKASE CLI final evidence

Outcome: supported anonymous public scope implemented, verified and locally promoted; full Press live acceptance PASS. Printing Press confirmed promotion and verified the source-bound acceptance marker. No bookings, payments, purchases, account login, publishing, PRs or shared configuration changes.

## Canonical paths
- Editable/buildable project: <project>
- Canonical Printing Press staging: <run-state>/working/omakase-pp-cli
- Local library: <local-library>/omakase
- Archived run evidence: <manuscripts>/omakase/20261001-020555-6d97b738
- Receipt ledger (disposable run state): <run-state>/pipeline/phase-receipts.jsonl
- Isolated Press binary used for final checks: <run-state>/tools/cli-printing-press

## Shipped behavior
First-party OMAKASE English/Japanese public discovery, literal name filtering after fuzzy source search, filters and pagination; lazy restaurant/course detail and Japanese names; JPY price floor/range/tax/included-service/variable semantics; separate service/reservation charges; cancellation rows and advance-payment caveats; source release states, dated JST timestamps and frequency rules; request/waitlist/lottery distinction; bounded comparison retaining course fee notes; explicit atomic summary inventory refresh and local lookup; public Premium features/prices and unknown account eligibility; canonical booking handoff links.

Exact date/party seats are unverified and inaccessible on the tested anonymous surfaces. `availability` reports unknown/null with query_evaluated:false and source access/action evidence. Amamoto explicitly requires login; the provider-linked ume reservation entry returned HTTP 403 challenge in Go and Chrome. Advanced seat search/labels and release calendars are Premium features; Gold is invitation-only. No paid account is silently required or acquired.

## Verification
- Full Go suite and go vet passed; buildable CLI and MCP.
- Consequential deterministic parsing/state, en/ja financial, capped transport, normalized cache-version and atomic inventory regressions passed. Synthetic tests are labeled tests and never represented as live evidence.
- Custom live source-correctness matrix: 27 checks passed. Public source examples include Sugita literal-name relevance, Amamoto English/Japanese course/fee/cancellation details, ume dated release, membership prices, source pagination, inventory refresh/find/status, field projection, offline reads and predictable error exits. See metrics.json and live/.
- Press shipcheck: all seven legs passed; verify 100%; scorecard 82/100. See shipcheck-final.json. After matrix help/dry-run fixes, verify-skill again reports zero findings.
- Full binary-owned live dogfood: 128/128 executed checks passed, zero failures; 93 explicitly skipped/unverified harness rows are not counted as passes. No hollow approved features. Acceptance JSON was written by the Press runner and is source-fingerprint-bound, never hand-authored or edited.
- Independent review: exactly one gpt-6.1-sol/xhigh reviewer with no inherited conversation. Same reviewer rechecked fixes. All original and follow-up findings resolved; final code/docs/live output/help/dry-run review clean. See review-final.md.
- Pinned gosec: zero unresolved findings in hand-authored OMAKASE code. Remaining generated-framework scanner findings are unchanged baseline/template candidates, documented in gosec-final.json; no shared generator writes or issue publishing.
- tools-audit: zero pending, two precise optional generated local-list descriptions accepted. pii-audit strict: zero findings. No credentials used. Raw HTML/CSRF/session data are not persisted; generated metadata endpoints use the same GET/origin/body/cache boundary through a preserved hook.

## Resource measurements
Measured actual source commands via macOS /usr/bin/time -l; no fixtures. See performance.md/metrics.json for all 18 cold/warm rows.
- Search: cold 835 output bytes, 1 request, 2496.1 ms, 23.75 MiB peak; warm 827 bytes, 0 requests, 19.0 ms, 18.17 MiB.
- Detail including Japanese name: cold 3232 bytes, 2 requests, 1116.2 ms, 23.78 MiB; warm 3224 bytes, 0 requests, 20.4 ms, 18.30 MiB.
- HTTP concurrency one, pacing 500 ms by default, at most one retry for 429/5xx, 15-second per-request and 60-second default whole-command timeout, 2 MiB responses, 128 normalized documents, discovery limit 10/max 50, pages max 25. Inventory refresh is always explicit.

## Toolchain and scope decisions
Initial installed Press v4.32.5 lacked truthful local-module skill-install validation. An existing vetted local-install v4.32.5 binary was copied to this isolated run tools directory for final checks; no global binary/config/source changed. Sole builder performed all research/planning/implementation/polish. User explicitly limited delegation to one independent reviewer, overriding skill brainstorming/multi-review/forked-polish worker defaults. Ordinary briefing/absorb gates and local promotion were preauthorized; no routine questions or parent polling.

## Evidence index
research.md; requirements.md; acceptance.md; review-round1.md; review-final.md; tests-final.log; vet-final.log; dogfood-fix-tests.log; shipcheck-final.json; verify-skill-final.json; scorecard-live.json; live-dogfood-final.json; phase5-acceptance.json; metrics.json; performance.md; gosec-final.json; tools-final.txt; pii-final.json; toolchain.md; customizations.json.

Promotion follow-up: byte-identical verified source confirmed in library; promoted binary offline smoke returned source ID hc778124 and JPY 52,800 course amount. Receipt ledger closed at sequence 39, 21-next-steps → done. See promotion.json, promoted-smoke.json and receipt-status.json.

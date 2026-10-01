# eplus-cli final evidence

Status: COMPLETE — verified source and binaries, supported local promotion succeeded, sanitized manuscripts archived; receipt ledger closes at 21-next-steps. No publishing, PRs, purchases, reservations, payments or account changes.

## Canonical deliverables

- Workspace/source: `<cli-dir>`
- CLI: `<cli-dir>/build/stage/bin/eplus-pp-cli`
- MCP: `<cli-dir>/build/stage/bin/eplus-pp-mcp`
- Bundle: `<cli-dir>/build/eplus-pp-mcp-darwin-arm64.mcpb`
- Local library target: `<local-library>/eplus`
- Run/state: `<run-dir>/state.json`
- Receipt ledger: `<run-dir>/pipeline/phase-receipts.jsonl`
- Acceptance marker: `<run-dir>/proofs/phase5-acceptance.json`
- Manuscripts target: `<archived-manuscripts>`

The initial project had no `.press/.press-run.json`; the recovered `state.json`, generated manifest run ID and receipt ledger identify the run. Prior owner PID 49044 was absent; Press reported a stale lock and supported `lock acquire` recovered it without force. Existing source/research/scaffold were preserved at `<run-dir>/resume-baseline` and in the original research directory. Other CLI builds and shared/global configuration were untouched.

## Delivered behavior

Domestic keyword/artist/date/category discovery, explicit bounded region/venue/location matching, lazy event/performance detail and sale-round booking handoff. International catalog/tour/product discovery remains a separate offering; inspect/compare exposes event-specific prices and conditions. Two-to-four-ID comparisons preserve failures and session identity. Public FAQ guidance is read live.

Canonical parent event IDs come from returned detail URLs; raw event/sub/performance/round codes remain available. Lottery acceptance does not imply available inventory. General sale/presale and lottery/first-come/request are distinct fields. Exact JST doors/start and sale deadlines survive compact JSON. Explicit overnight clock normalization preserves the service date. Ambiguous identical performances fail instead of receiving duplicated source identity. Unknown public fees/eligibility/collection remain unknown. International static JPY 0 is a placeholder; the live selected-variant GET supplies the actual JPY 7,300 example price. A domestic listing never establishes overseas bookability.

HTTP: serial reads, capped at two requests/second including redirects, 32 attempts/command, three attempts/request, four redirect hops, 4 MiB/body, 20-second default command deadline (explicit up to 120 seconds; request max 20 seconds). URL-checked 256-slot cache lasts two minutes and retains original observation time; fresh/no-cache/live controls bypass it. No resident browser, offline mirror or resolved credentials. Diagnostics are stderr; expected public-source limitations are JSON metadata.

## Verification

- Canonical Press shipcheck: all seven legs PASS, `evidence/shipcheck.json`.
- Runtime verify: 93.75% WARN; noncritical generated-framework warning remains, explicitly not 100%. Scorecard 76/100 B; live planning sample 1/1.
- Full Press live matrix: 42/42 mandatory PASS, 0 failed; 32 optional/skipped probes remain unverified with reasons in `evidence/live-dogfood.json`. Provider happy paths use real public GETs.
- Additional live CLI E2E: 21/21 assertions, `evidence/e2e.log`; real outputs in `evidence/live/`.
- Actual MCP calls: canonical search/detail/international detail PASS; six discovery tools have full schemas and read-only hints. Rebuilt runtime exposes nine tools. `evidence/mcp-e2e.log`, `evidence/mcp-tools.json`. Generated typed manifest count covers two spec tools; runtime inventory is the authority for the full surface.
- Full `go test -count=1 ./...` and `go vet ./...`: PASS, `evidence/go-test.log`, `evidence/go-vet.log`.
- Deterministic tests cover date/overnight/window parsing, source identity/ambiguity, sale state/phase, URL validation, filters, actual option markup, selected price/closed precedence, typed throttling, redirects/rate budget, cache/body limits, cancellation and projection. These fixtures are not current inventory evidence.
- Exactly one fresh independent reviewer, `gpt-6.1-sol`, effort `xhigh`, no inherited context. Five consequential findings fixed; same-context current-source recheck PASS with hashes and independent live checks. `evidence/INDEPENDENT_REVIEW.md`.
- Security: reachable vulnerability scan found none. Gosec has zero unresolved findings in hand-authored source; 22 generator-emitted findings are accurately retained in `evidence/GENERATOR_FINDINGS.md` and `evidence/gosec.json`. Strict PII gate has no findings; tools audit has no pending findings (one precise generated framework description accepted).

## Measured runtime

Actual macOS process wall time and peak RSS from `/usr/bin/time -l`; body/request stats come from the client. Cold rows bypass the response cache; cached rows reuse the same URL within TTL. Single observations, not statistical latency guarantees. Full data: `evidence/e2e-measurements.json`; reproducible scripts: `evidence/run_e2e.py`, `evidence/run_mcp_e2e.py`.

| Run | Output bytes | Requests | Wall ms | Peak RSS MiB |
|---|---:|---:|---:|---:|
| domestic-cold | 5866 | 1 | 619.89 | 24.11 |
| domestic-cached | 5858 | 0 | 11.5 | 20.38 |
| domestic-pagination | 5072 | 3 | 1895.56 | 26.38 |
| international-product-cold | 4854 | 2 | 4515.82 | 24.25 |
| international-product-cached | 4845 | 0 | 21.72 | 20.08 |
| projection | 4152 | 0 | 20.61 | 20.47 |

Binary sizes: CLI 13.71 MiB; MCP 16.62 MiB. Generated Go runtime retained; observed RSS is independent of file size.

## Actual coverage and limits

Anonymous CLI GETs to both providers succeed from the permitted network context. The sandbox's default DNS restriction caused early doctor/build-tool fetch failures; that is an environment restriction, not an eplus access blocker. No provider access blocker is currently fabricated or outstanding.

Domestic initial search data can contain other regions, so the CLI enforces region membership over bounded scanned records. International filters load only bounded tour schedules; streaming/archive ranges are source date ranges. Some categories are genuinely empty; empty bounded results do not prove universal absence. Public SSR lacks some final checkout fees, residency/phone/payment and collection details; select the product/performance and check its booking page. International eligibility is conditional and current inventory can change after observation. No live cancellation/overnight/429 case was claimed; those consequential state cases are deterministic fixtures.

Primary official sources: [domestic search](https://eplus.jp/sf/search), [international concert catalog](https://ib.eplus.jp/concert), [international FAQ](https://ib.eplus.jp/faq), [sample domestic performance](https://eplus.jp/sf/detail/4592490001-P0030001P021003), [sample international product](https://ib.eplus.jp/index.php?dispatch=products.view&product_id=7078&date=0). Raw historical research is retained run-scoped; archived copies remove website tokens, HAR auth/cookies and response bodies.

Local customization guard: `.printing-press-patches/public-event-discovery-preserves-source-semantics.json`, including narrow generated registration calls. Runtime version remains unstamped `0.0.0-dev`; no release bookkeeping was hand-bumped.

## Closure evidence

Supported `lock promote` returned `promoted: true` and released the eplus lock. All Go/dependency bytes in the promoted tree match the workspace. Promotion rechecked the source-bound acceptance and customization call-site guards. `evidence/promotion.json` records the canonical library path. Final next-step choice is already explicit in the user request: local completion only; no publishing or PR.

Final live doctor: `reachable (HTML body at /)`, no authentication required, unstamped `0.0.0-dev`. Source/dependency build succeeds after promotion. Original research and disposable receipts remain intact; only the run-scoped ephemeral session scratch is cleaned.

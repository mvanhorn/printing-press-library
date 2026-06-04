# Robinhood Top-4 Checkpoint

Updated: 2026-05-27T19:54:00Z

Status: complete for the Top-4 lane. The broader 12-venue goal is not complete.

## What Is Done

- Personal repo: `<personal-repo>/robinhood-cli`
- PP package: `library/payments/robinhood`
- Deep authenticated browser/API map: 19 surfaces, 217 latest browser routes, 259 merged brokerage/account routes.
- Unified API map: 275 route entries and 266 OpenAPI operations after mixing Robinhood's official Crypto OpenAPI into the browser-backed brokerage/account map.
- Ticker/account surfaces covered include NVDA, AAPL, TSLA, HOOD, SPY, QQQ, NVDA options, portfolio/home, account root, history, settings, documents, transfers, BTC, and markets.
- Documentation source map: `api-map/documentation-sources-2026-05-27.json`
- Unified map artifacts: `api-map/robinhood-routes.json`, `api-map/openapi/robinhood-unified.openapi.json`, `api-map/markdown/robinhood-routes.md`
- Personal CLI is live read/write capable by default with caller-owned `ROBINHOOD_BROKERAGE_TOKEN` or `ROBINHOOD_COOKIE`; `--dry-run` is opt-in.
- Personal CLI also executes Robinhood's official Crypto Trading API with caller-owned `ROBINHOOD_CRYPTO_API_KEY` and `ROBINHOOD_CRYPTO_PRIVATE_KEY_B64`; `crypto execute --dry-run` previews the exact signing path and request without sending.
- Duplicate official Crypto paths are method-aware, so POST order placement is labeled `write-mutate` instead of inheriting the GET order-list risk.
- PP-side brokerage and crypto writes are gated by `ROBINHOOD_PP_ALLOW_WRITES=1`; write operations default to dry-run when not explicitly live.
- MCP tools carry read/write risk metadata, including `mcp:read-only=false` and `mcp:risk` for live-capable executors.
- Personal MCP exposes `robinhood_crypto_plan` and `robinhood_crypto_execute` for the official Crypto API.
- Per-endpoint Markdown docs include `Mutation: yes|no` for all 275 unified route entries, including the 16 Robinhood-published Crypto entries.
- PP upload staging is ready: `publish package --target` produced `publish-staging/robinhood-pp-final-v3-2026-05-27/library/payments/robinhood`, with manuscripts included, no root binaries, and staged `publish validate` passing.

## Evidence

- Deep authenticated Chrome CDP capture: `proofs/cdp-stock-account-deep-sanitized-2026-05-27.json`
- Browser route map: `api-map/browser-cdp-routes-2026-05-27.json`
- Brokerage route map: `api-map/brokerage-routes.json`
- Unified official Crypto plus brokerage/account route map: `api-map/robinhood-routes.json`
- Official/docs/community sweep: `research/documentation-and-community-sweep-2026-05-27.md`
- Personal proof set includes typecheck, tests, build, CLI/MCP smokes, and npm pack dry-runs.
- Crypto execution follow-up proofs: `proofs/personal-crypto-best-bid-ask-dry-run-2026-05-27.json`, `proofs/personal-crypto-order-dry-run-2026-05-27.json`, `proofs/personal-mcp-crypto-exec-tools-2026-05-27.json`, `proofs/personal-test-crypto-exec-2026-05-27.txt`.
- PP recheck proofs: `proofs/pp-brokerage-all-routes-official-crypto-recheck-2026-05-27.json`, `proofs/pp-crypto-order-default-dry-run-recheck-2026-05-27.json`, `proofs/pp-brokerage-write-live-gate-recheck-2026-05-27.txt`.
- PP proof set includes go test/vet, verify, verify-skill, tools-audit, mcp-audit, and shipcheck 6/6.
- Final PP upload proofs: `proofs/pp-publish-validate-final-mirror-2026-05-27.json`, `proofs/pp-publish-package-final-v3-mirror-2026-05-27.json`, `proofs/pp-publish-validate-staged-final-v3b-2026-05-27.json`, `proofs/pp-go-test-staged-final-v3-2026-05-27.txt`.

## Boundary

No live Robinhood mutation was submitted. Browser capture was route observation/navigation only, and write proofs are dry-run/gated. Official Crypto live execution now exists, but using it for orders/cancels still requires an explicit exact user instruction.

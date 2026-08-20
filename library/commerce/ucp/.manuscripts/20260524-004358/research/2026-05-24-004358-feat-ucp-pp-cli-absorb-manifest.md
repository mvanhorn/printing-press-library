# UCP CLI Absorb Manifest

## Transport-layer reality check (drives every command below)
`checkout.coffeecircle.com` — the only public UCP merchant I could reach during research — declares only `transport: mcp` and `transport: embedded`. The reference Python/Node samples use `transport: rest`. **A useful CLI must speak both REST and MCP as a client.** A2A and embedded are deferred.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Fetch and lint `/.well-known/ucp` manifest | nextwaves.com/tools/ucp-tester (web), awesomeucp/ucp-doctor (TS) | `ucp check <domain>` — Go HTTP, JSON-schema validate against spec, A-F readiness grade | Offline-after-fetch, `--json`/`--select`, pipes cleanly, no browser needed |
| 2 | Endpoint compliance audit | davillafer/UCP-Compliance-Checker | `ucp check <domain> --deep` — probes each declared capability endpoint with a HEAD/OPTIONS, reports auth shape | Composable with shell, exit code is typed |
| 3 | Typed UCP models / schema validation | Universal-Commerce-Protocol/python-sdk (Pydantic), js-sdk (Zod) | `internal/schema/` — Go structs generated from the canonical JSON Schemas at `Universal-Commerce-Protocol/ucp` | Native Go types, single binary, no Python/Node runtime |
| 4 | Capability-aware client | OmnixHQ/ucp-client (TS) | `internal/client/` — every subcommand consults cached `merchants.capabilities` before issuing the request; refuses unsupported capabilities with `pp:unsupported-capability` exit code | Works offline against cached capability set |
| 5 | REST transport client | Universal-Commerce-Protocol/samples (Python/Node), ucp.dev/2026-04-08 REST docs | `internal/transport/rest/` — full REST client with `UCP-Agent`, `request-id`, `idempotency-key`, `request-signature` headers, ECDSA-P256 signing | Standard Go `net/http`, retries, structured errors |
| 6 | MCP transport client | UCP spec MCP binding (`mcp.openrpc.json` schemas) | `internal/transport/mcp/` — JSON-RPC over stdio + HTTP transports, dialing merchant MCP endpoints (e.g. `coffeecircle-shop.myshopify.com/api/ucp/mcp`) | Lets the CLI actually talk to live Shopify-hosted merchants |
| 7 | Reference-merchant runner | Universal-Commerce-Protocol/samples (Python/FastAPI + Node/Hono) | `ucp mock serve --port 8080 --variant python\|node` — vendor + spawn the reference servers under `internal/mock/` | One command brings up a test merchant — no Python/Node setup needed; works for `ucp check`, `search`, `cart`, `checkout` end-to-end |
| 8 | Catalog search (one merchant) | UCP `dev.ucp.shopping.catalog.search`, Shopify Catalog MCP | `ucp search "<q>" --merchant <domain>` | `--limit`, `--json`, `--select`, persisted to local FTS5 |
| 9 | Catalog lookup by SKU/GTIN | UCP `dev.ucp.shopping.catalog.lookup` | `ucp products lookup --sku <sku>` / `--gtin <gtin>` | Cached responses, agent-native output |
| 10 | Cart create / update / show | UCP `dev.ucp.shopping.cart`, Shopify Cart MCP | `ucp cart add/list/remove/show` with local mirror | Works offline after creation; PUT-semantics replay |
| 11 | Discount application | UCP `dev.ucp.shopping.discount` extension | `ucp cart apply-discount --code <code>` | Persists in local store, surfaces savings before checkout |
| 12 | Fulfillment option negotiation | UCP `dev.ucp.shopping.fulfillment` extension | `ucp cart fulfillment list/set` | Lists shipping methods returned by merchant, persists selection |
| 13 | Checkout session create + update | UCP `dev.ucp.shopping.checkout`, samples reference flows | `ucp checkout create/update/show` | State machine (`incomplete` → `ready_for_payment` → `ready_for_complete`) tracked in local store |
| 14 | Order status fetch + cancel | UCP `dev.ucp.shopping.order` | `ucp orders get <id>` / `ucp orders cancel <id>` | Persisted history, idempotency-aware |
| 15 | Identity linking via OAuth 2.0 | UCP `identity_linking` capability | `ucp auth link <domain>` — opens OAuth flow, persists token | Per-merchant credential store, encrypted at rest *(stub — requires browser flow; ships as guided-print mode in v1)* |
| 16 | UCP-Agent profile management | UCP spec `UCP-Agent` header | `ucp profile init/show/publish` — generates the agent's own UCP profile (signing keys, capabilities) | Local profile editor + signature verification |
| 17 | Merchant directory mirror | awesomeucp/merchants, Full-Vibe/ucp-ecosystem, homototus/ucp-directory | `ucp merchants list/add/refresh/import` — local SQLite-backed directory | One bulk refresh; filter by capability, region, payment handler |
| 18 | Conformance test fixtures | Universal-Commerce-Protocol/conformance-tests (Python) | `internal/fixtures/` — bundled golden requests/responses | Used by `ucp mock serve` and `ucp conform run` |
| 19 | MCP tool surface (CLI commands as MCP tools) | printing-press default behavior | Every user-facing Cobra command auto-mirrored as an MCP tool via `cobratree` | Claude/Cursor/Codex can drive the CLI as a tool |
| 20 | Doctor / health check | All UCP tools have one | `ucp doctor --json` — SQLite store, mock binary, key merchants reachable, agent profile valid | Cli-side troubleshoot in one shot |

## Transcendence (only possible with our approach)
*(from novel-features subagent, all scored ≥ 7/10 against user-value × Go-feasibility × novelty)*

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|-------------------------|-------|
| T1 | Parallel cross-merchant search with dedup, normalization, multi-factor ranking | `ucp search "wireless headphones" --merchants etsy,wayfair,mock --rank price+ship+conformance` | Go goroutines fan out to N merchants concurrently; SQLite normalizes responses across schemas and dedupes by GTIN / title-hash. Existing tools query one merchant. | 9 |
| T2 | Historical price/availability drift tracking with alerts | `ucp watch product <sku> --threshold 10% --notify` | Every probed price/availability sample persisted with timestamps; FTS5 + windowed SQL detects drift. No existing UCP tool persists history. | 9 |
| T3 | Multi-merchant cart optimizer (split a wish-list across merchants to minimize landed cost) | `ucp cart optimize --items items.json --constraint min-shipments=2` | Concurrent quote fan-out + persistent merchant capability cache + local solver over shipping/discount matrices. Unique. | 9 |
| T4 | Capability matrix diff between merchants OR same merchant over time | `ucp merchants diff <a> <b>` / `ucp merchants diff <a> --since 30d` | Every `ucp check` snapshot is persisted; diffs are pure SQL over capability arrays + manifest hashes. Doctor tools are stateless one-shots. | 8 |
| T5 | AP2 mandate-readiness preflight before invoking the AP2 CLI | `ucp checkout preflight --cart <id> --ap2-dry-run` | Local cart state + cached merchant identity_linking capability + JSON-schema validation of the would-be intent/cart/payment mandates, all offline. | 8 |
| T6 | Conformance test runner against any merchant, emits JUnit + agent-JSON | `ucp conform run <domain> --suite full --report junit,json` | Bundled `ucp mock serve` provides golden fixtures; parallel Go test harness with typed exit codes feeds CI. Compliance-checker only audits endpoint reachability, not behavior. | 8 |
| T7 | Spec-version schema diff + auto-migration of stored manifests when UCP publishes a new version | `ucp spec diff 2026-04-08 2026-07-01 --migrate-local` | Local store of historical manifests + bundled spec versions enables schema-aware migration with reversible SQL. SDKs ship one version; no tool tracks drift across versions. | 7 |
| T8 | Order-status watch loop with webhook receiver + agent-native JSONL event stream | `ucp orders watch --emit jsonl` | Long-running Go process owns local order registry, receives merchant `fulfillment` webhooks, streams JSONL to stdout for agent piping. | 7 |

## Stubs to acknowledge up front
- **Identity linking OAuth flow (#15)** ships as `--print-mode` (prints the URL + state, user pastes the redirect URL back). No headless browser in v1.
- **Webhook receiver (T8)** ships with `--listen :port` requiring user-supplied public URL (ngrok-style) — we don't host a tunnel.
- **Spec-migration (T7)** ships with one historical version pre-bundled (`2026-01-23`) plus current; future versions need a `ucp spec fetch` step.

## Source attribution (for README credits)
- `Universal-Commerce-Protocol/ucp` — protocol spec
- `Universal-Commerce-Protocol/python-sdk`, `js-sdk` — reference model generators
- `Universal-Commerce-Protocol/samples` — bundled mock-server source
- `Universal-Commerce-Protocol/conformance-tests` — fixture inspiration
- `OmnixHQ/ucp-client` — capability-aware client pattern
- `awesomeucp/ucp-doctor`, `davillafer/UCP-Compliance-Checker` — `ucp check` heuristics
- `nextwaves.com/tools/ucp-tester` — A-F grading rubric
- `awesomeucp/merchants`, `Full-Vibe/ucp-ecosystem`, `homototus/ucp-directory` — merchant directory seed

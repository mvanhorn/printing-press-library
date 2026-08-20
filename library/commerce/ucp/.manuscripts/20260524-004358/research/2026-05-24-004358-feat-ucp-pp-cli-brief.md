# UCP CLI Brief

## API Identity
- **Domain:** Universal Commerce Protocol (UCP) — open standard co-developed by Google + Shopify, launched Jan 2026, current spec `2026-04-08`. Defines how AI agents discover, browse, build carts, and complete checkout against any UCP-compliant merchant.
- **Users:** AI agent builders (Claude Code / Codex / Gemini agents), commerce-tool developers, merchants validating their own `/.well-known/ucp` endpoints, researchers probing the live UCP ecosystem.
- **Data profile:** Per-merchant manifests, capability negotiations, checkout sessions, line items, orders, payment handler configs. Mostly small JSON documents (KB-scale), but session state benefits from local persistence.

## Reachability Risk
- **Medium.** `ucp.dev` itself and `checkout.coffeecircle.com` (cited by `nextwaves.com` as a working example) are publicly reachable for manifest fetches. **Real retailer UCP endpoints (Etsy, Wayfair, etc.) are gated to approved Google/Microsoft AI Mode agents** — direct probes from an unapproved CLI will likely return 403 or `unauthorized agent`. Mitigation: ship a local mock-merchant runner that uses `Universal-Commerce-Protocol/samples` reference servers (Python/FastAPI + Node/Hono), and lean on the official `Universal-Commerce-Protocol/conformance-tests` fixtures.
- No evidence of widespread bot/Cloudflare blocks against the documented endpoints; auth model is `UCP-Agent: profile="<url>"` header plus signed requests, not API keys.

## Top Workflows
1. **Probe a merchant** — `ucp check <domain>` fetches `/.well-known/ucp`, lints schema, lists supported services/capabilities/transports/payment_handlers, reports readiness grade. (Replaces the nextwaves.com web tool with an offline-capable CLI.)
2. **Discover the live UCP ecosystem** — `ucp merchants list` mirrors a registry of known UCP merchants (seeded from `awesomeucp/merchants` + `Full-Vibe/ucp-ecosystem` + `homototus/ucp-directory`), filterable by capability, region, payment handler, last-checked-at.
3. **Search across UCP merchants** — `ucp search "query"` fans out a `dev.ucp.shopping.catalog` query to one or more merchants in parallel, normalizes results, persists to local store.
4. **Build a cart, then prep checkout** — `ucp cart add/list/remove`, `ucp checkout prep` creates a `dev.ucp.shopping.checkout` session draft with line items, buyer, fulfillment selections, ready for an AP2 CLI (or a manual completion) to authorize.
5. **Track an order** — `ucp orders get/cancel`, plus webhook event ingestion for downstream watchers.

## Table Stakes
Absorbed from competing tools (per the absorb-manifest phase):
- **Manifest discovery + validation** — `nextwaves.com/tools/ucp-tester`, `awesomeucp/ucp-doctor`, `davillafer/UCP-Compliance-Checker`
- **Typed models** — Python/JS official SDKs ship Pydantic + Zod models generated from the canonical JSON schemas; our Go CLI should do the equivalent (typed structs + validation) so command outputs are reliable.
- **Capability-aware client** — `OmnixHQ/ucp-client` only surfaces tools that the connected merchant actually supports.
- **Reference server interop** — must work against the `Universal-Commerce-Protocol/samples` Python and Node reference merchants out-of-the-box (matches official tests).
- **Conformance probes** — UCP spec mandates header set (`UCP-Agent`, `request-id`, `idempotency-key`, `request-signature`), status state machine (`incomplete` → `ready_for_complete` → `ready_for_payment` → `processing`), and signed-message handling.
- **MCP transport** — UCP services declare `transport: "rest" | "mcp" | "a2a" | "embedded"`; the CLI should at least *talk to* MCP-transport merchants, and ship its own commands as MCP tools (printing-press default behavior).

## Data Layer
Local SQLite store (printing-press default) is exactly the right shape for UCP, because the protocol is stateful across sessions and merchants:
- **Primary entities:** `merchants` (domain, manifest snapshot, capabilities, signing keys, last_checked_at), `carts` (id, merchant_id, line_items_json, buyer_json, totals_json, status), `checkout_sessions` (id, cart_id, merchant_id, status, payment_handler, mandate_id, created_at, updated_at), `orders` (id, session_id, status, fulfillment_json), `searches` (query, merchant_id, results_json, ts), `payment_handlers` (handler_id, merchant_id, config_json).
- **Sync cursor:** `merchants.last_checked_at` (per-domain manifest refresh), `orders.updated_at` (webhook reconciliation).
- **FTS/search:** FTS5 over cached product titles + descriptions + merchant_name → makes `ucp search` work offline against the corpus we've fanned out to before.

## Codebase Intelligence
DeepWiki not yet queried; the public-research signals are strong enough for the brief. Inputs the generator already has:
- **Auth model:** No global API key. Per-merchant headers `UCP-Agent: profile="<url>"`, `request-signature` (ECDSA-P256 over canonicalized request), `idempotency-key`. Treat as `auth.type: composed`; many endpoints are `no_auth: true` (manifest discovery, public catalog search). Identity Linking uses OAuth 2.0 per-merchant.
- **Data model:** Reverse-domain capability names (`dev.ucp.shopping.checkout`), extensions (`.discount`, `.fulfillment`) that extend the core schema, payment_handlers keyed by reverse-domain (`com.google.pay`, `com.shop.pay`).
- **Rate limiting:** Spec mandates merchant-defined; Shopify mentions trust-tier-based rate limits referenced in the agent profile.
- **Architecture:** "Server-selects" — server picks intersection of capabilities. Merchant publishes profile at `/.well-known/ucp`; agent advertises profile URL via `UCP-Agent` header; both compute the negotiation set and validate against schemas fetched lazily.

## User Vision
**(captured at the start of this run)** — CLI that lets a user OR an agent search products across UCP-compatible merchants, add items to a universal cart, and prep a checkout draft. AP2 (separate companion CLI, next session) handles payment authorization. Headline subcommands: `ucp check <domain>`, `ucp search`, `ucp cart`, `ucp checkout prep`. No real UCP merchant identified yet for live testing — fall back to a local mock server using the reference Python/Node samples if needed. Publish target: `library/commerce/ucp/`.

## Product Thesis
- **Name:** `ucp-pp-cli` (binary), `ucp` (Go module path / library slug)
- **Why it should exist:**
  - The official Python and JS SDKs ship typed models, not a CLI — agents and humans still need a terminal-grade tool.
  - The community Go SDK is a thin client wrapper; nothing in Go has a Cobra-style CLI with offline cart, agent-native output, and a merchant directory.
  - Existing validators (`nextwaves.com`, `ucp-doctor`) are web UIs or TypeScript libs; they don't compose with shell pipelines, `--json`, `--select`, or local-state aggregation.
  - **Cross-merchant aggregation** (search across N UCP merchants, persist results, filter offline) is a transcendence feature only possible with a local store + parallel fan-out — none of the existing tools do this.
  - The CLI becomes the natural local interface to UCP that an AI-David-style agent stack can call without standing up a browser.

## Build Priorities
1. **`ucp check <domain>`** — manifest fetch + schema lint + readiness grade. Output as table/JSON. *(Mirrors nextwaves + ucp-doctor, but offline-capable, agent-native, and composable.)*
2. **`ucp merchants list/add/refresh`** — local registry of known UCP merchants (seeded from awesomeucp/merchants + homototus/ucp-directory + a curated starter set), with `--refresh-all` to bulk-recheck manifests.
3. **`ucp search "<q>" --merchant <domain>` (and `--all` for fan-out)** — issue `dev.ucp.shopping.catalog` searches against one or many merchants in parallel; normalize, persist to local SQLite, support `--limit`, `--json`, `--select`.
4. **`ucp cart add/list/remove/show`** — local cart abstraction tied to a merchant; uses UCP cart capability when the merchant supports it, falls back to local-only line-item tracking otherwise.
5. **`ucp checkout prep`** — create or refresh a `dev.ucp.shopping.checkout` session draft from a cart, surface required-fields/missing-fields, emit a JSON envelope an AP2 CLI can authorize. Stops short of `POST /checkout-sessions/{id}/complete` — that's AP2's role.
6. **`ucp orders get/cancel/watch`** — fetch order, cancel where supported, optional webhook watcher.
7. **`ucp mock serve --port 8080`** — boot the bundled Python/Node reference merchant locally (vendored under `internal/mock/`), so `ucp check`/`search`/`cart`/`checkout` flows work end-to-end without any third-party UCP merchant being approved for our agent. (Solves the reachability risk.)
8. **`ucp doctor`** — diagnostic of the CLI's own setup (db path, OS deps, mock available, key UCP merchants reachable).
9. **`ucp sql "SELECT ..."`** + **`ucp search --local`** — printing-press defaults that compound with the SQLite store.

## Discovery Sources (no browser-sniff or crowd-sniff needed)
- **Official spec:** http://ucp.dev/2026-04-08/specification/overview (full schema, REST + MCP + A2A + embedded transports documented)
- **Google docs:** https://developers.google.com/merchant/ucp (the URL the user invoked with), plus `/guides/ucp-profile`
- **JSON schemas:** `Universal-Commerce-Protocol/ucp` repo holds canonical JSON Schema files that drive the Python/JS SDK model generators — the Go CLI generation should pull from these too rather than re-deriving from prose
- **Reference servers:** `Universal-Commerce-Protocol/samples` (Python FastAPI, Node Hono) — bundle one for `ucp mock serve`
- **Conformance fixtures:** `Universal-Commerce-Protocol/conformance-tests` — useful for the dogfood gate

Browser-sniff gate: **skip-silent** (spec is complete + machine-readable JSON Schemas exist). Crowd-sniff gate: **skip-silent** (official SDKs already cover the endpoint surface).

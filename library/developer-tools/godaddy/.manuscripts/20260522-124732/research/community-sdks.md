# GoDaddy — Community Coverage Inventory

Generated: 2026-05-22 10:58 UTC

## Decision

`build because gap exists`, with composition boundaries.

Reason: the official GoDaddy API surface is broad and split across 12 Swagger specs / 138 operations, while the strongest existing tools cover only slices. There is an official `@godaddy/cli`, but it is scoped to Developer Platform applications/webhooks, not the registrar/DNS/certificate/order/subscription surface. Do not duplicate that app/webhook CLI unless a later GoDaddy app-platform slice needs it. For the registrar/API-map goal, build the missing broad map and wrap/learn from existing domain-focused tools.

## Axis 0 — Printing Press Library / Catalog

Status: clear.

Evidence:

- `godaddy/research/official-library-preflight-2026-05-22.md`
- `printing-press-library search godaddy --json` returned `[]`
- live library tree, registry, open PRs, and open issues had no GoDaddy collision

## Axis 1 — Official SDKs / Tools

| Tool | Maintainer | Version checked | Coverage | Decision |
|---|---|---:|---|---|
| Official Swagger specs | GoDaddy | live 2026-05-22 | 12 Swagger 2.0 specs, 111 paths, 138 operations | primary source for codegen |
| `@godaddy/cli` | GoDaddy | `0.5.2`, modified 2026-05-14 | Developer Platform applications, auth, environment, webhooks, actions; JSON envelopes | compose/wrap only for app-platform scope; not a blocker for registrar/DNS CLI |
| GoDaddy MCP page | GoDaddy | live 2026-05-22 | public domain search and availability only; explicitly read-only/no purchase/no DNS modification | compose/avoid duplication for public domain search; not enough for account API map |

Official API docs:

- `https://developer.godaddy.com/doc`
- `https://developer.godaddy.com/doc/endpoint/domains`
- `https://developer.godaddy.com/mcp`

Evidence:

- `godaddy/proofs/developer-doc-next-data-2026-05-22.json`
- `godaddy/proofs/official-mcp-page-2026-05-22.html`
- `godaddy/proofs/github-readme-godaddy-cli-2026-05-22.md`
- `godaddy/proofs/npm-view-godaddy-cli-2026-05-22.json`

## Axis 2 — Community Wrappers

| Project | Language / package | Freshness | Coverage notes | Use |
|---|---|---|---|---|
| `@framers/agentos-ext-domain-godaddy` | TypeScript / npm | created 2026-05-02 | 5 domain tools: search, register, list domains, configure DNS, get domain info | strong reference for domain/DNS UX and tool naming; incomplete vs 138 ops |
| `@itentialopensource/adapter-godaddy` | npm adapter | modified 2026-05-19 | describes GoDaddy REST API v1/v2 integration | inspect later if code access is useful |
| `godaddy-mcp` | TypeScript / npm | modified 2025-06-03 | MCP server for domain availability only | reference for MCP packaging, not broad coverage |
| `GoDaddyPy` / `eXamadeus/godaddypy` | Python / PyPI + GitHub | PyPI `2.5.1`, GitHub pushed 2024-03-25 | simple A-record update/client use case | parser/client behavior hints only |
| `alyx/go-daddy` | Go | pushed 2024-08-19 | unofficial GoDaddy API client | possible Go helper reference |
| `cabemo/godaddy-cli` | Go | pushed 2023-04-14 | simple domain-management CLI | UX reference only |
| `sportwhiz/gdcli` | Go | pushed 2026-02-15 | alpha CLI for domains/orders | inspect if Go command structure helps |
| `community-sdks-godaddy` | Rust crate | updated 2026-03-11 | community Rust SDK | inspect only if Rust reference needed |

Evidence:

- `godaddy/proofs/community-gh-api-client-2026-05-22.json`
- `godaddy/proofs/community-gh-sdk-2026-05-22.json`
- `godaddy/proofs/community-gh-cli-2026-05-22.json`
- `godaddy/proofs/community-npm-search-godaddy-2026-05-22.json`
- `godaddy/proofs/npm-view-agentos-ext-domain-godaddy-2026-05-22.json`
- `godaddy/proofs/github-agentos-godaddy-service-2026-05-22.ts`
- `godaddy/proofs/npm-view-itential-adapter-godaddy-2026-05-22.json`
- `godaddy/proofs/npm-view-godaddy-mcp-2026-05-22.json`
- `godaddy/proofs/pypi-view-godaddypy-2026-05-22.json`
- `godaddy/proofs/community-crates-search-godaddy-2026-05-22.json`

## Axis 3 — Reverse-Engineered Maps

No complete public reverse-engineered GoDaddy map was found in this pass.

Useful partials:

- `mintuhouse/godaddy-api` / npm `godaddy-api`: older generated API package from official docs.
- `@framers/agentos-ext-domain-godaddy`: recent source shows practical calls for v1 domain availability, purchase, list, detail, and DNS add/replace/delete.
- GitHub CLI search found several small domain/DNS CLIs, but none appears to cover the 12-spec official surface plus account portal internals.

## Axis 4 — Change / Social Signal

Bird/X search for `GoDaddy API` found recent operator chatter, not an official breaking-change announcement. Most useful signal: a May 16, 2026 tweet says Codex plus the GoDaddy API helped transfer out roughly 50 domains, which reinforces that domain portfolio workflows are an actual current use case.

Evidence:

- `godaddy/proofs/community-bird-godaddy-api-2026-05-22.txt`

## Axis 5 — Strong Official CLI Sniff-Test

| CLI | Maintainer | Version checked | JSON/noninteractive support | Coverage | Decision |
|---|---|---:|---|---|---|
| `@godaddy/cli` (`godaddy`) | GoDaddy | `0.5.2` | yes; executable commands emit JSON envelopes; `--pretty`; NDJSON for long-running ops | apps, auth, environment, webhooks, actions | do not duplicate app/webhook scope; build registrar/DNS/cert/order/subscription gap |

This does not trigger the `gh`/`stripe`/`supabase` no-go anti-pattern because the official CLI is narrow relative to the target venue. It is strong enough to avoid duplicate work in its own scope.

## Wrap Target

Primary source: official Swagger specs under `godaddy/api-map/openapi/official/*.json`.

Secondary references:

- `@framers/agentos-ext-domain-godaddy` for practical domain/DNS workflows.
- `@godaddy/cli` for JSON-envelope UX and app-platform composition if that scope becomes relevant.
- GoDaddy official MCP page for public/read-only domain search boundaries.

## Undocumented Surface To Map Ourselves

Still pending:

1. Account portal navigation and product inventory.
2. Domain Control Center UI calls beyond public Swagger.
3. Managed WordPress / ManageWP internals.
4. M365 mailbox and productivity flows.
5. Billing/subscriptions UI beyond the official subscriptions/order routes.
6. Support/ticketing surfaces.

## Next

1. Inspect `@framers/agentos-ext-domain-godaddy` route handling for command UX and barriers.
2. Keep the 138-operation official route index as the codegen source of truth.
3. Do not implement `@godaddy/cli` app/webhook duplicates unless the goal expands into Developer Platform app management.

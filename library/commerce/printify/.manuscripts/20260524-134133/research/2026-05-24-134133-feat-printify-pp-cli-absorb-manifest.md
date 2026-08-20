# Printify PP CLI Absorb Manifest

Run: `20260524-134133`
API: Printify
Spec: `https://developers.printify.com/openapi.json`
Decision: proceed with official OpenAPI plus hand-coded workflow commands.

## Source Tools

| Source | URL | Language | What contributed | Absorb decision |
|---|---|---:|---|---|
| TSavo/printify-mcp | https://github.com/TSavo/printify-mcp | JavaScript/TypeScript | Shop lookup, catalog browsing, product operations, upload helpers, env-var expectations. | Absorb as table-stakes workflow coverage; do not copy MCP transport. |
| lawrencemq/printipy | https://github.com/lawrencemq/printipy | Python | Python wrapper shape around shops, products, uploads, orders, and publish/order helper vocabulary. | Absorb as command naming evidence and workflow coverage. |
| Official Printify OpenAPI | https://developers.printify.com/openapi.json | OpenAPI | Authoritative endpoint surface, schemas, bearer auth, product/upload/order/webhook operations. | Primary generation source. |
| Printify Help Center | https://help.printify.com/ | Docs | Product creation and personalization workflow context. | Use only as workflow narrative; no undocumented browser-only claims. |

## Absorbed Table Stakes

| Area | Commands / surfaces to preserve | Source evidence | Implementation |
|---|---|---|---|
| Shops | list shops, inspect shop metadata, choose active shop context. | Official API, TSavo/printify-mcp, printipy. | Generated endpoints plus context/narrative polish. |
| Catalog | blueprints, print providers, variants, shipping, shipping information. | Official API, wrapper tooling, user brief. | Generated endpoints; feed local sync tables for matrix commands. |
| Products | list/get/create/update/delete, publish/unpublish, publishing status callbacks, GPSR details. | Official API, user brief. | Generated endpoints plus hand-coded manifest helpers. |
| Uploads | list/get/upload/archive images. | Official API, TSavo/printify-mcp, user brief. | Generated endpoints plus file-oriented upload helper. |
| Orders | list/get/create/send-to-production, shipping costs, cancel. | Official API, printipy issue surface. | Generated endpoints plus local fulfillment analysis. |
| Webhooks | create/list/update/delete webhook subscriptions and topics. | Official API. | Generated endpoints. |

## Transcendence Features

| Score | Feature | Command | Buildability | Why it belongs |
|---:|---|---|---|---|
| 9 | Personalization coverage audit | `personalization-audit` | Hand-code | User's stated goal centers personalization options. Public API documents placeholder text/font fields, but browser-sniff was declined, so this audits supported fields and names unsupported UI gaps explicitly. |
| 9 | Placement matrix | `placement-matrix` | Hand-code | Product creation succeeds only if artwork lands in the intended print areas. This joins products, variants, print areas, placeholders, and uploads into a deterministic table. |
| 8 | Catalog margin matrix | `catalog-margin-matrix` | Hand-code | Catalog/provider/variant/shipping data is separated in raw API responses; operators need joined cost and margin rows before creating batches. |
| 8 | Product manifest drift | `product-drift` | Hand-code | Agents and batch workflows need to prove the created product matches the intended manifest after create/update. |
| 7 | Personalized batch manifest compiler | `personalization-batch` | Hand-code | Weekly product drops need repeatable per-row manifests using documented image and text placeholder fields. |
| 7 | Asset reuse map | `asset-reuse` | Hand-code | Upload cleanup and product auditing require joining uploaded images back to product print areas. |
| 6 | Fulfillment risk scan | `fulfillment-risk` | Hand-code | Order, line-item, product, publish, and shipment state live in separate responses; a local join catches risky open orders. |

## Killed Candidates

| Candidate | Reason |
|---|---|
| Catalog art fit finder | Print-area constraints were not sufficiently verified without browser discovery. |
| Personalization checkbox enabler | Public API support is unproven, and browser-sniff was declined. |
| AI listing copy generator | Depends on LLM generation rather than deterministic API/local data. |
| Provider chooser | Too subjective unless reduced to explicit cost/shipping rows; covered by catalog margin matrix. |
| Publish dashboard | Persistent dashboard/watch scope is larger than this CLI run; drift/risk commands cover the actionable checks. |
| Webhook coverage matrix | Too close to generated webhook management and not a stated weekly pain. |
| MCP parity report | Competitor parity is research, not a user workflow. |

## Crowd-Sniff Disposition

`crowd-sniff` produced five `/admin/printify/...` routes from community SDK material. These appear to be third-party admin/proxy routes, not official Printify Public API paths. They are rejected for generation and retained only as discovery provenance.

Accepted crowd routes: none.

## Stub Policy

No transcendence stubs are approved. Each surviving feature must be hand-coded or removed before shipcheck. The browser-only personalization checkbox must not be claimed as implemented.

## Approval Scope

Generate from the official Printify OpenAPI, add the table-stakes framework surfaces, and hand-code the seven transcendence commands above. Auth uses bearer PAT from `PRINTIFY_API_TOKEN`.

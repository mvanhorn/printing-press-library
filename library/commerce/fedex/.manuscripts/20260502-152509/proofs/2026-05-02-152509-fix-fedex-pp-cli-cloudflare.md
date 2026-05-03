# FedEx CLI Cloudflare-Pattern MCP Enrichment

## Verdict: SHIP (after polish-fix iteration)

After applying `mcp.transport: [stdio, http]` + `mcp.endpoint_tools: hidden` + `mcp.orchestration: code` to the spec and regen-merging into the published library, all 5 shipcheck legs PASS and runtime MCP catalog drops from 65 tools to 19 tools.

## What changed

| Surface | Before | After |
|---|---|---|
| MCP runtime catalog (verified via `tools/list`) | 65 tools | **19 tools** (-71%) |
| MCP `tools.go` static AddTool calls | 48 | **1** (-98%) |
| New `internal/mcp/code_orch.go` | absent | **22.9 KB** (`fedex_search` + `fedex_execute`) |
| MCP main.go | stdio-only | stdio + streamable HTTP via `--transport`/`--addr` |
| Endpoints reachable via `fedex_execute` | n/a | **all 48** (no capability lost) |
| tools-manifest.json size | 39 KB | 68 KB (richer schemas per tool, fewer tools) |
| Shipcheck verdict | 5/5 PASS | 5/5 PASS |

## Resulting MCP catalog (19 tools)

**Code orchestration layer (2):**
- `fedex_search(query)` — semantic search across all 48 endpoints
- `fedex_execute(endpoint, body)` — call any endpoint by name

**Hand-built intent layer (16, via cobratree walker):**
- Save-money: `rate_shop`, `ship_bulk`, `ship_etd`
- Address book: `address_save`, `address_list`, `address_delete`, `address_validate`
- Tracking: `track_diff`, `track_watch`
- Customer service: `return_create`
- Local archive: `archive`, `spend_report`, `manifest`

**Built-ins (3):** `context`, `import`, `export`, `tail` (auto-generated framework)

## Live verification (production FedEx, dual credentials)

| Test | Verdict | Output |
|---|---|---|
| `auth login --service default --env prod` | PASS | Token minted from Ship/Rate project |
| `auth login --service track --env prod` | PASS | Token minted from Track-only project |
| `track number 870977513854` (uses Track creds) | PASS | "Delivered, FedEx International Priority, OGDEN UT" |
| `address validate 1600 Amphitheatre Pkwy` (uses default creds) | PASS | BUSINESS classification, normalized to `1600 AMPHITHEATRE PKWY, MOUNTAIN VIEW CA 94043-1351` |
| `fedex_search query=address` (MCP code-orch) | PASS | 6 endpoint matches ranked by relevance score |
| `fedex_execute endpoint=addresses.validate` | PASS | Reachable; body proxied through to `/address/v1/addresses/resolve` |
| `rate shop` | FAIL | HTTP 400 `ACCOUNT.NUMBER.MISMATCH` — environmental (placeholder account `740561073` isn't tied to this prod project), not a CLI defect |

## Process notes

This iteration exposed three machine-level gaps that warrant retro:

1. **Skill missing "Pre-Generation MCP Enrichment" Phase 2 step.** The skill has detailed Auth Enrichment but no parallel MCP Enrichment, despite the documented `mcp:` spec block and the user-memory note "for 50+ endpoint specs, add mcp.transport+orchestration+endpoint_tools BEFORE generating". Fixing this would have prevented the second-pass regen-merge work.

2. **`mcp-sync` overwrites `tools.go` regardless of `endpoint_tools: hidden`.** Running mcp-sync after fresh-generate undoes the endpoint-hiding by re-emitting all 48 endpoint AddTool calls from the cobratree walker. mcp-sync's intent (migrate to runtime walker) conflicts with `endpoint_tools: hidden`'s intent (don't mirror endpoints as tools). Should be a flag or auto-detection.

3. **Polish-skill misclassified MCP dim fixes.** Reported "MCP Remote Transport 5/10 — would require adding HTTP transport to a generator-owned file. Retro candidate, not polish." Wrong: it's a `mcp.transport: [stdio, http]` spec field. The polish skill's classifier should map MCP scorecard dims to spec fields, not generator code.

4. **Scorecard's MCP tool count reads spec-derived count, not the runtime manifest.** Reports "MCP: 48 tools" even after `endpoint_tools: hidden` and `code: orchestration` collapse the runtime catalog to 19. Scorecard should read the actual manifest (or runtime probe) for accuracy.

All four are captured in user memory under feedback_pre_generation_mcp_enrichment / feedback_count_total_mcp_tools / feedback_polish_mcp_misclassify / feedback_act_on_loaded_memory.

# GoDaddy API Map Status

Generated: 2026-05-22 10:34 UTC
Updated: 2026-05-22 13:06 UTC

## Current Status

GoDaddy is the first non-Goodreads round-robin bounce. This pass is read-only and public-docs-only.

Axis 0 Printing Press preflight is clear: no GoDaddy package, registry entry, tree path, open PR, or open issue collision was found in the live `mvanhorn/printing-press-library` checks.

Official public API aggregation is started and complete enough for the next planning decision:

- 12 Swagger 2.0 specs downloaded
- 111 paths
- 138 operations
- API groups: `abuse`, `aftermarket`, `agreements`, `ans`, `auctions`, `certificates`, `countries`, `domains`, `orders`, `parking`, `shoppers`, `subscriptions`
- Normalized official route index generated under `godaddy/api-map/normalized/`
- First-pass operation barriers assigned: 61 read, 8 validation, 27 write, 17 destructive, 25 purchase/billing
- Community coverage inventory completed: existing tools cover slices, not the full target surface
- Local combined PP scaffold generated and locally verified under `library/developer-tools/godaddy`
- MCP surface reviewed and regenerated with code orchestration: `godaddy_search` + `godaddy_execute` cover the full documented surface while raw endpoint mirrors stay hidden from MCP.
- Route-risk warning pass applied: high-risk endpoint commands now expose `pp:risk`, `pp:barrier`, and `pp:warning` annotations in `agent-context`, plus visible `Risk warning:` help text.

The older goal-doc estimate of 7-9 Swagger files was stale. Do not codegen from one partial GoDaddy spec and call it the venue.

## Evidence

- Preflight doc: `godaddy/research/official-library-preflight-2026-05-22.md`
- Swagger inventory: `godaddy/research/official-swagger-inventory-2026-05-22.md`
- Portal proof: `godaddy/proofs/developer-doc-next-data-2026-05-22.json`
- Downloaded specs: `godaddy/api-map/openapi/official/*.json`
- Operation list: `godaddy/proofs/swagger-operations-2026-05-22.tsv`
- Human-readable route table: `godaddy/api-map/markdown/godaddy-official-routes.md`
- Normalized route index: `godaddy/api-map/normalized/official-routes.json`
- Risk model: `godaddy/docs/operation-risk-model.md`
- Community inventory: `godaddy/research/community-sdks.md`
- Codegen decision: `godaddy/docs/codegen-decision.md`
- MCP audit proof: `godaddy/proofs/pp-mcp-audit-godaddy-risk-warnings-2026-05-22.json`
- Shipcheck proof: `godaddy/proofs/pp-shipcheck-godaddy-risk-warnings-2026-05-22.json`
- Help warning proofs:
  - `godaddy/proofs/godaddy-risk-help-domains-purchase-2026-05-22.txt`
  - `godaddy/proofs/godaddy-risk-help-dns-replace-2026-05-22.txt`
- Agent-context warning proof: `godaddy/proofs/godaddy-risk-agent-context-2026-05-22.json`

## Reproduce

```bash
printing-press-library search godaddy --json
printing-press-library list --category developer-tools --json
curl -fsSL https://developer.godaddy.com/doc
curl -fsSL https://developer.godaddy.com/swagger/swagger_<api>.json
jq empty godaddy/api-map/openapi/official/*.json
```

The public developer portal's `__NEXT_DATA__` is currently the fastest way to find the full API group list.

## Notes

- Most specs point at `api.ote-godaddy.com`; production examples use `https://api.godaddy.com`. Normalize base URLs before codegen.
- Some specs have missing `host` metadata. Treat server normalization as a real preprocessing step, not a cosmetic cleanup.
- `domains` dominates the official surface with 50 paths and 65 operations.
- DNS/account actions are now explicitly labeled in CLI help and agent annotations. For example, official `PATCH /v1/domains/{domain}/records` adds records, while official `PUT /v1/domains/{domain}/records` replaces matching records.
- This pass does not cover GoDaddy account portal internals, managed WordPress, M365 mailbox management, billing UI calls, support tickets, or Domain Investor/Auctions internals beyond the public specs.

## Next

1. Add low-risk live read smokes after credentials are intentionally sourced.
2. Only after that, use authenticated browser/API-key mapping for account-specific and undocumented routes.
3. Keep mutation routes dry-run/approval-gated even though they are now visible in the map.

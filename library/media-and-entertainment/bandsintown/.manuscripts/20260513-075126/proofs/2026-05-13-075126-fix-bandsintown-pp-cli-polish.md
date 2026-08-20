# Phase 5.5 Polish Report — Bandsintown

```
                          Before     After     Delta
  Scorecard:               82/100    83/100    +1
  Verify:                  100%      100%      0
  Dogfood:                 PASS      PASS      —
  Publish-validate:        FAIL      PASS      cleared 3 blockers
  Tools-audit (pending):   0         0         (1 surfaced post-mcp-sync, then resolved)
  PII-audit:               0         0         —
  Verify-skill findings:   0         0         —
  Workflow-verify:         PASS      PASS      —
  Go vet:                  clean     clean     —
```

## Fixes applied

1. Ran `mcp-sync` to generate `tools-manifest.json` (was missing).
2. Added `printer: "4.5.2"` and `printer_name: "printing-press"` to `.printing-press.json` (publish-validate manifest gate).
3. Set `auth_type: "api_key"` in `.printing-press.json` (matches the CLI's BANDSINTOWN_APP_ID enforcement and the phase5-skip marker).
4. Staged `phase5-skip.json` under `$CLI_DIR/.manuscripts/<run-id>/proofs/` (publish-validate phase5 gate location).
5. Wrote `mcp-descriptions.json` override with an MCP-grade description for `artists_artist`; re-ran mcp-sync — tools-audit went 1 pending → 0 and **MCP Desc Quality jumped to 10/10**.

## Verdict

**`ship_recommendation: ship`**, `further_polish_recommended: no`.

> All hard gates (dogfood, verify, workflow-verify, verify-skill, publish-validate,
> tools-audit, pii-audit, go vet) pass cleanly; remaining scorecard gaps are
> structural to a 2-endpoint partner-gated API and would need spec/generator
> changes, not polish iteration.

## Retro candidates (filed for the Printing Press maintainers)

- **Swagger 2.0 `securityDefinitions.appId` (apiKey in query) + `security: [appId]`** renders to `tools-manifest.json` `auth.type: none` and `no_auth: true` on every endpoint. The CLI runtime correctly enforces the env var; only the manifest misclassifies. Worked around by hand-editing `.printing-press.json::auth_type` so the publish-validate phase5 gate passes against the phase5-skip marker.

## Known structural limits (not bugs)

- Scorecard MCP Token Efficiency 7/10, MCP Remote Transport 5/10, Cache Freshness 5/10, Breadth 6/10, Vision 6/10, Workflows 6/10, Type Fidelity 3/5, Auth Protocol 7/10 — structural for a 2-endpoint read-only API; would require spec edits (mcp.transport, mcp.endpoint_tools, mcp.intents) + regeneration.
- Live-check on `pull` reports `BANDSINTOWN_APP_ID not set` — environmental, not a CLI defect.
- Dogfood "search uses generic Search only or direct SQL" — by design; a 2-endpoint API has no useful domain Search to wire.

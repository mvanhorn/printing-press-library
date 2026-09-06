# Google Tag Manager CLI — Build Log (READ-ONLY)

Manifest transcendence rows: 9 planned, 9 built. Phase 3 will not pass until all 9 ship.

## Scope
Read-only. The OpenAPI spec (apis-guru `googleapis/tagmanager/v2`) was stripped to its
22 GET endpoints before generation; all 28 POST / 1 PUT / 1 DELETE operations were removed.
Auth converted from OAuth2 user-flow to bearer-token (`Authorization: Bearer`, env `GTM_ACCESS_TOKEN`)
to match the operator's gcloud reality and avoid an OAuth-client setup for a read CLI.

## Built — Priority 0 / 1 (generator-emitted)
- 22 typed GET endpoint commands under `tagmanager` (accounts, containers, workspaces + all child
  resources, version headers, live version, environments, user permissions, lookup, snippet).
- Local SQLite store (`resources` + FTS5) plus auth/config/doctor/MCP scaffolding.

## Built — Priority 2 transcendence (9/9, hand-coded)
Foundation: `internal/cli/gtm_model.go` (snapshot + entity tables, pull walker, tag/trigger/variable
reference index, audit/diff/consent/fleet logic) and `internal/cli/gtm_output.go` (read-only mirror access).

1. `pull` — mirror a container into local SQLite (live in one `versions:live` call, or a workspace). Foundation.
2. `audit` — dead tags, orphan triggers, unused variables, missing-consent, custom-HTML, all-pages; `--fail-on` → exit 3.
3. `diff` — workspace/version/container snapshot diff, field-level, container-scoped refs.
4. `fleet` — cross-container matrix (GA4 ids, consent %, custom-HTML, counts).
5. `consent-report` — Consent Mode v2 coverage per tag, vendor-classified.
6. `search` — substring search over every entity's name/type/params across all containers.
7. `uses` — reverse-dependency / blast-radius for a variable or trigger.
8. `fires` — trigger↔tag↔variable graph walk (both directions).
9. `export` — deterministic flattened JSON (volatile fields stripped) or raw GTM-shaped.

## Removed (read-only / quality)
- `import` (POST create/upsert — read-only violation, no write endpoints exist)
- `analytics`, `workflow` (generic scaffolding carrying wrong-domain Discord examples)

## Tests
- `internal/cli/gtm_model_test.go` — 11 table-driven tests (audit, ref-index, consent, diff,
  snapshot resolution, fleet, parse, end-to-end pull walker with a fake getter). Full suite green.

## Notable decisions
- apis-guru collapses all individual-resource `{path}` GETs to one slot, so only `.list` endpoints
  survive as typed commands; individual-resource reads are reached by the hand-coded transcendence commands.
- Bare source refs (`live`, `workspace:N`) resolve within the *active* container (most recently pulled)
  to prevent silent cross-container comparison; `container:<id>` switches explicitly.
- Read commands open the store READ-ONLY (`OpenReadOnly`) — concurrent-safe, intent-correct.
- `pull` validates `--account/--container/--workspace` are numeric (path-injection hardening).

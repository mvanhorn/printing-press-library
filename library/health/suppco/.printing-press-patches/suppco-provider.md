# SuppCo provider customization

## Why this patch exists

The generated framework is broader than this provider contract. SuppCo needs a small stateless read surface, a renewable bearer-token workflow, strict data minimization, hierarchical nutrient preservation, and a normalized two-read snapshot. A generic endpoint passthrough, sync database, or Cobra-tree MCP mirror would expose more behavior and data than the package owns.

## Reprint contract

Generate from the bundled `spec.yaml` with PrintingPress v4.29.0 or newer, then retain or reapply these handwritten surfaces:

PrintingPress v4.29.0+ intentionally strips root `dogfood-results.json` and `workflow-verify-report.json` from the staged public package. For catalog PR #1550, `dogfood-results.json` was regenerated with v4.30.1 and restored at maintainer request; its WARN records the intentional hand-edited MCP surface documented below. The durable full-acceptance proof remains `.manuscripts/<run-id>/proofs/phase5-acceptance.json`.

- `internal/provider/**`: bounded GET extraction, minimum projections, date validation, observation timestamps, canonical-origin and redirect policy, bearer-only stateless transport, missing-reader guards, and synthetic fixtures. Flatten product-bound components from `products[].ingredients`, use the final ancestry segment as the immediate parent, and ignore the aggregate top-level nutrient view.
- `internal/regimen/**`: deterministic product/component and schedule-activity normalization plus the snapshot contract. Preserve every component row; never add parent and child amounts. Reject blank units, encode leaf `component_ids` as an empty array, keep `provider_schedule` and `effective_regimen` structurally independent, and sort activities by label and scheduled products by provider product ID while preserving provider `schedule_days` order.
- `internal/cli/auth.go` and `auth_suppco_test.go`: `auth set-token` reads stdin only, rejects positional secrets, clears legacy generic Authorization credentials before persisting a replacement, and warns when `SUPPCO_ACCESS_TOKEN` overrides the saved value.
- `internal/config/config.go` and `internal/cliutil/credentials_test.go`: a direct `SUPPCO_ACCESS_TOKEN` masks any legacy saved `auth_header` and case-insensitive generic `Authorization` header at runtime so the documented environment-first credential precedence is real, without rewriting saved configuration or removing unrelated headers. Bearer-token persistence explicitly clears those legacy authorization forms.
- `internal/config/config_perms_test.go`: isolate the generated credentials-file path so permission tests cannot consume an operator's existing private credentials file.
- `internal/cli/suppco_contract.go`, `suppco_provider.go`, and `suppco_surface.go`: no-cache client hook, shared service construction, verifier mock-origin binding, narrow runtime command tree, live-only provider mode, refusal of delivery, profiles, and insecure TLS, and rejection of unsupported output transformations.
- Retain the generated `agent-context` command as hidden internal metadata because PrintingPress live dogfood uses it to enumerate the bounded command tree; it remains absent from normal help and MCP registration.
- `internal/cli/stack_products.go`, `stack_nutrients.go`, `promoted_schedule.go`, and `regimen.go`: CLI handlers must call the shared provider service rather than return raw endpoint payloads.
- `internal/cli/doctor.go`: validate local config/client construction without probing `/` or a private stack response, and omit generated cache/sync guidance.
- `internal/mcp/suppco_tools.go` and the `RegisterTools`/context/client portions of `internal/mcp/tools.go`: expose exactly `stack_products`, `stack_nutrients`, `schedule_show`, `regimen_snapshot`, and `context` over stdio, with safe actionable 401/403/429 errors and handler-level synthetic tests. Do not restore SQL, search, sync, or a Cobra-tree mirror.
- `internal/cliutil/ratelimit.go` and the pacing call in `internal/client/client.go`: provider pacing honors context cancellation instead of sleeping after the caller has canceled.
- `internal/client/client.go`: bound untrusted provider response bodies before decoding; oversized responses fail without echoing content.
- `.printing-press.json`: retain the generated `mcp_tool_count` convention. It counts the three API-derived tools; the derived `regimen_snapshot` and utility `context` tools are intentionally excluded. `internal/mcp/tools_test.go` pins the actual registered runtime total at five.
- `.golangci.yml`: retain the v2 configuration schema marker required by current golangci-lint releases. The generator's broader dormant framework still has baseline lint debt; SuppCo review compares diagnostics with a fresh print and treats only package-introduced findings as actionable until PrintingPress corrects that baseline.
- `AGENTS.md`, `README.md`, and `SKILL.md`: keep the periodic token-replacement model, exact command/tool surface, privacy boundary, and downstream Trainer Core ownership explicit. Do not restore generated discovery instructions for commands removed from the runtime tree.

The generated source files for unused framework features may remain in the tree, but their commands and flags must not be reachable. Removing the runtime registration is intentional: PrintingPress's verifier otherwise misclassifies the package as database-backed and requires a sync pipeline.

## Behavioral invariants

- Base origin is `https://api.supp.co`; a loopback origin is accepted only when PrintingPress mock mode, its live-HTTP verifier flag, and its fixed non-secret mock credential are all present together. Verify-like environment variables cannot redirect a real saved bearer token.
- `doctor` performs no provider read. End-to-end token validation belongs to an explicit minimized read command.
- TLS verification remains enabled, cross-origin redirects fail closed, response cookies are never retained, and saved Cookie or generic Authorization headers are rejected.
- Tokens never appear in argv, output, fixtures, docs, review artifacts, or logs.
- Runtime MCP registration is exactly five tools, while manifest `mcp_tool_count` remains three API-derived tools under the PrintingPress library convention.
- Provider-controlled error bodies and provider identifiers never appear in CLI or MCP error text; retain only body-free HTTP status metadata needed for classification.
- `/api/users/me_compact/` is projected in memory; unrelated fields are discarded and raw envelopes are never emitted.
- `/api/schedules/{date}` is projected to `date` plus minimized `activities`, scheduled products, and reminder state; the deprecated guessed `items` object is not part of the contract.
- Snapshot performs one compact-stack read followed by one dated-schedule read. Each section records its own completion observation; top-level `as_of` is recorded after both reads.
- `provider_schedule` is factual, `user_override` is absent, `effective_source` is `provider_schedule`, and `effective_regimen` is a normalized copy without cadence inference.
- No local sync, search, SQLite read path, writeback, output webhook, clinical semantics, or Trainer Core behavior is part of this package.

## Required reprint checks

Run focused provider/regimen/CLI/MCP tests, full `go test ./...`, `go vet ./...`, `go build ./...`, the canonical PrintingPress shipcheck, publish validation, PII audit, credential-pattern scan, and an exact diff against a fresh print. Any authorized live shape-conformance check must retain only schema pass/fail facts, never raw account data.

# Browserbase CLI — Build Log

Manifest transcendence rows: 7 planned, 7 built. Phase 3 gate passed (all 7 resolve via Cobra; dogfood novel_features_check found==planned; Priority 1 review gate passed on 3 absorbed commands).

## Phase 2 Generation
- Spec: official Browserbase OpenAPI v3 (openapi.v1.yaml), enriched with `x-category: cloud`, `x-mcp: {transport: [stdio], orchestration: endpoint-mirror}`, `x-learn` block, `x-auth-env-vars: [BROWSERBASE_API_KEY]`.
- Fixed generation blockers:
  1. Resource `search` collided with reserved framework template → renamed via `x-pp-resource: websearch` on the operation.
  2. `functions/invoke` deep body param `session-create-params-browser-settings-enable-native-select-polyfill` exceeded 64-char MCP key limit → wrapped request body in `oneOf` so the generator emits the `--body-json` fallback (correct surface for a JSON-payload invoke).
  3. Removed stale preserve snapshot from failed `--force` attempt.
- Quality gates passed: go mod tidy, go test, govulncheck, go vet, go build, binary runs, `--help`, version, doctor.
- Generator emitted framework surface (sync/search/analytics/tail/doctor/agent-context/sql/export/import/stale/orphans/load/which/learnings/playbook) + typed endpoint commands for all resources.
- Novel stubs skipped at generate (name collision with resource parents) — will wire as subcommands: `sessions orphans`, `sessions run`, `fetch batch`, `projects digest`, `agents runs diff`, `usage trend`, `web history`.

## Phase 3 Build Plan (transcendence, all hand-code)
1. `sessions orphans` — local scan of synced sessions for running+old, optional `--stop` batch release. [pp:data-source local] ✅
2. `sessions run` — create + print connectUrl + guaranteed REQUEST_RELEASE on SIGINT/timeout. [pp:data-source live] ✅
3. `fetch batch` — paced loop over /v1/fetch with resumable checkpoint. [pp:data-source auto] ✅
4. `projects digest` — join sessions + agent runs + downloads by project/day from local store. [pp:data-source local] ✅
5. `agents runs diff` — sync run messages, unified diff of sequences + results. [pp:data-source live] ✅ (lcsDiff has table-driven tests)
6. `usage trend` — accumulate project usage snapshots on sync, render trend. [pp:data-source local] ✅
7. `web history` — local fetch/search cache ledger with re-emit. [pp:data-source local] ✅

## Intentionally deferred
- DOM snapshot/click/fill (browse CLI parity) — `(stub)` per manifest approval; needs live CDP session transport, not a replayable HTTP surface. `sessions run` prints connectUrl.
- Skipped body fields: `functions/invoke` deep nested body → `--body-json` fallback (correct surface).

## Generator limitations found
- OpenAPI parser has no `flag_name` extension for deep body properties; only internal-YAML supports it. Worked around with `oneOf` → `--body-json` for the one deep-body endpoint.
- Novel command names that collide with generated resource parents are skipped as stubs at generate; must be wired as subcommands in Phase 3 (expected pattern per skill).

# Rightbrain CLI Brief

## API Identity
- **Domain:** AI agent/task platform ("agents-as-a-service"). Versioned AI functions (Tasks) and tool-using conversational Agents, exposed over REST, run inside customers' existing tools.
- **Company:** Rightbrain (rightbrain.ai). Founded June 2023 by Peter Cheyne and Matt Wells. Newcastle-founded, London-registered. £3M/$3.8M seed announced May 2026, led by NPIF II – PXN Equity Finance (Salica, NYDIG participating).
- **Users:** Backend/platform engineers and AI engineers at B2B services firms and consultancies who need governed, versioned LLM functionality (approvals, evals, audit) rather than raw model calls. Also agents themselves — the platform ships an MCP gateway.
- **Data profile:** Deeply hierarchical and UUID-keyed. Every resource lives at `/org/{org_id}/project/{project_id}/<resource>`. Core entities: Task, Task Agent, Skill, Collection (RAG), Document, Trigger, Run, Revision, Eval set/run, Approval request, MCP server, Integration, Audit event, API key, OAuth client, Model.
- **Spec:** Official OpenAPI 3.0.3 — `https://app.rightbrain.ai/api/v1/openapi.json` (HTTP 200, 1,269,491 bytes, 257 paths, 345 operations, 461 schemas). Also served as YAML at `/api/v1/openapi.yaml` (HTTP 200).
- **Auth:** `Authorization: Bearer <token>`. Three schemes in spec: `HTTPBearer` (JWT), `ApiKey` (apiKey-in-header, but wire format is `Bearer {api_key}`), `OAuth2` (client credentials + authorization code/PKCE). Token endpoint `https://oauth.rightbrain.ai/oauth2/token`. Canonical env vars from the official CLI/SDK: **`RB_API_KEY`, `RB_ORG_ID`, `RB_PROJECT_ID`**.
- **Pagination:** cursor-based. Query params `cursor` (uuid) + `page_limit` (1–100, default 100). Envelope `{pagination: {next_cursor, has_next, page_limit}, results: []}`. Six endpoints return bare arrays instead: `model`, `input_processor`, `task_forwarder_type`, `guardrail`, `api_key`, `task/{id}/share`.
- **Errors:** `{"detail": {"reason": "<STABLE_CODE>", "message": "...", "<specific>Error": {...}}}`. Branch on `reason`, not `message`. 401 is the exception — rejected pre-application, returns `{"error": {"code": 401, ...}}`. 422 uses FastAPI validation shape. Status codes in use: 400, 401, 403, 404, 409, 422, 500.
- **Run headers:** `x-task-run-id`, `x-task-revision-id`.

## Reachability Risk
- **None.** `GET https://app.rightbrain.ai/api/v1/whoami` → HTTP **401** with the documented auth envelope (expected for an auth-required API with no key supplied). `GET /api/v1/org` → HTTP **401**. `GET /api/v1/openapi.json` → HTTP **200**. No bot protection, no WAF, no challenge pages, no Cloudflare. Plain stdlib HTTP is sufficient; no Surf/browser transport needed.
- Tier/permission hints from 4xx body: none — the 401 envelope carries no tier or plan text.
- Probe-safe endpoint used: none declared (`x-pp-safe-probe` absent); GET-only probing was used.

### ⚠ Spec defect that must be corrected before generation
The spec's `servers[0]` is `https://api.rightbrain.ai/api/v1` labelled "Production API", but **`api.rightbrain.ai` has no DNS record (NXDOMAIN)**. Confirmed twice independently (direct `getent hosts` + curl exit 000; and the research pass). The working production base URL — used by every docs example, by `@rightbrain/sdk`'s default `baseUrl`, and by the GitHub Action's default host — is **`https://app.rightbrain.ai/api/v1`**.

The spec also publicly ships two non-production servers: `https://stag.leftbrain.me/api/v1` (staging) and `https://app.rblocal.dev/api/v1` (local dev).

**Action:** override the base URL to `https://app.rightbrain.ai/api/v1` at generation time. Generating verbatim produces a CLI that cannot resolve its own host — every command would fail with a DNS error.

### Second spec-hygiene issue
33 operations sit under `/internal/*` and require the `internal.admin` scope (Rightbrain staff only): trial extension, signup webhooks, LLM model cost administration, org consumption reports, cache purge, Kratos/Hydra hooks. These are unreachable for every real user and would ship as ~33 dead commands. **Action:** exclude `/internal/*` from the generated surface. That leaves **312 user-reachable operations**.

## Top Workflows
1. **Run a task and get structured output.** `POST .../task/{task_id}/run` with `task_input`, optionally pinning `revision_id`/`revision_tag`, optionally attaching files. Read `response` (structured), plus token/timing/credit metrics. This is the platform's atomic unit and the #1 reason anyone touches the API.
2. **Run an agent conversationally and watch it work.** `POST .../task-agent/{agent_id}/run` returns `text/event-stream`. Events: `session_id`, `text`, `tool_call`, `tool_result`, `formatted_output`, `approval_required`, `error`, `done`. Multi-turn by passing `session_id` back. Runs end in `running`/`waiting_for_human`/`completed`/`failed`.
3. **Ship a change safely.** Author a revision → run the eval set against it → check the pass rate as a release gate → promote by weight (A/B) or pin a tag → roll back if the metrics move the wrong way.
4. **Unblock a paused agent.** An agent hits a gated tool, emits `approval_required`, and parks at `waiting_for_human`. Someone must list pending approval requests and approve/reject/resume them. Runs stall silently until they do.
5. **Account for spend and debug regressions.** Usage/timing/credit reports, reporting groups, run listings with token counts and `charged_credits`, and a cryptographically verifiable audit log.

## Table Stakes
Everything the incumbent `rightbrain` npm CLI v0.3.0 (published 2026-07-30, 472 downloads/month) does:
- `login` (browser OAuth, `--non-interactive`, `--url`), `logout`, `whoami`, `token --json`
- `init`, `switch-project`, `env list` / `env use`, `generate` (TypeScript type generation)
- `organization list`, `project list`
- `task list | get | create | update | run`
- `model list`
- Global `--env` / `RB_CLI_ENV`, JSON-when-piped, config file `rightbrain.json`

Plus what the rest of the first-party ecosystem provides:
- `@rightbrain/sdk`: `runTask` with `taskId`, `inputs`, `files`, `revisionId`, `useFallbackModel`, `accept`
- `rightbrain-agent-skill` / `rb-api.mjs`: generic authenticated HTTP passthrough with `--query --data --form --file --accept --output --sse --all --timeout --compact --dry-run --project-scope --org-scope`, typed exit codes 0/2/3/4/5
- Terraform provider: `rightbrain_task` resource, `model` data source
- GitHub Action: run a task with a task access token
- Bruno collection: API Keys, Forwarders, Input Processors, MCP, Models, OAuth Clients, Public Gateway, Tags, Tasks

**The gap this CLI exists to close:** the official CLI reaches roughly 15% of the API. It has *no* commands for agents, skills, collections, documents, triggers, evals, approvals, MCP servers, integrations, audit, or reporting. Those ~85% of 345 operations are reachable today only via raw curl or a generic HTTP passthrough script.

## Data Layer
- **Primary entities:** `task`, `task_agent`, `task_run`, `agent_run`, `skill`, `collection`, `document`, `trigger`, `revision`, `eval_set`, `eval_run`, `approval_request`, `mcp_server`, `integration`, `audit_event`, `model`, `api_key`, `project`, `org`.
- **Sync cursor:** `cursor` + `page_limit` on list endpoints; `created` timestamps on runs for incremental windows.
- **FTS/search:** run inputs/outputs (`run_data.submitted`, `response`), task names + prompts, agent names, skill content, audit event descriptions. Cross-entity search is impossible today — the dashboard has no global search and the API has no search endpoint.
- **Why local state pays off here:** runs carry `input_tokens`/`output_tokens`/`total_tokens`/`charged_credits`/`llm_call_timing` per call, but the API only ever returns a page at a time. Cost and latency questions ("which task burns the most credits", "did p95 regress after the revision flip") require joining many pages across many tasks — exactly what a local SQLite mirror makes trivial and the API makes tedious.

## Codebase Intelligence
- Source: ecosystem analysis of the RightbrainAI GitHub org (14 repos) + npm registry + the published OpenAPI spec.
- **Auth:** `Authorization: Bearer <token>`; env vars `RB_API_KEY` / `RB_ORG_ID` / `RB_PROJECT_ID` written by `rightbrain init` into `.env`. OAuth scopes seen in spec: `api.read`, `api.write`, `api.task`, `project:view/edit/admin`, `task:view/edit`, `task_agent:view/edit/run`, `skill:edit`, `mcp_server:view`, `organization:view/edit`, `internal.admin`.
- **Data model:** strictly org → project → resource. Both UUIDs appear in *every* path. IAM is per-resource-type (`.../iam/members`, `.../iam/test`) on org, project, task, and task-agent.
- **Rate limiting:** no rate-limit headers documented, no 429 in the documented status table. Credits are the metering unit (`charged_credits` per run), but no public rate card exists — `rightbrain.ai/pricing` returns 404.
- **Architecture:** FastAPI backend (422 validation shape, `<Lambda>` operation summaries on health routes), Ory Kratos/Hydra for identity, `/-/health` and `/-/ready` liveness routes. Streaming agent runs over SSE. Tamper-evident audit log with cryptographic integrity verification.
- **Ecosystem verdict:** zero third-party tooling exists. `gh search code "@rightbrain/sdk"` returns only Rightbrain's own repos. No Python, Go, Rust, Java, or Ruby SDK. This CLI would be the first non-first-party tool and the only one in Go.

## Product Thesis
- **Name:** `rightbrain-pp-cli`
- **Why it should exist:** The official CLI stops at tasks. Everything that makes Rightbrain a *production* platform — agents, approvals, evals, revisions, triggers, collections, audit — has no command-line surface at all, and no local state anywhere. This CLI covers all 312 user-reachable operations, mirrors runs into SQLite so credit spend, latency regressions, and eval pass rates become queryable across pages and across tasks, and unblocks the two things that silently stall production: agent runs parked on approval, and revisions promoted without an eval gate.

## Build Priorities
1. **Correct the spec before generating** — base URL to `https://app.rightbrain.ai/api/v1`, drop `/internal/*`, set canonical `RB_API_KEY` env var, declare cursor pagination and the `/whoami` verify path.
2. **Full typed surface over all 312 non-internal operations**, with org/project resolved from config so no user ever pastes two UUIDs into a command.
3. **Local mirror + sync** for tasks, agents, runs, skills, collections, evals, audit events — the substrate every transcendence feature stands on.
4. **Absorb the incumbent** — every `rightbrain` npm CLI command, matched and beaten with `--json`, `--select`, `--dry-run`, typed exit codes, and offline operation.
5. **Agent-run ergonomics** — SSE streaming rendered readably, multi-turn sessions, and approval triage.
6. **Transcendence** — the cross-page, cross-task analyses no existing tool can perform.

## Reachability Gate
- Decision: **PASS**
- Reason: `GET /api/v1/whoami` → 401 (expected, no key supplied); `GET /api/v1/openapi.json` → 200. Auth-required API, no blocking, no bot protection.

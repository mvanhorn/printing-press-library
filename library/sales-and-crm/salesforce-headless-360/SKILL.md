---
name: pp-salesforce-headless-360
description: "The agent-context packager for Salesforce. Signed, FLS-safe, cross-surface bundles any agent can consume. Trigger phrases: `customer context for salesforce`, `bundle salesforce account`, `salesforce meeting prep`, `verify salesforce bundle`, `freshness score salesforce`, `use salesforce-headless-360`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["salesforce-headless-360-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/salesforce-headless-360-pp-cli/...@latest","bins":["salesforce-headless-360-pp-cli"],"label":"Install via go install"}]}}'
---

# Salesforce Headless 360

Use this skill when an agent needs portable Salesforce Customer 360 context, signed bundle verification, account freshness scoring, or Slack injection with Salesforce audience safety.

## Killer Commands

```bash
# Build one signed Customer 360 bundle for an agent.
salesforce-headless-360-pp-cli agent context 001xx000003DGb2AAG --since P90D --output acme.json

# Verify before an agent trusts or acts on a bundle.
salesforce-headless-360-pp-cli agent verify acme.json --strict

# Post a field-gated summary into Slack after channel-audience FLS intersection.
salesforce-headless-360-pp-cli agent inject --slack C123456 --bundle acme.json

# Preview then patch one Salesforce record with a signed, audited intent.
salesforce-headless-360-pp-cli agent update 001xx000003DGb2AAG --org prod \
  --field Description="Agent follow-up scheduled" \
  --dry-run --json
salesforce-headless-360-pp-cli agent update 001xx000003DGb2AAG --org prod \
  --field Description="Agent follow-up scheduled" \
  --json

# Retry-safe create-or-update through the SF360 idempotency key.
salesforce-headless-360-pp-cli agent upsert --org prod \
  --sobject Account \
  --idempotency-key "sf360:account:acme-renewal:2026-04-22" \
  --field Name="Acme Renewal" \
  --field Description="Created by signed agent workflow" \
  --json

# Agent-typical workflow verbs.
salesforce-headless-360-pp-cli agent log-activity --type call --what 001xx000003DGb2AAG --subject "Renewal call completed" --idempotency-key "call:001xx000003DGb2AAG:2026-04-22" --org prod
salesforce-headless-360-pp-cli agent advance --opp 006xx000001ABCDE --stage "Proposal/Price Quote" --org prod
salesforce-headless-360-pp-cli agent close-case --case 500xx000001ABCDE --resolution "Resolved by renewal workflow" --org prod
salesforce-headless-360-pp-cli agent note --entity 001xx000003DGb2AAG --text "Agent summary posted after strict bundle verification." --org prod

# Multi-agent approval: propose, countersign, execute.
salesforce-headless-360-pp-cli agent plan update 001xx000003DGb2AAG --org prod \
  --field Description="Pending approver review" \
  --output /tmp/acme-write-plan.json
salesforce-headless-360-pp-cli agent sign-plan /tmp/acme-write-plan.json
salesforce-headless-360-pp-cli agent execute-plan /tmp/acme-write-plan.json --require-countersignatures 1

# Inspect local write forensics.
salesforce-headless-360-pp-cli agent write-audit list --status executed
salesforce-headless-360-pp-cli agent write-audit inspect <jti>
salesforce-headless-360-pp-cli agent write-audit verify <jti>
```

## Known Limitations

- **Task and Event writes do not persist idempotency keys.** `agent log-activity` creates Tasks without an `SF360_Idempotency_Key__c` field (Activity-object metadata restrictions deferred this to v1.2). Agents should treat Task retries as potentially duplicating until resolved. See `docs/findings/2026-04-24-live-verify-findings.md#finding-f-008`.
- **`--org <alias>` support varies by command.** `trust register` requires it. Most other commands resolve the target org via `sf config get target-org` (set once per session: `sf config set target-org=<alias>`). See `docs/plans/2026-04-24-001-fix-sf360-live-verify-findings-plan.md` Phase 1 for the planned global-flag unification.
- **`FLS.AllowFieldWrite(user)` currently discards the `user` parameter.** `--run-as-user` is enforced only at the Apex companion layer. Writes through the UI API path without the Apex companion use the acting token holder's FLS, not the `--run-as-user` target's. See `docs/findings/2026-04-24-live-verify-findings.md#finding-f-004`.

## Safety Notes

- FLS enforcement is always on before Salesforce records enter bundles, sync output, Data Cloud enrichment, or Slack linkage summaries.
- Write FLS and CRUD enforcement is always on before fields enter an update, create, upsert, activity, workflow, or Chatter note payload.
- Use `--dry-run` first for every write path. Dry-run validates and renders the payload without DML or write-audit emission.
- `agent create`, `agent upsert`, and retryable workflow writes need stable `--idempotency-key` values. Prefer opaque intent hashes; do not put PII or customer business identifiers in keys.
- MCP write tools refuse mutation unless `confirm:true` is passed. Keep `dry_run:true` for preview calls and add `confirm:true` only after checking the payload.
- JWT auth requires `agent context --run-as-user <UserId>` for bundle emission; integration-user permissions are not treated as the human user's FLS boundary.
- JWT writes require `--run-as-user <UserId>` and the Apex write companion because integration-user permissions are not a safe write boundary.
- Bulk writes are gated by `--confirm-bulk <N>` and the value must match the computed record count exactly.
- Slack inject re-FLSes the bundle against the Slack channel audience and blocks unmapped or external members unless the caller explicitly waives that guard.
- `doctor` is local and has no telemetry; use `doctor --mock` as the first smoke test when auth or infrastructure is uncertain.
- If doctor detects Agentforce MCP or Salesforce DX MCP, prefer those tools for tasks they cover directly.

## When To Use Each Verb

Use `context` for broad cross-surface Customer 360 packaging: Account, related CRM records, files, optional Data Cloud profile, and linked Slack context in one signed artifact.

Use `brief` for a narrow one-opportunity handoff when a human or agent needs deal context without a full account bundle.

Use `decay` for freshness triage. It returns a score and signal breakdown that agents can sort or branch on.

Use `verify` before trusting a bundle. Add `--strict` when the next step can mutate systems or expose data.

Use `inject` only when the target Slack audience is the intended audience. It is for collaboration handoff, not bulk publishing.

Use `update`, `create`, and `upsert` for single-record CRM mutation. Prefer `upsert` with a stable idempotency key when an agent, script, or MCP client may retry.

Use `log-activity`, `advance`, `close-case`, and `note` for common agent workflows. They are convenience verbs over the same signed write intent, FLS filter, audit writer, and D9 error envelope.

Use `plan`, `sign-plan`, and `execute-plan` when a proposed write needs another agent or human-controlled key to countersign before execution.

Use `write-audit` when investigating what an agent wrote, which `kid` signed it, whether the intent JWS still verifies, and whether execution ended as `executed`, `rejected`, or `conflict`.

## Install

```bash
go install github.com/mvanhorn/salesforce-headless-360-pp-cli/...@latest
salesforce-headless-360-pp-cli doctor --mock
```

## Authentication

```bash
salesforce-headless-360-pp-cli auth login --sf prod
salesforce-headless-360-pp-cli auth login --web --client-id "$SF_CLIENT_ID" --org sandbox
salesforce-headless-360-pp-cli auth login --jwt --org ci
```

Run `salesforce-headless-360-pp-cli doctor` after auth. The doctor rows show REST, Data Cloud, Slack linkage, Slack Web API, trust key store, Apex companion, local store, sf CLI passthrough, and competing-tool status.

## Direct Use

If the user provides arguments, run the CLI with those arguments. Prefer `--agent` for machine-readable output unless the user asks for human output.

```bash
salesforce-headless-360-pp-cli <args> --agent
```

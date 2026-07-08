# Salesforce Headless 360 CLI

**Portable Salesforce Customer 360 context for agents: signed, FLS-gated, locally inspectable, and Slack-aware.**

Salesforce has strong APIs, Agentforce MCP, and Salesforce DX MCP. Those tools are often the right answer. Salesforce Headless 360 exists for the narrower job they do not center: emitting one verifiable Customer 360 artifact that an agent can read offline, verify before use, and inject into collaboration surfaces without widening the audience. Run `doctor` first; if Agentforce MCP or DX MCP already covers the job, this CLI will say so.

No telemetry. `doctor` is local. All logs are local.

## Install

```bash
go install github.com/mvanhorn/salesforce-headless-360-pp-cli/...@latest
```

The primary binary is `salesforce-headless-360-pp-cli`. The MCP server binary is `salesforce-headless-360-pp-mcp`.

## Quick Start

This path verifies the CLI in under a minute without a Salesforce account:

```bash
salesforce-headless-360-pp-cli doctor --mock
salesforce-headless-360-pp-cli agent context 001ACME0001 --dry-run --json
salesforce-headless-360-pp-cli which "verify a signed salesforce bundle"
```

Expected result: `doctor --mock` reports green rows for REST, Data Cloud, Slack linkage, Slack Web API, trust key store, Apex companion, local store, sf CLI passthrough, and competing-tool checks.

## Killer Commands

- `agent context` packages one Account plus Contacts, Opportunities, Cases, Tasks, Events, Chatter, Files, optional Data Cloud profile, and linked Slack channels into a signed bundle.
- `agent verify` checks the bundle JWS, expiration, manifest SHA, live key status, and optional ContentVersion byte hashes before an agent trusts the file.
- `agent inject` posts a bundle summary to Slack only after intersecting Salesforce FLS across the Slack channel audience.

```bash
salesforce-headless-360-pp-cli agent context 001xx000003DGb2AAG --since P90D --output acme.json
salesforce-headless-360-pp-cli agent verify acme.json --strict
salesforce-headless-360-pp-cli agent inject --slack C123456 --bundle acme.json
```

## Agent Writes

v1.1 makes the trust substrate active. Agents can update, upsert, create, log activities, advance opportunities, close cases, and post Chatter notes, with every action signed, audited, FLS-safe, and tied back to the acting key.

Install the CLI and MCP server:

```bash
go install github.com/mvanhorn/salesforce-headless-360-pp-cli/...@latest
salesforce-headless-360-pp-cli doctor --mock
salesforce-headless-360-pp-cli trust register --org prod
```

Start every write with `--dry-run`; it validates the payload and shows what would be sent without DML or write-audit emission.

```bash
salesforce-headless-360-pp-cli agent update 001xx000003DGb2AAG --org prod \
  --field Description="Agent follow-up scheduled" \
  --dry-run --json

salesforce-headless-360-pp-cli agent update 001xx000003DGb2AAG --org prod \
  --field Description="Agent follow-up scheduled" \
  --json
```

Use `agent upsert` when retries are possible. The `--idempotency-key` is written to `SF360_Idempotency_Key__c`, so the same agent intent can be retried without creating duplicate records.

```bash
salesforce-headless-360-pp-cli agent upsert --org prod \
  --sobject Account \
  --idempotency-key "sf360:account:acme-renewal:2026-04-22" \
  --field Name="Acme Renewal" \
  --field Description="Created by signed agent workflow" \
  --json
```

Convenience verbs cover the actions agents usually need during a customer workflow:

```bash
salesforce-headless-360-pp-cli agent log-activity --type call --what 001xx000003DGb2AAG --subject "Renewal call completed" --idempotency-key "call:001xx000003DGb2AAG:2026-04-22" --org prod
salesforce-headless-360-pp-cli agent advance --opp 006xx000001ABCDE --stage "Proposal/Price Quote" --org prod
salesforce-headless-360-pp-cli agent close-case --case 500xx000001ABCDE --resolution "Resolved by renewal workflow" --org prod
salesforce-headless-360-pp-cli agent note --entity 001xx000003DGb2AAG --text "Agent summary posted after strict bundle verification." --org prod
```

Plan mode supports multi-agent approval: one agent proposes, another countersigns, and an executor runs the signed plan.

```bash
salesforce-headless-360-pp-cli agent plan update 001xx000003DGb2AAG --org prod \
  --field Description="Pending approver review" \
  --output /tmp/acme-write-plan.json
salesforce-headless-360-pp-cli agent sign-plan /tmp/acme-write-plan.json
salesforce-headless-360-pp-cli agent execute-plan /tmp/acme-write-plan.json --require-countersignatures 1
```

Audit forensics are local and inspectable:

```bash
salesforce-headless-360-pp-cli agent write-audit list --status executed
salesforce-headless-360-pp-cli agent write-audit inspect <jti>
salesforce-headless-360-pp-cli agent write-audit verify <jti>
```

The write trust model, including UI API vs Apex path selection, idempotency, concurrency, and plan replay protection, is documented in [docs/security.md](docs/security.md). HIPAA sync-mode behavior for write audit failures is documented in [docs/hipaa.md](docs/hipaa.md). Every write, plan, and write-audit verb has MCP parity; see [SKILL.md](SKILL.md) for agent-facing command patterns.

## Unique Features

### Agent Context Packager

The core workflow is a signed JSON artifact rather than another live API session. An agent can receive `acme.json`, verify it offline, inspect provenance, and avoid making repeated Salesforce calls during reasoning.

### Trust And Compliance

The trust path combines device keys, Salesforce-hosted public keys, bundle audit rows, compliance redaction counters, and optional deep file-byte verification. The security design is documented in [docs/security.md](docs/security.md) instead of being left as README shorthand.

### Competing-Tool Yield

Doctor treats Agentforce MCP and Salesforce DX MCP as first-class neighboring tools. When either one is configured, the CLI reports that you may not need this CLI for overlapping work.

## Authentication

- `auth login --sf <alias>` reuses Salesforce CLI credentials and requires `sf` version `2.60.0` or newer.
- `auth login --web --client-id <id>` uses a loopback OAuth web flow with PKCE.
- `auth login --jwt --org <alias>` supports CI, but bundle emission requires `agent context --run-as-user <UserId>` so reads are scoped to the acting user.

> **Note on `--org`:** Command examples below use `--org prod` for clarity, but the CLI currently resolves the target org via `sf config set target-org=<alias>` preflight for most commands. A global `--org` persistent flag is planned — see [`docs/plans/2026-04-24-001-fix-sf360-live-verify-findings-plan.md`](docs/plans/2026-04-24-001-fix-sf360-live-verify-findings-plan.md) Phase 1. If a command rejects `--org`, set the default org with `sf config set target-org=<alias>` and retry without the flag.

### Auth targets

| Target | Login command |
|---|---|
| Production | `sf org login web --alias prod` |
| Sandbox (generic) | `sf org login web --alias sandbox --instance-url https://test.salesforce.com` |
| Sandbox (My Domain) | `sf org login web --alias sandbox --instance-url https://<mydomain>--<sandbox>.sandbox.my.salesforce.com` |
| Developer Edition | `sf org login web --alias de` |

```bash
salesforce-headless-360-pp-cli auth login --sf prod
salesforce-headless-360-pp-cli auth login --web --client-id "$SF_CLIENT_ID" --org sandbox
SF360_AUTH_METHOD=jwt salesforce-headless-360-pp-cli agent context 001xx000003DGb2AAG --run-as-user 005xx00000ABCDE
```

Profiles are stored under `~/.config/pp/salesforce-headless-360/profiles/`. Token material is local.

## Commands

| Command | Purpose |
| --- | --- |
| `accounts get`, `accounts list` | Read Account records through Salesforce REST-style access. |
| `contacts get`, `contacts list` | Read Contact records linked to accounts or queries. |
| `opportunities get`, `opportunities list` | Read deal records for bundle and brief workflows. |
| `cases list`, `tasks list`, `events list` | Read service and activity context for Customer 360 assembly. |
| `sync` | Hydrate the local SQLite store for offline bundle assembly and analytics. |
| `search` | Search locally synced Salesforce context. |
| `analytics` | Aggregate locally synced records without another Salesforce round trip. |
| `trust register`, `trust list-keys`, `trust revoke` | Manage bundle signing keys and org trust roots. |
| `agent context`, `agent brief`, `agent decay`, `agent verify`, `agent inject` | Build, summarize, score, verify, and safely share agent-context artifacts. |
| `doctor` | Check local readiness and optional source availability. |
| `which` | Resolve a natural-language capability query to the best CLI command. |

Run `salesforce-headless-360-pp-cli --help` or `salesforce-headless-360-pp-cli <command> --help` for flags.

## Output Formats

Human terminal output is optimized for scanning. Piped or agent-oriented output should use `--json`, `--compact`, or `--agent`.

```bash
salesforce-headless-360-pp-cli accounts list --json
salesforce-headless-360-pp-cli accounts list --json --select id,name,owner.name
salesforce-headless-360-pp-cli agent decay --account 001xx000003DGb2AAG --json
```

Commands that read data return a provenance envelope when applicable. Parse `.results` for records and `.meta.source` to distinguish live Salesforce reads from local cache reads.

## Cookbook

### Verify Before Acting

```bash
salesforce-headless-360-pp-cli trust register --org prod
salesforce-headless-360-pp-cli agent context 001xx000003DGb2AAG --since P30D --output acme.json
salesforce-headless-360-pp-cli agent verify acme.json --strict
```

### Meeting Prep

```bash
salesforce-headless-360-pp-cli agent brief --opp 006xx000001ABCDE --json
salesforce-headless-360-pp-cli agent context 001xx000003DGb2AAG --dry-run
```

### Freshness Triage

```bash
salesforce-headless-360-pp-cli sync --account 001xx000003DGb2AAG
salesforce-headless-360-pp-cli agent decay --account 001xx000003DGb2AAG --json
salesforce-headless-360-pp-cli analytics --type opportunities --group-by stage --limit 10
```

### Insight-Native Agent Commands

`agent decay` returns a freshness score and signal breakdown. `agent brief` returns narrative markdown plus structured JSON. `analytics` aggregates synced data locally so agents can reason over a stable cache instead of paginating live APIs.

## Agent Usage

`--agent` expands to machine-oriented defaults: JSON output, compact formatting, no prompts, no color, and affirmative confirmation for commands that need explicit non-interactive approval. Use it when an LLM, MCP client, shell script, or scheduled job is the caller.

```bash
salesforce-headless-360-pp-cli agent context 001xx000003DGb2AAG --dry-run --agent
salesforce-headless-360-pp-cli doctor --agent
salesforce-headless-360-pp-cli agent-context --pretty
```

## FLS And Trust Model

- FLS enforcement is applied before records enter bundles, sync output, Data Cloud enrichment, or Slack linkage summaries.
- Device-scoped Ed25519 keys sign bundles, while org-registered public keys live in Salesforce Certificate records when available and CMDT records as a fallback.
- File-byte attestation stores ContentVersion SHA-256 values in the manifest, and `agent verify --deep` re-hashes bytes when Salesforce auth is available.
- JWT and Bulk paths require the Apex companion or `--run-as-user` guard because integration-user permissions are not a safe FLS boundary.

Read the full trust and threat model in [docs/security.md](docs/security.md). HIPAA deployment guidance is in [docs/hipaa.md](docs/hipaa.md).

## MCP Usage

- `salesforce-headless-360-pp-mcp` exposes agent-context, brief, decay, verify, refresh, and doctor tools for MCP-compatible clients.
- `doctor` detects Agentforce MCP and Salesforce DX MCP via environment variables and local MCP registries, then frames overlapping coverage as "you may not need this CLI."

Claude Code example:

```bash
salesforce-headless-360-pp-cli auth login --sf prod
claude mcp add salesforce-headless-360 salesforce-headless-360-pp-mcp
```

Claude Desktop example:

```json
{
  "mcpServers": {
    "salesforce-headless-360": {
      "command": "salesforce-headless-360-pp-mcp"
    }
  }
}
```

## Editions Supported

| Edition | Supported | Notes |
| --- | --- | --- |
| Enterprise | Yes | Certificate-backed trust registration is the preferred path. |
| Unlimited | Yes | Same trust and audit model as Enterprise. |
| Developer Edition | Yes | Good for local validation, mock flows, and package testing. |
| Professional Edition | Caveat | Certificate APIs may be unavailable; use CMDT fallback only after reviewing [docs/security.md](docs/security.md). |

## Exit Codes

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `2` | Usage error |
| `3` | Resource not found |
| `4` | Authentication required |
| `5` | Salesforce or upstream API error |
| `7` | Rate limited |
| `10` | Configuration error |

## Troubleshooting

- `doctor --mock` should be your first smoke test; it removes Salesforce, Slack, local keystore, and sf CLI availability from the equation.
- `SF360.AUTH.JWT_NO_RUN_AS_USER` means JWT mode tried to emit a bundle without a user-scoping flag.
- `agent verify` with an unknown key means the bundle signer is not present in the local keystore; run `trust register --org <alias>`.
- Data Cloud, Slack linkage, and Slack Web API yellow rows mean optional enrichments are unavailable, not that REST Customer 360 reads are blocked.

### Sandbox auth gotchas

- **Sandbox password is snapshot from prod at provisioning time.** If you rotated your prod password after the sandbox was created, the sandbox has the old one. Use the sandbox login page's "Forgot Your Password?" link — the reset email is delivered to your real prod inbox (Salesforce strips the `.<sandbox-name>` suffix during mail delivery).
- **The "Log In" shortcut from Setup → Sandboxes does not always carry prod SSO.** On some orgs it drops you at the sandbox password form. If that happens, use the Forgot Password path instead.
- **Some orgs require Sandbox Access public-group selection when creating a sandbox.** If the New Sandbox form rejects with "Enter a valid public group name," create a public group containing yourself first (Setup → Public Groups → New, 30 seconds), then retry.

### Running verification against a real org

The canonical runbook is [`docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md`](docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md). Before running it:

1. Deploy the CLI's metadata to your sandbox: `sf project deploy start --source-dir metadata --target-org <alias>`.
2. Assign the permission set: `sf org assign permset --name SF360_Key_Registrar --target-org <alias>`.
3. Seed test fixtures: `ORG=<alias> bash scripts/seed-run.sh`.
4. Export env vars and run: `bash scripts/live-verify.sh`.

The most recent live-verification report against a real Developer sandbox is in [`docs/live-verification-report.md`](docs/live-verification-report.md). Detailed findings from that run are in [`docs/findings/2026-04-24-live-verify-findings.md`](docs/findings/2026-04-24-live-verify-findings.md).

## FAQ

### Does this replace Agentforce MCP or Salesforce DX MCP?

No. If those tools cover your task directly, use them. This CLI is for portable signed context bundles, offline verification, and Slack audience-safety workflows.

### Where is the live verification report?

At [docs/live-verification-report.md](docs/live-verification-report.md). The first run against a real Developer sandbox was completed 2026-04-24 by Trent Matthias (NFC). The runbook used is [docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md](docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md), and 20 findings that surfaced during that run are catalogued in [docs/findings/2026-04-24-live-verify-findings.md](docs/findings/2026-04-24-live-verify-findings.md) with fix plans at [docs/plans/2026-04-24-001-fix-sf360-live-verify-findings-plan.md](docs/plans/2026-04-24-001-fix-sf360-live-verify-findings-plan.md).

### Does this CLI send telemetry?

No. Feedback, doctor output, logs, tokens, profiles, keystores, bundle audit cache rows, and generated bundles are local unless you explicitly configure an outbound sink or Slack injection.

### How do README claims map to code?

See [docs/README-claim-map.md](docs/README-claim-map.md). Every bullet claim in this README maps to an implementation path and a test path.

## Sources And Inspiration

Salesforce Headless 360 is designed to coexist with Salesforce's official APIs, Salesforce CLI, Agentforce MCP, and Salesforce DX MCP. The trust posture is documented so an operator can decide when this CLI is unnecessary.

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

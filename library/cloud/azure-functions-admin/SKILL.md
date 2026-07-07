---
name: pp-azure-functions-admin
description: "Inspect, audit, and right-size your Azure Functions from the terminal — cold-start trends, config drift Trigger phrases: `check my azure functions`, `are my functions cold-starting`, `should I move off the consumption plan`, `audit my function app settings`, `find unused azure functions`, `use azure-functions-admin`, `run azure-functions-admin`."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - azure-functions-admin-pp-cli
    install:
      - kind: go
        bins: [azure-functions-admin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/azure-functions-admin/cmd/azure-functions-admin-pp-cli
---

# Azure Functions — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `azure-functions-admin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install azure-functions-admin --cli-only
   ```
2. Verify: `azure-functions-admin-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/azure-functions-admin/cmd/azure-functions-admin-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you need to understand or audit Azure Functions you already run: take inventory across a subscription, decide whether to move an app off the Consumption plan to kill cold starts, audit app settings for leaked secrets or dev/prod drift, or triage which function is failing. It is the right tool for read-only inspection and analysis, especially for agents and scripts that want JSON output and a local cache.

## Anti-triggers

Do not use this CLI for:
- Deploying or publishing function code — use `func azure functionapp publish` or `az functionapp deployment`.
- Creating, restarting, scaling, or deleting function apps — this CLI is read-only by design.
- Changing app settings or rotating keys — use `az functionapp config appsettings set` / `az functionapp keys set`.
- Building a new Functions project — use the Azure Functions Core Tools (`func init`).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cold-start & plan-fit observatory
- **`coldstart`** — See how often your functions cold-start and how slow those starts are, with the projected cost of moving to a Premium always-ready plan.

  _Reach for this to answer 'are cold starts hurting us, and is Premium worth it' instead of guessing from anecdotes._

  ```bash
  azure-functions-admin coldstart --app my-function-app --agent
  ```
- **`scaling`** — Track instance scale-out and execution-time drift for an app over a time window, flagging when p95 duration creeps above baseline.

  _Reach for this to catch a slow-degrading function before it pages you._

  ```bash
  azure-functions-admin scaling --app my-function-app --window 7d --agent
  ```
- **`plan-fit`** — Per-app recommendation of Consumption vs Premium vs Dedicated across a resource group, based on invocation density, cold-start sensitivity, and duration.

  _Reach for this when deciding how to host a fleet of functions cost-effectively._

  ```bash
  azure-functions-admin plan-fit --resource-group my-resource-group --agent
  ```

### Config hygiene & secret hygiene
- **`drift`** — Diff app settings across function apps and their deployment slots, flagging settings present in one environment but not another and plaintext values that should be Key Vault references.

  _Reach for this before a release to catch dev/prod config divergence._

  ```bash
  azure-functions-admin drift --resource-group my-resource-group --agent
  ```
- **`secrets-audit`** — Flag app settings holding raw secret values instead of @Microsoft.KeyVault references, plus stale or unused function keys.

  _Reach for this to find leaked secrets and tighten config hygiene._

  ```bash
  azure-functions-admin secrets-audit --app my-function-app --agent
  ```

### Operational triage
- **`failures`** — Cluster recent invocation failures by function and exception type from Application Insights over a time window.

  _Reach for this to triage which function and which exception is failing most._

  ```bash
  azure-functions-admin failures --app my-function-app --since 24h --agent
  ```
- **`stale`** — Find functions with zero invocations in the last N days as cleanup candidates.

  _Reach for this to prune dead functions and shrink your attack surface._

  ```bash
  azure-functions-admin stale --days 90 --agent
  ```

## Command Reference

**apps** — Inspect Azure Function Apps (Microsoft.Web/sites where kind~functionapp)

- `azure-functions-admin-pp-cli apps get` — Get a single Function App
- `azure-functions-admin-pp-cli apps list` — List Function Apps in a subscription (filter kind~functionapp)

**functions** — List functions within a Function App

- `azure-functions-admin-pp-cli functions <subscriptionId> <resourceGroup> <name>` — List functions in a Function App

**plans** — List App Service / hosting plans (serverfarms)

- `azure-functions-admin-pp-cli plans <subscriptionId>` — List hosting plans (tier decode: Y1=Consumption, EP*=Premium, P*=Dedicated)

**settings** — Read Function App application settings

- `azure-functions-admin-pp-cli settings <subscriptionId> <resourceGroup> <name>` — List application settings (values are masked by the analysis layer)

**slots** — List deployment slots for a Function App

- `azure-functions-admin-pp-cli slots <subscriptionId> <resourceGroup> <name>` — List deployment slots

**subscriptions** — List Azure subscriptions reachable by the current credential

- `azure-functions-admin-pp-cli subscriptions` — List accessible subscriptions


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
azure-functions-admin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Cold-start check before a plan decision

```bash
azure-functions-admin coldstart --app <app> --agent
```

Returns cold-start frequency, p50/p95 cold-start latency, and the projected always-ready cost so you can decide on Premium with numbers, not vibes.

### Audit a whole resource group for plan fit

```bash
azure-functions-admin plan-fit --resource-group <rg> --agent
```

One call ranks every app's ideal plan from its real invocation density and cold-start sensitivity.

### Find leaked secrets in app settings

```bash
azure-functions-admin secrets-audit --app <app> --agent
```

Flags settings holding raw secret values that should be @Microsoft.KeyVault references, plus unused keys.

### Trim a verbose response for an agent

```bash
azure-functions-admin apps list --agent --select value.name,value.location,value.kind,value.properties.state
```

App-list responses are large and deeply nested; --select with dotted paths returns only the fields an agent needs, saving context.

### Triage today's failures

```bash
azure-functions-admin failures --app <app> --since 24h --agent
```

Clusters the last day of invocation failures by function and exception type so you fix the biggest one first.

## Auth Setup

This CLI talks to Azure Resource Manager using a standard Azure credential, the same way the official tools do. The simplest setup for a newcomer:

1. Have an Azure subscription with at least one Function App.
2. Create a read-only service principal (run once, in the Azure Cloud Shell or after `az login`):
   `az ad sp create-for-rbac --name azure-functions-admin --role Reader --scopes /subscriptions/<your-subscription-id>`
   That prints an `appId`, `password`, and `tenant`.
3. Export them so the CLI can read them:
   `export AZURE_TENANT_ID=<tenant>`
   `export AZURE_CLIENT_ID=<appId>`
   `export AZURE_CLIENT_SECRET=<password>`
   `export AZURE_SUBSCRIPTION_ID=<your-subscription-id>`
4. Run `azure-functions-admin doctor` to confirm everything connects.

The `Reader` role is enough for every command except `keys`, which lists function/host keys — that needs the `Website Contributor` role (or a custom role granting `Microsoft.Web/sites/host/listkeys/action`). If you are already signed in with `az login`, the CLI will use that session automatically and you can skip the service principal.

Run `azure-functions-admin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  azure-functions-admin-pp-cli apps list mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
azure-functions-admin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
azure-functions-admin-pp-cli feedback --stdin < notes.txt
azure-functions-admin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/azure-functions-admin-pp-cli/feedback.jsonl`. They are never POSTed unless `AZURE_FUNCTIONS_ADMIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AZURE_FUNCTIONS_ADMIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
azure-functions-admin-pp-cli profile save briefing --json
azure-functions-admin-pp-cli --profile briefing apps list mock-value
azure-functions-admin-pp-cli profile list --json
azure-functions-admin-pp-cli profile show briefing
azure-functions-admin-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `azure-functions-admin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/azure-functions-admin/cmd/azure-functions-admin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add azure-functions-admin-pp-mcp -- azure-functions-admin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which azure-functions-admin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   azure-functions-admin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `azure-functions-admin-pp-cli <command> --help`.

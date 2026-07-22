# Azure Functions CLI

**Inspect, audit, and right-size your Azure Functions from the terminal — cold-start trends, config drift, and plan-fit answers the portal and az don't give you.**

Azure Functions runs your code on demand without you managing servers. This CLI is a read-only inspector for the Function Apps in your Azure subscription: it lists your apps and functions, reads their settings and keys, and pulls invocation history from Application Insights into a local database. That local history unlocks answers a stateless `az` command can't give — like how often your functions cold-start, whether you should move off the Consumption plan, and where plaintext secrets are hiding in your app settings. It does not deploy code (use `func` or `az functionapp` for that); it tells you what your functions are doing and how to improve them.

Created by [@sdhilip200](https://github.com/sdhilip200) (Dhilip Subramanian).

## Install

The recommended path installs both the `azure-functions-admin-pp-cli` binary and the `pp-azure-functions-admin` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install azure-functions-admin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install azure-functions-admin --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install azure-functions-admin --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install azure-functions-admin --agent claude-code
npx -y @mvanhorn/printing-press-library install azure-functions-admin --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/azure-functions-admin/cmd/azure-functions-admin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/azure-functions-admin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install azure-functions-admin --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-azure-functions-admin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-azure-functions-admin --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install azure-functions-admin --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/azure-functions-admin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AZURE_TENANT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/azure-functions-admin/cmd/azure-functions-admin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "azure-functions-admin": {
      "command": "azure-functions-admin-pp-mcp",
      "env": {
        "AZURE_TENANT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

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

## Quick Start

```bash
# Confirms the CLI is installed and shows what a health check will verify (no credentials needed).
azure-functions-admin doctor --dry-run

# Lists every Function App in your subscription — your starting inventory.
azure-functions-admin apps list

# Pulls apps, functions, settings, and invocation history into the local database so analysis commands work offline and fast.
azure-functions-admin sync

# The headline answer for one app: how often it cold-starts and how slow those starts are (swap in your app name).
azure-functions-admin coldstart --app my-function-app --agent

# Per-app Consumption-vs-Premium-vs-Dedicated recommendation across your whole subscription (add --resource-group to scope it).
azure-functions-admin plan-fit --agent

```

## Unique Features

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

## Usage

Run `azure-functions-admin-pp-cli --help` for the full command reference and flag list.

## Commands

### apps

Inspect Azure Function Apps (Microsoft.Web/sites where kind~functionapp)

- **`azure-functions-admin-pp-cli apps get`** - Get a single Function App
- **`azure-functions-admin-pp-cli apps list`** - List Function Apps in a subscription (filter kind~functionapp)

### functions

List functions within a Function App

- **`azure-functions-admin-pp-cli functions <subscriptionId> <resourceGroup> <name>`** - List functions in a Function App

### plans

List App Service / hosting plans (serverfarms)

- **`azure-functions-admin-pp-cli plans <subscriptionId>`** - List hosting plans (tier decode: Y1=Consumption, EP*=Premium, P*=Dedicated)

### settings

Read Function App application settings

- **`azure-functions-admin-pp-cli settings <subscriptionId> <resourceGroup> <name>`** - List application settings (values are masked by the analysis layer)

### slots

List deployment slots for a Function App

- **`azure-functions-admin-pp-cli slots <subscriptionId> <resourceGroup> <name>`** - List deployment slots

### subscriptions

List Azure subscriptions reachable by the current credential

- **`azure-functions-admin-pp-cli subscriptions`** - List accessible subscriptions

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
azure-functions-admin-pp-cli apps list mock-value

# JSON for scripting and agents
azure-functions-admin-pp-cli apps list mock-value --json

# Filter to specific fields
azure-functions-admin-pp-cli apps list mock-value --json --select id,name,status

# Dry run — show the request without sending
azure-functions-admin-pp-cli apps list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
azure-functions-admin-pp-cli apps list mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
azure-functions-admin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/azure-functions-admin/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AZURE_TENANT_ID` | per_call | Yes | Set to your API credential. |
| `AZURE_CLIENT_ID` | per_call | Yes | Set to your API credential. |
| `AZURE_CLIENT_SECRET` | per_call | Yes | Set to your API credential. |
| `AZURE_SUBSCRIPTION_ID` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `azure-functions-admin-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `azure-functions-admin-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AZURE_TENANT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor fails with 'no credential found'** — Export AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, and AZURE_SUBSCRIPTION_ID, or run `az login` first.
- **apps list returns empty** — Check AZURE_SUBSCRIPTION_ID is correct and the service principal has Reader granted at the subscription scope, not just one resource group.
- **keys returns HTTP 403** — Listing keys needs the Website Contributor role: `az role assignment create --assignee <appId> --role 'Website Contributor' --scope /subscriptions/<id>`.
- **coldstart or metrics shows no data** — The app must have Application Insights enabled (an APPLICATIONINSIGHTS_CONNECTION_STRING app setting). Without it, there is no invocation telemetry to analyze.
- **429 Too Many Requests** — ARM throttles reads per subscription; the CLI backs off and retries automatically — re-run after a moment or narrow the resource group.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**azure-cli (az functionapp)**](https://github.com/Azure/azure-cli) — Python (4200 stars)
- [**azure-functions-core-tools**](https://github.com/Azure/azure-functions-core-tools) — C# (2200 stars)
- [**azure-sdk-for-go**](https://github.com/Azure/azure-sdk-for-go) — Go (1800 stars)
- [**azure-mcp**](https://github.com/Azure/azure-mcp) — C# (1500 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

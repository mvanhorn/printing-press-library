# BotSee CLI

**Have your agents measure and improve your brand's AI Visibility with botsee.io. Run audits across every major LLM, see what's missing, then get specific recommendations and ready-to-publish content to close the gap. API-first, agent-native**

BotSee is the API-first GEO platform; this CLI is the API-first front door. The flagship `ai-visibility-audit <url>` mirrors the proven Python create-site+analyze flow as a single idempotent Go command. The other 64 commands cover the entire BotSee surface (CRUD on sites/customer-types/personas/questions, the full analysis lifecycle, recommendations, blog content, billing, signup including USDC/x402, webhooks, api-key rotation, usage) plus framework commands (sync/search/sql/doctor) over a local SQLite cache.

Created by [@grahac](https://github.com/grahac) (grahac).

## Install

The recommended path installs both the `botsee-pp-cli` binary and the `pp-botsee` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install botsee
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install botsee --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install botsee --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install botsee --agent claude-code
npx -y @mvanhorn/printing-press-library install botsee --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/botsee/cmd/botsee-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/botsee-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install botsee --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-botsee --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-botsee --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install botsee --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/botsee-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BOTSEE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "botsee": {
      "command": "botsee-pp-mcp",
      "env": {
        "BOTSEE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

BotSee uses Bearer tokens prefixed `bts_live_`. Set `BOTSEE_API_KEY` in your environment and the CLI threads it into every authenticated call. Manage keys with `api-keys list / create / rotate / reset / delete` — `reset` consumes a one-time token emailed to the account holder if the key was lost. The CLI never logs key values.

## Quick Start

```bash
# Verify auth, reachability, and remaining rate-limit budget
botsee-pp-cli doctor

# Flagship — bootstrap a new site, run analysis, print results. Idempotent: a second run on the same domain reuses the existing site and just runs a fresh analysis.
botsee-pp-cli ai-visibility-audit example.com --types 2 --personas 2 --questions 5 --watch

# Inspect the customer-types / personas / questions tree for the site (with copy-paste edit commands)
botsee-pp-cli site-config --site $SITE_UUID --agent

# Generate next-step recommendations from the analysis (cached locally)
botsee-pp-cli recommendations $ANALYSIS_UUID --agent

# Cross-site cited-source rollup — useful once you've audited multiple domains
botsee-pp-cli sites-summary --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Flagship workflow
- **`ai-visibility-audit`** — One command runs the full BotSee audit: idempotent on existing sites, bootstraps customer types and personas and questions if missing, then runs analysis across every LLM and prints a structured visibility report.

  _This is THE command. Reach for it when an agent or user says 'audit my AI visibility' or 'check how AI sees example.com' — it handles the full lifecycle._

  ```bash
  botsee-pp-cli ai-visibility-audit example.com --types 2 --personas 2 --questions 5 --watch --agent
  ```
- **`recommendations`** — Generate next-step recommendations from an analysis — promoted to top-level so agents can discover it without descending into the analysis subcommand tree.

  _Use after an analysis to get LLM-generated action items without re-spending if you already pulled them once._

  ```bash
  botsee-pp-cli recommendations $ANALYSIS_UUID --agent
  ```

### Workflow plumbing
- **`site-config`** — Print the full customer-types -> personas -> questions tree for a site, with UUIDs and the actual edit commands users can copy-paste to add, update, or remove any node.

  _Reach for this when a user asks 'what is set up for this site' or 'show me my BotSee config' — it surfaces every UUID needed for follow-up edits._

  ```bash
  botsee-pp-cli site-config --site $SITE_UUID --agent
  ```
- **`sites-summary`** — Aggregate cited sources across every synced site, grouped by domain, with citation count, distinct sites citing each domain, and first-seen timestamp.

  _Use for multi-site users and agencies who need the cross-portfolio answer 'which sources keep getting cited everywhere' — impossible without local aggregation._

  ```bash
  botsee-pp-cli sites-summary --agent --select domain,citation_count,distinct_sites_citing,first_seen
  ```

## Usage

Run `botsee-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage account

- **`botsee-pp-cli account`** - Returns account details including company name, site count, and owner information.

### analysis

Manage analysis

- **`botsee-pp-cli analysis get`** - Returns analysis details including status. Poll this endpoint until status is 'completed' or 'failed'.
- **`botsee-pp-cli analysis run`** - Starts an analysis run. This is asynchronous - poll GET /api/v1/analysis/:uuid for status. Returns 202 Accepted.

### api-keys

Manage api keys

- **`botsee-pp-cli api-keys create`** - Creates a new API key. The raw key is returned only once in the response body — store it immediately.
- **`botsee-pp-cli api-keys delete`** - Revokes an API key. You cannot revoke the key being used to make this request — use rotate or another key.
- **`botsee-pp-cli api-keys list`** - Lists all API keys for the organization (raw key never returned).
- **`botsee-pp-cli api-keys reset`** - Exchanges a one-time reset token (from email) for a fresh API key. Public endpoint — no Bearer auth required, the token authenticates the request.

### billing

Manage billing

- **`botsee-pp-cli billing get-settings`** - Returns the organization's billing settings and current credit balance.
- **`botsee-pp-cli billing topoff-via-x402`** - Discovery call without payment headers returns 402 and does not require auth. Final paid retry requires API key (Authorization, x-api-key, or api_key query param) and supports both `payment` and `payment-signature` headers. Method-compatible with POST, PUT, PATCH, and DELETE.
- **`botsee-pp-cli billing update-settings`** - Updates the organization's monthly spend limit. Other settings are read-only via this endpoint.

### botsee-auth

Manage botsee auth

- **`botsee-pp-cli botsee-auth`** - Validates the API key and returns organization info and credit balance.

### customer-types

Manage customer types

- **`botsee-pp-cli customer-types delete`** - Archives a customer type. Returns 204 No Content on success.
- **`botsee-pp-cli customer-types get`** - Returns a customer type with its personas.
- **`botsee-pp-cli customer-types update`** - Updates a customer type. Only include fields you want to change.

### personas

Manage personas

- **`botsee-pp-cli personas delete`** - Archives a persona. Returns 204 No Content on success.
- **`botsee-pp-cli personas get`** - Returns a persona with its questions.
- **`botsee-pp-cli personas update`** - Updates a persona. Only include fields you want to change.

### pricing

Manage pricing

- **`botsee-pp-cli pricing`** - Returns the credit cost for each chargeable operation. Analysis costs are estimates; the actual debit is computed from real LLM usage at the documented multiplier after the run completes.

### questions

Manage questions

- **`botsee-pp-cli questions delete`** - Deletes a question. Returns 204 No Content on success.
- **`botsee-pp-cli questions update`** - Updates a question. Only include fields you want to change.

### rate-limits

Manage rate limits

- **`botsee-pp-cli rate-limits`** - Returns the caller's current rate-limit state without consuming additional budget. Useful for agents that want to back off proactively before hitting 429.

### signup

Manage signup

- **`botsee-pp-cli signup via-cc`** - Creates a credit-card signup token. USDC signups must use `/api/v1/signup/usdc`.
- **`botsee-pp-cli signup via-usdc-token`** - Creates a USDC signup token. Use `no_email: true` for autonomous agent flows (no setup_url returned).

### sites

Manage sites

- **`botsee-pp-cli sites create`** - Creates a new site. Auto-generates product_name and value_proposition from URL if not provided (5 credits).
- **`botsee-pp-cli sites delete`** - Archives a site. Returns 204 No Content on success.
- **`botsee-pp-cli sites get`** - Returns a site with its customer types and persona counts.
- **`botsee-pp-cli sites list`** - Returns a paginated list of sites for the organization.

### usage

Manage usage

- **`botsee-pp-cli usage by-key`** - Returns credit usage breakdown per API key.
- **`botsee-pp-cli usage get`** - Returns credit balance, auto-charge settings, and paginated transaction history.

### webhooks

Manage webhooks

- **`botsee-pp-cli webhooks create`** - Registers a webhook URL. Returns the webhook with its signing secret (shown only once).
- **`botsee-pp-cli webhooks delete`** - Deletes a webhook. Returns 204 No Content on success.
- **`botsee-pp-cli webhooks list`** - Lists all registered webhooks for the organization.
- **`botsee-pp-cli webhooks list-events`** - Returns the catalog of event types this API can emit, with JSON Schemas per event. Use this to self-discover available events programmatically without parsing the full OpenAPI doc.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
botsee-pp-cli account

# JSON for scripting and agents
botsee-pp-cli account --json

# Filter to specific fields
botsee-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
botsee-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
botsee-pp-cli account --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
botsee-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/botsee-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BOTSEE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `botsee-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BOTSEE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`doctor` fails with 401 Unauthorized** — Confirm `BOTSEE_API_KEY` is set and starts with `bts_live_`. Rotate via `botsee-pp-cli api-keys rotate <id>` if compromised.
- **`ai-visibility-audit` says site exists but I expected a fresh setup** — Default behavior is idempotent: reuses the existing site by normalized domain match. Pass `--regenerate` to force LLM re-generation of customer types and personas, or use a different domain.
- **Cost estimate looks too high or too low** — The estimator reads `cost_multiplier` from the live `/pricing` endpoint; if BotSee adjusts it, your estimates adjust automatically. Use `--estimate-only` to preview without spending.
- **429 Too Many Requests during sync** — The 600 req/min budget is per key. Add `--max-pages 5` to `sync`, or check the X-RateLimit-Reset header via `rate-limits get`.
- **USDC top-up via x402 hangs** — x402 challenges return a 402 with payment requirements; the CLI prints the `payTo` address. Pay from a wallet (Pinch, Coinbase CDP), then re-run with `--payment <proof>`.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

# Oura CLI

# Overview 
The Oura API allows Oura users and partner applications to improve their user experience with Oura data.
This document describes the Oura API Version 2 (V2), which is the only available integration point for Oura data. The previous V1 API has been sunset.
# Getting Started 
## What is an API?
An API (Application Programming Interface) allows different software applications to communicate with each other. The Oura API enables you to access your Oura Ring data programmatically.
## Quick Start Guide
1. Register an [API Application](https://cloud.ouraring.com/oauth/applications) and implement OAuth2
2. **Make Your First API Call**:
   ```
   curl -X GET https://api.ouraring.com/v2/usercollection/personal_info \
   -H "Authorization: Bearer YOUR_TOKEN_HERE"
   ```
3. **Explore Data Types**:
   - Browse the available endpoints in this documentation to discover what data you can access
   - Each endpoint includes example requests and responses
4. **Set Up Webhooks (Strongly Recommended)**:
   - Webhooks are the preferred way to consume Oura data
   - We have not had customers hit rate limits with webhooks properly implemented
   - Make a single request for historical data when a user first connects, then use webhooks for ongoing updates
   - Webhook notifications come approximately 30 seconds after data syncs from the mobile app
   - [Set up webhooks](#tag/Webhook-Subscription-Routes) to receive notifications when data changes
## Common Questions
- **Data Delay**: Different data types sync at different times - sleep data requires users to open the Oura app, while daily activity and stress may sync in the background
# Data Access
In order to access data, a registered [API Application](https://cloud.ouraring.com/oauth/applications) is required.
 API Applications are limited to **10** users before requiring approval from Oura. There is no limit once an application is approved.
 Additionally, Oura users **must provide consent** to share each data type an API Application has access to.
All data access requests through the Oura API require [Authentication](https://cloud.ouraring.com/docs/authentication).
Additionally, we recommend that Oura users keep their mobile app updated to support API access for the latest data types.
# Authentication
The Oura Cloud API supports authentication through the industry-standard OAuth2 protocol. For more information, see our [Authentication instructions](https://cloud.ouraring.com/docs/authentication).
Access tokens must be included in the request header as follows:
```http
GET /v2/usercollection/personal_info HTTP/1.1
Host: api.ouraring.com
Authorization: Bearer <token>
```
Please note that personal access tokens were deprecated in December 2025 and are no longer available for use.
# Oura HTTP Response Codes
| Response Code                        | Description |
| ------------------------------------ | - |
| 200 OK                               | Successful Response         |
| 400 Query Parameter Validation Error | The request contains query parameters that are invalid or incorrectly formatted. |
| 401 Unauthorized                     | Invalid or expired authentication token. |
| 403 Forbidden                        | The requested resource requires additional permissions or the user's Oura subscription has expired. |
| 429 Too Many Requests                | Rate limit exceeded. See response headers for retry guidance. |

## Rate Limits
The API enforces rate limits at two layers to ensure fair access across all applications:
- a per-access-token limit, which throttles single-token floods, and
- a per-application limit, which caps the aggregate traffic across all of an application's end-user tokens so one fan-out app can't dominate shared capacity.

A request that trips either layer receives a `429 Too Many Requests`. The `X-RateLimit-Tier` response header identifies which layer fired.

If your application regularly approaches rate limits, [webhooks](#tag/Webhook-Subscription-Routes) are strongly recommended — most applications that implement webhooks correctly do not encounter rate limit issues.

## Rate Limit Response Headers
When a `429 Too Many Requests` response is returned, five headers are included to guide retries. Prefer these over fixed-interval backoff:
- **`Retry-After`** — integer seconds to wait before retrying. RFC 7231-compliant; safe to feed directly into your client's backoff logic.
- **`X-RateLimit-Limit`** — the request ceiling for the current window.
- **`X-RateLimit-Window`** — the rolling window length in seconds that the ceiling applies to.
- **`X-RateLimit-Reset`** — Unix epoch (seconds) at which the window resets and quota is fully restored.
- **`X-RateLimit-Tier`** — identifies which limit was exceeded, useful when contacting support.

Learn more at [Oura](https://cloud.ouraring.com/v2/docs).

Created by [@coopdogGGs](https://github.com/coopdogGGs) (ryanc00per).

## Install

The recommended path installs both the `oura-pp-cli` binary and the `pp-oura` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install oura
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install oura --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install oura --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install oura --agent claude-code
npx -y @mvanhorn/printing-press-library install oura --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/oura/cmd/oura-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/oura-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install oura --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-oura --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-oura --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install oura --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/oura-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OURA_OAUTH2` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/oura/cmd/oura-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "oura": {
      "command": "oura-pp-mcp",
      "env": {
        "OURA_OAUTH2": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
oura-pp-cli usercollection multiple-daily-sleep-documents --json

oura-pp-cli usercollection multiple-daily-readiness-documents --json

oura-pp-cli usercollection multiple-daily-activity-documents --json

oura-pp-cli doctor

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`sync`** — Mirror Oura usercollection data into a local SQLite store for offline search and analysis
- **`search`** — FTS5 full-text search across locally synced Oura records
- **`analytics`** — Aggregate group-by and count queries over locally synced health data
- **`workflow`** — Chain multiple Oura API operations into one agent-friendly command

## Usage

Run `oura-pp-cli --help` for the full command reference and flag list.

## Commands

### sandbox

Manage sandbox

- **`oura-pp-cli sandbox multiple-daily-activity-documents`** - Sandbox - Multiple Daily Activity Documents
- **`oura-pp-cli sandbox multiple-daily-cardiovascular-age-documents`** - Sandbox - Multiple Daily Cardiovascular Age Documents
- **`oura-pp-cli sandbox multiple-daily-readiness-documents`** - Sandbox - Multiple Daily Readiness Documents
- **`oura-pp-cli sandbox multiple-daily-resilience-documents`** - Sandbox - Multiple Daily Resilience Documents
- **`oura-pp-cli sandbox multiple-daily-sleep-documents`** - Sandbox - Multiple Daily Sleep Documents
- **`oura-pp-cli sandbox multiple-daily-spo2-documents`** - Sandbox - Multiple Daily Spo2 Documents
- **`oura-pp-cli sandbox multiple-daily-stress-documents`** - Sandbox - Multiple Daily Stress Documents
- **`oura-pp-cli sandbox multiple-enhanced-tag-documents`** - Sandbox - Multiple Enhanced Tag Documents
- **`oura-pp-cli sandbox multiple-heartrate-documents`** - Sandbox - Multiple Heartrate Documents
- **`oura-pp-cli sandbox multiple-rest-mode-period-documents`** - Sandbox - Multiple Rest Mode Period Documents
- **`oura-pp-cli sandbox multiple-ring-battery-level-documents`** - Sandbox - Multiple Ring Battery Level Documents
- **`oura-pp-cli sandbox multiple-ring-configuration-documents`** - Sandbox - Multiple Ring Configuration Documents
- **`oura-pp-cli sandbox multiple-session-documents`** - Sandbox - Multiple Session Documents
- **`oura-pp-cli sandbox multiple-sleep-documents`** - Sandbox - Multiple Sleep Documents
- **`oura-pp-cli sandbox multiple-sleep-time-documents`** - Sandbox - Multiple Sleep Time Documents
- **`oura-pp-cli sandbox multiple-tag-documents`** - Sandbox - Multiple Tag Documents
- **`oura-pp-cli sandbox multiple-v-o2-max-documents`** - Sandbox - Multiple Vo2 Max Documents
- **`oura-pp-cli sandbox multiple-workout-documents`** - Sandbox - Multiple Workout Documents
- **`oura-pp-cli sandbox single-daily-activity-document`** - Sandbox - Single Daily Activity Document
- **`oura-pp-cli sandbox single-daily-cardiovascular-age-document`** - Sandbox - Single Daily Cardiovascular Age Document
- **`oura-pp-cli sandbox single-daily-readiness-document`** - Sandbox - Single Daily Readiness Document
- **`oura-pp-cli sandbox single-daily-resilience-document`** - Sandbox - Single Daily Resilience Document
- **`oura-pp-cli sandbox single-daily-sleep-document`** - Sandbox - Single Daily Sleep Document
- **`oura-pp-cli sandbox single-daily-spo2-document`** - Sandbox - Single Daily Spo2 Document
- **`oura-pp-cli sandbox single-daily-stress-document`** - Sandbox - Single Daily Stress Document
- **`oura-pp-cli sandbox single-enhanced-tag-document`** - Sandbox - Single Enhanced Tag Document
- **`oura-pp-cli sandbox single-rest-mode-period-document`** - Sandbox - Single Rest Mode Period Document
- **`oura-pp-cli sandbox single-ring-configuration-document`** - Sandbox - Single Ring Configuration Document
- **`oura-pp-cli sandbox single-session-document`** - Sandbox - Single Session Document
- **`oura-pp-cli sandbox single-sleep-document`** - Sandbox - Single Sleep Document
- **`oura-pp-cli sandbox single-sleep-time-document`** - Sandbox - Single Sleep Time Document
- **`oura-pp-cli sandbox single-tag-document`** - Sandbox - Single Tag Document
- **`oura-pp-cli sandbox single-v-o2-max-document`** - Sandbox - Single Vo2 Max Document
- **`oura-pp-cli sandbox single-workout-document`** - Sandbox - Single Workout Document

### usercollection

Manage usercollection

- **`oura-pp-cli usercollection multiple-daily-activity-documents`** - Multiple Daily Activity Documents
- **`oura-pp-cli usercollection multiple-daily-cardiovascular-age-documents`** - Multiple Daily Cardiovascular Age Documents
- **`oura-pp-cli usercollection multiple-daily-readiness-documents`** - Multiple Daily Readiness Documents
- **`oura-pp-cli usercollection multiple-daily-resilience-documents`** - Multiple Daily Resilience Documents
- **`oura-pp-cli usercollection multiple-daily-sleep-documents`** - Multiple Daily Sleep Documents
- **`oura-pp-cli usercollection multiple-daily-spo2-documents`** - Multiple Daily Spo2 Documents
- **`oura-pp-cli usercollection multiple-daily-stress-documents`** - Multiple Daily Stress Documents
- **`oura-pp-cli usercollection multiple-enhanced-tag-documents`** - Multiple Enhanced Tag Documents
- **`oura-pp-cli usercollection multiple-heartrate-documents`** - Multiple Heartrate Documents
- **`oura-pp-cli usercollection multiple-rest-mode-period-documents`** - Multiple Rest Mode Period Documents
- **`oura-pp-cli usercollection multiple-ring-battery-level-documents`** - Multiple Ring Battery Level Documents
- **`oura-pp-cli usercollection multiple-ring-configuration-documents`** - Multiple Ring Configuration Documents
- **`oura-pp-cli usercollection multiple-session-documents`** - Multiple Session Documents
- **`oura-pp-cli usercollection multiple-sleep-documents`** - Multiple Sleep Documents
- **`oura-pp-cli usercollection multiple-sleep-time-documents`** - Multiple Sleep Time Documents
- **`oura-pp-cli usercollection multiple-tag-documents`** - Multiple Tag Documents
- **`oura-pp-cli usercollection multiple-v-o2-max-documents`** - Multiple Vo2 Max Documents
- **`oura-pp-cli usercollection multiple-workout-documents`** - Multiple Workout Documents
- **`oura-pp-cli usercollection single-daily-activity-document`** - Single Daily Activity Document
- **`oura-pp-cli usercollection single-daily-cardiovascular-age-document`** - Single Daily Cardiovascular Age Document
- **`oura-pp-cli usercollection single-daily-readiness-document`** - Single Daily Readiness Document
- **`oura-pp-cli usercollection single-daily-resilience-document`** - Single Daily Resilience Document
- **`oura-pp-cli usercollection single-daily-sleep-document`** - Single Daily Sleep Document
- **`oura-pp-cli usercollection single-daily-spo2-document`** - Single Daily Spo2 Document
- **`oura-pp-cli usercollection single-daily-stress-document`** - Single Daily Stress Document
- **`oura-pp-cli usercollection single-enhanced-tag-document`** - Single Enhanced Tag Document
- **`oura-pp-cli usercollection single-personal-info-document`** - Single Personal Info Document
- **`oura-pp-cli usercollection single-rest-mode-period-document`** - Single Rest Mode Period Document
- **`oura-pp-cli usercollection single-ring-configuration-document`** - Single Ring Configuration Document
- **`oura-pp-cli usercollection single-session-document`** - Single Session Document
- **`oura-pp-cli usercollection single-sleep-document`** - Single Sleep Document
- **`oura-pp-cli usercollection single-sleep-time-document`** - Single Sleep Time Document
- **`oura-pp-cli usercollection single-tag-document`** - Single Tag Document
- **`oura-pp-cli usercollection single-v-o2-max-document`** - Single Vo2 Max Document
- **`oura-pp-cli usercollection single-workout-document`** - Single Workout Document

### webhook

Manage webhook

- **`oura-pp-cli webhook create`** - Create Webhook Subscription
- **`oura-pp-cli webhook delete`** - Delete Webhook Subscription
- **`oura-pp-cli webhook get-subscription`** - Get Webhook Subscription
- **`oura-pp-cli webhook list-subscriptions`** - List Webhook Subscriptions
- **`oura-pp-cli webhook renew-subscription`** - Renew Webhook Subscription
- **`oura-pp-cli webhook update`** - Update Webhook Subscription


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
oura-pp-cli sandbox multiple-daily-activity-documents

# JSON for scripting and agents
oura-pp-cli sandbox multiple-daily-activity-documents --json

# Filter to specific fields
oura-pp-cli sandbox multiple-daily-activity-documents --json --select id,name,status

# Dry run — show the request without sending
oura-pp-cli sandbox multiple-daily-activity-documents --dry-run

# Agent mode — JSON + compact + no prompts in one flag
oura-pp-cli sandbox multiple-daily-activity-documents --agent
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
oura-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/oura-documentation-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OURA_OAUTH2` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `oura-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `oura-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OURA_OAUTH2`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

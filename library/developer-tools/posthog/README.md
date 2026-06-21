# PostHog CLI

**Every PostHog resource in one CLI — with offline search, agent-native output, and cross-resource analytics no dashboard can do.**

posthog-pp-cli syncs your flags, insights, experiments, persons, errors, and LLM traces to a local SQLite store. Query anything offline, run compound analytics across resources the UI keeps separate, and pipe results directly to agents or scripts.

Created by [@riteshtiwari](https://github.com/riteshtiwari) (riteshtiwari).

## Install

The recommended path installs both the `posthog-pp-cli` binary and the `pp-posthog` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install posthog
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install posthog --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install posthog --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install posthog --agent claude-code
npx -y @mvanhorn/printing-press-library install posthog --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/posthog/cmd/posthog-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/posthog-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install posthog --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-posthog --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-posthog --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install posthog --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/posthog-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `POSTHOG_PERSONAL_APIKEY_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "posthog": {
      "command": "posthog-pp-mcp",
      "env": {
        "POSTHOG_PERSONAL_APIKEY_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Uses your PostHog personal API key (phx_...). Set POSTHOG_API_KEY or run `posthog-pp-cli auth set-token`. Supports both US (app.posthog.com) and EU (eu.posthog.com) instances via POSTHOG_HOST.

## Quick Start

```bash
# Connect to your PostHog instance with your personal API key
posthog-pp-cli auth set-token

# Sync flags, insights, experiments, persons, and errors to local store
posthog-pp-cli sync --full

# List all feature flags with rollout rules
posthog-pp-cli flags list --json

# Run HogQL to see your top events
posthog-pp-cli query run --sql "SELECT event, count() FROM events WHERE timestamp > now() - INTERVAL 7 DAY GROUP BY event ORDER BY count() DESC LIMIT 20"

# Find everything that references a flag before archiving
posthog-pp-cli flags blast-radius my-checkout-v2

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Flag safety
- **`flags blast-radius`** — Find every insight, dashboard, experiment, and survey that references a flag before you archive or rename it.

  _Use before archiving or renaming a flag to prevent breaking dashboards and experiments silently._

  ```bash
  posthog-pp-cli flags blast-radius --key my-checkout-v2 --json
  ```
- **`flags rollout-health`** — Go/no-go confidence for a flag ramp — error rate and key metric movement correlated with flag exposure.

  _Use before ramping a flag to 100% to catch regressions that aren't visible in overall metrics._

  ```bash
  posthog-pp-cli flags rollout-health --key new-checkout --window 24h --agent
  ```
- **`flags stale`** — List flags that haven't been evaluated in N days — cleanup candidates before they accumulate.

  _Use in quarterly flag cleanup sprints to identify dead code paths safely._

  ```bash
  posthog-pp-cli flags stale --days 30 --json
  ```

### LLM observability
- **`llm cost-attribution`** — Break down LLM spend by feature flag variant — see whether the expensive model variant pays for itself.

  _Use when evaluating whether to promote a more expensive LLM variant based on actual cost-per-conversion._

  ```bash
  posthog-pp-cli llm cost-attribution --flag model-tier --agent
  ```

### Local state that compounds
- **`persons at-risk`** — Surface which users in a cohort are going quiet and recently hit errors — before they churn.

  _Use in weekly retention reviews to prioritize proactive outreach before churn events._

  ```bash
  posthog-pp-cli persons at-risk --cohort paying-users --silent-days 14 --json
  ```
- **`events property-drift`** — Catch tracking regressions — properties that silently disappeared from an event between two time windows.

  _Use after a deploy to catch silent schema changes that corrupt ongoing experiments and dashboards._

  ```bash
  posthog-pp-cli events property-drift --event checkout_completed --agent
  ```
- **`experiments pre-check`** — Know today whether an experiment will reach significance this sprint, or needs traffic adjustment now.

  _Use at the start of each sprint to surface experiments that need traffic changes before they run out of time._

  ```bash
  posthog-pp-cli experiments pre-check --agent
  ```
- **`dashboard health`** — Find broken dashboards before a stakeholder meeting does — stale data, deleted cohorts, archived flags.

  _Use before weekly business reviews to catch broken insight tiles that would embarrass the data team._

  ```bash
  posthog-pp-cli dashboard health --stale-days 7 --agent
  ```

## Usage

Run `posthog-pp-cli --help` for the full command reference and flag list.

## Recipes — Novel Features

These compound commands exist nowhere else. Each wraps multiple API calls into a single actionable answer.

### Before archiving a flag

```bash
# Find every insight, dashboard, experiment, and survey referencing a flag
posthog-pp-cli flags blast-radius my-flag --project 12345

# Go/no-go signal before ramping to 100%
posthog-pp-cli flags rollout-health my-flag --project 12345
```

### Quarterly flag cleanup

```bash
posthog-pp-cli flags stale --project 12345 --days 60 --json
```

### Before a stakeholder meeting

```bash
posthog-pp-cli dashboard health --project 12345
```

### Sprint planning

```bash
posthog-pp-cli experiments pre-check --project 12345 --sprint-days 14
```

### Retention review

```bash
posthog-pp-cli persons at-risk --project 12345 --silent-days 14 --json
```

### After a deploy

```bash
posthog-pp-cli events property-drift '$pageview' --project 12345
```

### LLM cost visibility

```bash
posthog-pp-cli llm cost-attribution --project 12345 --flag my-model-flag
```

---

## Commands

### code

Manage code

- **`posthog-pp-cli code invites-check-access-retrieve`** - Check whether the authenticated user has access to PostHog Code.
- **`posthog-pp-cli code invites-redeem-create`** - Redeem a PostHog Code invite code to enable access.

### environments

Manage environments

### organizations

Manage organizations

- **`posthog-pp-cli organizations create`** - Create
- **`posthog-pp-cli organizations destroy`** - Destroy
- **`posthog-pp-cli organizations list`** - List
- **`posthog-pp-cli organizations partial-update`** - Partial update
- **`posthog-pp-cli organizations retrieve`** - Retrieve
- **`posthog-pp-cli organizations update`** - Update

### projects

Manage projects

### public-hog-function-templates

Manage public hog function templates

- **`posthog-pp-cli public-hog-function-templates list`** - List

### user-home-settings

Manage user home settings

- **`posthog-pp-cli user-home-settings partial-update`** - Update the authenticated user's pinned sidebar tabs and/or homepage for the current team. Pass `@me` as the UUID. Send `tabs` to replace the pinned tab list, `homepage` to set the home destination (any PostHog URL — dashboard, insight, search results, scene). Either field may be omitted to leave it unchanged; sending `homepage: null` or `{}` clears the homepage.
- **`posthog-pp-cli user-home-settings retrieve`** - Get the authenticated user's pinned sidebar tabs and configured homepage for the current team. Pass `@me` as the UUID.

### users

Manage users

- **`posthog-pp-cli users cancel-email-change-request-partial-update`** - Cancel email change request partial update
- **`posthog-pp-cli users destroy`** - Destroy
- **`posthog-pp-cli users list`** - List
- **`posthog-pp-cli users partial-update`** - Update one or more of the authenticated user's profile fields or settings.
- **`posthog-pp-cli users request-email-verification-create`** - Request email verification create
- **`posthog-pp-cli users retrieve`** - Retrieve a user's profile and settings. Pass `@me` as the UUID to fetch the authenticated user; non-staff callers may only access their own account.
- **`posthog-pp-cli users update`** - Replace the authenticated user's profile and settings. Pass `@me` as the UUID to update the authenticated user. Prefer the PATCH endpoint for partial updates — PUT requires every writable field to be provided.
- **`posthog-pp-cli users verify-email-create`** - Verify email create

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
posthog-pp-cli organizations list

# JSON for scripting and agents
posthog-pp-cli organizations list --json

# Filter to specific fields
posthog-pp-cli organizations list --json --select id,name,status

# Dry run — show the request without sending
posthog-pp-cli organizations list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
posthog-pp-cli organizations list --agent
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
posthog-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/posthog-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `POSTHOG_PERSONAL_APIKEY_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `posthog-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $POSTHOG_PERSONAL_APIKEY_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized** — Run `posthog-pp-cli auth set-token` and paste your phx_ personal API key (not the project token)
- **Empty results after sync** — Check your project ID: `posthog-pp-cli projects list` — then set POSTHOG_PROJECT_ID
- **EU instance not connecting** — Set POSTHOG_HOST=https://eu.posthog.com and re-run sync
- **Query rate limit (429)** — PostHog limits analytics queries to 240/min. Add --delay 500 flag or reduce query frequency

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**posthog-python**](https://github.com/PostHog/posthog-python) — Python (54 stars)
- [**posthog-go**](https://github.com/PostHog/posthog-go) — Go (48 stars)
- [**@posthog/cli**](https://github.com/PostHog/posthog/tree/master/cli) — TypeScript
- [**PostHog MCP (official)**](https://github.com/PostHog/posthog/tree/master/services/mcp) — TypeScript
- [**heygen-com/posthog-mcp**](https://github.com/heygen-com/posthog-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

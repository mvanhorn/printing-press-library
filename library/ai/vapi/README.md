# Vapi CLI

Voice AI for developers.

## Install

This is a private CLI generated with CLI Printing Press and installed locally on this Mac at:

```bash
/Users/knox/.local/bin/vapi-pp-cli
```

To rebuild from this private checkout:

```bash
cd /Users/knox/Developer/PrintingPress/vapi-cli-worktree/private/vapi-pp-cli
go build -o /Users/knox/.local/bin/vapi-pp-cli ./cmd/vapi-pp-cli
```

The CLI reads `VAPI_TOKEN` or a saved token from `~/.config/vapi-pp-cli/config.toml`:

```bash
vapi-pp-cli auth set-token YOUR_TOKEN_HERE
# or
export VAPI_TOKEN="your-token-here"
```

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-vapi --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-vapi --force
```

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install vapi --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vapi-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `VAPI_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/vapi/cmd/vapi-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "vapi": {
      "command": "vapi-pp-mcp",
      "env": {
        "VAPI_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
vapi-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export VAPI_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
vapi-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
vapi-pp-cli chat list
```

## Usage

Run `vapi-pp-cli --help` for the full command reference and flag list.

### Private outbound-calling commands

`dial` is the low-level ergonomic command for having an assistant make calls. It wraps `POST /call` with guardrails so agents can preview payloads before placing real calls. Calls record by default through `assistantOverrides.artifactPlan.recordingEnabled=true`; pass `--no-record` for a specific opt-out.

`juno` is the personal house-manager layer for the tasks Cathryn actually delegates: reservations, ordering, quote gathering, follow-ups, status review, and general phone calls. It also has the short alias `b`.

```bash
# Preview only — no API call is sent
vapi-pp-cli dial \
  --assistant-id asst_123 \
  --phone-number-id pn_123 \
  --to +15551234567 \
  --dry-run --agent

# Restaurant reservation
vapi-pp-cli juno reservation \
  --assistant-id asst_juno \
  --phone-number-id pn_house \
  --to +15551234567 \
  --business "Via Carota" \
  --party-size 4 \
  --preferred-window "Friday 7-9pm" \
  --occasion birthday \
  --dry-run --agent

# Order or order-ready quote
vapi-pp-cli juno order \
  --assistant-id asst_juno \
  --phone-number-id pn_house \
  --to +15551234567 \
  --business "Local florist" \
  --item "seasonal birthday bouquet" \
  --max-spend "$100" \
  --dry-run --agent

# Get comparable quotes from multiple vendors
vapi-pp-cli juno quote \
  --assistant-id asst_juno \
  --phone-number-id pn_house \
  --to-file ./vendors.txt \
  --service "fix leak under kitchen sink" \
  --preferred-window "tomorrow after 10am" \
  --max-spend "$150 diagnostic" \
  --dry-run --agent

# Follow up on a previous call, carrying its Juno context forward
vapi-pp-cli juno followup \
  --call-id call_123 \
  --note "They asked us to call back after 2pm" \
  --dry-run --agent

# Poll a call outcome and save the recording
vapi-pp-cli juno status call_123 \
  --watch \
  --transcript \
  --download ./call.wav \
  --agent

# Create/update the Juno assistant payload
vapi-pp-cli juno assistant payload --agent
vapi-pp-cli juno assistant create --dry-run --agent

# Check Juno readiness and save default IDs into a profile
vapi-pp-cli juno setup --dry-run --agent
vapi-pp-cli juno setup \
  --assistant-id asst_juno \
  --phone-number-id pn_house \
  --profile-name juno-default \
  --agent

# Place a guarded smoke-test call
vapi-pp-cli --profile juno-default juno test-call \
  --to +15551234567 \
  --dry-run --agent

# Summarize recent calls
vapi-pp-cli juno report --recent 10 --agent

# Inspect available caller lines
vapi-pp-cli juno phone-numbers --agent

# Real call — requires --yes
VAPI_TOKEN=*** vapi-pp-cli juno reservation ... --yes --agent
```

Juno injects structured assistant variables:

- `juno_task_type`
- `juno_goal`
- `juno_business`
- `juno_script`
- `juno_constraints`
- `juno_success_criteria`
- `juno_escalate_if`
- `juno_guardrails`
- `juno_previous_call_id` for follow-up calls

Useful shared flags: `--constraint`, `--success`, `--escalate-if`, `--preferred-window`, `--deadline`, `--max-spend`, `--context`, `--variables-json`, `--assistant-json`, `--phone-number-json`, `--sip-uri`, `--no-record`.

## Commands

### assistant

Manage assistant

- **`vapi-pp-cli assistant create`** - Create Assistant
- **`vapi-pp-cli assistant find-all`** - List Assistants
- **`vapi-pp-cli assistant find-one`** - Get Assistant
- **`vapi-pp-cli assistant remove`** - Delete Assistant
- **`vapi-pp-cli assistant update`** - Update Assistant

### call

Manage call

- **`vapi-pp-cli call create`** - Create Call
- **`vapi-pp-cli call delete-data`** - Delete Call
- **`vapi-pp-cli call find-all`** - List Calls
- **`vapi-pp-cli call find-one`** - Get Call
- **`vapi-pp-cli call update`** - Update Call

### campaign

Manage campaign

- **`vapi-pp-cli campaign create`** - Create Campaign
- **`vapi-pp-cli campaign find-all`** - List Campaigns
- **`vapi-pp-cli campaign find-one`** - Get Campaign
- **`vapi-pp-cli campaign remove`** - Delete Campaign
- **`vapi-pp-cli campaign update`** - Update Campaign

### chat

Manage chat

- **`vapi-pp-cli chat create`** - Creates a new chat with optional SMS delivery via transport field. Requires at least one of: assistantId/assistant, sessionId, or previousChatId. Note: sessionId and previousChatId are mutually exclusive. Transport field enables SMS delivery with two modes: (1) New conversation - provide transport.phoneNumberId and transport.customer to create a new session, (2) Existing conversation - provide sessionId to use existing session data. Cannot specify both sessionId and transport fields together. The transport.useLLMGeneratedMessageForOutbound flag controls whether input is processed by LLM (true, default) or forwarded directly as SMS (false).
- **`vapi-pp-cli chat create-open-aichat`** - Create Chat (OpenAI Compatible)
- **`vapi-pp-cli chat delete`** - Delete Chat
- **`vapi-pp-cli chat get`** - Get Chat
- **`vapi-pp-cli chat list`** - List Chats

### eval

Manage eval

- **`vapi-pp-cli eval create`** - Create Eval
- **`vapi-pp-cli eval get`** - Get Eval
- **`vapi-pp-cli eval get-paginated`** - List Evals
- **`vapi-pp-cli eval get-run`** - Get Eval Run
- **`vapi-pp-cli eval get-runs-paginated`** - List Eval Runs
- **`vapi-pp-cli eval remove`** - Delete Eval
- **`vapi-pp-cli eval remove-run`** - Delete Eval Run
- **`vapi-pp-cli eval run`** - Create Eval Run
- **`vapi-pp-cli eval update`** - Update Eval

### file

Manage file

- **`vapi-pp-cli file create`** - Upload File
- **`vapi-pp-cli file find-all`** - List Files
- **`vapi-pp-cli file find-one`** - Get File
- **`vapi-pp-cli file remove`** - Delete File
- **`vapi-pp-cli file update`** - Update File

### observability

Manage observability

- **`vapi-pp-cli observability scorecard-create`** - Create Scorecard
- **`vapi-pp-cli observability scorecard-get`** - Get Scorecard
- **`vapi-pp-cli observability scorecard-get-paginated`** - List Scorecards
- **`vapi-pp-cli observability scorecard-remove`** - Delete Scorecard
- **`vapi-pp-cli observability scorecard-update`** - Update Scorecard

### phone-number

Manage phone number

- **`vapi-pp-cli phone-number create`** - Create Phone Number
- **`vapi-pp-cli phone-number find-all`** - List Phone Numbers
- **`vapi-pp-cli phone-number find-all-paginated`** - List Phone Numbers
- **`vapi-pp-cli phone-number find-one`** - Get Phone Number
- **`vapi-pp-cli phone-number remove`** - Delete Phone Number
- **`vapi-pp-cli phone-number update`** - Update Phone Number

### provider

Manage provider

- **`vapi-pp-cli provider resource-create-resource`** - Create Provider Resource
- **`vapi-pp-cli provider resource-delete-resource`** - Delete Provider Resource
- **`vapi-pp-cli provider resource-get-resource`** - Get Provider Resource
- **`vapi-pp-cli provider resource-get-resources-paginated`** - List Provider Resources
- **`vapi-pp-cli provider resource-update-resource`** - Update Provider Resource

### reporting

Manage reporting

- **`vapi-pp-cli reporting insight-create`** - Create Insight
- **`vapi-pp-cli reporting insight-find-all`** - Get Insights
- **`vapi-pp-cli reporting insight-find-one`** - Get Insight
- **`vapi-pp-cli reporting insight-preview`** - Preview Insight
- **`vapi-pp-cli reporting insight-remove`** - Delete Insight
- **`vapi-pp-cli reporting insight-run`** - Run Insight
- **`vapi-pp-cli reporting insight-update`** - Update Insight

### session

Manage session

- **`vapi-pp-cli session create`** - Create Session
- **`vapi-pp-cli session find-all-paginated`** - List Sessions
- **`vapi-pp-cli session find-one`** - Get Session
- **`vapi-pp-cli session remove`** - Delete Session
- **`vapi-pp-cli session update`** - Update Session

### squad

Manage squad

- **`vapi-pp-cli squad create`** - Create Squad
- **`vapi-pp-cli squad find-all`** - List Squads
- **`vapi-pp-cli squad find-one`** - Get Squad
- **`vapi-pp-cli squad remove`** - Delete Squad
- **`vapi-pp-cli squad update`** - Update Squad

### structured-output

Manage structured output

- **`vapi-pp-cli structured-output create`** - Create Structured Output
- **`vapi-pp-cli structured-output find-all`** - List Structured Outputs
- **`vapi-pp-cli structured-output find-one`** - Get Structured Output
- **`vapi-pp-cli structured-output remove`** - Delete Structured Output
- **`vapi-pp-cli structured-output run`** - Run Structured Output
- **`vapi-pp-cli structured-output update`** - Update Structured Output

### tool

Manage tool

- **`vapi-pp-cli tool create`** - Create Tool
- **`vapi-pp-cli tool find-all`** - List Tools
- **`vapi-pp-cli tool find-one`** - Get Tool
- **`vapi-pp-cli tool remove`** - Delete Tool
- **`vapi-pp-cli tool update`** - Update Tool

### vapi-analytics

Manage vapi analytics

- **`vapi-pp-cli vapi-analytics`** - Create Analytics Queries


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
vapi-pp-cli chat list

# JSON for scripting and agents
vapi-pp-cli chat list --json

# Filter to specific fields
vapi-pp-cli chat list --json --select id,name,status

# Dry run — show the request without sending
vapi-pp-cli chat list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
vapi-pp-cli chat list --agent
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
vapi-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/vapi-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `VAPI_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `vapi-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `vapi-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $VAPI_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

# Google Business Profile CLI

**Google Business Profile operations plus a local archive, search, analytics, and monitoring workflow.**

Use the Google Business Profile CLI to manage API resources, then archive them locally for fast search, grouped analytics, and rollout verification. It is designed for operator workflows where repeated read access and auditability matter as much as raw endpoint coverage.

## Install

The recommended path installs both the `google-business-profile-pp-cli` binary and the `pp-google-business-profile` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install google-business-profile
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install google-business-profile --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install google-business-profile --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install google-business-profile --agent claude-code
npx -y @mvanhorn/printing-press-library install google-business-profile --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-business-profile/cmd/google-business-profile-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-business-profile-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-business-profile --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-business-profile --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-google-business-profile skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-google-business-profile. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-business-profile-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOGLE_BUSINESS_PROFILE_OAUTH2` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-business-profile/cmd/google-business-profile-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-business-profile": {
      "command": "google-business-profile-pp-mcp",
      "env": {
        "GOOGLE_BUSINESS_PROFILE_OAUTH2": "<your-token>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate with OAuth2 using `google-business-profile-pp-cli login` so the CLI can mint and refresh access tokens for Google Business Profile APIs. The `GOOGLE_BUSINESS_PROFILE_OAUTH2` env var is a fallback only when you already have a short-lived access token and need a non-interactive run.

## Quick Start

```bash
# Check auth and runtime health first.
google-business-profile-pp-cli doctor --json

# Archive the latest resources locally for search and analytics.
google-business-profile-pp-cli workflow archive --agent

# Search the local archive for a location clue.
google-business-profile-pp-cli search "Toronto" --data-source local --type locations --limit 20 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local operator workflows
- **`workflow archive`** — Build a local SQLite archive of Google Business Profile resources so agents can inspect accounts and locations without re-hitting the API on every question.

  _Lets an agent answer follow-up questions from a synchronized snapshot instead of burning quota and latency on repeated list calls._

  ```bash
  google-business-profile-pp-cli workflow archive --agent
  ```
- **`workflow status`** — Show local archive freshness and sync state across resource tables so operators know whether offline answers are still trustworthy.

  _Prevents agents from making stale operational recommendations from an old archive._

  ```bash
  google-business-profile-pp-cli workflow status --agent
  ```
- **`search`** — Search locally synced Business Profile data with FTS5 when a native API search path is absent or live auth is unavailable.

  _Gives fast operator lookup across synced records without crafting custom filters for each API surface._

  ```bash
  google-business-profile-pp-cli search "Toronto" --data-source local --type locations --limit 20 --json
  ```
- **`analytics`** — Run grouped analytics over synced data to count resources and surface distribution patterns without exporting to spreadsheets first.

  _Turns the CLI into a lightweight reporting surface for location inventory and QA work._

  ```bash
  google-business-profile-pp-cli analytics --type locations --group-by storeCode --limit 25 --json
  ```

### Monitoring and verification
- **`tail`** — Poll the API and emit NDJSON change events so operators can watch location-related activity in near real time.

  _Makes the CLI useful for active monitoring, not just one-shot API calls._

  ```bash
  google-business-profile-pp-cli tail --resource locations --interval 30s --follow=false --agent
  ```

## Usage

Run `google-business-profile-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Manage accounts

- **`google-business-profile-pp-cli accounts accept`** - Accepts the specified invitation.
- **`google-business-profile-pp-cli accounts create`** - Creates an account with the specified name and type under the given parent. - Personal accounts and Organizations cannot be created. - User Groups cannot be created with a Personal account as primary owner. - Location Groups cannot be created with a primary owner of a Personal account if the Personal account is in an Organization. - Location Groups cannot own Location Groups.
- **`google-business-profile-pp-cli accounts create-admins`** - Invites the specified user to become an administrator for the specified account. The invitee must accept the invitation in order to be granted access to the account. See AcceptInvitation to programmatically accept an invitation.
- **`google-business-profile-pp-cli accounts decline`** - Declines the specified invitation.
- **`google-business-profile-pp-cli accounts delete`** - Removes the specified admin from the specified account.
- **`google-business-profile-pp-cli accounts get`** - Gets the specified account. Returns `NOT_FOUND` if the account does not exist or if the caller does not have access rights to it.
- **`google-business-profile-pp-cli accounts list`** - Lists all of the accounts for the authenticated user. This includes all accounts that the user owns, as well as any accounts for which the user has management rights.
- **`google-business-profile-pp-cli accounts list-admins`** - Lists the admins for the specified account.
- **`google-business-profile-pp-cli accounts list-invitations`** - Lists pending invitations for the specified account.
- **`google-business-profile-pp-cli accounts patch`** - Updates the Admin for the specified Account Admin.

### attributes

Manage attributes

- **`google-business-profile-pp-cli attributes`** - Returns the list of attributes that would be available for a location with the given primary category and country.

### business-profile-performance-locations

Manage business profile performance locations

- **`google-business-profile-pp-cli business-profile-performance-locations fetch-multi-daily-metrics-time-series`** - Returns the values for each date from a given time range that are associated with the specific daily metrics. Note: Only daily data is available. Hourly metrics are not supported. Example request: `GET https://businessprofileperformance.googleapis.com/v1/locations/12345:fetchMultiDailyMetricsTimeSeries?dailyMetrics=WEBSITE_CLICKS&dailyMetrics=CALL_CLICKS&daily_range.start_date.year=2022&daily_range.start_date.month=1&daily_range.start_date.day=1&daily_range.end_date.year=2022&daily_range.end_date.month=3&daily_range.end_date.day=31`
- **`google-business-profile-pp-cli business-profile-performance-locations get-daily-metrics-time-series`** - Returns the values for each date from a given time range that are associated with the specific daily metric. Note: Only daily data is available. Hourly metrics are not supported. Example request: `GET https://businessprofileperformance.googleapis.com/v1/locations/12345:getDailyMetricsTimeSeries?dailyMetric=WEBSITE_CLICKS&daily_range.start_date.year=2022&daily_range.start_date.month=1&daily_range.start_date.day=1&daily_range.end_date.year=2022&daily_range.end_date.month=3&daily_range.end_date.day=31`
- **`google-business-profile-pp-cli business-profile-performance-locations list`** - Returns the search keywords used to find a business in search or maps. Each search keyword is accompanied by impressions which are aggregated on a monthly basis. Example request: `GET https://businessprofileperformance.googleapis.com/v1/locations/12345/searchkeywords/impressions/monthly?monthly_range.start_month.year=2022&monthly_range.start_month.month=1&monthly_range.end_month.year=2022&monthly_range.end_month.month=3`

### categories

Manage categories

- **`google-business-profile-pp-cli categories batch-get`** - Returns a list of business categories for the provided language and GConcept ids.
- **`google-business-profile-pp-cli categories list`** - Returns a list of business categories. Search will match the category name but not the category ID. Search only matches the front of a category name (that is, 'food' may return 'Food Court' but not 'Fast Food Restaurant').

### chains

Manage chains

- **`google-business-profile-pp-cli chains get`** - Gets the specified chain. Returns `NOT_FOUND` if the chain does not exist.
- **`google-business-profile-pp-cli chains search`** - Searches the chain based on chain name.

### google_locations

Manage google locations

- **`google-business-profile-pp-cli google-locations search`** - Search all of the possible locations that are a match to the specified request.

### locations

Manage locations

- **`google-business-profile-pp-cli locations <name>`** - Moves a location from an account that the user owns to another account that the same user administers. The user must be an owner of the account the location is currently associated with and must also be at least a manager of the destination account.

### my-business-business-accounts

Manage my business business accounts

- **`google-business-profile-pp-cli my-business-business-accounts create`** - Creates a new Location that will be owned by the logged in user.
- **`google-business-profile-pp-cli my-business-business-accounts list`** - Lists the locations for the specified account.

### my-business-business-locations

Manage my business business locations

- **`google-business-profile-pp-cli my-business-business-locations delete`** - Deletes a location. If this location cannot be deleted using the API and it is marked so in the `google.mybusiness.businessinformation.v1.LocationState`, use the [Google Business Profile](https://business.google.com/manage/) website.
- **`google-business-profile-pp-cli my-business-business-locations get-google-updated`** - Retrieves attributes for a location as they appear live on Google Maps and Search. This consumer-facing view may have been updated by Google or user-generated content and may differ from the merchant's version.
- **`google-business-profile-pp-cli my-business-business-locations patch`** - Updates the specified location.

### my-business-lodging-locations

Manage my business lodging locations

- **`google-business-profile-pp-cli my-business-lodging-locations get-google-updated`** - Returns the Google updated Lodging of a specific location.
- **`google-business-profile-pp-cli my-business-lodging-locations get-lodging`** - Returns the Lodging of a specific location.
- **`google-business-profile-pp-cli my-business-lodging-locations update-lodging`** - Updates the Lodging of a specific location.

### my-business-notifications-accounts

Manage my business notifications accounts

- **`google-business-profile-pp-cli my-business-notifications-accounts get-notification-setting`** - Returns the pubsub notification settings for the account.
- **`google-business-profile-pp-cli my-business-notifications-accounts update-notification-setting`** - Sets the pubsub notification setting for the account informing Google which topic to send pubsub notifications for. Use the notification_types field within notification_setting to manipulate the events an account wants to subscribe to. An account will only have one notification setting resource, and only one pubsub topic can be set. To delete the setting, update with an empty notification_types

### my-business-place-locations

Manage my business place locations

- **`google-business-profile-pp-cli my-business-place-locations create`** - Creates a place action link associated with the specified location, and returns it. The request is considered duplicate if the `parent`, `place_action_link.uri` and `place_action_link.place_action_type` are the same as a previous request.
- **`google-business-profile-pp-cli my-business-place-locations delete`** - Deletes a place action link from the specified location.
- **`google-business-profile-pp-cli my-business-place-locations get`** - Gets the specified place action link.
- **`google-business-profile-pp-cli my-business-place-locations list`** - Lists the place action links for the specified location.
- **`google-business-profile-pp-cli my-business-place-locations patch`** - Updates the specified place action link and returns it.

### my-business-q-locations

Manage my business q locations

- **`google-business-profile-pp-cli my-business-q-locations create`** - Adds a question for the specified location.
- **`google-business-profile-pp-cli my-business-q-locations delete`** - Deletes a specific question written by the current user.
- **`google-business-profile-pp-cli my-business-q-locations delete-answersdelete`** - Deletes the answer written by the current user to a question.
- **`google-business-profile-pp-cli my-business-q-locations list`** - Returns the paginated list of questions and some of its answers for a specified location. This operation is only valid if the specified location is verified.
- **`google-business-profile-pp-cli my-business-q-locations list-answers`** - Returns the paginated list of answers for a specified question.
- **`google-business-profile-pp-cli my-business-q-locations patch`** - Updates a specific question written by the current user.
- **`google-business-profile-pp-cli my-business-q-locations upsert`** - Creates an answer or updates the existing answer written by the user for the specified question. A user can only create one answer per question.

### my-business-verifications-locations

Manage my business verifications locations

- **`google-business-profile-pp-cli my-business-verifications-locations complete`** - Completes a `PENDING` verification. It is only necessary for non `AUTO` verification methods. `AUTO` verification request is instantly `VERIFIED` upon creation.
- **`google-business-profile-pp-cli my-business-verifications-locations fetch-verification-options`** - Reports all eligible verification options for a location in a specific language.
- **`google-business-profile-pp-cli my-business-verifications-locations get-voice-of-merchant-state`** - Gets the VoiceOfMerchant state.
- **`google-business-profile-pp-cli my-business-verifications-locations list`** - List verifications of a location, ordered by create time.
- **`google-business-profile-pp-cli my-business-verifications-locations verify`** - Starts the verification process for a location.

### place_action_type_metadata

Manage place action type metadata

- **`google-business-profile-pp-cli place-action-type-metadata`** - Returns the list of available place action types for a location or country.

### verification_tokens

Manage verification tokens

- **`google-business-profile-pp-cli verification-tokens`** - Generate a token for the provided location data to verify the location.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-business-profile-pp-cli accounts list

# JSON for scripting and agents
google-business-profile-pp-cli accounts list --json

# Filter to specific fields
google-business-profile-pp-cli accounts list --json --select id,name,status

# Dry run — show the request without sending
google-business-profile-pp-cli accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-business-profile-pp-cli accounts list --agent
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
google-business-profile-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/google-business-profile-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOOGLE_BUSINESS_PROFILE_OAUTH2` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `google-business-profile-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-business-profile-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOOGLE_BUSINESS_PROFILE_OAUTH2`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **Search returns no local results** — Run `google-business-profile-pp-cli workflow archive --agent` or `google-business-profile-pp-cli sync --agent` first so the SQLite archive has data.
- **Doctor reports missing OAuth credentials** — Run `google-business-profile-pp-cli login` to establish OAuth2, or provide `GOOGLE_BUSINESS_PROFILE_OAUTH2` only for short-lived non-interactive runs.

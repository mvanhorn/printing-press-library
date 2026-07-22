# Sutra CLI

**Every Sutra (Arketa) Partner API resource plus a local mirror and the studio analytics the dashboard buries.**

The Sutra/Arketa Partner API is pure CRUD with zero reporting endpoints, so studio operators are stuck with canned vendor reports they cannot customize and a client list the vendor is cagey about exporting. This CLI syncs your studio's full dataset (locations, classes, clients, purchases, referrals, reservations) into a local SQLite database you own, then answers the questions the dashboard hides: instructor scorecards, no-show rates, capacity utilization, expiring memberships, churn risk, referral conversion, client LTV, and revenue with prior-period comparison. The daily front-desk loop (book, cancel, check-in) is in here too, all offline-queryable and agent-native.

## Install

The recommended path installs both the `sutra-fitness-pp-cli` binary and the `pp-sutra-fitness` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sutra-fitness
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sutra-fitness --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sutra-fitness --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sutra-fitness --agent claude-code
npx -y @mvanhorn/printing-press-library install sutra-fitness --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/cmd/sutra-fitness-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sutra-fitness-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sutra-fitness --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sutra-fitness --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sutra-fitness --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sutra-fitness --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sutra-fitness-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SUTRA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/cmd/sutra-fitness-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sutra-fitness": {
      "command": "sutra-fitness-pp-mcp",
      "env": {
        "SUTRA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authentication uses a partner API key plus your partner ID. Set SUTRA_API_KEY to the key Arketa issued you (sent as the X-API-Key header) and SUTRA_PARTNER_ID to your partner identifier, which scopes every request. Run 'sutra-fitness-pp-cli doctor' to confirm both are set and the API is reachable before syncing.

## Quick Start

```bash
# Confirm SUTRA_API_KEY and SUTRA_PARTNER_ID are configured before any live call.
sutra-fitness-pp-cli doctor --dry-run

# Pull your studio's full dataset into a local SQLite mirror you own (run this first).
sutra-fitness-pp-cli sync --resources classes,clients,purchases,reservations,locations,referrals

# See fill rate per instructor across the synced schedule.
sutra-fitness-pp-cli utilization --group-by instructor

# List memberships and class-packs expiring in the next week for renewal outreach.
sutra-fitness-pp-cli expiring --within 7d

# Rank instructors by fill, no-show, and check-in rate.
sutra-fitness-pp-cli scorecard

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Studio analytics the dashboard buries
- **`scorecard`** — Rank instructors by class fill rate, no-show rate, and check-in rate across your synced schedule.

  _Reach for this when you need to compare teacher performance to decide who to coach or feature, not raw class rows._

  ```bash
  sutra-fitness-pp-cli scorecard --agent
  ```
- **`no-shows`** — Surface no-show rates grouped by instructor, class, or client from synced reservations.

  _Use when you need to quantify and pivot no-shows, e.g. which instructor or client drives the most absences._

  ```bash
  sutra-fitness-pp-cli no-shows --group-by instructor --json
  ```
- **`utilization`** — Compute fill ratio (booked vs capacity) per class, instructor, time-slot, or location over a date window.

  _Reach for this to find under-filled slots and over-subscribed classes before schedule planning._

  ```bash
  sutra-fitness-pp-cli utilization --group-by instructor
  ```
- **`revenue`** — Sum purchase revenue by membership type for a window and show the delta versus the prior equal window.

  _Use for the Monday revenue review: is this week up or down versus last, by membership type._

  ```bash
  sutra-fitness-pp-cli revenue --group-by type --compare-prior
  ```
- **`ltv`** — Rank clients by total purchase spend with tenure since signup.

  _Use to identify your highest-value members for VIP outreach or to understand revenue concentration._

  ```bash
  sutra-fitness-pp-cli ltv --limit 25
  ```

### Retention and renewals
- **`expiring`** — List active memberships and class-packs expiring within a window or running low on credits, with client contact info.

  _Use for deterministic renewal outreach: who needs to re-up this week, with their email and phone._

  ```bash
  sutra-fitness-pp-cli expiring --within 7d --low-credits
  ```
- **`churn`** — Flag non-removed clients with no recent check-in and/or an expired plan using a mechanical recency threshold.

  _Reach for this to build a win-back list of members drifting away, distinct from hard date-based expiry._

  ```bash
  sutra-fitness-pp-cli churn --inactive-days 30 --json
  ```
- **`referral-funnel`** — Trace referrals to whether the referred client signed up, purchased, and attended, and rank top referrers.

  _Reach for this to measure whether your referral program actually converts and who your best referrers are._

  ```bash
  sutra-fitness-pp-cli referral-funnel --json
  ```

## Recipes


### Monday revenue review

```bash
sutra-fitness-pp-cli revenue --group-by type --compare-prior
```

Totals revenue by membership type for the current window and shows the delta versus the prior equal window.

### Renewal outreach list

```bash
sutra-fitness-pp-cli expiring --within 7d --low-credits --csv
```

Exports a CSV of members expiring this week or low on credits, with contact info, ready for an email or SMS campaign.

### Agent-friendly instructor ranking

```bash
sutra-fitness-pp-cli scorecard --agent --select instructors.name,instructors.fill_rate,instructors.no_show_rate
```

Returns a narrowed, machine-readable instructor ranking so an agent can reason over fill and no-show rates without parsing the full nested payload.

### At-risk client sweep

```bash
sutra-fitness-pp-cli churn --inactive-days 30 --json
```

Lists members with no check-in in 30 days or an expired plan as a JSON win-back list.

## Usage

Run `sutra-fitness-pp-cli --help` for the full command reference and flag list.

## Commands

### classes

Class management operations

- **`sutra-fitness-pp-cli classes get-partner`** - Retrieve a paginated list of partner classes with optional filtering
- **`sutra-fitness-pp-cli classes get-partner-class`** - Retrieve details for a specific class

### clients

Client management operations

- **`sutra-fitness-pp-cli clients get-partner`** - Retrieve a paginated list of partner clients with optional filtering
- **`sutra-fitness-pp-cli clients get-partner-clients`** - Retrieve details for a specific client

### locations

Location management operations

- **`sutra-fitness-pp-cli locations <partnerId>`** - Retrieve a paginated list of partner locations with optional filtering

### purchases

Purchase management operations

- **`sutra-fitness-pp-cli purchases <partnerId>`** - Retrieve a paginated list of partner purchases with optional filtering

### referrals

Referral management operations

- **`sutra-fitness-pp-cli referrals <partnerId>`** - Retrieve a paginated list of partner referrals with optional filtering


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sutra-fitness-pp-cli classes get-partner mock-value mock-value

# JSON for scripting and agents
sutra-fitness-pp-cli classes get-partner mock-value mock-value --json

# Filter to specific fields
sutra-fitness-pp-cli classes get-partner mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
sutra-fitness-pp-cli classes get-partner mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sutra-fitness-pp-cli classes get-partner mock-value mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
sutra-fitness-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/partner-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SUTRA_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `sutra-fitness-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sutra-fitness-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SUTRA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized / API key required** — Set SUTRA_API_KEY to your Arketa partner key; it is sent as the X-API-Key header. Verify with 'sutra-fitness-pp-cli doctor'.
- **403 Forbidden / access denied to partner data** — Set SUTRA_PARTNER_ID to your own partner identifier; the key only grants access to its own partner's data.
- **Analytics commands return empty or 'no local mirror'** — Run 'sutra-fitness-pp-cli sync --resources classes,clients,purchases,reservations' first; analytics read the local store.
- **Stale numbers in reports** — Re-run 'sutra-fitness-pp-cli sync --since 24h' to pull recently updated records via the updated_at cursor.
- **429 / rate limited during a large sync** — The API caps at ~25 req/sec; re-run sync (it resumes from the cursor) or narrow with --resources and --since.

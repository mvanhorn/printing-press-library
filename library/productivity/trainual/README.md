# Trainual CLI

**Every Trainual feature plus compliance audit, assignment gap detection, and training analytics no other tool provides.**

Sync your entire Trainual account to a local SQLite database, then run compliance audits, detect assignment gaps, and produce coverage matrices that would take hours of manual spreadsheet work. Works offline, pipes to jq, and outputs agent-native JSON.

## Install

The recommended path installs both the `trainual-pp-cli` binary and the `pp-trainual` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install trainual
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install trainual --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trainual-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-trainual --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-trainual --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-trainual skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-trainual. The skill defines how its required CLI can be installed.
```

## Authentication

Trainual uses Bearer token authentication. Generate your API key in Trainual Settings > Integrations > API, then set TRAINUAL_API_KEY in your environment.

## Quick Start

```bash
# Verify API key is valid and Trainual is reachable
trainual-pp-cli doctor


# Pull all users, subjects, topics, tests, and roles into local SQLite
trainual-pp-cli sync --full


# See who's behind on training across all curriculums
trainual-pp-cli compliance-audit --threshold 80


# Detect users missing assignments their peers have
trainual-pp-cli assignment-gaps --by-role --json


# Role-by-subject completion matrix for stakeholder reporting
trainual-pp-cli coverage-matrix

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Training intelligence
- **`compliance-audit`** — Surface every employee below a completion threshold, grouped by role — the Monday compliance check in one command.

  _Use when an agent needs to identify training compliance gaps across an organization without clicking through dozens of Trainual UI screens._

  ```bash
  trainual-pp-cli compliance-audit --threshold 80 --role "Front Desk" --agent
  ```
- **`assignment-gaps`** — Detect users missing subject assignments that peers in their role have — catch forgotten onboarding assignments before managers complain.

  _Use when an agent needs to verify assignment completeness across roles — catches partial gaps, not just fully unassigned users._

  ```bash
  trainual-pp-cli assignment-gaps --by-role --json
  ```
- **`content-audit`** — List all curriculums with course counts, test counts, and enrollment — flag empty, untested, and orphaned content in one table.

  _Use when an agent needs to identify structural training content problems (empty curriculums, missing tests) across the entire content library._

  ```bash
  trainual-pp-cli content-audit --show-empty --show-untested --agent
  ```
- **`onboarding-status`** — Show new hires from the last N days with their assignment completeness and completion percentage, grouped by role.

  _Use when an agent needs to check whether recent hires are on track with their training assignments._

  ```bash
  trainual-pp-cli onboarding-status --days 30 --json --select user,role,completion_percentage
  ```

### Local state that compounds
- **`coverage-matrix`** — Produce a role-by-subject matrix showing completion percentage per cell — the exact report ops managers build by hand in Excel.

  _Use when an agent needs a complete training coverage snapshot across all roles and subjects in one structured output._

  ```bash
  trainual-pp-cli coverage-matrix --json
  ```
- **`role-completion`** — Rank roles by average completion percentage — instantly see which venues or teams are ahead or behind.

  _Use when an agent needs to compare training progress across organizational units (venues, departments, teams)._

  ```bash
  trainual-pp-cli role-completion --sort avg_completion --agent
  ```
- **`completion-trend`** — Show how a user's completion percentage has changed over successive syncs — week-over-week training progress.

  _Use when an agent needs to track whether a flagged employee's training is actually improving over time._

  ```bash
  trainual-pp-cli completion-trend 1618115 --weeks 8 --json
  ```

### Agent-native plumbing
- **`bulk-assign`** — Assign subjects to all users in a role with one command — the role-based fan-out that individual assign-subjects can't do.

  _Use when an agent needs to ensure all members of a role have consistent subject assignments without iterating manually._

  ```bash
  trainual-pp-cli bulk-assign --role "Kitchen" --subjects 101,102,103 --dry-run
  ```

## Usage

Run `trainual-pp-cli --help` for the full command reference and flag list.

## Commands

### roles

Roles (groups) for organizing users and assignments

- **`trainual-pp-cli roles list`** - List all roles with optional assigned user data

### subjects

Training subjects (curriculums) containing courses and tests

- **`trainual-pp-cli subjects get`** - Get a specific subject by ID
- **`trainual-pp-cli subjects list`** - List all subjects with optional assigned user data

### tests

Tests (surveys/quizzes) within a subject

- **`trainual-pp-cli tests get`** - Get a specific test
- **`trainual-pp-cli tests list`** - List all tests for a subject

### topics

Topics (courses) within a subject

- **`trainual-pp-cli topics get`** - Get a specific topic
- **`trainual-pp-cli topics list`** - List all topics for a subject

### users

Manage employees and their training assignments

- **`trainual-pp-cli users archive`** - Archive (deactivate) a user
- **`trainual-pp-cli users assign-roles`** - Assign roles to a user
- **`trainual-pp-cli users assign-subjects`** - Assign subjects (curriculums) to a user
- **`trainual-pp-cli users create`** - Invite a new user (triggers invitation email, consumes seat)
- **`trainual-pp-cli users get`** - Get a specific user by ID
- **`trainual-pp-cli users list`** - List all users with optional completion and assignment data
- **`trainual-pp-cli users unarchive`** - Restore an archived user
- **`trainual-pp-cli users unassign-roles`** - Remove role assignments from a user
- **`trainual-pp-cli users unassign-subjects`** - Remove subject assignments from a user
- **`trainual-pp-cli users update`** - Update user details


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
trainual-pp-cli users list

# JSON for scripting and agents
trainual-pp-cli users list --json

# Filter to specific fields
trainual-pp-cli users list --json --select id,email,completion_percentage

# CSV for spreadsheet import
trainual-pp-cli users list --csv

# Dry run — show the request without sending
trainual-pp-cli users list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
trainual-pp-cli users list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-trainual -g
```

Then invoke `/pp-trainual <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add trainual trainual-pp-mcp -e TRAINUAL_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trainual-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TRAINUAL_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "trainual": {
      "command": "trainual-pp-mcp",
      "env": {
        "TRAINUAL_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
trainual-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Cookbook

```bash
# Monday compliance check: who is below 80% completion?
trainual-pp-cli compliance-audit --threshold 80 --json

# Compliance check scoped to a single role
trainual-pp-cli compliance-audit --threshold 90 --role "Front Desk" --agent

# Find users missing assignments their peers have
trainual-pp-cli assignment-gaps --by-role --json

# Onboarding tracker: new hires in the last 14 days
trainual-pp-cli onboarding-status --days 14 --json

# Coverage matrix for stakeholder reporting
trainual-pp-cli coverage-matrix --csv > coverage.csv

# Rank roles by training completion
trainual-pp-cli role-completion --sort avg_completion --agent

# Flag empty and untested curriculums
trainual-pp-cli content-audit --show-empty --show-untested --json

# Bulk-assign subjects to all users in a role (preview first)
trainual-pp-cli bulk-assign --role "Kitchen" --subjects 101,102,103 --dry-run

# Export all users as JSONL for external processing
trainual-pp-cli export users --format jsonl -o users.jsonl

# Search local data for a keyword
trainual-pp-cli search "safety" --data-source local --json

# Invite a new user
trainual-pp-cli users create --email jane.doe@example.com --first-name Jane --last-name Doe --role "New Hire"

# Pipe compliance gaps to jq for further analysis
trainual-pp-cli compliance-audit --threshold 80 --json | jq '[.[] | select(.gap > 30)]'
```

## Configuration

Config file: `~/.config/trainual-pp-cli/config.json`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TRAINUAL_API_KEY` | per_call | Yes | API key from Trainual Settings > Integrations > API. |
| `TRAINUAL_BASE_URL` | config | No | Override the API base URL (default: `https://app.trainual.com/api/v1`). Useful for self-hosted or staging environments. |
| `TRAINUAL_CONFIG` | config | No | Override the config file path (default: `~/.config/trainual-pp-cli/config.json`). |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `trainual-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TRAINUAL_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every request** — Regenerate your API key in Trainual Settings > Integrations > API and update TRAINUAL_API_KEY
- **completion_percentage shows 0 for users with progress** — Use completion_percentage field (not avg_completion which is broken in the Trainual API)
- **sync takes too long** — Trainual API paginates at 50/page max. Large orgs (500+ users) may take a few minutes on first sync.
- **assignment-gaps shows false positives** — Some roles intentionally have different subject sets. Use --role to scope to a specific role for accurate gap detection.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

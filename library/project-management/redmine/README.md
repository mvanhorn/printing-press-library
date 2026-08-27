# Redmine CLI

**Every Redmine feature, plus a local mirror that finally answers the cross-project questions the Roadmap page never could.**

Redmine's REST API is deeply relational but strictly request-per-entity — no burndown endpoint, no workload endpoint, no staleness concept. This CLI absorbs the full REST surface (issues, projects, time entries, wiki, versions, memberships, and more) and adds a synced SQLite mirror underneath, so burndown, workload, cycle-time, and blocker-chain questions resolve to one local query instead of dozens of live requests.

Learn more at [Redmine](https://www.redmine.org/projects/redmine/wiki/Rest_api).

## Install

The recommended path installs both the `redmine-pp-cli` binary and the `pp-redmine` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install redmine
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install redmine --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install redmine --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install redmine --agent claude-code
npx -y @mvanhorn/printing-press-library install redmine --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/redmine/cmd/redmine-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/redmine-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install redmine --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-redmine --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-redmine --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install redmine --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/redmine-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `REDMINE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/redmine/cmd/redmine-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "redmine": {
      "command": "redmine-pp-mcp",
      "env": {
        "REDMINE_PROJECT_ID": "<project_id>",
        "REDMINE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Redmine is self-hosted, so two things are required instead of just a key: REDMINE_URL (your instance's base URL) and REDMINE_API_KEY (from your account's My Account page, or Administration > Settings > API for admins). Both are sent as-is — REDMINE_API_KEY becomes the X-Redmine-API-Key header on every request.

## Quick Start

```bash
# Verify REDMINE_URL and REDMINE_API_KEY are set and the instance is reachable, without making a real call.
redmine-pp-cli doctor --dry-run

# Pull a full local mirror so search, digest, and every transcendence command have data to work with. The status_id=* override is required to include closed issues — Redmine's default issue list only returns open ones.
redmine-pp-cli sync --resources issues-json,projects-json,time-entries-json,users-json --full --resource-param issues-json:status_id=*

# Confirm the sync worked by listing open issues in a known project.
redmine-pp-cli issues-json get-issues --project-id demo --status-id o --json

# See what changed in the last week across issues assigned to you.
redmine-pp-cli digest --since 7d --mine

# Check whether anyone on the project is carrying too much open work.
redmine-pp-cli workload --project demo

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`roadmap burndown`** — See open/closed issue counts, average completion, and logged-vs-estimated hours for a release version — the Roadmap page Redmine never exposed as an API.

  _Reach for this before a release-readiness call instead of hand-counting issues in the web UI._

  ```bash
  redmine-pp-cli roadmap burndown 1.0 --project demo --json
  ```
- **`workload`** — See every assignee's open-issue count and estimated hours in one aggregate view, with a flag for anyone over threshold.

  _Use this before sprint planning to catch overload without opening every engineer's issue list one by one._

  ```bash
  redmine-pp-cli workload --project demo --threshold 5 --json
  ```
- **`issues stale`** — Find open issues nobody has touched in N days — Redmine has no built-in staleness concept.

  _Use this for triage sweeps to surface issues that fell through the cracks, not recently active ones._

  ```bash
  redmine-pp-cli issues stale --project demo --days 14 --json
  ```
- **`digest`** — A personal activity report of what was created, updated, or closed in a time window, optionally scoped to issues assigned to or watched by you.

  _Use this for a 'what did I miss' catch-up instead of paging through each project's activity feed by hand._

  ```bash
  redmine-pp-cli digest --since 7d --mine --json
  ```
- **`issues cycle-time`** — Average days from creation to close, grouped by tracker or project — real duration math the API can't do.

  _Use this to spot which issue types are taking longer than they should to resolve._

  ```bash
  redmine-pp-cli issues cycle-time --group-by tracker --project demo --json
  ```
- **`issues blockers`** — Walk the full multi-hop 'blocks'/'blocked by' dependency chain for an issue, not just its direct relations.

  _Use this before starting work on an issue to confirm it isn't blocked three hops away by something you haven't seen yet._

  ```bash
  redmine-pp-cli issues blockers 3 --depth 3 --json
  ```

## Recipes

### Triage open bugs with a narrow agent payload

```bash
redmine-pp-cli issues-json get-issues --project-id demo --status-id o --tracker-id 1 --agent --select id,subject,tracker.name,priority.name
```

Nested tracker/priority/status objects on every issue add up fast across a full project; --select keeps only the fields an agent actually needs to triage.

### Find issues nobody has touched in two weeks

```bash
redmine-pp-cli issues stale --project demo --days 14 --json
```

Surfaces open issues with no recent activity, which Redmine itself has no concept of.

### See who's carrying the most open work

```bash
redmine-pp-cli workload --project demo --threshold 5
```

One command instead of opening each engineer's assigned-issue list to eyeball the count.

### Trace why an issue is blocked

```bash
redmine-pp-cli issues blockers 3 --depth 3
```

Walks the full transitive blocker chain instead of clicking through each linked issue one hop at a time.

### Check release readiness for a version

```bash
redmine-pp-cli roadmap burndown 1.0 --project demo --json
```

Open/closed counts, average done_ratio, and hours variance for the version in one call, replacing a manual Roadmap-page count.

## Usage

Run `redmine-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `REDMINE_CONFIG_DIR`, `REDMINE_DATA_DIR`, `REDMINE_STATE_DIR`, or `REDMINE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `REDMINE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export REDMINE_HOME=/srv/redmine
redmine-pp-cli doctor
```

Under `REDMINE_HOME=/srv/redmine`, the four dirs resolve to `/srv/redmine/config`, `/srv/redmine/data`, `/srv/redmine/state`, and `/srv/redmine/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "redmine": {
      "command": "redmine-pp-mcp",
      "env": {
        "REDMINE_HOME": "/srv/redmine"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `REDMINE_DATA_DIR` overrides an explicit `--home` for that kind. Use `REDMINE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `REDMINE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `redmine-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### attachments

Status: Beta, Availability: 1.3

- **`redmine-pp-cli attachments delete`** - Delete attachment
- **`redmine-pp-cli attachments download-file`** - Undocumented: This operation is not listed in the official documentation.
- **`redmine-pp-cli attachments download-thumbnail`** - Undocumented: This operation is not listed in the official documentation.
- **`redmine-pp-cli attachments get`** - Show attachment
- **`redmine-pp-cli attachments update`** - Update attachment

### custom-fields-json

Manage custom fields json

- **`redmine-pp-cli custom-fields-json`** - List custom fields

### enumerations

Status: Alpha, Availability: 2.2

- **`redmine-pp-cli enumerations get-document-categories`** - List document categories
- **`redmine-pp-cli enumerations get-issue-priorities`** - List issue priorities
- **`redmine-pp-cli enumerations get-time-entry-activities`** - List time entry activities

### groups

Status: Alpha, Availability: 2.1

- **`redmine-pp-cli groups delete`** - Delete group
- **`redmine-pp-cli groups get`** - Show group
- **`redmine-pp-cli groups update`** - Update group

### groups-json

Manage groups json

- **`redmine-pp-cli groups-json create-group`** - Create group
- **`redmine-pp-cli groups-json get-groups`** - List groups

### issue-categories

Status: Alpha, Availability: 1.3

- **`redmine-pp-cli issue-categories delete-issue-category`** - Delete issue category
- **`redmine-pp-cli issue-categories get-issue-category`** - Show issue category
- **`redmine-pp-cli issue-categories update-issue-category`** - Update issue category

### issue-statuses-json

Manage issue statuses json

- **`redmine-pp-cli issue-statuses-json`** - List issue statuses

### issues

Status: Stable, Availability: 1.0

- **`redmine-pp-cli issues delete`** - Delete issue
- **`redmine-pp-cli issues get`** - Show issue
- **`redmine-pp-cli issues update`** - Update issue

### issues-json

Manage issues json

- **`redmine-pp-cli issues-json create-issue`** - Create issue
- **`redmine-pp-cli issues-json get-issues`** - List issues

### journals

Status: Alpha, Availability: 5.0

- **`redmine-pp-cli journals <journal_id>`** - The journal is deleted when it is updated with empty `notes` and has no property changes (`details`).

### memberships

Manage memberships

- **`redmine-pp-cli memberships delete`** - Delete membership
- **`redmine-pp-cli memberships get`** - Show membership
- **`redmine-pp-cli memberships update`** - Warning: sending an empty role_ids returns 422 but still destroys the membership — role assignments are saved immediately and a member left without roles is removed.

### my

Manage my

- **`redmine-pp-cli my get-account`** - Show my account
- **`redmine-pp-cli my update-account`** - Requests authenticated via OAuth are rejected with 403, regardless of the token's scopes.

### news

Status: Stable, Availability: 1.1

- **`redmine-pp-cli news delete`** - Delete news
- **`redmine-pp-cli news get`** - Show news
- **`redmine-pp-cli news update`** - Update news

### news-json

Manage news json

- **`redmine-pp-cli news-json`** - List news

### projects

Status: Stable, Availability: 1.0

- **`redmine-pp-cli projects delete`** - Delete project
- **`redmine-pp-cli projects get`** - Show project
- **`redmine-pp-cli projects update`** - Update project

### projects-json

Manage projects json

- **`redmine-pp-cli projects-json create-project`** - Create project
- **`redmine-pp-cli projects-json get-projects`** - List projects

### queries-json

Manage queries json

- **`redmine-pp-cli queries-json`** - List queries

### relations

Manage relations

- **`redmine-pp-cli relations delete-issue`** - Delete issue relation
- **`redmine-pp-cli relations get-issue`** - Show issue relation

### roles

Status: Alpha, Availability: 1.4

- **`redmine-pp-cli roles <role_id>`** - Show role

### roles-json

Manage roles json

- **`redmine-pp-cli roles-json`** - List roles

### search-json

Manage search json

- **`redmine-pp-cli search-json`** - Search

### time-entries

Status: Stable, Availability: 1.1

- **`redmine-pp-cli time-entries delete-time-entry`** - Delete time entry
- **`redmine-pp-cli time-entries get-time-entry`** - Show time entry
- **`redmine-pp-cli time-entries update-time-entry`** - Update time entry

### time-entries-json

Manage time entries json

- **`redmine-pp-cli time-entries-json create-time-entry`** - Create time entry
- **`redmine-pp-cli time-entries-json get-time-entries`** - List time entries

### trackers-json

Manage trackers json

- **`redmine-pp-cli trackers-json`** - List trackers

### uploads-json

Manage uploads json

- **`redmine-pp-cli uploads-json`** - Upload attachment file

### users

Status: Stable, Availability: 1.1

- **`redmine-pp-cli users delete`** - Delete user
- **`redmine-pp-cli users get`** - Show user
- **`redmine-pp-cli users get-current`** - Show current user
- **`redmine-pp-cli users update`** - Update user

### users-json

Manage users json

- **`redmine-pp-cli users-json create-user`** - Create user
- **`redmine-pp-cli users-json get-users`** - List users

### versions

Status: Alpha, Availability: 1.3

- **`redmine-pp-cli versions delete`** - Delete version
- **`redmine-pp-cli versions get`** - Show version
- **`redmine-pp-cli versions update`** - Update version


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`redmine-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`redmine-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`redmine-pp-cli learnings list`** - Inspect taught rows
- **`redmine-pp-cli learnings forget <query>`** - Undo a teach
- **`redmine-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`redmine-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`redmine-pp-cli teach-pattern`** - Install a query/resource template up front
- **`redmine-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `REDMINE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `redmine-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
redmine-pp-cli attachments get mock-value

# JSON for scripting and agents
redmine-pp-cli attachments get mock-value --json
# Filter to specific fields
redmine-pp-cli attachments get mock-value --json --select attachment

# Dry run — show the request without sending
redmine-pp-cli attachments get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
redmine-pp-cli attachments get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `REDMINE_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `redmine-pp-cli custom-fields-json`
- `redmine-pp-cli custom-fields-json get`
- `redmine-pp-cli custom-fields-json list`
- `redmine-pp-cli custom-fields-json search`
- `redmine-pp-cli enumerations`
- `redmine-pp-cli enumerations get`
- `redmine-pp-cli enumerations list`
- `redmine-pp-cli enumerations search`
- `redmine-pp-cli enumerations-issue-priorities-json`
- `redmine-pp-cli enumerations-issue-priorities-json get`
- `redmine-pp-cli enumerations-issue-priorities-json list`
- `redmine-pp-cli enumerations-issue-priorities-json search`
- `redmine-pp-cli enumerations-time-entry-activities-json`
- `redmine-pp-cli enumerations-time-entry-activities-json get`
- `redmine-pp-cli enumerations-time-entry-activities-json list`
- `redmine-pp-cli enumerations-time-entry-activities-json search`
- `redmine-pp-cli groups-json`
- `redmine-pp-cli groups-json get`
- `redmine-pp-cli groups-json list`
- `redmine-pp-cli groups-json search`
- `redmine-pp-cli issue-statuses-json`
- `redmine-pp-cli issue-statuses-json get`
- `redmine-pp-cli issue-statuses-json list`
- `redmine-pp-cli issue-statuses-json search`
- `redmine-pp-cli issues-json`
- `redmine-pp-cli issues-json get`
- `redmine-pp-cli issues-json list`
- `redmine-pp-cli issues-json search`
- `redmine-pp-cli memberships-json`
- `redmine-pp-cli memberships-json get`
- `redmine-pp-cli memberships-json list`
- `redmine-pp-cli memberships-json search`
- `redmine-pp-cli news-json`
- `redmine-pp-cli news-json get`
- `redmine-pp-cli news-json list`
- `redmine-pp-cli news-json search`
- `redmine-pp-cli projects-json`
- `redmine-pp-cli projects-json get`
- `redmine-pp-cli projects-json list`
- `redmine-pp-cli projects-json search`
- `redmine-pp-cli queries-json`
- `redmine-pp-cli queries-json get`
- `redmine-pp-cli queries-json list`
- `redmine-pp-cli queries-json search`
- `redmine-pp-cli roles-json`
- `redmine-pp-cli roles-json get`
- `redmine-pp-cli roles-json list`
- `redmine-pp-cli roles-json search`
- `redmine-pp-cli search-json`
- `redmine-pp-cli search-json get`
- `redmine-pp-cli search-json list`
- `redmine-pp-cli search-json search`
- `redmine-pp-cli time-entries-json`
- `redmine-pp-cli time-entries-json get`
- `redmine-pp-cli time-entries-json list`
- `redmine-pp-cli time-entries-json search`
- `redmine-pp-cli trackers-json`
- `redmine-pp-cli trackers-json get`
- `redmine-pp-cli trackers-json list`
- `redmine-pp-cli trackers-json search`
- `redmine-pp-cli users-json`
- `redmine-pp-cli users-json get`
- `redmine-pp-cli users-json list`
- `redmine-pp-cli users-json search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `REDMINE_PROJECT_ID` resolves `{project_id}`

Base URL: `http://redmine:3000`

## Health Check

```bash
redmine-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `redmine-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/redmine-pp-cli/config.toml`; `--home`, `REDMINE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `REDMINE_PROJECT_ID` | endpoint | Yes |  |
| `REDMINE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `redmine-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `redmine-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $REDMINE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports 'connection refused' or a timeout** — Confirm REDMINE_URL points at a reachable instance (e.g. http://redmine:3000 inside this devcontainer, http://localhost:3001 from the host browser) and that the Redmine container is healthy.
- **401 Unauthorized on every command** — Confirm REDMINE_API_KEY is set and REST API access is enabled on the instance (Administration > Settings > API > 'Enable REST web service').
- **issues-json create-issue fails with 'tracker_id is invalid' or similar** — Run redmine-pp-cli trackers-json (or issue-statuses-json / enumerations get-issue-priorities) to get valid IDs for this instance — trackers, statuses, and priorities are configured per-instance and IDs are not portable across Redmine installs.
- **roadmap burndown, issues cycle-time, or workload undercounts closed issues** — The default 'sync' only pulls open issues (Redmine's own API default). Re-sync with '--resource-param issues-json:status_id=*' to include closed issues in the local mirror before running these commands.
- **roadmap burndown or workload returns empty for a project that has data** — Run sync first (or re-run it) — these commands read only from the local mirror, not live from the API.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `REDMINE_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**redmine-mcp-server**](https://github.com/onozaty/redmine-mcp-server) — TypeScript (22 stars)
- [**redmine-openapi**](https://github.com/d-yoshi/redmine-openapi) — YAML (17 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

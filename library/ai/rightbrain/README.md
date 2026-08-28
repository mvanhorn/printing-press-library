# Rightbrain CLI

**The first Rightbrain CLI that reaches past tasks — agents, approvals, evals, triggers and audit, plus a local mirror that makes credit spend and latency regressions queryable.**

Rightbrain's own CLI covers login and task CRUD; the rest of the platform has no command line at all. This one exposes the whole user-reachable API surface, resolves your org and project from config so you never paste two UUIDs into a command, and mirrors runs into SQLite so the questions the API can only answer one page at a time — which revision is actually serving traffic, what got slower this week, which agent runs are parked awaiting approval — become single commands.

Learn more at [Rightbrain](https://docs.rightbrain.ai/api).

Created by [@papaonlegs](https://github.com/papaonlegs) (Farouk Umar).

## Install

The recommended path installs both the `rightbrain-pp-cli` binary and the `pp-rightbrain` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install rightbrain
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install rightbrain --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install rightbrain --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install rightbrain --agent claude-code
npx -y @mvanhorn/printing-press-library install rightbrain --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/rightbrain/cmd/rightbrain-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rightbrain-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install rightbrain --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rightbrain --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-rightbrain --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install rightbrain --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rightbrain-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RB_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/rightbrain/cmd/rightbrain-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "rightbrain": {
      "command": "rightbrain-pp-mcp",
      "env": {
        "RB_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Rightbrain authenticates every request with `Authorization: Bearer <token>`. The simplest path is an API key created under Settings -> API Clients in the dashboard: set `RB_API_KEY`, plus `RB_ORG_ID` and `RB_PROJECT_ID` for the project it is scoped to. Those are the same three variables Rightbrain's own `rightbrain init` writes into `.env`, so an existing setup works unchanged. For service-to-service use, mint an OAuth 2.0 access token yourself at `https://oauth.rightbrain.ai/oauth2/token` and hand the result to this CLI as `RB_API_KEY` or via `auth set-token` — the CLI consumes an already-minted bearer token and does not perform the exchange for you. Org and project always come from the URL path rather than the token, which is why this CLI keeps them in config and injects them for you. Run `rightbrain-pp-cli doctor` to confirm credentials, reachability, and cache state in one shot.

## Quick Start

```bash
# Confirm the API is reachable and your credentials resolve before anything else
rightbrain-pp-cli doctor

# Show which user, org, and project the current token resolves to
rightbrain-pp-cli whoami

# Mirror the project locally; a first sync must walk the org and project parents, so run it unfiltered
rightbrain-pp-cli sync

# Find a task by name or prompt text without opening the dashboard
rightbrain-pp-cli search "summarize" --type project_task

# See what moved on cost, latency, and failure rate this week
rightbrain-pp-cli drift --since 7d --group-by task

# Catch any agent run parked awaiting a human before a customer does
rightbrain-pp-cli approvals --older-than 1h

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Ship changes without breaking production
- **`gate`** — Decide whether a candidate task revision is safe to promote, by comparing its eval pass rate against the last recorded result for a different revision of the same task.

  _Reach for this instead of running an eval and eyeballing the number when you need a machine-checkable promote/block decision with a non-zero exit code. Sync eval history first so a baseline exists._

  ```bash
  rightbrain-pp-cli gate 0195d1ff-1f05-437a-95ac-6de8969cb47b --min-pass-rate 0.9 --agent
  ```
- **`rollout`** — Show what traffic each task revision actually received versus the weight you configured, with failure rate, credits, and p50/p95 latency per revision.

  _Use this when an A/B revision is live and you need to know whether the split is real and whether the canary is slower or pricier._

  ```bash
  rightbrain-pp-cli rollout 0195d1ff-1f05-437a-95ac-6de8969cb47b --since 7d --agent
  ```
- **`eval-flake`** — Rank a task's eval test cases by how often they fail, separating genuine flake from cases that fail consistently.

  _Use this when the gate fails and you need to know whether the eval set itself is unreliable before touching the prompt._

  ```bash
  rightbrain-pp-cli eval-flake 0195d1ff-1f05-437a-95ac-6de8969cb47b --last 10 --agent
  ```

### Keep agents unblocked
- **`approvals`** — Triage every pending agent approval request by how long it has been parked and how soon it expires, with expired requests separated from actionable ones.

  _Run this first when an agent 'stopped responding' — a run parked at waiting_for_human emits nothing and pages nobody, and an approval whose window lapsed needs a re-run rather than a decision._

  ```bash
  rightbrain-pp-cli approvals --older-than 1h --agent
  ```
- **`agent-trace`** — Reconstruct one agent run as an indented timeline of paired tool calls and results with per-step elapsed time and a tool-duration histogram.

  _Reach for this when an agent run was slow or failed and you need to see which tool call hung, rather than replaying a raw event stream._

  ```bash
  rightbrain-pp-cli agent-trace 0195d207-32bb-d03d-cfdc-f4516e9222c8 --tools --agent
  ```

### Account for spend and latency
- **`drift`** — Compare this window against the previous one across every task and agent — mean credits, tokens, p95 latency, failure rate — and flag what moved.

  _Use this for the weekly regression sweep; it answers 'did anything get slower or pricier' without paging through run history per task._

  ```bash
  rightbrain-pp-cli drift --since 7d --group-by task --agent --select movers.name,movers.credits_delta_pct,movers.p95_delta_pct
  ```
- **`changelog`** — What changed in the project over a window, with resource UUIDs resolved to task, agent, skill, and collection names, plus an optional tamper-evidence verdict via --verify.

  _Reach for this for a Friday status or a compliance question — it is the only view that reads in names rather than UUIDs._

  ```bash
  rightbrain-pp-cli changelog --since 7d --verify --agent
  ```

## Recipes

### Gate a revision before promoting it

```bash
rightbrain-pp-cli gate 0195d1ff-1f05-437a-95ac-6de8969cb47b --min-pass-rate 0.9
```

Runs the task's eval set against the candidate revision, compares the pass rate to the last recorded result for the revision currently taking traffic, and exits non-zero if it regressed — so it drops straight into CI.

### Check whether an A/B split is real

```bash
rightbrain-pp-cli rollout 0195d1ff-1f05-437a-95ac-6de8969cb47b --since 7d
```

Puts each revision's configured weight next to the traffic share it actually received, along with failure rate, mean credits, and p50/p95 latency, which is how you catch a canary that is starved or three times slower.

### Find every agent run stuck on a human

```bash
rightbrain-pp-cli approvals --older-than 1h --agent
```

Fans out across every agent in the project and returns one queue of runs parked at waiting_for_human sorted by parked age, with the gated tool named — the view the API and dashboard both lack.

### Narrow a large drift report for an agent

```bash
rightbrain-pp-cli drift --since 7d --group-by task --agent --select movers.name,movers.p95_delta_pct,movers.credits_delta_pct
```

Drift over a busy project returns a lot of per-task detail; --select trims the payload to just the mover name and the two deltas that decide whether to investigate, keeping agent context small.

### Trace a slow agent run

```bash
rightbrain-pp-cli agent-trace 0195d207-32bb-d03d-cfdc-f4516e9222c8 --tools
```

Rebuilds the run as a timeline of paired tool calls and results with elapsed time per step and a tool-duration histogram, so the nine-second step is obvious instead of buried in a flat event array.

### Write the Friday client update

```bash
rightbrain-pp-cli changelog --since 7d --verify
```

Lists what changed in the project with UUIDs resolved to task and agent names, and attaches the cryptographic integrity verdict so the same output answers the compliance question.

## Usage

Run `rightbrain-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `RIGHTBRAIN_CONFIG_DIR`, `RIGHTBRAIN_DATA_DIR`, `RIGHTBRAIN_STATE_DIR`, or `RIGHTBRAIN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `RIGHTBRAIN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export RIGHTBRAIN_HOME=/srv/rightbrain
rightbrain-pp-cli doctor
```

Under `RIGHTBRAIN_HOME=/srv/rightbrain`, the four dirs resolve to `/srv/rightbrain/config`, `/srv/rightbrain/data`, `/srv/rightbrain/state`, and `/srv/rightbrain/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "rightbrain": {
      "command": "rightbrain-pp-mcp",
      "env": {
        "RIGHTBRAIN_HOME": "/srv/rightbrain"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `RIGHTBRAIN_DATA_DIR` overrides an explicit `--home` for that kind. Use `RIGHTBRAIN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `RIGHTBRAIN_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `rightbrain-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### agent_shares

Manage agent shares

- **`rightbrain-pp-cli agent-shares <org_id> <project_id>`** - List TaskAgent shares in a project.

TaskAgent shares are public share records scoped to the requested project. The
endpoint does not expose share-specific IAM.

### avatar

Manage avatar

- **`rightbrain-pp-cli avatar <org_id>`** - Update organization avatar image

### domains

Manage domains

- **`rightbrain-pp-cli domains create-organization`** - Org Domains Post
- **`rightbrain-pp-cli domains delete-organization`** - Org Domain Delete
- **`rightbrain-pp-cli domains list-organization`** - Org Domains List

### iam

Manage iam

- **`rightbrain-pp-cli iam organization-get-member`** - Retrieve a specific member that has been granted direct access to the organization.
- **`rightbrain-pp-cli iam organization-list-members`** - Lists all members that have been granted direct access to the organization.
- **`rightbrain-pp-cli iam organization-test-permissions`** - Test the permissions that the caller (or another subject) holds on the organization.
- **`rightbrain-pp-cli iam organization-update-member-roles`** - Update the roles that a member holds on the organization.

### integration

Manage integration

- **`rightbrain-pp-cli integration`** - Handle the OAuth callback for a platform integration.

### invite

Manage invite

- **`rightbrain-pp-cli invite create-organization`** - Org Invites Create
- **`rightbrain-pp-cli invite delete-organization`** - Org Invites Delete
- **`rightbrain-pp-cli invite list-organization`** - Org Invites List

### join

Manage join

- **`rightbrain-pp-cli join <org_id>`** - Org Join

### model

Manage model

- **`rightbrain-pp-cli model exclude-by-id-from-org`** - Exclude one model across the organization.
Requires `organization:edit` access. Organization exclusions are inherited by every
project and cannot be overridden at project scope. Provider and vendor rules also
apply to future matching models. A request is rejected if it would leave any
affected project without at least one active model.
- **`rightbrain-pp-cli model exclude-by-provider-from-org`** - Exclude all models supplied by a provider across the organization.
Requires `organization:edit` access. Organization exclusions are inherited by every
project and cannot be overridden at project scope. Provider and vendor rules also
apply to future matching models. A request is rejected if it would leave any
affected project without at least one active model.
- **`rightbrain-pp-cli model exclude-by-vendor-from-org`** - Exclude all models served by a vendor across the organization.
Requires `organization:edit` access. Organization exclusions are inherited by every
project and cannot be overridden at project scope. Provider and vendor rules also
apply to future matching models. A request is rejected if it would leave any
affected project without at least one active model.
- **`rightbrain-pp-cli model list-exclusions-for-org`** - List organization rules with their impact on active models. 
Requires `organization:edit` access. Organization exclusions are inherited by every
project and cannot be overridden at project scope. Provider and vendor rules also
apply to future matching models. A request is rejected if it would leave any
affected project without at least one active model.
- **`rightbrain-pp-cli model remove-exclusion-from-org`** - Remove an organization rule and widen model availability. 
Requires `organization:edit` access. Organization exclusions are inherited by every
project and cannot be overridden at project scope. Provider and vendor rules also
apply to future matching models. A request is rejected if it would leave any
affected project without at least one active model.

### org

Manage org

- **`rightbrain-pp-cli org create-organization`** - Org Create
- **`rightbrain-pp-cli org get-organization`** - Org Get
- **`rightbrain-pp-cli org list-organizations`** - List organizations based on the user's relationship to them.

Use the `membership` parameter to filter organizations by your access level:
- **active**: Organizations you are a member of (default)
- **joinable**: Organizations you can request to join
- **joinable_by_domain**: Organizations that accept members from your email domain
- **joinable_by_invite**: Organizations you have pending invites to

This endpoint is useful for:
- Displaying the user's current organizations
- Showing organizations the user can join
- Building organization switching interfaces
- **`rightbrain-pp-cli org update-organization`** - Org Update

### project

Manage project

- **`rightbrain-pp-cli project create`** - Create Project
- **`rightbrain-pp-cli project delete`** - Soft delete a project.

Marks the project as deleted without removing it from the database.
Requires create_project permission on the owning organization.

- **`rightbrain-pp-cli project get`** - Get Project
- **`rightbrain-pp-cli project list`** - List Project
- **`rightbrain-pp-cli project update`** - Update Project

### public

Manage public

- **`rightbrain-pp-cli public get-task-agent-share`** - Return a public TaskAgent share page or sanitized public data.

Browser/default requests render a human-readable HTML share page. Clients that
send `Accept: application/json`, `application/yaml`, or `text/yaml` receive
sanitized public data.

The public DTO is allowlisted. It intentionally includes operational
configuration needed to understand the shared agent, such as input processor
configuration and tool names, but redacts credentials, tokens, auth configs,
ciphertext, connected user IDs, source-project provenance, tags, runs, files,
and sessions. Public paths remain present in OpenAPI for schema generation but
are marked `x-fern-ignore` by the `/public/*` OpenAPI configuration.
- **`rightbrain-pp-cli public get-task-share`** - Access a task via share link.

Returns task details based on share permissions.
May require additional authentication for sensitive tasks.
- **`rightbrain-pp-cli public get-task-share-file`** - Download a file (image or document) from a task run example associated with a public task share

### runs

Manage runs

- **`rightbrain-pp-cli runs get-project-task-timing-report`** - Aggregate visible Task timing across the project.
- **`rightbrain-pp-cli runs get-project-task-usage-report`** - Aggregate visible Task usage across the project.
- **`rightbrain-pp-cli runs list-project-task`** - List all task runs across a project.

Returns a paginated list of task runs for all tasks in the project.
Runs belonging to soft-deleted tasks are excluded. Visibility is applied
per task: users only see runs they would see on each task run list
(owner_only / editors_and_owners / all_viewers). A page may contain
fewer than page_limit results when visibility filters apply.

**Filtering:** task_id, task_revision_id, status (success/error), start_date, end_date.

### shares

Manage shares

- **`rightbrain-pp-cli shares <org_id> <project_id>`** - List all task shares across a project.

Returns a paginated list of all task shares for tasks within the project.
Shares for soft-deleted tasks are excluded.

**Filtering options:**
- `active`: Filter by active status (true/false/omit for all)
- `task_id`: Filter to shares for a specific task
- `task_name`: Filter by task name (case-insensitive partial match)

**Use cases:**
- Audit all public shares in a project
- Find active shares for a specific task
- Review all shares created by the team

### skills

Manage skills

- **`rightbrain-pp-cli skills get`** - Retrieve lightweight metadata and active revision summary for a skill.
- **`rightbrain-pp-cli skills list`** - Browse the global skill catalog using lightweight metadata only. Supports filtering by source, tag name, and free-text search.
- **`rightbrain-pp-cli skills list-sources`** - List all available skill sources in the global catalog.
- **`rightbrain-pp-cli skills list-tags`** - List global skill tags available for filtering the declarative skill catalog.

### task-mcp-server

Manage task mcp server

- **`rightbrain-pp-cli task-mcp-server`** - Callback Task Mcp Server

### user

Manage user

- **`rightbrain-pp-cli user get-current`** - Get user profile information.

Returns:
- User ID and email
- Name and avatar
- Organization memberships
- Account settings
- **`rightbrain-pp-cli user update-current`** - Update current user profile information
- **`rightbrain-pp-cli user upload-avatar`** - Upload and update user avatar image.

Accepts image files up to 5MB in size.
Supported formats: JPEG, PNG, GIF, WebP

### whoami

Manage whoami

- **`rightbrain-pp-cli whoami`** - Get current authenticated user information including token details.

Returns the authenticated user's basic information (ID, email, name).
If authenticated via OAuth client credentials, also includes the client's
organization and project context.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`rightbrain-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`rightbrain-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`rightbrain-pp-cli learnings list`** - Inspect taught rows
- **`rightbrain-pp-cli learnings forget <query>`** - Undo a teach
- **`rightbrain-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`rightbrain-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`rightbrain-pp-cli teach-pattern`** - Install a query/resource template up front
- **`rightbrain-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `RIGHTBRAIN_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `rightbrain-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rightbrain-pp-cli project list "$RB_ORG_ID"

# JSON for scripting and agents
rightbrain-pp-cli project list "$RB_ORG_ID" --json

# Filter to specific fields
rightbrain-pp-cli project list "$RB_ORG_ID" --json --select id,name,status

# Dry run — show the request without sending
rightbrain-pp-cli project list "$RB_ORG_ID" --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rightbrain-pp-cli project list "$RB_ORG_ID" --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
rightbrain-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `rightbrain-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/rightbrain-pp-cli/config.toml`; `--home`, `RIGHTBRAIN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RB_API_KEY` | per_call | Yes | Set to your API credential. |
| `RB_ORG_ID` | scope | No | Default organization ID. Also settable with `scope use` or `--org-id`. |
| `RB_PROJECT_ID` | scope | No | Default project ID. Also settable with `scope use` or `--project-id`. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `rightbrain-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `rightbrain-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RB_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every command fails with 401 and `Access credentials are invalid`** — Set RB_API_KEY to a key from Settings -> API Clients, then run `rightbrain-pp-cli doctor` to confirm.
- **403 on a command that used to work** — The key is scoped to one user and one project; check RB_PROJECT_ID matches the project holding the resource.
- **404 for a task or agent you can see in the dashboard** — Wrong org or project in config. Run `rightbrain-pp-cli whoami` and compare against the dashboard URL.
- **A local command returns nothing at all** — The local mirror is empty. Run `rightbrain-pp-cli sync --resources project_task,project_task_run --full` first.
- **409 when running an agent with a pinned revision** — The revision does not match the session's revision. Start a new session or drop the revision pin.
- **422 with a list of fields** — Schema validation failed. The response names the offending field under `detail[].loc`; task runs need a `task_input` object, not `input_params`.
- **An agent run stops producing output and never completes** — It is parked at waiting_for_human on a gated tool. Run `rightbrain-pp-cli approvals` to find and resolve it.

## Known Gaps

Honest limits of this release.

- **Three commands have not been exercised against live data.** `gate`, `rollout`,
  and `eval-flake` all read eval-run or task-run history. The workspace this CLI
  was verified against has neither, so they are covered by unit tests and by a
  seeded local mirror with known values — not by real Rightbrain data. `approvals`,
  `agent-trace`, `changelog`, `drift`, `sync`, `search`, `doctor`, and `whoami`
  were all run live.
- **A first `sync` must be unfiltered.** Every Rightbrain resource is parent-keyed
  on organization then project, and `sync --resources <child>` does not cascade
  upward to populate those parents. On a fresh machine a filtered sync reports
  success with zero records. Run plain `sync` first; filtered syncs work from then on.
- **No browser OAuth login.** Rightbrain's own npm CLI has `login`; this one does
  not. Supply an already-minted credential via `RB_API_KEY` or `auth set-token`.
  There is also no `token` print command — deliberately, since writing a live
  bearer token to stdout is a leak vector — and no environment switching.
- **Two spec endpoints were dropped as redundant.** `GET .../task/{task_id}/run`
  and `GET .../task_run/recent_by_task` collided into a single sync resource in
  the generator. Both are strict subsets of `runs list-project-task`, which takes
  the same filters plus `task_id`, so no capability was lost.
- **Two OAuth callback endpoints are not exposed as commands.**
  `/integration/callback` and `/task_mcp_server/callback` are redirect targets an
  OAuth provider calls; a person supplying their own `code`/`state` can never
  succeed. They are removed from the command surface rather than shipped as
  commands that always fail. The real surfaces live under `project integration`
  and `project task-mcp-server`.
- **`sync --json` emits newline-delimited JSON**, one object per progress event,
  rather than a single document. That is the right shape for a streaming command
  but differs from every other `--json` surface here; parse it per line.
- **Staff-only endpoints are not exposed.** 33 `/internal/*` operations require an
  `internal.admin` scope no customer credential carries; they are excluded rather
  than shipped as commands that always fail.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**github-action-tasks**](https://github.com/RightbrainAI/github-action-tasks) — JavaScript (1 stars)
- [**rightbrain-sdk-demo**](https://github.com/RightbrainAI/rightbrain-sdk-demo) — TypeScript (1 stars)
- [**rightbrain-agent-skill**](https://github.com/RightbrainAI/rightbrain-agent-skill) — JavaScript
- [**terraform-provider-tasks**](https://github.com/RightbrainAI/terraform-provider-tasks) — Go
- [**bruno**](https://github.com/RightbrainAI/bruno) — JSON
- [**cursor-plugin**](https://github.com/RightbrainAI/cursor-plugin) — Markdown

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

# Conductor Cloud CLI

**Every Conductor Cloud API primitive, plus bounded session orchestration that handles asynchronous lifecycle races.**

Use `launch`, `monitor`, `steer`, and `run` to control coding-agent work without relying on the desktop app. The CLI treats status changes, transcript cursors, cancellation, and timeouts as one lifecycle instead of unrelated API calls.

## Install

The recommended path installs both the `conductor-pp-cli` binary and the `pp-conductor` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install conductor
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install conductor --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install conductor --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install conductor --agent claude-code
npx -y @mvanhorn/printing-press-library install conductor --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/conductor/cmd/conductor-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/conductor-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install conductor --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-conductor --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-conductor --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install conductor --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/conductor-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CONDUCTOR_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/conductor/cmd/conductor-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "conductor": {
      "command": "conductor-pp-mcp",
      "env": {
        "CONDUCTOR_SESSIONID": "<sessionId>",
        "CONDUCTOR_WORKSPACEID": "<workspaceId>",
        "CONDUCTOR_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set `CONDUCTOR_API_KEY` to a Conductor Cloud API key. The CLI sends it only as a bearer token to `api.conductor.build` and never prints it.

## Quick Start

```bash
# Check configuration and command wiring without credentials or network mutations.
conductor-pp-cli doctor --dry-run

# Confirm the key resolves to the expected Conductor user and organization.
conductor-pp-cli me --agent

# Find a project ID before launching a project-backed workspace.
conductor-pp-cli projects list --limit 20 --agent

# Preview a bounded launch payload before creating anything.
conductor-pp-cli launch --repository-url https://github.com/example/acme --branch main --harness codex --model gpt-5.4 --effort high --brief 'Run the focused test suite' --dry-run --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Bounded agent orchestration
- **`launch`** — Create a workspace and first session, send a brief, and return the Conductor deep link.

  _Use this when an agent needs a new isolated Conductor workspace and task in one call._

  ```bash
  conductor-pp-cli launch --repository-url https://github.com/example/acme --branch main --harness codex --model gpt-5.4 --effort high --brief-file issue.md --agent
  ```
- **`run`** — Launch, monitor, and collect the final transcript and deep link with explicit timeout rules.

  _Use this when the caller needs a complete Conductor task receipt, not just a launched workspace._

  ```bash
  conductor-pp-cli run --repository-url https://github.com/example/acme --branch main --harness codex --model gpt-5.4 --effort high --brief-file issue.md --timeout 30m --agent
  ```
- **`plan-implement`** — Keep a planner or reviewer session separate from the implementation session in one workspace.

  _Use this when implementation should start from an explicit plan without mixing planner and coder context._

  ```bash
  conductor-pp-cli plan-implement --repository-url https://github.com/example/acme --branch main --planner-agent claude --implementer-agent codex --brief-file issue.md --agent
  ```

### Safe lifecycle control
- **`monitor`** — Poll a session until real completion while streaming only new transcript events.

  _Use this instead of treating one idle response as proof that a queued task finished._

  ```bash
  conductor-pp-cli monitor sess_123 --timeout 30m --interval 5s --dry-run --agent
  ```
- **`steer`** — Send follow-up guidance to an existing Conductor session with a clear delivery receipt.

  _Use this to correct or refine active work without creating another session._

  ```bash
  conductor-pp-cli steer sess_123 --message 'Run the focused tests before changing the schema' --dry-run --agent
  ```

### Transcript operations
- **`daily-report`** — Return recent Conductor session rows and mechanical activity totals from transcript search.

  _Use this for a compact, deterministic activity feed that another agent can analyze._

  ```bash
  conductor-pp-cli daily-report --since 24h --limit 50 --agent
  ```

## Recipes

### Monitor an existing session

```bash
conductor-pp-cli monitor sess_123 --timeout 30m --interval 5s --agent
```

Streams incremental events and returns only after the task has demonstrably started and later completed.

### Send steering guidance

```bash
conductor-pp-cli steer sess_123 --message 'Keep the change scoped to the parser' --agent
```

Adds guidance to the current session without creating a second workspace.

### Review recent work

```bash
conductor-pp-cli daily-report --since 24h --limit 50 --agent
```

Returns mechanical session activity for downstream review or reporting.

## Usage

Run `conductor-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CONDUCTOR_CONFIG_DIR`, `CONDUCTOR_DATA_DIR`, `CONDUCTOR_STATE_DIR`, or `CONDUCTOR_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CONDUCTOR_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CONDUCTOR_HOME=/srv/conductor
conductor-pp-cli doctor
```

Under `CONDUCTOR_HOME=/srv/conductor`, the four dirs resolve to `/srv/conductor/config`, `/srv/conductor/data`, `/srv/conductor/state`, and `/srv/conductor/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "conductor": {
      "command": "conductor-pp-mcp",
      "env": {
        "CONDUCTOR_HOME": "/srv/conductor"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CONDUCTOR_DATA_DIR` overrides an explicit `--home` for that kind. Use `CONDUCTOR_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CONDUCTOR_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `conductor-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### me

Manage me

- **`conductor-pp-cli me`** - Get authenticated identity.

### messages

Manage messages

- **`conductor-pp-cli messages <messageId>`** - Get a message.

### projects

Manage projects

- **`conductor-pp-cli projects get`** - Get a project.
- **`conductor-pp-cli projects list`** - List projects.

### roundhouse-public-sql

Manage roundhouse public sql

- **`conductor-pp-cli roundhouse-public-sql`** - Runs a single read-only SQL SELECT statement over your organization's session transcripts and returns the matching rows.

Queries may ONLY read from the view session_transcripts_view; every other table, view, and function is off-limits. Query it like a table, e.g. SELECT ... FROM session_transcripts_view. Its columns are:

- session_id (text): ID of the session (chat).
- workspace_id (text): ID of the workspace the session belongs to.
- transcript (text): concise plain-text transcript of the session's conversation.
- session_title (text, nullable): title of the session.
- agent_type (text, nullable): agent that ran the session, e.g. 'claude' or 'codex'.
- model (text, nullable): model the session used.
- workspace_name (text, nullable): display name of the workspace.
- workspace_state (text): workspace lifecycle state, e.g. 'ready' or 'archived'.
- repo_url (text): git remote URL of the workspace's repository.
- session_created_at (timestamptz): when the session was created.
- transcript_updated_at (timestamptz): when the transcript last changed.

Limits: at most 500 rows are returned (the response sets truncated: true when the query matched more), statements time out after 5 seconds, and the query text may be at most 10000 characters. The statement must be a single query: semicolon-chained statements, writes, U& Unicode-escape syntax, and queries containing the text set_config anywhere (even inside a search string) are rejected. Errors come back as 400s carrying the Postgres error message.

Example queries:

- Search transcripts: SELECT session_id, session_title, workspace_name FROM session_transcripts_view WHERE transcript ILIKE '%database migration%' ORDER BY transcript_updated_at DESC LIMIT 20
- Recent sessions in live workspaces: SELECT session_title, workspace_name, transcript_updated_at FROM session_transcripts_view WHERE workspace_state = 'ready' ORDER BY transcript_updated_at DESC LIMIT 50

### sessions

Manage sessions

- **`conductor-pp-cli sessions create`** - Creates a session in an existing workspace. If the workspace is still initializing, the session is accepted immediately and initialized in the workspace when its first message is delivered. Accepted model ids by agent — claude: fable-5, opus-5-1m, opus-4-8-1m, opus-4-8, opus-4-7-1m, opus-4-7, opus-1m, opus, opus-4-6-1m, sonnet-5-1m, sonnet-4-6-1m, sonnet, haiku; codex: gpt-5.5, gpt-5.4, gpt-5.6-sol, gpt-5.6-terra, gpt-5.6-luna, gpt-5.3-codex-spark, gpt-5.3-codex, gpt-5.2-codex; cursor: auto, composer-2.5, grok-4.5. Accepted effort levels by agent — claude: low, medium, high, xhigh, max; codex: none, low, medium, high, xhigh, max, ultra; codex max requires a GPT-5.6 model, and ultra requires GPT-5.6 Sol or Terra. Omit effort to use the agent's default — claude: high; codex: high. Models accepting fastMode by agent — claude: opus-5-1m, opus-4-8-1m, opus-4-8, opus-4-7-1m, opus-4-7, opus-1m, opus, opus-4-6-1m; codex: gpt-5.5, gpt-5.4, gpt-5.6-sol, gpt-5.6-terra, gpt-5.6-luna, gpt-5.3-codex-spark, gpt-5.3-codex, gpt-5.2-codex; cursor: auto, composer-2.5, grok-4.5. Omit fastMode (or send false) for other models.
- **`conductor-pp-cli sessions get`** - Get a session.

### workspaces

Manage workspaces

- **`conductor-pp-cli workspaces create`** - Creates a cloud workspace and first session, and returns a deep link that opens the workspace in the Conductor desktop app. Pass either projectId or repositoryUrl. With an organization API key, the workspace launches on the organization's machine; the machine must include the repository. Accepted model ids by agent — claude: fable-5, opus-5-1m, opus-4-8-1m, opus-4-8, opus-4-7-1m, opus-4-7, opus-1m, opus, opus-4-6-1m, sonnet-5-1m, sonnet-4-6-1m, sonnet, haiku; codex: gpt-5.5, gpt-5.4, gpt-5.6-sol, gpt-5.6-terra, gpt-5.6-luna, gpt-5.3-codex-spark, gpt-5.3-codex, gpt-5.2-codex; cursor: auto, composer-2.5, grok-4.5. Accepted effort levels by agent — claude: low, medium, high, xhigh, max; codex: none, low, medium, high, xhigh, max, ultra; codex max requires a GPT-5.6 model, and ultra requires GPT-5.6 Sol or Terra. Omit effort to use the agent's default — claude: high; codex: high.
- **`conductor-pp-cli workspaces get`** - Get a workspace.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`conductor-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`conductor-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`conductor-pp-cli learnings list`** - Inspect taught rows
- **`conductor-pp-cli learnings forget <query>`** - Undo a teach
- **`conductor-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`conductor-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`conductor-pp-cli teach-pattern`** - Install a query/resource template up front
- **`conductor-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CONDUCTOR_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `conductor-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
conductor-pp-cli me

# JSON for scripting and agents
conductor-pp-cli me --json

# Filter to specific fields
conductor-pp-cli me --json --select id,name,status

# Dry run — show the request without sending
conductor-pp-cli me --dry-run

# Agent mode — JSON + compact + no prompts in one flag
conductor-pp-cli me --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `CONDUCTOR_SESSION_ID` resolves `{sessionId}`
- `CONDUCTOR_WORKSPACE_ID` resolves `{workspaceId}`

Base URL: `https://api.conductor.build`

## Health Check

```bash
conductor-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `conductor-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/roundhouse-public-pp-cli/config.toml`; `--home`, `CONDUCTOR_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CONDUCTOR_SESSIONID` | endpoint | Yes |  |
| `CONDUCTOR_WORKSPACEID` | endpoint | Yes |  |
| `CONDUCTOR_API_KEY` | per_call | No | Set to your API credential. |
| `CONDUCTOR_BEARER_AUTH` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `conductor-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `conductor-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CONDUCTOR_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **The API returns UNAUTHORIZED.** — Set a valid `CONDUCTOR_API_KEY` and rerun `conductor-pp-cli doctor`.
- **A session reports idle immediately after a message is queued.** — Use `monitor`; it waits for working state or transcript movement before accepting a later idle state.
- **Cancellation returns before work has stopped.** — Use the confirmation mode and poll until session status returns to idle.

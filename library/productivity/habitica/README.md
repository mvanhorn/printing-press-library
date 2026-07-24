# Habitica CLI

**Turn chores into quests, check off workouts, and protect your reward budget from the command line.**

Habitica CLI covers the official task and game-management API, then layers a preview-first daily workflow on top. Use `today`, `plan chores`, `reward afford`, `tag load`, and `week review` to make account state useful to people and agents without accidental scoring or spending.

## Install

The recommended path installs both the `habitica-pp-cli` binary and the `pp-habitica` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install habitica
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install habitica --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install habitica --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install habitica --agent claude-code
npx -y @mvanhorn/printing-press-library install habitica --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/habitica/cmd/habitica-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/habitica-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install habitica --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-habitica --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-habitica --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install habitica --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/habitica-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HABITICA_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/habitica/cmd/habitica-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "habitica": {
      "command": "habitica-pp-mcp",
      "env": {
        "HABITICA_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set `HABITICA_USER_ID` and `HABITICA_API_TOKEN` from Habitica Settings → API. The CLI sends them only as the official `x-api-user` and `x-api-key` headers and uses its own `x-client` platform identifier; never place either value in a plan file or command line.

## Quick Start

```bash
# Check local configuration without sending a mutation.
habitica-pp-cli doctor --dry-run

# Start the day with a focused quest queue.
habitica-pp-cli today --agent --select tasks.text,tasks.type,stats.gp

# Decide whether a reward fits the remaining gold budget.
habitica-pp-cli reward afford "Weekend movie" --reserve-gp 20 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Daily ritual
- **`today`** — Turn today’s dailies, due to-dos, and character state into one actionable quest queue.

  _Use this before planning the day instead of issuing separate task and profile calls._

  ```bash
  habitica-pp-cli today --agent --select tasks.text,tasks.type,stats.gp
  ```
- **`plan chores`** — Preview a chore batch as Habitica quests and create it only after an explicit apply confirmation.

  _Use this to turn a prepared chore list into quests while retaining a complete mutation preview._

  ```bash
  habitica-pp-cli plan chores --file examples/chores.yaml --dry-run --agent
  ```

### Reward decisions
- **`reward afford`** — Check whether a reward fits current gold while preserving a chosen reserve for later goals.

  _Use this before spending gold when a generic purchase request cannot explain the tradeoff._

  ```bash
  habitica-pp-cli reward afford "Weekend movie" --reserve-gp 20 --agent
  ```

### Workload insight
- **`tag load`** — Compare active, due, overdue, and checklist-blocked work across Habitica tags.

  _Use this to rebalance chores by category rather than scanning individual tasks._

  ```bash
  habitica-pp-cli tag load --agent --select tags.name,tags.overdue
  ```
- **`week review`** — Review seven-day overdue, stalled, and completed-task trends from real local snapshots.

  _Use this in weekly review to see whether the Habitica system is improving or accumulating debt._

  ```bash
  habitica-pp-cli week review --agent --select overdue_delta,completed_delta
  ```

## Recipes

### Build today’s quest queue

```bash
habitica-pp-cli today --agent --select tasks.text,tasks.type,stats.gp
```

Narrow the daily briefing to the fields an agent needs for planning.

### Preview chores before creating quests

```bash
habitica-pp-cli plan chores --file examples/chores.yaml --dry-run --agent
```

Validate the batch without mutating; create it only after reviewing it and rerunning with --apply --yes.

### Protect weekend reward gold

```bash
habitica-pp-cli reward afford "Weekend movie" --reserve-gp 20 --agent
```

See affordability and the remaining balance before a reward purchase.

### Rebalance tagged work

```bash
habitica-pp-cli tag load --agent --select tags.name,tags.overdue
```

Use the live task and tag view to compare category pressure.

### Review seven-day task health

```bash
habitica-pp-cli week review --agent --select overdue_delta,completed_delta
```

Run sync before the first review; later local snapshots show whether the backlog is improving.

## Usage

Run `habitica-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HABITICA_CONFIG_DIR`, `HABITICA_DATA_DIR`, `HABITICA_STATE_DIR`, or `HABITICA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HABITICA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HABITICA_HOME=/srv/habitica
habitica-pp-cli doctor
```

Under `HABITICA_HOME=/srv/habitica`, the four dirs resolve to `/srv/habitica/config`, `/srv/habitica/data`, `/srv/habitica/state`, and `/srv/habitica/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "habitica": {
      "command": "habitica-pp-mcp",
      "env": {
        "HABITICA_HOME": "/srv/habitica"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `HABITICA_DATA_DIR` overrides an explicit `--home` for that kind. Use `HABITICA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HABITICA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `habitica-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### notifications

Read and acknowledge Habitica notifications

- **`habitica-pp-cli notifications`** - List notifications

### tags

Manage personal task tags

- **`habitica-pp-cli tags create`** - Create a personal tag
- **`habitica-pp-cli tags list`** - List personal tags

### tasks

Manage Habitica habits, dailies, to-dos, rewards, checklists, and task tags

- **`habitica-pp-cli tasks add-checklist`** - Add an item to a daily or todo checklist
- **`habitica-pp-cli tasks create-user`** - Create a habit, daily, todo, or reward
- **`habitica-pp-cli tasks delete`** - Delete a task
- **`habitica-pp-cli tasks get`** - Get a task by ID or alias
- **`habitica-pp-cli tasks list-user`** - List the authenticated user's tasks
- **`habitica-pp-cli tasks score`** - Score a task up or down
- **`habitica-pp-cli tasks score-checklist`** - Toggle a checklist item
- **`habitica-pp-cli tasks update`** - Update a task

### user

Inspect character state, rewards, inventory, and purchases

- **`habitica-pp-cli user buy`** - Buy an item with gold
- **`habitica-pp-cli user buy-list`** - List account-specific equipment available for purchase
- **`habitica-pp-cli user equip`** - Equip or unequip an item
- **`habitica-pp-cli user feed`** - Feed a pet
- **`habitica-pp-cli user get`** - Get the authenticated user profile and stats
- **`habitica-pp-cli user hatch`** - Hatch a pet
- **`habitica-pp-cli user in-app-rewards`** - List in-app rewards in the user's reward column


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`habitica-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`habitica-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`habitica-pp-cli learnings list`** - Inspect taught rows
- **`habitica-pp-cli learnings forget <query>`** - Undo a teach
- **`habitica-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`habitica-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`habitica-pp-cli teach-pattern`** - Install a query/resource template up front
- **`habitica-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `HABITICA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `habitica-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
habitica-pp-cli notifications

# JSON for scripting and agents
habitica-pp-cli notifications --json

# Filter to specific fields
habitica-pp-cli notifications --json --select id,name,status

# Dry run — show the request without sending
habitica-pp-cli notifications --dry-run

# Agent mode — JSON + compact + no prompts in one flag
habitica-pp-cli notifications --agent
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
habitica-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `habitica-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/habitica-pp-cli/config.toml`; `--home`, `HABITICA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HABITICA_API_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `habitica-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `habitica-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HABITICA_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **The API reports missing authentication or client headers.** — Set HABITICA_USER_ID and HABITICA_API_TOKEN, then run habitica-pp-cli doctor --json.
- **A read is rate limited.** — Wait for the API reset window shown in the error, then retry the bounded command.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Hopla**](https://github.com/melvio/hopla) — Python (20 stars)
- [**Habitica MCP Server**](https://github.com/iBreaker/habitica-mcp-server) — JavaScript (17 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

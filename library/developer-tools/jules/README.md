# Jules CLI

Jules Planning & Progress API for async coding tasks

Learn more at [Jules](https://jules.google/docs/api/).

Created by [@wryenmeek](https://github.com/wryenmeek) (github-actionsbot).

## Install

The recommended path installs both the `jules-pp-cli` binary and the `pp-jules` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install jules
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install jules --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install jules --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install jules --agent claude-code
npx -y @mvanhorn/printing-press-library install jules --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/jules/cmd/jules-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jules-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install jules --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-jules --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-jules --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install jules --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jules-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `JULES_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/jules/cmd/jules-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "jules": {
      "command": "jules-pp-mcp",
      "env": {
        "JULES_API_KEY": "<your-key>"
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

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export JULES_API_KEY="<paste-your-key>"
```
To persist credentials, pipe the token in: `echo "$JULES_API_KEY" | jules-pp-cli auth set-token` (there is no positional form, so the key never lands in shell history or `ps` output). Stored secrets live in `credentials.toml` under the data directory, not in `config.toml`.

### 3. Verify Setup

```bash
jules-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
jules-pp-cli sessions list
```

## Unique Features

Beyond mirroring the Jules API, this CLI ships nine hand-coded commands for
running Jules sessions unattended and safely at scale:

- **`sessions create --quota-safe`** — Check quota before dispatching a new
  session and retry on 429/400 with exponential backoff (`--max-retries`,
  default 5) instead of failing the first time a rate limit is hit.
- **`sessions create --check-conflicts`** — Detect other in-flight sessions
  against the same repo before dispatching, to avoid two sessions racing on
  the same codebase.
- **`monitor`** — Poll one session (`--session-id`) or all of them, printing
  state changes as they happen; add `--reconcile` to flag sessions that have
  gone quiet for more than 30 minutes.
- **`checkpoint`** — Save a session's current state and activity history
  under a local label (`checkpoint save --session-id <id> --label <name>`),
  then `list` or `restore` it later to see what changed since.
- **`archive --stale <duration>`** — Find sessions with no activity for a
  given duration (e.g. `7d`) and report them so you can free up quota; use
  `--dry-run` to preview without changing anything.
- **`diff-validate --session-id <id>`** — Inspect a completed session's
  artifacts for empty diffs, whitespace-only changes, oversized patches, or a
  missing commit message before you act on its output.
- **`trigger add --cron <expr> --workflow <name>`** — Record a cron schedule
  paired with a GitHub workflow name so recurring Jules runs are documented
  in one place (`trigger list`, `trigger pause`).
- **`persona record --session-id <id> --outcome <outcome>`** — Save a
  session's prompt and outcome locally under a name, then `persona show` it
  later to reuse a prompt that worked well.
- **`compliance check --session-id <id>`** — Run a session's changes through
  named policy checks (code review, security scan, license, dependencies,
  secrets) before treating it as ready to merge; `compliance list-policies`
  shows what's available.

All nine are local-first: `monitor`, `archive`, `diff-validate`, and the
`sessions create` flags call the Jules API; `checkpoint`, `persona`, and
`trigger` store their state in this CLI's local SQLite database and never
send it to Jules.

## Usage

Run `jules-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `JULES_CONFIG_DIR`, `JULES_DATA_DIR`, `JULES_STATE_DIR`, or `JULES_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `JULES_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export JULES_HOME=/srv/jules
jules-pp-cli doctor
```

Under `JULES_HOME=/srv/jules`, the four dirs resolve to `/srv/jules/config`, `/srv/jules/data`, `/srv/jules/state`, and `/srv/jules/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "jules": {
      "command": "jules-pp-mcp",
      "env": {
        "JULES_HOME": "/srv/jules"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `JULES_DATA_DIR` overrides an explicit `--home` for that kind. Use `JULES_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `JULES_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `jules-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### sessions

Manage sessions

- **`jules-pp-cli sessions approve-plan`** - Approve pending plan
- **`jules-pp-cli sessions create`** - Create new coding task session
- **`jules-pp-cli sessions delete`** - Delete session
- **`jules-pp-cli sessions get`** - Get specific session
- **`jules-pp-cli sessions list`** - List all sessions
- **`jules-pp-cli sessions send-message`** - Send message to session

### sources

Manage sources

- **`jules-pp-cli sources get`** - Get specific repository
- **`jules-pp-cli sources list`** - List connected repositories

### Session safety and lifecycle (unique to this CLI)

- **`jules-pp-cli sessions create --quota-safe`** - Dispatch with quota checks and exponential backoff on rate limits
- **`jules-pp-cli sessions create --check-conflicts`** - Detect in-flight sessions on the same repo before dispatching
- **`jules-pp-cli monitor`** - Poll session state and flag stalled sessions with `--reconcile`
- **`jules-pp-cli checkpoint`** - Save, list, and restore local snapshots of a session's state
- **`jules-pp-cli archive --stale <duration>`** - Find (and optionally clear) sessions with no recent activity
- **`jules-pp-cli diff-validate --session-id <id>`** - Validate a session's diff before acting on it
- **`jules-pp-cli trigger`** - Record cron + GitHub workflow trigger chains
- **`jules-pp-cli persona`** - Record and reuse successful session prompts locally
- **`jules-pp-cli compliance check --session-id <id>`** - Run governance policy checks against a session

### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`jules-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`jules-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`jules-pp-cli learnings list`** - Inspect taught rows
- **`jules-pp-cli learnings forget <query>`** - Undo a teach
- **`jules-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`jules-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`jules-pp-cli teach-pattern`** - Install a query/resource template up front
- **`jules-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `JULES_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `jules-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
jules-pp-cli sessions list

# JSON for scripting and agents
jules-pp-cli sessions list --json
# Filter to specific fields by name
jules-pp-cli sessions list --json --select <field>[,<field>...]

# Dry run — show the request without sending
jules-pp-cli sessions list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
jules-pp-cli sessions list --agent
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

## Cookbook

See [Unique Features](#unique-features) above for `sessions create --quota-safe`
and `--check-conflicts` — the two safety flags this CLI adds on top of the
plain create endpoint.

```bash
# Dispatch a session
jules-pp-cli sessions create \
  --prompt "fix the auth bug" \
  --source-context '{"repo":"my-org/my-repo"}'

# Preview a session before dispatching it
jules-pp-cli sessions create --prompt "refactor the DB layer" --dry-run

# List sessions, then inspect one
jules-pp-cli sessions list --json
jules-pp-cli sessions get <sessionId>

# Send a follow-up message to a running session
jules-pp-cli sessions send-message <sessionId> --prompt "also update the tests"

# Approve a plan that's waiting on requirePlanApproval
jules-pp-cli sessions approve-plan <sessionId>

# Poll a specific session and flag it if it goes quiet for 30+ minutes
jules-pp-cli monitor --session-id <sessionId> --reconcile --interval 30s

# Save a checkpoint before a risky follow-up message, then compare later
jules-pp-cli checkpoint --session-id <sessionId> --label before-retry save
jules-pp-cli checkpoint --session-id <sessionId> --label before-retry restore

# Find sessions idle for a week (dry run first)
jules-pp-cli archive --stale 7d --dry-run
jules-pp-cli archive --stale 7d --yes

# Validate a completed session's diff before merging its output
jules-pp-cli diff-validate --session-id <sessionId> --strict

# Run compliance checks before treating a session as mergeable
jules-pp-cli compliance check --session-id <sessionId>
jules-pp-cli compliance list-policies

# Record a prompt that worked well, and reuse it later
jules-pp-cli persona record --name refactor-pattern --session-id <sessionId> --outcome success
jules-pp-cli persona --name refactor-pattern --json show

# Sync sessions/sources into the local store, then query them offline
jules-pp-cli sync
jules-pp-cli search "auth bug" --json

# Ask the CLI which command matches a capability you have in mind
jules-pp-cli which "detect stalled sessions"
```

## Health Check

```bash
jules-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `jules-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/jules-pp-cli/config.toml`; `--home`, `JULES_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `JULES_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `jules-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `jules-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $JULES_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

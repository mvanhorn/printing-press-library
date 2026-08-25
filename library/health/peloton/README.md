# Peloton CLI

Read-only Peloton workout, class, and structural-provider facts in a private local store.

Created by [@itsmefelix-](https://github.com/itsmefelix-) (Felix Banuchi).
Contributors: [@jrmii](https://github.com/jrmii) (Jim Martin), [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `peloton-pp-cli` binary and the `pp-peloton` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install peloton
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install peloton --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install peloton --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install peloton --agent claude-code
npx -y @mvanhorn/printing-press-library install peloton --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/peloton/cmd/peloton-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peloton-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install peloton --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-peloton --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-peloton --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install peloton --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

Peloton has no OAuth provisioning service — the bundle just needs your Peloton login. Set it first if you haven't:

```bash
export PELOTON_OAUTH_USERNAME="your-username"
export PELOTON_OAUTH_PASSWORD="your-password"
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peloton-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PELOTON_OAUTH_USERNAME` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/peloton/cmd/peloton-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "peloton": {
      "command": "peloton-pp-mcp",
      "env": {
        "PELOTON_USER_ID": "<user_id>",
        "PELOTON_OAUTH_USERNAME": "<your-peloton-email-or-username>",
        "PELOTON_OAUTH_PASSWORD": "<your-peloton-password>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Your Peloton Login

Peloton has no OAuth provisioning service — this CLI just needs your Peloton login. The first live command logs in automatically and persists the result; later commands reuse or refresh it.

```bash
export PELOTON_OAUTH_USERNAME="your-peloton-email-or-username"
export PELOTON_OAUTH_PASSWORD="your-peloton-password"
```

### 3. Verify Setup

```bash
peloton-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
peloton-pp-cli classes search --browse-category example-value --content-format example-value
```

## Usage

Run `peloton-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml`, saved profiles, and the managed auth bundle (`oauth-token.json`) |
| `data` | Durable local data: `data.db` (the local sync/offline store) and `feedback.jsonl` |
| `state` | Resolved but not currently used by this CLI |
| `cache` | Regenerable HTTP response cache |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PELOTON_CONFIG_DIR`, `PELOTON_DATA_DIR`, `PELOTON_STATE_DIR`, or `PELOTON_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PELOTON_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PELOTON_HOME=/srv/peloton
peloton-pp-cli doctor
```

Under `PELOTON_HOME=/srv/peloton`, the four dirs resolve to `/srv/peloton/config`, `/srv/peloton/data`, `/srv/peloton/state`, and `/srv/peloton/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "peloton": {
      "command": "peloton-pp-mcp",
      "env": {
        "PELOTON_HOME": "/srv/peloton"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PELOTON_DATA_DIR` overrides an explicit `--home` for that kind. Use `PELOTON_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PELOTON_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Peloton's managed auth persists to `oauth-token.json` under the config directory automatically (see [Set Your Peloton Login](#2-set-your-peloton-login) above) — not `credentials.toml` under the data directory; that generic credentials-file mechanism exists in the underlying framework but this CLI's real login flow never writes to it. Run `peloton-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Current account/profile fact; no implicit account expansion.

- **`peloton-pp-cli account`** - Show the current profile fact.

### auth

Manage Peloton credentials. No OAuth provisioning service is involved; this wraps the automatic-login lifecycle (see [Set Your Peloton Login](#2-set-your-peloton-login) above).

- **`peloton-pp-cli auth setup`** - Show how to supply your Peloton login for automatic sign-in.
- **`peloton-pp-cli auth status`** - Show whether a bearer token and/or session cookie are currently available.
- **`peloton-pp-cli auth logout`** - Remove persisted Peloton credentials.

### classes

Read-only catalog, class detail, planned structure, and provider filter vocabulary.

- **`peloton-pp-cli classes catalog`** - List a caller-scoped archived class catalog page.
- **`peloton-pp-cli classes filters`** - Show provider class/filter vocabulary and embedded instructor metadata.
- **`peloton-pp-cli classes search`** - Search the caller-scoped catalog by factual provider filters.
- **`peloton-pp-cli classes show`** - Show class metadata and supported planned structure.
- **`peloton-pp-cli classes structure`** - Inspect ordered provider segments and target ranges without coaching labels.

### doctor

Check configuration, credential, and API-connectivity health.

- **`peloton-pp-cli doctor`** - Report auth state, credential location, API reachability, and local sync cache freshness.

### offline

Inspect locally synced provider facts with no network access.

- **`peloton-pp-cli offline history`** - List locally stored recorded workout facts.
- **`peloton-pp-cli offline workout <workout_id>`** - Show a locally stored workout detail and its recorded history fact.
- **`peloton-pp-cli offline performance <workout_id>`** - Show locally stored recorded performance samples for one workout.
- **`peloton-pp-cli offline intervals <workout_id>`** - Show the stored class segments associated with a recorded workout, when available.
- **`peloton-pp-cli offline classes search`** - Search local class facts by stored fields and structural predicates.
- **`peloton-pp-cli offline classes show <ride_id>`** - Show one locally stored class fact.
- **`peloton-pp-cli offline classes structure <ride_id>`** - Show ordered stored class segments and target fields.
- **`peloton-pp-cli offline classes filters`** - Show locally stored provider filter vocabulary.
- **`peloton-pp-cli offline strength <workout_id>`** - Show stored movement-tracker fields for one workout.
- **`peloton-pp-cli offline repeat <first_workout_id> <second_workout_id>`** - Compare two recorded workouts, only when their stored class identifiers match.

### strength

Provider-supplied performed movement facts present only in workout detail payloads.

- **`peloton-pp-cli strength <workout_id>`** - Inspect provider workout detail containing movement_tracker_data when present; no template fallback.

### sync

Sync API data to local SQLite for offline search and analysis.

- **`peloton-pp-cli sync`** - Sync the default resources (`workouts`, `classes`). Naming `workouts` also cascades into per-workout `performance` samples and `workout_details` payloads (no bulk endpoint exists for those; one request per workout each), which back `offline workout`/`intervals`/`repeat`/`strength`.
- **`peloton-pp-cli sync --resources <list>`** - Sync specific resources: `workouts`, `classes`, `performance`, or `workout_details` (`strength` is accepted as an alias for `workout_details`).
- **`peloton-pp-cli sync --resources performance --full --max-parents <n>`** - Bound and resume a per-workout dependent backfill (performance/workout_details have no bulk endpoint). Default (no `--full`) only fetches workouts missing a record, so repeated calls drain a large backlog for free; `--full` redoes everything (e.g. to backfill a fix) and resumes across calls via a persisted offset; `--max-parents` caps how much happens per call.
- **`peloton-pp-cli sync --resources performance --stale-before <timestamp|duration> --max-parents <n>`** - Targeted alternative to `--full`: refetch only records last fetched before the given cutoff (RFC3339 timestamp or a `--since`-style duration like `7d`), skipping already-correct records instead of walking the whole backlog.

### workouts

Read-only recorded workout history, detail, and recorded performance facts.

- **`peloton-pp-cli workouts list`** - List workout history in newest-first pages; `user_id` must be supplied explicitly (no account-linking shortcut yet).
- **`peloton-pp-cli workouts performance`** - Show recorded performance samples and summaries for one workout.
- **`peloton-pp-cli workouts show`** - Show a recorded workout detail payload.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
peloton-pp-cli classes search --browse-category example-value --content-format example-value

# JSON for scripting and agents
peloton-pp-cli classes search --browse-category example-value --content-format example-value --json

# Filter to specific fields
peloton-pp-cli classes search --browse-category example-value --content-format example-value --json --select id,name,status

# Dry run — show the request without sending
peloton-pp-cli classes search --browse-category example-value --content-format example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
peloton-pp-cli classes search --browse-category example-value --content-format example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `PELOTON_USER_ID` resolves `{user_id}`

Base URL: `https://api.onepeloton.com`

## Health Check

```bash
peloton-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `peloton-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/peloton-pp-cli/config.toml`; `--home`, `PELOTON_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PELOTON_USER_ID` | endpoint | Yes |  |
| `PELOTON_OAUTH_USERNAME` | auth_flow_input | No | Your Peloton login email or username, used to log in automatically. Not needed if a session from a prior login is already persisted. |
| `PELOTON_OAUTH_PASSWORD` | auth_flow_input | No | Your Peloton account password, used to log in automatically. Not needed if a session from a prior login is already persisted. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `peloton-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `peloton-pp-cli doctor` to check credentials — it distinguishes "no credentials anywhere" from "bootstrap env vars unset but a persisted session already works"
- Verify the environment variable is set: `echo $PELOTON_OAUTH_USERNAME`
- If env vars aren't set and `doctor` also reports no persisted session, export `PELOTON_OAUTH_USERNAME`/`PELOTON_OAUTH_PASSWORD` once (see [Set Your Peloton Login](#2-set-your-peloton-login) above) and retry
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

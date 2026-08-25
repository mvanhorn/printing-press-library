# Respond.io CLI

**Every Respond.io feature, plus an offline mirror and SQL that answers inbox questions no single API call can.**

respondio-pp-cli wraps the full Respond.io Unification API v2 - contacts, messaging, conversations, comments, and workspace - then syncs contacts, users, and channels into a local SQLite store. Offline you can segment by tag, find custom-field gaps, check channel mix, and gauge workload by agent with --json output built for agents.

## Install

The recommended path installs both the `respondio-pp-cli` binary and the `pp-respondio` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install respondio
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install respondio --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install respondio --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install respondio --agent claude-code
npx -y @mvanhorn/printing-press-library install respondio --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/cmd/respondio-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/respondio-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install respondio --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-respondio --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-respondio --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install respondio --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/respondio-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RESPOND_IO_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/cmd/respondio-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "respondio": {
      "command": "respondio-pp-mcp",
      "env": {
        "RESPONDIO_IDENTIFIER": "<identifier>",
        "RESPOND_IO_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Auth uses a Respond.io API access token (Settings > Integrations > Developer API). Set RESPOND_IO_API_TOKEN and run respondio-pp-cli auth login. All commands are read-mostly; mutations support --dry-run.

## Quick Start

```bash
# Check token, config, and API reachability (works without live auth).
respondio-pp-cli doctor --dry-run

# Look up a contact by identifier (id:, email:, phone:).
respondio-pp-cli contact get id:123

# Mirror contacts and workspace into the local database.
respondio-pp-cli sync --resources contact,space_user,space_channel

# Search contacts offline in the local mirror.
respondio-pp-cli contact search --query acme

# Pull a tag cohort from local data.
respondio-pp-cli contact by-tag VIP

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`overview`** — One-command summary of open conversations, unassigned contacts, per-agent distribution, and recent activity.

  _Use instead of a dozen API calls when you need the current inbox load at a glance._

  ```bash
  respondio-pp-cli overview --json
  ```
- **`report workload`** — Per-agent message and conversation handling volume from the synced workspace.

  _Balance the team and spot overloaded agents without manual tallying._

  ```bash
  respondio-pp-cli report workload --json
  ```

### Segment & find
- **`report channel-mix`** — See which messaging channels (WhatsApp, Instagram, email...) your contacts actually use.

  _Answer 'where do our customers talk to us' instantly from local data._

  ```bash
  respondio-pp-cli report channel-mix --json
  ```
- **`contact by-tag`** — List every contact that carries a given tag (VIP, unpaid, in-trial).

  _Pull a whole segment for campaigns or support outreach in one command._

  ```bash
  respondio-pp-cli contact by-tag VIP --json
  ```
- **`contact field-gaps`** — Find contacts missing a custom field value (e.g. no orderId or region).

  _Spot data quality gaps to drive clean-up or re-enrichment._

  ```bash
  respondio-pp-cli contact field-gaps --name orderId --json
  ```
- **`contact idle`** — Find unassigned contacts with no recent activity worth working.

  _Route idle work before it goes cold._

  ```bash
  respondio-pp-cli contact idle --days 7 --json
  ```

### Agent-native plumbing
- **`contact search`** — Free-text search across synced contacts without hitting the API.

  _Find a customer by name/email offline, fast, without api.respond.io._

  ```bash
  respondio-pp-cli contact search --query acme --json
  ```

## Recipes

### Inbox load at a glance

```bash
respondio-pp-cli overview --json
```

Summarize open conversations, unassigned contacts, and activity.

### Narrow a contact payload

```bash
respondio-pp-cli contact get id:123 --agent --select firstName,email,lifecycle
```

Keep context small by selecting only the fields you need.

### Find data-quality gaps

```bash
respondio-pp-cli contact field-gaps --name orderId --json
```

List contacts missing a custom field to drive enrichment.

### Where customers talk to you

```bash
respondio-pp-cli report channel-mix --json
```

Show the channel-source distribution across synced contacts.

### Team workload

```bash
respondio-pp-cli report workload --json
```

Per-agent handling volume from synced assignments.

## Usage

Run `respondio-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `RESPONDIO_CONFIG_DIR`, `RESPONDIO_DATA_DIR`, `RESPONDIO_STATE_DIR`, or `RESPONDIO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `RESPONDIO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export RESPONDIO_HOME=/srv/respondio
respondio-pp-cli doctor
```

Under `RESPONDIO_HOME=/srv/respondio`, the four dirs resolve to `/srv/respondio/config`, `/srv/respondio/data`, `/srv/respondio/state`, and `/srv/respondio/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "respondio": {
      "command": "respondio-pp-mcp",
      "env": {
        "RESPONDIO_HOME": "/srv/respondio"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `RESPONDIO_DATA_DIR` overrides an explicit `--home` for that kind. Use `RESPONDIO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `RESPONDIO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `respondio-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### comment

Add internal comments to contacts

- **`respondio-pp-cli comment <identifier>`** - Create an internal comment on a contact

### contact

Manage Respond.io contacts and contact-level actions

- **`respondio-pp-cli contact add-tags`** - Add tags to a contact
- **`respondio-pp-cli contact create`** - Create a new contact (identifier must be email: or phone:)
- **`respondio-pp-cli contact delete`** - Delete a contact
- **`respondio-pp-cli contact get`** - Get a contact by identifier (id:123, email:user@example.com, phone:+1234567890)
- **`respondio-pp-cli contact list`** - List contacts with optional search and filters
- **`respondio-pp-cli contact list-channels`** - List all channels connected to a contact
- **`respondio-pp-cli contact merge`** - Merge two contacts into one
- **`respondio-pp-cli contact remove-tags`** - Remove tags from a contact
- **`respondio-pp-cli contact update`** - Update an existing contact
- **`respondio-pp-cli contact update-lifecycle`** - Update a contact's lifecycle stage
- **`respondio-pp-cli contact upsert`** - Create or update a contact keyed by email/phone identifier

### conversation

Manage conversation assignment and status

- **`respondio-pp-cli conversation assign`** - Assign or unassign a conversation
- **`respondio-pp-cli conversation update-status`** - Open or close a conversation

### message

Send and retrieve messages for a contact

- **`respondio-pp-cli message get`** - Get a message by ID
- **`respondio-pp-cli message list`** - List messages for a contact
- **`respondio-pp-cli message send`** - Send a message to a contact

### space

Respond.io workspace-level operations

- **`respondio-pp-cli space create-custom-field`** - Create a custom field definition
- **`respondio-pp-cli space create-tag`** - Create a workspace tag
- **`respondio-pp-cli space delete-tag`** - Delete a workspace tag
- **`respondio-pp-cli space get-custom-field`** - Get a custom field by ID
- **`respondio-pp-cli space get-user`** - Get a user by ID
- **`respondio-pp-cli space list-channels`** - List all channels in the workspace
- **`respondio-pp-cli space list-closing-notes`** - List closing note categories
- **`respondio-pp-cli space list-custom-fields`** - List all custom field definitions
- **`respondio-pp-cli space list-templates`** - List message templates for a channel
- **`respondio-pp-cli space list-users`** - List users in the workspace
- **`respondio-pp-cli space update-tag`** - Update a workspace tag


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`respondio-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`respondio-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`respondio-pp-cli learnings list`** - Inspect taught rows
- **`respondio-pp-cli learnings forget <query>`** - Undo a teach
- **`respondio-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`respondio-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`respondio-pp-cli teach-pattern`** - Install a query/resource template up front
- **`respondio-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `RESPONDIO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `respondio-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
respondio-pp-cli contact list

# JSON for scripting and agents
respondio-pp-cli contact list --json

# Filter to specific fields
respondio-pp-cli contact list --json --select id,name,status

# Dry run — show the request without sending
respondio-pp-cli contact list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
respondio-pp-cli contact list --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `RESPONDIO_IDENTIFIER` resolves `{identifier}`

Base URL: `https://api.respond.io/v2`

## Health Check

```bash
respondio-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `respondio-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/respondio-pp-cli/config.toml`; `--home`, `RESPONDIO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RESPONDIO_IDENTIFIER` | endpoint | Yes |  |
| `RESPOND_IO_API_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `respondio-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `respondio-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RESPOND_IO_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 401 / 403 on commands** — Confirm RESPOND_IO_API_TOKEN is set and valid via 'respondio-pp-cli doctor'.
- **Rate limited (429) on wide syncs** — The CLI respects X-RateLimit headers and retries with backoff; narrow with --resources and --limit.
- **Empty local results** — Run 'respondio-pp-cli sync' first - novel reports read the local mirror, not the live API.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**respond-io/typescript-sdk**](https://github.com/respond-io/typescript-sdk) — TypeScript (3 stars)
- [**repat/respond-io-client**](https://github.com/repat/respond-io-client) — PHP
- [**D1DX/respond-skill**](https://github.com/D1DX/respond-skill) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

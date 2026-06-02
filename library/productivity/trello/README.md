# Trello CLI

**Every Trello feature, plus a local SQLite mirror, offline search, and cross-board analytics no other Trello tool has.**

trello-pp-cli mirrors your boards, lists, cards, checklists, and activity into local SQLite, so you can grep, SQL-query, and run cross-board analytics offline. On top of full CRUD parity with every existing Trello CLI and MCP, it adds overdue sweeps, member workload balance, cycle time, bottleneck detection, and velocity trends that require a local join no single API call provides.

Learn more at [Trello](https://trello.com/home).

Created by [@techwithtam](https://github.com/techwithtam) (Tam Nguyen).

## Install

The recommended path installs both the `trello-pp-cli` binary and the `pp-trello` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install trello
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install trello --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install trello --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install trello --agent claude-code
npx -y @mvanhorn/printing-press-library install trello --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/trello/cmd/trello-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trello-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-trello --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-trello --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-trello skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-trello. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trello-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in both `TRELLO_API_KEY` and `TRELLO_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/trello/cmd/trello-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "trello": {
      "command": "trello-pp-mcp",
      "env": {
        "TRELLO_API_KEY": "<your-api-key>",
        "TRELLO_API_TOKEN": "<your-user-token>"
      }
    }
  }
}
```

</details>

## Authentication

Trello auth is a key plus token pair passed as query params. Get both at https://trello.com/app-key. Set TRELLO_API_KEY and TRELLO_API_TOKEN, then run 'trello-pp-cli doctor' to verify.

## Quick Start

```bash
# verify the binary and config without needing auth
trello-pp-cli doctor --dry-run

# mirror your boards/lists/cards into local SQLite
trello-pp-cli trello-sync

# see every overdue card across all boards at once
trello-pp-cli overdue --agent

# offline full-text search across all synced cards
trello-pp-cli search "deploy" --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-board analytics
- **`overdue`** — Every past-due card across all your boards, ranked by lateness and owner.

  _Reach for this when you need the full blast radius of slipped work across everything, not one board's slice._

  ```bash
  trello-pp-cli overdue --agent
  ```
- **`workload`** — Open and due-soon card load per member across every board.

  _Reach for this before assigning new work, when 'who has capacity' can't come from one board._

  ```bash
  trello-pp-cli workload --window 7d --agent
  ```
- **`velocity`** — Cards completed per week over the last N weeks, per board or member, with trend.

  _Reach for this to sanity-check a deadline with real historical throughput._

  ```bash
  trello-pp-cli velocity --weeks 8 --agent
  ```

### Flow diagnostics
- **`cycletime`** — How long cards take from started to done, with median and p90 per list or label.

  _Reach for this to quantify where work stalls and set realistic SLAs._

  ```bash
  trello-pp-cli cycletime --board Eng --agent
  ```
- **`bottleneck`** — Which list is clogged now, by card count and how long cards have aged in place.

  _Reach for this in standup to name the exact stage starving throughput._

  ```bash
  trello-pp-cli bottleneck --board Eng --agent
  ```
- **`churn`** — Cards that bounce backward between lists, revealing rework and unstable requirements.

  _Reach for this in a retro to find where work keeps getting kicked back._

  ```bash
  trello-pp-cli churn --weeks 4 --agent
  ```

### Hidden-state analytics
- **`checklist-progress`** — Real card-level progress from checkitem completion across boards.

  _Reach for this to catch cards nearing a deadline with unchecked subtasks._

  ```bash
  trello-pp-cli checklist-progress --below 80 --agent
  ```
- **`blocked`** — Cards flagged blocked by label, checkitem text, or comment, and how long they've sat.

  _Reach for this to assemble an unblock list scattered across labels, checklists, and comments._

  ```bash
  trello-pp-cli blocked --over 3d --agent
  ```

## Recipes


### Morning triage

```bash
trello-pp-cli overdue --agent --select cards.name,cards.due,cards.board
```

Narrow the overdue sweep to just name, due date, and board for a quick agent-readable list.

### Sprint capacity check

```bash
trello-pp-cli workload --window 7d --agent
```

See per-member load before assigning new cards.

### Find the bottleneck

```bash
trello-pp-cli bottleneck --board Eng --agent
```

Name the clogged stage in one shot during standup.

## Usage

Run `trello-pp-cli --help` for the full command reference and flag list.

## Commands

### actions

https://trello.com/docs/api/action/index.html

- **`trello-pp-cli actions delete-by-id`** - Delete actions by id action()
- **`trello-pp-cli actions get-by-id`** - Get actions by id action()
- **`trello-pp-cli actions get-by-id-by-field`** - Get actions by id action by field()
- **`trello-pp-cli actions update-by-id`** - Update actions by id action()

### batch

https://trello.com/docs/api/batch/index.html

- **`trello-pp-cli batch`** - Get batch()

### boards

https://trello.com/docs/api/board/index.html

- **`trello-pp-cli boards add`** - Add boards()
- **`trello-pp-cli boards get-by-id`** - Get boards by id board()
- **`trello-pp-cli boards get-by-id-by-field`** - Get boards by id board by field()
- **`trello-pp-cli boards update-by-id`** - Update boards by id board()

### cards

https://trello.com/docs/api/card/index.html

- **`trello-pp-cli cards add`** - Add cards()
- **`trello-pp-cli cards delete-by-id`** - Delete cards by id card()
- **`trello-pp-cli cards get-by-id`** - Get cards by id card()
- **`trello-pp-cli cards get-by-id-by-field`** - Get cards by id card by field()
- **`trello-pp-cli cards update-by-id`** - Update cards by id card()

### checklists

https://trello.com/docs/api/checklist/index.html

- **`trello-pp-cli checklists add`** - Add checklists()
- **`trello-pp-cli checklists delete-by-id`** - Delete checklists by id checklist()
- **`trello-pp-cli checklists get-by-id`** - Get checklists by id checklist()
- **`trello-pp-cli checklists get-by-id-by-field`** - Get checklists by id checklist by field()
- **`trello-pp-cli checklists update-by-id`** - Update checklists by id checklist()

### labels

https://trello.com/docs/api/label/index.html

- **`trello-pp-cli labels add`** - Add labels()
- **`trello-pp-cli labels delete-by-id`** - Delete labels by id label()
- **`trello-pp-cli labels get-by-id`** - Get labels by id label()
- **`trello-pp-cli labels update-by-id`** - Update labels by id label()

### lists

https://trello.com/docs/api/list/index.html

- **`trello-pp-cli lists add`** - Add lists()
- **`trello-pp-cli lists get-by-id`** - Get lists by id list()
- **`trello-pp-cli lists get-by-id-by-field`** - Get lists by id list by field()
- **`trello-pp-cli lists update-by-id`** - Update lists by id list()

### members

https://trello.com/docs/api/member/index.html

- **`trello-pp-cli members get-by-id`** - If you specify 'me' as the username, this call will respond as if you had supplied the username associated with the supplied token
- **`trello-pp-cli members get-by-id-by-field`** - Get members by id member by field()
- **`trello-pp-cli members update-by-id`** - Update members by id member()

### notifications

https://trello.com/docs/api/notification/index.html

- **`trello-pp-cli notifications add-all-read`** - Add notifications all read()
- **`trello-pp-cli notifications get-by-id`** - Get notifications by id notification()
- **`trello-pp-cli notifications get-by-id-by-field`** - Get notifications by id notification by field()
- **`trello-pp-cli notifications update-by-id`** - Update notifications by id notification()

### organizations

https://trello.com/docs/api/organization/index.html

- **`trello-pp-cli organizations add`** - Add organizations()
- **`trello-pp-cli organizations delete-by-id-org`** - Delete organizations by id org()
- **`trello-pp-cli organizations get-by-id-org`** - Get organizations by id org()
- **`trello-pp-cli organizations get-by-id-org-by-field`** - Get organizations by id org by field()
- **`trello-pp-cli organizations update-by-id-org`** - Update organizations by id org()

### search_resource

Manage search resource

- **`trello-pp-cli search-resource get-search`** - Get search()
- **`trello-pp-cli search-resource get-search-members`** - Get search members()

### sessions

https://trello.com/docs/api/session/index.html

- **`trello-pp-cli sessions add`** - Add sessions()
- **`trello-pp-cli sessions get-socket`** - This is the route for WebSocket requests.  See the socket API reference for a description of WebSocket usage.
- **`trello-pp-cli sessions update-by-id`** - Update sessions by id session()

### tokens

https://trello.com/docs/api/token/index.html

- **`trello-pp-cli tokens delete-by`** - Delete tokens by token()
- **`trello-pp-cli tokens get-by`** - Get tokens by token()
- **`trello-pp-cli tokens get-by-by-field`** - Get tokens by token by field()

### types_resource

Manage types resource

- **`trello-pp-cli types-resource <id>`** - Get types by id()

### webhooks

https://trello.com/docs/api/webhook/index.html

- **`trello-pp-cli webhooks add`** - Add webhooks()
- **`trello-pp-cli webhooks delete-by-id`** - Delete webhooks by id webhook()
- **`trello-pp-cli webhooks get-by-id`** - Get webhooks by id webhook()
- **`trello-pp-cli webhooks get-by-id-by-field`** - Get webhooks by id webhook by field()
- **`trello-pp-cli webhooks update`** - Update webhooks()
- **`trello-pp-cli webhooks update-by-id`** - Update webhooks by id webhook()


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
trello-pp-cli batch --urls https://example.com/resource --key your-token-here --token your-token-here

# JSON for scripting and agents
trello-pp-cli batch --urls https://example.com/resource --key your-token-here --token your-token-here --json

# Filter to specific fields
trello-pp-cli batch --urls https://example.com/resource --key your-token-here --token your-token-here --json --select id,name,status

# Dry run — show the request without sending
trello-pp-cli batch --urls https://example.com/resource --key your-token-here --token your-token-here --dry-run

# Agent mode — JSON + compact + no prompts in one flag
trello-pp-cli batch --urls https://example.com/resource --key your-token-here --token your-token-here --agent
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
trello-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/trello-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TRELLO_API_KEY` | per_call | Yes | Trello developer API key from https://trello.com/app-key. |
| `TRELLO_API_TOKEN` | per_call | Yes | Trello user token authorized for the boards you want to access. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `trello-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `trello-pp-cli doctor` to check credentials
- Verify both environment variables are set without printing them: `test -n "$TRELLO_API_KEY" && test -n "$TRELLO_API_TOKEN" && echo ok`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 invalid token** — Regenerate your token at https://trello.com/app-key and re-export TRELLO_API_TOKEN.
- **empty results after hydration** — Run `trello-pp-cli trello-sync` to mirror boards, lists, cards, members, checklists, and recent actions.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**EndlessHoper/trello-mcp**](https://github.com/EndlessHoper/trello-mcp) — TypeScript
- [**delorenj/mcp-server-trello**](https://github.com/delorenj/mcp-server-trello) — TypeScript
- [**mheap/trello-cli**](https://github.com/mheap/trello-cli) — JavaScript
- [**sarumont/py-trello**](https://github.com/sarumont/py-trello) — Python
- [**kocakli/Trello-Desktop-MCP**](https://github.com/kocakli/trello-desktop-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

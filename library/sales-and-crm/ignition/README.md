# Ignition CLI

**A read-only, agent-native CLI for Ignition proposals, invoices, billing, clients, and payments, with a local SQLite mirror and invoicing analytics the web UI does not offer.**

Ignition (ignitionapp.com) has no public API, so agents drive it through fragile browser automation. This CLI gives the firm's agents a stable read interface over the GraphQL BFF plus local-join analytics (outstanding, pipeline, unbilled, client-billing) that answer invoicing questions in one command. It is read and draft only by design: nothing here sends email, activates a proposal, creates or charges an invoice, or mutates client state.

Learn more at [Ignition](https://go.ignitionapp.com).

Created by [@corben-tech](https://github.com/corben-tech).

## Install

The recommended path installs both the `ignition-pp-cli` binary and the `pp-ignition` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ignition
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ignition --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ignition --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ignition --agent claude-code
npx -y @mvanhorn/printing-press-library install ignition --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/ignition/cmd/ignition-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ignition-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ignition --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ignition --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ignition --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ignition --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
ignition-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ignition-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/ignition/cmd/ignition-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ignition": {
      "command": "ignition-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Ignition auth is a browser session cookie plus a per-page X-CSRF-Token (from the page's csrf-token meta tag). Set IGNITION_SESSION_COOKIE and IGNITION_CSRF_TOKEN from an authenticated Ignition tab; the CSRF token is minted per page-load, so refresh it if reads start returning auth errors.

## Quick Start

```bash
# confirm the binary and config resolve before hitting the API
ignition-pp-cli doctor --dry-run

# list invoices with only the fields that matter
ignition-pp-cli search-index invoices --json --select results.nodes.number,results.nodes.status,results.nodes.total

# see what is billed but unpaid
ignition-pp-cli outstanding --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Invoicing analytics
- **`outstanding`** — See everything billed but not yet paid, summed per client.

  _Reach for this to answer 'who owes us money' in one read instead of paging the invoice index by hand._

  ```bash
  ignition-pp-cli outstanding --json
  ```
- **`pipeline`** — Count proposals across DRAFT, AWAITING_ACCEPTANCE, ACCEPTED, and LOST for the whole book of business.

  _Pick this over a raw proposal list when you want the funnel shape, not individual rows._

  ```bash
  ignition-pp-cli pipeline --json
  ```
- **`client-billing`** — One client's full money picture: what was proposed, invoiced, and still outstanding.

  _Use before a client call to see their entire Ignition money state in one shot._

  ```bash
  ignition-pp-cli client-billing --client-id cli_example --json
  ```
- **`unbilled`** — Surface accepted proposals that do not yet have a matching invoice or billing item.

  _This is the direct 'invoice more efficiently' lever: it finds work you agreed to but haven't billed._

  ```bash
  ignition-pp-cli unbilled --json
  ```
- **`rejected-payments`** — List clients with a rejected or failed payment so collections can follow up.

  _Read this to build a follow-up list without opening each client in the Ignition UI._

  ```bash
  ignition-pp-cli rejected-payments --json
  ```

## Recipes

### Unbilled accepted work

```bash
ignition-pp-cli unbilled --json
```

accepted proposals with no matching invoice yet — the invoicing to-do list

### Trim a deep proposal payload

```bash
ignition-pp-cli search-index proposals --json --select results.nodes.name,results.nodes.status,results.nodes.client.name
```

narrow the nested GraphQL response to the fields agents actually read

### One client's money picture

```bash
ignition-pp-cli client-billing --client-id cli_example --json
```

proposed, invoiced, and outstanding for a single client before a call

## Usage

Run `ignition-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IGNITION_CONFIG_DIR`, `IGNITION_DATA_DIR`, `IGNITION_STATE_DIR`, or `IGNITION_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IGNITION_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IGNITION_HOME=/srv/ignition
ignition-pp-cli doctor
```

Under `IGNITION_HOME=/srv/ignition`, the four dirs resolve to `/srv/ignition/config`, `/srv/ignition/data`, `/srv/ignition/state`, and `/srv/ignition/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ignition": {
      "command": "ignition-pp-mcp",
      "env": {
        "IGNITION_HOME": "/srv/ignition"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `IGNITION_DATA_DIR` overrides an explicit `--home` for that kind. Use `IGNITION_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IGNITION_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `ignition-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### acknowledgements

GraphQL BFF operations for acknowledgements

- **`ignition-pp-cli acknowledgements`** - Fetch acknowledgements

### adds

GraphQL BFF operations for adds

- **`ignition-pp-cli adds`** - Fetch adds on plan trial banner

### apps

GraphQL BFF operations for apps

- **`ignition-pp-cli apps`** - Fetch apps with capability

### auth-api

Operations on graphql

- **`ignition-pp-cli auth-api`** - POST /auth-api/graphql

### billings

GraphQL BFF operations for billings

- **`ignition-pp-cli billings`** - Fetch billings

### brandings

GraphQL BFF operations for brandings

- **`ignition-pp-cli brandings`** - Fetch brandings theme

### clients

GraphQL BFF operations for clients

- **`ignition-pp-cli clients forms`** - Fetch clients forms
- **`ignition-pp-cli clients get`** - Fetch clients
- **`ignition-pp-cli clients get-2`** - Fetch clients
- **`ignition-pp-cli clients proposals`** - Fetch clients proposals
- **`ignition-pp-cli clients summary-client`** - Fetch clients summary client
- **`ignition-pp-cli clients summary-rejected-payments`** - Fetch clients summary rejected payments
- **`ignition-pp-cli clients sync-status`** - Fetch clients sync status
- **`ignition-pp-cli clients tags`** - Fetch clients tags

### codes

GraphQL BFF operations for codes

- **`ignition-pp-cli codes`** - Fetch codes version

### compliances

GraphQL BFF operations for compliances

- **`ignition-pp-cli compliances`** - Fetch compliances availability

### currents

GraphQL BFF operations for currents

- **`ignition-pp-cli currents practice`** - Fetch currents practice
- **`ignition-pp-cli currents user`** - Fetch currents user

### features

GraphQL BFF operations for features

- **`ignition-pp-cli features`** - Fetch features gate

### forms

GraphQL BFF operations for forms

- **`ignition-pp-cli forms`** - Fetch forms templates

### payments

GraphQL BFF operations for payments

- **`ignition-pp-cli payments`** - Fetch payments settings

### preferreds

GraphQL BFF operations for preferreds

- **`ignition-pp-cli preferreds`** - Fetch preferreds proposal editor

### proposals

GraphQL BFF operations for proposals

- **`ignition-pp-cli proposals`** - Fetch proposals

### search_index

Verified paged search over the Ignition search index (proposals, invoices, billing items). Query shape verified live in ignition_gql.py + the browser-harness ignitionapp domain skill. Records live under results.nodes.

- **`ignition-pp-cli search-index billing-items`** - List all billing items via the search index. Records under results.nodes.
- **`ignition-pp-cli search-index invoices`** - List all invoices via the search index. Records under results.nodes.
- **`ignition-pp-cli search-index proposals`** - List all proposals via the search index (status, client, name). Filter by client-side node.client.id; there is no server-side client-id text filter.

### site

GraphQL BFF operations for site

- **`ignition-pp-cli site`** - Fetch site navigation

### solos

GraphQL BFF operations for solos

- **`ignition-pp-cli solos`** - Fetch solos plan banner

### unseens

GraphQL BFF operations for unseens

- **`ignition-pp-cli unseens`** - Fetch unseens count


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`ignition-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`ignition-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`ignition-pp-cli learnings list`** - Inspect taught rows
- **`ignition-pp-cli learnings forget <query>`** - Undo a teach
- **`ignition-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`ignition-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`ignition-pp-cli teach-pattern`** - Install a query/resource template up front
- **`ignition-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `IGNITION_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `ignition-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ignition-pp-cli acknowledgements --operation-name example-resource

# JSON for scripting and agents
ignition-pp-cli acknowledgements --operation-name example-resource --json

# Filter to specific fields
ignition-pp-cli acknowledgements --operation-name example-resource --json --select id,name,status

# Dry run — show the request without sending
ignition-pp-cli acknowledgements --operation-name example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ignition-pp-cli acknowledgements --operation-name example-resource --agent
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
ignition-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `ignition-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/ignition-pp-cli/config.toml`; `--home`, `IGNITION_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `IGNITION_SESSION_COOKIE` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `ignition-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ignition-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $IGNITION_SESSION_COOKIE`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **reads return an auth or 'User does not exist in the current context' error** — refresh IGNITION_CSRF_TOKEN from the csrf-token meta tag of a logged-in Ignition tab; the token is per-page and expires
- **empty results but no error** — run ignition-pp-cli sync first so the local mirror is populated, then query

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://go.ignitionapp.com/graphql
- Capture coverage: 262 API entries from 262 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: graphql (92% confidence)
- Candidate command ideas: create_graphql — Derived from observed POST /auth-api/graphql traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

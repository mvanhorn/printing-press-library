# Pin Generator CLI

**Every Pin Generator MCP tool as a typed command, with credit-safe dry runs, preflight pricing and a local spend ledger.**

Drive Pin Generator from scripts and agents: discover profiles and boards, turn posts or store products into pins with pin-post and pin-products, and check plans with preflight before spending. Anything that costs credits previews for free until you pass --execute.

Created by [@ElizabethcStone](https://github.com/ElizabethcStone) (Elizabeth Stone).

## Install

The recommended path installs both the `pingenerator-pp-cli` binary and the `pp-pingenerator` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pingenerator
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pingenerator --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pingenerator --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pingenerator --agent claude-code
npx -y @mvanhorn/printing-press-library install pingenerator --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/pingenerator/cmd/pingenerator-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pingenerator-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install pingenerator --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pingenerator --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pingenerator --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install pingenerator --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pingenerator-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PINGENERATOR_ACCESS_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/pingenerator/cmd/pingenerator-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "pingenerator": {
      "command": "pingenerator-pp-mcp",
      "env": {
        "PINGENERATOR_ACCESS_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

### Remote MCP over HTTP

`pingenerator-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `PINGENERATOR_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `PINGENERATOR_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export PINGENERATOR_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
pingenerator-pp-mcp --transport http
```

Set `PINGENERATOR_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

Pin Generator's account access key (pingenerator.com/profile, API access key) authorizes the MCP endpoint as a Bearer token. Set PINGENERATOR_ACCESS_KEY. The web app's richer REST API needs a Firebase login session and is not used.

## Quick Start

```bash
# Confirms the CLI runs offline; then run get-account-status to check your key
pingenerator-pp-cli doctor --dry-run


# Credits remaining and connected profiles
pingenerator-pp-cli get-account-status --agent


# Board IDs you will schedule into
pingenerator-pp-cli list-boards --pinterest-profile myprofile --agent --select boards.id,boards.name


# Posts that still need a pin
pingenerator-pp-cli unpinned-posts --website-url https://example.com --agent


# Free preview; add --execute to spend 1 credit and schedule
pingenerator-pp-cli pin-post https://example.com/best-beaches --pinterest-profile myprofile --board-id 992269799088633298

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Credit-safe pin pipelines

- **`pin-post`** — Turn a blog post URL into a scheduled Pinterest pin in one command, previewed for free before any credit is spent.

  _Reach for this when an agent must pin a specific post; it previews cost and skips posts this CLI already pinned (spend ledger). Use unpinned-posts to check the account's queue first._

  ```bash
  pingenerator-pp-cli pin-post https://example.com/best-beaches --pinterest-profile myprofile --board-id 992269799088633298 --agent
  ```
- **`preflight`** — Price a planned batch in credits and confirm the profile, board and design profile exist before spending anything.

  _Run this before any --execute batch so a bad board ID or low balance fails free._

  ```bash
  pingenerator-pp-cli preflight --pinterest-profile myprofile --board-id 992269799088633298 --tool generate-and-schedule-ai-pin --count 5 --agent
  ```
- **`pin-products`** — Turn a connected store's products into a batch of scheduled pins, skipping products this CLI already pinned (spend ledger).

  _Use this for e-commerce pinning in bulk; the dry run shows the total credit cost first._

  ```bash
  pingenerator-pp-cli pin-products --store-id store_123 --search linen --pinterest-profile myprofile --board-id 992269799088633298 --agent
  ```

### Queue and coverage insight

- **`unpinned-posts`** — List sitemap posts that have no scheduled pin and no ledgered spend.

  _Start a blog-to-pins run here so only new posts get pinned._

  ```bash
  pingenerator-pp-cli unpinned-posts --website-url https://example.com --limit 50 --agent
  ```
- **`ledger`** — See every credit-spending call this CLI executed, including unconfirmed ones that may have spent, with totals by tool and time window.

  _Check this after an agent run, or before a retry; an unconfirmed entry means check list-scheduled-pins before trying again._

  ```bash
  pingenerator-pp-cli ledger --since 7d --agent
  ```
- **`queue-gaps`** — Find empty and overloaded days in the scheduled-pin queue, per board.

  _Use this to keep a steady posting cadence across client accounts._

  ```bash
  pingenerator-pp-cli queue-gaps --days 14 --per-day 5 --agent
  ```
- **`credit-runway`** — Estimate weekly credit burn from active automations and how many days of credit remain.

  _Run weekly for accounts with automations so batches do not fail on an empty balance._

  ```bash
  pingenerator-pp-cli credit-runway --agent
  ```

## Recipes


### Pin every new post

```bash
pingenerator-pp-cli unpinned-posts --website-url https://example.com --agent --select unpinned.url,unpinned.title
```

List posts with no scheduled, queued or ledgered pin, then pass each URL to pin-post.

### Price before you spend

```bash
pingenerator-pp-cli preflight --pinterest-profile myprofile --board-id 992269799088633298 --tool generate-and-schedule-ai-pin --count 5
```

Shows the credit range and checks the board exists, spending nothing.

### Trim template output

```bash
pingenerator-pp-cli list-templates --agent --select templates.name,templates.style
```

Keeps only template names and styles from a large response.

### Audit agent spending

```bash
pingenerator-pp-cli ledger --since 7d --agent
```

Every executed spend this week with credit totals; unconfirmed rows may have spent and should be checked in list-scheduled-pins before a retry.

## Usage

Run `pingenerator-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PINGENERATOR_CONFIG_DIR`, `PINGENERATOR_DATA_DIR`, `PINGENERATOR_STATE_DIR`, or `PINGENERATOR_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PINGENERATOR_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PINGENERATOR_HOME=/srv/pingenerator
pingenerator-pp-cli doctor
```

Under `PINGENERATOR_HOME=/srv/pingenerator`, the four dirs resolve to `/srv/pingenerator/config`, `/srv/pingenerator/data`, `/srv/pingenerator/state`, and `/srv/pingenerator/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:PINGENERATOR_HOME = "C:\srv\pingenerator"
pingenerator-pp-cli doctor
```

Under `$env:PINGENERATOR_HOME = "C:\srv\pingenerator"`, the four dirs resolve to `C:\srv\pingenerator\config`, `C:\srv\pingenerator\data`, `C:\srv\pingenerator\state`, and `C:\srv\pingenerator\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "pingenerator": {
      "command": "pingenerator-pp-mcp",
      "env": {
        "PINGENERATOR_HOME": "/srv/pingenerator"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PINGENERATOR_DATA_DIR` overrides an explicit `--home` for that kind. Use `PINGENERATOR_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PINGENERATOR_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `pingenerator-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `pingenerator-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `pingenerator-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `PINGENERATOR_NO_LOCAL_DATA=1`. Reads go to the API, the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.

## Commands

### mcp

Raw Pin Generator MCP JSON-RPC access (escape hatch for tools not yet typed)

- **`pingenerator-pp-cli mcp`** - Send a raw JSON-RPC request to the Pin Generator MCP endpoint. tools/call can spend credits; prefer the typed commands, which dry-run by default.


### credit-runway


- **`pingenerator-pp-cli credit-runway`** - Estimate weekly credit burn from active automations and how many days of credit remain.


### ledger


- **`pingenerator-pp-cli ledger`** - See every credit-spending call this CLI executed, including unconfirmed ones that may have spent, with totals by tool and time window.


### pin-post


- **`pingenerator-pp-cli pin-post`** - Turn a blog post URL into a scheduled Pinterest pin in one command, previewed for free before any credit is spent.


### pin-products


- **`pingenerator-pp-cli pin-products`** - Turn a connected store's products into a batch of scheduled pins, skipping products this CLI already pinned (spend ledger).


### preflight


- **`pingenerator-pp-cli preflight`** - Price a planned batch in credits and confirm the profile, board and design profile exist before spending anything.


### queue-gaps


- **`pingenerator-pp-cli queue-gaps`** - Find empty and overloaded days in the scheduled-pin queue, per board.


### unpinned-posts


- **`pingenerator-pp-cli unpinned-posts`** - List sitemap posts that have no scheduled pin and no ledgered spend.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`pingenerator-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`pingenerator-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`pingenerator-pp-cli learnings list`** - Inspect taught rows
- **`pingenerator-pp-cli learnings forget <query>`** - Undo a teach
- **`pingenerator-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`pingenerator-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`pingenerator-pp-cli teach-pattern`** - Install a query/resource template up front
- **`pingenerator-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PINGENERATOR_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `pingenerator-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pingenerator-pp-cli mcp --jsonrpc 2.0

# JSON for scripting and agents
pingenerator-pp-cli mcp --jsonrpc 2.0 --json
# Filter to specific fields by name
pingenerator-pp-cli mcp --jsonrpc 2.0 --json --select <field>[,<field>...]

# Dry run — show the request without sending
pingenerator-pp-cli mcp --jsonrpc 2.0 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pingenerator-pp-cli mcp --jsonrpc 2.0 --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
pingenerator-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `pingenerator-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/pingenerator-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\pingenerator-pp-cli\config.toml` on Windows; `--home`, `PINGENERATOR_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `PINGENERATOR_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `PINGENERATOR_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PINGENERATOR_ACCESS_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `pingenerator-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pingenerator-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PINGENERATOR_ACCESS_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Exit code 4: no access key, or user not found** — Set PINGENERATOR_ACCESS_KEY to the API access key from pingenerator.com/profile, or run pingenerator-pp-cli auth set-token
- **Exit code 5 with 'no credits remaining'** — Run pingenerator-pp-cli credit-runway, then top up credits in the Pin Generator app
- **Exit code 7: rate limited** — Space out batch loops; the prior CLI observed a limit of about 60 requests per minute per key
- **A generate, schedule or create-automation command printed a request but nothing happened** — That was the free preview; re-run with --execute to spend credits
- **The ledger shows an unconfirmed spend** — Run pingenerator-pp-cli list-scheduled-pins to see whether the pin exists before retrying

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

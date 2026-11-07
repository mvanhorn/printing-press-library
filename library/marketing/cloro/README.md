# cloro CLI

**ChatGPT, Google AI Mode and Gemini answers with their cited sources, Google results with the AI Overview, and a citation check across engines.**

Each engine is a typed command and an MCP tool: chatgpt, aimode, bing-aimode, gemini, google (with --include-aioverview) and news, geo-targeted by country. The cited command runs one prompt on several engines in parallel and tells you which of them cite your domain, and at which source positions.

Learn more at [cloro](https://cloro.dev/?utm_source=printing-press-library&utm_medium=byok).

Created by [@rbatista191](https://github.com/rbatista191) (Ricardo Batista).

## Install

The recommended path installs both the `cloro-pp-cli` binary and the `pp-cloro` agent skill (Claude Code, Codex, Cursor, Gemini CLI, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install cloro
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install cloro --cli-only
```

For skill only: installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install cloro --skill-only
```

To constrain the skill install to one or more specific agents (repeatable; agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install cloro --agent claude-code
npx -y @mvanhorn/printing-press-library install cloro --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/cloro/cmd/cloro-pp-cli@latest
```

This installs the CLI only; no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cloro-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install cloro --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cloro --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cloro --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install cloro --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle, Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cloro-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CLORO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/cloro/cmd/cloro-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "cloro": {
      "command": "cloro-pp-mcp",
      "env": {
        "CLORO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

### Remote MCP over HTTP

`cloro-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `CLORO_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `CLORO_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export CLORO_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
cloro-pp-mcp --transport http
```

Set `CLORO_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Quick Start

```bash
# Check the install and config without spending credits
cloro-pp-cli doctor --dry-run

# Read the credit balance (free)
cloro-pp-cli credits

# A ChatGPT answer with its cited sources (7 credits)
cloro-pp-cli chatgpt --prompt "best running shoes for flat feet" --country US

# Google results with the AI Overview (7 credits)
cloro-pp-cli google --query "best running shoes for flat feet" --country US --include-aioverview

# Which engines cite a domain for the prompt (26 credits for all four engines)
cloro-pp-cli cited nike.com --prompt "best running shoes for flat feet" --country US

```

## Unique Features

### AI visibility
- **`cited`**: See whether ChatGPT, Google AI Mode, Gemini and the Google AI Overview cite a domain for a prompt, with the source positions, in one command.

  _Reach for this when the question is whether a site is cited for a prompt, not what the answers say._

  ```bash
  cloro-pp-cli cited nike.com --prompt "best running shoes for flat feet" --country US --engines aimode,gemini --agent
  ```

## Recipes

### ChatGPT sources only

```bash
cloro-pp-cli chatgpt --prompt "best running shoes for flat feet" --country US --agent --select result.sources.url,result.sources.position
```

Keep only the cited URLs and their positions from a large answer payload.

### AI Overview sources for a keyword

```bash
cloro-pp-cli google --query "best running shoes for flat feet" --country US --include-aioverview --agent --select result.aioverview.sources.url
```

Read which pages the Google AI Overview cites.

### Citation check on two engines

```bash
cloro-pp-cli cited nike.com --prompt "best running shoes for flat feet" --country US --engines aimode,gemini --agent
```

Run fewer engines to spend fewer credits.

### Countries for an engine

```bash
cloro-pp-cli countries --model aimode
```

List the country codes an engine supports (free).

## Usage

Run `cloro-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CLORO_CONFIG_DIR`, `CLORO_DATA_DIR`, `CLORO_STATE_DIR`, or `CLORO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CLORO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CLORO_HOME=/srv/cloro
cloro-pp-cli doctor
```

Under `CLORO_HOME=/srv/cloro`, the four dirs resolve to `/srv/cloro/config`, `/srv/cloro/data`, `/srv/cloro/state`, and `/srv/cloro/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:CLORO_HOME = "C:\srv\cloro"
cloro-pp-cli doctor
```

Under `$env:CLORO_HOME = "C:\srv\cloro"`, the four dirs resolve to `C:\srv\cloro\config`, `C:\srv\cloro\data`, `C:\srv\cloro\state`, and `C:\srv\cloro\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "cloro": {
      "command": "cloro-pp-mcp",
      "env": {
        "CLORO_HOME": "/srv/cloro"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CLORO_DATA_DIR` overrides an explicit `--home` for that kind. Use `CLORO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CLORO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `cloro-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `cloro-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `cloro-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `CLORO_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `CLORO_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### aimode

Manage aimode

- **`cloro-pp-cli aimode`** - Extract structured data from AI Mode about your brand, products, or any topic across various regions

### async

Manage async

- **`cloro-pp-cli async batch`** - Submit up to 500 async tasks in one request. Each task is validated independently, so one invalid task does not block the rest. Returns per-task results.
- **`cloro-pp-cli async create`** - Submit an asynchronous task for background processing. Returns a task ID that you can use to poll for results or receive via webhook.
- **`cloro-pp-cli async get`** - Poll the status and result of an asynchronous task by ID. Returns the task state and, once complete, the full structured result payload.
- **`cloro-pp-cli async queue`** - Get organization-wide async queue metrics including queued and processing task counts, and concurrency usage.

### bing-aimode

Manage bing-aimode

- **`cloro-pp-cli bing-aimode`** - Extract structured data from Bing AI Mode: answer text, cited sources and citation pills. The prompt can have a maximum of 3,500 characters after URL encoding. Costs 4 credits, plus 2 for a sync request and 2 for a US state. To check a domain on Bing AI Mode with `cited`, add `bing-aimode` to `--engines`.

### chatgpt

Manage chatgpt

- **`cloro-pp-cli chatgpt`** - Extract structured data from ChatGPT, answer text, cited sources, shopping cards, brand entities, and ads, for any prompt across supported regions

### countries

Manage countries

- **`cloro-pp-cli countries`** - Returns a list of all supported ISO 3166-1 alpha-2 country codes. Can be filtered by model to get countries available for specific AI providers.

### credits

Manage credits

- **`cloro-pp-cli credits`** - Read your organization's current credit balance and billing cycle. Lets you observe your balance programmatically, including from async-only workloads, without making a sync /v1/monitor/* request just to inspect the X-Credits-Remaining header. The balance is cached and can lag by up to 60 seconds. Queued async tasks are re-checked against your live balance when the scheduler picks them up.

### gemini

Manage gemini

- **`cloro-pp-cli gemini`** - Extract structured data from Google Gemini, generated answer text and cited sources for any prompt, with Markdown and HTML export formats

### google

Manage google

- **`cloro-pp-cli google`** - Extract structured data from Google Search including organic results, People Also Ask, related searches, and optional AI Overview

### goto

Manage goto

- **`cloro-pp-cli goto`** - Resolve one Google redirect link to its destination URL. Send the `redirectLink` that ships beside a link on a [Google Search](https://cloro.dev/docs/api-reference/endpoint/monitor-google?utm_source=printing-press-library&utm_medium=byok), [Google News](https://cloro.dev/docs/api-reference/endpoint/monitor-google-news?utm_source=printing-press-library&utm_medium=byok), or [AI Mode](https://cloro.dev/docs/api-reference/endpoint/monitor-aimode?utm_source=printing-press-library&utm_medium=byok) response, and the response gives you the URL it points to.

Costs 1 credit per request, with no sync surcharge, and only when the call returns the destination. Each request resolves one link. If the link does not resolve, the call returns `502`, and you can retry it later.

### news

Manage news

- **`cloro-pp-cli news`** - Extract structured news articles from Google News including titles, links, snippets, sources, dates, and thumbnails

### product

Manage product

- **`cloro-pp-cli product`** - Return the product panel that a Google Search shopping card or a Google AI Mode product opens: merchant offers, description, specifications, reviews, images and related products. Send the `productResultToken` from a Google Search `result.shoppingCards` card, or from a Google AI Mode `result.shoppingCards` or `result.inlineProducts` entry.

Costs 1 credit, plus the 2-credit sync surcharge, and only when the request succeeds. A token expires five minutes after the scrape that issued it. If you send the request async, set a high `priority`, so that the token does not expire in the queue.

### states

Manage states

- **`cloro-pp-cli states`** - Returns a list of US states supported for state-level geo-targeting. Only US is currently supported, other country values return an empty array.


### cited


- **`cloro-pp-cli cited`** - See whether ChatGPT, Google AI Mode, Gemini and the Google AI Overview cite a domain for a prompt, with the source positions, in one command.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`cloro-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`cloro-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`cloro-pp-cli learnings list`** - Inspect taught rows
- **`cloro-pp-cli learnings forget <query>`** - Undo a teach
- **`cloro-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`cloro-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`cloro-pp-cli teach-pattern`** - Install a query/resource template up front
- **`cloro-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CLORO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `cloro-pp-cli` opens the database, older binaries refuse it with a version error; upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cloro-pp-cli async get mock-value

# JSON for scripting and agents
cloro-pp-cli async get mock-value --json
# Filter to specific fields
cloro-pp-cli async get mock-value --json --select credits,response,task

# Dry run: show the request without sending
cloro-pp-cli async get mock-value --dry-run

# Agent mode: JSON + compact + no prompts in one flag
cloro-pp-cli async get mock-value --agent
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
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
cloro-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `cloro-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/cloro-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\cloro-pp-cli\config.toml` on Windows; `--home`, `CLORO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `CLORO_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `CLORO_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLORO_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `cloro-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cloro-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLORO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 INVALID_API_KEY_FORMAT**: export CLORO_API_KEY=<key from https://dashboard.cloro.dev/api-keys?utm_source=printing-press-library&utm_medium=byok>
- **403 INSUFFICIENT_CREDITS**: Check the balance with cloro-pp-cli credits, then add credits in the dashboard or check fewer engines per cited run
- **429 CONCURRENCY_LIMIT_EXCEEDED**: Wait for running requests to finish, or send bulk work through cloro-pp-cli async batch
- **A Google result link is empty or a google.com/goto URL**: Pass its redirectLink to cloro-pp-cli goto --url <redirectLink>

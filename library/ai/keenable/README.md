# Keenable CLI

**Keenable web search with reproducible research, citations, and a local evidence trail.**

Search current web knowledge and fetch clean Markdown through the documented Keenable API, then preserve the evidence locally so agents can replay, compare, and cite prior research instead of starting from a blank response.

## Install

The recommended path installs both the `keenable-pp-cli` binary and the `pp-keenable` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install keenable
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install keenable --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install keenable --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install keenable --agent claude-code
npx -y @mvanhorn/printing-press-library install keenable --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/keenable/cmd/keenable-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/keenable-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install keenable --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-keenable --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-keenable --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install keenable --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/keenable-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `KEENABLE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/keenable/cmd/keenable-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "keenable": {
      "command": "keenable-pp-mcp",
      "env": {
        "KEENABLE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set KEEENABLE_API_KEY for authenticated organization-scoped usage. Keenable also exposes keyless public endpoints that require an X-Keenable-Title application header and share an IP-based pool.

## Quick Start

```bash
# Check the generated CLI without making a request.
keenable-pp-cli doctor --dry-run

# Preview the typed search operation and its request shape.
keenable-pp-cli web-search search-post --dry-run

# Preview Markdown page retrieval.
keenable-pp-cli fetch fetch --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Evidence that compounds
- **`research snapshot`** — Save an exact search and fetch run as an immutable local evidence snapshot.

  _Choose this when an agent needs to reproduce or audit the evidence behind a research answer._

  ```bash
  keenable-pp-cli research snapshot --query "AI agent evaluation methods" --max-results 8 --agent
  ```
- **`research replay`** — Rerun a saved research recipe and report how current evidence changed.

  _Choose this for weekly monitoring and regression checks where ranking drift matters._

  ```bash
  keenable-pp-cli research replay --snapshot latest --agent
  ```
- **`research local-search`** — Search saved titles, snippets, URLs, and Markdown without spending an upstream request.

  _Choose this when an agent needs to recall prior evidence while preserving a clear local-versus-live boundary._

  ```bash
  keenable-pp-cli research local-search "retrieval evaluation" --agent
  ```
- **`research diff`** — Compare saved runs for URL changes, rank movement, metadata edits, and content-hash drift.

  _Choose this to detect changing sources before an agent relies on stale research._

  ```bash
  keenable-pp-cli research diff --before latest --after latest --agent
  ```

### Agent-ready evidence
- **`research citations`** — Export source-linked Markdown or JSON citations from a saved research run.

  _Choose this when another agent or reviewer needs portable, inspectable evidence rather than raw API output._

  ```bash
  keenable-pp-cli research citations --snapshot latest --format markdown
  ```
- **`research fetch-many`** — Fetch a bounded URL list with concurrency limits and explicit partial failures.

  _Choose this when a pipeline has several known sources and must not silently drop a failed fetch._

  ```bash
  keenable-pp-cli research fetch-many --url https://docs.keenable.ai/api-reference --url https://docs.keenable.ai/mcp-server --agent
  ```
- **`research coverage`** — Measure domain diversity, rank share, timestamp coverage, and missing metadata in a saved run.

  _Choose this when an agent must explain whether its evidence is broad, balanced, and date-grounded._

  ```bash
  keenable-pp-cli research coverage --snapshot latest --agent
  ```

## Recipes

### Save a reproducible run

```bash
keenable-pp-cli research snapshot --query "AI agent evaluation methods" --max-results 8 --agent
```

Persist the exact query and evidence for later replay.

### Fetch several sources safely

```bash
keenable-pp-cli research fetch-many --url https://docs.keenable.ai/api-reference --url https://docs.keenable.ai/mcp-server --agent
```

Keep successes and failures explicit in one structured result.

### Recall saved evidence

```bash
keenable-pp-cli research local-search "retrieval evaluation" --agent --select results
```

Search the local corpus without claiming fresh upstream data.

### Compare research drift

```bash
keenable-pp-cli research diff --before latest --after latest --agent
```

Inspect changed sources and content between two saved runs.

## Usage

Run `keenable-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `KEENABLE_CONFIG_DIR`, `KEENABLE_DATA_DIR`, `KEENABLE_STATE_DIR`, or `KEENABLE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `KEENABLE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export KEENABLE_HOME=/srv/keenable
keenable-pp-cli doctor
```

Under `KEENABLE_HOME=/srv/keenable`, the four dirs resolve to `/srv/keenable/config`, `/srv/keenable/data`, `/srv/keenable/state`, and `/srv/keenable/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "keenable": {
      "command": "keenable-pp-mcp",
      "env": {
        "KEENABLE_HOME": "/srv/keenable"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `KEENABLE_DATA_DIR` overrides an explicit `--home` for that kind. Use `KEENABLE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `KEENABLE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `keenable-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### fetch

Manage fetch

- **`keenable-pp-cli fetch fetch`** - Extract clean content (markdown) from a web page URL.
- **`keenable-pp-cli fetch public`** - Keyless twin of GET /v1/fetch. Same query parameters and same response, no API key. Rate limited per IP rather than per organization, and consumes no credits.

### web_search

Manage web search

- **`keenable-pp-cli web-search search-post`** - Perform a web search using a JSON request body. Supports date filters and site restriction.
- **`keenable-pp-cli web-search search-post-public`** - Keyless twin of POST /v1/search. Same request body and same response, no API key. Rate limited per IP rather than per organization, and consumes no credits.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`keenable-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`keenable-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`keenable-pp-cli learnings list`** - Inspect taught rows
- **`keenable-pp-cli learnings forget <query>`** - Undo a teach
- **`keenable-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`keenable-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`keenable-pp-cli teach-pattern`** - Install a query/resource template up front
- **`keenable-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `KEENABLE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `keenable-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
keenable-pp-cli fetch fetch --url https://example.com/resource

# JSON for scripting and agents
keenable-pp-cli fetch fetch --url https://example.com/resource --json
# Filter to specific fields
keenable-pp-cli fetch fetch --url https://example.com/resource --json --select author,content,description

# Dry run — show the request without sending
keenable-pp-cli fetch fetch --url https://example.com/resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
keenable-pp-cli fetch fetch --url https://example.com/resource --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
keenable-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `keenable-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/keenable-pp-cli/config.toml`; `--home`, `KEENABLE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `KEENABLE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `keenable-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `keenable-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $KEENABLE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **400 Missing app identifier on a keyless request** — Use KEEENABLE_API_KEY or set the generated CLI's application title so it sends X-Keenable-Title.
- **401 or 403 authentication failure** — Rotate the Keenable key and export it as KEEENABLE_API_KEY; do not paste it into command arguments.
- **429 rate limit exceeded** — Retry after the server's Retry-After value or use authenticated organization-scoped access.
- **Fetch returns not indexed** — Retry with the fetch live=true option when the source permits direct retrieval.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**keenable-cli**](https://github.com/keenableai/keenable-cli) — Rust (2 stars)
- [**keenable-mcp**](https://github.com/keenableai/keenable-mcp) — JavaScript (2 stars)
- [**keenable-sdk**](https://github.com/keenableai/keenable-sdk) — Python/TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

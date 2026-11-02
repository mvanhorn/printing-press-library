# Government Data CLI

Created by [@Jon-275](https://github.com/Jon-275) (Jon Malone).

**Find California and U.S. open datasets and SEC EDGAR filings with clear provenance.**

Search either public-data catalog or both, inspect metadata and download selected resources. Direct EDGAR commands search filings and retrieve company submissions and financial facts; local snapshot tools support repeatable catalog research.

## Install

The recommended path installs both the `government-data-pp-cli` binary and the `pp-government-data` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install government-data
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install government-data --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install government-data --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install government-data --agent claude-code
npx -y @mvanhorn/printing-press-library install government-data --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/cmd/government-data-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/government-data-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install government-data --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-government-data --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-government-data --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install government-data --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/government-data-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DATA_GOV_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/cmd/government-data-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "government-data": {
      "command": "government-data-pp-mcp",
      "env": {
        "DATA_GOV_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

California public reads need no key. Federal official v4 gateway uses a free api.data.gov key. Public DEMO_KEY is for initial exploration only: 30 requests per hour and 50 per day. Configure a personal key for automated use. SEC reads use SEC_USER_AGENT with your app name and contact email, and require no API key.

## Quick Start

```bash
# Check the command setup without network access.
government-data-pp-cli doctor --dry-run

# Compare a bounded selection from both catalogs.
government-data-pp-cli catalog find water --source both --limit 5 --agent --demo

# Find a company filing after setting SEC_USER_AGENT.
government-data-pp-cli edgar filings 0000320193 --form 10-K --limit 1 --agent

```

## Recipes

### Inspect concise search results

```bash
government-data-pp-cli catalog find water --source both --limit 5 --agent --select results.title,results.source,results.id --demo
```

Keep only the fields needed to select records.

### Company financial fact

```bash
government-data-pp-cli edgar facts 0000320193 --concept Assets --agent
```

Retrieve reported asset facts with units, periods and filing references.

## Unique Features

### Compare catalogs
- **`overlap`** — Find exact resource URLs shared across saved catalog records.

  _Find exact resource URLs shared across saved catalog records._
- **`intake audit`** — List missing metadata fields before selecting resources.

  _List missing metadata fields before selecting resources._
- **`coverage`** — Compare declared formats within a saved topic scope.

  _Compare declared formats within a saved topic scope._

### Repeatable research
- **`changes`** — Compare metadata and resource links between two explicit snapshots.

  _Compare metadata and resource links between two explicit snapshots._
- **`intake lock`** — Save selected resource URLs and metadata fingerprints for repeatable intake.

  _Save selected resource URLs and metadata fingerprints for repeatable intake._

## Usage

Run `government-data-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GOVERNMENT_DATA_CONFIG_DIR`, `GOVERNMENT_DATA_DATA_DIR`, `GOVERNMENT_DATA_STATE_DIR`, or `GOVERNMENT_DATA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GOVERNMENT_DATA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GOVERNMENT_DATA_HOME=/srv/government-data
government-data-pp-cli doctor
```

Under `GOVERNMENT_DATA_HOME=/srv/government-data`, the four dirs resolve to `/srv/government-data/config`, `/srv/government-data/data`, `/srv/government-data/state`, and `/srv/government-data/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:GOVERNMENT_DATA_HOME = "C:\srv\government-data"
government-data-pp-cli doctor
```

Under `$env:GOVERNMENT_DATA_HOME = "C:\srv\government-data"`, the four dirs resolve to `C:\srv\government-data\config`, `C:\srv\government-data\data`, `C:\srv\government-data\state`, and `C:\srv\government-data\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "government-data": {
      "command": "government-data-pp-mcp",
      "env": {
        "GOVERNMENT_DATA_HOME": "/srv/government-data"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GOVERNMENT_DATA_DATA_DIR` overrides an explicit `--home` for that kind. Use `GOVERNMENT_DATA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GOVERNMENT_DATA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `government-data-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Generated endpoint reads can save locally: in `--data-source auto` mode successful generated reads are written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `government-data-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `government-data-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `GOVERNMENT_DATA_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `GOVERNMENT_DATA_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### california

California public catalog API

- **`government-data-pp-cli california dataset`** - Retrieve complete dataset metadata and resource references
- **`government-data-pp-cli california find`** - Search catalog datasets using text and filters
- **`government-data-pp-cli california organizations`** - List dataset publishing organizations
- **`government-data-pp-cli california resource`** - Retrieve California resource download metadata
- **`government-data-pp-cli california tags`** - List California dataset keyword tags
- **`government-data-pp-cli california topics`** - List source-native dataset topics or keywords

### federal

U.S. Data.gov official v4 catalog

- **`government-data-pp-cli federal dataset`** - Retrieve complete dataset metadata and resource references
- **`government-data-pp-cli federal find`** - Search catalog datasets using text and filters
- **`government-data-pp-cli federal organizations`** - List dataset publishing organizations
- **`government-data-pp-cli federal topics`** - List source-native dataset topics or keywords


### changes


- **`government-data-pp-cli changes`** - Compare metadata and resource links between two explicit snapshots.


### coverage


- **`government-data-pp-cli coverage`** - Compare declared formats within a saved topic scope.


### intake


- **`government-data-pp-cli intake lock`** - Save selected resource URLs and metadata fingerprints for repeatable intake.

- **`government-data-pp-cli intake audit`** - List missing metadata fields before selecting resources.

- **`government-data-pp-cli intake`** - Audit saved catalog metadata and preserve selected resource provenance


### overlap


- **`government-data-pp-cli overlap`** - Find exact resource URLs shared across saved catalog records.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`government-data-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`government-data-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`government-data-pp-cli learnings list`** - Inspect taught rows
- **`government-data-pp-cli learnings forget <query>`** - Undo a teach
- **`government-data-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`government-data-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`government-data-pp-cli teach-pattern`** - Install a query/resource template up front
- **`government-data-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GOVERNMENT_DATA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `government-data-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
government-data-pp-cli california dataset --id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
government-data-pp-cli california dataset --id 550e8400-e29b-41d4-a716-446655440000 --json
# Filter to specific fields
government-data-pp-cli california dataset --id 550e8400-e29b-41d4-a716-446655440000 --json --select id,title,name

# Dry run — show the request without sending
government-data-pp-cli california dataset --id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
government-data-pp-cli california dataset --id 550e8400-e29b-41d4-a716-446655440000 --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
government-data-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `government-data-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/government-data-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\government-data-pp-cli\config.toml` on Windows; `--home`, `GOVERNMENT_DATA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `GOVERNMENT_DATA_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `GOVERNMENT_DATA_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DATA_GOV_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `government-data-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `government-data-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DATA_GOV_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ckanapi**](https://github.com/ckan/ckanapi) — Python
- [**Data.gov MCP Server**](https://github.com/melaodoidao/datagov-mcp-server) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Catalog snapshots and SEC access

Set `DATA_GOV_API_KEY` to a free api.data.gov key for automated federal catalog use. For initial exploration, `--demo` opts into the limited public DEMO_KEY. California and SEC reads do not need that key. Set `SEC_USER_AGENT` to your application name and a real contact email before running EDGAR commands; requests send it to SEC only.

```bash
government-data-pp-cli catalog find water --source california --limit 5 --save water-a
government-data-pp-cli sync --resources california --param q=water --max-pages 1
government-data-pp-cli overlap --help
government-data-pp-cli changes --help
government-data-pp-cli intake audit --help
government-data-pp-cli coverage --help
government-data-pp-cli edgar search water --forms 10-K --limit 5 --json
government-data-pp-cli edgar filings 0000320193 --form 10-K --limit 1 --json
government-data-pp-cli edgar facts 0000320193 --concept Assets --json
```

Snapshot IDs are immutable. Snapshot comparisons describe bounded query results; a missing record is not evidence that an agency deleted it. Exact URL overlap is not a claim that two datasets are semantically identical. The `coverage` and `intake` commands apply to saved catalog metadata, not EDGAR facts.

`sync` captures a fresh bounded query. It accepts `--resources`, `--param q=...`, `--max-pages`, and `--full`; incremental `--since` is explicitly unsupported. Use `catalog export` for bounded JSON metadata output and `resource download --url ... --output ...` for a selected HTTPS file. Downloads refuse overwrites and cross-origin redirects; use the final agency URL if a redirect is refused.

EDGAR history scans and full-text search are bounded by `--max-scan-pages`. Recent submissions alone are not full filing history. Financial facts retain source taxonomies, units, periods and filing references; they are not reconstructed or investment-ranked financial statements.

Custom workflow families: `catalog find`, `catalog export`, `catalog organizations`, `catalog topics`, `dataset show`, `dataset resources`, `resource download`, `edgar company`, `edgar search`, `edgar filings`, `edgar documents`, and `edgar facts`. Use their help for filters and output flags. For saved snapshots, supply the snapshot ID to the snapshot flag on overlap, coverage and intake audit; changes accepts explicit from/to snapshot IDs.

Desktop MCP installs expose an optional **SEC application and contact** setting. Supply an application name and real contact email to use EDGAR tools. Custom catalog, resource, and SEC requests honor `--ca-cert`, `--tls-server-name`, `--timeout`, and `--rate-limit`; SEC remains capped at two requests per second unless local pacing is explicitly disabled with zero. Saved catalog records are searchable with `--type datasets`, `--type california`, or `--type federal` in local mode.

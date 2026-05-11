# USPTO Patents CLI

**Every patent prosecution feature from every existing tool, plus offline search, family trees, and portfolio analytics no other CLI offers**

Search and look up US patent applications, track prosecution history, traverse family trees, and monitor competitor portfolios — all with a local SQLite cache for offline search and cross-entity queries. Works with the USPTO Open Data Portal API.

Learn more at [USPTO Patents](https://data.uspto.gov/apis/getting-started).

## Install

The recommended path installs both the `uspto-patents-pp-cli` binary and the `pp-uspto-patents` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install uspto-patents
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install uspto-patents --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/uspto-patents-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-uspto-patents --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-uspto-patents --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-uspto-patents skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-uspto-patents. The skill defines how its required CLI can be installed.
```

## Authentication

Get a free API key from data.uspto.gov (requires ID.me identity verification). Set it with `export USPTO_API_KEY=your-key`. Run `doctor` to verify connectivity.

## Quick Start

```bash
# Verify your API key and ODP connectivity
uspto-patents-pp-cli doctor


# Search for patents by inventor
uspto-patents-pp-cli patent list-applications --q 'applicationMetaData.firstInventorName:Tesla'


# Look up a specific patent application
uspto-patents-pp-cli patent get 14412875 --json


# See the full prosecution timeline
uspto-patents-pp-cli patent timeline 14412875


# Cache patent data from the last 30 days locally for offline search
uspto-patents-pp-cli sync --since 30d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Prosecution intelligence
- **`patent timeline`** — See every prosecution event in chronological order — office actions, amendments, continuity filings, and PTAB proceedings merged into one stream

  _When evaluating a patent's prosecution strength, agents need the full event history in one call instead of querying three separate endpoints_

  ```bash
  uspto-patents-pp-cli patent timeline 14412875 --json
  ```
- **`patent family`** — Walk the full patent family tree — continuations, divisionals, and CIPs — rendered as an indented tree or flat table

  _Patent families often span dozens of related applications; agents evaluating IP portfolios need the full tree, not just direct parents and children_

  ```bash
  uspto-patents-pp-cli patent family 14412875 --json
  ```
- **`patent batch-history`** — Fetch prosecution history for a list of patents in one command — transactions, continuity, and assignments with automatic rate-limit throttling

  _Agents evaluating prior art need prosecution context for multiple patents without managing rate limits or individual API calls_

  ```bash
  uspto-patents-pp-cli patent batch-history 14412875 15123456 16789012 --json
  ```
- **`patent one-look`** — Get the full current state of any patent in one command — status, owner, family depth, PTA days, PTAB exposure, and attorney of record

  _Agents evaluating a patent need the complete current picture without making half a dozen sequential API calls_

  ```bash
  uspto-patents-pp-cli patent one-look 14412875 --json
  ```

### Portfolio analytics
- **`portfolio snapshot`** — Get aggregate stats for any assignee's patent portfolio — filing counts, grant rates, art-unit distribution, and status breakdown

  _Agents doing competitive intelligence need portfolio-level metrics, not individual application records_

  ```bash
  uspto-patents-pp-cli portfolio snapshot "Apple Inc" --json
  ```
- **`portfolio diff`** — See what changed in an assignee's patent portfolio since your last check — new filings, status changes, and ownership transfers

  _Agents monitoring competitor IP activity need change detection, not full portfolio dumps they have to diff themselves_

  ```bash
  uspto-patents-pp-cli portfolio diff "Apple Inc" --since 2026-04-01 --json
  ```

### Local state that compounds
- **`related`** — Find patents in your local cache that share the same art unit, assignee, or inventor as a given patent

  _Agents doing prior-art search need to discover related patents by multiple dimensions, not just keyword search_

  ```bash
  uspto-patents-pp-cli related 14412875 --json
  ```

## Usage

Run `uspto-patents-pp-cli --help` for the full command reference and flag list.

## Commands

### datasets

Manage datasets

- **`uspto-patents-pp-cli datasets get`** - Bulk data- find a product by its identifier (shortName)
- **`uspto-patents-pp-cli datasets get-products`** - Returns a 302 redirect to the actual download location for the given productIdentifier and fileName.
- **`uspto-patents-pp-cli datasets list`** - Query parameters are optional. When no query parameters supplied, top 25 applications are returned

### patent

Manage patent

- **`uspto-patents-pp-cli patent create`** - Search patent application status codes and status code description
- **`uspto-patents-pp-cli patent create-appeals`** - Search appeals decisions using json payload
- **`uspto-patents-pp-cli patent create-appeals-2`** - Download appeals decisions search results in json or csv format using json payload
- **`uspto-patents-pp-cli patent create-applications`** - Search patent applications by supplying json payload
- **`uspto-patents-pp-cli patent create-applications-2`** - Download patent data by supplying json payload
- **`uspto-patents-pp-cli patent create-interferences`** - Request body matches PatentSearchRequest; example tailored for interferences.
- **`uspto-patents-pp-cli patent create-interferences-2`** - Download interferences decisions search results in json or csv format using json payload
- **`uspto-patents-pp-cli patent create-trials`** - Search trials decisions documents using json payload
- **`uspto-patents-pp-cli patent create-trials-2`** - Search trials documents using json payload
- **`uspto-patents-pp-cli patent create-trials-3`** - Search trials proceedings using json payload
- **`uspto-patents-pp-cli patent create-trials-4`** - Download trials decisions documents search results in json or csv format using json payload
- **`uspto-patents-pp-cli patent create-trials-5`** - Download trials documents search results in json or csv format using json payload
- **`uspto-patents-pp-cli patent create-trials-6`** - Download trials proceedings search results in json or csv format using json payload
- **`uspto-patents-pp-cli patent get`** - Patent application data for a provided application number
- **`uspto-patents-pp-cli patent get-appeals`** - Retrieve appeals decisions by document Identifier
- **`uspto-patents-pp-cli patent get-appeals-2`** - Retrieve appeals decisions by appeal number
- **`uspto-patents-pp-cli patent get-applications`** - Get patent term adjustment data for an application number
- **`uspto-patents-pp-cli patent get-applications-2`** - Get patent assignment data for an application number
- **`uspto-patents-pp-cli patent get-applications-3`** - Associated (pgpub, grant) documents meta-data for an application
- **`uspto-patents-pp-cli patent get-applications-4`** - Get attorney/agent data for an application number
- **`uspto-patents-pp-cli patent get-applications-5`** - Get continuity data for an application number
- **`uspto-patents-pp-cli patent get-applications-6`** - Documents details for an application number
- **`uspto-patents-pp-cli patent get-applications-7`** - Get foreign-priority data for an application number
- **`uspto-patents-pp-cli patent get-applications-8`** - Get patent application meta data
- **`uspto-patents-pp-cli patent get-applications-9`** - Get transaction data for an application number
- **`uspto-patents-pp-cli patent get-interferences`** - Returns a single interference decision document for the provided `documentIdentifier`.
- **`uspto-patents-pp-cli patent get-interferences-2`** - Returns one or more interference decision records for the provided `interferenceNumber`.
- **`uspto-patents-pp-cli patent get-trials`** - Retrieve a single trials decisions document by document identifier
- **`uspto-patents-pp-cli patent get-trials-2`** - Retrieve a single trials document by document identifier
- **`uspto-patents-pp-cli patent get-trials-3`** - Retrieve a single trials proceeding by trial number
- **`uspto-patents-pp-cli patent get-trials-4`** - Retrieve all trials decisions documents by trial number
- **`uspto-patents-pp-cli patent get-trials-5`** - Retrieve all trials documents by trial number
- **`uspto-patents-pp-cli patent list`** - Search patent application status codes and status code description
- **`uspto-patents-pp-cli patent list-appeals`** - Search appeals decisions using query parameters
- **`uspto-patents-pp-cli patent list-appeals-2`** - Download appeals decisions search results in json or csv format using query parameters
- **`uspto-patents-pp-cli patent list-applications`** - Query parameters are optional. When no query parameters supplied, top 25 applications are returned
- **`uspto-patents-pp-cli patent list-applications-2`** - Query parameters are optional. When no query parameters supplied, top 25 applications are returned
- **`uspto-patents-pp-cli patent list-interferences`** - Query interference decisions (decision type/category, party names, date ranges, etc.).
- **`uspto-patents-pp-cli patent list-interferences-2`** - Download interferences decisions search results in json or csv format using query parameters
- **`uspto-patents-pp-cli patent list-trials`** - Search trials decisions documents using query parameters
- **`uspto-patents-pp-cli patent list-trials-2`** - Search trials documents using query parameters
- **`uspto-patents-pp-cli patent list-trials-3`** - Search trials proceedings using query parameters
- **`uspto-patents-pp-cli patent list-trials-4`** - Download trials decisions documents search results in json or csv format using query parameters
- **`uspto-patents-pp-cli patent list-trials-5`** - Download trials document search results in json or csv format using query parameters
- **`uspto-patents-pp-cli patent list-trials-6`** - Download trials proceedings search results in json or csv format using query parameters

### petition

Manage petition

- **`uspto-patents-pp-cli petition create`** - Search petition decision applications by supplying json payload
- **`uspto-patents-pp-cli petition create-decisions`** - Download petition decision data by supplying json payload
- **`uspto-patents-pp-cli petition get`** - Petition decision application data for a provided application number
- **`uspto-patents-pp-cli petition list`** - Query parameters are optional. When no query parameters supplied, top 25 petition decisions are returned
- **`uspto-patents-pp-cli petition list-decisions`** - Query parameters are optional. When no query parameters supplied, top 25 petition decisions are returned


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
uspto-patents-pp-cli datasets list

# JSON for scripting and agents
uspto-patents-pp-cli datasets list --json

# Filter to specific fields
uspto-patents-pp-cli datasets list --json --select id,name,status

# Dry run — show the request without sending
uspto-patents-pp-cli datasets list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
uspto-patents-pp-cli datasets list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-uspto-patents -g
```

Then invoke `/pp-uspto-patents <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add uspto-patents uspto-patents-pp-mcp -e USPTO_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/uspto-patents-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `USPTO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "uspto-patents": {
      "command": "uspto-patents-pp-mcp",
      "env": {
        "USPTO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
uspto-patents-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/uspto-patents-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `USPTO_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `uspto-patents-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $USPTO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **403 Forbidden on every request** — Set USPTO_API_KEY — get one free at data.uspto.gov/apis/getting-started (requires ID.me)
- **429 Too Many Requests** — ODP rate limit is 60 req/min. The CLI throttles automatically; if you see this, wait 60 seconds
- **Patent number not found** — Try the application number format (e.g., 14412875) instead of grant number (e.g., US9,123,456). The CLI auto-resolves but some edge cases need the app number.
- **Empty search results** — Check field names — ODP uses applicationMetaData.firstInventorName, not 'inventor'. Run `patent list-applications --help` for available fields.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**patent_mcp_server**](https://github.com/riemannzeta/patent_mcp_server) — Python
- [**ip_tools**](https://github.com/parkerhancock/ip_tools) — Python
- [**uspto-odp**](https://github.com/patent-dev/uspto-odp) — Go
- [**bulk-file-loader**](https://github.com/patent-dev/bulk-file-loader) — Go
- [**uspto-opendata-python**](https://github.com/ip-tools/uspto-opendata-python) — Python
- [**fastpat**](https://github.com/iamlemec/fastpat) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

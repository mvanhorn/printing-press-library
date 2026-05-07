# Postman Explore CLI

**The CLI for the public API directory at postman.com/explore — search, rank, and watch community-contributed Postman Collections, agent-native and offline.**

Postman's API Network is the world's largest public API directory, but discovery is web-only and unscriptable. This CLI mirrors the public surface into a local SQLite store with FTS, then layers commands the website cannot offer: canonical-collection lookup, cross-publisher comparison, trend ranking by custom metrics, and drift detection across snapshots. No authentication required; designed for AI agents and power users who need the network as data, not as a webpage.

Learn more at [Postman Explore](https://www.postman.com/explore).

## Install

The recommended path installs both the `postman-explore-pp-cli` binary and the `pp-postman-explore` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install postman-explore
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install postman-explore --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/postman-explore-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Authentication

No authentication required. The CLI uses a Surf HTTP transport with Chrome TLS fingerprinting to clear Cloudflare's HTML challenge gate; the proxy API itself is open.

## Quick Start

```bash
# Confirm reachability and see network-wide entity counts in one call.
postman-explore-pp-cli networkentity get-network-entity-counts


# Populate the local store so search, top, drift, canonical, and publishers all work offline.
postman-explore-pp-cli sync --all-types --limit 200


# The headline command — the best community Postman Collection for a known vendor.
postman-explore-pp-cli canonical stripe


# Trend ranking by metric, narrowed to a category.
postman-explore-pp-cli top --metric weekForkCount --type collection --category payments --limit 5 --json


# Compare publishers across a category by aggregate fork count.
postman-explore-pp-cli publishers top --category developer-productivity --limit 5


# What changed on the network this week?
postman-explore-pp-cli drift --since 7d --type collection

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Discovery that compounds locally
- **`canonical`** — One command finds the best community Postman Collection for a vendor, ranked by publisher verification, fork count, and recency.

  _When the user asks an agent for a vendor's Postman Collection, this returns the best canonical choice in one call instead of forcing the agent to dedupe a search result list._

  ```bash
  postman-explore-pp-cli canonical stripe --json
  ```
- **`top`** — Rank entities by any metric (weekForks, monthViewCount, etc.) with category and entity-type narrowing.

  _When agents need to recommend the most-forked collections THIS week, not the all-time leaders, this is the only path that works._

  ```bash
  postman-explore-pp-cli top --metric weekForkCount --type collection --category payments --limit 10
  ```
- **`similar`** — Given an entity numeric id, return collections with overlapping name, summary, and category — ranked by FTS relevance score against the seed entity's text.

  _When an integrator finds one collection but wants to compare against alternatives, this returns the rivals in one call rather than forcing a fresh search._

  ```bash
  postman-explore-pp-cli similar 10289 --limit 5 --json
  ```

### Comparative analysis
- **`publishers top`** — Aggregate fork counts across every entity per publisher within a category; rank teams by total community gravity.

  _API curators picking between vendors need cross-publisher comparison. Agents recommending an integration partner need this to break ties._

  ```bash
  postman-explore-pp-cli publishers top --category developer-productivity --limit 5 --json
  ```
- **`category landscape`** — For a category slug, return: total entity counts per type, top 5 publishers by aggregate fork count, and top 5 entities by viewCount. One command, one structured payload.

  _API curators evaluating a vertical (payments, AI, devops) want one snapshot of who dominates and what's popular; this is exactly that snapshot._

  ```bash
  postman-explore-pp-cli category landscape payments --json
  ```

### Time-windowed signals
- **`drift`** — Compare two synced snapshots and report new entities, removed entities, and entities whose updatedAt advanced.

  _Agents tracking when a vendor publishes a new collection or updates an existing one rely on this; there is no other way to know._

  ```bash
  postman-explore-pp-cli drift --since 7d --type collection --json
  ```
- **`velocity`** — Rank collections by acceleration ratio: (weekForkCount × 4) / monthForkCount. Surfaces collections breaking out before they top the popular list.

  _Catching a breakout collection before it tops the popular sort is high-value for API curators tracking emerging vendors._

  ```bash
  postman-explore-pp-cli velocity --type collection --top 10
  ```

### Quality signals
- **`browse`** — When passed --verified-only, the browse command filters to entities owned by publishers with the verified-team flag set.

  _When you want only the official-vendor collections (not community forks), this is the cleanest filter._

  ```bash
  postman-explore-pp-cli browse collection --verified-only --category payments --json --limit 5
  ```

## Usage

Run `postman-explore-pp-cli --help` for the full command reference and flag list.

## Commands

### category

Manage category

- **`postman-explore-pp-cli category get`** - Returns full details for a category by its URL slug (e.g.,
`artificial-intelligence`, `developer-productivity`, `payments`).
Includes hero image, icon, and category-specific featured entities.
- **`postman-explore-pp-cli category list-categories`** - Returns all categories used to organize the API network (e.g., AI,
E-commerce, Communication, DevOps). Categories are spotlighted —
the response order matches the category strip on postman.com/explore.

### networkentity

Manage networkentity

- **`postman-explore-pp-cli networkentity get-network-entity`** - Returns full entity record by internal numeric id (the `id` field from
`listNetworkEntities`, NOT the `entityId` UUID).
- **`postman-explore-pp-cli networkentity get-network-entity-counts`** - Returns aggregate counts across the entire public API network:
collections, workspaces, APIs, flows, notebooks, teams. This is the
only stats endpoint exposed by the proxy.
- **`postman-explore-pp-cli networkentity list-network-entities`** - List public collections, workspaces, APIs, or flows. Supports category
filtering, pagination, and minimum-fork filtering. This is the primary
browse endpoint powering postman.com/explore/{collections,workspaces,apis,flows}.

Note on sort: only `popular` is reliably supported. Other values like
`recent`, `new`, `week`, `alltime` return HTTP 400 "Invalid sort type".

### search-all

Manage search all

- **`postman-explore-pp-cli search-all search_all`** - Search collections, workspaces, requests, flows, and teams by free-text
query. With no `queryIndices` set, the response groups results by
entity type (object-keyed `data.{collection,workspace,api,flow,team,request}`).
With one or more dotted indices set (`runtime.collection`,
`collaboration.workspace`, `apinetwork.team`, `flow.flow`,
`runtime.request`), the response narrows to those types.

This is the engine behind the search bar on postman.com.

### team

Publisher teams on the API network

- **`postman-explore-pp-cli team get`** - Returns small profile object (id, name, description, publicHandle,
profileURL, createdAt, updatedAt) for a team by its numeric id.
For full workspace listing use `/v1/api/team?publicHandle=`.
- **`postman-explore-pp-cli team get-workspaces`** - Returns the array of public workspaces owned by a team identified by
its `publicHandle` (e.g., `stripedev`, `salesforce-developers`,
`meta`). Use `/v1/api/networkentity/{id}` for individual entity detail.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
postman-explore-pp-cli category get mock-value

# JSON for scripting and agents
postman-explore-pp-cli category get mock-value --json

# Filter to specific fields
postman-explore-pp-cli category get mock-value --json --select id,name,status

# Dry run — show the request without sending
postman-explore-pp-cli category get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
postman-explore-pp-cli category get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only** - the public discovery surface has no write or delete operations; this CLI never mutates external state
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-postman-explore -g
```

Then invoke `/pp-postman-explore <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-mcp@latest
```

Then register it:

```bash
claude mcp add postman-explore postman-explore-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-postman-explore --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-postman-explore --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-postman-explore skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-postman-explore. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/postman-explore-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle, install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "postman-explore": {
      "command": "postman-explore-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
postman-explore-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/postman-explore-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `networkentity list-network-entities --entity-type collection` or `category list-categories` to see available items

### API-specific

- **HTTP 403 with Cloudflare challenge HTML** — Confirm the binary is using Surf transport (the default in v3). Run `postman-explore-pp-cli doctor --json` to verify; reinstall via `go install github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-cli@latest` if Surf is missing.
- **search or canonical returns empty results after install** — Run `postman-explore-pp-cli sync --all-types` first — those commands query the local store, which is empty until sync populates it.
- **top returns 0 results despite sync** — Confirm `--metric` matches one of: forkCount, monthForkCount, monthViewCount, monthWatchCount, publicViewCount, viewCount, watchCount, weekForkCount, weekViewCount, weekWatchCount. Other names produce empty result sets.
- **browse returns 'Invalid sort type provided'** — Only `--sort popular` is accepted by the proxy. The web UI's other sort options (recent, week, alltime) return HTTP 400 against the API.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://postman.com/explore
- Capture coverage: 17 API entries from 24 total network entries
- Reachability: browser_http (95% confidence)
- Protocols: rest_json (60% confidence), rpc_envelope (95% confidence)
- Auth signals: none
- Protection signals: cloudflare_html_challenge (95% confidence)
- Generation hints: client_pattern:proxy-envelope, uses_browser_http_transport, no_auth_required, skip_clearance_cookie
- Emitted command surface: browse; networkentity get-network-entity; networkentity get-network-entity-counts; category list-categories; category get; team get-workspaces; team get; search-all

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

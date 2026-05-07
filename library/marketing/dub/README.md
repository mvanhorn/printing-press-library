# Dub CLI

**Every Dub feature, plus offline search, agent-native output, and a local SQLite store no other Dub tool has.**

dub-pp-cli covers all 53 Dub API operations as typed commands, then transcends with cross-resource joins the API cannot answer alone — dead-link detection, drift detection, partner leaderboards from /partners/analytics joined with local commissions, bounty submission triage, and a Monday-morning workspace health report. The official dub-cli covers 6 commands; we cover 67.

Learn more at [Dub](https://dub.co/support).

## Install

The recommended path installs both the `dub-pp-cli` binary and the `pp-dub` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install dub
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install dub --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dub-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dub --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dub --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-dub skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-dub. The skill defines how its required CLI can be installed.
```

## Authentication

dub-pp-cli reads DUB_API_KEY from the environment (Speakeasy convention; DUB_TOKEN also accepted for compatibility with prior community CLIs). The key is workspace-scoped — the workspace is implicit in the key. Get one from dub.co/settings/tokens and run `dub-pp-cli doctor` to verify connectivity.

## Quick Start

```bash
# Verify API key, base URL, and rate-limit headroom.
dub-pp-cli doctor

# Populate the local SQLite store so search and transcendence commands work offline.
dub-pp-cli sync

# List the workspace's links with the fields agents care about.
dub-pp-cli links list --json --select id,key,url,clicks

# Find dormant links — the headline transcendence feature.
dub-pp-cli links stale --days 90 --json

# Cross-resource Monday-morning report: rate-limit headroom, expiring links, bounty triage backlog.
dub-pp-cli health --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`links stale`** — Find archived, expired, or zero-traffic links across the workspace before they pile up.

  _Use this to clean up dormant short links before a portfolio review or before bulk-archiving. The /analytics endpoint can't filter links by 'no clicks in N days' in a single call._

  ```bash
  dub-pp-cli links stale --days 90 --json --select id,key,clicks,archived
  ```
- **`links drift`** — Detect links whose click rate dropped more than threshold percent week-over-week.

  _Catches dying campaigns before reporting deadlines. Use this in a weekly automation to surface attribution links that quietly stopped converting._

  ```bash
  dub-pp-cli links drift --window 7d --threshold 30 --json
  ```
- **`links duplicates`** — Find every link in the workspace pointing to the same destination URL.

  _Surfaces accidental duplicates from bulk-create overruns and consolidation candidates after a migration._

  ```bash
  dub-pp-cli links duplicates --json
  ```
- **`links lint`** — Audit short-key slugs for lookalike collisions, reserved-word violations, and brand-conflict hazards.

  _Use this before a brand campaign launch to catch lookalike slugs that confuse partners or get reserved-word treatment._

  ```bash
  dub-pp-cli links lint --json
  ```
- **`links rollup`** — Performance dashboard aggregated by tag or folder — clicks, leads, sales rolled up across every link wearing each label.

  _Use this to compare campaign performance across tag dimensions without reconciling 5 separate API calls._

  ```bash
  dub-pp-cli links rollup --by clicks --group-by tag --json
  ```
- **`funnel`** — Click-to-lead-to-sale conversion rates per link or campaign.

  _Surfaces where prospects drop off in your attribution funnel. Use before quarterly reporting to spot links with high clicks and low conversion._

  ```bash
  dub-pp-cli funnel --link mylink --min-clicks 50 --json
  ```
- **`customers journey`** — See every link a customer clicked, when they became a lead, and when they purchased — in one timeline.

  _Use this for QBR-style account reviews or to debug attribution issues for a specific customer._

  ```bash
  dub-pp-cli customers journey cust_abc123 --json
  ```

### Agent-native plumbing
- **`links rewrite`** — Show every link that would change and the exact patch BEFORE sending.

  _Use this before any campaign-wide rewrite. Diff preview prevents the worst class of bulk-mutation mistakes._

  ```bash
  dub-pp-cli links rewrite --match 'utm_source=oldcampaign' --replace 'utm_source=newcampaign' --dry-run
  ```
- **`health`** — Cross-resource Monday-morning report: rate-limit headroom, expired-but-active links, dead destination URLs, unverified domains, dormant tags, bounty submissions awaiting review.

  _Use this as the first thing every morning, or as a CI canary. Surfaces what needs attention without dashboard hopping._

  ```bash
  dub-pp-cli health --json
  ```
- **`since`** — What happened in the last N hours? Created, updated, deleted links plus partner approvals, new bounty submissions, and top-clicked entities.

  _Use this in agent loops to summarize workspace activity since the last check-in. Cheap and idempotent._

  ```bash
  dub-pp-cli since 24h --json
  ```

### Partner ops
- **`partners leaderboard`** — Rank partners by commission earned, conversion rate, and clicks generated.

  _Use this to identify top performers before a partner-tier review, or dormant partners worth deactivating._

  ```bash
  dub-pp-cli partners leaderboard --by commission --top 10 --json
  ```
- **`partners audit-commissions`** — Reconcile partners, commissions, bounties, and payouts to flag stale rates, missing payouts, and expired bounties still earning.

  _Run this before a payout cycle to catch billing surprises. Use in CI before any commission-rate migration._

  ```bash
  dub-pp-cli partners audit-commissions --json
  ```
- **`bounties triage`** — Group partner-submitted bounty proof by status, age, and bounty type. Surfaces backlog awaiting review.

  _Run weekly to keep bounty submissions from rotting. Bounty programs lose partner trust when submissions sit unreviewed._

  ```bash
  dub-pp-cli bounties triage --status pending --older-than 7d --json
  ```
- **`bounties payout-projection`** — Project upcoming payouts from approved-but-unpaid submissions multiplied by current commission rates.

  _Use this for finance/marketing planning. Surfaces upcoming payout liability before the next payout cycle._

  ```bash
  dub-pp-cli bounties payout-projection --window 30d --json
  ```

## Usage

Run `dub-pp-cli --help` for the full command reference and flag list.

## Commands

### bounties

Manage bounties

### commissions

Manage commissions

- **`dub-pp-cli commissions bulk-update`** - Bulk update up to 100 commissions with the same status.
- **`dub-pp-cli commissions list`** - Retrieve a paginated list of commissions for your partner program.
- **`dub-pp-cli commissions update`** - Update an existing commission amount. This is useful for handling refunds (partial or full) or fraudulent sales.

### customers

Manage customers

- **`dub-pp-cli customers delete`** - Delete a customer from a workspace.
- **`dub-pp-cli customers get`** - Retrieve a paginated list of customers for the authenticated workspace.
- **`dub-pp-cli customers get-id`** - Retrieve a customer by ID for the authenticated workspace. To retrieve a customer by external ID, prefix the ID with `ext_`.
- **`dub-pp-cli customers update`** - Update a customer for the authenticated workspace.

### domains

Manage domains

- **`dub-pp-cli domains check-status`** - Check if a domain name is available for purchase. You can check multiple domains at once.
- **`dub-pp-cli domains create`** - Create a domain for the authenticated workspace.
- **`dub-pp-cli domains delete`** - Delete a domain from a workspace. It cannot be undone. This will also delete all the links associated with the domain.
- **`dub-pp-cli domains list`** - Retrieve a paginated list of domains for the authenticated workspace.
- **`dub-pp-cli domains register`** - Register a domain for the authenticated workspace. Only available for Enterprise Plans.
- **`dub-pp-cli domains update`** - Update a domain for the authenticated workspace.

### dub-analytics

Manage dub analytics

- **`dub-pp-cli dub-analytics retrieve-analytics`** - Retrieve analytics for a link, a domain, or the authenticated workspace. The response type depends on the `event` and `type` query parameters.

### events

Manage events

- **`dub-pp-cli events list`** - Retrieve a paginated list of events for the authenticated workspace.

### folders

Manage folders

- **`dub-pp-cli folders create`** - Create a folder for the authenticated workspace.
- **`dub-pp-cli folders delete`** - Delete a folder from the workspace. All existing links will still work, but they will no longer be associated with this folder.
- **`dub-pp-cli folders list`** - Retrieve a paginated list of folders for the authenticated workspace.
- **`dub-pp-cli folders update`** - Update a folder in the workspace.

### links

Manage links

- **`dub-pp-cli links bulk-create`** - Bulk create up to 100 links for the authenticated workspace.
- **`dub-pp-cli links bulk-delete`** - Bulk delete up to 100 links for the authenticated workspace.
- **`dub-pp-cli links bulk-update`** - Bulk update up to 100 links with the same data for the authenticated workspace.
- **`dub-pp-cli links create`** - Create a link for the authenticated workspace.
- **`dub-pp-cli links delete`** - Delete a link for the authenticated workspace.
- **`dub-pp-cli links get`** - Retrieve a paginated list of links for the authenticated workspace.
- **`dub-pp-cli links get-count`** - Retrieve the number of links for the authenticated workspace.
- **`dub-pp-cli links get-info`** - Retrieve the info for a link.
- **`dub-pp-cli links update`** - Update a link for the authenticated workspace. If there's no change, returns it as it is.
- **`dub-pp-cli links upsert`** - Upsert a link for the authenticated workspace by its URL. If a link with the same URL already exists, return it (or update it if there are any changes). Otherwise, a new link will be created.

### partners

Manage partners

- **`dub-pp-cli partners approve`** - Approve a pending partner application to your program. The partner will be enrolled in the specified group and notified of the approval.
- **`dub-pp-cli partners ban`** - Ban a partner from your program. This will disable all links and mark all commissions as canceled.
- **`dub-pp-cli partners create`** - Creates or updates a partner record (upsert behavior). If a partner with the same email already exists, their program enrollment will be updated with the provided tenantId. If no existing partner is found, a new partner will be created using the supplied information.
- **`dub-pp-cli partners create-link`** - Create a link for a partner that is enrolled in your program.
- **`dub-pp-cli partners deactivate`** - This will deactivate the partner from your program and disable all their active links. Their commissions and payouts will remain intact. You can reactivate them later if needed.
- **`dub-pp-cli partners list`** - List all partners for a partner program.
- **`dub-pp-cli partners list-applications`** - Retrieve a paginated list of pending applications for your partner program.
- **`dub-pp-cli partners reject`** - Reject a pending partner application to your program. The partner will be notified via email that their application was not approved.
- **`dub-pp-cli partners retrieve-analytics`** - Retrieve analytics for a partner within a program. The response type vary based on the `groupBy` query parameter.
- **`dub-pp-cli partners retrieve-links`** - Retrieve a partner's links by their partner ID or tenant ID.
- **`dub-pp-cli partners upsert-link`** - Upsert a link for a partner that is enrolled in your program. If a link with the same URL already exists, return it (or update it if there are any changes). Otherwise, a new link will be created.

### payouts

Manage payouts

- **`dub-pp-cli payouts list`** - Retrieve a paginated list of payouts for your partner program.

### qr

Manage qr

- **`dub-pp-cli qr get-qrcode`** - Retrieve a QR code for a link.

### tags

Manage tags

- **`dub-pp-cli tags create`** - Create a tag for the authenticated workspace.
- **`dub-pp-cli tags delete`** - Delete a tag from the workspace. All existing links will still work, but they will no longer be associated with this tag.
- **`dub-pp-cli tags get`** - Retrieve a paginated list of tags for the authenticated workspace.
- **`dub-pp-cli tags update`** - Update a tag in the workspace.

### tokens

Manage tokens

- **`dub-pp-cli tokens create-referrals-embed`** - Create a referrals embed token for the given partner/tenant. The endpoint first attempts to locate an existing enrollment using the provided tenantId. If no enrollment is found, it resolves the partner by email and creates a new enrollment as needed. This results in an upsert-style flow that guarantees a valid enrollment and returns a usable embed token.

### track

Manage track

- **`dub-pp-cli track lead`** - Track a lead for a short link.
- **`dub-pp-cli track open`** - This endpoint is used to track when a user opens your app via a Dub-powered deep link (for both iOS and Android).
- **`dub-pp-cli track sale`** - Track a sale for a short link.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dub-pp-cli commissions list

# JSON for scripting and agents
dub-pp-cli commissions list --json

# Filter to specific fields
dub-pp-cli commissions list --json --select id,name,status

# Dry run — show the request without sending
dub-pp-cli commissions list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dub-pp-cli commissions list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-dub -g
```

Then invoke `/pp-dub <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/dub/cmd/dub-pp-mcp@latest
```

Then register it:

```bash
claude mcp add dub dub-pp-mcp -e DUB_API_KEY=<your-token>
```

### Remote MCP (Streamable HTTP)

`dub-pp-mcp` also serves over Streamable HTTP for cloud agent hosts. Set
`DUB_MCP_TRANSPORT=http` (and optionally `DUB_MCP_ADDR=:8080`) before
launching:

```bash
DUB_API_KEY=<your-token> DUB_MCP_TRANSPORT=http DUB_MCP_ADDR=:8080 dub-pp-mcp
# Server: Dub MCP server listening (Streamable HTTP) on :8080/mcp
```

The default transport is `stdio`, so existing local installs are unaffected.

### Tool surface modes

`dub-pp-mcp` exposes the API to agents through one of two tool surfaces.
`DUB_MCP_SURFACE` selects which:

- `thin` (default) — two tools, `dub_search` and `dub_execute`, that cover the entire 53-endpoint API. The agent calls `dub_search("create a short link")`, gets a ranked endpoint id back, then calls `dub_execute("links_create", {...})`. Loads ~1K tokens of tool definitions into the agent's catalog instead of ~30K.
- `full` — the 53 typed endpoint-mirror tools (`links_create`, `analytics_retrieve`, etc.). Use when your MCP host benefits from per-endpoint typed argument schemas and your context budget can absorb the catalog.
- `both` — registers both surfaces. Useful if you want to A/B in one server.

Default is `thin` because the agent's tool catalog is loaded every conversation regardless of transport, and the orchestration: code pattern collapses the cost without losing coverage.

Pattern source: Anthropic 2026-04 "Building agents that reach production systems with MCP" (Cloudflare's MCP server uses the same search+execute shape for ~2,500 endpoints in ~1K tokens).

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dub-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DUB_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/dub/cmd/dub-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dub": {
      "command": "dub-pp-mcp",
      "env": {
        "DUB_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
dub-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/dub-pp-cli/config.toml`

Environment variables:
- `DUB_TOKEN`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dub-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DUB_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized: Invalid API key** — Confirm DUB_API_KEY is set and starts with `dub_`. Tokens from dub.co/settings/tokens are workspace-scoped.
- **429 rate_limit_exceeded on /analytics** — Analytics has tighter per-second caps (Pro 2/s, Advanced 8/s). The client retries with backoff; pass `--page-size 50` to reduce throughput.
- **search returns nothing after fresh install** — Run `dub-pp-cli sync` once to populate the local store. Search queries the local FTS index, not the live API.
- **links rewrite blasted too many links** — Always use `--dry-run` first. The diff preview shows every link that would change before any mutation.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**dub-cli (official)**](https://github.com/dubinc/dub-cli) — TypeScript
- [**dubinc/dub-ts**](https://github.com/dubinc/dub-ts) — TypeScript
- [**gitmaxd/dubco-mcp-server-npm**](https://github.com/gitmaxd/dubco-mcp-server-npm) — JavaScript
- [**sujjeee/dubco**](https://github.com/sujjeee/dubco) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

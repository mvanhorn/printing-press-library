# Operon CLI

**Every Operon API endpoint, plus a local demand mirror, auction replay, and trust-history queries that the API does not expose.**

This CLI (binary name: `operon-pp-cli`) is the agent-native CLI and MCP server for Operon, the ad network for AI agents. It wraps the full placement and demand surface with agent-native UX (typed exit codes, JSON/select/compact output modes, dry-run on every mutation), adds three live-composition queries the API does not expose (similar-advertiser lookup, click-chain verification, spec drift detection), and ships a local SQLite store + `sync` command that unlocks seven more transcendence queries: demand stale, demand health, placement replay, placement watch, auction explain, campaign trust-history, and campaign group-by-wallet.

Learn more at [Operon](https://operon.so).

Printed by [@yaooooooooooooooo](https://github.com/yaooooooooooooooo) (yaooooooooooooooo).

## Install

The recommended path installs both the `operon-pp-cli` binary and the `pp-operon` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install operon
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install operon --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/operon-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-operon --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-operon --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-operon skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-operon. The skill defines how its required CLI can be installed.
```

## Authentication

Two auth lanes. For read-only smoke testing and the sandbox lane, set X-Operon-Client to any well-formed UUID via OPERON_CLIENT_UUID or the auto-generated config file. For production placements, set OPERON_API_KEY to a Bearer key issued by Operon ops. The doctor command reports which lane is active.

## Quick Start

```bash
# Verify auth setup, network reachability, and spec contract.
operon-pp-cli doctor


# Sync /demand into the local SQLite store. Prerequisite for the transcendence commands below.
operon-pp-cli sync --json


# Composite freshness + coverage report over the locally synced demand index.
operon-pp-cli demand health --json


# Find advertisers overlapping with adv_changenow on category, serviceType, or assets.
operon-pp-cli demand similar adv_changenow --json


# Decode a stored auction.ranking[] into a sorted readable table.
operon-pp-cli auction explain imp_a1b2c3d4e5f60718 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Live composition
- **`demand similar`** — Find advertisers with overlapping category, assets, and serviceType to a given advertiser.

  _Pre-bid competitive analysis: 'who else is competing for this category?'_

  ```bash
  operon-pp-cli demand similar adv_changenow --json
  ```
- **`click follow`** — Walk the /c/{impressionId} redirect chain, validate the URL scheme, and confirm the final landing matches the advertiser's clickUrl.

  _End-to-end attribution debugging in one command instead of three curl invocations._

  ```bash
  operon-pp-cli click follow imp_a1b2c3d4e5f60718
  ```
- **`spec verify`** — Re-fetch the published OpenAPI spec, compare schemas against what the live API actually returns, flag drift.

  _Catches contract drift the same way the press scorecard would - before downstream integrators do._

  ```bash
  operon-pp-cli spec verify --json
  ```

### Local store
- **`sync`** — Fetch /demand and upsert into a local SQLite store so freshness, replay, watch, and trust-history queries work offline.

  _Prerequisite for every offline-readable transcendence command; turns a stateless API into one with a history._

  ```bash
  operon-pp-cli sync --json
  ```
- **`demand stale`** — List demand entries the local mirror hasn't seen refreshed within the last N hours.

  _Spot churned advertisers before placing campaigns against names that are about to disappear from the pool._

  ```bash
  operon-pp-cli demand stale --hours 48
  ```
- **`demand health`** — Composite freshness + coverage + trust report across the locally synced demand index.

  _Single-call health dashboard for the index; tells you immediately whether a category has zero coverage._

  ```bash
  operon-pp-cli demand health --json
  ```
- **`placement replay`** — Re-issue a previously logged placement request to the live API and diff the new winner, scoutScore, decision, and ranking size against the original.

  _Catches ranking and scoutScore drift between two timepoints — useful when an advertiser keeps losing to the same competitor._

  ```bash
  operon-pp-cli placement replay imp_a1b2c3d4e5f60718
  ```
- **`placement watch`** — Poll the local placement log and stream new auction outcomes as a compact line or JSONL stream.

  _Live tail for in-flight auction outcomes so a publisher can see decisions land in real time._

  ```bash
  operon-pp-cli placement watch --duration 30s --json
  ```
- **`auction explain`** — Decode the auction.ranking[] from a logged placement into a sorted human table (rank, service, score, bid, eligible, reason).

  _Turns an opaque JSON blob into a readable explanation of *why* a given advertiser won (or didn't)._

  ```bash
  operon-pp-cli auction explain imp_a1b2c3d4e5f60718
  ```
- **`campaign trust-history`** — Render a sparkline + table of locally observed trust scores for a campaign or advertiser id.

  _Visual trust audit trail — watch a campaign's score drift up or down over weeks._

  ```bash
  operon-pp-cli campaign trust-history adv_changenow
  ```
- **`campaign group-by-wallet`** — Group locally mirrored campaigns by x402 payer wallet with per-wallet count, total balance, and category list.

  _Spot one advertiser running multiple campaigns from the same wallet (common with agencies)._

  ```bash
  operon-pp-cli campaign group-by-wallet --json
  ```

## Usage

Run `operon-pp-cli --help` for the full command reference and flag list.

## Commands

### c

Manage c

- **`operon-pp-cli c track-click`** - Public 302 redirect endpoint. Logs the click against the impression (with idempotent dedup) and forwards to the winning advertiser's `clickUrl`. Validates the URL scheme defensively and falls back to `https://operon.so` on any error - all error paths are documented as 302 because the endpoint never returns a 4xx (graceful fallback is intentional, so end users mid-click never see an error page).

Not called by integrators directly; this URL is emitted in `placement.clickUrl` for `campaign` and `sandbox_fixture` winners.

### demand

Manage demand

- **`operon-pp-cli demand list`** - Returns the public projection of active production-lane advertisers. The response strips operational fields (`endpoint`, `clickUrl`, `creative`) and commercial terms (`bid`). Sandbox fixtures are never surfaced.

Gated on a well-formed `X-Operon-Client` UUID in production to prevent anonymous scraping by competing networks. Dev mode is permissive.

Rate limits: 30 requests/minute per UUID, 600 requests/minute global ceiling.

### developers

One-time developer registration for the sandbox-to-production graduation path.

- **`operon-pp-cli developers register`** - One-time developer registration. Lifts the caller's UUID from the 100-call/hour sandbox quota to the 1000-call/hour registered quota and queues them for the production demand pool. Invoked by `npx @operon/sdk register` after a successful `npx @operon/sdk test`.

Idempotent: re-registering the same client_uuid returns the same success shape without resending confirmation emails.

Rate limits: per-IP and per-email caps both enforced (1 hour windows).

### operon-ad-network-health

Manage operon ad network health

- **`operon-pp-cli operon-ad-network-health check`** - Returns 200 OK with body `ok` when the server is up. Never returns 4xx; liveness probes that fail return 5xx or connection errors, not structured responses.

### placement

Manage placement

- **`operon-pp-cli placement request`** - Returns a ranked placement (sponsored or blocked) for the current impression. Two auth lanes:

- **Production lane**: pass a Bearer API key in `Authorization`. Quotas: 60 requests/minute per key. Auction draws from the live demand pool.
- **Sandbox lane**: pass a stable UUID in `X-Operon-Client` (no Bearer). Quotas: 60 requests/minute per IP, 100 requests/hour per UUID, plus a global per-replica ceiling. Auction draws from `sandbox_fixture` placeholders only. Used by `npx @operon/sdk test` first-run callers.

When `X-Operon-Client` is present (either lane), the response includes a `_meta` block with sandbox state and a developer-facing message.

### x402

Manage x402

- **`operon-pp-cli x402 cancel-campaign`** - Marks the campaign cancelled and returns unspent USDC to the funding wallet. Bearer token issued at creation. Rate limit: 60/min per IP.
- **`operon-pp-cli x402 create-campaign`** - x402-gated. First call (no `X-PAYMENT` header) returns HTTP 402 with a JSON body containing the payment challenge per the x402 protocol. Construct a `PaymentPayload` from the challenge's `accepts[0]`, base64-encode the JSON, send it as the `X-PAYMENT` request header, and retry. On verification + on-chain settlement the campaign is registered for trust monitoring and enters the quality-weighted auction. Trust score is null at creation and populates as behavioral signal accrues.

Rate limit: 10 requests/minute per IP.

The full advertiser sub-spec including the x402 payment-flow detail lives at `/x402/openapi.json` and is the authoritative reference for the campaign lifecycle.
- **`operon-pp-cli x402 read-campaign`** - Returns current balance, stats, status, and trust score. Bearer token issued at creation. Rate limit: 60/min per IP.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
operon-pp-cli demand

# JSON for scripting and agents
operon-pp-cli demand --json

# Filter to specific fields
operon-pp-cli demand --json --select id,name,status

# Dry run — show the request without sending
operon-pp-cli demand --dry-run

# Agent mode — JSON + compact + no prompts in one flag
operon-pp-cli demand --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-operon -g
```

Then invoke `/pp-operon <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add operon operon-pp-mcp -e OPERON_AD_NETWORK_BEARER_AUTH=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/operon-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OPERON_AD_NETWORK_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "operon": {
      "command": "operon-pp-mcp",
      "env": {
        "OPERON_AD_NETWORK_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
operon-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/operon-ad-network-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OPERON_AD_NETWORK_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `operon-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OPERON_AD_NETWORK_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on /demand** — Set OPERON_CLIENT_UUID to a well-formed UUID v4. Run `operon-pp-cli doctor` to confirm the header is sent.
- **429 Too Many Requests on /placement** — 60/min per Bearer key, 100/hour per UUID in the sandbox lane. Wait the Retry-After interval; use --dry-run to iterate without consuming quota.
- **503 on /developers** — Registration is feature-flagged on the server. Contact Operon support if you hit this in production.
- **spec verify reports endpoint drift** — Spec mismatches are real findings. If drift appears, file an issue with the probe output.
- **`demand stale` / `demand health` reports `No demand entries synced`** — Run `operon-pp-cli sync` to populate the local SQLite store before invoking local-read commands.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**zuckerbot**](https://github.com/DatalisHQ/zuckerbot) — TypeScript
- [**@operon/sdk**](https://www.npmjs.com/package/@operon/sdk) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

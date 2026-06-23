# GoDaddy CLI

Combined CLI for multiple API services

Created by [@zaydiscold](https://github.com/zaydiscold) (zaydiscold).

## Install

The recommended path installs both the `godaddy-pp-cli` binary and the `pp-godaddy` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install godaddy
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install godaddy --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install godaddy --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install godaddy --agent claude-code
npx -y @mvanhorn/printing-press-library install godaddy --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/godaddy/cmd/godaddy-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/godaddy-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install godaddy --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-godaddy --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-godaddy --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install godaddy --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/godaddy-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "godaddy": {
      "command": "godaddy-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
godaddy-pp-cli doctor
```

This checks your configuration.

### 3. Configure Auth

GoDaddy's public APIs use `Authorization: sso-key KEY:SECRET`.

```bash
export GODADDY_API_KEY="..."
export GODADDY_API_SECRET="..."
godaddy-pp-cli doctor --json
```

Advanced users can set `GODADDY_AUTH_HEADER="sso-key KEY:SECRET"` directly or persist `auth_header` in `~/.config/godaddy-pp-cli/config.toml`.

### 4. Try Your First Command

```bash
godaddy-pp-cli agents get mock-value
```

## Usage

Run `godaddy-pp-cli --help` for the full command reference and flag list.

## Commands

### abuse

Manage abuse

- **`godaddy-pp-cli abuse create-ticket`** - Create a new abuse ticket
- **`godaddy-pp-cli abuse create-ticket-v2`** - Create a new abuse ticket
- **`godaddy-pp-cli abuse get-ticket-info`** - Return the abuse ticket data for a given ticket id
- **`godaddy-pp-cli abuse get-ticket-info-v2`** - Return the abuse ticket data for a given ticket id
- **`godaddy-pp-cli abuse get-tickets`** - List all abuse tickets ids that match user provided filters
- **`godaddy-pp-cli abuse get-tickets-v2`** - List all abuse tickets ids that match user provided filters

### aftermarket

Manage aftermarket

- **`godaddy-pp-cli aftermarket add-expiry-listings`** - Add expiry listings into GoDaddy Auction
- **`godaddy-pp-cli aftermarket delete-listings`** - Remove listings from GoDaddy Auction

### agents

Manage agents

- **`godaddy-pp-cli agents get`** - Retrieves detailed information about a registered agent
- **`godaddy-pp-cli agents get-events`** - Returns a paginated, strictly ordered list of ANS events. When providerId is omitted, the API returns events for all providers; when providerId is provided, only events associated with that provider are returned. Pagination is driven by an opaque cursor token returned in each response.
- **`godaddy-pp-cli agents register`** - Registers a new agent with the Agent Name Service. Supports three registration flows:
1. GoDaddy domains with CSRs (synchronous) - Returns 202 immediately with wait instruction
2. External domains with CSRs (async ACME) or BYOC - Returns 202 with validation requirements
   Note: BYOC is permitted for server certificates only. Identity certificates are ALWAYS issued by the RA.

The Registration Authority (RA) validates the agent's identity and submitted
information, then interacts with the CA to issue certificates or validates
provided server certificates.
- **`godaddy-pp-cli agents resolve-ansname`** - Resolves an ANSName to an actionable endpoint reference. The query includes
the ANSName of the target agent and can optionally incorporate capability
filters to refine the search. The ANS service queries the Agent Registry,
which returns a digitally signed endpoint record if found.
- **`godaddy-pp-cli agents search-ansname`** - Searches the Agent Name Service registry using flexible criteria such as
partial agent names, agent host domains, and version ranges. The search
can return multiple matching agents along with their metadata and endpoints.
Results are paginated to handle large datasets efficiently.

### agreements

Manage agreements

- **`godaddy-pp-cli agreements`** - Retrieve Legal Agreements for provided agreements keys

### api-customers

Manage customers

### auctions-aftermarket

Manage aftermarket

- **`godaddy-pp-cli auctions-aftermarket <customerId>`** - Places multiple bids with a single request.

### certificates

Manage certificates

- **`godaddy-pp-cli certificates create`** - <p>Creating a certificate order can be a long running asynchronous operation in the PKI workflow. The PKI API supports 2 options for getting the completion stateful actions for this asynchronous operations: 1) by polling operations -- see /v1/certificates/{certificateId}/actions 2) via WebHook style callback -- see '/v1/certificates/{certificateId}/callback'.</p>
- **`godaddy-pp-cli certificates create-endpoint`** - <p>Creating a certificate order for a subscription can be a long running asynchronous operation in the PKI workflow. The PKI API supports 2 options for getting the completion stateful actions for this asynchronous operations: 1) by polling operations -- see /v1/certificates/{certificateId}/actions 2) via WebHook style callback -- see '/v1/certificates/{certificateId}/callback'.</p>
- **`godaddy-pp-cli certificates download-entitlement`** - Download certificate by entitlement
- **`godaddy-pp-cli certificates get`** - Once the certificate order has been created, this method can be used to check the status of the certificate. This method can also be used to retrieve details of the certificate.
- **`godaddy-pp-cli certificates get-entitlement`** - Once the certificate order has been created, this method can be used to check the status of the certificate. This method can also be used to retrieve details of the certificates associated to an entitlement.
- **`godaddy-pp-cli certificates retrieve-ssl-by-domain-reseller`** - The pagination starts at page 1. Each page contains a page of *subscriptions*, not certificates. This endpoint is meant for paging the subscriptions under the authorized user's account. Each subscription contains a snapshot of certificates contained within the subscription. To fetch further certificates under a subscription, use the /v2/certificates/subscription/{guid} endpoint with the subscription GUID obtained from this call. If any filtering is applied, subscriptions without any certificates will be omitted.
- **`godaddy-pp-cli certificates retrieve-ssl-by-domain-subscription-reseller`** - GET a page of certificates for a specific domain product
- **`godaddy-pp-cli certificates validate`** - Validate a pending order for certificate

### countries

Manage countries

- **`godaddy-pp-cli countries get`** - Retrieves summary country information for the provided marketId and filters
- **`godaddy-pp-cli countries get-country`** - Retrieves country and summary state information for provided countryKey

### customers

Manage customers

- **`godaddy-pp-cli customers auctions get-listings <customerId>`** - Get listings from GoDaddy Auctions

### domains

Manage domains

- **`godaddy-pp-cli domains available`** - Determine whether or not the specified domain is available for purchase
- **`godaddy-pp-cli domains available-bulk`** - Determine whether or not the specified domains are available for purchase
- **`godaddy-pp-cli domains cancel`** - Cancel a purchased domain
- **`godaddy-pp-cli domains contacts-validate`** - All contacts specified in request will be validated against all domains specifed in "domains". As an alternative, you can also pass in tlds, with the exception of `uk`, which requires full domain names
- **`godaddy-pp-cli domains get`** - Retrieve details for the specified Domain
- **`godaddy-pp-cli domains get-agreement`** - Retrieve the legal agreement(s) required to purchase the specified TLD and add-ons
- **`godaddy-pp-cli domains get-maintenances`** - Retrieve the details for an upcoming system Maintenances
- **`godaddy-pp-cli domains get-usage`** - Retrieve api usage request counts for a specific year/month.  The data is retained for a period of three months.
- **`godaddy-pp-cli domains list`** - Retrieve a list of Domains for the specified Shopper
- **`godaddy-pp-cli domains list-maintenances`** - Retrieve a list of upcoming system Maintenances
- **`godaddy-pp-cli domains purchase`** - Purchase and register the specified Domain
- **`godaddy-pp-cli domains schema`** - Retrieve the schema to be submitted when registering a Domain for the specified TLD
- **`godaddy-pp-cli domains suggest`** - Suggest alternate Domain names based on a seed Domain, a set of keywords, or the shopper's purchase history
- **`godaddy-pp-cli domains tlds`** - Retrieves a list of TLDs supported and enabled for sale
- **`godaddy-pp-cli domains update`** - Update details for the specified Domain
- **`godaddy-pp-cli domains validate`** - Validate the request body using the Domain Purchase Schema for the specified TLD

### domains-customers

Manage customers

### orders

Manage orders

- **`godaddy-pp-cli orders get`** - <strong>API Resellers</strong><ul><li>This endpoint does not support subaccounts and therefore API Resellers should not supply an X-Shopper-Id header</li></ul>
- **`godaddy-pp-cli orders list`** - <strong>API Resellers</strong><ul><li>This endpoint does not support subaccounts and therefore API Resellers should not supply an X-Shopper-Id header</li></ul>

### parking

Manage parking

- **`godaddy-pp-cli parking get-metrics`** - Returns a list of parking metrics for the specified customer, using specified filters
- **`godaddy-pp-cli parking get-metrics-by-domain`** - Returns a list of domain metrics for the specified customer and portfolio, using specified filters

### shoppers

Manage shoppers

- **`godaddy-pp-cli shoppers create-subaccount`** - Create a Subaccount owned by the authenticated Reseller
- **`godaddy-pp-cli shoppers delete`** - <strong>Notes:</strong><ul><li>Shopper deletion is not supported in OTE</li><li>**shopperId** is **not the same** as **customerId**.  **shopperId** is a number of max length 10 digits (*ex:* 1234567890) whereas **customerId** is a UUIDv4 (*ex:* 295e3bc3-b3b9-4d95-aae5-ede41a994d13)</li></ul>
- **`godaddy-pp-cli shoppers get`** - <strong>Notes:</strong><ul><li>**shopperId** is **not the same** as **customerId**.  **shopperId** is a number of max length 10 digits (*ex:* 1234567890) whereas **customerId** is a UUIDv4 (*ex:* 295e3bc3-b3b9-4d95-aae5-ede41a994d13)</li></ul>
- **`godaddy-pp-cli shoppers update`** - <strong>Notes:</strong><ul><li>**shopperId** is **not the same** as **customerId**.  **shopperId** is a number of max length 10 digits (*ex:* 1234567890) whereas **customerId** is a UUIDv4 (*ex:* 295e3bc3-b3b9-4d95-aae5-ede41a994d13)</li></ul>

### subscriptions

Manage subscriptions

- **`godaddy-pp-cli subscriptions cancel`** - Cancel the specified Subscription
- **`godaddy-pp-cli subscriptions get`** - Retrieve details for the specified Subscription
- **`godaddy-pp-cli subscriptions list`** - Retrieve a list of Subscriptions for the specified Shopper
- **`godaddy-pp-cli subscriptions product-groups`** - Retrieve a list of ProductGroups for the specified Shopper
- **`godaddy-pp-cli subscriptions update`** - Only Subscription properties that can be changed without immediate financial impact can be modified via PATCH, whereas some properties can be changed by purchasing a renewal<br/><strong>This endpoint only supports JWT authentication</strong>

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
godaddy-pp-cli agents get mock-value

# JSON for scripting and agents
godaddy-pp-cli agents get mock-value --json

# Filter to specific fields
godaddy-pp-cli agents get mock-value --json --select id,name,status

# Dry run — show the request without sending
godaddy-pp-cli agents get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
godaddy-pp-cli agents get mock-value --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
godaddy-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/godaddy-pp-cli/config.toml`

Auth options:

```toml
auth_header = "sso-key KEY:SECRET"
base_url = "https://api.godaddy.com"
```

Environment overrides:

- `GODADDY_API_KEY` + `GODADDY_API_SECRET` build the `sso-key` header.
- `GODADDY_AUTH_HEADER` provides the full `Authorization` value directly.
- `GODADDY_BASE_URL` or `GODADDY_API_BASE_URL` overrides the default production API host.
- `GODADDY_ALLOW_WRITES=1` is required for live account-changing requests. Use `--dry-run` to preview without sending.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

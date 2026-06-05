# Durianpay CLI

**Every Durianpay API in one terminal — legacy and SNAP — with the dual-signature scheme handled for you and a local SQLite store no SDK has.**

Durianpay has no CLI, no MCP server, and no maintained SDK; SNAP signing exists only inside Postman pre-request scripts. This CLI wraps both API generations, mints and caches B2B tokens, signs every SNAP request (RSA-SHA256 + HMAC-SHA512) transparently, and routes payments to the right surface per method. Synced orders, payments, refunds, and disbursements land in SQLite for reconciliation joins the API can't express.

## Install

The recommended path installs both the `durianpay-pp-cli` binary and the `pp-durianpay` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install durianpay
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install durianpay --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install durianpay --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install durianpay --agent claude-code
npx -y @mvanhorn/printing-press-library install durianpay --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/durianpay/cmd/durianpay-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/durianpay-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-durianpay --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-durianpay --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-durianpay skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-durianpay. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/durianpay-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DURIANPAY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/durianpay/cmd/durianpay-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "durianpay": {
      "command": "durianpay-pp-mcp",
      "env": {
        "DURIANPAY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Two credential sets. Legacy: set DURIANPAY_API_KEY (sandbox keys start dp_test_, live dp_live_) — sent as HTTP Basic with the key as username. SNAP additionally needs DURIANPAY_SNAP_CLIENT_KEY, DURIANPAY_SNAP_CLIENT_SECRET, DURIANPAY_SNAP_PRIVATE_KEY (path to your RSA-2048 private key PEM), DURIANPAY_SNAP_CHANNEL_ID, and optionally DURIANPAY_SNAP_PARTNER_ID (defaults to the client key); generate the keypair with 'snap keygen' and upload the public key in Dashboard > Settings > API Keys. Sandbox and live credentials are completely separate — switch with 'durianpay-pp-cli env sandbox' / 'env live'.

## Quick Start

```bash
# Verify config, credentials, and API reachability before anything else
durianpay-pp-cli doctor --dry-run

# Decode a SNAP response code offline — works with zero credentials
durianpay-pp-cli explain 2001800

# First authenticated call: list recent orders (legacy surface)
durianpay-pp-cli orders list --limit 5 --json

# Pull recent activity into the local SQLite store
durianpay-pp-cli sync --resources orders,payments --since 7d

# Join orders against payments locally to surface settlement gaps
durianpay-pp-cli reconcile --since 7d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### SNAP made painless
- **`snap sign`** — Build and inspect both SNAP signatures locally — see the exact string-to-sign, body hash, and headers so a 403 is diagnosed in seconds instead of bisected by hand.

  _Reach for this whenever a SNAP call returns 401/403 — it shows every signing intermediate without touching the API._

  ```bash
  durianpay-pp-cli snap sign --method POST --path /v1.0/transfer-interbank --body @req.json --debug --agent
  ```
- **`snap token`** — Check cached B2B token freshness against the 900-second TTL, and generate the RSA-2048 keypair SNAP onboarding requires with 'snap keygen'.

  _Use during SNAP onboarding or whenever token expiry is a suspected 401 cause._

  ```bash
  durianpay-pp-cli snap token --status --agent
  ```
- **`disbursements verify-completion`** — Recompute the HMAC-SHA256(disbursement_id|amount) completion signature locally and compare it to the webhook's value.

  _Use when validating a disbursement-completion callback before marking a payout settled._

  ```bash
  durianpay-pp-cli disbursements verify-completion --id dis_abc123 --amount 50000.00 --key dp_test_demo --signature 936658ef04244256212e98d13c1059dc606f777fcbfac7fdbdb7bb65f86bd196
  ```

### One CLI, two API generations
- **`pay`** — Charge a payment with the right API surface chosen automatically — SNAP where the method supports it, legacy otherwise, with --surface to override.

  _Pick this over raw charge/snap commands when you want company-policy-correct surface selection per payment method._

  ```bash
  durianpay-pp-cli pay --method qris --amount 50000 --dry-run
  ```
- **`payout`** — Send a disbursement via SNAP transfer-interbank or e-wallet topup with account inquiry first, falling back to legacy batch submit when asked.

  _Use for one-off pay-outs from the terminal; use 'disbursements' commands for legacy batch flows._

  ```bash
  durianpay-pp-cli payout --bank 014 --account 12345678 --amount 100000.00 --dry-run
  ```

### Offline reference that compounds
- **`explain`** — Decode any SNAP response code offline — meaning, originating service, HTTP status, and the likely fix.

  _First stop when a SNAP call returns a non-2xx responseCode._

  ```bash
  durianpay-pp-cli explain 4001801 --agent
  ```
- **`sandbox simulate`** — Print the exact sandbox magic values (even/odd account numbers, magic amounts) that force a success, pending, or failure outcome.

  _Use when writing integration tests against the sandbox to force specific outcomes deterministically._

  ```bash
  durianpay-pp-cli sandbox simulate --scenario invalid-account --method bank-transfer
  ```

### Local state that compounds
- **`reconcile`** — Join synced orders against payments locally to flag charged-but-unsettled orders, payments with no order, and amount mismatches.

  _Run at period close instead of exporting two CSVs and VLOOKUPing them._

  ```bash
  durianpay-pp-cli reconcile --since 7d --agent
  ```
- **`refund-audit`** — Flag refunds that exceed their source payment, target unsettled payments, or duplicate an earlier refund.

  _Use during reconciliation to catch refund anomalies before finance does._

  ```bash
  durianpay-pp-cli refund-audit --since 30d --agent
  ```
- **`stuck`** — List disbursements still pending past a chosen age, bucketed against Durianpay's 2/5/10/90/210-minute webhook retry ladder.

  _Use as the disbursement-ops health check — anything bucketed past the final retry needs manual chase._

  ```bash
  durianpay-pp-cli stuck --older-than 90m --agent
  ```

## Recipes


### Debug a SNAP 403 in one command

```bash
durianpay-pp-cli snap sign --method POST --path /v1.0/balance-inquiry --body @req.json --debug
```

Prints the minified body, its SHA-256 hex, the full string-to-sign, and the resulting X-SIGNATURE so you can see exactly which input diverges.

### Send a payout with account verification

```bash
durianpay-pp-cli payout --bank 014 --account 1234567890 --amount 250000.00 --name "Budi Santoso"
```

Runs SNAP account inquiry first, then transfer-interbank, with the B2B token minted/cached automatically.

### Period-close reconciliation

```bash
durianpay-pp-cli sync --resources orders,payments --since 30d && durianpay-pp-cli reconcile --since 30d --agent
```

Syncs a month of activity then joins orders against payments locally, flagging unsettled and mismatched rows.

### Narrow a deep payment object for an agent

```bash
durianpay-pp-cli payments list --limit 20 --agent --select data.id,data.status,data.amount,data.payment_details_type
```

Payment objects are verbose; --select with dotted paths keeps agent context small.

### Force a sandbox failure case

```bash
durianpay-pp-cli sandbox simulate --scenario invalid-account --method bank-transfer
```

Prints the odd-numbered account value that makes the sandbox reject the transfer, ready to paste into a test.

## Usage

Run `durianpay-pp-cli --help` for the full command reference and flag list.

## Commands

### customers

Manage customers

- **`durianpay-pp-cli customers`** - Patch id

### disbursements

Manage disbursements

- **`durianpay-pp-cli disbursements fetch-by-id`** - Retrieves a disbursement batch status and details (legacy). Returns completion signature when status is completed.
- **`durianpay-pp-cli disbursements submit`** - Submits a disbursement batch (legacy). Supports idempotency via X-Idempotency-Key header. Query force_disburse=true bypasses approval; skip_validation=false validates recipient accounts.

### merchants

Manage merchants

- **`durianpay-pp-cli merchants fetch-sub-account-fees`** - The following endpoint retrieves fees of a single subaccount
- **`durianpay-pp-cli merchants fetch-sub-account-fees-1`** - The following endpoint retrieves balance of a single subaccount
- **`durianpay-pp-cli merchants fetch-subaccount`** - This following endpoint retrieves the details of all created subaccounts
- **`durianpay-pp-cli merchants register-sub-account`** - The following endpoint registers a sub account under your master account.
- **`durianpay-pp-cli merchants update-sub-account-api`** - The following endpoint updates subaccount's informations
- **`durianpay-pp-cli merchants update-sub-account-fees`** - This following endpoint allows master accounts to manage fees for subaccounts

### orders

Manage orders

- **`durianpay-pp-cli orders create`** - The following endpoint creates an Order
- **`durianpay-pp-cli orders fetch`** - The following endpoint retrieves the details of all Orders created
- **`durianpay-pp-cli orders fetch-by-id`** - The following endpoint retrieves the details of a single Order

### payments

Manage payments

- **`durianpay-pp-cli payments charge`** - Charges a payment against an order. The `type` field selects the payment method: VA (virtual account), EWALLET, QRIS, CARD, BNPL, ONLINE_BANKING, RETAIL_STORE. The `request` object shape varies per type; see docs.durianpay.id/reference for per-method bodies.
- **`durianpay-pp-cli payments create-virtual-account`** - The following endpoint creates a new virtual account
- **`durianpay-pp-cli payments fetch-api`** - The following endpoint retrieves the details of all Payments created
- **`durianpay-pp-cli payments fetch-by-id`** - The following endpoint retrieves the details of a single payment
- **`durianpay-pp-cli payments fetch-virtual-account`** - The following endpoint retrieves the details of all Virtual Accounts created
- **`durianpay-pp-cli payments fetch-virtual-account-by-id`** - The following endpoint retrieves the details of a single Virtual Account
- **`durianpay-pp-cli payments virtual-accounts-patch-by-id`** - The following endpoint is used to override Expiry Time of a single Virtual Account.

### refunds

Manage refunds

- **`durianpay-pp-cli refunds create`** - The following endpoint creates an Refund.
- **`durianpay-pp-cli refunds fetch`** - The following endpoint retrieves list of all Refunds created
- **`durianpay-pp-cli refunds fetch-by-id`** - The following endpoint retrieves the details of a single refund
- **`durianpay-pp-cli refunds get-by-payment-id`** - Refund Fetch by Payment ID


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
durianpay-pp-cli customers --id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
durianpay-pp-cli customers --id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
durianpay-pp-cli customers --id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
durianpay-pp-cli customers --id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
durianpay-pp-cli customers --id 550e8400-e29b-41d4-a716-446655440000 --agent
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
durianpay-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/durianpay-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DURIANPAY_API_KEY` | per_call | Yes | Set to your API credential. |
| `DURIANPAY_API_PASSWORD` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `durianpay-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `durianpay-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DURIANPAY_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 DPAY_UNAUTHORIZED_ACCESS on legacy calls** — Check DURIANPAY_API_KEY is set and matches the environment — sandbox keys (dp_test_) fail against live; run 'doctor' to see which env is active
- **403 or invalid-signature on SNAP calls** — Run 'snap sign --method POST --path <path> --body @req.json --debug' to inspect the string-to-sign and both signatures; commonest causes are an unminified body hash, expired B2B token (900s TTL), or the public key not uploaded to the dashboard
- **SNAP token requests rejected** — Confirm DURIANPAY_SNAP_PRIVATE_KEY points at the RSA private key whose public half is uploaded in Dashboard > Settings > API Keys for the SAME environment
- **Duplicate X-EXTERNAL-ID error on SNAP transactions** — X-EXTERNAL-ID must be unique per calendar day; the CLI auto-generates one — pass --external-id only when you intend an idempotent retry of the same logical request
- **reconcile/stuck/refund-audit return no rows** — These read the local store — run 'sync --resources orders,payments,refunds,disbursements --since 30d' first

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**abmid/dpay-sdk-go**](https://github.com/abmid/dpay-sdk-go) — Go (6 stars)
- [**ayatmaulana/durianpay-go-sdk**](https://github.com/ayatmaulana/durianpay-go-sdk) — Go (4 stars)
- [**durianpay/dpay-php**](https://github.com/durianpay/dpay-php) — PHP (2 stars)
- [**stripe/stripe-cli**](https://github.com/stripe/stripe-cli) — Go
- [**Midtrans/SNAP-BI-Signature-Demo**](https://github.com/Midtrans/SNAP-BI-Signature-Demo) — Java

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

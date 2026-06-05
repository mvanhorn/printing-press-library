---
name: pp-durianpay
description: "Every Durianpay API in one terminal — legacy and SNAP — with the dual-signature scheme handled for you and a local SQLite store. Trigger phrases: `charge a payment with durianpay`, `send a durianpay payout`, `debug my SNAP signature`, `check durianpay payment status`, `reconcile durianpay orders and payments`, `use durianpay`, `run durianpay`."
author: "ardihanan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - durianpay-pp-cli
    install:
      - kind: go
        bins: [durianpay-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/durianpay/cmd/durianpay-pp-cli
---

# Durianpay — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `durianpay-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install durianpay --cli-only
   ```
2. Verify: `durianpay-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/durianpay/cmd/durianpay-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Durianpay has no CLI, no MCP server, and no maintained SDK; SNAP signing exists only inside Postman pre-request scripts. This CLI wraps both API generations, mints and caches B2B tokens, signs every SNAP request (RSA-SHA256 + HMAC-SHA512) transparently, and routes payments to the right surface per method. Synced orders, payments, refunds, and disbursements land in SQLite for reconciliation joins the API can't express.

## When to Use This CLI

Use this CLI for Durianpay payment and disbursement operations from the terminal or from AI agents: charging payments, sending pay-outs, checking statuses, minting SNAP tokens, debugging SNAP signatures, simulating sandbox outcomes, and running reconciliation joins over synced local data. It is the right choice whenever a task names Durianpay, SNAP signatures for Durianpay, or Indonesian payment/disbursement ops against api.durianpay.id.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for other Indonesian gateways (Xendit, Midtrans, Doku) — the SNAP signing here is wired to Durianpay's endpoints and credential model
- Do not use it to build checkout UIs — that's the Durianpay web/mobile SDKs' job; this CLI is server-side API operations only
- Do not use it for Durianpay dashboard administration (user management, settlement schedule changes) — those have no public API
- Do not point it at production with --env live for testing — sandbox credentials and 'sandbox simulate' exist for that

## Unique Capabilities

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

## Command Reference

**customers** — Manage customers

- `durianpay-pp-cli customers` — Patch id

**disbursements** — Manage disbursements

- `durianpay-pp-cli disbursements fetch-by-id` — Retrieves a disbursement batch status and details (legacy). Returns completion signature when status is completed.
- `durianpay-pp-cli disbursements submit` — Submits a disbursement batch (legacy). Supports idempotency via X-Idempotency-Key header.

**merchants** — Manage merchants

- `durianpay-pp-cli merchants fetch-sub-account-fees` — The following endpoint retrieves fees of a single subaccount
- `durianpay-pp-cli merchants fetch-sub-account-fees-1` — The following endpoint retrieves balance of a single subaccount
- `durianpay-pp-cli merchants fetch-subaccount` — This following endpoint retrieves the details of all created subaccounts
- `durianpay-pp-cli merchants register-sub-account` — The following endpoint registers a sub account under your master account.
- `durianpay-pp-cli merchants update-sub-account-api` — The following endpoint updates subaccount's informations
- `durianpay-pp-cli merchants update-sub-account-fees` — This following endpoint allows master accounts to manage fees for subaccounts

**orders** — Manage orders

- `durianpay-pp-cli orders create` — The following endpoint creates an Order
- `durianpay-pp-cli orders fetch` — The following endpoint retrieves the details of all Orders created
- `durianpay-pp-cli orders fetch-by-id` — The following endpoint retrieves the details of a single Order

**payments** — Manage payments

- `durianpay-pp-cli payments charge` — Charges a payment against an order.
- `durianpay-pp-cli payments create-virtual-account` — The following endpoint creates a new virtual account
- `durianpay-pp-cli payments fetch-api` — The following endpoint retrieves the details of all Payments created
- `durianpay-pp-cli payments fetch-by-id` — The following endpoint retrieves the details of a single payment
- `durianpay-pp-cli payments fetch-virtual-account` — The following endpoint retrieves the details of all Virtual Accounts created
- `durianpay-pp-cli payments fetch-virtual-account-by-id` — The following endpoint retrieves the details of a single Virtual Account
- `durianpay-pp-cli payments virtual-accounts-patch-by-id` — The following endpoint is used to override Expiry Time of a single Virtual Account.

**refunds** — Manage refunds

- `durianpay-pp-cli refunds create` — The following endpoint creates an Refund.
- `durianpay-pp-cli refunds fetch` — The following endpoint retrieves list of all Refunds created
- `durianpay-pp-cli refunds fetch-by-id` — The following endpoint retrieves the details of a single refund
- `durianpay-pp-cli refunds get-by-payment-id` — Refund Fetch by Payment ID


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
durianpay-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Two credential sets. Legacy: set DURIANPAY_API_KEY (sandbox keys start dp_test_, live dp_live_) — sent as HTTP Basic with the key as username. SNAP additionally needs DURIANPAY_SNAP_CLIENT_KEY, DURIANPAY_SNAP_CLIENT_SECRET, DURIANPAY_SNAP_PRIVATE_KEY (path to your RSA-2048 private key PEM), DURIANPAY_SNAP_CHANNEL_ID, and optionally DURIANPAY_SNAP_PARTNER_ID (defaults to the client key); generate the keypair with 'snap keygen' and upload the public key in Dashboard > Settings > API Keys. Sandbox and live credentials are completely separate — switch with 'durianpay-pp-cli env sandbox' / 'env live'.

Run `durianpay-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  durianpay-pp-cli customers --id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
durianpay-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
durianpay-pp-cli feedback --stdin < notes.txt
durianpay-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/durianpay-pp-cli/feedback.jsonl`. They are never POSTed unless `DURIANPAY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DURIANPAY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
durianpay-pp-cli profile save briefing --json
durianpay-pp-cli --profile briefing customers --id 550e8400-e29b-41d4-a716-446655440000
durianpay-pp-cli profile list --json
durianpay-pp-cli profile show briefing
durianpay-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `durianpay-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/durianpay/cmd/durianpay-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add durianpay-pp-mcp -- durianpay-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which durianpay-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   durianpay-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `durianpay-pp-cli <command> --help`.

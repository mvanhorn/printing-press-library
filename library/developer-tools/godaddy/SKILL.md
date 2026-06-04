---
name: pp-godaddy
description: "Printing Press CLI for GoDaddy. Combined CLI for multiple API services"
author: "zaydiscold"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - godaddy-pp-cli
---

# GoDaddy — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `godaddy-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install godaddy --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `godaddy-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/godaddy/cmd/godaddy-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Command Reference

**abuse** — Manage abuse

- `godaddy-pp-cli abuse create-ticket` — Create a new abuse ticket
- `godaddy-pp-cli abuse create-ticket-v2` — Create a new abuse ticket
- `godaddy-pp-cli abuse get-ticket-info` — Return the abuse ticket data for a given ticket id
- `godaddy-pp-cli abuse get-ticket-info-v2` — Return the abuse ticket data for a given ticket id
- `godaddy-pp-cli abuse get-tickets` — List all abuse tickets ids that match user provided filters
- `godaddy-pp-cli abuse get-tickets-v2` — List all abuse tickets ids that match user provided filters

**aftermarket** — Manage aftermarket

- `godaddy-pp-cli aftermarket add-expiry-listings` — Add expiry listings into GoDaddy Auction
- `godaddy-pp-cli aftermarket delete-listings` — Remove listings from GoDaddy Auction

**agents** — Manage agents

- `godaddy-pp-cli agents get` — Retrieves detailed information about a registered agent
- `godaddy-pp-cli agents get-events` — Returns a paginated, strictly ordered list of ANS events.
- `godaddy-pp-cli agents register` — Registers a new agent with the Agent Name Service. Supports three registration flows: 1.
- `godaddy-pp-cli agents resolve-ansname` — Resolves an ANSName to an actionable endpoint reference.
- `godaddy-pp-cli agents search-ansname` — Searches the Agent Name Service registry using flexible criteria such as partial agent names, agent host domains

**agreements** — Manage agreements

- `godaddy-pp-cli agreements` — Retrieve Legal Agreements for provided agreements keys

**api-customers** — Manage customers


**auctions-aftermarket** — Manage aftermarket

- `godaddy-pp-cli auctions-aftermarket <customerId>` — Places multiple bids with a single request.

**certificates** — Manage certificates

- `godaddy-pp-cli certificates create` — Creating a certificate order can be a long running asynchronous operation in the PKI workflow.
- `godaddy-pp-cli certificates create-endpoint` — Creating a certificate order for a subscription can be a long running asynchronous operation in the PKI workflow.
- `godaddy-pp-cli certificates download-entitlement` — Download certificate by entitlement
- `godaddy-pp-cli certificates get` — Once the certificate order has been created, this method can be used to check the status of the certificate.
- `godaddy-pp-cli certificates get-entitlement` — Once the certificate order has been created, this method can be used to check the status of the certificate.
- `godaddy-pp-cli certificates retrieve-ssl-by-domain-reseller` — The pagination starts at page 1. Each page contains a page of *subscriptions*, not certificates.
- `godaddy-pp-cli certificates retrieve-ssl-by-domain-subscription-reseller` — GET a page of certificates for a specific domain product
- `godaddy-pp-cli certificates validate` — Validate a pending order for certificate

**countries** — Manage countries

- `godaddy-pp-cli countries get` — Retrieves summary country information for the provided marketId and filters
- `godaddy-pp-cli countries get-country` — Retrieves country and summary state information for provided countryKey

**customers** — Manage customers

- `godaddy-pp-cli customers auctions get-listings <customerId>` — Get listings from GoDaddy Auctions

**domains** — Manage domains

- `godaddy-pp-cli domains available` — Determine whether or not the specified domain is available for purchase
- `godaddy-pp-cli domains available-bulk` — Determine whether or not the specified domains are available for purchase
- `godaddy-pp-cli domains cancel` — Cancel a purchased domain
- `godaddy-pp-cli domains contacts-validate` — All contacts specified in request will be validated against all domains specifed in 'domains'.
- `godaddy-pp-cli domains get` — Retrieve details for the specified Domain
- `godaddy-pp-cli domains get-agreement` — Retrieve the legal agreement(s) required to purchase the specified TLD and add-ons
- `godaddy-pp-cli domains get-maintenances` — Retrieve the details for an upcoming system Maintenances
- `godaddy-pp-cli domains get-usage` — Retrieve api usage request counts for a specific year/month. The data is retained for a period of three months.
- `godaddy-pp-cli domains list` — Retrieve a list of Domains for the specified Shopper
- `godaddy-pp-cli domains list-maintenances` — Retrieve a list of upcoming system Maintenances
- `godaddy-pp-cli domains purchase` — Purchase and register the specified Domain
- `godaddy-pp-cli domains schema` — Retrieve the schema to be submitted when registering a Domain for the specified TLD
- `godaddy-pp-cli domains suggest` — Suggest alternate Domain names based on a seed Domain, a set of keywords, or the shopper's purchase history
- `godaddy-pp-cli domains tlds` — Retrieves a list of TLDs supported and enabled for sale
- `godaddy-pp-cli domains update` — Update details for the specified Domain
- `godaddy-pp-cli domains validate` — Validate the request body using the Domain Purchase Schema for the specified TLD

**domains-customers** — Manage customers


**orders** — Manage orders

- `godaddy-pp-cli orders get` — API Resellers This endpoint does not support subaccounts and therefore API Resellers should not supply an X-Shopper-Id
- `godaddy-pp-cli orders list` — API Resellers This endpoint does not support subaccounts and therefore API Resellers should not supply an X-Shopper-Id

**parking** — Manage parking

- `godaddy-pp-cli parking get-metrics` — Returns a list of parking metrics for the specified customer, using specified filters
- `godaddy-pp-cli parking get-metrics-by-domain` — Returns a list of domain metrics for the specified customer and portfolio, using specified filters

**shoppers** — Manage shoppers

- `godaddy-pp-cli shoppers create-subaccount` — Create a Subaccount owned by the authenticated Reseller
- `godaddy-pp-cli shoppers delete` — Notes: Shopper deletion is not supported in OTE **shopperId** is **not the same** as **customerId**.
- `godaddy-pp-cli shoppers get` — Notes: **shopperId** is **not the same** as **customerId**.
- `godaddy-pp-cli shoppers update` — Notes: **shopperId** is **not the same** as **customerId**.

**subscriptions** — Manage subscriptions

- `godaddy-pp-cli subscriptions cancel` — Cancel the specified Subscription
- `godaddy-pp-cli subscriptions get` — Retrieve details for the specified Subscription
- `godaddy-pp-cli subscriptions list` — Retrieve a list of Subscriptions for the specified Shopper
- `godaddy-pp-cli subscriptions product-groups` — Retrieve a list of ProductGroups for the specified Shopper
- `godaddy-pp-cli subscriptions update` — Only Subscription properties that can be changed without immediate financial impact can be modified via PATCH


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
godaddy-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

GoDaddy uses `Authorization: sso-key KEY:SECRET`.

```bash
export GODADDY_API_KEY="..."
export GODADDY_API_SECRET="..."
godaddy-pp-cli doctor --json
```

Advanced options:

- `GODADDY_AUTH_HEADER="sso-key KEY:SECRET"` provides the full authorization value.
- `auth_header = "sso-key KEY:SECRET"` can be persisted in `~/.config/godaddy-pp-cli/config.toml`.
- `GODADDY_BASE_URL` or `GODADDY_API_BASE_URL` overrides the default OTE API host.
- `GODADDY_ALLOW_WRITES=1` is required for live account-changing requests; use `--dry-run` to preview without sending.

Run `godaddy-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  godaddy-pp-cli agents get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
godaddy-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
godaddy-pp-cli feedback --stdin < notes.txt
godaddy-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.godaddy-pp-cli/feedback.jsonl`. They are never POSTed unless `GODADDY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GODADDY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
godaddy-pp-cli profile save briefing --json
godaddy-pp-cli --profile briefing agents get mock-value
godaddy-pp-cli profile list --json
godaddy-pp-cli profile show briefing
godaddy-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `godaddy-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add godaddy-pp-mcp -- godaddy-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which godaddy-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   godaddy-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `godaddy-pp-cli <command> --help`.

---
name: pp-odoo18cli
description: "Printing Press CLI for Odoo18cli. Generic Odoo 18 CLI covering the main business models: sales, purchases, products, partners, manufacturing,..."
author: "Andrea M. Piovesana"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - odoo18cli-pp-cli
---

# Odoo18cli — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `odoo18cli-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install odoo18cli --cli-only
   ```
2. Verify: `odoo18cli-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Generic Odoo 18 CLI covering the main business models: sales, purchases,
products, partners, manufacturing, inventory, and accounting.

Connects to the Odoo XML-RPC external API at /xmlrpc/2/common (auth)
and /xmlrpc/2/object (ORM: search_read, read, create, write).

Supports multiple Odoo instances via profiles:
  odoo18cli --profile edarredo partners list --limit 10

Auth: set ODOO_URL, ODOO_DB, ODOO_USER, ODOO_API_KEY environment variables.
Generate an API key in Odoo via Settings → Users → your user → Account Security.

## Command Reference

**xmlrpc** — Manage xmlrpc

- `odoo18cli-pp-cli xmlrpc authenticate` — Calls common.authenticate(db, username, api_key, {}) via XML-RPC. Returns integer UID used in all subsequent object...
- `odoo18cli-pp-cli xmlrpc confirm-sales-order` — Confirm a quotation into a sales order
- `odoo18cli-pp-cli xmlrpc create-manufacturing-order` — Create a new manufacturing order
- `odoo18cli-pp-cli xmlrpc create-partner` — Create a new partner
- `odoo18cli-pp-cli xmlrpc create-product` — Create a new product template
- `odoo18cli-pp-cli xmlrpc create-purchase-order` — Create a new purchase order (RFQ)
- `odoo18cli-pp-cli xmlrpc create-sales-order` — Create a new sales order
- `odoo18cli-pp-cli xmlrpc get-bom` — Get a single BOM by ID
- `odoo18cli-pp-cli xmlrpc get-invoice` — Get a single invoice or bill by ID
- `odoo18cli-pp-cli xmlrpc get-manufacturing-order` — Get a single manufacturing order by ID
- `odoo18cli-pp-cli xmlrpc get-partner` — Get a single partner by ID
- `odoo18cli-pp-cli xmlrpc get-pricelist` — Get a single pricelist by ID
- `odoo18cli-pp-cli xmlrpc get-product` — Get a single product template by ID
- `odoo18cli-pp-cli xmlrpc get-purchase-order` — Get a single purchase order by ID
- `odoo18cli-pp-cli xmlrpc get-sales-order` — Get a single sales order by ID
- `odoo18cli-pp-cli xmlrpc get-transfer` — Get a single transfer by ID
- `odoo18cli-pp-cli xmlrpc list-accounts` — List chart of accounts
- `odoo18cli-pp-cli xmlrpc list-boms` — List bills of materials
- `odoo18cli-pp-cli xmlrpc list-inventory` — List stock quantities (on-hand inventory)
- `odoo18cli-pp-cli xmlrpc list-invoices` — Calls execute_kw on account.move/search_read. Use domain to filter by move_type (out_invoice, in_invoice,...
- `odoo18cli-pp-cli xmlrpc list-journal-entries` — List journal entry lines
- `odoo18cli-pp-cli xmlrpc list-manufacturing-orders` — List manufacturing orders
- `odoo18cli-pp-cli xmlrpc list-partners` — List partners (customers and/or vendors)
- `odoo18cli-pp-cli xmlrpc list-pricelists` — List customer pricelists
- `odoo18cli-pp-cli xmlrpc list-products` — List product templates
- `odoo18cli-pp-cli xmlrpc list-purchase-orders` — List purchase orders
- `odoo18cli-pp-cli xmlrpc list-sales-orders` — Calls execute_kw on sale.order/search_read. Returns sales orders matching the optional domain filter.
- `odoo18cli-pp-cli xmlrpc list-stock-moves` — List stock moves
- `odoo18cli-pp-cli xmlrpc list-supplier-prices` — List supplier price rules
- `odoo18cli-pp-cli xmlrpc list-transfers` — List stock transfers (pickings)
- `odoo18cli-pp-cli xmlrpc list-workcenters` — List work centers
- `odoo18cli-pp-cli xmlrpc list-workorders` — List work orders
- `odoo18cli-pp-cli xmlrpc update-product` — Update product template fields


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
odoo18cli-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `odoo18cli-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export ODOO_API_KEY="<your-key>"
```

Or persist it in `~/.config/odoo-pp-cli/config.toml`.

Run `odoo18cli-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  odoo18cli-pp-cli xmlrpc authenticate --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
odoo18cli-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
odoo18cli-pp-cli feedback --stdin < notes.txt
odoo18cli-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.odoo18cli-pp-cli/feedback.jsonl`. They are never POSTed unless `ODOO18CLI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ODOO18CLI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
odoo18cli-pp-cli profile save briefing --json
odoo18cli-pp-cli --profile briefing xmlrpc authenticate
odoo18cli-pp-cli profile list --json
odoo18cli-pp-cli profile show briefing
odoo18cli-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `odoo18cli-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add odoo18cli-pp-mcp -- odoo18cli-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which odoo18cli-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   odoo18cli-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `odoo18cli-pp-cli <command> --help`.

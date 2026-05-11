---
name: pp-syndicate-links
description: "Syndicate Links CLI — the agentic commerce rail. Trigger phrases: `list my Syndicate programs`, `create a publisher tracking link`, `mint an attribution token for an agent checkout`, `show pending publisher partnerships`, `report a conversion via Syndicate Links`, `use syndicate-links-pp-cli`, `run sl-cli`."
author: "Syndicate Links"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - syndicate-links-pp-cli
    install:
      - kind: go
        bins: [syndicate-links-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/syndicate-links/cmd/syndicate-links-pp-cli
---

# Syndicate Links — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `syndicate-links-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install syndicate-links --cli-only
   ```
2. Verify: `syndicate-links-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Attribution and commission infrastructure for AI-driven commerce. Two-sided: merchants publish programs + products; publishers (humans and AI agents) create tracking links and earn commissions. The API powers the rail; this CLI is muscle memory for agents who operate on it.

## Command Reference

**affiliate** — Manage affiliate

- `syndicate-links-pp-cli affiliate agent-click` — Requires the affiliate_agent plan (api_click_reporting feature flag). Includes 60-second IP-hash dedup window.
- `syndicate-links-pp-cli affiliate apply-to-program` — Empty body. Auto-approves when the program has autoApprove=true.
- `syndicate-links-pp-cli affiliate attribution-token` — Short-lived HMAC token (not JWT) that an AI agent can present at checkout to bind a conversion. Signed with...
- `syndicate-links-pp-cli affiliate browse-programs` — Public catalog. No auth required.
- `syndicate-links-pp-cli affiliate claim-payout` — Today only method=lightning_invoice is supported. Server decodes BOLT11, validates ±2% sat tolerance, ensures >10...
- `syndicate-links-pp-cli affiliate clicks-report` — Total clicks over a date range
- `syndicate-links-pp-cli affiliate conversions-report` — Aggregate conversion stats over a date range
- `syndicate-links-pp-cli affiliate create-agent-key` — Returns the new aff_agent_* key once. Overwrites any prior agent key.
- `syndicate-links-pp-cli affiliate create-link` — Requires an approved partnership in the target program. If productId is omitted, the API tries to resolve it from...
- `syndicate-links-pp-cli affiliate dashboard-summary` — Composite dashboard summary
- `syndicate-links-pp-cli affiliate earnings-chart` — Daily earnings + conversions over a fixed lookback window
- `syndicate-links-pp-cli affiliate earnings-report` — Daily earnings over a date range
- `syndicate-links-pp-cli affiliate get-balance` — Get commission balance breakdown
- `syndicate-links-pp-cli affiliate get-my-profile` — Get the publisher's profile
- `syndicate-links-pp-cli affiliate get-program` — Public. No auth required.
- `syndicate-links-pp-cli affiliate list-events` — List the publisher's click + conversion events
- `syndicate-links-pp-cli affiliate list-links` — List the publisher's tracking links
- `syndicate-links-pp-cli affiliate list-partnerships` — List the publisher's partnerships
- `syndicate-links-pp-cli affiliate my-partnerships` — List partnerships the publisher has joined
- `syndicate-links-pp-cli affiliate my-payouts` — List the publisher's payouts
- `syndicate-links-pp-cli affiliate program-products` — List products in a program
- `syndicate-links-pp-cli affiliate publisher-register` — Returns the API key once. SL uses 'publisher' externally; 'affiliate' is the API path.
- `syndicate-links-pp-cli affiliate record-click` — Record a click
- `syndicate-links-pp-cli affiliate record-conversion` — attributionMethod + referrerContext (NOT ai_referral / ai_surface flags). AI surface keys go inside referrerContext...
- `syndicate-links-pp-cli affiliate search-products` — Search products across all active programs
- `syndicate-links-pp-cli affiliate update-my-profile` — Allowed: name, website, description, payoutMethod, lightningAddress. Server validates payoutMethod configuration.

**merchant** — Merchant-side endpoints. Auth: mk_live_* key.

- `syndicate-links-pp-cli merchant ai-endorsements-report` — Conversions whose attribution method is agent_token or ai_referral, with disclosure notes for transparency dashboards.
- `syndicate-links-pp-cli merchant approve-partnership` — Note: path param is the partnership ID, not the publisher ID. Counts against plan maxAffiliatesPerProgram.
- `syndicate-links-pp-cli merchant billing` — Get current plan, usage, and tier list
- `syndicate-links-pp-cli merchant bulk-create-products` — Land many products in one program in a single round-trip. CSV import is client-side — parse to JSON, then POST here.
- `syndicate-links-pp-cli merchant create-product` — Create a product
- `syndicate-links-pp-cli merchant create-program` — Counts against the merchant's plan maxPrograms limit.
- `syndicate-links-pp-cli merchant create-webhook` — HMAC secret returned once. Events: click | conversion | payout.completed | affiliate.joined.
- `syndicate-links-pp-cli merchant delete-product` — Delete a product
- `syndicate-links-pp-cli merchant delete-webhook` — Delete a webhook
- `syndicate-links-pp-cli merchant get-company-profile` — Read merchant company profile
- `syndicate-links-pp-cli merchant list-conversions` — List conversion events
- `syndicate-links-pp-cli merchant list-payouts` — List all payouts to publishers
- `syndicate-links-pp-cli merchant list-products` — List products
- `syndicate-links-pp-cli merchant list-programs` — List programs
- `syndicate-links-pp-cli merchant list-publishers` — All partnerships across this merchant's programs. Publishers on paid plans surface first via isPriority.
- `syndicate-links-pp-cli merchant list-webhooks` — List webhooks
- `syndicate-links-pp-cli merchant refund-order` — Full claw-back only — partial refunds are planned but not implemented. Reverses publisher totals and appends a...
- `syndicate-links-pp-cli merchant register` — Returns the API key once — bcrypt-hashed in the DB. No auth required.
- `syndicate-links-pp-cli merchant reject-partnership` — Reject a publisher partnership
- `syndicate-links-pp-cli merchant report` — Aggregate metrics over a date range
- `syndicate-links-pp-cli merchant report-conversion` — Runs attribution match (clickId → trackingCode → cookie), records the event, charges the tracking fee, fires the...
- `syndicate-links-pp-cli merchant settings` — Composite settings endpoint (company + stripe + webhooks)
- `syndicate-links-pp-cli merchant stripe-status` — Currently a stub — always reports disconnected. Stripe-based merchant payouts are planned.
- `syndicate-links-pp-cli merchant update-company-profile` — Update merchant company profile
- `syndicate-links-pp-cli merchant update-product` — Update a product
- `syndicate-links-pp-cli merchant update-program` — Update a program


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
syndicate-links-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Syndicate Links keys come in three shapes. The prefix routes the request server-side:

- `mk_live_*` — merchant key (use for `merchant ...` commands)
- `ak_live_*` / `aff_human_*` — human publisher key (use for `affiliate ...` commands)
- `aff_agent_*` — agent publisher key (agent plan required)

Set via env var (preferred — works in CI and agent runs):

```bash
export SL_API_KEY="mk_live_..."         # or ak_live_*, aff_agent_*
```

Both `SL_API_KEY` (canonical Syndicate Links name) and `SYNDICATE_LINKS_BEARER_AUTH`
(press default) are honored. Either works.

Or persist on disk:

```bash
syndicate-links-pp-cli auth set-token YOUR_TOKEN_HERE
```

Run `syndicate-links-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  syndicate-links-pp-cli affiliate agent-click --tracking-code example-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
syndicate-links-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
syndicate-links-pp-cli feedback --stdin < notes.txt
syndicate-links-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.syndicate-links-pp-cli/feedback.jsonl`. They are never POSTed unless `SYNDICATE_LINKS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SYNDICATE_LINKS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
syndicate-links-pp-cli profile save briefing --json
syndicate-links-pp-cli --profile briefing affiliate agent-click --tracking-code example-value
syndicate-links-pp-cli profile list --json
syndicate-links-pp-cli profile show briefing
syndicate-links-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `syndicate-links-pp-cli --help` output
2. **Starts with `install mcp`** → see MCP Server section below
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server

This Printing Press CLI ships **without** a generated MCP server. Syndicate Links
maintains a separate, hand-tuned MCP server at
[`syndicate-links-mcp`](https://www.npmjs.com/package/syndicate-links-mcp) on
npm — 7 tools shaped specifically for agent commerce flows (program discovery,
attribution token issuance, conversion lookup, balance + payout).

To install the hand-tuned MCP server in Claude Code:

```bash
claude mcp add syndicate-links -- npx -y syndicate-links-mcp
```

Set `SL_API_KEY` in the MCP server's env (same key format as the CLI).


Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which syndicate-links-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   syndicate-links-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `syndicate-links-pp-cli <command> --help`.

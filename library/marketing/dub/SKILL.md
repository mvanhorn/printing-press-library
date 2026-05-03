---
name: pp-dub
description: "Every Dub feature, plus offline search, agent-native output, and a local SQLite store no other Dub tool has. Trigger phrases: `shorten a link with Dub`, `audit my Dub links`, `find dormant Dub links`, `review Dub bounty submissions`, `Dub partner leaderboard`, `use dub-pp-cli`, `run dub-pp-cli`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["dub-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/other/dub/cmd/dub-pp-cli@latest","bins":["dub-pp-cli"],"label":"Install via go install"}]}}'
---

# Dub — Printing Press CLI

dub-pp-cli covers all 53 Dub API operations as typed commands, then transcends with cross-resource joins the API cannot answer alone — dead-link detection, drift detection, partner leaderboards from /partners/analytics joined with local commissions, bounty submission triage, and a Monday-morning workspace health report. The official dub-cli covers 6 commands; we cover 67.

## When to Use This CLI

Use dub-pp-cli when an agent or operator needs to manage Dub at workspace scale — bulk link operations, campaign-wide UTM rewrites, partner program audits, bounty triage, or local cross-resource analysis. The CLI's local SQLite store makes joins across links, analytics, partners, commissions, and bounties cheap and offline. Reach for dub-pp-cli over the official `dub-cli` when you need anything beyond link shortening or workspace config.

## Unique Capabilities

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

## Command Reference

**bounties** — Manage bounties


**commissions** — Manage commissions

- `dub-pp-cli commissions bulk-update` — Bulk update up to 100 commissions with the same status.
- `dub-pp-cli commissions list` — Retrieve a paginated list of commissions for your partner program.
- `dub-pp-cli commissions update` — Update an existing commission amount. This is useful for handling refunds (partial or full) or fraudulent sales.

**customers** — Manage customers

- `dub-pp-cli customers delete` — Delete a customer from a workspace.
- `dub-pp-cli customers get` — Retrieve a paginated list of customers for the authenticated workspace.
- `dub-pp-cli customers get-id` — Retrieve a customer by ID for the authenticated workspace. To retrieve a customer by external ID, prefix the ID with...
- `dub-pp-cli customers update` — Update a customer for the authenticated workspace.

**domains** — Manage domains

- `dub-pp-cli domains check-status` — Check if a domain name is available for purchase. You can check multiple domains at once.
- `dub-pp-cli domains create` — Create a domain for the authenticated workspace.
- `dub-pp-cli domains delete` — Delete a domain from a workspace. It cannot be undone. This will also delete all the links associated with the domain.
- `dub-pp-cli domains list` — Retrieve a paginated list of domains for the authenticated workspace.
- `dub-pp-cli domains register` — Register a domain for the authenticated workspace. Only available for Enterprise Plans.
- `dub-pp-cli domains update` — Update a domain for the authenticated workspace.

**dub-analytics** — Manage dub analytics

- `dub-pp-cli dub-analytics` — Retrieve analytics for a link, a domain, or the authenticated workspace. The response type depends on the `event`...

**events** — Manage events

- `dub-pp-cli events` — Retrieve a paginated list of events for the authenticated workspace.

**folders** — Manage folders

- `dub-pp-cli folders create` — Create a folder for the authenticated workspace.
- `dub-pp-cli folders delete` — Delete a folder from the workspace. All existing links will still work, but they will no longer be associated with...
- `dub-pp-cli folders list` — Retrieve a paginated list of folders for the authenticated workspace.
- `dub-pp-cli folders update` — Update a folder in the workspace.

**links** — Manage links

- `dub-pp-cli links bulk-create` — Bulk create up to 100 links for the authenticated workspace.
- `dub-pp-cli links bulk-delete` — Bulk delete up to 100 links for the authenticated workspace.
- `dub-pp-cli links bulk-update` — Bulk update up to 100 links with the same data for the authenticated workspace.
- `dub-pp-cli links create` — Create a link for the authenticated workspace.
- `dub-pp-cli links delete` — Delete a link for the authenticated workspace.
- `dub-pp-cli links get` — Retrieve a paginated list of links for the authenticated workspace.
- `dub-pp-cli links get-count` — Retrieve the number of links for the authenticated workspace.
- `dub-pp-cli links get-info` — Retrieve the info for a link.
- `dub-pp-cli links update` — Update a link for the authenticated workspace. If there's no change, returns it as it is.
- `dub-pp-cli links upsert` — Upsert a link for the authenticated workspace by its URL. If a link with the same URL already exists, return it (or...

**partners** — Manage partners

- `dub-pp-cli partners approve` — Approve a pending partner application to your program. The partner will be enrolled in the specified group and...
- `dub-pp-cli partners ban` — Ban a partner from your program. This will disable all links and mark all commissions as canceled.
- `dub-pp-cli partners create` — Creates or updates a partner record (upsert behavior). If a partner with the same email already exists, their...
- `dub-pp-cli partners create-link` — Create a link for a partner that is enrolled in your program.
- `dub-pp-cli partners deactivate` — This will deactivate the partner from your program and disable all their active links. Their commissions and payouts...
- `dub-pp-cli partners list` — List all partners for a partner program.
- `dub-pp-cli partners list-applications` — Retrieve a paginated list of pending applications for your partner program.
- `dub-pp-cli partners reject` — Reject a pending partner application to your program. The partner will be notified via email that their application...
- `dub-pp-cli partners retrieve-analytics` — Retrieve analytics for a partner within a program. The response type vary based on the `groupBy` query parameter.
- `dub-pp-cli partners retrieve-links` — Retrieve a partner's links by their partner ID or tenant ID.
- `dub-pp-cli partners upsert-link` — Upsert a link for a partner that is enrolled in your program. If a link with the same URL already exists, return it...

**payouts** — Manage payouts

- `dub-pp-cli payouts` — Retrieve a paginated list of payouts for your partner program.

**qr** — Manage qr

- `dub-pp-cli qr` — Retrieve a QR code for a link.

**tags** — Manage tags

- `dub-pp-cli tags create` — Create a tag for the authenticated workspace.
- `dub-pp-cli tags delete` — Delete a tag from the workspace. All existing links will still work, but they will no longer be associated with this...
- `dub-pp-cli tags get` — Retrieve a paginated list of tags for the authenticated workspace.
- `dub-pp-cli tags update` — Update a tag in the workspace.

**tokens** — Manage tokens

- `dub-pp-cli tokens` — Create a referrals embed token for the given partner/tenant. The endpoint first attempts to locate an existing...

**track** — Manage track

- `dub-pp-cli track lead` — Track a lead for a short link.
- `dub-pp-cli track open` — This endpoint is used to track when a user opens your app via a Dub-powered deep link (for both iOS and Android).
- `dub-pp-cli track sale` — Track a sale for a short link.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dub-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find dormant links worth archiving

```bash
dub-pp-cli sync && dub-pp-cli links stale --days 60 --json --select key,url,clicks,createdAt
```

Sync first, then ask for links with zero clicks in the last 60 days. Pipe through `jq` to filter further or build an archive batch.

### Preview a campaign-wide UTM swap before sending

```bash
dub-pp-cli links rewrite --match 'utm_source=launch' --replace 'utm_source=summer' --dry-run --json
```

The diff preview shows every link that would change. Drop `--dry-run` only after reading the patch.

### Triage bounty submissions waiting more than 7 days

```bash
dub-pp-cli bounties triage --status pending --older-than 7d --json --select id,partnerId,bountyType,submittedAt
```

Surfaces the partner submissions rotting in the queue. Pipe through `jq` to group by `partnerId` for outreach lists.

### Per-partner ROI for the top 10 partners

```bash
dub-pp-cli partners leaderboard --by commission --top 10 --agent --select 'partnerId,partnerName,clicks,leads,sales,commission'
```

Combines /partners/analytics with local commissions and payouts. Use `--select` with dotted paths to keep the response small for agents — leaderboards return wide rows by default.

### Sync, then run the full Monday-morning health check

```bash
dub-pp-cli sync && dub-pp-cli health --json
```

Combined report across rate-limit headroom, expired-but-active links, dead destination URLs, unverified domains, and bounty submissions awaiting review.

## Auth Setup

dub-pp-cli reads DUB_API_KEY from the environment (Speakeasy convention; DUB_TOKEN also accepted for compatibility with prior community CLIs). The key is workspace-scoped — the workspace is implicit in the key. Get one from dub.co/settings/tokens and run `dub-pp-cli doctor` to verify connectivity.

Run `dub-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dub-pp-cli commissions list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

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
dub-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
dub-pp-cli feedback --stdin < notes.txt
dub-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.dub-pp-cli/feedback.jsonl`. They are never POSTed unless `DUB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DUB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
dub-pp-cli profile save briefing --json
dub-pp-cli --profile briefing commissions list
dub-pp-cli profile list --json
dub-pp-cli profile show briefing
dub-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `dub-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/dub/cmd/dub-pp-cli@latest
   ```
3. Verify: `dub-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/dub/cmd/dub-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add dub-pp-mcp -- dub-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dub-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dub-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dub-pp-cli <command> --help`.

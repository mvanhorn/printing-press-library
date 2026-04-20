---
name: pp-dub
description: "Use this skill whenever the user wants to create, manage, or analyze short links; track click analytics; manage domains; generate QR codes; run affiliate/partner programs; handle commissions or payouts; or work with the Dub link-management platform. Dub CLI covering links, analytics, domains, QR codes, folders, tags, partners, bounties, commissions, and payouts. Requires a Dub API token. Triggers on phrasings like 'create a short link for this URL', 'how many clicks did my campaign get last week', 'generate a QR code for this link', 'pay out partners for March', 'which links are my top performers'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["dub-pp-cli"],"env":["DUB_API_KEY"]},"primaryEnv":"DUB_API_KEY","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@latest","bins":["dub-pp-cli"],"label":"Install via go install"}]}}'
---

# Dub — Printing Press CLI

Create short links, track analytics, manage domains, and run affiliate/partner programs via the Dub API. Full coverage of links, analytics, domains, QR codes, folders, tags, partners, bounties, commissions, and payouts. Agent-native output on every command; offline SQLite sync for bulk analytics.

## When to Use This CLI

Reach for this when the user wants to:

- Create or update short links (marketing campaigns, affiliate tracking, one-off redirects)
- Analyze link performance (clicks, unique visitors, geographic breakdown, referrer data)
- Manage an affiliate or partner program (bounties, commissions, payouts, leaderboards)
- Bulk-ship links for a launch or campaign
- Generate QR codes for print / physical campaigns

Don't reach for this if the user just needs a one-off random short link without tracking — a no-auth service is faster. Dub earns its complexity when the user actually wants the analytics and partner-program layers.

## Unique Capabilities

These commands reward the combination of link management + partner ops + analytics.

### Bulk operations

- **`links bulk-create`** / **`bulk-update`** / **`bulk-delete`** — Atomic multi-link operations. Handles transactional rollback on partial failure.

  _One command to ship 500 links for a campaign. Dub's API has bulk endpoints; this surfaces them as first-class CLI commands._

- **`links duplicates`** / **`stale`** — Find duplicate destinations or links nobody's clicked in N days. Inventory-hygiene tools that don't exist in the web dashboard.

### Analytics slicing

- **`analytics`** — Multi-dimensional analytics with flexible `--group-by` (country, device, referrer, tag, folder, date).

- **`events`** — Raw click event stream. Pipe into jq for custom slicing.

- **`tags analytics`** — Aggregate analytics for every link with a given tag.

- **`domains report`** — Per-domain performance summary.

- **`customers journey`** — Customer-journey analytics: what path did a unique visitor take through your links.

- **`partners leaderboard`** — Top-performing partners by conversions.

- **`funnel`** — Conversion funnel from click → signup → purchase (when conversion tracking is enabled).

### Partner program ops

- **`bounties`** — Create, list, approve, reject bounty submissions.

- **`commissions`** — Track commissions per partner.

- **`payouts`** — Run payouts to partners (preview with `--dry-run`).

- **`partners`** — Partner CRUD + ban/unban.

- **`campaigns`** — Campaign management (groups partners + links + bounties).

### Integration utility

- **`qr`** — Generate QR codes for any short link (PNG or SVG).

- **`track`** — Ingest custom conversion events from external sources.

- **`folders`** / **`tags`** — Organizational primitives for large link portfolios.

- **`search`** — Full-text search across synced data.

## Command Reference

Links:

- `dub-pp-cli links` — List / get / update
- `dub-pp-cli links create` — Create a single link
- `dub-pp-cli links bulk-create | bulk-update | bulk-delete` — Bulk ops
- `dub-pp-cli links duplicates` / `stale` — Hygiene
- `dub-pp-cli qr <linkId>` — QR code generation

Analytics:

- `dub-pp-cli analytics [--group-by …]` — Multi-dim analytics
- `dub-pp-cli events` — Raw event stream
- `dub-pp-cli funnel` — Conversion funnel
- `dub-pp-cli tags analytics` — Tag-scoped analytics
- `dub-pp-cli domains report` — Per-domain report
- `dub-pp-cli customers journey` — Per-customer path

Partners / Commissions:

- `dub-pp-cli partners [leaderboard|ban|approve]`
- `dub-pp-cli bounties [list|approve-bounty|ban]`
- `dub-pp-cli commissions`
- `dub-pp-cli payouts [--dry-run]`
- `dub-pp-cli campaigns`

Organization:

- `dub-pp-cli folders` — Folder CRUD
- `dub-pp-cli tags` — Tag CRUD
- `dub-pp-cli domains` — Domain management

Auth / utility:

- `dub-pp-cli auth set-token <DUB_API_KEY>`
- `dub-pp-cli tokens` — Manage API tokens (admin)
- `dub-pp-cli sync` / `export` / `import` / `archive` — Local store
- `dub-pp-cli search <query>` — Full-text search
- `dub-pp-cli doctor` — Verify
- `dub-pp-cli tail` — Live event log

## Recipes

### Ship 200 links for a launch

```bash
cat launch-links.jsonl | dub-pp-cli links bulk-create --domain dub.sh --agent
```

One atomic request, transactional rollback if any link fails validation. Much faster than 200 individual creates.

### Weekly campaign analytics

```bash
# Analytics retrieve with a time window and group-by dimension:
dub-pp-cli analytics retrieve --interval 7d --group-by country --agent
dub-pp-cli analytics retrieve --interval 7d --group-by device --agent

# Tag-level rollup aggregates EVERY tag — filter client-side for "q4-launch":
dub-pp-cli tags analytics --agent | jq '.[] | select(.tag_name=="q4-launch")'
```

`analytics retrieve` is the API-backed analytics call with `--interval` (time window) and `--group-by` (country, device, referrer, etc.). `tags analytics` is a local-DB rollup over all tags — jq filters to the campaign tag. The top-level `dub-pp-cli analytics` command is a different thing: it summarizes locally-synced data and uses `--type`, not `--interval`.

### Partner program month-end

```bash
dub-pp-cli commissions --interval 30d --agent          # what's owed (last 30 days)
dub-pp-cli payouts --status pending --agent            # list pending payouts
dub-pp-cli payouts create --partner-id <id> --agent    # create a payout (per partner)
dub-pp-cli partners leaderboard --sort earnings --limit 25 --agent
```

`commissions` supports `--interval` for time-windowing. `payouts` is scoped by status or partner; the leaderboard is aggregated from synced data and ranks by earnings/clicks/sales/commissions/name. For bulk previews, use `--dry-run` (universal) on mutation commands.

### Find stale links eating domain budget

```bash
dub-pp-cli links stale --days 90 --agent
dub-pp-cli links bulk-delete --link-ids "$(dub-pp-cli links stale --days 90 --agent | jq -r '.[].id' | paste -sd, -)" --dry-run
```

Identify links nobody's clicked in 90 days, then preview a bulk delete (`--link-ids` is a comma-separated list, max 100 per call). Domain-hygiene task that's hard to do in the web UI.

## Auth Setup

Requires a Dub API token.

```bash
# Get a token: https://app.dub.co/settings/tokens
export DUB_API_KEY="dub_xxx"
dub-pp-cli auth set-token "$DUB_API_KEY"
dub-pp-cli doctor
```

Optional:
- `DUB_BASE_URL` — override API base (for self-hosted or region-specific endpoints)
- `DUB_WORKSPACE` — default workspace slug

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes`. Universal flags: `--select`, `--dry-run`, `--rate-limit N`, `--no-cache`.

Flag glossary:
- `--data-source auto|live|local` — read from live API, local sync, or let the CLI decide
- `--interval <duration>` — analytics time window (7d, 30d, etc.)
- `--group-by <field>` — analytics aggregation dimension

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found (link, partner, domain) |
| 4 | Auth required |
| 5 | API error |
| 7 | Rate limited |
| 10 | Config error |

## Installation

### CLI

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@main
dub-pp-cli auth set-token YOUR_DUB_API_KEY
dub-pp-cli doctor
```

### MCP Server

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-mcp@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-mcp@main
claude mcp add -e DUB_API_KEY=<key> dub-pp-mcp -- dub-pp-mcp
```

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → run `dub-pp-cli --help`
2. **`install`** → CLI; **`install mcp`** → MCP
3. **Anything else** → check `which dub-pp-cli` (install if missing), verify `DUB_API_KEY` is set (prompt for setup if not), route by intent: short-link creation → `links create`; analytics lookup → `analytics` or `tags analytics`; partner ops → `partners` / `commissions` / `payouts`. Run with `--agent`.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
dub-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
dub-pp-cli --profile <name> <command>

# List / inspect / remove
dub-pp-cli profile list
dub-pp-cli profile show <name>
dub-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
dub-pp-cli <command> --deliver file:/path/to/out.json
dub-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
dub-pp-cli feedback "what surprised you or tripped you up"
dub-pp-cli feedback list         # show local entries
dub-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.dub-pp-cli/feedback.jsonl` as JSON lines. When `DUB_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `DUB_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).


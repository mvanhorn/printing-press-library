---
name: pp-memberstack
description: "The first CLI for Memberstack — every Admin API endpoint plus stale-member detection, plan coverage, JWT decode Trigger phrases: `list memberstack members`, `find stale memberstack members`, `decode a memberstack jwt`, `export memberstack members to csv`, `verify a memberstack token`, `use memberstack`, `run memberstack-pp-cli`."
author: "Anton Sahibzada"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - memberstack-pp-cli
    install:
      - kind: go
        bins: [memberstack-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/auth/memberstack/cmd/memberstack-pp-cli
---

# Memberstack — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `memberstack-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install memberstack --cli-only
   ```
2. Verify: `memberstack-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/auth/memberstack/cmd/memberstack-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Memberstack ships an SDK and an MCP server, but no command-line tool. memberstack-pp-cli covers every Admin REST endpoint with agent-native --json output, then adds a local SQLite mirror that powers stale-member detection, plan-coverage pivots, custom-fields flatten, drift audit, and one-shot snapshot export — things no dashboard, SDK, or MCP can do. Writable via the CLI: member emails, custom-field values, metaData, plan-connections, and custom data records. Dashboard-only: defining new custom field names, creating plans, and creating new data tables (the REST API does not expose these).

## When to Use This CLI

Reach for memberstack-pp-cli whenever a Memberstack task is scriptable: bulk member operations, custom-fields exports for BI, sandbox cleanup, JWT inspection during frontend debugging, ad-hoc analytics on plan coverage or signup velocity, or backups and GDPR exports. Use it instead of writing a Node script when the workflow is one-shot or short. Keep the MCP server for long-running agent conversations that need natural-language access.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`stale`** — Find members whose lastLogin is older than N days.

  _When an agent needs to find dormant accounts for cleanup or re-engagement campaigns, this is the single command to reach for. The dashboard and MCP cannot filter by lastLogin server-side._

  ```bash
  memberstack-pp-cli stale --days 30 --json --select id,auth.email,lastLogin
  ```
- **`plan-coverage`** — Pivot active plan-connections across all members; flag members with zero active plans.

  _Agents auditing 'which members have access to which plans' should call this once instead of N member-get calls._

  ```bash
  memberstack-pp-cli plan-coverage --agent
  ```
- **`fields flatten`** — Pivot every member's customFields map into a flat table (CSV or JSON).

  _Agents producing BI or marketing exports should not have to hand-write the pivot; one command produces the spreadsheet shape._

  ```bash
  memberstack-pp-cli fields flatten --csv > members.csv
  ```
- **`audit`** — Diff the local snapshot against live to surface what changed since the last sync.

  _Agents validating a deploy or investigating support tickets should call this to see what mutated._

  ```bash
  memberstack-pp-cli audit --sample 100 --json
  ```

### Agent-native plumbing
- **`token decode`** — Decode a Memberstack JWT locally without calling the API.

  _When an agent needs to inspect a JWT during frontend debugging, this is faster than verify and works without the secret key._

  ```bash
  memberstack-pp-cli token decode eyJhbGciOi...
  ```
- **`bulk delete`** — Wipe members matching a SQL WHERE clause, with --dry-run preview.

  _Agents asked to clean up sandbox test accounts should call this once. Always preview first; explicit --apply required to commit._

  ```bash
  memberstack-pp-cli bulk delete --where "auth_email LIKE '%@test.local'" --dry-run
  ```
- **`watch`** — Poll the cursor and print new members as they arrive (live tail).

  _When an agent needs to react to new signups (alerting, enrichment, Slack notify), pipe this through jq instead of building a webhook receiver._

  ```bash
  memberstack-pp-cli watch --since 5m --json
  ```
- **`records find`** — Build the Prisma findMany payload from --where, --order-by, --limit flags.

  _Agents querying custom data tables get the same expressiveness without constructing JSON envelopes._

  ```bash
  memberstack-pp-cli records find products --where 'inStock=true' --order-by price:asc --limit 10
  ```

### Reachability mitigation
- **`plans list`** — Build a plans index from planConnections observed across sync, with (planId, member-count, lastSeen).

  _When an agent needs to know which plan IDs are in use without opening the dashboard, this is the shortcut._

  ```bash
  memberstack-pp-cli plans list --json
  ```

## Command Reference

**data-tables** — List and inspect custom data table schemas

- `memberstack-pp-cli data-tables get` — Get a single data table schema
- `memberstack-pp-cli data-tables list` — List custom data tables

**members** — Member CRUD, JWT verification, and plan-connection helpers

- `memberstack-pp-cli members create` — Create a member
- `memberstack-pp-cli members delete` — Delete a member
- `memberstack-pp-cli members get` — Look up by Memberstack ID (mem_*) or URL-encoded email.
- `memberstack-pp-cli members list` — Paginated list of members. Cursor is the numeric `endCursor` from the previous response.
- `memberstack-pp-cli members update` — Update a member
- `memberstack-pp-cli members verify-token` — Validates a Memberstack-issued JWT server-side and returns the decoded claims.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
memberstack-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find and clean up dormant test accounts

```bash
memberstack-pp-cli stale --days 60 --json --select id,auth.email | jq -r '.[].id' | xargs -I{} memberstack-pp-cli members delete {} --dry-run
```

Dry-run preview; remove --dry-run when you are ready to commit.

### Export every member with custom fields flattened to CSV

```bash
memberstack-pp-cli fields flatten --csv > members-$(date +%F).csv
```

One row per member; one column per custom field plus auth.email, planConnections, lastLogin.

### Watch new signups

```bash
memberstack-pp-cli watch --since 1h --agent --select id,auth.email,createdAt
```

Cursor-polls the API; one JSON object per new member, pipeable to jq or any other notifier.

### Find members with no active plan

```bash
memberstack-pp-cli plan-coverage --agent --select id,auth.email,activePlanCount | jq 'map(select(.activePlanCount == 0))'
```

Identifies free-tier members who should be re-engaged or pruned.

### Query a custom data table without building a Prisma envelope

```bash
memberstack-pp-cli records find products --where 'inStock=true,price>=20' --order-by price:asc --limit 10 --agent --select records.id,records.data.name,records.data.price
```

Shorthand flags compile to the Prisma findMany payload the API expects.

## Auth Setup

Authenticate by exporting MEMBERSTACK_SECRET_KEY (sk_sb_* for sandbox / test mode, sk_* or sk_live_* for live mode). Keys come from Memberstack > Dev Tools. The CLI never reads the browser-side public key (pk_*) — that is for the DOM SDK only. Rate limit is 25 requests/sec; the sync command paces itself automatically.

Run `memberstack-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  memberstack-pp-cli data-tables list --agent --select id,name,status
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
memberstack-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
memberstack-pp-cli feedback --stdin < notes.txt
memberstack-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/memberstack-pp-cli/feedback.jsonl`. They are never POSTed unless `MEMBERSTACK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MEMBERSTACK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
memberstack-pp-cli profile save briefing --json
memberstack-pp-cli --profile briefing data-tables list
memberstack-pp-cli profile list --json
memberstack-pp-cli profile show briefing
memberstack-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `memberstack-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/auth/memberstack/cmd/memberstack-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add memberstack-pp-mcp -- memberstack-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which memberstack-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   memberstack-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `memberstack-pp-cli <command> --help`.

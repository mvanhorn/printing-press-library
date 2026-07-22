---
name: pp-copper
description: "The Copper CRM command line no one else built: full CRUD plus a local database, weighted pipeline forecasting, stale-deal detection, and the bulk operations Copper's own API refuses to provide. Trigger phrases: `forecast my copper pipeline`, `find stale deals in copper`, `bulk update copper opportunities`, `log a call in copper`, `dedupe copper contacts`, `use copper`, `run copper`."
author: "Kerry Morrison"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - copper-pp-cli
    install:
      - kind: go
        bins: [copper-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/copper/cmd/copper-pp-cli
---

# Copper — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `copper-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install copper --cli-only
   ```
2. Verify: `copper-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/copper/cmd/copper-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Copper has no CLI, no Go client, and no agent-native tool. This turns a click-heavy web CRM into a scriptable, offline-queryable surface. It mirrors people, companies, leads, opportunities, projects, tasks, and activities into local SQLite, then adds the weighted forecast (forecast), cold-deal sweep (stale), and rate-limit-aware bulk editor (bulk) that the API and web UI leave out.

## When to Use This CLI

Use this CLI for scripted or agent-driven Copper work: weighted pipeline forecasting, finding and acting on stale deals, mass field updates across many opportunities, idempotent contact sync, and offline cross-entity queries. It is the right tool when the web UI would mean repetitive clicking or when a spreadsheet export would be stale on arrival.

## Anti-triggers

Do not use this CLI for:
- Configuring Copper account settings, billing, or user provisioning (use the web app)
- Editing a logged activity in place (Copper activities are immutable; use log fix to delete+recreate)
- OAuth-based multi-tenant distribution to other Copper orgs (partner-gated; this CLI uses personal API-key auth)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Pipeline intelligence
- **`forecast`** — Weighted expected-revenue roll-up: sums monetary_value x win_probability over open opportunities, grouped by stage, assignee, or close-month.

  _Reach for this instead of exporting CSV and pivoting by hand for any expected-revenue or commit/quota question._

  ```bash
  copper-pp-cli forecast --pipeline 12345 --by stage --agent
  ```
- **`stale`** — Surfaces open opportunities with no interaction in N days, sorted by staleness x value, across all reps.

  _Use to find deals going cold before they die; pipe the output into bulk reassign._

  ```bash
  copper-pp-cli stale --days 21 --by assignee --agent
  ```
- **`who`** — Joins an opportunity to its company, people, and recent activities into one related-records view.

  _Use for one-call deal prep before nudging or logging a touch._

  ```bash
  copper-pp-cli who opportunity:88 --agent
  ```

### Operations Copper's API refuses to provide
- **`bulk`** — Applies field updates (stage, owner, custom fields) across many opportunities with bounded concurrency and heuristic 429 backoff.

  _Use for mass updates to existing records; the safe way to avoid the rate-limit wall a naive loop hits._

  ```bash
  copper-pp-cli bulk move --query stale.json --set pipeline_stage_id=9 --concurrency 4 --dry-run
  ```
- **`upsert`** — Match-then-create-or-update for people and leads; normalizes the people.emails[] vs leads.email shape difference.

  _Use to sync external rows without creating duplicates; not a blind create._

  ```bash
  copper-pp-cli upsert person --match email --file contacts.json --dry-run
  ```
- **`dedupe`** — Local SQLite self-join surfacing people or leads that share an email, name, or company.

  _Run before or after a sync to catch duplicate contacts the API will not flag._

  ```bash
  copper-pp-cli dedupe people --on email --agent
  ```

### CRM hygiene
- **`log`** — Creates an activity with the type resolved by name (bumps interaction_count); log fix deletes and recreates to edit an immutable activity.

  _Use to log or correct a single touch; for the same touch across many records use bulk._

  ```bash
  copper-pp-cli log call --on opportunity:88 --note "Left voicemail"
  ```

## Command Reference

**account** — Account details

- `copper-pp-cli account` — Fetch account details

**activities** — Manage activities (notes and logged interactions)

- `copper-pp-cli activities create` — Create a new activity
- `copper-pp-cli activities delete` — Delete an activity
- `copper-pp-cli activities get` — Fetch an activity by id
- `copper-pp-cli activities search` — List/search activities

**activity-types** — Activity types

- `copper-pp-cli activity-types` — List all activity types

**companies** — Manage companies

- `copper-pp-cli companies activities` — List a company's activities
- `copper-pp-cli companies create` — Create a new company
- `copper-pp-cli companies delete` — Delete a company
- `copper-pp-cli companies get` — Fetch a company by id
- `copper-pp-cli companies search` — List/search companies
- `copper-pp-cli companies update` — Update a company

**contact-types** — Contact types

- `copper-pp-cli contact-types` — List all contact types

**custom-activity-types** — Custom activity types

- `copper-pp-cli custom-activity-types create` — Create a new custom activity type
- `copper-pp-cli custom-activity-types get` — Fetch a custom activity type by id
- `copper-pp-cli custom-activity-types list` — List all custom activity types
- `copper-pp-cli custom-activity-types update` — Update a custom activity type

**custom-field-definitions** — Custom field definitions

- `copper-pp-cli custom-field-definitions create` — Create a new custom field definition
- `copper-pp-cli custom-field-definitions delete` — Delete a custom field definition
- `copper-pp-cli custom-field-definitions get` — Fetch a custom field definition by id
- `copper-pp-cli custom-field-definitions list` — List all custom field definitions
- `copper-pp-cli custom-field-definitions update` — Update a custom field definition

**customer-sources** — Customer (lead) sources

- `copper-pp-cli customer-sources` — List all customer sources

**lead-statuses** — Lead statuses

- `copper-pp-cli lead-statuses` — List all lead statuses

**leads** — Manage leads

- `copper-pp-cli leads activities` — List a lead's activities
- `copper-pp-cli leads convert` — Convert a lead into a person/company/opportunity
- `copper-pp-cli leads create` — Create a new lead
- `copper-pp-cli leads delete` — Delete a lead
- `copper-pp-cli leads get` — Fetch a lead by id
- `copper-pp-cli leads search` — List/search leads
- `copper-pp-cli leads update` — Update a lead
- `copper-pp-cli leads upsert` — Create or update a lead, matched by email or a custom field

**loss-reasons** — Opportunity loss reasons

- `copper-pp-cli loss-reasons` — List all loss reasons

**opportunities** — Manage opportunities (deals)

- `copper-pp-cli opportunities create` — Create a new opportunity
- `copper-pp-cli opportunities delete` — Delete an opportunity
- `copper-pp-cli opportunities get` — Fetch an opportunity by id
- `copper-pp-cli opportunities search` — List/search opportunities
- `copper-pp-cli opportunities update` — Update an opportunity

**people** — Manage people (contacts)

- `copper-pp-cli people activities` — List a person's activities
- `copper-pp-cli people create` — Create a new person
- `copper-pp-cli people delete` — Delete a person
- `copper-pp-cli people fetch-by-email` — Fetch a person by email address
- `copper-pp-cli people get` — Fetch a person by id
- `copper-pp-cli people search` — List/search people
- `copper-pp-cli people update` — Update a person

**pipeline-stages** — Pipeline stages

- `copper-pp-cli pipeline-stages` — List all pipeline stages

**pipelines** — Sales pipelines

- `copper-pp-cli pipelines list` — List all pipelines
- `copper-pp-cli pipelines stages` — List stages within a specific pipeline

**projects** — Manage projects

- `copper-pp-cli projects create` — Create a new project
- `copper-pp-cli projects delete` — Delete a project
- `copper-pp-cli projects get` — Fetch a project by id
- `copper-pp-cli projects search` — List/search projects
- `copper-pp-cli projects update` — Update a project

**tags** — Tags used across records

- `copper-pp-cli tags` — List all tags

**tasks** — Manage tasks

- `copper-pp-cli tasks create` — Create a new task
- `copper-pp-cli tasks delete` — Delete a task
- `copper-pp-cli tasks get` — Fetch a task by id
- `copper-pp-cli tasks search` — List/search tasks
- `copper-pp-cli tasks update` — Update a task

**users** — Users in the account

- `copper-pp-cli users get` — Fetch a user by id
- `copper-pp-cli users search` — List/search users

**webhooks** — Webhook subscriptions

- `copper-pp-cli webhooks create` — Create a new webhook subscription
- `copper-pp-cli webhooks delete` — Delete (unsubscribe) a webhook subscription
- `copper-pp-cli webhooks get` — Fetch a webhook subscription by id
- `copper-pp-cli webhooks list` — List all webhook subscriptions


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
copper-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Monday weighted forecast by rep

```bash
copper-pp-cli forecast --by assignee --agent --select assignee_name,weighted_value,open_value
```

Commit view per rep without a spreadsheet pivot.

### Sweep then bulk-reassign cold deals

```bash
copper-pp-cli stale --days 30 --by assignee --agent
```

List cold deals grouped by owner. Save the JSON, then feed the ids into the bulk reassign command (with --set assignee_id=<id>) to mass-reassign — preview with --dry-run before applying.

### Idempotent contact import

```bash
copper-pp-cli upsert person --match email --file leads.json --dry-run
```

Create-or-update external rows without duplicating contacts.

### One-call deal prep

```bash
copper-pp-cli who opportunity:88 --agent --select company.name,people.name,activities.details
```

Pull the deal's company, contacts, and recent touches in one narrowed view.

## Auth Setup

Copper uses a multi-header API key. Set COPPER_API_KEY (System Settings -> API Keys -> Create a Key) and COPPER_USER_EMAIL (the email of the key owner). Every request sends X-PW-AccessToken, X-PW-UserEmail, and X-PW-Application: developer_api.

Run `copper-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  copper-pp-cli account --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `COPPER_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `COPPER_CONFIG_DIR`, `COPPER_DATA_DIR`, `COPPER_STATE_DIR`, `COPPER_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `COPPER_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `copper-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "copper": {
        "command": "copper-pp-mcp",
        "env": {
          "COPPER_HOME": "/srv/copper"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `COPPER_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `COPPER_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
copper-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
copper-pp-cli feedback --stdin < notes.txt
copper-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `COPPER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `COPPER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
copper-pp-cli profile save briefing --json
copper-pp-cli --profile briefing account
copper-pp-cli profile list --json
copper-pp-cli profile show briefing
copper-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `copper-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/copper/cmd/copper-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add copper-pp-mcp -- copper-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which copper-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   copper-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `copper-pp-cli <command> --help`.

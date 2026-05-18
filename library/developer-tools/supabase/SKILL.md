<!-- // PATCH: hand-edited headline + Known Gaps section + narrowed trigger phrases vs generated defaults; aligned with README/SKILL contract. -->
---
name: pp-supabase
description: "The full Supabase Management API (108 endpoints) plus a local SQLite cache of orgs, projects, functions, branches, and secret names — powering cross-project queries no live API answers in one call, with Auth Admin lookup, PostgREST schema introspection, and Storage usage rollup on top. Trigger phrases: `supabase auth admin lookup`, `supabase secret name audit`, `supabase branches drift`, `supabase project estate rollup`, `supabase storage usage`, `supabase pgrst schema`, `use supabase`, `run supabase`. Anti-triggers: `supabase start` (use the official supabase CLI), `supabase db push` (official CLI), `supabase gen types` (official CLI), supabase realtime subscribe (WebSocket — out of scope)."
author: "Giuliano Giacaglia"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - supabase-pp-cli
---

# Supabase — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `supabase-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install supabase --cli-only
   ```
2. Verify: `supabase-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The official supabase CLI is local-dev tooling (Docker, migrations, types). This CLI is the runtime API surface it omits, with `--json --select --dry-run` consistency across every command. The local store enables cross-project queries (`secrets where-name STRIPE_KEY`, `branches drift --older-than 7d`, `functions inventory --org acme`) that no single live API call answers.

## When to Use This CLI

Use this CLI when an agent needs to sweep Management API entities across orgs, run cross-project queries from a local SQLite cache (`secrets where-name`, `functions inventory`, `branches drift`, `projects estate`), look up Auth users by email (`auth-admin lookup`), aggregate Storage bucket usage (`storage usage`), or fetch a per-project PostgREST schema for typed query planning (`pgrst schema`).

For local Docker + migration workflows (`supabase start`, `supabase db push`, `supabase gen types`), use the official Supabase CLI. For PostgREST row CRUD, Storage object lifecycle (upload/download/sign), Auth Admin user mutations, and Edge Function runtime invocation, use `supabase-js` — these are documented Known Gaps in this CLI's first release (see README's `## Known Gaps`).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-project rollups
- **`secrets where-name`** — Find every project (across orgs) holding a secret with a given name, plus when each was last synced.

  _Use when an agent needs cross-project security posture answers ('which clients have STRIPE_KEY', 'is this secret name leaked anywhere'). The supabase-community MCP cannot answer this without per-project iteration._

  ```bash
  supabase-pp-cli secrets where-name STRIPE_KEY --json
  ```
- **`functions inventory`** — Per-project, per-org rollup of every edge function with slug, version, status, and last-deployed timestamp.

  _Use during deploy review ('which projects deployed stripe-webhook?') or stale-function audits ('which functions haven't redeployed in 90 days?')._

  ```bash
  supabase-pp-cli functions inventory --org acme --json
  ```
- **`branches drift`** — List preview branches older than N days that haven't been merged or deleted, grouped by parent project.

  _Use during weekly cleanup to prevent preview-branch sprawl that costs DB resources._

  ```bash
  supabase-pp-cli branches drift --older-than 7d --json
  ```
- **`projects estate`** — One-row-per-project rollup of function count, branch count, api-key count, secret-name count, and last-synced-at.

  _Use for the Monday morning estate review; one screen replaces 12 dashboard tabs._

  ```bash
  supabase-pp-cli projects estate --json
  ```
- **`auth-admin recent`** — Iterate synced projects, call Auth Admin per project, aggregate users created within the window into one table.

  _Use for weekly user-growth review across an entire org portfolio._

  ```bash
  supabase-pp-cli auth-admin recent --since 7d --json
  ```

### Service-specific patterns
- **`auth-admin lookup`** — Look up an Auth user by email and optionally join their row from a user-named PostgREST context table on user_id.

  _Use during support-ticket triage to see the user plus their domain row in one envelope instead of three dashboard clicks._

  ```bash
  supabase-pp-cli auth-admin lookup user@example.com --context-table profiles --context-key user_id --json
  ```
- **`pgrst schema`** — Fetch the per-project PostgREST OpenAPI from the Management API and list tables, columns, types, and detected indexes for typed query planning.

  _Use before authoring a typed query so an agent knows the table's column types and constraints without trial-and-error._

  ```bash
  supabase-pp-cli pgrst schema --table profiles --json
  ```
- **`storage usage`** — For each bucket, list objects and aggregate file count, total bytes, and largest object.

  _Use when an agent needs to answer 'how close are we to the storage ceiling' or 'which bucket is biggest'._

  ```bash
  supabase-pp-cli storage usage --bucket avatars --json
  ```

## Command Reference

**branches** — Manage branches

- `supabase-pp-cli branches delete-a-branch` — Deletes the specified database branch. By default, deletes immediately. Use force=false to schedule deletion with...
- `supabase-pp-cli branches get-a-branch-config` — Fetches configurations of the specified database branch
- `supabase-pp-cli branches update-a-branch-config` — Updates the configuration of the specified database branch

**oauth** — OAuth related endpoints

- `supabase-pp-cli oauth authorize-project-claim` — Initiates the OAuth authorization flow for the specified provider. After successful authentication, the user can...
- `supabase-pp-cli oauth authorize-user` — [Beta] Authorize user through oauth
- `supabase-pp-cli oauth exchange-token` — [Beta] Exchange auth code for user's access and refresh token
- `supabase-pp-cli oauth revoke-token` — [Beta] Revoke oauth app authorization and it's corresponding tokens

**organizations** — Organizations related endpoints

- `supabase-pp-cli organizations create-an` — Create an organization
- `supabase-pp-cli organizations get-an` — Gets information about the organization
- `supabase-pp-cli organizations list-all` — Returns a list of organizations that you currently belong to.

**projects** — Projects related endpoints

- `supabase-pp-cli projects create-a` — Create a project
- `supabase-pp-cli projects delete-a` — Deletes the given project
- `supabase-pp-cli projects get` — Gets a specific project that belongs to the authenticated user
- `supabase-pp-cli projects get-available-regions` — [Beta] Gets the list of available regions that can be used for a new project
- `supabase-pp-cli projects list-all` — Returns a list of all projects you've previously created.
- `supabase-pp-cli projects update-a` — Updates the given project

**snippets** — Manage snippets

- `supabase-pp-cli snippets get-a` — Gets a specific SQL snippet
- `supabase-pp-cli snippets list-all` — Lists SQL snippets for the logged in user

**supabase-profile** — Manage supabase profile

- `supabase-pp-cli supabase-profile` — Gets the user's profile


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
supabase-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Cross-project secret audit

```bash
supabase-pp-cli sync && supabase-pp-cli secrets where-name STRIPE_KEY --json
```

Sync the local store, then list every project across every org holding a secret named STRIPE_KEY — the Monday-morning security sweep in one shell call.

### Support ticket triage

```bash
supabase-pp-cli auth-admin lookup user@example.com --context-table profiles --context-key user_id --agent --select user.id,user.email,user.last_sign_in_at,context.tier
```

Look up the user in Auth Admin and join their profiles row in one envelope; --select narrows the payload to just the fields the support workflow cares about.

### Stale preview branches

```bash
supabase-pp-cli branches drift --older-than 14d --json
```

Surface preview branches older than 14 days that have not been merged or deleted, grouped by parent project — the Tuesday cleanup target list.

### Storage ceiling check

```bash
supabase-pp-cli storage usage --json | jq '.[] | select(.total_bytes > 100000000)'
```

List every bucket over 100MB; pipe through jq to filter on byte thresholds.

### PostgREST schema before query

```bash
supabase-pp-cli pgrst schema --table orders --agent --select tables.columns.name,tables.columns.type,tables.columns.nullable
```

Get just the column types of the orders table from the Management-API-routed schema endpoint, so an agent can author a typed PostgREST query without trial-and-error.

## Auth Setup

Supabase has three credential types across two API hosts. The Management API at api.supabase.com uses a Personal Access Token (`SUPABASE_ACCESS_TOKEN`, header `Authorization: Bearer <PAT>`). Project APIs at `<ref>.supabase.co` use the `apikey:` header — set `SUPABASE_PUBLISHABLE_KEY` (or legacy `SUPABASE_ANON_KEY`) for RLS-respecting calls, or `SUPABASE_SERVICE_ROLE_KEY` (or new `SUPABASE_SECRET_KEY`) for server-side calls that bypass RLS and unlock Auth Admin endpoints. Also set `SUPABASE_URL=https://<ref>.supabase.co`. Edge Functions gotcha: keys go in `apikey:`, never `Authorization: Bearer`.

Run `supabase-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  supabase-pp-cli projects get mock-value --agent --select id,name,status
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
supabase-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
supabase-pp-cli feedback --stdin < notes.txt
supabase-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.supabase-pp-cli/feedback.jsonl`. They are never POSTed unless `SUPABASE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUPABASE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
supabase-pp-cli profile save briefing --json
supabase-pp-cli --profile briefing projects get mock-value
supabase-pp-cli profile list --json
supabase-pp-cli profile show briefing
supabase-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `supabase-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add supabase-pp-mcp -- supabase-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which supabase-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   supabase-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `supabase-pp-cli <command> --help`.

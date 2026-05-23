<!-- // PATCH: hand-edited headline + Known Gaps section + narrowed trigger phrases vs generated defaults; aligned with README/SKILL contract. -->
# Supabase CLI

**The full Supabase Management API (108 endpoints) plus a local SQLite cache of orgs, projects, functions, branches, and secret names — powering cross-project queries no live API answers in one call, with Auth Admin lookup and PostgREST schema introspection on top.**

The official Supabase CLI is local-dev tooling (Docker, migrations, types). This CLI is the API-surface companion, with `--json --select --dry-run` consistency across every command. Cross-project queries (`secrets where-name STRIPE_KEY`, `branches drift --older-than 7d`, `functions inventory --org acme`) operate over the local store; `auth-admin lookup`, `pgrst schema`, and `storage usage` hit live project APIs.

> **Known Gaps** (see `## Known Gaps` below): hand-written Auth Admin user CRUD, Storage object lifecycle, PostgREST row CRUD, Edge Function runtime invoke. Use `supabase-js` or the Supabase dashboard for those until a follow-up polish session adds them.

Printed by [@giacaglia](https://github.com/giacaglia) (Giuliano Giacaglia).

## Install

The recommended path installs both the `supabase-pp-cli` binary and the `pp-supabase` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install supabase
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install supabase --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install supabase --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install supabase --agent claude-code
npx -y @mvanhorn/printing-press-library install supabase --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/cmd/supabase-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/supabase-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-supabase --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-supabase --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-supabase skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-supabase. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/supabase-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SUPABASE_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "supabase": {
      "command": "supabase-pp-mcp",
      "env": {
        "SUPABASE_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Supabase has three credential types across two API hosts. The Management API at api.supabase.com uses a Personal Access Token (`SUPABASE_ACCESS_TOKEN`, header `Authorization: Bearer <PAT>`). Project APIs at `<ref>.supabase.co` use the `apikey:` header — set `SUPABASE_PUBLISHABLE_KEY` (or legacy `SUPABASE_ANON_KEY`) for RLS-respecting calls, or `SUPABASE_SERVICE_ROLE_KEY` (or new `SUPABASE_SECRET_KEY`) for server-side calls that bypass RLS and unlock Auth Admin endpoints. Also set `SUPABASE_URL=https://<ref>.supabase.co`. Edge Functions gotcha: keys go in `apikey:`, never `Authorization: Bearer`.

## Quick Start

```bash
# Verify both Management and project credentials are wired and reachable.
supabase-pp-cli doctor

# List your organizations via the Management API (requires SUPABASE_ACCESS_TOKEN).
supabase-pp-cli organizations list-all --json

# Populate the local SQLite store with orgs/projects/functions/branches/secrets.
supabase-pp-cli sync --json

# Cross-project audit: every project holding the named secret (local SQL).
supabase-pp-cli secrets where-name STRIPE_KEY --json

# Fetch the per-project PostgREST schema via Management API (replacement for the anon-key path being removed April 2026).
supabase-pp-cli pgrst schema --table profiles --json

```

## Unique Features

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

## Known Gaps

These project-surface command groups were planned in the absorb manifest but deferred from this initial build (Phase 3 scope contraction). Use `supabase-js` or the Supabase dashboard for them until a follow-up polish session lands them:

- **Auth Admin user CRUD** — bulk create, invite, update, delete users + MFA factor management. `auth-admin lookup` and `auth-admin recent` (cross-project signup window) DO ship; this gap covers the write side.
- **Storage object lifecycle** — upload, download, signed URL generation, move, copy, delete. `storage usage` (per-bucket size/count rollup) DOES ship; this gap covers object-level CRUD.
- **PostgREST row CRUD** — `pgrst select / insert / upsert / delete` against user-defined tables. `pgrst schema` (typed schema fetch via Management API) DOES ship; row CRUD is deferred.
- **Edge Function runtime invocation** — `POST /functions/v1/<name>` with body. Function lifecycle (deploy/list/get) is available via `projects functions ...` from the Management API mirror; the runtime invoke wrapper is deferred.
- **Realtime WebSocket subscribe** — permanently out of scope (WebSocket doesn't fit a one-shot CLI shape).

Workarounds: for PostgREST row queries, `curl https://<ref>.supabase.co/rest/v1/<table>?<filter> -H 'apikey: $SUPABASE_PUBLISHABLE_KEY' -H 'Authorization: Bearer $SUPABASE_PUBLISHABLE_KEY'` or the `supabase-js` PostgREST builder. For Storage object operations, `supabase-js`'s storage client. For Edge Function invoke, `curl https://<ref>.supabase.co/functions/v1/<name> -H 'apikey: $SUPABASE_PUBLISHABLE_KEY' -d <body>`.

## Usage

Run `supabase-pp-cli --help` for the full command reference and flag list.

## Commands

### branches

Manage branches

- **`supabase-pp-cli branches delete-a-branch`** - Deletes the specified database branch. By default, deletes immediately. Use force=false to schedule deletion with 1-hour grace period (only when soft deletion is enabled).
- **`supabase-pp-cli branches get-a-branch-config`** - Fetches configurations of the specified database branch
- **`supabase-pp-cli branches update-a-branch-config`** - Updates the configuration of the specified database branch

### oauth

OAuth related endpoints

- **`supabase-pp-cli oauth authorize-project-claim`** - Initiates the OAuth authorization flow for the specified provider. After successful authentication, the user can claim ownership of the specified project.
- **`supabase-pp-cli oauth authorize-user`** - [Beta] Authorize user through oauth
- **`supabase-pp-cli oauth exchange-token`** - [Beta] Exchange auth code for user's access and refresh token
- **`supabase-pp-cli oauth revoke-token`** - [Beta] Revoke oauth app authorization and it's corresponding tokens

### organizations

Organizations related endpoints

- **`supabase-pp-cli organizations create-an`** - Create an organization
- **`supabase-pp-cli organizations get-an`** - Gets information about the organization
- **`supabase-pp-cli organizations list-all`** - Returns a list of organizations that you currently belong to.

### projects

Projects related endpoints

- **`supabase-pp-cli projects create-a`** - Create a project
- **`supabase-pp-cli projects delete-a`** - Deletes the given project
- **`supabase-pp-cli projects get`** - Gets a specific project that belongs to the authenticated user
- **`supabase-pp-cli projects get-available-regions`** - [Beta] Gets the list of available regions that can be used for a new project
- **`supabase-pp-cli projects list-all`** - Returns a list of all projects you've previously created.
- **`supabase-pp-cli projects update-a`** - Updates the given project

### snippets

Manage snippets

- **`supabase-pp-cli snippets get-a`** - Gets a specific SQL snippet
- **`supabase-pp-cli snippets list-all`** - Lists SQL snippets for the logged in user

### supabase-profile

Manage supabase profile

- **`supabase-pp-cli supabase-profile get`** - Gets the user's profile

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
supabase-pp-cli projects get mock-value

# JSON for scripting and agents
supabase-pp-cli projects get mock-value --json

# Filter to specific fields
supabase-pp-cli projects get mock-value --json --select id,name,status

# Dry run — show the request without sending
supabase-pp-cli projects get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
supabase-pp-cli projects get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
supabase-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/supabase-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SUPABASE_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `supabase-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SUPABASE_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on Auth Admin or RLS-bypassing PostgREST calls** — These endpoints require the service_role key. Set `SUPABASE_SERVICE_ROLE_KEY` (or new `SUPABASE_SECRET_KEY`); the publishable/anon key is RLS-only.
- **401 Unauthorized on Edge Function invoke** — Keys go in the `apikey:` header, not `Authorization: Bearer`. The CLI sets this correctly by default; check that you haven't overridden the auth header via --header.
- **403 Forbidden on Management API** — Management API needs a Personal Access Token (sbp_...), not a project key. Generate one at supabase.com/dashboard/account/tokens and `export SUPABASE_ACCESS_TOKEN=...`.
- **Empty results from `pgrst select` despite knowing rows exist** — Row-Level Security is filtering them out. Either use the service_role key (bypasses RLS) or sign in as a user whose JWT passes the table's RLS policy.
- **`branches drift` returns nothing** — Sync at least once: `supabase-pp-cli sync` populates the local store. Drift compares synced rows to filter conditions; an empty store has nothing to compare.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**supabase/cli**](https://github.com/supabase/cli) — Go (9000 stars)
- [**supabase/supabase-js**](https://github.com/supabase/supabase-js) — TypeScript (4400 stars)
- [**supabase-community/supabase-mcp**](https://github.com/supabase-community/supabase-mcp) — TypeScript (2700 stars)
- [**supabase/supabase-py**](https://github.com/supabase/supabase-py) — Python (2500 stars)
- [**supabase/auth**](https://github.com/supabase/auth) — Go (1500 stars)
- [**alexander-zuev/supabase-mcp-server**](https://github.com/alexander-zuev/supabase-mcp-server) — Python (820 stars)
- [**supabase-community/supabase-go**](https://github.com/supabase-community/supabase-go) — Go (400 stars)
- [**nedpals/supabase-go**](https://github.com/nedpals/supabase-go) — Go (350 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

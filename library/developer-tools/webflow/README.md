# Webflow CLI

**The whole Webflow Data API, plus a local database that lets you audit SEO, diff staging against live, and preview a publish before you trigger it.**

Webflow's own CLI stops at sites, CMS, forms, and assets. Every page-level operation you actually need has an endpoint and no command: SEO metadata, page copy, redirects, robots.txt, custom code. This CLI covers all 117 Data API operations and adds a local SQLite mirror on top. The seven audit commands read that mirror rather than the API, so run `sync --full` once before `seo audit`, `drift`, `publish preview`, `overview`, `redirects audit`, `collections completeness`, or `items bulk-set`; without it they return empty.

Learn more at [Webflow](https://developers.webflow.com).

Created by [@kmorebetter](https://github.com/kmorebetter) (Kerry Morrison).

## Install

The recommended path installs both the `webflow-pp-cli` binary and the `pp-webflow` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install webflow
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install webflow --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install webflow --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install webflow --agent claude-code
npx -y @mvanhorn/printing-press-library install webflow --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/cmd/webflow-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/webflow-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install webflow --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-webflow --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-webflow --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install webflow --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/webflow-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WEBFLOW_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/cmd/webflow-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "webflow": {
      "command": "webflow-pp-mcp",
      "env": {
        "WEBFLOW_COLLECTION_ID": "<collection_id>",
        "WEBFLOW_FORM_ID": "<form_id>",
        "WEBFLOW_PAGE_ID": "<page_id>",
        "WEBFLOW_WORKSPACE_ID_OR_SLUG": "<workspace_id_or_slug>",
        "WEBFLOW_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Webflow accepts two token shapes on the same `Authorization: Bearer` header. A site token is generated per site under Site settings, Apps & Integrations, API access. A workspace token starts with `ws-` and covers every site in the workspace. Set either one as `WEBFLOW_API_TOKEN` in your environment; that is the only way to supply a static token, since this CLI has no set-token command. `login` runs the OAuth2 flow instead, and `auth status` shows what is currently loaded. Scopes matter more than the token shape: a token created without data scopes authenticates fine and then returns `403 OAuthForbidden: You are missing the following scopes` on the first real call. `doctor` tells you whether a token loaded and whether the API is reachable; it does not enumerate scopes. Use `token introspect` for that, and read the 403 body, which names the exact scope you are missing. One endpoint sits outside all of this: `workspaces audit-logs` needs an Enterprise workspace plus a workspace token carrying `workspace_activity:read`, a scope the OAuth2 flow cannot grant at all.

## Quick Start

```bash
# Confirms a token loaded and the API is reachable. It does not enumerate scopes; use `token introspect` or read a 403 body for that.
webflow-pp-cli doctor

# Every site the token can reach; you need a site ID for almost everything else.
webflow-pp-cli sites list

# Builds the local mirror the audit commands read. Sync everything rather than naming resources: CMS items live under a collection, so they only resolve after collections are synced.
webflow-pp-cli sync --full

# Ranked SEO findings for every page on that site, computed offline.
webflow-pp-cli seo audit 580e63e98c9a982ac9b8b741

# Everything a publish would change, before you trigger the build.
webflow-pp-cli publish preview 580e63e98c9a982ac9b8b741

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`seo audit`** — Find every page on a site with a missing, duplicated, or over-length SEO title or description before it costs you traffic.

  _Reach for this instead of paging the pages endpoint yourself when you need the site-wide metadata problems ranked, not the raw page objects._

  ```bash
  webflow-pp-cli seo audit 580e63e98c9a982ac9b8b741 --agent
  ```
- **`drift`** — See exactly which CMS items were edited in staging but never published, field by field.

  _Reach for this before any publish when you need to know what is actually about to go live rather than trusting the staging view._

  ```bash
  webflow-pp-cli drift 580e63fc8c9a982ac9b8b745 --agent
  ```
- **`overview`** — One row per site you have synced locally: pages, collections, unpublished items, SEO findings, days since last publish.

  _Reach for this when the question starts with 'across all my sites'; every other tool forces you to ask it one site at a time. Covers the sites present in the local mirror, so sync every site you care about first._

  ```bash
  webflow-pp-cli overview --agent --select sites.displayName,sites.unpublishedItems,sites.daysSincePublish
  ```

### Publish safety
- **`publish preview`** — See everything that would change if you published this site right now: pages edited since the last publish, draft pages, and unpublished CMS items, alongside the site's redirect-rule count.

  _Reach for this as the pre-flight check before calling the publish endpoint, especially in CI where a surprise publish is expensive to undo._

  ```bash
  webflow-pp-cli publish preview 580e63e98c9a982ac9b8b741 --agent
  ```
- **`redirects audit`** — Catch redirects that shadow a live page, point nowhere, loop, or duplicate another rule.

  _Reach for this during launch hygiene; the redirect endpoints let you read and write rules but never tell you which ones are wrong._

  ```bash
  webflow-pp-cli redirects audit 580e63e98c9a982ac9b8b741 --agent
  ```

### Bulk content work
- **`items bulk-set`** — Apply the same field value to many CMS items selected by a condition. Previews the change set by default; --apply writes it, paced so you never hit a rate limit mid-run.

  _Reach for this instead of looping the update endpoint yourself; it previews the change set first and survives a 60-request-per-minute ceiling._

  ```bash
  webflow-pp-cli items bulk-set 580e63fc8c9a982ac9b8b745 --match status=draft --set author=editorial --agent
  ```
- **`collections completeness`** — Per-field fill rate across a whole collection, so you can spot required fields nobody filled and schema fields nobody uses.

  _Reach for this before a content batch goes live when you need to know which fields are systematically empty rather than checking items one at a time._

  ```bash
  webflow-pp-cli collections completeness 580e63fc8c9a982ac9b8b745 --agent
  ```

## Recipes

### Rank SEO problems across a site without a crawler

```bash
webflow-pp-cli seo audit 580e63e98c9a982ac9b8b741 --agent --select findings.pageSlug,findings.issue,findings.severity
```

Reads the local pages mirror and returns only the three fields an agent needs to build a fix list, instead of the full page objects.

### Check what a publish would actually change

```bash
webflow-pp-cli publish preview 580e63e98c9a982ac9b8b741 --json
```

Joins publish timestamps, page updates, unpublished item counts, and pending redirects into one pre-flight answer.

### Find CMS items edited in staging but never published

```bash
webflow-pp-cli drift 580e63fc8c9a982ac9b8b745 --json
```

Diffs the staged and live copies of every item in the collection field by field.

### Retag many items in one paced pass

```bash
webflow-pp-cli items bulk-set 580e63fc8c9a982ac9b8b745 --match category=news --set category=updates
```

Selects matching items from the local mirror and prints the change set. It previews by default; add --apply to execute against the API.

### Search every synced page, item, and submission at once

```bash
webflow-pp-cli search "pricing" --limit 20 --json
```

Full-text search across the local mirror, which no other Webflow tool offers.

## Usage

Run `webflow-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WEBFLOW_CONFIG_DIR`, `WEBFLOW_DATA_DIR`, `WEBFLOW_STATE_DIR`, or `WEBFLOW_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WEBFLOW_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WEBFLOW_HOME=/srv/webflow
webflow-pp-cli doctor
```

Under `WEBFLOW_HOME=/srv/webflow`, the four dirs resolve to `/srv/webflow/config`, `/srv/webflow/data`, `/srv/webflow/state`, and `/srv/webflow/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "webflow": {
      "command": "webflow-pp-mcp",
      "env": {
        "WEBFLOW_HOME": "/srv/webflow"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WEBFLOW_DATA_DIR` overrides an explicit `--home` for that kind. Use `WEBFLOW_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WEBFLOW_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `webflow-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### asset-folders

Manage asset folders

- **`webflow-pp-cli asset-folders <asset_folder_id>`** - Get details about a specific Asset Folder

Required scope | `assets:read`

### assets

Assets are files that are uploaded to your Webflow account.

- **`webflow-pp-cli assets delete`** - Delete an Asset

Required Scope: `assets: write`
- **`webflow-pp-cli assets get`** - Get details about an asset

Required scope | `assets:read`
- **`webflow-pp-cli assets patch`** - Update details of an Asset.

Required scope | `assets:write`

### collections

Collections are CMS collections of items.

- **`webflow-pp-cli collections delete`** - Delete a collection using its ID.

Required scope | `cms:write`
- **`webflow-pp-cli collections details`** - Get the full details of a collection from its ID.

Required scope | `cms:read`

### form-submissions

Manage form submissions

- **`webflow-pp-cli form-submissions delete`** - Delete a form submission


Required scope | `forms:write`
- **`webflow-pp-cli form-submissions get`** - Get information about a given form submissio.

Required scope | `forms:read`
- **`webflow-pp-cli form-submissions modify`** - Update hidden fields on a form submission

Required scope | `forms:write`

### forms

Forms are forms that are created on your Webflow site.

- **`webflow-pp-cli forms <form_id>`** - Get information about a given form.

Required scope | `forms:read`

### pages

Pages are the pages in your Webflow site.

- **`webflow-pp-cli pages get-metadata`** - Get metadata information for a single page.

Required scope | `pages:read`
- **`webflow-pp-cli pages update-settings`** - Update Page-level metadata, including SEO and Open Graph fields.

Required scope | `pages:write`

### sites

Sites are the sites in your Webflow workspace.

- **`webflow-pp-cli sites delete`** - Delete a site.

<Warning title="Enterprise Only">This endpoint requires an Enterprise workspace.</Warning>

Required scope | `sites:write`
- **`webflow-pp-cli sites get`** - Get details of a site.

Required scope | `sites:read`
- **`webflow-pp-cli sites list`** - List of all sites the provided access token is able to access.

Required scope | `sites:read`
- **`webflow-pp-cli sites update`** - Update a site.

<Warning title="Enterprise Only">This endpoint requires an Enterprise workspace.</Warning>

Required scope | `sites:write`

### token

Manage token

- **`webflow-pp-cli token authorized-by`** - Information about the Authorized User

Required Scope | `authorized_user:read`
- **`webflow-pp-cli token introspect`** - Information about the authorization token

<Note>Access to this endpoint requires a bearer token from a [Data Client App](/data/docs/data-clients/getting-started).</Note>

### webhooks

Webhooks are the webhooks in your Webflow site.

- **`webflow-pp-cli webhooks get`** - Get a specific Webhook instance

Required scope: `sites:read`
- **`webflow-pp-cli webhooks remove`** - Remove a Webhook

Required scope: `sites:read`

### workspaces

Manage workspaces



### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`webflow-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`webflow-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`webflow-pp-cli learnings list`** - Inspect taught rows
- **`webflow-pp-cli learnings forget <query>`** - Undo a teach
- **`webflow-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`webflow-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`webflow-pp-cli teach-pattern`** - Install a query/resource template up front
- **`webflow-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WEBFLOW_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `webflow-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
webflow-pp-cli asset-folders mock-value

# JSON for scripting and agents
webflow-pp-cli asset-folders mock-value --json

# Filter to specific fields
webflow-pp-cli asset-folders mock-value --json --select id,name,status

# Dry run — show the request without sending
webflow-pp-cli asset-folders mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
webflow-pp-cli asset-folders mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `WEBFLOW_COLLECTION_ID` resolves `{collection_id}`
- `WEBFLOW_FORM_ID` resolves `{form_id}`
- `WEBFLOW_PAGE_ID` resolves `{page_id}`
- `WEBFLOW_WORKSPACE_ID_OR_SLUG` resolves `{workspace_id_or_slug}`

Base URL: `https://api.webflow.com/v2`

## Health Check

```bash
webflow-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `webflow-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/webflow-pp-cli/config.toml`; `--home`, `WEBFLOW_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WEBFLOW_COLLECTION_ID` | endpoint | Yes |  |
| `WEBFLOW_FORM_ID` | endpoint | Yes |  |
| `WEBFLOW_PAGE_ID` | endpoint | Yes |  |
| `WEBFLOW_WORKSPACE_ID_OR_SLUG` | endpoint | Yes |  |
| `WEBFLOW_API_TOKEN` | per_call | No | Set to your API credential. |
| `WEBFLOW_OAUTH2` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `webflow-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `webflow-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WEBFLOW_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 with `OAuthForbidden: You are missing the following scopes`** — Your token authenticates but lacks data scopes. Generate a site token under Site settings, Apps & Integrations, API access, then export it as `WEBFLOW_API_TOKEN`. This CLI has no set-token command; `login` is the OAuth2 path.
- **429 Too Many Requests partway through a bulk operation** — Starter and Basic plans allow 60 requests per minute, CMS and Business allow 120. Run `webflow-pp-cli sites plan get-site 580e63e98c9a982ac9b8b741` to see your tier; bulk commands already pace against `Retry-After`.
- **An audit command returns nothing at all** — The local mirror is empty. Run `webflow-pp-cli sync --full` first.
- **`token introspect` returns 500 internal_error** — Webflow returns a 500 here for tokens with no scopes. Use `webflow-pp-cli doctor` instead, which degrades gracefully.
- **Item edits appear in the Designer but not on the live site** — Staged and live items are separate. Run `webflow-pp-cli drift 580e63fc8c9a982ac9b8b745` to see the gap, then `webflow-pp-cli collections items publish 580e63fc8c9a982ac9b8b745` to push them live.
- **`sync --resources items` fails with `WEBFLOW_COLLECTION_ID not set`** — CMS items are scoped to a collection. Either run `webflow-pp-cli sync --full` so collections are synced first, or set `WEBFLOW_COLLECTION_ID` to the collection you want and re-run.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**js-webflow-api**](https://github.com/webflow/js-webflow-api) — TypeScript (344 stars)
- [**webflow-mcp-server**](https://github.com/webflow/mcp-server) — TypeScript (135 stars)
- [**webflow-skills**](https://github.com/webflow/webflow-skills) — Markdown (106 stars)
- [**webflow-python**](https://github.com/webflow/webflow-python) — Python (21 stars)
- [**openapi-spec**](https://github.com/webflow/openapi-spec) — YAML (13 stars)
- [**webflowctl**](https://github.com/joinflux/webflowctl) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

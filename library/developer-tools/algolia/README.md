# Algolia CLI

**Every Algolia feature, plus offline search and a local database no other Algolia tool has.**

The Algolia CLI manages indices, records, search, rules, synonyms, API keys, and settings from the terminal — with a local SQLite mirror, cross-index search, settings diffing, and relevance regression checks that the official CLI cannot offer.

## Install

The recommended path installs both the `algolia-pp-cli` binary and the `pp-algolia` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install algolia
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install algolia --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install algolia --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install algolia --agent claude-code
npx -y @mvanhorn/printing-press-library install algolia --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/cmd/algolia-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/algolia-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install algolia --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-algolia --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-algolia --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install algolia --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/algolia-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ALGOLIA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/cmd/algolia-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "algolia": {
      "command": "algolia-pp-mcp",
      "env": {
        "ALGOLIA_APPLICATION_ID": "<appId>",
        "ALGOLIA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Algolia uses two credentials: your Application ID (ALGOLIA_APPLICATION_ID) and an API key (ALGOLIA_API_KEY), sent as x-algolia-application-id and x-algolia-api-key headers. Find both in the Algolia dashboard under Settings > API Keys. Export them before running the CLI.

## Quick Start

```bash
# Verify your Algolia credentials are set and the API is reachable.
algolia-pp-cli doctor

# See every index in your application with record counts and sizes.
algolia-pp-cli indexes list-indices

# Search an index with the same parameters as the dashboard.
algolia-pp-cli indexes query search-single-index algolia_movie_sample_dataset --body-json '{"query":"dune"}'

# Mirror your index list into the local SQLite store for offline queries.
algolia-pp-cli sync --resources indexes

# Search across all synced indices at once.
algolia-pp-cli find --query "dune"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`find`** — Search across every synced index in one shot, with each hit labeled by its source index.

  _Use this when you need to know which of your indices contains a record, without issuing N API calls._

  ```bash
  algolia-pp-cli find --query "dune" --limit 20
  ```
- **`settings diff`** — Field-level comparison of settings between two indices (or a settings file vs an index).

  _Use this to verify prod/staging parity before a release instead of exporting and eyeballing JSON._

  ```bash
  algolia-pp-cli settings diff algolia_movie_sample_dataset staging_movies
  ```
- **`rules stale`** — Find rules that reference attributes missing from the index's searchable attributes or that can never match.

  _Use this to find dead-weight relevance rules before they silently corrupt search results._

  ```bash
  algolia-pp-cli rules stale --index algolia_movie_sample_dataset
  ```
- **`apikeys report`** — Audit all API keys: write-capable, unrestricted, and expired keys grouped by ACL, with last-use from synced logs.

  _Use this for key rotation audits to catch write-capable keys that have gone unused for months._

  ```bash
  algolia-pp-cli apikeys report
  ```
- **`objects gaps`** — List records missing attributes required by the index's searchable settings — records search can never return.

  _Use this after catalog imports to find records that are silently unreachable by search._

  ```bash
  algolia-pp-cli objects gaps --index algolia_movie_sample_dataset
  ```
- **`objects diff`** — Compare records of two indices (added/removed/changed by objectID) to verify prod/staging parity.

  _Use this to reconcile dev and prod copies after a copy/move operation instead of exporting and diffing files._

  ```bash
  algolia-pp-cli objects diff algolia_movie_sample_dataset staging_movies
  ```

### Agent-native plumbing
- **`search check`** — Assert that a query returns expected objectIDs, with a typed exit code for CI pipelines.

  _Use this after every rule or synonym change to catch relevance regressions before they reach users._

  ```bash
  algolia-pp-cli search check --index algolia_movie_sample_dataset --query "dune" --expect media-sample-data-438631
  ```
- **`logs errors`** — Aggregate synced log entries by error code and link failed tasks for a digest of what went wrong.

  _Use this for weekly error triage to see error spikes and affected indices at a glance._

  ```bash
  algolia-pp-cli logs errors --since 24h
  ```

## Recipes

### Verify a relevance change

```bash
algolia-pp-cli search check --index algolia_movie_sample_dataset --query "dune" --expect media-sample-data-438631
```

Assert that a query still returns an expected hit — run in CI after every rule or synonym change.

### Find dead rules

```bash
algolia-pp-cli rules stale --index algolia_movie_sample_dataset
```

Surface rules that reference removed attributes or can never match.

### Compare prod and staging settings

```bash
algolia-pp-cli settings diff algolia_movie_sample_dataset staging_movies
```

See exactly which settings differ between two indices before a release.

### Cross-index lookup

```bash
algolia-pp-cli find --query "dune" --select index,objectID,title --limit 10
```

Find which synced index contains a record and narrow the output with --select.

### Audit API keys

```bash
algolia-pp-cli apikeys report
```

Get a permission matrix of write-capable, unrestricted, and expired keys.

## Usage

Run `algolia-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ALGOLIA_CONFIG_DIR`, `ALGOLIA_DATA_DIR`, `ALGOLIA_STATE_DIR`, or `ALGOLIA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ALGOLIA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ALGOLIA_HOME=/srv/algolia
algolia-pp-cli doctor
```

Under `ALGOLIA_HOME=/srv/algolia`, the four dirs resolve to `/srv/algolia/config`, `/srv/algolia/data`, `/srv/algolia/state`, and `/srv/algolia/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "algolia": {
      "command": "algolia-pp-mcp",
      "env": {
        "ALGOLIA_HOME": "/srv/algolia"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ALGOLIA_DATA_DIR` overrides an explicit `--home` for that kind. Use `ALGOLIA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ALGOLIA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `algolia-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### clusters

Multi-cluster operations.

Multi-cluster operations are **deprecated**.
If you have issues with your Algolia infrastructure
due to large volumes of data, contact the Algolia support team.

- **`algolia-pp-cli clusters assign-user-id`** - Assigns or moves a user ID to a cluster.

The time it takes to move a user is proportional to the amount of data linked to the user ID.
- **`algolia-pp-cli clusters batch-assign-user-ids`** - Assigns multiple user IDs to a cluster.

**You can't move users with this operation**.
- **`algolia-pp-cli clusters get-top-user-ids`** - Get the IDs of the 10 users with the highest number of records per cluster.

Since it can take a few seconds to get the data from the different clusters,
the response isn't real-time.
- **`algolia-pp-cli clusters get-user-id`** - Returns the user ID data stored in the mapping.

Since it can take a few seconds to get the data from the different clusters,
the response isn't real-time.
- **`algolia-pp-cli clusters has-pending-mappings`** - To determine when the time-consuming process of creating a large batch of users or migrating users from one cluster to another is complete, this operation retrieves the status of the process.
- **`algolia-pp-cli clusters list`** - Lists the available clusters in a multi-cluster setup.
- **`algolia-pp-cli clusters list-user-ids`** - Lists the userIDs assigned to a multi-cluster application.

Since it can take a few seconds to get the data from the different clusters,
the response isn't real-time.
- **`algolia-pp-cli clusters remove-user-id`** - Deletes a user ID and its associated data from the clusters.
- **`algolia-pp-cli clusters search-user-ids`** - Since it can take a few seconds to get the data from the different clusters,
the response isn't real-time.

To ensure rapid updates, the user IDs index isn't built at the same time as the mapping. Instead, it's built every 12 hours, at the same time as the update of user ID usage. For example, if you add or move a user ID, the search will show an old value until the next time the mapping is rebuilt (every 12 hours).

### dictionaries

Manage your dictionaries.

Customize language-specific settings, such as stop words, plurals, or word segmentation.

Dictionaries are application-wide.

- **`algolia-pp-cli dictionaries get-dictionary-languages`** - Lists supported languages with their supported dictionary types and number of custom entries.
- **`algolia-pp-cli dictionaries get-dictionary-settings`** - Retrieves the languages for which standard dictionary entries are turned off.
- **`algolia-pp-cli dictionaries set-dictionary-settings`** - Turns standard stop word dictionary entries on or off for a given language.

### indexes

Manage indexes

- **`algolia-pp-cli indexes add-or-update-object`** - If a record with the specified object ID exists, the existing record is replaced.
Otherwise, a new record is added to the index.

If you want to use auto-generated object IDs, use the [`saveObject` operation](https://www.algolia.com/doc/rest-api/search/save-object).
To update _some_ attributes of an existing record, use the [`partial` operation](https://www.algolia.com/doc/rest-api/search/partial-update-object) instead.
To add, update, or replace multiple records, use the [`batch` operation](https://www.algolia.com/doc/rest-api/search/batch).
- **`algolia-pp-cli indexes delete-index`** - Deletes an index and all its settings.

- Deleting an index doesn't delete its analytics data.
- If you try to delete a non-existing index, the operation is ignored without warning.
- If the index you want to delete has replica indices, the replicas become independent indices.
- If the index you want to delete is a replica index, you must first unlink it from its primary index before you can delete it.
  For more information, see [Delete replica indices](https://www.algolia.com/doc/guides/managing-results/refine-results/sorting/how-to/deleting-replicas).
- **`algolia-pp-cli indexes delete-object`** - Deletes a record by its object ID.

To delete more than one record, use the [`batch` operation](https://www.algolia.com/doc/rest-api/search/batch).
To delete records matching a query, use the [`deleteBy` operation](https://www.algolia.com/doc/rest-api/search/delete-by).
- **`algolia-pp-cli indexes get-object`** - Retrieves one record by its object ID.

To retrieve more than one record, use the [`objects` operation](https://www.algolia.com/doc/rest-api/search/get-objects).
- **`algolia-pp-cli indexes get-objects`** - Retrieves one or more records, potentially from different indices.

Records are returned in the same order as the requests.
- **`algolia-pp-cli indexes list-indices`** - Lists all indices in the current Algolia application.

The request follows any index restrictions of the API key you use to make the request.
- **`algolia-pp-cli indexes multiple-batch`** - Adds, updates, or deletes records in multiple indices with a single API request.

- Actions are applied in the order they are specified.
- Actions are equivalent to the individual API requests of the same name.

This operation is subject to [indexing rate limits](https://support.algolia.com/hc/articles/4406975251089-Is-there-a-rate-limit-for-indexing-on-Algolia).
- **`algolia-pp-cli indexes save-object`** - Adds a record to an index or replaces it.

- If the record doesn't have an object ID, a new record with an auto-generated object ID is added to your index.
- If a record with the specified object ID exists, the existing record is replaced.
- If a record with the specified object ID doesn't exist, a new record is added to your index.
- If you add a record to an index that doesn't exist yet, a new index is created.

To update _some_ attributes of a record, use the [`partial` operation](https://www.algolia.com/doc/rest-api/search/partial-update-object).
To add, update, or replace multiple records, use the [`batch` operation](https://www.algolia.com/doc/rest-api/search/batch).

This operation is subject to [indexing rate limits](https://support.algolia.com/hc/articles/4406975251089-Is-there-a-rate-limit-for-indexing-on-Algolia).
- **`algolia-pp-cli indexes search`** - Runs multiple search queries against one or more indices in a single API request.

Use cases include:

- Searching different indices, such as products and marketing content.
- Run multiple queries on the same index with different parameters or filters.

If you know the expected result type, use the `searchForHits` or `searchForFacets` helper to simplify the response format.

### keys

Manage keys

- **`algolia-pp-cli keys add-api`** - Creates a new API key with specific permissions and restrictions.
- **`algolia-pp-cli keys delete-api`** - Deletes the API key.
- **`algolia-pp-cli keys get-api`** - Gets the permissions and restrictions of an API key.

When authenticating with the admin API key, you can request information for any of your application's keys.
When authenticating with other API keys, you can only retrieve information for that key,
with the description replaced by `<redacted>`.
- **`algolia-pp-cli keys list-api`** - Lists all API keys associated with your Algolia application, including their permissions and restrictions.
- **`algolia-pp-cli keys update-api`** - Replaces the permissions of an existing API key.

Any unspecified attribute resets that attribute to its default value.

### logs

Manage logs

- **`algolia-pp-cli logs`** - The request must be authenticated by an API key with the [`logs` ACL](https://www.algolia.com/doc/guides/security/api-keys/#access-control-list-acl).

- Logs are held for the last seven days.
- Up to 1,000 API requests per server are logged.
- This request counts towards your [operations quota](https://support.algolia.com/hc/articles/17245378392977-How-does-Algolia-count-records-and-operations) but doesn't appear in the logs itself.

### security

Manage security

- **`algolia-pp-cli security append-source`** - Adds a source to the list of allowed sources.
- **`algolia-pp-cli security delete-source`** - Deletes a source from the list of allowed sources.
- **`algolia-pp-cli security get-sources`** - Retrieves all allowed IP addresses with access to your application.
- **`algolia-pp-cli security replace-sources`** - Replaces the list of allowed sources.

### task

Manage task

- **`algolia-pp-cli task <taskID>`** - Checks the status of a given application task.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`algolia-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`algolia-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`algolia-pp-cli learnings list`** - Inspect taught rows
- **`algolia-pp-cli learnings forget <query>`** - Undo a teach
- **`algolia-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`algolia-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`algolia-pp-cli teach-pattern`** - Install a query/resource template up front
- **`algolia-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ALGOLIA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `algolia-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
algolia-pp-cli clusters list

# JSON for scripting and agents
algolia-pp-cli clusters list --json
# Filter to specific fields by name
algolia-pp-cli clusters list --json --select <field>[,<field>...]

# Dry run — show the request without sending
algolia-pp-cli clusters list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
algolia-pp-cli clusters list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
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
- `ALGOLIA_APP_ID` resolves `{appId}`

Base URL: `https://{appId}.algolia.net`

## Health Check

```bash
algolia-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `algolia-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/search-pp-cli/config.toml`; `--home`, `ALGOLIA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ALGOLIA_APPLICATION_ID` | endpoint | Yes |  |
| `ALGOLIA_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `algolia-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `algolia-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ALGOLIA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports missing credentials** — export ALGOLIA_APPLICATION_ID=<your-app-id> and ALGOLIA_API_KEY=<your-api-key>
- **HTTP 400 'Index not found'** — Check the index name — list indices with: algolia-pp-cli indices list
- **HTTP 403 'API key has no ACLs to perform this action'** — Use a key with the required ACL (search, browse, settings, etc.) from the dashboard.
- **Rate limited (HTTP 429)** — Algolia rate limits per key — retry after a few seconds or use a key with higher quota.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**algoliasearch-client-javascript**](https://github.com/algolia/algoliasearch-client-javascript) — JavaScript (1385 stars)
- [**algoliasearch-client-go**](https://github.com/algolia/algoliasearch-client-go) — Go (200 stars)
- [**@algolia/cli**](https://github.com/algolia/cli) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

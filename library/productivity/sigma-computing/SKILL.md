---
name: pp-sigma-computing
description: "The first full-featured CLI for the Sigma Computing API, plus a local mirror, offline search, and governance audits no other Sigma tool has. Trigger phrases: `audit Sigma grants`, `who can see this Sigma workbook`, `offboard a Sigma member`, `bulk provision Sigma members`, `find stale Sigma workbooks`, `export Sigma workbooks`, `use sigma-computing`, `run sigma-computing`."
author: "Chris Hatton"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sigma-computing-pp-cli
---

# Sigma — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sigma-computing-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install sigma-computing --cli-only
   ```
2. Verify: `sigma-computing-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Sigma REST resource as a composable command with --json and a local SQLite store. On top of the raw API it adds governance and provisioning commands the platform forces you to hand-roll: grant audit resolves team grants down to people, member offboard reassigns a leaver's content instead of orphaning it, member provision bulk-onboards from a CSV, and workbook copy fixes the documented ownership bug automatically.

## When to Use This CLI

Use this CLI for Sigma administration and BI delivery automation: bulk member onboarding/offboarding, grant and access audits, stale-content cleanup, workbook copy/export pipelines, and connection/data-model management. It is the right tool when you need scriptable, --json output for CI or governance reviews rather than clicking through the Sigma web UI, and especially when SCIM provisioning is unavailable.

### Do NOT use this CLI for

- **Querying warehouse data or building dashboards.** This manages Sigma *resources* (workbooks, members, grants) via the admin REST API; it does not run SQL against your data warehouse or render visualizations. Use Sigma's UI or the embed SDK for end-user BI.
- **Non-Sigma data sources.** It only speaks the Sigma Computing API. For another BI tool or a generic database, use that tool's own client.
- **Interactive end-user workflows** (exploring a workbook, applying filters in-session). It is an automation/governance tool, not a viewer.
- **Live workbook query results.** `workbooks elements query` returns element metadata, not a streamed result set for analysis — pull data through Sigma's export or warehouse instead.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Governance you can't get from one API call
- **`grant audit`** — Expand a workbook, connection, workspace, or dataset's grants into the full effective member list and flag org-wide or public access.

  _Reach for this when answering 'who can actually see this resource?' — it resolves team grants down to people, which the raw API leaves to you._

  ```bash
  sigma-computing-pp-cli grant audit workbook abc123 --agent
  ```
- **`workbook stale`** — List workbooks untouched for more than N days, joined to their owner and folder path.

  _Reach for this during content cleanup or governance reviews to find abandoned workbooks and who owns them._

  ```bash
  sigma-computing-pp-cli workbook stale --days 90 --agent
  ```
- **`access review`** — Show everything one member can reach across workbooks, connections, and workspaces via direct and team grants.

  _Use this to verify a deactivated member lost access, or to answer a security question about one person's reach._

  ```bash
  sigma-computing-pp-cli access review user@example.com --agent
  ```

### Bundled correctness
- **`workbook copy`** — Copy a workbook and automatically reassign ownership to the intended recipient instead of the calling admin.

  _Use this instead of the raw copy endpoint whenever the new workbook should belong to someone other than the API service account._

  ```bash
  sigma-computing-pp-cli workbook copy abc123 --to user@example.com
  ```
- **`member offboard`** — Deactivate a member and reassign every workbook and file they own to another member in one command.

  _Use this when someone leaves so their content doesn't become orphaned — the single biggest admin offboarding gap._

  ```bash
  sigma-computing-pp-cli member offboard leaver@example.com --transfer-to manager@example.com --dry-run
  ```

### Provisioning at scale
- **`member provision`** — Create or update members in bulk from a CSV, assigning teams and user attributes idempotently in one pass.

  _Reach for this for Monday onboarding batches when SCIM isn't available and clicking through the UI per-person isn't viable._

  ```bash
  sigma-computing-pp-cli member provision --from new-hires.csv --dry-run
  ```
- **`export bulk`** — Resolve a set of workbooks by offline name/path search and export them all to CSV, PDF, or XLSX in one invocation.

  _Reach for this for recurring stakeholder exports where you want every workbook matching a name pattern, not one at a time._

  ```bash
  sigma-computing-pp-cli export bulk --query "finance" --format pdf
  ```

## Command Reference

**account-types** — Manage account types

- `sigma-computing-pp-cli account-types create` — Create a custom account type with specified permissions.
- `sigma-computing-pp-cli account-types delete` — Delete a custom account type and reassign its users to another account type.
- `sigma-computing-pp-cli account-types list` — Returns a list of all account types available in the organization.

**allowed-ips** — Manage allowed ips

- `sigma-computing-pp-cli allowed-ips` — Retrieve a paginated list of all IP addresses and CIDR ranges in your organization's IP allowlist.

**allowed-ips-batch-create** — Manage allowed ips batch create

- `sigma-computing-pp-cli allowed-ips-batch-create` — Add new IP address entries to your organization's IP allowlist.

**allowed-ips-batch-delete** — Manage allowed ips batch delete

- `sigma-computing-pp-cli allowed-ips-batch-delete` — Remove existing IP address entries from your organization's IP allowlist.

**api-connectors** — Manage api connectors

- `sigma-computing-pp-cli api-connectors create` — This endpoint creates a new API connector that defines how to call an external HTTP endpoint from within Sigma.
- `sigma-computing-pp-cli api-connectors delete` — This endpoint archives an API connector, preventing it from being used in new workbook actions.
- `sigma-computing-pp-cli api-connectors get` — This endpoint returns full details for a single API connector, including its request parameters and configuration.
- `sigma-computing-pp-cli api-connectors list` — This endpoint returns a paginated list of API connectors.
- `sigma-computing-pp-cli api-connectors update` — This endpoint updates one or more fields on an existing API connector.

**api-credentials** — Manage api credentials

- `sigma-computing-pp-cli api-credentials create` — This endpoint creates a new API credential for use with API connectors and the **Call API** action in Sigma.
- `sigma-computing-pp-cli api-credentials delete` — This endpoint archives an API credential so it can no longer be associated with new API connectors.
- `sigma-computing-pp-cli api-credentials get` — This endpoint returns nonsensitive details for a single API credential.
- `sigma-computing-pp-cli api-credentials list` — This endpoint returns a paginated list of API credentials.
- `sigma-computing-pp-cli api-credentials update` — This endpoint updates one or more fields on an existing API credential.

**connection** — Manage connection


**connections** — Manage connections

- `sigma-computing-pp-cli connections create` — Create a connection to a cloud data warehouse from Sigma.
- `sigma-computing-pp-cli connections create-path-grant` — Add a grant to a specific connection path to grant permissions for users or teams.
- `sigma-computing-pp-cli connections delete` — Delete a specific connection by connection ID.
- `sigma-computing-pp-cli connections delete-path-grant` — Delete a specific permission granted to a specific connection path for a user or team.
- `sigma-computing-pp-cli connections get` — Get the metadata of a specific connection by connection ID.
- `sigma-computing-pp-cli connections get-inode-path` — Get the connection path for a specific table. If the inodeId is not for a table, this endpoint returns an error.
- `sigma-computing-pp-cli connections list` — Get a list of available connections and the connection IDs.
- `sigma-computing-pp-cli connections list-path-grants` — Get a list of permissions granted to a specific connection path.
- `sigma-computing-pp-cli connections list-paths` — List all paths for all connections available to the user.
- `sigma-computing-pp-cli connections list-table-columns` — Returns column names, types, and other details for a table in a data warehouse connection.
- `sigma-computing-pp-cli connections update` — Update a specific connection by connection ID.
- `sigma-computing-pp-cli connections update-deprecated` — Update the metadata of a specific connection. This endpoint is deprecated. Instead, use the [PUT endpoint](https://help.
- `sigma-computing-pp-cli connections v3alpha-create` — Create a connection to a cloud data warehouse from Sigma.
- `sigma-computing-pp-cli connections v3alpha-get` — Get the metadata of a specific connection by connection ID.
- `sigma-computing-pp-cli connections v3alpha-patch` — Update a connection to a cloud data warehouse from Sigma. Only the options specified in your request are updated.

**credentials** — Manage credentials

- `sigma-computing-pp-cli credentials create` — Create API client credentials for a given user.
- `sigma-computing-pp-cli credentials delete` — Revoke the API client credentials associated with a given client ID.

**data-models** — Manage data models

- `sigma-computing-pp-cli data-models create-spec` — This endpoint creates a new data model in Sigma from a code representation.
- `sigma-computing-pp-cli data-models get` — Get details of a specific data model by `dataModelId`.
- `sigma-computing-pp-cli data-models list` — This endpoint retrieves a list of all data models available.
- `sigma-computing-pp-cli data-models materialize-element` — This endpoint runs a scheduled materialization for an element in a data model.
- `sigma-computing-pp-cli data-models tag` — Add a version tag to a data model and optionally set up a connection to swap to for a specific version of the data
- `sigma-computing-pp-cli data-models v3alpha-source-swap` — Swap each data source used by a data model.

**datasets** — Manage datasets

- `sigma-computing-pp-cli datasets get` — Get a specific dataset by datasetId **Deprecation notice**: [Datasets](doc:datasets) are deprecated.
- `sigma-computing-pp-cli datasets list` — Get a list of available datasets.

**deployment-policies** — Manage deployment policies

- `sigma-computing-pp-cli deployment-policies archive-deployment` — Archive a deployment policy.
- `sigma-computing-pp-cli deployment-policies create-deployment` — Create a deployment policy to define what documents to deploy to a tenant organization and how to swap sources for
- `sigma-computing-pp-cli deployment-policies get-deployment` — Get details for a deployment policy.
- `sigma-computing-pp-cli deployment-policies list-deployable-tenants` — Returns a paginated list of tenant organizations that the calling organization can deploy to.
- `sigma-computing-pp-cli deployment-policies list-deployments` — List all deployment policies set up for an organization.
- `sigma-computing-pp-cli deployment-policies update-deployment` — Update a deployment policy. Only the fields included in the request are changed.

**favorites** — Manage favorites

- `sigma-computing-pp-cli favorites add` — Favorite a folder or document for a specific user.
- `sigma-computing-pp-cli favorites list` — Get the favorite documents for a specific user.
- `sigma-computing-pp-cli favorites remove` — Unfavorite a folder or document for a specific user.

**files** — Manage files

- `sigma-computing-pp-cli files create` — Create an empty workspace, folder, workbook, or report in Sigma.
- `sigma-computing-pp-cli files delete` — Delete a folder or document, such as a workbook, data model, or report.
- `sigma-computing-pp-cli files get` — Get information about a specific document or folder, such as a workbook, report, or data model.
- `sigma-computing-pp-cli files list` — List all documents, such as workbooks and folders, accessible from the parent.
- `sigma-computing-pp-cli files update` — Update a folder or document, such as a workbook, data model, or report.

**grants** — Manage grants

- `sigma-computing-pp-cli grants create` — Create a grant or update an existing grant on a document, folder, or workspace to a user or a team by ID.
- `sigma-computing-pp-cli grants delete` — Delete a grant by grant ID.
- `sigma-computing-pp-cli grants get` — Return a grant object by grant ID.
- `sigma-computing-pp-cli grants list` — List all grants for a given object. ### Usage notes You can specify one of the following: - A user by userId.

**members** — Manage members

- `sigma-computing-pp-cli members 1-list` — **Attention:** This API endpoint uses pagination by default. List all users in Sigma.
- `sigma-computing-pp-cli members create` — Create a user. ### Usage notes - Creating a user with this endpoint sends an email invitation to the user.
- `sigma-computing-pp-cli members delete` — Deactivate a specific user by memberId. Users cannot be fully deleted, only deactivated.
- `sigma-computing-pp-cli members get` — Returns a specific user by member ID.
- `sigma-computing-pp-cli members list` — **Attention**: This endpoint will return only paginated responses starting June 2, 2026.
- `sigma-computing-pp-cli members update` — Update a specific user by memberId.

**organizations** — Manage organizations

- `sigma-computing-pp-cli organizations` — Update settings for the organization associated with the API credentials.

**plugins** — Manage plugins

- `sigma-computing-pp-cli plugins create-custom` — Register a new custom plugin in the organization.
- `sigma-computing-pp-cli plugins delete-custom` — Permanently delete a custom plugin.
- `sigma-computing-pp-cli plugins get-custom` — Get the metadata for a custom plugin by ID.
- `sigma-computing-pp-cli plugins list-custom` — List custom plugins owned by the current organization.
- `sigma-computing-pp-cli plugins update-custom` — Update metadata fields on an existing custom plugin. Only fields included in the request are changed.

**query** — Manage query


**reports** — Manage reports

- `sigma-computing-pp-cli reports create` — This endpoint lets you create an empty report in Sigma
- `sigma-computing-pp-cli reports get` — This endpoint retrieves a report by its unique identifier (`reportId`).
- `sigma-computing-pp-cli reports list` — This endpoint retrieves a list of all available reports.

**saml** — Manage saml

- `sigma-computing-pp-cli saml activate-service-provider-certificate` — Activate a certificate.
- `sigma-computing-pp-cli saml create-service-provider-certificate` — Create a certificate for a given SAML service provider.
- `sigma-computing-pp-cli saml deactivate-service-provider-certificate` — Deactivate a certificate.
- `sigma-computing-pp-cli saml get-service-provider-certificate` — Get a certificate to use with your IdP when configuring Sigma as a service provider.
- `sigma-computing-pp-cli saml list-service-provider-certificates` — List all certificates for a given SAML service provider.
- `sigma-computing-pp-cli saml list-service-providers` — List all SAML service providers in the organization.
- `sigma-computing-pp-cli saml remove-service-provider-certificate` — Remove a certificate.

**shared-templates** — Manage shared templates

- `sigma-computing-pp-cli shared-templates accept-template-share` — If a workbook template has been shared with your organization but not yet accepted, accept the template share.
- `sigma-computing-pp-cli shared-templates delete-external-share` — Remove a template shared with your organization.
- `sigma-computing-pp-cli shared-templates list-templates-shared-with-you` — Lists all workbook templates shared with your organization.

**sigma-computing-public-auth** — Manage sigma computing public auth

- `sigma-computing-pp-cli sigma-computing-public-auth` — Use your Sigma client ID and secret with this endpoint to generate an access token valid for one hour

**source-swap-policies** — Manage source swap policies

- `sigma-computing-pp-cli source-swap-policies create-source-swap-policy` — Create a source swap policy.
- `sigma-computing-pp-cli source-swap-policies delete-source-swap-policy` — Delete a source swap policy by policy ID.
- `sigma-computing-pp-cli source-swap-policies get-source-swap-policy` — Get a source swap policy by policy ID.
- `sigma-computing-pp-cli source-swap-policies list` — Get a list of source swap policies.
- `sigma-computing-pp-cli source-swap-policies update-source-swap-policy` — Update a source swap policy.

**tags** — Manage tags

- `sigma-computing-pp-cli tags create-version` — Create a version tag to use, for example, for workbooks.
- `sigma-computing-pp-cli tags delete-version` — Delete a specific version tag.
- `sigma-computing-pp-cli tags list-version` — Get a list of version tags.
- `sigma-computing-pp-cli tags update-version` — Update the description of a specific version tag.

**teams** — Manage teams

- `sigma-computing-pp-cli teams 1-list` — **Attention:** This API endpoint uses pagination by default. List all teams in Sigma.
- `sigma-computing-pp-cli teams create` — Create a Sigma team. ### Usage notes - Specify members to add to the team by userId or memberId.
- `sigma-computing-pp-cli teams delete` — Delete a specific team. ### Usage notes - Retrieve the **teamId** by calling the [/v2/teams](listteams) endpoint.
- `sigma-computing-pp-cli teams get` — Returns details about a team based on team ID.
- `sigma-computing-pp-cli teams list` — **Attention**: This endpoint will return only paginated responses starting June 2, 2026.
- `sigma-computing-pp-cli teams update` — Update the name, description, and visibility of a team.

**templates** — Manage templates

- `sigma-computing-pp-cli templates get` — Get a template by template ID.
- `sigma-computing-pp-cli templates list` — Returns a list of available templates. ### Usage notes - Official Sigma templates are created by SigmaSchedulerRobot.
- `sigma-computing-pp-cli templates save-workbook-from` — Create a workbook by saving it from a template.

**tenants** — Manage tenants

- `sigma-computing-pp-cli tenants create` — Create a new tenant organization with the specified name and organization slug to be used in the URL.
- `sigma-computing-pp-cli tenants delete` — Delete a tenant organization by tenantOrganizationId.
- `sigma-computing-pp-cli tenants get` — Retrieve details of a specific tenant organization by tenantOrganizationId.
- `sigma-computing-pp-cli tenants list` — Retrieve a paginated list of tenant organizations with optional filtering and sorting.
- `sigma-computing-pp-cli tenants patch` — Update the details of an existing tenant organization.

**translations** — Manage translations

- `sigma-computing-pp-cli translations create-org` — This endpoint creates a new organization translation file for the specified locale
- `sigma-computing-pp-cli translations delete-org` — This endpoint deletes the translation file for the specified locale (**lng**) without any custom translations.
- `sigma-computing-pp-cli translations delete-org-with-variant` — This endpoint deletes the translation file for the specified custom translation (**lng_variant**) for a locale (**lng**)
- `sigma-computing-pp-cli translations get-org` — This endpoint returns a JSON file containing the translations for a specified locale (**lng**)
- `sigma-computing-pp-cli translations get-org-with-variant` — This endpoint returns a JSON file containing the translations and custom translations (**lng_variant**)
- `sigma-computing-pp-cli translations list-org-locales` — This paginated endpoint lists all the translation files that have been defined at the organization level.
- `sigma-computing-pp-cli translations update-org` — This endpoint updates the translation file for a specified locale (**lng**) without custom translations.
- `sigma-computing-pp-cli translations update-org-with-variant` — This endpoint updates the translation file for the specified custom translation (**lng_variant**) for a locale (**lng**)

**user-attributes** — Manage user attributes

- `sigma-computing-pp-cli user-attributes create` — Create a new user attribute. An optional description and default value can be provided.
- `sigma-computing-pp-cli user-attributes delete` — Delete a user attribute.
- `sigma-computing-pp-cli user-attributes get` — Get details for a specific user attribute.
- `sigma-computing-pp-cli user-attributes list` — Get a list of available user attributes, values, and owners.

**webhooks** — Manage webhooks

- `sigma-computing-pp-cli webhooks <workbookId> <sequenceId>` — Post data to a webhook endpoint. The request body must match the configured webhook specification configured in Sigma.

**whoami** — Manage whoami

- `sigma-computing-pp-cli whoami` — Get the identity and authentication status of the current user.

**workbooks** — Manage workbooks

- `sigma-computing-pp-cli workbooks create` — This endpoint lets you create an empty workbook in Sigma
- `sigma-computing-pp-cli workbooks create-spec` — This endpoint creates a new workbook from a code representation.
- `sigma-computing-pp-cli workbooks get` — This endpoint retrieves a workbook by its unique identifier (`workbookId`).
- `sigma-computing-pp-cli workbooks list` — This endpoint retrieves a list of all available workbooks.
- `sigma-computing-pp-cli workbooks tag` — Add a version tag to a workbook and optionally set up a connection to swap to for a specific version of the workbook.
- `sigma-computing-pp-cli workbooks v3alpha-source-swap` — Swap each data source used by a workbook.

**workspaces** — Manage workspaces

- `sigma-computing-pp-cli workspaces 1-list` — **Attention:** This API endpoint uses pagination by default. This endpoint returns all workspaces.
- `sigma-computing-pp-cli workspaces create` — This endpoint allows clients to create a workspace with specific characteristics.
- `sigma-computing-pp-cli workspaces delete` — You can use this endpoint to delete an existing workspace by its workspaceId.
- `sigma-computing-pp-cli workspaces get` — This endpoint retrieves the details of a specific workspace by its workspaceId.
- `sigma-computing-pp-cli workspaces list` — **Attention**: This endpoint will return only paginated responses starting June 2, 2026.
- `sigma-computing-pp-cli workspaces update` — This endpoint updates the name of an existing workspace identified by its workspaceId.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sigma-computing-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Audit who can see a connection

```bash
sigma-computing-pp-cli grant audit connection conn123 --agent --select email,grantedVia
```

Resolves direct and team grants into the effective member list, showing how each person got access.

### Offboard a departing employee safely

```bash
sigma-computing-pp-cli member offboard leaver@example.com --transfer-to manager@example.com --dry-run
```

Previews the deactivation plus every workbook reassignment before committing; drop --dry-run to apply.

### Bulk-onboard new hires

```bash
sigma-computing-pp-cli member provision --from new-hires.csv --dry-run
```

Creates members and assigns teams + user attributes idempotently from a CSV; safe to re-run.

### Find abandoned workbooks for cleanup

```bash
sigma-computing-pp-cli workbook stale --days 120 --json --select name,ownerEmail,path
```

Lists workbooks untouched for 120+ days with just the fields you need to triage.

### Export every finance workbook to PDF

```bash
sigma-computing-pp-cli export bulk --query finance --format pdf
```

Offline search resolves the matching workbooks, then exports them all in one pass.

## Auth Setup

Sigma uses OAuth2 client-credentials. Create an API client under Administration > Developer Access to get a client ID and secret. Run 'auth login' to store them, or set SIGMA_COMPUTING_CLIENT_ID, SIGMA_COMPUTING_CLIENT_SECRET, and SIGMA_COMPUTING_BASE_URL (your org's API base URL, also under Developer Access). The CLI exchanges these for a ~1-hour bearer token and refreshes it automatically.

Run `sigma-computing-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sigma-computing-pp-cli account-types list --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `SIGMA_COMPUTING_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SIGMA_COMPUTING_CONFIG_DIR`, `SIGMA_COMPUTING_DATA_DIR`, `SIGMA_COMPUTING_STATE_DIR`, `SIGMA_COMPUTING_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SIGMA_COMPUTING_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `sigma-computing-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "sigma-computing": {
        "command": "sigma-computing-pp-mcp",
        "env": {
          "SIGMA_COMPUTING_HOME": "/srv/sigma-computing"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SIGMA_COMPUTING_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SIGMA_COMPUTING_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
sigma-computing-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sigma-computing-pp-cli feedback --stdin < notes.txt
sigma-computing-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SIGMA_COMPUTING_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SIGMA_COMPUTING_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sigma-computing-pp-cli profile save briefing --json
sigma-computing-pp-cli --profile briefing account-types list
sigma-computing-pp-cli profile list --json
sigma-computing-pp-cli profile show briefing
sigma-computing-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sigma-computing-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add sigma-computing-pp-mcp -- sigma-computing-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sigma-computing-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sigma-computing-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sigma-computing-pp-cli <command> --help`.

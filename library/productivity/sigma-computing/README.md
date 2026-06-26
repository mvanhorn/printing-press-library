# Sigma CLI

**The first full-featured CLI for the Sigma Computing API, plus a local mirror, offline search, and governance audits no other Sigma tool has.**

Every Sigma REST resource as a composable command with --json and a local SQLite store. On top of the raw API it adds governance and provisioning commands the platform forces you to hand-roll: grant audit resolves team grants down to people, member offboard reassigns a leaver's content instead of orphaning it, member provision bulk-onboards from a CSV, and workbook copy fixes the documented ownership bug automatically.

## Install

The recommended path installs both the `sigma-computing-pp-cli` binary and the `pp-sigma-computing` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sigma-computing
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sigma-computing --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sigma-computing --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sigma-computing --agent claude-code
npx -y @mvanhorn/printing-press-library install sigma-computing --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sigma-computing-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sigma-computing --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sigma-computing --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sigma-computing --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sigma-computing --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sigma-computing-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SIGMA_COMPUTING_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sigma-computing": {
      "command": "sigma-computing-pp-mcp",
      "env": {
        "SIGMA_COMPUTING_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Sigma uses OAuth2 client-credentials. Create an API client under Administration > Developer Access to get a client ID and secret. Run 'auth login' to store them, or set SIGMA_COMPUTING_CLIENT_ID, SIGMA_COMPUTING_CLIENT_SECRET, and SIGMA_COMPUTING_BASE_URL (your org's API base URL, also under Developer Access). The CLI exchanges these for a ~1-hour bearer token and refreshes it automatically.

## Quick Start

```bash
# Confirm credentials and base URL resolve and the token exchange works.
sigma-computing-pp-cli doctor

# List workbooks to confirm read access and see the resource shape.
sigma-computing-pp-cli workbooks list --json

# Mirror workbooks, members, teams, connections, and grants into the local store for offline search and audits.
sigma-computing-pp-cli sync

# First governance payoff: find abandoned workbooks and their owners.
sigma-computing-pp-cli workbook stale --days 90

```

## Unique Features

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

## Usage

Run `sigma-computing-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SIGMA_COMPUTING_CONFIG_DIR`, `SIGMA_COMPUTING_DATA_DIR`, `SIGMA_COMPUTING_STATE_DIR`, or `SIGMA_COMPUTING_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SIGMA_COMPUTING_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SIGMA_COMPUTING_HOME=/srv/sigma-computing
sigma-computing-pp-cli doctor
```

Under `SIGMA_COMPUTING_HOME=/srv/sigma-computing`, the four dirs resolve to `/srv/sigma-computing/config`, `/srv/sigma-computing/data`, `/srv/sigma-computing/state`, and `/srv/sigma-computing/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `SIGMA_COMPUTING_DATA_DIR` overrides an explicit `--home` for that kind. Use `SIGMA_COMPUTING_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SIGMA_COMPUTING_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `sigma-computing-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account-types

Manage account types

- **`sigma-computing-pp-cli account-types create`** - Create a custom account type with specified permissions.

  ### Usage notes
  - To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
  - Use the permission names from the [/v2/accountTypes/:accountTypeId/permissions](listaccounttypepermissions) endpoint.

  ### Usage scenarios
  - Create custom account types to define specific permission sets for your organization's users.
- **`sigma-computing-pp-cli account-types delete`** - Delete a custom account type and reassign its users to another account type.

  ### Usage notes
  - To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
  - Default Sigma account types cannot be deleted.
  - All users assigned to the deleted account type will be reassigned to the specified **reassignToAccountTypeId**.
  - Retrieve account type IDs by calling the [/v2/accountTypes](listaccounttypes) endpoint.

  ### Usage scenarios
  - Remove custom account types that are no longer needed.
  - Consolidate account types by moving users to a different account type before deletion.
- **`sigma-computing-pp-cli account-types list`** - Returns a list of all account types available in the organization.

  ### Usage notes
  - Use the **accountTypeId** with the [/v2/accountTypes/:accountTypeId/permissions](listaccounttypepermissions) endpoint to retrieve more detailed permissions.
  - To perform this operation, you must use API credentials owned by a user assigned the Admin account type.

  ### Usage scenarios
  - Display available account types in an admin interface.
  - Show an overview of account types for management.

### allowed-ips

Manage allowed ips

- **`sigma-computing-pp-cli allowed-ips`** - Retrieve a paginated list of all IP addresses and CIDR ranges in your organization's IP allowlist.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

  ### Usage notes
  - Returns the IP allowlist entries.
  - Results are sorted by creation date in descending order (newest first).

### allowed-ips-batch-create

Manage allowed ips batch create

- **`sigma-computing-pp-cli allowed-ips-batch-create`** - Add new IP address entries to your organization's IP allowlist.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

  ### Usage notes
  - Supports IPv4, IPv6, and CIDR ranges (e.g., `192.168.1.0/24`, `2001:db8::/32`).
  - Maximum of 200 total IP entries allowed per organization.
  - Duplicate IP addresses are not allowed. If a request contains an IP address already on the list, the request fails.

  ### Usage scenarios
  - Add IP ranges for new locations or employees.
  - Bulk import IP addresses from a list.

### allowed-ips-batch-delete

Manage allowed ips batch delete

- **`sigma-computing-pp-cli allowed-ips-batch-delete`** - Remove existing IP address entries from your organization's IP allowlist.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

  ### Usage notes
  - Retrieve the `ipAllowlistEntryId` values with the [/v3alpha/allowedIps](v3alpha_listipallowlistentries) endpoint or from the response of the [/v3alpha/allowedIps:batchCreate](v3alpha_createipallowlistentries) endpoint.
  - Entries are identified by their unique IDs, not by IP addresses.

  ### Usage scenarios
  - Remove compromised or outdated IP addresses from access.
  - Clean up unused IP allowlist entries.

### api-connectors

Manage api connectors

- **`sigma-computing-pp-cli api-connectors create`** - This endpoint creates a new API connector that defines how to call an external HTTP endpoint from within Sigma. For more information on API connectors, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- The user making this request must be assigned an account type with the **Manage API connectors** permission enabled.
- If a credential is provided using the `authId` parameter, the user making this request must have at least **Can view** permission on the credential and the request URL must match the credential's allowlist.
- Retrieve the **apiCredentialId** (used as `authId`) by calling the [/v2/api-credentials](listapicredentials) endpoint.

### Usage scenarios
- **Programmatic connector setup:** Automate creation of API connectors as part of an environment provisioning workflow.
- **Integration onboarding:** Create connectors for each external service your workbooks need to interact with.
- **`sigma-computing-pp-cli api-connectors delete`** - This endpoint archives an API connector, preventing it from being used in new workbook actions. For more information on API connectors, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- Retrieve the **apiConnectorId** by calling the [/v2/api-connectors](listapiconnectors) endpoint.
- The user making this request must be assigned an account type with the **Manage API connectors** permission enabled and must have **Can edit** access to the connector.

### Usage scenarios
- **Cleanup:** Remove connectors that are no longer in use to keep the organization's connector list tidy.
- **Decommissioning:** Archive connectors associated with deprecated or retired external services.
- **`sigma-computing-pp-cli api-connectors get`** - This endpoint returns full details for a single API connector, including its request parameters and configuration. For more information on API connectors, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- Only returns API connectors that the user making this request has at least **Can view** access to.
- Retrieve the **apiConnectorId** by calling the [/v2/api-connectors](listapiconnectors) endpoint.

### Usage scenarios
- **Connector inspection:** Retrieve the full configuration of a connector for display or validation before use.
- **Connector duplication:** Read an existing connector's configuration to use as the basis for a new one.
- **`sigma-computing-pp-cli api-connectors list`** - This endpoint returns a paginated list of API connectors. For more information on API connectors, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- Only returns API connectors that the user making this request has at least **Can view** access to.
- Use the `name` query parameter to filter by connector name.
- Use the `orderBy` query parameter to set sort order.

### Usage scenarios
- **Connector discovery:** Retrieve a list of available API connectors available to users configuring **Call API** actions.
- **Connector management:** View and audit the organization's API connectors.
- **`sigma-computing-pp-cli api-connectors update`** - This endpoint updates one or more fields on an existing API connector. For more information on API connectors, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- Retrieve the **apiConnectorId** by calling the [/v2/api-connectors](listapiconnectors) endpoint.
- The user making this request must be assigned an account type with the **Manage API connectors** permission enabled and must have **Can edit** access to the connector.
- Omitted fields are left unchanged. Pass `null` for `authId` to remove the credential association.

### Usage scenarios
- **Endpoint migration:** Update the connector URL or parameters when an external API changes its interface.
- **Credential rotation:** Rebind the connector to a new credential after rotating secrets.

### api-credentials

Manage api credentials

- **`sigma-computing-pp-cli api-credentials create`** - This endpoint creates a new API credential for use with API connectors and the **Call API** action in Sigma. For more information on API credentials, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- The user making this request must be assigned an account type with the **Manage API connectors** permission enabled.
- The `allowlist` parameter is required and must contain at least one hostname glob pattern. Use `["*"]` to allow the credential to be used against any host.
- The following authentication methods are supported: `basic`, `bearer`, `apiKey`, `oAuthClientCredentials`, `oAuthAuthorizationCode`, `oAuthPasswordCredentials`, `awsSigV4`.
- Secret fields are encrypted at rest and are never returned in subsequent read responses.

### Usage scenarios
- **Credential provisioning:** Automate credential creation as part of environment setup or onboarding.
- **Multi-service authentication:** Create separate credentials for each external service, with allowlists scoped to only that service's domains.
- **`sigma-computing-pp-cli api-credentials delete`** - This endpoint archives an API credential so it can no longer be associated with new API connectors. For more information on API credentials, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- The user making this request must be assigned an account type with the **Manage API connectors** permission enabled and have **Can edit** access to the API credential.
- Retrieve the **apiCredentialId** by calling the [/v2/api-credentials](listapicredentials) endpoint.
- Archiving a credential does not automatically unbind any API connectors that reference it.

### Usage scenarios
- **Credential decommissioning:** Remove credentials for decommissioned services or expired tokens.
- **Security cleanup:** Archive compromised or rotated credentials to prevent accidental reuse.
- **`sigma-computing-pp-cli api-credentials get`** - This endpoint returns nonsensitive details for a single API credential. For more information on API credentials, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- The user making this request must be assigned an account type with the **Manage API connectors** permission enabled and have at least **Can view** access on the credential.
- Retrieve the **apiCredentialId** by calling the [/v2/api-credentials](listapicredentials) endpoint.
- Secret fields (passwords, tokens, client secrets, secret access keys) are never returned.

### Usage scenarios
- **Credential inspection:** Retrieve the configuration of a credential to verify its settings before using it with a connector.
- **`sigma-computing-pp-cli api-credentials list`** - This endpoint returns a paginated list of API credentials. For more information on API credentials, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- The user making this request must be assigned an account type with the **Manage API connectors** permission enabled.
- Only returns [API credentials](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma#add-a-new-api-credential-to-sigma) that the user making this request has at least **Can view** access to.
- Secret fields such as passwords, tokens, and client secrets are never included in the response.
- Use the `name` query parameter to filter by credential name.
- Use the `orderBy` query parameter to set sort order.

### Usage scenarios
- **Credential management:** View and audit the credentials used by API connectors in your Sigma organization.
- **API connector creation:** Retrieve available credentials to present as options when creating an API connector.
- **`sigma-computing-pp-cli api-credentials update`** - This endpoint updates one or more fields on an existing API credential. For more information on API credentials, see [Configure API credentials and connectors in Sigma](https://help.sigmacomputing.com/docs/configure-api-credentials-and-connectors-in-sigma).

### Usage notes
- The user making this request must be assigned an account type with the **Manage API connectors** permission enabled and have **Can edit** access to the API credential.
- Retrieve the **apiCredentialId** by calling the [/v2/api-credentials](listapicredentials) endpoint.
- Omitted fields are left unchanged.
- If a `credential` is provided, the provided authentication details (ID, secret, etc.) replace the previous values. To leave secrets unchanged, omit the `credential` parameter.

### Usage scenarios
- **Secret rotation:** Update the secret fields for a credential automatically for an external service.
- **Allowlist updates:** Expand or restrict the set of hostnames the credential can be used against.

### connection

Manage connection


### connections

Manage connections

- **`sigma-computing-pp-cli connections create`** - Create a connection to a cloud data warehouse from Sigma. For additional details, see [Connect to data sources](https://help.sigmacomputing.com/docs/connect-to-data-sources).
- **`sigma-computing-pp-cli connections create-path-grant`** - Add a grant to a specific connection path to grant permissions for users or teams.

  ### Usage notes
  - Retrieve the **connectionPathId** by calling the [/v2/connection/{connectionId}/lookup](lookupconnection) endpoint and using the `inodeId` included in the response, or by calling the [/v2/connections/paths](listconnectionpaths) endpoint and using the `urlId` included in the response.
  - Specify the team or user IDs to grant permissions:

    - Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.
    - Retrieve the **teamId** by calling the [/v2/teams](listteams) endpoint.
- **`sigma-computing-pp-cli connections delete`** - Delete a specific connection by connection ID.

  ### Usage notes
  - Retrieve the **connectionId** by calling the [/v2/connections](listconnections) endpoint.
- **`sigma-computing-pp-cli connections delete-path-grant`** - Delete a specific permission granted to a specific connection path for a user or team.

  ### Usage notes
  - Retrieve the **connectionPathId** by calling the [/v2/connection/{connectionId}/lookup](lookupconnection) endpoint and using the `inodeId` included in the response, or by calling the [/v2/connections/paths](listconnectionpaths) endpoint and using the `urlId` included in the response.
  - Retrieve the **grantId** by calling the [/v2/connections/paths/{connectionPathId}/grants](listconnectionpathgrants) endpoint.
- **`sigma-computing-pp-cli connections get`** - Get the metadata of a specific connection by connection ID.

### Usage notes
- Retrieve the **connectionId** by calling the [/v2/connections](listconnections) endpoint.
- **`sigma-computing-pp-cli connections get-inode-path`** - Get the connection path for a specific table. If the inodeId is not for a table, this endpoint returns an error.

  ### Usage notes
  - Retrieve the **inodeId** by calling the [/v2/workbooks/{workbookId}/sources](getworkbooksources) or [/v2/datasets/{datasetId}/source](getdatasetsources) endpoint.
- **`sigma-computing-pp-cli connections list`** - Get a list of available connections and the connection IDs.
- **`sigma-computing-pp-cli connections list-path-grants`** - Get a list of permissions granted to a specific connection path.

  ### Usage notes
  - Retrieve the **connectionPathId** by calling the [/v2/connection/{connectionId}/lookup](lookupconnection) endpoint and using the `inodeId` included in the response, or by calling the [/v2/connections/paths](listconnectionpaths) endpoint and using the `urlId` included in the response.
- **`sigma-computing-pp-cli connections list-paths`** - List all paths for all connections available to the user.

  ### Usage notes
  - Call this endpoint to retrieve the specific databases, catalogs, schemas, and tables available to the user associated with the API credentials.
- **`sigma-computing-pp-cli connections list-table-columns`** - Returns column names, types, and other details for a table in a data warehouse connection.

### Usage notes
- Retrieve the **tableId** by first calling the [/v2/connections/paths](listconnectionpaths) endpoint to discover available paths, then calling the [/v2/connection/{connectionId}/lookup](lookupconnection) endpoint with the path to the desired table and using the `inodeId` included in the response.
- **`sigma-computing-pp-cli connections update`** - Update a specific connection by connection ID. When updating the connection, send any connection details that you want to keep. Retrieve the current state of a connection by calling the [/v2/connections/{connectionId}](getconnection) endpoint.

  ### Usage notes
  - To restore a deleted connection, pass the `restore` parameter.
  - Retrieve the **connectionId** by calling the [/v2/connections](listconnections) endpoint.
- **`sigma-computing-pp-cli connections update-deprecated`** - Update the metadata of a specific connection. This endpoint is deprecated. Instead, use the [PUT endpoint](https://help.sigmacomputing.com/reference/updateconnection).
- **`sigma-computing-pp-cli connections v3alpha-create`** - Create a connection to a cloud data warehouse from Sigma. For additional details, see [Connect to data sources](https://help.sigmacomputing.com/docs/connect-to-data-sources).

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage Notes
- Retrieve the **userAttributeId** by calling the [/v2/user-attributes](listuserattributes) endpoint.
- **`sigma-computing-pp-cli connections v3alpha-get`** - Get the metadata of a specific connection by connection ID.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the **connectionId** by calling the [/v2/connections](listconnections) endpoint.
- **`sigma-computing-pp-cli connections v3alpha-patch`** - Update a connection to a cloud data warehouse from Sigma. Only the options specified in your request are updated. Other options remain the same.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the **connectionId** by calling the [/v2/connections](listconnections) endpoint.

### Usage scenarios
- Remove an optional parameter by setting it to `null`. If the field has a default value, the field resets to the default.
- Update a field value by providing a new value.
- Leave a field unchanged by omitting it from the request.

### credentials

Manage credentials

- **`sigma-computing-pp-cli credentials create`** - Create API client credentials for a given user. To make this request, your API client credentials must be associated with a Sigma admin user for the same Sigma organization.

  **Important:** The response includes sensitive information. Ensure that you securely store and handle the returned `clientId` and `clientSecret`. These credentials grant access to your Sigma account and should never be exposed publicly or shared unnecessarily.

  ### Usage Notes:
  - Retrieve the **ownerId** by calling the [/v2/members](listmembers) endpoint and using the `memberId` included in the response.
- **`sigma-computing-pp-cli credentials delete`** - Revoke the API client credentials associated with a given client ID. To make this request, your API client credentials must be associated with a Sigma admin user for the same Sigma organization.

### data-models

Manage data models

- **`sigma-computing-pp-cli data-models create-spec`** - This endpoint creates a new data model in Sigma from a code representation. Use it to programmatically define and create data models that are then accessible from the Sigma UI.

For more information on managing data models via the Sigma API, see [Manage data models as code](https://help.sigmacomputing.com/docs/manage-data-models-as-code). For more information on using this endpoint, including an end-to-end example, see [Create a data model from a code representation](https://help.sigmacomputing.com/docs/create-a-data-model-from-a-code-representation).

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned an account type with **Create, edit, and publish data models** permission.
- Retrieve a representation of an existing data model by calling the [/v2/dataModels/{dataModelId}/spec](getdatamodelspec) endpoint.
- To get a list of folders, call the [/v2/files](fileslist) endpoint and review the `id` field in the response for files with a `type` of `folder`.
- The default format of the representation is JSON. To use YAML, add the header `Content-Type: application/yaml`.

### Usage scenarios
- **Automation**: If you want to create several data models across one or more organizations, you can use this endpoint to programmatically create the data models.
- **Integration**: Using this endpoint, you can create data models based on external data sources or integrate Sigma with other tools and platforms.
- **Migration**: You can use this endpoint to migrate data models from one organization to another.
- **Version control**: Developers can use this endpoint to programmatically manage, update, and version control data models between several organizations or tenants.
- **Agentic workflows**: Agents can use these endpoints to directly manage data model contents in code.
- **`sigma-computing-pp-cli data-models get`** - Get details of a specific data model by `dataModelId`.

  ### Usage notes
  - Retrieve the **dataModelId** by calling the [/v2/dataModels](listdatamodels) endpoint.
- **`sigma-computing-pp-cli data-models list`** - This endpoint retrieves a list of all data models available. You can use the response from this endpoint to review existing data models and determine if there are duplicates or gaps.

### Usage notes
This endpoint requires no parameters for basic requests, but supports query parameters for pagination and response limit.

Users with the Admin account type can optionally retrieve all data models in the organization.

#### Pagination
This endpoint supports pagination, which lets you retrieve large sets of data in manageable segments. The response includes pagination details as follows:
- hasMore: A boolean value indicating whether there are more pages of data available beyond the current page.
- total: The total number of entries available across all pages.
- nextPage: An identifier or token that you can use in a subsequent request to retrieve the next page of data.

#### Example response for pagination
{
  "hasMore": true,
  "total": 104,
  "nextPage": "50"
}
To request additional pages, include the `nextPage` option in your followup request according to the endpoint's parameter requirements. This process can be repeated until the `nextPage` option returns `null`, indicating that no further data is available.

### Usage scenarios
- **Data model navigation:** Allows users to easily navigate through their available of data models and access the one they need.
- **Track lineage**: Identify dependencies and lineage of data sources and data models.
- **`sigma-computing-pp-cli data-models materialize-element`** - This endpoint runs a scheduled materialization for an element in a data model. Materialization processes the data of the specified element, allowing the data to be stored or cached for optimized access and performance.

  For more details on materialization, see [Materialization](https://help.sigmacomputing.com/docs/materialization).

  ### Usage notes
  - The materialization schedule for the specified element must be created beforehand.
  - Retrieve the **sheetId** by calling the [/v2/dataModels/{dataModelId}/materialization-schedules](listdatamodelmaterializationschedules) endpoint.
  - Retrieve the **dataModelId** by calling the [/v2/dataModels](listdatamodels) endpoint.

  ### Usage scenarios
  - **Performance optimization:** Use this endpoint to improve response times for frequently accessed data model elements.
  - **Data refresh:** Allows users to manually (programmatically) refresh the data of specific data model elements to ensure that the latest data is available for analysis and reporting.

  ### Best practices
  - Prioritize materialization for elements that are heavily used or form critical components of business reports.
  - Monitor the performance impacts of materialization and adjust strategies as necessary to optimize resource usage and response times.
- **`sigma-computing-pp-cli data-models tag`** - Add a version tag to a data model and optionally set up a connection to swap to for a specific version of the data model.

  ### Usage notes
  - Retrieve the **dataModelId** by calling the [/v2/dataModels](listdatamodels) endpoint.
  - Retrieve the **tag** by calling the [/v2/tags](listversiontag) endpoint and using the `name` in the response.
  - Retrieve the **connectionId** to use as the **fromId** or **toId** by calling the [/v2/connections](listconnections) endpoint.
  - If your data model includes a source that is not mapped to a new source, that source is not swapped.
  - When swapping data models used as the source for the data model:
    - You can only swap a data model source to another version of the same data model source. You cannot swap a data model source to a table in your data warehouse or a dataset.
    - When you swap sources from one data model version to a new one, specify the version tag of the data model that you swap to with `toVersionTagId`:
      - To swap to the latest published version of the data model, specify `toVersionTagId` as `null`.
      - To swap to a specific tagged version of the data model, specify the `toVersionTagId` of the data model.
    - If the data model already uses a specific tagged version of a data model as a source, use `fromVersionTagId` to indicate which tagged version to swap from.
      - To swap from the latest published version of the data model, specify `fromVersionTagId` as `null`.
      - To swap from a specific tagged version of the data model, specify the `fromVersionTagId` of the data model.
  - To retrieve the `fromVersionTagId` for a data model used as the data model source, call the [/v2/dataModels/{dataModelId}/sources](listdatamodelsources) endpoint and use the `versionTagId` in the response.
  - To retrieve the `versionTagId` for a data model, call the [/v2/dataModels](listdatamodels) endpoint and use the `versionTagId` in the response.

  ### Usage scenarios
  - **Lifecycle management**: Identify production and development resources.
- **`sigma-computing-pp-cli data-models v3alpha-source-swap`** - Swap each data source used by a data model. You can swap from one table, dataset, data model element, or custom SQL element to another for each data source.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the **dataModelId** by calling the [/v2/dataModels](listdatamodels) endpoint.
- Retrieve data model sources by calling the [/v2/dataModels/{dataModelId}/sources](listdatamodelsources) endpoint.
- Retrieve the **datasetId** by calling the [/v2/datasets](listdatasets) endpoint.
- Retrieve the **tableId** by calling the [/v2/connection](lookupconnection) endpoint.
- Retrieve the **connectionId** for a new custom SQL element by calling the [/v2/connections](listconnections) endpoint.

- Use the `sourceMapping` options in the request body to swap specific sources. For each source in the source mapping, provide the current source in use in the **from** object, and provide the source that you want to use instead in the **to** object.
- If your source is a data model with one or more tagged versions, you can also provide a `versionTagId` to swap to or from a tagged version of the data model. The data model version must already be tagged.
- When swapping **from** a custom SQL element, identify it by its element ID using the `customSqlId` field. When swapping **to** a custom SQL element, supply the `connectionId` of the connection to run against and the SQL query in the `definition` field.
- If your source is a data model or a dataset, you can use the `metricMapping` option to map metrics that rely on column names from the old source to the new one so that they continue to work.
- If the source that you swap from uses column names, links, or relationships different from the source that you are swapping to, use the `columnMapping` option to map each column in the original source to a corresponding column in the new source.
- When `columnMapping` is omitted, columns are automatically matched by display name, ignoring differences in case and punctuation. For example, a custom SQL output column named `DOCK_COUNT` automatically maps to a column with display name `Dock Count` in the target source. Provide `columnMapping` explicitly to override the automatic match for a column.

### Usage scenarios
- **Data model development**: Use a test data source during development, then swap to a production source when ready.
- **Dataset migration**: Update references to legacy dataset sources with data model sources by mapping dataset IDs to their new data model equivalents.

### datasets

Manage datasets

- **`sigma-computing-pp-cli datasets get`** - Get a specific dataset by datasetId

  **Deprecation notice**: [Datasets](doc:datasets) are deprecated. Starting June 2, 2026, you will no longer be able to create datasets or edit existing datasets. Migrate your datasets to data models and update any documents that use datasets as a data source to use a different source. See [Migrate a dataset to a data model](doc:migrate-a-dataset-to-a-data-model).

  ### Usage notes
  - Retrieve the **datasetId** by calling the [/v2/datasets](listdatasets) endpoint.
- **`sigma-computing-pp-cli datasets list`** - Get a list of available datasets. Available datasets include any datasets in your My Documents folder and any datasets you have access to.

 **Deprecation notice**: [Datasets](doc:datasets) are deprecated. Starting June 2, 2026, you will no longer be able to create datasets or edit existing datasets. Migrate your datasets to data models and update any documents that use datasets as a data source to use a different source. See [Migrate a dataset to a data model](doc:migrate-a-dataset-to-a-data-model).

 ### Usage scenarios
 - **Plan dataset migration**: Review the owner, location (path) of the dataset, migration status, and total number of documents that reference the dataset to plan dataset migration tasks.
 - **Clean up after dataset migration**: Retrieve the `dataModelId` of the data model created for the dataset. Make a GET request to the [/v2/dataModels/{dataModelId}](getdatamodel) endpoint to retrieve more details about the created data model.
 - **Identify unused datasets**: Review datasets that have 0 documents referencing it as a source and delete those datasets by making a DELETE request to the [/v2/files/{inodeId}](filesdelete) endpoint.

### deployment-policies

Manage deployment policies

- **`sigma-computing-pp-cli deployment-policies archive-deployment`** - Archive a deployment policy.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the `deploymentPolicyId` from the [/v2/deploymentPolicies](listdeployments) endpoint.
- **`sigma-computing-pp-cli deployment-policies create-deployment`** - Create a deployment policy to define what documents to deploy to a tenant organization and how to swap sources for those documents.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the **versionTagId** by calling the [/v2/tags](listversiontag) endpoint.
- Retrieve the identifier for `sourceSwapPolicies` by calling the [/v2/sourceSwapPolicies](listsourceswappolicies) endpoint and using the `policyId` in the response.
- **`sigma-computing-pp-cli deployment-policies get-deployment`** - Get details for a deployment policy.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the `deploymentPolicyId` from the [/v2/deploymentPolicies](listdeployments) endpoint.
- **`sigma-computing-pp-cli deployment-policies list-deployable-tenants`** - Returns a paginated list of tenant organizations that the calling organization can deploy to.

- **Parent organizations**: Returns all tenant organizations.
- **Tenant organizations**: Returns tenant organizations that the calling tenant has been granted deployment capabilities to.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned an account type with the **Manage deployment policies** permission enabled.
- **`sigma-computing-pp-cli deployment-policies list-deployments`** - List all deployment policies set up for an organization.
    
**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.
- **`sigma-computing-pp-cli deployment-policies update-deployment`** - Update a deployment policy. Only the fields included in the request are changed.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the `deploymentPolicyId` from the [/v2/deploymentPolicies](listdeployments) endpoint.
- Retrieve the identifier for `sourceSwapPolicies` by calling the [/v2/sourceSwapPolicies](listsourceswappolicies) endpoint and using the `policyId` in the response.
- The version tag cannot be changed after a deployment policy is created.

### favorites

Manage favorites

- **`sigma-computing-pp-cli favorites add`** - Favorite a folder or document for a specific user.

  ### Usage notes
  - Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.
  - Retrieve the **inodeId** by calling the [/v2/files](fileslist) endpoint and using the `id` included in the response.
- **`sigma-computing-pp-cli favorites list`** - Get the favorite documents for a specific user. This endpoint has the same functionality as the [List all favorite documents of a member](https://help.sigmacomputing.com/reference/listfavoriteinodes) endpoint.

  ### Usage notes
  - Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.
- **`sigma-computing-pp-cli favorites remove`** - Unfavorite a folder or document for a specific user.

  ### Usage notes
  - Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.
  - Retrieve the **inodeId** by calling the [/v2/files](fileslist) endpoint and using the **id** included in the response.

### files

Manage files

- **`sigma-computing-pp-cli files create`** - Create an empty workspace, folder, workbook, or report in Sigma.

  ### Usage notes
  - Specify a **parentId** for a folder, workbook, or report to place it within another folder. Retrieve the ID to use as a **parentId** by calling the [/v2/files](fileslist) endpoint and reviewing the `id` field in the response for files with a `type` of `folder`.
  - Specify an **ownerId** for a folder, workbook, or report to create it on behalf of another user. Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.

  ### Usage scenarios
  - **Project onboarding**: Start a new project by creating a workspace and folders to contain the workbooks for the project.
- **`sigma-computing-pp-cli files delete`** - Delete a folder or document, such as a workbook, data model, or report.

  ### Usage notes
  - Retrieve the **inodeId** by calling the [/v2/files](fileslist) endpoint and using the `id` included in the response.
- **`sigma-computing-pp-cli files get`** - Get information about a specific document or folder, such as a workbook, report, or data model.

  ### Usage notes
  - Retrieve the **inodeId** by calling the [/v2/files](fileslist) endpoint and using the `id` included in the response.
  - Retrieve the **inodeId** for a data model by calling the [/v2/members/{memberId}/files/recents](listrecentinodes) endpoint and using the `id` included in the response.
- **`sigma-computing-pp-cli files list`** - List all documents, such as workbooks and folders, accessible from the parent.

  ### Which files are returned by this endpoint

  The documents and folders accessible through this endpoint match those returned when you search in the Sigma UI. Some documents that you can view in the Sigma UI are not returned by this endpoint.

  - Returned documents and folders are limited to those that you have access to, such as through ownership, a document directly shared with you, a document link shared with you that you have opened, or access inherited through a folder or a workspace. This restriction also applies to users granted the Admin account type.
  - Newly created files are not returned immediately after creation. Updated files are reflected immediately.
  - If no `typeFilters` are set, only workbooks, folders, data models, datasets (deprecated), and reports are returned. To return other file types, such as a shortcut (symlink), specify a file type using the `typeFilters` option.

  ### Usage notes

  - Use the **parentId** to specify a folder and return details about the nested files and documents:
    - Retrieve the ID to use as a **parentId** by calling the [/v2/files](fileslist) endpoint and reviewing the `id` field in the response for files with a `type` of `folder`.
    - To use the "My Documents" folder as the parent folder, call the [/v2/members/{memberId}](getmember) endpoint and use the `homeFolderId` included in the response.
    - If parentId is not specified, it is assumed to be the root.
  - Use the available filters to return files that contain a specific keyword in the name, files with specific permissions granted to the user associated with the API credentials, or files of a specific type.
- **`sigma-computing-pp-cli files update`** - Update a folder or document, such as a workbook, data model, or report.

  ### Usage notes
  - Retrieve the **inodeId** by calling the [/v2/files](fileslist) endpoint and using the `id` included in the response.
  - Specify a **parentId** to place the document within a folder. Retrieve the ID to use as a **parentId** by calling the [/v2/files](fileslist) endpoint and reviewing the `id` field in the response for files with a `type` of `folder`.
  - To restore a deleted folder or document, set the **restore** parameter to `true`.

### grants

Manage grants

- **`sigma-computing-pp-cli grants create`** - Create a grant or update an existing grant on a document, folder, or workspace to a user or a team by ID.

  ### Usage notes
  - This endpoint can only be used to escalate the level of access that a user has to a document. To remove or reduce a user's access to a specific document, call [/v2/grants/{grantId}](deletegrant) to delete the existing grant and re-grant the desired level of access.
  - Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.
  - Retrieve the **teamId** by calling the [/v2/teams](listteams) endpoint.
  - Retrieve the **inodeId** by calling the [/v2/files](fileslist) endpoint and using the `id` included in the response.
  - Retrieve the **inodeId** for a data model by calling the [/v2/dataModels](listdatamodels) endpoint and using the `dataModelId` included in the response.
  - Optionally specify a **tagId** to associate the permission with a specific version tag of a workbook, dataset (deprecated), or database table. Retrieve the **tagId** by calling the [/v2/tags](listversiontag) endpoint and using the `versionTagId` included in the response.
- **`sigma-computing-pp-cli grants delete`** - Delete a grant by grant ID.

  ### Usage notes
  Depending on the grant object that you want to delete, you can retrieve the grantId in different ways:

  - For most documents, you can retrieve the **grantId** by calling the [/v2/grants](listgrants) endpoint with the **inodeId** for the document. For example, use the **workbookId** as the **inodeId**.
  - For a dataset (deprecated), you can also retrieve the **grantId** by calling the [/v2/datasets/{datasetId}/grants](listdatasetgrants) endpoint.
  - For a connection path, you can retrieve the **grantId** by calling the [/v2/connections/paths/{connectionPathId}/grants](listconnectionpathgrants) endpoint.

  ### Usage scenarios
  - **Principle of least privilege**: Revoke unnecessary privileges on a document by removing the grant.
- **`sigma-computing-pp-cli grants get`** - Return a grant object by grant ID.

  ### Usage notes

  Depending on the grant object that you want to return details about, you can retrieve the grantId in different ways:

  - For most documents, you can retrieve the **grantId** by calling the [/v2/grants](listgrants) endpoint with the **inodeId** for the document. For example, use the **workbookId** as the **inodeId**.
  - For a dataset (deprecated), you can also retrieve the **grantId** by calling the [/v2/datasets/{datasetId}/grants](listdatasetgrants) endpoint.
  - For a connection path, you can retrieve the **grantId** by calling the [/v2/connections/paths/{connectionPathId}/grants](listconnectionpathgrants) endpoint.
- **`sigma-computing-pp-cli grants list`** - List all grants for a given object.

  ### Usage notes

  You can specify one of the following:

  - A user by userId. Retrieve the **userId** by calling the [/v2/members](listmembers) endpoint and using the `memberId` included in the response.
  - A team by teamId. Retrieve the **teamId** by calling the [/v2/teams](listteams) endpoint.
  - A document (such as a workbook, report, data model, or dataset (deprecated)), folder, or workspace by inodeId.
    - Retrieve the **inodeId** by calling the [/v2/files](fileslist) endpoint and using the `id` included in the response.
    - Retrieve the **inodeId** for a data model by calling the [/v2/dataModels](listdatamodels) endpoint and using the `dataModelId` included in the response.

  ### Usage scenarios
  - **Manage inherited permissions**: Identify files and folders that have permissions directly granted to them.
  - **Review long-lived permissions**: Audit permissions granted more than a year ago and determine if they are still needed.

### members

Manage members

- **`sigma-computing-pp-cli members 1-list`** - **Attention:** This API endpoint uses pagination by default.

List all users in Sigma.

### Usage notes
- Filter your results using the `email` query parameter.
- If using `email` to filter by email address, you must URL encode the "@" character as `%40`.
- **[Deprecated]** Using the `search` parameter is deprecated. If using `search` to filter by email address, you must URL encode the "@" character as `%40`.
- Using `email` and `search` together is not supported.
- **`sigma-computing-pp-cli members create`** - Create a user.

  ### Usage notes
  - Creating a user with this endpoint sends an email invitation to the user. Embed users are not sent email invitations.
  - Review the account types returned by the [/v2/members](listmembers) endpoint to understand the format of the **memberType** string.
  - If **memberType** is omitted, the organization's Invitation default account type (configured in Admin > Account types) is used. If no Invitation default is configured, a built-in default account type is used.
  - Retrieve the **teamId**(s) by calling the [/v2/teams](listteams) endpoints.
- **`sigma-computing-pp-cli members delete`** - Deactivate a specific user by memberId. Users cannot be fully deleted, only deactivated. The deactivated user's documents are reassigned to the user associated with the API client credentials. For more information, see [Deactivate users](doc:deactivate-users).

### Usage notes
- Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.
- The user is directly marked **archived** by this API. See [Deactivate users](doc:deactivate-users) for more details on deactivation.

### Usage scenarios
- **User offboarding**: Manage your user base by efficiently offboarding users after they leave your organization.

### Best practices
- **Confirm memberId**: Before deactivating a user, make sure the memberId is correct.
- Do **not** use this for members provisioned through SCIM.
- **`sigma-computing-pp-cli members get`** - Returns a specific user by member ID.

### Usage notes
- Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.
- **`sigma-computing-pp-cli members list`** - **Attention**: This endpoint will return only paginated responses starting June 2, 2026. To start returning paginated responses before that date, include the query parameter `limit` in your request.

  List all users in Sigma.

  ### Usage notes
  - Filter your results using the `email` query parameter.
  - If using `email` to filter by email address, you must URL encode the "@" character as `%40`.
  - **[Deprecated]** Using the `search` parameter is deprecated. If using `search` to filter by email address, you must URL encode the "@" character as `%40`.
  - Using `email` and `search` together is not supported.
- **`sigma-computing-pp-cli members update`** - Update a specific user by memberId.

  ### Usage notes
  - Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.
  - Review the account types returned by the same endpoint to understand the format of the **memberType** string.
  - To deactivate a user and reassign their documents to a specific user, set  `newOwnerId`to the user ID of the desired document owner, and `isArchived` to `True`. For more details, see [Deactivate a user](doc:deactivate-users).

### organizations

Manage organizations

- **`sigma-computing-pp-cli organizations`** - Update settings for the organization associated with the API credentials.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- To update settings for a tenant organization, use [impersonation](doc:impersonate-users) to obtain a token for that tenant, then call this endpoint with that token.

### plugins

Manage plugins

- **`sigma-computing-pp-cli plugins create-custom`** - Register a new custom plugin in the organization. If `url` is omitted, the plugin is created with an empty production URL, and Sigma cannot load the plugin in workbooks until one is set.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type or an account type with the **Manage plugins** permission enabled.
- **`sigma-computing-pp-cli plugins delete-custom`** - Permanently delete a custom plugin. Workbook elements that reference the deleted plugin remain in their workbooks but can no longer render plugin content.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the **pluginId** by calling the [/v2/plugins](listcustomplugins) endpoint.
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type or an account type with the **Manage plugins** permission enabled.
- **`sigma-computing-pp-cli plugins get-custom`** - Get the metadata for a custom plugin by ID.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the **pluginId** by calling the [/v2/plugins](listcustomplugins) endpoint.
- **`sigma-computing-pp-cli plugins list-custom`** - List custom plugins owned by the current organization.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.
- **`sigma-computing-pp-cli plugins update-custom`** - Update metadata fields on an existing custom plugin. Only fields included in the request are changed. The `url` field cannot be updated by this endpoint.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the **pluginId** by calling the [/v2/plugins](listcustomplugins) endpoint.
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type or an account type with the **Manage plugins** permission enabled.

### query

Manage query


### reports

Manage reports

- **`sigma-computing-pp-cli reports create`** - This endpoint lets you create an empty report in Sigma, enabling you to build presentation-ready documents for sharing insights with stakeholders.

### Usage notes
- The **name** parameter is required to provide a name for the new report.
- Use the **folderId** to specify the folder in which to save the report. Retrieve the **folderId** by calling the [/v2/files](fileslist) endpoint and reviewing the `id` field in the response for files with a `type` of `folder`.

### Usage scenarios
- **Reliable export formatting**: Users can quickly generate a new blank report to create predictable, paginated exports.

### Best practices
- **Naming conventions**: Establish and follow consistent naming conventions for reports to make it easier to manage and identify them within larger projects.
- **Folder organization**: Use the **folderId** to organize reports into relevant folders, which helps in maintaining a tidy workspace, especially in environments with multiple users or teams.
- **Access control**: Regularly review and manage access permissions for new reports, ensuring that only the appropriate personnel can view or edit sensitive data.
- **`sigma-computing-pp-cli reports get`** - This endpoint retrieves a report by its unique identifier (`reportId`). It provides detailed information about the report, including its name, URL, path, and other metadata.

### Usage notes
- The **reportId** parameter must be a valid UUID that uniquely identifies the report. Invalid or nonexistent IDs return an error. Retrieve the **reportId** by calling the [/v2/reports](listreports) endpoint.

### Usage scenarios
- **Report search**: Allows users to view all of their reports and locate a specific one, as well as filter by criteria such as if it is archived.
- **Data retrieval**: Developers can use this endpoint to programmatically retrieve details about a specific report to display its content or metadata in a custom user interface.
- **Integration**: Use this endpoint for integrations where other systems need to fetch report details based on an ID provided through another interface or workflow.

### Best practices
- Validate the **reportId** on the client side before making a request to avoid unnecessary server load caused by invalid requests.
- **`sigma-computing-pp-cli reports list`** - This endpoint retrieves a list of all available reports.

Available reports include any reports in your My Documents folder and any reports you have access to.

Users with the Admin account type can optionally retrieve all reports in the organization.

### Usage notes
This endpoint requires no parameters for basic requests, but supports query parameters for pagination and response limit.

### Pagination

This endpoint supports pagination, which lets you retrieve large sets of data in manageable segments. The response includes pagination details as follows:

- hasMore: A boolean value indicating whether there are more pages of data available beyond the current page.
- total: The total number of entries available across all pages.
- nextPage: An identifier or token that you can use in a subsequent request to retrieve the next page of data.

#### Example response for pagination
```json
{
  "hasMore": true,
  "total": 104,
  "nextPage": "50"
}
```

To request additional pages, include the `nextPage` option in your next request as the value of the `page` option. Repeat this process until `nextPage` returns `null`, indicating that there are no more pages to return.

### Usage scenarios
- **Report navigation**: Allows users to view their collection of reports and locate ones they need.
- **Integration points**: Useful for building integrations that need to present users with a list of their available reports, such as in custom applications using embeds.

### saml

Manage saml

- **`sigma-computing-pp-cli saml activate-service-provider-certificate`** - Activate a certificate.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Activating a certificate deactivates all certificates used for the same purpose by the same SAML service provider. For example, activating a signing certificate deactivates all other signing certificates in use by the same SAML SP.
- Retrieve the `samlServiceProviderId` by calling the [/saml/service-providers](listsamlserviceproviders) endpoint.
- Retrieve the `samlServiceProviderCertificateId` by calling the [/saml/service-providers/certificates](listsamlspcertificates) endpoint.
- **`sigma-computing-pp-cli saml create-service-provider-certificate`** - Create a certificate for a given SAML service provider.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Retrieve the `samlServiceProviderId` by calling the [/saml/service-providers](listsamlserviceproviders) endpoint.
- **`sigma-computing-pp-cli saml deactivate-service-provider-certificate`** - Deactivate a certificate.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Retrieve the `samlServiceProviderId` by calling the [/saml/service-providers](listsamlserviceproviders) endpoint.
- Retrieve the `samlServiceProviderCertificateId` by calling the [/saml/service-providers/certificates](listsamlspcertificates) endpoint.
- **`sigma-computing-pp-cli saml get-service-provider-certificate`** - Get a certificate to use with your IdP when configuring Sigma as a service provider.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Retrieve the `samlServiceProviderId` by calling the [/saml/service-providers](listsamlserviceproviders) endpoint.
- Retrieve the `samlServiceProviderCertificateId` by calling the [/saml/service-providers/certificates](listsamlspcertificates) endpoint.
- Signing certificates are used by your IdP to verify signatures that Sigma attaches to SAML requests.
- Encryption certificates are used by your IdP to encrypt SAML responses sent to Sigma.
- **`sigma-computing-pp-cli saml list-service-provider-certificates`** - List all certificates for a given SAML service provider.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Retrieve the `samlServiceProviderId` by calling the [/saml/service-providers](listsamlserviceproviders) endpoint.
- **`sigma-computing-pp-cli saml list-service-providers`** - List all SAML service providers in the organization.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- **`sigma-computing-pp-cli saml remove-service-provider-certificate`** - Remove a certificate.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Retrieve the `samlServiceProviderId` by calling the [/saml/service-providers](listsamlserviceproviders) endpoint.
- Retrieve the `samlServiceProviderCertificateId` by calling the [/saml/service-providers/certificates](listsamlspcertificates) endpoint.

### shared-templates

Manage shared templates

- **`sigma-computing-pp-cli shared-templates accept-template-share`** - If a workbook template has been shared with your organization but not yet accepted, accept the template share.
  
  ### Usage notes
  - Retrieve pending template shares by calling the [/v2/shared_templates/shared_with_you](listtemplatessharedwithyou) endpoint and specifying `pending` as `true`.
  - Retrieve the **shareId** by calling the [/v2/shared_templates/shared_with_you](listtemplatessharedwithyou) endpoint and using the `shareId` included in the response.
  - Retrieve the **connectionId** by calling the [/v2/connections](listconnections) endpoint.
  - Retrieve the **inodeId** for a table by calling the [/v2/connections/{connectionId}/lookup](lookupconnection) endpoint.

  - The key of the **sourceSwaps** object is the original id of the table, dataset, or connection in the organization that shared the template
- **`sigma-computing-pp-cli shared-templates delete-external-share`** - Remove a template shared with your organization.

  **Usage notes**
  - Retrieve the **shareId** by calling the [/v2/shared_templates/shared_with_you](listtemplatessharedwithyou) endpoint and using the `shareId` included in the response.
- **`sigma-computing-pp-cli shared-templates list-templates-shared-with-you`** - Lists all workbook templates shared with your organization.
  
  ### Usage notes
  - Use the `pending` parameter to identify templates that have been shared but not yet accepted.

### sigma-computing-public-auth

Manage sigma computing public auth

- **`sigma-computing-pp-cli sigma-computing-public-auth`** - Use your Sigma client ID and secret with this endpoint to generate an access token valid for one hour, or to refresh your token. You can then use the access token to authenticate requests made to the Sigma API.

To make any API call with the Sigma API, including calls from the API documentation, you must have a valid bearer token. To generate a token, you must have a valid **Client ID** and **Secret**. See [Generate Sigma API client credentials](generate-client-credentials).

You make all API calls to a specific URL that corresponds to the cloud where your Sigma environment is hosted. Set the **Base URL** to the relevant URL for your environment. For details, see [Identify your API request URL](get-started-sigma-api#identify-your-api-request-url).

Generate a token by sending a POST request to this `/v2/auth/token` endpoint, or use the **Try It!** option on this page.

### Usage notes

- The API token is valid for 1 hour. When the token expires, an endpoint response returns an unauthorized error.
- Refresh your access token before it expires using the `refresh_token` option.
- If your client credentials are owned by a user assigned the Admin account type, you can generate an access token as a specific user using impersonation.

### source-swap-policies

Manage source swap policies

- **`sigma-computing-pp-cli source-swap-policies create-source-swap-policy`** - Create a source swap policy.

  **Beta**: Creating a **Connection** source swap policy is in private beta and must be enabled for your organization. Creating a **Deployment** source swap policy is in public beta. This documentation is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

  ### Usage notes
  - Retrieve the **connectionId** to use as the **fromConnectionId** by calling the [/v2/connections](listconnections) endpoint.
  - Retrieve the **userAttributeId** to use in the **swaps** object by calling the [/v2/user-attributes](listuserattributes) endpoint.
- **`sigma-computing-pp-cli source-swap-policies delete-source-swap-policy`** - Delete a source swap policy by policy ID.

  **Beta**: Deleting a **Connection** source swap policy is in private beta and must be enabled for your organization. Deleting a **Deployment** source swap policy is in public beta. This documentation is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

  ### Usage notes
  - Retrieve the **policyId** by calling the [/v2/sourceSwapPolicies](listsourceswappolicies) endpoint.
- **`sigma-computing-pp-cli source-swap-policies get-source-swap-policy`** - Get a source swap policy by policy ID.

  **Beta**: Getting details for a **Connection** source swap policy is in private beta and must be enabled for your organization. Getting details for a **Deployment** source swap policy is in public beta. This documentation is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

  ### Usage notes
  - Retrieve the **policyId** by calling the [/v2/sourceSwapPolicies](listsourceswappolicies) endpoint.
- **`sigma-computing-pp-cli source-swap-policies list`** - Get a list of source swap policies.

  **Beta**: Getting a list of **Connection** source swap policies is in private beta and must be enabled for your organization. Getting a list of **Deployment** source swap policies is in public beta. This documentation is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.
- **`sigma-computing-pp-cli source-swap-policies update-source-swap-policy`** - Update a source swap policy.

  **Beta**: Updating a **Connection** source swap policy is in private beta and must be enabled for your organization. Updating a **Deployment** source swap policy is in public beta. This documentation is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

  ### Usage notes
  - Retrieve the **policyId** by calling the [/v2/sourceSwapPolicies](listsourceswappolicies) endpoint.
  - Retrieve the **connectionId** to use as the **fromConnectionId** by calling the [/v2/connections](listconnections) endpoint.
  - Retrieve the **userAttributeId** to use in the **swaps** object by calling the [/v2/user-attributes](listuserattributes) endpoint.

### tags

Manage tags

- **`sigma-computing-pp-cli tags create-version`** - Create a version tag to use, for example, for workbooks.
- **`sigma-computing-pp-cli tags delete-version`** - Delete a specific version tag. Retrieve the **tagId** by calling the [/v2/tags](listversiontag) endpoint and using the `versionTagId` in the response.
- **`sigma-computing-pp-cli tags list-version`** - Get a list of version tags.
- **`sigma-computing-pp-cli tags update-version`** - Update the description of a specific version tag. Retrieve the **tagId** by calling the [/v2/tags](listversiontag) endpoint and using the `versionTagId` in the response.

### teams

Manage teams

- **`sigma-computing-pp-cli teams 1-list`** - **Attention:** This API endpoint uses pagination by default.

  List all teams in Sigma.
- **`sigma-computing-pp-cli teams create`** - Create a Sigma team.

  ### Usage notes
  - Specify members to add to the team by userId or memberId.
  - Retrieve the **memberId** by calling the [/v2/members](listmembers) endpoint.
- **`sigma-computing-pp-cli teams delete`** - Delete a specific team.

  ### Usage notes
  - Retrieve the **teamId** by calling the [/v2/teams](listteams) endpoint.
- **`sigma-computing-pp-cli teams get`** - Returns details about a team based on team ID. The response includes an array of member IDs that identify the users in the team.

  ### Usage notes
  - Retrieve the **teamId** by calling the [/v2/teams](listteams) endpoint.
- **`sigma-computing-pp-cli teams list`** - **Attention**: This endpoint will return only paginated responses starting June 2, 2026. To start returning paginated responses before that date, include the query parameter `limit` in your request.

  List all teams in Sigma.
- **`sigma-computing-pp-cli teams update`** - Update the name, description, and visibility of a team.

  ### Usage notes
  - Retrieve the **teamId** by calling the [/v2/teams](listteams) endpoint.

### templates

Manage templates

- **`sigma-computing-pp-cli templates get`** - Get a template by template ID.

  ### Usage notes
  - Retrieve the **templateId** by calling the [/v2/templates](listtemplates) endpoint.
- **`sigma-computing-pp-cli templates list`** - Returns a list of available templates.

  ### Usage notes
  - Official Sigma templates are created by SigmaSchedulerRobot.
- **`sigma-computing-pp-cli templates save-workbook-from`** - Create a workbook by saving it from a template.

  ### Usage notes
  - Retrieve the **templateId** by calling the [/v2/templates](listtemplates) endpoint.
  - Retrieve the **folderId** by calling the [/v2/files](fileslist) endpoint and reviewing the `id` field in the response for files with a `type` of `folder`. To use the "My Documents" folder as the destination folder, call the [/v2/members/{memberId}](getmember) endpoint and use the `homeFolderId` included in the response.
  - If you leave the name or description options blank, the workbook created inherits the values of the template.

### tenants

Manage tenants

- **`sigma-computing-pp-cli tenants create`** - Create a new tenant organization with the specified name and organization slug to be used in the URL.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- The **tenantOrganizationName** is displayed as the organization name in the Sigma interface.
- The **tenantOrganizationSlug** must be unique and is used in URLs to identify the tenant organization.
- **`sigma-computing-pp-cli tenants delete`** - Delete a tenant organization by tenantOrganizationId.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Retrieve the **tenantOrganizationId** by calling the [/v2/tenants](listtenants) endpoint.
- This action permanently removes the tenant organization and cannot be undone.
- **`sigma-computing-pp-cli tenants get`** - Retrieve details of a specific tenant organization by tenantOrganizationId.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Retrieve the **tenantOrganizationId** by calling the [/v2/tenants](listtenants) endpoint.
- **`sigma-computing-pp-cli tenants list`** - Retrieve a paginated list of tenant organizations with optional filtering and sorting.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Use **pageToken** and **pageSize** parameters to manage paginated responses.
- Use **search** to filter results by organization slug or ID of the user who created the tenant.
- Use **key** and **order** to sort results by creation date, ID of the user who created the tenant, name, or URL.
- **`sigma-computing-pp-cli tenants patch`** - Update the details of an existing tenant organization. The organization name and slug can be updated through this endpoint.

**Beta**: This documentation describes a public beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.
    
### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Retrieve the **tenantOrganizationId** by calling the [/v2/tenants](listtenants) endpoint.

### translations

Manage translations

- **`sigma-computing-pp-cli translations create-org`** - This endpoint creates a new organization translation file for the specified locale, containing the translation key-value pairs, if provided. You can also use this endpoint to create custom translations for a locale.
Entering common phrases and their translations used in workbooks across your organization ensures consistent translations for all users.

### Usage Notes
- Retrieve the supported locale identifiers to use for **lng** from [Supported languages and locales](doc:manage-workbook-localization#supported-languages-and-locales).
- **`sigma-computing-pp-cli translations delete-org`** - This endpoint deletes the translation file for the specified locale (**lng**) without any custom translations.

### Usage notes
- Retrieve the **lng** by calling the [/v2/translations/organization](listorglocales) endpoint.
- **`sigma-computing-pp-cli translations delete-org-with-variant`** - This endpoint deletes the translation file for the specified custom translation (**lng_variant**) for a locale (**lng**).

### Usage notes
- Retrieve the **lng** and **lng_variant** by calling the [/v2/translations/organization](listorglocales) endpoint.
- **`sigma-computing-pp-cli translations get-org`** - This endpoint returns a JSON file containing the translations for a specified locale (**lng**) without custom translations.

### Usage notes
- Retrieve the **lng** by calling the [/v2/translations/organization](listorglocales) endpoint.
- **`sigma-computing-pp-cli translations get-org-with-variant`** - This endpoint returns a JSON file containing the translations and custom translations (**lng_variant**) for a locale (**lng**).

### Usage notes
- Retrieve the **lng** and **lng_variant** by calling the [/v2/translations/organization](listorglocales) endpoint.
- **`sigma-computing-pp-cli translations list-org-locales`** - This paginated endpoint lists all the translation files that have been defined at the organization level.

### Usage notes
This endpoint requires no parameters for basic requests but supports query parameters for pagination and response limit.

#### Pagination

This endpoint supports pagination, which lets you retrieve large sets of data in manageable segments. The response includes pagination details as follows:

- hasMore: A boolean value indicating whether there are more pages of data available beyond the current page.
- total: The total number of entries available across all pages.
- nextPage: An identifier or token that you can use in a subsequent request to retrieve the next page of data.

#### Example response for pagination
{
  "hasMore": true,
  "total": 104,
  "nextPage": "50"
}

To request additional pages, include the `nextPage` option in your followup request according to the endpoint's parameter requirements. This process can be repeated until `nextPage` is `null`.
- **`sigma-computing-pp-cli translations update-org`** - This endpoint updates the translation file for a specified locale (**lng**) without custom translations.

### Usage notes
- Retrieve the **lng** by calling the [/v2/translations/organization](listorglocales) endpoint.
- **`sigma-computing-pp-cli translations update-org-with-variant`** - This endpoint updates the translation file for the specified custom translation (**lng_variant**) for a locale (**lng**).

### Usage notes
- Retrieve the **lng** and **lng_variant** by calling the [/v2/translations/organization](listorglocales) endpoint.

### user-attributes

Manage user attributes

- **`sigma-computing-pp-cli user-attributes create`** - Create a new user attribute. An optional description and default value can be provided.
- **`sigma-computing-pp-cli user-attributes delete`** - Delete a user attribute.

### Usage notes
- To perform this operation, you must use API credentials owned by a user assigned the Admin account type.
- Retrieve the **userAttributeId** by calling the [/v2/user-attributes](listuserattributes) endpoint.
- This action permanently removes the user attribute from the organization and cannot be undone.
- **`sigma-computing-pp-cli user-attributes get`** - Get details for a specific user attribute.

  ### Usage notes
  - Retrieve the **userAttributeId** by calling the [/v2/user-attributes](listuserattributes) endpoint.
- **`sigma-computing-pp-cli user-attributes list`** - Get a list of available user attributes, values, and owners.

### webhooks

Manage webhooks

- **`sigma-computing-pp-cli webhooks <workbookId> <sequenceId>`** - Post data to a webhook endpoint. The request body must match the configured webhook specification configured in Sigma.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the webhook URL by creating a webhook through the Sigma UI. The URL will be formatted as `/v2/webhooks/{workbookId}/{sequenceId}`.

### whoami

Manage whoami

- **`sigma-computing-pp-cli whoami`** - Get the identity and authentication status of the current user.

  ### Usage notes
  Call this endpoint after authenticating to retrieve user details. It does not require any parameters other than the user's valid session or authentication token included in the request headers.

  ### Usage scenarios
  - **Session validation:** Quickly confirm user authentication and retrieve session-specific details at the start of a new session.
  - **User configuration:** Retrieve settings or configurations specific to the user, allowing for a customized application experience based on user roles and permissions.

  ### Best practices
  - Call this endpoint at the beginning of each session to ensure that the user's credentials are still valid and have not been revoked.
  - Use the information provided by this endpoint to tailor the user interface and functionality accessible to the user, enhancing security and user experience.

### workbooks

Manage workbooks

- **`sigma-computing-pp-cli workbooks create`** - This endpoint lets you create an empty workbook in Sigma, letting you start a data analysis project or report without any pre-existing templates or data sources set up.

### Usage notes
- The `name` parameter is required to provide a name for the new workbook.
- Use the **folderId** to specify the folder in which to save the workbook. Retrieve the **folderId** by calling the [/v2/files](fileslist) endpoint and reviewing the `id` field in the response for files with a `type` of `folder`.

### Usage scenarios
- **Project initialization**: When starting a new project that requires data analysis or reporting, users can quickly generate a new blank workbook to begin structuring their data and analyses.
- **Template preparation**: Administrators or team leads might create blank workbooks to set up standardized templates that their teams can use to maintain consistency in data handling and reporting.

### Best practices
- **Naming conventions**: Establish and follow consistent naming conventions for workbooks to make it easier to manage and identify them within larger projects.
- **Folder organization**: Use the **folderId** to organize workbooks into relevant folders, which helps in maintaining a tidy workspace, especially in environments with multiple users or teams.
- **Access control**: Regularly review and manage access permissions for new workbooks, ensuring that only the appropriate personnel can view or edit sensitive data.
- **`sigma-computing-pp-cli workbooks create-spec`** - This endpoint creates a new workbook from a code representation. Use it to programmatically define and create workbooks that are then accessible from the Sigma UI.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

For more information on managing workbooks programmatically, see [Manage workbooks as code](https://help.sigmacomputing.com/docs/manage-workbooks-as-code). For more information on using this endpoint, including end-to-end instructions, see [Create a workbook from a code representation](https://help.sigmacomputing.com/docs/create-a-workbook-from-a-code-representation).

### System and user requirements
- To perform this operation, you must use API credentials owned by a user assigned an account type with **Create, edit, and publish workbooks** permission.
- To perform this operation, you must use API credentials owned by a user with **Can edit** access for the workbook.

### Usage notes
- You can define the layout of contents on the workbook page using XML in the `layout` field. For more information, see [Customize the layout of a workbook in code representation](https://help.sigmacomputing.com/docs/customize-the-layout-of-a-workbook-in-code-representation).
- You can combine multiple YAML documents to create one workbook representation. For more information, see [Prepare a representation from multiple YAML documents](https://help.sigmacomputing.com/docs/create-a-workbook-from-a-code-representation#prepare-a-representation-from-multiple-yaml-documents).
- To retrieve the representation of an existing workbook, use the [/v2/workbooks/{workbookId}/spec](getworkbookspec) endpoint.
- To update an existing workbook from code, use the [/v2/workbooks/{workbookId}/spec](updateworkbookspec) endpoint.
- To get a list of folders, call the [/v2/files](fileslist) endpoint and review the `id` field in the response for files with a `type` of `folder`.

### Usage scenarios
- **Agentic workflows**: Agents can use these endpoints to directly manage workbook contents in code.
- **Automation**: If you want to create several workbooks across one or more organizations, you can use this endpoint to programmatically create the workbooks.
- **Integration**: Using this endpoint, you can create workbooks based on external data sources or integrate Sigma with other tools and platforms.
- **Migration**: You can use this endpoint to migrate workbooks from one organization to another.
- **`sigma-computing-pp-cli workbooks get`** - This endpoint retrieves a workbook by its unique identifier (`workbookId`). It provides detailed information about the workbook, including its name, URL, path, and other metadata. You can use this endpoint to fetch specific workbook details for display or further processing within client applications.

### Usage notes
- The **workbookId** parameter must be a valid UUID that uniquely identifies the workbook. Invalid or nonexistent IDs return an error. Retrieve the **workbookId** by calling the [/v2/workbooks](listworkbooks) endpoint.

### Usage scenarios
- **Data retrieval**: Developers can use this endpoint to programmatically retrieve details about a specific workbook to display its content or metadata in a custom user interface.
- **Integration**: This endpoint is crucial for integrations where other systems need to fetch workbook details based on an ID provided through another interface or workflow.

### Best practices
- Validate the **workbookId** on the client side before making a request to avoid unnecessary server load caused by invalid requests.
- **`sigma-computing-pp-cli workbooks list`** - This endpoint retrieves a list of all available workbooks.

  Available workbooks include any workbooks in your My Documents folder and any workbooks you have access to.

  Users with the Admin account type can optionally retrieve all workbooks in the organization.

  ### Usage notes
  This endpoint requires no parameters for basic requests, but supports query parameters for pagination and response limit.

  #### Pagination

  This endpoint supports pagination, which lets you retrieve large sets of data in manageable segments. The response includes pagination details as follows:

  - hasMore: A boolean value indicating whether there are more pages of data available beyond the current page.
  - total: The total number of entries available across all pages.
  - nextPage: An identifier or token that you can use in a subsequent request to retrieve the next page of data.

  #### Example response for pagination
 ```json
  {
    "hasMore": true,
    "total": 104,
    "nextPage": "50"
  }
  ```

  To request additional pages, include the `nextPage` option in your next request as the value of the `page` option. Repeat this process until `nextPage` returns `null`, indicating that there are no more pages to return.

  ### Usage scenarios
  - **Workbook navigation:** Allows users to easily navigate through their collection of workbooks and access the one they need.
  - **Integration points:** Useful for building integrations that need to present users with a list of their available workbooks, such as in custom applications using Sigma Embeds.
- **`sigma-computing-pp-cli workbooks tag`** - Add a version tag to a workbook and optionally set up a connection to swap to for a specific version of the workbook.

  ### Usage notes
  - Retrieve the **workbookId** by calling the [/v2/workbooks](listworkbooks) endpoint.
  - Retrieve the **tag** by calling the [/v2/tags](listversiontag) endpoint and using the `name` in the response.
  - Retrieve the **connectionId** to use as the **fromId** or **toId** by calling the [/v2/connections](listconnections) endpoint.
  - If your workbook includes a source that is not mapped to a new source, that source is not swapped.
  - When swapping data models used as the source for the workbook:
    - You can only swap a data model source to another version of the same data model source. You cannot swap a data model source to a table in your data warehouse or a dataset.
    - When you swap sources from one data model version to a new one, specify the version tag of the data model that you swap to with `toVersionTagId`:
      - To swap to the latest published version of the data model, specify `toVersionTagId` as `null`.
      - To swap to a specific tagged version of the data model, specify the `toVersionTagId` of the data model.
    - If the workbook already uses a specific tagged version of a data model as a source, use `fromVersionTagId` to indicate which tagged version to swap from.
      - To swap from the latest published version of the data model, specify `fromVersionTagId` as `null`.
      - To swap from a specific tagged version of the data model, specify the `fromVersionTagId` of the data model.
  - To retrieve the `fromVersionTagId` for a data model used as the workbook source, call the [/v2/workbooks/{workbookId}/sources](getworkbooksources) endpoint and use the `versionTagId` in the response.
  - To retrieve the `versionTagId` for a data model, call the [/v2/dataModels](listdatamodels) endpoint and use the `versionTagId` in the response.

  ### Usage scenarios
  - **Lifecycle management**: Identify production and development resources.
- **`sigma-computing-pp-cli workbooks v3alpha-source-swap`** - Swap each data source used by a workbook. You can swap from one table, dataset, data model element, or custom SQL element to another for each data source.

**Beta**: This documentation describes a private beta feature and is subject to the [Beta features](doc:sigma-product-releases#beta-features) disclaimer.

### Usage notes
- Retrieve the **workbookId** by calling the [/v2/workbooks](listworkbooks) endpoint.
- Retrieve workbook sources by calling the [/v2/workbooks/{workbookId}/sources](getworkbooksources) endpoint.
- Retrieve the **dataModelId** by calling the [/v2/dataModels](listdatamodels) endpoint.
- Retrieve the **datasetId** by calling the [/v2/datasets](listdatasets) endpoint.
- Retrieve the **tableId** by calling the [/v2/connection](lookupconnection) endpoint.
- Retrieve the **connectionId** for a new custom SQL element by calling the [/v2/connections](listconnections) endpoint.

- Use the `sourceMapping` options in the request body to swap specific sources. For each source in the source mapping, provide the current source in use in the **from** object, and provide the source that you want to use instead in the **to** object.
- If your source is a data model with one or more tagged versions, you can also provide a `versionTagId` to swap to or from a tagged version of the data model. The data model version must already be tagged.
- When swapping **from** a custom SQL element, identify it by its element ID using the `customSqlId` field. When swapping **to** a custom SQL element, supply the `connectionId` of the connection to run against and the SQL query in the `definition` field.
- If your source is a data model or a dataset, you can use the `metricMapping` option to map metrics that rely on column names from the old source to the new one so that they continue to work.
- If the source that you swap from uses column names, links, or relationships different from the source that you are swapping to, use the `columnMapping` option to map each column in the original source to a corresponding column in the new source.
- When `columnMapping` is omitted, columns are automatically matched by display name, ignoring differences in case and punctuation. For example, a custom SQL output column named `DOCK_COUNT` automatically maps to a data model column with display name `Dock Count`. Provide `columnMapping` explicitly to override the automatic match for a column.

### Usage scenarios
- **Workbook development**: Use a test data source during development, then swap to a production source when ready to share the workbook.
- **Dataset migration**: Update references to deprecated dataset sources with data model sources by mapping dataset IDs to their new data model equivalents.

### workspaces

Manage workspaces

- **`sigma-computing-pp-cli workspaces 1-list`** - **Attention:** This API endpoint uses pagination by default.

  This endpoint returns all workspaces. See [Manage Workspaces](doc:manage-workspaces) for more details about workspaces in Sigma.

  ### Usage notes
  - **Pagination**: Use the `page` and `limit` parameters to control the size and segment of the workspace list returned.
  - **Filtering by name**: Optionally, use the `name` parameter to filter workspaces by a case-insensitive substring match.
  - **Filtering by exact name**: Optionally, use the `exactName` parameter to filter workspaces by an exact name match (also case-insensitive). When provided, `exactName` takes precedence over `name` and uses an indexed equality lookup, which is significantly faster than substring search.

  ### Usage scenarios
  - **User interface display**: Populate a user interface with a list of all available workspaces, using pagination to efficiently load data and filtering to quickly find specific workspaces.
  - **Administrative overview**: Provide system administrators with an overview of all workspaces for management and monitoring purposes, with the ability to browse through pages and search by name.

  ### Best practices
  - Use caching to reduce load times and server demand when frequently accessing workspace lists.
  - Regularly update and synchronize workspace lists to ensure that displayed information is current and accurate.
- **`sigma-computing-pp-cli workspaces create`** - This endpoint allows clients to create a workspace with specific characteristics.

### Usage notes
- Set **NoDuplicates** to **true** to prevent the creation of a workspace with a name that already exists.

### Usage scenarios
- **Initial setup**: Useful for users setting up a new workspace after signing up.
- **Project separation**: Helps in creating separate workspaces for different projects or teams.

### Best practices
- **Check for existing names**: Before setting `noDuplicates` to true, make sure to search for existing workspace names to avoid conflicts.
- **Consistent naming conventions**: Adopt a consistent naming convention for workspaces to ensure clarity and avoid confusion.
- **`sigma-computing-pp-cli workspaces delete`** - You can use this endpoint to delete an existing workspace by its workspaceId. **Caution:** Deleted workspaces cannot be recovered.

### Usage notes
- Retrieve the **workspaceId** by calling the [/v2/workspaces](listworkspaces) endpoint.

### Usage scenarios
- **Cleanup operations**: Useful for removing workspaces that are no longer needed or relevant.
- **Resource management**: Helps in managing the overall resource allocation by removing unused workspaces.

### Best practices
- **Confirm before deletion**: Always ensure that deletion operations are preceded by explicit user confirmations to prevent accidental loss of data.
- **Audit and logging**: Maintain audit logs for deletion actions to track who deleted the workspace and when.
- **`sigma-computing-pp-cli workspaces get`** - This endpoint retrieves the details of a specific workspace by its workspaceId.

### Usage notes
- Retrieve the **workspaceId** by calling the [/v2/workspaces](listworkspaces) endpoint.

### Usage scenarios
- **Workspace management**: Useful for administrators or users who need to view the details of a specific workspace.
- **Integration checks**: Can be used by external systems to verify the existence and status of a workspace as part of integration workflows.

### Best practices
- **Validate workspaceId**: Ensure the workspaceId provided is valid and corresponds to an existing workspace. Handle any errors gracefully.
- **Access controls**: Implement proper authorization checks to ensure that only entitled users can access workspace details.
- **`sigma-computing-pp-cli workspaces list`** - **Attention**: This endpoint will return only paginated responses starting June 2, 2026. To start returning paginated responses before that date, include the query parameter `limit` in your request.

  This endpoint returns a list of all workspaces. You can use pagination and optionally filter by name to manage large sets of data. See [Manage Workspaces](doc:manage-workspaces) for more details about workspaces in Sigma.
  ### Usage notes
  - **Filtering by name**: Optionally, use the `name` parameter to filter workspaces by a case-insensitive substring match.
  - **Filtering by exact name**: Optionally, use the `exactName` parameter to filter workspaces by an exact name match (also case-insensitive). When provided, `exactName` takes precedence over `name` and uses an indexed equality lookup, which is significantly faster than substring search.

  ### Usage scenarios
  - **Pagination**: Use the `page` and `limit` parameters to control the size and segment of the workspace list returned.
  - **User interface display**: Populate a user interface with a list of all available workspaces, using pagination to efficiently load data and filtering to quickly find specific workspaces.
  - **Administrative overview**: Provide system administrators with an overview of all workspaces for management and monitoring purposes, with the ability to browse through pages and search by name.

  ### Best practices
  - Use caching to reduce load times and server demand when frequently accessing workspace lists.
  - Regularly update and synchronize workspace lists to ensure that displayed information is current and accurate.
- **`sigma-computing-pp-cli workspaces update`** - This endpoint updates the name of an existing workspace identified by its workspaceId.

### Usage notes
- Set `NoDuplicates` to `true` to prevent the creation of a duplicate workspace.
- Retrieve the **workspaceId** by calling the [/v2/workspaces](listworkspaces) endpoint.

### Usage scenarios
- **Configuration change**: Allows users to update workspace settings or names as projects evolve or requirements change.
- **Access control adjustments**: Update workspace details in response to organizational changes or policy updates.

### Best practices
- **Partial updates**: Use PATCH to support partial updates, only sending the fields that need to be updated.
- **Validation**: Validate input data to ensure it adheres to expected formats and constraints. Handle errors gracefully and inform the user of any constraints.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sigma-computing-pp-cli account-types list

# JSON for scripting and agents
sigma-computing-pp-cli account-types list --json

# Filter to specific fields
sigma-computing-pp-cli account-types list --json --select id,name,status

# Dry run — show the request without sending
sigma-computing-pp-cli account-types list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sigma-computing-pp-cli account-types list --agent
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
sigma-computing-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `sigma-computing-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/sigma-computing-public-pp-cli/config.toml`; `--home`, `SIGMA_COMPUTING_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SIGMA_COMPUTING_CLIENT_ID` | auth_flow_input | Yes |  |
| `SIGMA_COMPUTING_CLIENT_SECRET` | auth_flow_input | Yes | Set during initial auth setup. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `sigma-computing-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sigma-computing-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SIGMA_COMPUTING_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 / token exchange fails** — Verify SIGMA_CLIENT_ID and SIGMA_CLIENT_SECRET, and that SIGMA_BASE_URL matches your org's API base URL under Administration > Developer Access (it is per-cloud and cannot be guessed).
- **Materialization run fails immediately** — A materialization schedule must have run successfully at least once before a manual run works; create and let the schedule run first.
- **Copied workbook is owned by the wrong person** — Use 'workbook copy --to <member>' which auto-reassigns ownership, instead of the raw copy endpoint.
- **List command seems to return only some rows** — List endpoints are cursor-paginated; the CLI follows nextPage automatically on 'sync' and list commands — use 'sync' for the complete set.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sigma-agent-skills**](https://github.com/sigmacomputing/sigma-agent-skills) — Shell (8 stars)
- [**sigma-sample-api**](https://github.com/sigmacomputing/sigma-sample-api) — Python (7 stars)
- [**embed-sdk**](https://github.com/sigmacomputing/embed-sdk) — TypeScript (5 stars)
- [**ja2z/mcp-server**](https://github.com/ja2z/mcp-server) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

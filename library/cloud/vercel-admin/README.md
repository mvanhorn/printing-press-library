# Vercel Admin CLI

Vercel combines the best developer experience with an obsessive focus on end-user performance. Our platform enables frontend teams to do their best work.

Learn more at [Vercel Admin](https://vercel.com/support).

## Install

The recommended path installs both the `vercel-admin-pp-cli` binary and the `pp-vercel-admin` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install vercel-admin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install vercel-admin --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install vercel-admin --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install vercel-admin --agent claude-code
npx -y @mvanhorn/printing-press-library install vercel-admin --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/vercel-admin/cmd/vercel-admin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vercel-admin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install vercel-admin --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-vercel-admin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-vercel-admin --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install vercel-admin --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vercel-admin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `VERCEL_ADMIN_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/vercel-admin/cmd/vercel-admin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "vercel-admin": {
      "command": "vercel-admin-pp-mcp",
      "env": {
        "VERCEL_ADMIN_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
vercel-admin-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export VERCEL_ADMIN_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
vercel-admin-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
vercel-admin-pp-cli access-groups list
```

## Usage

Run `vercel-admin-pp-cli --help` for the full command reference and flag list.

## Commands

### access-groups

Manage access groups

- **`vercel-admin-pp-cli access-groups create`** - Allows to create an access group
- **`vercel-admin-pp-cli access-groups delete`** - Allows to delete an access group
- **`vercel-admin-pp-cli access-groups list`** - List access groups for a team, project or member
- **`vercel-admin-pp-cli access-groups read`** - Allows to read an access group
- **`vercel-admin-pp-cli access-groups update`** - Allows to update an access group metadata

### aliases

Manage aliases

- **`vercel-admin-pp-cli aliases delete-alias`** - Delete an Alias with the specified ID.
- **`vercel-admin-pp-cli aliases get-alias`** - Retrieves an Alias for the given host name or alias ID.
- **`vercel-admin-pp-cli aliases list`** - Retrieves a list of aliases for the authenticated User or Team. When `domain` is provided, only aliases for that domain will be returned. When `projectId` is provided, it will only return the given project aliases.

### artifacts

Manage artifacts

- **`vercel-admin-pp-cli artifacts download`** - Downloads a cache artifact indentified by its `hash` specified on the request path. The artifact is downloaded as an octet-stream. The client should verify the content-length header and response body.
- **`vercel-admin-pp-cli artifacts exists`** - Check that a cache artifact with the given `hash` exists. This request returns response headers only and is equivalent to a `GET` request to this endpoint where the response contains no body.
- **`vercel-admin-pp-cli artifacts query`** - Query information about an array of artifacts.
- **`vercel-admin-pp-cli artifacts record-events`** - Records an artifacts cache usage event. The body of this request is an array of cache usage events. The supported event types are `HIT` and `MISS`. The source is either `LOCAL` the cache event was on the users filesystem cache or `REMOTE` if the cache event is for a remote cache. When the event is a `HIT` the request also accepts a number `duration` which is the time taken to generate the artifact in the cache.
- **`vercel-admin-pp-cli artifacts status`** - Check the status of Remote Caching for this principal. Returns a JSON-encoded status indicating if Remote Caching is enabled, disabled, or disabled due to usage limits.
- **`vercel-admin-pp-cli artifacts upload`** - Uploads a cache artifact identified by the `hash` specified on the path. The cache artifact can then be downloaded with the provided `hash`.

### billing

Manage billing

- **`vercel-admin-pp-cli billing buy-credits`** - Purchases credits for a Vercel team using the default payment method on file. The purchase is charged immediately via Stripe invoice. Supported credit types are `v0`, `gateway`, and `agent`. The `amount` field specifies the number of credits to purchase and must be a positive integer. An optional `source` query parameter can be provided to identify the caller. Defaults to `api` if not specified. This is only available for Owner, Member, Developer, Security, and Billing roles for the supplied team.
- **`vercel-admin-pp-cli billing list-charges`** - Returns the billing charge data in FOCUS v1.3 JSONL format for a specified Vercel team, within a date range specified by `from` and `to` query parameters. Supports 1-day granularity with a maximum date range of 1 year. The response is streamed as newline-delimited JSON (JSONL) and can be optionally compressed with gzip if the `Accept-Encoding: gzip` header is provided. This is only available for Owner, Member, Developer, Security, Billing, and Enterprise Viewer roles for the supplied team.
- **`vercel-admin-pp-cli billing list-contract-commitments`** - Returns commitment allocations per contract period in FOCUS v1.3 JSONL format for a specified Vercel team. The response is streamed as newline-delimited JSON (JSONL). This endpoint is only applicable to Enterprise Vercel customers. An empty response is returned for non-Enterprise (Pro/Flex) customers.

### bulk-redirects

Manage bulk redirects

- **`vercel-admin-pp-cli bulk-redirects delete-redirects`** - Deletes the provided redirects from the latest version of the projects' bulk redirects. Stages a new change with the new redirects and returns the alias for the new version in the response.
- **`vercel-admin-pp-cli bulk-redirects edit-redirect`** - Edits a single redirect identified by its source path. Stages a new change with the modified redirect and returns the alias for the new version in the response.
- **`vercel-admin-pp-cli bulk-redirects get-redirects`** - Get the version history for a project's bulk redirects
- **`vercel-admin-pp-cli bulk-redirects get-versions`** - Get the version history for a project's bulk redirects
- **`vercel-admin-pp-cli bulk-redirects restore-redirects`** - Restores the provided redirects in the staging version to the value in the production version. If no production version exists, removes the redirects from staging.
- **`vercel-admin-pp-cli bulk-redirects stage-redirects`** - Stages new redirects for a project and returns the new version.
- **`vercel-admin-pp-cli bulk-redirects update-version`** - Update a version by promoting staging to production or restoring a previous production version

### certs

Manage certs

- **`vercel-admin-pp-cli certs get-by-id`** - Get cert by id
- **`vercel-admin-pp-cli certs issue`** - Issue a new cert
- **`vercel-admin-pp-cli certs remove`** - Remove cert
- **`vercel-admin-pp-cli certs upload`** - Upload a cert

### connect

Manage connect

- **`vercel-admin-pp-cli connect create-connector`** - Create a connector from type-specific configuration and optionally link it to a project during creation.
- **`vercel-admin-pp-cli connect create-connector-authorization-request`** - Create an authorization request for a connector and return the URL and verifier details needed to complete the flow.
- **`vercel-admin-pp-cli connect create-network`** - Allows to create a Secure Compute network.
- **`vercel-admin-pp-cli connect delete-network`** - Allows to delete a Secure Compute network.
- **`vercel-admin-pp-cli connect get-connector-token`** - Get an access token for a connector identified by the path parameter and scoped to the requester.
- **`vercel-admin-pp-cli connect list-networks`** - Allows to list Secure Compute networks.
- **`vercel-admin-pp-cli connect read-network`** - Allows to read a Secure Compute network.
- **`vercel-admin-pp-cli connect update-network`** - Allows to update a Secure Compute network.

### deployments

Manage deployments

- **`vercel-admin-pp-cli deployments create`** - Create a new deployment with all the required and intended data. If the deployment is not a git deployment, all files must be provided with the request, either referenced or inlined. Additionally, a deployment id can be specified to redeploy a previous deployment.
- **`vercel-admin-pp-cli deployments delete`** - This API allows you to delete a deployment, either by supplying its `id` in the URL or the `url` of the deployment as a query parameter. You can obtain the ID, for example, by listing all deployments.
- **`vercel-admin-pp-cli deployments get`** - List deployments under the authenticated user or team. If a deployment hasn't finished uploading (is incomplete), the `url` property will have a value of `null`.
- **`vercel-admin-pp-cli deployments get-idorurl`** - Retrieves information for a deployment either by supplying its ID (`id` property) or Hostname (`url` property). Additional details will be included when the authenticated user or team is an owner of the deployment.

### domains

Manage domains

- **`vercel-admin-pp-cli domains create-or-transfer`** - This endpoint is used for adding a new apex domain name with Vercel for the authenticating user. Note: This endpoint is no longer used for initiating domain transfers from external registrars to Vercel. For this, please use the endpoint [Transfer-in a domain](https://vercel.com/docs/rest-api/reference/endpoints/domains-registrar/transfer-in-a-domain).
- **`vercel-admin-pp-cli domains delete`** - Delete a previously registered domain name from Vercel. Deleting a domain will automatically remove any associated aliases.
- **`vercel-admin-pp-cli domains get`** - Retrieves a list of domains registered for the authenticated user or team. By default it returns the last 20 domains if no limit is provided.
- **`vercel-admin-pp-cli domains get-domain`** - Get information for a single domain in an account or team.
- **`vercel-admin-pp-cli domains patch`** - Update or move apex domain. Note: This endpoint is no longer used for updating auto-renew or nameservers. For this, please use the endpoints [Update auto-renew for a domain](https://vercel.com/docs/rest-api/reference/endpoints/domains-registrar/update-auto-renew-for-a-domain) and [Update nameservers for a domain](https://vercel.com/docs/rest-api/reference/endpoints/domains-registrar/update-nameservers-for-a-domain).
- **`vercel-admin-pp-cli domains update-record`** - Updates an existing DNS record for a domain name.

### drains

Manage drains

- **`vercel-admin-pp-cli drains create`** - Create a new Drain with the provided configuration.
- **`vercel-admin-pp-cli drains delete`** - Delete a specific Drain by passing the drain id in the URL.
- **`vercel-admin-pp-cli drains get`** - Allows to retrieve the list of Drains of the authenticated team.
- **`vercel-admin-pp-cli drains get-id`** - Get the information for a specific Drain by passing the drain id in the URL.
- **`vercel-admin-pp-cli drains test`** - Validate the delivery configuration of a Drain using sample events.
- **`vercel-admin-pp-cli drains update`** - Update the configuration of an existing drain.

### edge-cache

Manage edge cache

- **`vercel-admin-pp-cli edge-cache dangerously-delete-by-src-images`** - Marks a source image as deleted, causing cache entries associated with that source image to be revalidated in the foreground on the next request. Use this method with caution because one source image can be associated with many paths and deleting the cache can cause many concurrent requests to the origin leading to cache stampede problem. This method is for advanced use cases and is not recommended; prefer using `invalidateBySrcImage` instead.
- **`vercel-admin-pp-cli edge-cache dangerously-delete-by-tags`** - Marks a cache tag as deleted, causing cache entries associated with that tag to be revalidated in the foreground on the next request. Use this method with caution because one tag can be associated with many paths and deleting the cache can cause many concurrent requests to the origin leading to cache stampede problem. This method is for advanced use cases and is not recommended; prefer using `invalidateByTag` instead.
- **`vercel-admin-pp-cli edge-cache invalidate-by-src-images`** - Marks a source image as stale, causing its corresponding transformed images to be revalidated in the background on the next request.
- **`vercel-admin-pp-cli edge-cache invalidate-by-tags`** - Marks a cache tag as stale, causing cache entries associated with that tag to be revalidated in the background on the next request.

### edge-config

Manage edge config

- **`vercel-admin-pp-cli edge-config create`** - Creates an Edge Config.
- **`vercel-admin-pp-cli edge-config delete`** - Delete an Edge Config by id.
- **`vercel-admin-pp-cli edge-config get`** - Returns all Edge Configs.
- **`vercel-admin-pp-cli edge-config get-edgeconfig`** - Returns an Edge Config.
- **`vercel-admin-pp-cli edge-config update`** - Updates an Edge Config.

### env

Manage env

- **`vercel-admin-pp-cli env create-shared-variable`** - Creates shared environment variable(s) for a team.
- **`vercel-admin-pp-cli env delete-shared-variable`** - Deletes one or many Shared Environment Variables for a given team.
- **`vercel-admin-pp-cli env get-shared-var`** - Retrieve the decrypted value of a Shared Environment Variable by id.
- **`vercel-admin-pp-cli env list-shared-variable`** - Lists all Shared Environment Variables for a team, taking into account optional filters.
- **`vercel-admin-pp-cli env update-shared-variable`** - Updates a given Shared Environment Variable for a Team.

### events

Manage events

- **`vercel-admin-pp-cli events list-types`** - Returns the list of user-facing event types with descriptions.
- **`vercel-admin-pp-cli events list-user`** - Retrieves a list of "events" generated by the User on Vercel. Events are generated when the User performs a particular action, such as logging in, creating a deployment, and joining a Team (just to name a few). When the `teamId` parameter is supplied, then the events that are returned will be in relation to the Team that was specified.

### files

Manage files

- **`vercel-admin-pp-cli files`** - Before you create a deployment you need to upload the required files for that deployment. To do it, you need to first upload each file to this endpoint. Once that's completed, you can create a new deployment with the uploaded files. The file content must be placed inside the body of the request. In the case of a successful response you'll receive a status code 200 with an empty body.

### installations

Manage installations

- **`vercel-admin-pp-cli installations <integrationConfigurationId>`** - This endpoint updates an integration installation.

### integrations

Manage integrations

- **`vercel-admin-pp-cli integrations connect-resource-to-project`** - Connects an integration resource to a Vercel project. This endpoint establishes a connection between a provisioned integration resource (from storage APIs like `POST /v1/storage/stores/integration/direct`) and a specific Vercel project.
- **`vercel-admin-pp-cli integrations create-log-drain`** - Creates an Integration log drain. This endpoint must be called with an OAuth2 client (integration), since log drains are tied to integrations. If it is called with a different token type it will produce a 400 error.
- **`vercel-admin-pp-cli integrations delete-configuration`** - Allows to remove the configuration with the `id` provided in the parameters. The configuration and all of its resources will be removed. This includes Webhooks, LogDrains and Project Env variables.
- **`vercel-admin-pp-cli integrations delete-log-drain`** - Deletes the Integration log drain with the provided `id`. When using an OAuth2 Token, the log drain can be deleted only if the integration owns it.
- **`vercel-admin-pp-cli integrations exchange-sso-token`** - During the autorization process, Vercel sends the user to the provider [redirectLoginUrl](https://vercel.com/docs/integrations/create-integration/submit-integration#redirect-login-url), that includes the OAuth authorization `code` parameter. The provider then calls the SSO Token Exchange endpoint with the sent code and receives the OIDC token. They log the user in based on this token and redirects the user back to the Vercel account using deep-link parameters included the redirectLoginUrl. Providers should not persist the returned `id_token` in a database since the token will expire. See [**Authentication with SSO**](https://vercel.com/docs/integrations/create-integration/marketplace-api#authentication-with-sso) for more details.
- **`vercel-admin-pp-cli integrations get-billing-plans`** - Get a list of billing plans for an integration and product.
- **`vercel-admin-pp-cli integrations get-configuration`** - Allows to retrieve a the configuration with the provided id in case it exists. The authenticated user or team must be the owner of the config in order to access it.
- **`vercel-admin-pp-cli integrations get-configuration-products`** - Returns products available for an integration configuration. Each product includes a `metadataSchema` field with the JSON Schema for required and optional metadata fields.
- **`vercel-admin-pp-cli integrations get-configurations`** - Allows to retrieve all configurations for an authenticated integration. When the `project` view is used, configurations generated for the authorization flow will be filtered out of the results.
- **`vercel-admin-pp-cli integrations get-log-drains`** - Retrieves a list of all Integration log drains that are defined for the authenticated user or team. When using an OAuth2 token, the list is limited to log drains created by the authenticated integration.
- **`vercel-admin-pp-cli integrations git-namespaces`** - Lists git namespaces for a supported provider. Supported providers are `github`, `gitlab` and `bitbucket`. If the provider is not provided, it will try to obtain it from the user that authenticated the request.
- **`vercel-admin-pp-cli integrations search-repo`** - Lists git repositories linked to a namespace `id` for a supported provider. A specific namespace `id` can be obtained via the `git-namespaces`  endpoint. Supported providers are `github`, `gitlab` and `bitbucket`. If the provider or namespace is not provided, it will try to obtain it from the user that authenticated the request.

### log-drains

Manage log drains

- **`vercel-admin-pp-cli log-drains create-configurable`** - Creates a configurable log drain. This endpoint must be called with a team AccessToken (integration OAuth2 clients are not allowed)
- **`vercel-admin-pp-cli log-drains delete-configurable`** - Deletes a Configurable Log Drain. This endpoint must be called with a team AccessToken (integration OAuth2 clients are not allowed). Only log drains owned by the authenticated team can be deleted.
- **`vercel-admin-pp-cli log-drains get-all`** - Retrieves a list of all the Log Drains owned by the account. This endpoint must be called with an account AccessToken (integration OAuth2 clients are not allowed). Only log drains owned by the authenticated account can be accessed.
- **`vercel-admin-pp-cli log-drains get-configurable`** - Retrieves a Configurable Log Drain. This endpoint must be called with a team AccessToken (integration OAuth2 clients are not allowed). Only log drains owned by the authenticated team can be accessed.

### microfrontends

Manage microfrontends

- **`vercel-admin-pp-cli microfrontends create-group-with-applications`** - Creates a microfrontends group and attaches multiple projects in a single request.
- **`vercel-admin-pp-cli microfrontends get-config-for-project`** - Get the microfrontends config for a project by ID or name.
- **`vercel-admin-pp-cli microfrontends get-groups`** - Get the microfrontends group IDs for a team.
- **`vercel-admin-pp-cli microfrontends get-in-group`** - Get the microfrontends for a given group ID.

### observability

Manage observability

- **`vercel-admin-pp-cli observability get-configuration-projects`** - Lists the projects that are currently configured as disabled for Observability Plus on a team.
- **`vercel-admin-pp-cli observability update-configuration-project`** - Updates whether Observability Plus is disabled for a single project.

### projects

Manage projects

- **`vercel-admin-pp-cli projects accept-transfer-request`** - Accept a project transfer request initated by another team. <br/> The `code` is generated using the `POST /projects/:idOrName/transfer-request` endpoint.
- **`vercel-admin-pp-cli projects create`** - Allows to create a new project with the provided configuration. It only requires the project `name` but more configuration can be provided to override the defaults.
- **`vercel-admin-pp-cli projects delete`** - Delete a specific project by passing either the project `id` or `name` in the URL.
- **`vercel-admin-pp-cli projects get`** - Allows to retrieve the list of projects of the authenticated user or team. The list will be paginated and the provided query parameters allow filtering the returned projects.
- **`vercel-admin-pp-cli projects get-idorname`** - Get the information for a specific project by passing either the project `id` or `name` in the URL.
- **`vercel-admin-pp-cli projects update`** - Update the fields of a project using either its `name` or `id`.

### registrar

Manage registrar

- **`vercel-admin-pp-cli registrar buy-domains`** - Buy multiple domains at once
- **`vercel-admin-pp-cli registrar buy-single-domain`** - Buy a domain
- **`vercel-admin-pp-cli registrar get-bulk-availability`** - Get availability for multiple domains. If the domains are available, they can be purchased using the [Buy a domain](https://vercel.com/docs/rest-api/reference/endpoints/domains-registrar/buy-a-domain) endpoint or the [Buy multiple domains](https://vercel.com/docs/rest-api/reference/endpoints/domains-registrar/buy-multiple-domains) endpoint.
- **`vercel-admin-pp-cli registrar get-contact-info-schema`** - Some TLDs require additional contact information. Use this endpoint to get the schema for the tld-specific contact information for a domain.
- **`vercel-admin-pp-cli registrar get-domain-auth-code`** - Get the auth code for a domain. This is required to transfer a domain from Vercel to another registrar.
- **`vercel-admin-pp-cli registrar get-domain-availability`** - Get availability for a specific domain. If the domain is available, it can be purchased using the [Buy a domain](https://vercel.com/docs/rest-api/reference/endpoints/domains-registrar/buy-a-domain) endpoint or the [Buy multiple domains](https://vercel.com/docs/rest-api/reference/endpoints/domains-registrar/buy-multiple-domains) endpoint.
- **`vercel-admin-pp-cli registrar get-domain-price`** - Get price data for a specific domain
- **`vercel-admin-pp-cli registrar get-domain-transfer-in`** - Get the transfer status for a domain
- **`vercel-admin-pp-cli registrar get-order`** - Get information about a domain order by its ID
- **`vercel-admin-pp-cli registrar get-supported-tlds`** - Get a list of TLDs supported by Vercel
- **`vercel-admin-pp-cli registrar get-tld`** - Get the metadata for a specific TLD.
- **`vercel-admin-pp-cli registrar get-tld-price`** - Get price data for a specific TLD. This only reflects base prices for the given TLD. Premium domains may have different prices. Use the [Get price data for a domain](https://vercel.com/docs/rest-api/reference/endpoints/domains-registrar/get-price-data-for-a-domain) endpoint to get the price data for a specific domain.
- **`vercel-admin-pp-cli registrar renew-domain`** - Renew a domain
- **`vercel-admin-pp-cli registrar transfer-in-domain`** - Transfer a domain in from another registrar
- **`vercel-admin-pp-cli registrar update-domain-auto-renew`** - Update the auto-renew setting for a domain
- **`vercel-admin-pp-cli registrar update-domain-nameservers`** - Update the nameservers for a domain. Pass an empty array to use Vercel's default nameservers.

### sandboxes

Manage sandboxes

- **`vercel-admin-pp-cli sandboxes create`** - Creates a named sandbox environment. Named sandboxes have a unique name within a project and support automatic snapshotting on shutdown.
- **`vercel-admin-pp-cli sandboxes create-session-directory`** - Creates a new directory in a session's filesystem. By default, parent directories are created recursively if they don't exist (similar to `mkdir -p`).
- **`vercel-admin-pp-cli sandboxes create-session-snapshot`** - Creates a point-in-time snapshot of a running session's filesystem. Snapshots can be used to quickly restore a session to a previous state or to create new sessions with pre-configured environments. The session must be running and able to accept commands for a snapshot to be created. The session will be terminated after the snapshot is created.
- **`vercel-admin-pp-cli sandboxes delete-drive`** - Deletes a drive by project and name. Attached drives cannot be deleted. Stop or replace the session currently using the drive before retrying deletion. Drives are in private beta. Register your interest to get access: https://vercel.com/changelog/drives-for-vercel-sandbox-in-private-beta
- **`vercel-admin-pp-cli sandboxes delete-sandbox`** - Deletes a sandbox by name. If sandboxes are currently running, they will be stopped first. This operation deletes all sandbox entities with the given name and the named sandbox metadata.
- **`vercel-admin-pp-cli sandboxes delete-session-snapshot`** - Permanently deletes a snapshot and frees its associated storage. This action cannot be undone. After deletion, the snapshot can no longer be used to create new sessions.
- **`vercel-admin-pp-cli sandboxes extend-session-timeout`** - Extends the maximum execution time of a running session. The session must be active and able to accept commands. The total timeout cannot exceed the maximum allowed limit for your account.
- **`vercel-admin-pp-cli sandboxes get-named-sandbox`** - Retrieves a named sandbox by name, including its current sandbox and routes. If the sandbox is stopped and resume is true, a new sandbox will be created from the most recent snapshot.
- **`vercel-admin-pp-cli sandboxes get-or-create-drive`** - Gets an existing drive by project and name, or creates it when it does not exist. Drives are in private beta. Register your interest to get access: https://vercel.com/changelog/drives-for-vercel-sandbox-in-private-beta
- **`vercel-admin-pp-cli sandboxes get-session`** - Retrieves detailed information about a specific session, including its current status, resource configuration, and exposed routes.
- **`vercel-admin-pp-cli sandboxes get-session-command`** - Retrieves the current status and details of a command executed in a session. Use the `wait` parameter to block until the command finishes execution.
- **`vercel-admin-pp-cli sandboxes get-session-command-logs`** - Streams the output of a command in real-time using newline-delimited JSON (ND-JSON). Each entry includes the output data and stream type. Stream types include `stdout`, `stderr`, and `error` (for stream failures).
- **`vercel-admin-pp-cli sandboxes get-session-snapshot`** - Retrieves detailed information about a specific snapshot, including its creation time, size, expiration date, and the source session it was created from.
- **`vercel-admin-pp-cli sandboxes kill-session-command`** - Sends a signal to terminate a running command in a session. The signal can be used to gracefully stop (SIGTERM) or forcefully kill (SIGKILL) the process. The command must still be running for this operation to succeed.
- **`vercel-admin-pp-cli sandboxes list`** - Retrieves a paginated list of named sandboxes belonging to a specific project. Results can be sorted by creation time or name, and optionally filtered by name prefix.
- **`vercel-admin-pp-cli sandboxes list-drives`** - Retrieves a paginated list of drives belonging to a specific project. Drives are in private beta. Register your interest to get access: https://vercel.com/changelog/drives-for-vercel-sandbox-in-private-beta
- **`vercel-admin-pp-cli sandboxes list-session-commands`** - Retrieves a list of all commands that have been executed in a session, including their current status, exit codes, and execution times, ordered from the most recent to the oldest.
- **`vercel-admin-pp-cli sandboxes list-session-snapshots`** - Retrieves a paginated list of snapshots for a specific project.
- **`vercel-admin-pp-cli sandboxes list-sessions`** - Retrieves a paginated list of sessions belonging to a specific sandbox. Results are sorted by creation time and paginated using an opaque cursor.
- **`vercel-admin-pp-cli sandboxes read-session-file`** - Downloads the contents of a file from a session's filesystem. The file content is returned as a binary stream with appropriate Content-Disposition headers for file download.
- **`vercel-admin-pp-cli sandboxes run-session-command`** - Executes a shell command inside a running session. The command runs asynchronously and returns immediately with a command ID that can be used to track its progress and retrieve its output. Optionally, use the `wait` parameter to stream the command status until completion.
- **`vercel-admin-pp-cli sandboxes stop-session`** - Stops a running session and releases its allocated resources. All running processes within the session will be terminated. This action cannot be undone. A stopped session cannot be restarted.
- **`vercel-admin-pp-cli sandboxes update-sandbox`** - Updates the configuration of a sandbox. Only the provided fields will be modified; omitted fields remain unchanged.
- **`vercel-admin-pp-cli sandboxes update-session-network-policy`** - Replaces the network access policy of a running session. Use this to control which external hosts the session can communicate with. This is a full replacement. Any previously configured network rules will be overwritten.
- **`vercel-admin-pp-cli sandboxes write-session-files`** - Uploads and extracts files to a session's filesystem. Files must be uploaded as a gzipped tarball (`.tar.gz`) with the `Content-Type` header set to `application/gzip`. The tarball contents are extracted to the session's working directory, or to a custom directory specified via the `x-cwd` header.

### security

Manage security

- **`vercel-admin-pp-cli security add-bypass-ip`** - Create new system bypass rules
- **`vercel-admin-pp-cli security get-active-attack-status`** - Retrieve active attack data within the last N days (default: 1 day)
- **`vercel-admin-pp-cli security get-bypass-ip`** - Retrieve the system bypass rules configured for the specified project
- **`vercel-admin-pp-cli security get-firewall-config`** - Retrieve the specified firewall configuration for a project. The deployed configVersion will be `active`
- **`vercel-admin-pp-cli security get-firewall-events`** - Retrieve firewall actions for a project
- **`vercel-admin-pp-cli security put-firewall-config`** - Set the firewall configuration to provided rules and settings. Creates or overwrite the existing firewall configuration.
- **`vercel-admin-pp-cli security remove-bypass-ip`** - Remove system bypass rules
- **`vercel-admin-pp-cli security update-attack-challenge-mode`** - Update the setting for determining if the project has Attack Challenge mode enabled.
- **`vercel-admin-pp-cli security update-firewall-config`** - Process updates to modify the existing firewall config for a project

### storage

Manage storage

- **`vercel-admin-pp-cli storage`** - Creates an integration store with automatic billing plan handling. For free resources, omit `billingPlanId` to auto-discover free plans. For paid resources, provide a `billingPlanId` from the billing plans endpoint.

### teams

Manage teams

- **`vercel-admin-pp-cli teams create`** - Create a new Team under your account. You need to send a POST request with the desired Team slug, and optionally the Team name.
- **`vercel-admin-pp-cli teams delete`** - Delete a team under your account. You need to send a `DELETE` request with the desired team `id`. An optional array of reasons for deletion may also be sent.
- **`vercel-admin-pp-cli teams get`** - Get a paginated list of all the Teams the authenticated User is a member of.
- **`vercel-admin-pp-cli teams get-teamid`** - Get information for the Team specified by the `teamId` parameter.
- **`vercel-admin-pp-cli teams patch`** - Update the information of a Team specified by the `teamId` parameter. The request body should contain the information that will be updated on the Team.

### user

Manage user

- **`vercel-admin-pp-cli user create-auth-token`** - Creates and returns a new authentication token for the currently authenticated User. The `bearerToken` property is only provided once, in the response body, so be sure to save it on the client for use with API requests.
- **`vercel-admin-pp-cli user delete-auth-token`** - Invalidate an authentication token, such that it will no longer be valid for future HTTP requests.
- **`vercel-admin-pp-cli user get-auth`** - Retrieves information related to the currently authenticated User.
- **`vercel-admin-pp-cli user get-auth-token`** - Retrieve metadata about an authentication token belonging to the currently authenticated User.
- **`vercel-admin-pp-cli user list-auth-tokens`** - Retrieve a list of the current User's authentication tokens.
- **`vercel-admin-pp-cli user request-delete`** - Initiates the deletion process for the currently authenticated User, by sending a deletion confirmation email. The email contains a link that the user needs to visit in order to proceed with the deletion process.

### webhooks

Manage webhooks

- **`vercel-admin-pp-cli webhooks create`** - Creates a webhook
- **`vercel-admin-pp-cli webhooks delete`** - Deletes a webhook
- **`vercel-admin-pp-cli webhooks get`** - Get a list of webhooks
- **`vercel-admin-pp-cli webhooks get-id`** - Get a webhook


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
vercel-admin-pp-cli access-groups list

# JSON for scripting and agents
vercel-admin-pp-cli access-groups list --json

# Filter to specific fields
vercel-admin-pp-cli access-groups list --json --select id,name,status

# Dry run — show the request without sending
vercel-admin-pp-cli access-groups list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
vercel-admin-pp-cli access-groups list --agent
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
vercel-admin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/vercel-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `VERCEL_ADMIN_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `vercel-admin-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `vercel-admin-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $VERCEL_ADMIN_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

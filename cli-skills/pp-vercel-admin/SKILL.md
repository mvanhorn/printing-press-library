---
name: pp-vercel-admin
description: "Printing Press CLI for Vercel Admin. Vercel combines the best developer experience with an obsessive focus on end-user performance."
author: "hnshah"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - vercel-admin-pp-cli
    install:
      - kind: go
        bins: [vercel-admin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/vercel-admin/cmd/vercel-admin-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/cloud/vercel-admin/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Vercel Admin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `vercel-admin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install vercel-admin --cli-only
   ```
2. Verify: `vercel-admin-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/vercel-admin/cmd/vercel-admin-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Vercel combines the best developer experience with an obsessive focus on end-user performance. Our platform enables frontend teams to do their best work.

## Command Reference

**access-groups** — Manage access groups

- `vercel-admin-pp-cli access-groups create` — Allows to create an access group
- `vercel-admin-pp-cli access-groups delete` — Allows to delete an access group
- `vercel-admin-pp-cli access-groups list` — List access groups for a team, project or member
- `vercel-admin-pp-cli access-groups read` — Allows to read an access group
- `vercel-admin-pp-cli access-groups update` — Allows to update an access group metadata

**aliases** — Manage aliases

- `vercel-admin-pp-cli aliases delete-alias` — Delete an Alias with the specified ID.
- `vercel-admin-pp-cli aliases get-alias` — Retrieves an Alias for the given host name or alias ID.
- `vercel-admin-pp-cli aliases list` — Retrieves a list of aliases for the authenticated User or Team.

**artifacts** — Manage artifacts

- `vercel-admin-pp-cli artifacts download` — Downloads a cache artifact indentified by its `hash` specified on the request path.
- `vercel-admin-pp-cli artifacts exists` — Check that a cache artifact with the given `hash` exists.
- `vercel-admin-pp-cli artifacts query` — Query information about an array of artifacts.
- `vercel-admin-pp-cli artifacts record-events` — Records an artifacts cache usage event. The body of this request is an array of cache usage events.
- `vercel-admin-pp-cli artifacts status` — Check the status of Remote Caching for this principal.
- `vercel-admin-pp-cli artifacts upload` — Uploads a cache artifact identified by the `hash` specified on the path.

**billing** — Manage billing

- `vercel-admin-pp-cli billing buy-credits` — Purchases credits for a Vercel team using the default payment method on file.
- `vercel-admin-pp-cli billing list-charges` — Returns the billing charge data in FOCUS v1.
- `vercel-admin-pp-cli billing list-contract-commitments` — Returns commitment allocations per contract period in FOCUS v1.3 JSONL format for a specified Vercel team.

**bulk-redirects** — Manage bulk redirects

- `vercel-admin-pp-cli bulk-redirects delete-redirects` — Deletes the provided redirects from the latest version of the projects' bulk redirects.
- `vercel-admin-pp-cli bulk-redirects edit-redirect` — Edits a single redirect identified by its source path.
- `vercel-admin-pp-cli bulk-redirects get-redirects` — Get the version history for a project's bulk redirects
- `vercel-admin-pp-cli bulk-redirects get-versions` — Get the version history for a project's bulk redirects
- `vercel-admin-pp-cli bulk-redirects restore-redirects` — Restores the provided redirects in the staging version to the value in the production version.
- `vercel-admin-pp-cli bulk-redirects stage-redirects` — Stages new redirects for a project and returns the new version.
- `vercel-admin-pp-cli bulk-redirects update-version` — Update a version by promoting staging to production or restoring a previous production version

**certs** — Manage certs

- `vercel-admin-pp-cli certs get-by-id` — Get cert by id
- `vercel-admin-pp-cli certs issue` — Issue a new cert
- `vercel-admin-pp-cli certs remove` — Remove cert
- `vercel-admin-pp-cli certs upload` — Upload a cert

**connect** — Manage connect

- `vercel-admin-pp-cli connect create-connector` — Create a connector from type-specific configuration and optionally link it to a project during creation.
- `vercel-admin-pp-cli connect create-connector-authorization-request` — Create an authorization request for a connector and return the URL and verifier details needed to complete the flow.
- `vercel-admin-pp-cli connect create-network` — Allows to create a Secure Compute network.
- `vercel-admin-pp-cli connect delete-network` — Allows to delete a Secure Compute network.
- `vercel-admin-pp-cli connect get-connector-token` — Get an access token for a connector identified by the path parameter and scoped to the requester.
- `vercel-admin-pp-cli connect list-networks` — Allows to list Secure Compute networks.
- `vercel-admin-pp-cli connect read-network` — Allows to read a Secure Compute network.
- `vercel-admin-pp-cli connect update-network` — Allows to update a Secure Compute network.

**deployments** — Manage deployments

- `vercel-admin-pp-cli deployments create` — Create a new deployment with all the required and intended data.
- `vercel-admin-pp-cli deployments delete` — This API allows you to delete a deployment
- `vercel-admin-pp-cli deployments get` — List deployments under the authenticated user or team.
- `vercel-admin-pp-cli deployments get-idorurl` — Retrieves information for a deployment either by supplying its ID (`id` property) or Hostname (`url` property).

**domains** — Manage domains

- `vercel-admin-pp-cli domains create-or-transfer` — This endpoint is used for adding a new apex domain name with Vercel for the authenticating user.
- `vercel-admin-pp-cli domains delete` — Delete a previously registered domain name from Vercel.
- `vercel-admin-pp-cli domains get` — Retrieves a list of domains registered for the authenticated user or team.
- `vercel-admin-pp-cli domains get-domain` — Get information for a single domain in an account or team.
- `vercel-admin-pp-cli domains patch` — Update or move apex domain. Note: This endpoint is no longer used for updating auto-renew or nameservers.
- `vercel-admin-pp-cli domains update-record` — Updates an existing DNS record for a domain name.

**drains** — Manage drains

- `vercel-admin-pp-cli drains create` — Create a new Drain with the provided configuration.
- `vercel-admin-pp-cli drains delete` — Delete a specific Drain by passing the drain id in the URL.
- `vercel-admin-pp-cli drains get` — Allows to retrieve the list of Drains of the authenticated team.
- `vercel-admin-pp-cli drains get-id` — Get the information for a specific Drain by passing the drain id in the URL.
- `vercel-admin-pp-cli drains test` — Validate the delivery configuration of a Drain using sample events.
- `vercel-admin-pp-cli drains update` — Update the configuration of an existing drain.

**edge-cache** — Manage edge cache

- `vercel-admin-pp-cli edge-cache dangerously-delete-by-src-images` — Marks a source image as deleted
- `vercel-admin-pp-cli edge-cache dangerously-delete-by-tags` — Marks a cache tag as deleted
- `vercel-admin-pp-cli edge-cache invalidate-by-src-images` — Marks a source image as stale
- `vercel-admin-pp-cli edge-cache invalidate-by-tags` — Marks a cache tag as stale

**edge-config** — Manage edge config

- `vercel-admin-pp-cli edge-config create` — Creates an Edge Config.
- `vercel-admin-pp-cli edge-config delete` — Delete an Edge Config by id.
- `vercel-admin-pp-cli edge-config get` — Returns all Edge Configs.
- `vercel-admin-pp-cli edge-config get-edgeconfig` — Returns an Edge Config.
- `vercel-admin-pp-cli edge-config update` — Updates an Edge Config.

**env** — Manage env

- `vercel-admin-pp-cli env create-shared-variable` — Creates shared environment variable(s) for a team.
- `vercel-admin-pp-cli env delete-shared-variable` — Deletes one or many Shared Environment Variables for a given team.
- `vercel-admin-pp-cli env get-shared-var` — Retrieve the decrypted value of a Shared Environment Variable by id.
- `vercel-admin-pp-cli env list-shared-variable` — Lists all Shared Environment Variables for a team, taking into account optional filters.
- `vercel-admin-pp-cli env update-shared-variable` — Updates a given Shared Environment Variable for a Team.

**events** — Manage events

- `vercel-admin-pp-cli events list-types` — Returns the list of user-facing event types with descriptions.
- `vercel-admin-pp-cli events list-user` — Retrieves a list of 'events' generated by the User on Vercel.

**files** — Manage files

- `vercel-admin-pp-cli files` — Before you create a deployment you need to upload the required files for that deployment.

**installations** — Manage installations

- `vercel-admin-pp-cli installations <integrationConfigurationId>` — This endpoint updates an integration installation.

**integrations** — Manage integrations

- `vercel-admin-pp-cli integrations connect-resource-to-project` — Connects an integration resource to a Vercel project.
- `vercel-admin-pp-cli integrations create-log-drain` — Creates an Integration log drain.
- `vercel-admin-pp-cli integrations delete-configuration` — Allows to remove the configuration with the `id` provided in the parameters.
- `vercel-admin-pp-cli integrations delete-log-drain` — Deletes the Integration log drain with the provided `id`.
- `vercel-admin-pp-cli integrations exchange-sso-token` — During the autorization process, Vercel sends the user to the provider [redirectLoginUrl](https://vercel.
- `vercel-admin-pp-cli integrations get-billing-plans` — Get a list of billing plans for an integration and product.
- `vercel-admin-pp-cli integrations get-configuration` — Allows to retrieve a the configuration with the provided id in case it exists.
- `vercel-admin-pp-cli integrations get-configuration-products` — Returns products available for an integration configuration.
- `vercel-admin-pp-cli integrations get-configurations` — Allows to retrieve all configurations for an authenticated integration.
- `vercel-admin-pp-cli integrations get-log-drains` — Retrieves a list of all Integration log drains that are defined for the authenticated user or team.
- `vercel-admin-pp-cli integrations git-namespaces` — Lists git namespaces for a supported provider. Supported providers are `github`, `gitlab` and `bitbucket`.
- `vercel-admin-pp-cli integrations search-repo` — Lists git repositories linked to a namespace `id` for a supported provider.

**log-drains** — Manage log drains

- `vercel-admin-pp-cli log-drains create-configurable` — Creates a configurable log drain.
- `vercel-admin-pp-cli log-drains delete-configurable` — Deletes a Configurable Log Drain.
- `vercel-admin-pp-cli log-drains get-all` — Retrieves a list of all the Log Drains owned by the account.
- `vercel-admin-pp-cli log-drains get-configurable` — Retrieves a Configurable Log Drain.

**microfrontends** — Manage microfrontends

- `vercel-admin-pp-cli microfrontends create-group-with-applications` — Creates a microfrontends group and attaches multiple projects in a single request.
- `vercel-admin-pp-cli microfrontends get-config-for-project` — Get the microfrontends config for a project by ID or name.
- `vercel-admin-pp-cli microfrontends get-groups` — Get the microfrontends group IDs for a team.
- `vercel-admin-pp-cli microfrontends get-in-group` — Get the microfrontends for a given group ID.

**observability** — Manage observability

- `vercel-admin-pp-cli observability get-configuration-projects` — Lists the projects that are currently configured as disabled for Observability Plus on a team.
- `vercel-admin-pp-cli observability update-configuration-project` — Updates whether Observability Plus is disabled for a single project.

**projects** — Manage projects

- `vercel-admin-pp-cli projects accept-transfer-request` — Accept a project transfer request initated by another team.
- `vercel-admin-pp-cli projects create` — Allows to create a new project with the provided configuration.
- `vercel-admin-pp-cli projects delete` — Delete a specific project by passing either the project `id` or `name` in the URL.
- `vercel-admin-pp-cli projects get` — Allows to retrieve the list of projects of the authenticated user or team.
- `vercel-admin-pp-cli projects get-idorname` — Get the information for a specific project by passing either the project `id` or `name` in the URL.
- `vercel-admin-pp-cli projects update` — Update the fields of a project using either its `name` or `id`.

**registrar** — Manage registrar

- `vercel-admin-pp-cli registrar buy-domains` — Buy multiple domains at once
- `vercel-admin-pp-cli registrar buy-single-domain` — Buy a domain
- `vercel-admin-pp-cli registrar get-bulk-availability` — Get availability for multiple domains.
- `vercel-admin-pp-cli registrar get-contact-info-schema` — Some TLDs require additional contact information.
- `vercel-admin-pp-cli registrar get-domain-auth-code` — Get the auth code for a domain. This is required to transfer a domain from Vercel to another registrar.
- `vercel-admin-pp-cli registrar get-domain-availability` — Get availability for a specific domain.
- `vercel-admin-pp-cli registrar get-domain-price` — Get price data for a specific domain
- `vercel-admin-pp-cli registrar get-domain-transfer-in` — Get the transfer status for a domain
- `vercel-admin-pp-cli registrar get-order` — Get information about a domain order by its ID
- `vercel-admin-pp-cli registrar get-supported-tlds` — Get a list of TLDs supported by Vercel
- `vercel-admin-pp-cli registrar get-tld` — Get the metadata for a specific TLD.
- `vercel-admin-pp-cli registrar get-tld-price` — Get price data for a specific TLD. This only reflects base prices for the given TLD.
- `vercel-admin-pp-cli registrar renew-domain` — Renew a domain
- `vercel-admin-pp-cli registrar transfer-in-domain` — Transfer a domain in from another registrar
- `vercel-admin-pp-cli registrar update-domain-auto-renew` — Update the auto-renew setting for a domain
- `vercel-admin-pp-cli registrar update-domain-nameservers` — Update the nameservers for a domain. Pass an empty array to use Vercel's default nameservers.

**sandboxes** — Manage sandboxes

- `vercel-admin-pp-cli sandboxes create` — Creates a named sandbox environment.
- `vercel-admin-pp-cli sandboxes create-session-directory` — Creates a new directory in a session's filesystem.
- `vercel-admin-pp-cli sandboxes create-session-snapshot` — Creates a point-in-time snapshot of a running session's filesystem.
- `vercel-admin-pp-cli sandboxes delete-drive` — Deletes a drive by project and name. Attached drives cannot be deleted.
- `vercel-admin-pp-cli sandboxes delete-sandbox` — Deletes a sandbox by name. If sandboxes are currently running, they will be stopped first.
- `vercel-admin-pp-cli sandboxes delete-session-snapshot` — Permanently deletes a snapshot and frees its associated storage. This action cannot be undone.
- `vercel-admin-pp-cli sandboxes extend-session-timeout` — Extends the maximum execution time of a running session. The session must be active and able to accept commands.
- `vercel-admin-pp-cli sandboxes get-named-sandbox` — Retrieves a named sandbox by name, including its current sandbox and routes.
- `vercel-admin-pp-cli sandboxes get-or-create-drive` — Gets an existing drive by project and name, or creates it when it does not exist. Drives are in private beta.
- `vercel-admin-pp-cli sandboxes get-session` — Retrieves detailed information about a specific session, including its current status, resource configuration
- `vercel-admin-pp-cli sandboxes get-session-command` — Retrieves the current status and details of a command executed in a session.
- `vercel-admin-pp-cli sandboxes get-session-command-logs` — Streams the output of a command in real-time using newline-delimited JSON (ND-JSON).
- `vercel-admin-pp-cli sandboxes get-session-snapshot` — Retrieves detailed information about a specific snapshot, including its creation time, size, expiration date
- `vercel-admin-pp-cli sandboxes kill-session-command` — Sends a signal to terminate a running command in a session.
- `vercel-admin-pp-cli sandboxes list` — Retrieves a paginated list of named sandboxes belonging to a specific project.
- `vercel-admin-pp-cli sandboxes list-drives` — Retrieves a paginated list of drives belonging to a specific project. Drives are in private beta.
- `vercel-admin-pp-cli sandboxes list-session-commands` — Retrieves a list of all commands that have been executed in a session, including their current status, exit codes
- `vercel-admin-pp-cli sandboxes list-session-snapshots` — Retrieves a paginated list of snapshots for a specific project.
- `vercel-admin-pp-cli sandboxes list-sessions` — Retrieves a paginated list of sessions belonging to a specific sandbox.
- `vercel-admin-pp-cli sandboxes read-session-file` — Downloads the contents of a file from a session's filesystem.
- `vercel-admin-pp-cli sandboxes run-session-command` — Executes a shell command inside a running session.
- `vercel-admin-pp-cli sandboxes stop-session` — Stops a running session and releases its allocated resources.
- `vercel-admin-pp-cli sandboxes update-sandbox` — Updates the configuration of a sandbox. Only the provided fields will be modified; omitted fields remain unchanged.
- `vercel-admin-pp-cli sandboxes update-session-network-policy` — Replaces the network access policy of a running session.
- `vercel-admin-pp-cli sandboxes write-session-files` — Uploads and extracts files to a session's filesystem. Files must be uploaded as a gzipped tarball (`.tar.

**security** — Manage security

- `vercel-admin-pp-cli security add-bypass-ip` — Create new system bypass rules
- `vercel-admin-pp-cli security get-active-attack-status` — Retrieve active attack data within the last N days (default: 1 day)
- `vercel-admin-pp-cli security get-bypass-ip` — Retrieve the system bypass rules configured for the specified project
- `vercel-admin-pp-cli security get-firewall-config` — Retrieve the specified firewall configuration for a project. The deployed configVersion will be `active`
- `vercel-admin-pp-cli security get-firewall-events` — Retrieve firewall actions for a project
- `vercel-admin-pp-cli security put-firewall-config` — Set the firewall configuration to provided rules and settings. Creates or overwrite the existing firewall configuration.
- `vercel-admin-pp-cli security remove-bypass-ip` — Remove system bypass rules
- `vercel-admin-pp-cli security update-attack-challenge-mode` — Update the setting for determining if the project has Attack Challenge mode enabled.
- `vercel-admin-pp-cli security update-firewall-config` — Process updates to modify the existing firewall config for a project

**storage** — Manage storage

- `vercel-admin-pp-cli storage` — Creates an integration store with automatic billing plan handling.

**teams** — Manage teams

- `vercel-admin-pp-cli teams create` — Create a new Team under your account.
- `vercel-admin-pp-cli teams delete` — Delete a team under your account. You need to send a `DELETE` request with the desired team `id`.
- `vercel-admin-pp-cli teams get` — Get a paginated list of all the Teams the authenticated User is a member of.
- `vercel-admin-pp-cli teams get-teamid` — Get information for the Team specified by the `teamId` parameter.
- `vercel-admin-pp-cli teams patch` — Update the information of a Team specified by the `teamId` parameter.

**user** — Manage user

- `vercel-admin-pp-cli user create-auth-token` — Creates and returns a new authentication token for the currently authenticated User.
- `vercel-admin-pp-cli user delete-auth-token` — Invalidate an authentication token, such that it will no longer be valid for future HTTP requests.
- `vercel-admin-pp-cli user get-auth` — Retrieves information related to the currently authenticated User.
- `vercel-admin-pp-cli user get-auth-token` — Retrieve metadata about an authentication token belonging to the currently authenticated User.
- `vercel-admin-pp-cli user list-auth-tokens` — Retrieve a list of the current User's authentication tokens.
- `vercel-admin-pp-cli user request-delete` — Initiates the deletion process for the currently authenticated User, by sending a deletion confirmation email.

**webhooks** — Manage webhooks

- `vercel-admin-pp-cli webhooks create` — Creates a webhook
- `vercel-admin-pp-cli webhooks delete` — Deletes a webhook
- `vercel-admin-pp-cli webhooks get` — Get a list of webhooks
- `vercel-admin-pp-cli webhooks get-id` — Get a webhook


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
vercel-admin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `vercel-admin-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
vercel-admin-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `VERCEL_ADMIN_TOKEN` as an environment variable.

Run `vercel-admin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  vercel-admin-pp-cli access-groups list --agent --select id,name,status
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
vercel-admin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
vercel-admin-pp-cli feedback --stdin < notes.txt
vercel-admin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/vercel-admin-pp-cli/feedback.jsonl`. They are never POSTed unless `VERCEL_ADMIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `VERCEL_ADMIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
vercel-admin-pp-cli profile save briefing --json
vercel-admin-pp-cli --profile briefing access-groups list
vercel-admin-pp-cli profile list --json
vercel-admin-pp-cli profile show briefing
vercel-admin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `vercel-admin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/vercel-admin/cmd/vercel-admin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add vercel-admin-pp-mcp -- vercel-admin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which vercel-admin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   vercel-admin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `vercel-admin-pp-cli <command> --help`.

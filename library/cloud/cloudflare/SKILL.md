---
name: pp-cloudflare
description: "Unified Cloudflare CLI. Covers DNS, Workers (KV/D1/R2/Pages), Zones, Cache, Page Rules, Rulesets, SSL/TLS, IAM, API Tokens — plus 8 transcendence commands (idempotent dns apply, redirect set, propagate watch, where-is, zones diff, worker bindings show, cache purge release, setup_zone). Trigger phrases: `set up dns for`, `purge cloudflare cache`, `redirect to`, `propagation check`, `where is this domain`, `use cloudflare`, `run cloudflare-pp-cli`."
author: "Alex Osti"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cloudflare-pp-cli
---

# Cloudflare — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cloudflare-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install cloudflare --cli-only
   ```
2. Verify: `cloudflare-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/cloudflare/cmd/cloudflare-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI wraps the Cloudflare API directly for DNS, Workers, R2, KV, D1, Pages, Cache, Page Rules, Rulesets, SSL/TLS, IAM, and API Tokens. It complements wrangler by giving agents a smaller API-shaped surface with JSON-first output, field selection, local sync, search, analytics, and workflow commands.

## When to Use This CLI

Use this CLI when you need to manage Cloudflare imperatively across multiple products in one place: DNS + Workers + Page Rules + Access + Cache + WAF. It also doubles as an MCP server with the Cloudflare-pattern search+execute pair, so agents acting on infrastructure can navigate the covered API surface without burning tokens on per-endpoint tool definitions.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Idempotent infra
- **`dns apply`** — Apply a single DNS record with no-op-if-identical semantics, so the same command is safe to run repeatedly.

  _Reach for this when you need to provision DNS from a script or agent — guaranteed safe to run again with the same args._

  ```bash
  cloudflare-pp-cli dns apply --zone example.com --type A --name @ --content 203.0.113.10 --json
  ```
- **`redirect set`** — Create a 301/302 redirect from a URL pattern to a URL template — the Page Rule under the hood is composed for you.

  _Reach for this when you want a domain-level 301 redirect without hand-rolling a Page Rule body._

  ```bash
  cloudflare-pp-cli redirect set "legacy.example.com/*" "https://example.com/$1" --status 301 --json
  ```

### Verification
- **`propagate watch`** — Verify a DNS record has propagated by querying multiple public resolvers (Cloudflare 1.1.1.1, Google 8.8.8.8, Quad9 9.9.9.9).

  _Reach for this immediately after `dns apply` to confirm the record is visible globally before kicking off downstream steps._

  ```bash
  cloudflare-pp-cli propagate watch example.com A --expect 203.0.113.10 --watch
  ```
- **`cache purge release`** — Purge cache by URL/tag/hostname and verify with cf-cache-status header probes (MISS then HIT).

  _Reach for this in deploy/release scripts where downstream steps depend on cached content actually being purged._

  ```bash
  cloudflare-pp-cli cache purge release --zone example.com --tags release-v1 --probe https://example.com/
  ```

### Cross-product
- **`where-is`** — Find every place a hostname appears across DNS records, Worker routes, and Page Rules in one command.

  _Reach for this before deleting or changing a domain — check that nothing else depends on it._

  ```bash
  cloudflare-pp-cli where-is example.com --json
  ```
- **`zones diff`** — Diff two zones across settings, page rules, and DNS records (semantic name+type match) to find drift.

  _Reach for this before promoting staging to prod, during incident review, or when onboarding a tenant zone from a template._

  ```bash
  cloudflare-pp-cli zones diff staging.example.com prod.example.com --json
  ```
- **`worker bindings show`** — Show every binding (KV, R2, D1, queue, secret, cron, route, custom domain) for one Worker in a single table.

  _Reach for this when debugging a Worker in production — see everything wired to it without 10 dashboard tabs._

  ```bash
  cloudflare-pp-cli worker bindings show my-worker --account <account_id> --json
  ```

### Composition
- **`setup_zone`** — End-to-end zone setup: A record, optional redirect, SSL strict, Always-Use-HTTPS — one command instead of four dashboard tabs.

  _Reach for this when wiring a new domain end-to-end — the primary intent for a freshly-deployed app._

  ```bash
  cloudflare-pp-cli setup_zone example.com --origin 203.0.113.10 --redirect-from "legacy.example.com/*" --json
  ```

## Command Reference

**accounts** — Manage accounts


**accounts d1** — Manage d1

- `cloudflare-pp-cli accounts d1 export-database` — Returns a URL where the SQL contents of your D1 can be downloaded. Note: this process may take some time fo...
- `cloudflare-pp-cli accounts d1 import-database` — Generates a temporary URL for uploading an SQL file to, then instructing the D1 to import it and polling it...
- `cloudflare-pp-cli accounts d1 list-databases` — Returns a list of D1 databases.
- `cloudflare-pp-cli accounts d1 query-database` — Returns the query result as an object.
- `cloudflare-pp-cli accounts d1 raw-database-query` — Returns the query result rows as arrays rather than objects. This is a performance-optimized version of the...
- `cloudflare-pp-cli accounts d1 time-travel-get-bookmark` — Retrieves the current bookmark, or the nearest bookmark at or before a provided timestamp. Bookmarks can be...
- `cloudflare-pp-cli accounts d1 time-travel-restore` — Restores a D1 database to a previous point in time either via a bookmark or a timestamp.
- `cloudflare-pp-cli accounts d1 update-partial-database` — Updates partially the specified D1 database.

**accounts iam** — Manage iam

- `cloudflare-pp-cli accounts iam account-permission-group-list` — List all the permissions groups for an account.
- `cloudflare-pp-cli accounts iam account-resource-group-details` — Get information about a specific resource group in an account.
- `cloudflare-pp-cli accounts iam account-resource-group-list` — List all the resource groups for an account.
- `cloudflare-pp-cli accounts iam account-user-group-create` — Create a new user group under the specified account.
- `cloudflare-pp-cli accounts iam account-user-group-delete` — Remove a user group from an account.
- `cloudflare-pp-cli accounts iam account-user-group-details` — Get information about a specific user group in an account.
- `cloudflare-pp-cli accounts iam account-user-group-list` — List all the user groups for an account.
- `cloudflare-pp-cli accounts iam account-user-group-member-create` — Add members to a User Group.
- `cloudflare-pp-cli accounts iam account-user-group-member-delete` — Remove a member from User Group
- `cloudflare-pp-cli accounts iam account-user-group-member-get` — Get details of a specific member in a user group.
- `cloudflare-pp-cli accounts iam account-user-group-member-list` — List all the members attached to a user group.
- `cloudflare-pp-cli accounts iam account-user-group-members-update` — Replace the set of members attached to a User Group.
- `cloudflare-pp-cli accounts iam account-user-group-update` — Modify an existing user group.

**accounts members** — Manage members

- `cloudflare-pp-cli accounts members account-list` — List all members of an account.

**accounts pages** — Manage pages

- `cloudflare-pp-cli accounts pages deployment-get-deployment-logs` — Fetch deployment logs for a project.
- `cloudflare-pp-cli accounts pages deployment-get-deployments` — Fetch a list of project deployments.
- `cloudflare-pp-cli accounts pages deployment-retry-deployment` — Retry a previous deployment.
- `cloudflare-pp-cli accounts pages deployment-rollback-deployment` — Rollback the production deployment to a previous deployment. You can only rollback to succesful builds on p...
- `cloudflare-pp-cli accounts pages domains-add-domain` — Add a new domain for the Pages project.
- `cloudflare-pp-cli accounts pages domains-delete-domain` — Delete a Pages project's domain.
- `cloudflare-pp-cli accounts pages domains-get-domain` — Fetch a single domain.
- `cloudflare-pp-cli accounts pages domains-get-domains` — Fetch a list of all domains associated with a Pages project.
- `cloudflare-pp-cli accounts pages project-create-project` — Create a new project.
- `cloudflare-pp-cli accounts pages project-delete-project` — Delete a project by name.
- `cloudflare-pp-cli accounts pages project-get-project` — Fetch a project by name.
- `cloudflare-pp-cli accounts pages project-get-projects` — Fetch a list of all user projects.
- `cloudflare-pp-cli accounts pages project-update-project` — Set new attributes for an existing project. Modify environment variables. To delete an environment variable...
- `cloudflare-pp-cli accounts pages purge-build-cache` — Purge all cached build artifacts for a Pages project

**accounts r2** — Manage r2

- `cloudflare-pp-cli accounts r2 create-bucket` — Creates a new R2 bucket.
- `cloudflare-pp-cli accounts r2 create-temp-access-credentials` — Creates temporary access credentials on a bucket that can be optionally scoped to prefixes or objects.
- `cloudflare-pp-cli accounts r2 delete-bucket-cors-policy` — Delete the CORS policy for a bucket.
- `cloudflare-pp-cli accounts r2 delete-bucket-sippy-config` — Disables Sippy on this bucket.
- `cloudflare-pp-cli accounts r2 delete-custom-domain` — Remove custom domain registration from an existing R2 bucket.
- `cloudflare-pp-cli accounts r2 delete-object` — Deletes an object from an R2 bucket. For most workloads, we recommend using R2's [S3-compatible...
- `cloudflare-pp-cli accounts r2 delete-objects` — Deletes multiple objects from an R2 bucket. Two modes are supported: 1. **Delete by list** (default): Provi...
- `cloudflare-pp-cli accounts r2 get-bucket` — Gets properties of an existing R2 bucket.
- `cloudflare-pp-cli accounts r2 get-bucket-cors-policy` — Get the CORS policy for a bucket.
- `cloudflare-pp-cli accounts r2 get-bucket-lifecycle-configuration` — Get object lifecycle rules for a bucket.
- `cloudflare-pp-cli accounts r2 get-bucket-local-uploads-configuration` — Get the local uploads configuration for a bucket. When enabled, object's data is written to the nearest reg...
- `cloudflare-pp-cli accounts r2 get-bucket-lock-configuration` — Get lock rules for a bucket.
- `cloudflare-pp-cli accounts r2 get-bucket-public-policy` — Gets state of public access over the bucket's R2-managed (r2.dev) domain.
- `cloudflare-pp-cli accounts r2 get-bucket-sippy-config` — Gets configuration for Sippy for an existing R2 bucket.
- `cloudflare-pp-cli accounts r2 get-custom-domain-settings` — Get the configuration for a custom domain on an existing R2 bucket.
- `cloudflare-pp-cli accounts r2 get-object` — Retrieves an object from an R2 bucket. Returns the object body along with metadata headers. For most worklo...
- `cloudflare-pp-cli accounts r2 list-buckets` — Lists all R2 buckets on your account.
- `cloudflare-pp-cli accounts r2 list-custom-domains` — Gets a list of all custom domains registered with an existing R2 bucket.
- `cloudflare-pp-cli accounts r2 list-objects` — Lists objects in an R2 bucket. Returns object metadata including key, size, etag, last modified date, HTTP ...
- `cloudflare-pp-cli accounts r2 patch-bucket` — Updates properties of an existing R2 bucket.
- `cloudflare-pp-cli accounts r2 put-bucket-cors-policy` — Set the CORS policy for a bucket.
- `cloudflare-pp-cli accounts r2 put-bucket-lifecycle-configuration` — Set the object lifecycle rules for a bucket.
- `cloudflare-pp-cli accounts r2 put-bucket-local-uploads-configuration` — Set the local uploads configuration for a bucket. When enabled, object's data is written to the nearest reg...
- `cloudflare-pp-cli accounts r2 put-bucket-lock-configuration` — Set lock rules for a bucket.
- `cloudflare-pp-cli accounts r2 put-bucket-public-policy` — Updates state of public access over the bucket's R2-managed (r2.dev) domain.
- `cloudflare-pp-cli accounts r2 put-bucket-sippy-config` — Sets configuration for Sippy for an existing R2 bucket.
- `cloudflare-pp-cli accounts r2 put-object` — Uploads an object to an R2 bucket. The object body is provided as the request body. Returns metadata about ...

**accounts r2-catalog** — Manage r2 catalog

- `cloudflare-pp-cli accounts r2-catalog enable-catalog` — Enable an R2 bucket as an Apache Iceberg catalog. This operation creates the necessary catalog infrastructu...
- `cloudflare-pp-cli accounts r2-catalog get-maintenance-config` — Retrieve the maintenance configuration for a specific catalog, including compaction settings and credential...
- `cloudflare-pp-cli accounts r2-catalog get-table-maintenance-config` — Retrieve the maintenance configuration for a specific table, including compaction settings.
- `cloudflare-pp-cli accounts r2-catalog list-catalogs` — Returns a list of R2 buckets that have been enabled as Apache Iceberg catalogs for the specified account. E...
- `cloudflare-pp-cli accounts r2-catalog list-namespaces` — Returns a list of namespaces in the specified R2 catalog. Supports hierarchical filtering and pagination fo...
- `cloudflare-pp-cli accounts r2-catalog list-tables` — Returns a list of tables in the specified namespace within an R2 catalog. Supports pagination for efficient...
- `cloudflare-pp-cli accounts r2-catalog store-credentials` — Store authentication credentials for a catalog. These credentials are used to authenticate with R2 storage ...
- `cloudflare-pp-cli accounts r2-catalog update-maintenance-config` — Update the maintenance configuration for a catalog. This allows you to enable or disable compaction and adj...
- `cloudflare-pp-cli accounts r2-catalog update-table-maintenance-config` — Update the maintenance configuration for a specific table. This allows you to enable or disable compaction ...

**accounts registrar** — Manage registrar

- `cloudflare-pp-cli accounts registrar domain-registration-create` — Starts a domain registration workflow. This is a billable operation — successful registration charges the...
- `cloudflare-pp-cli accounts registrar domain-registration-get` — Returns the current state of a domain registration. This is the canonical read endpoint for a domain you ow...
- `cloudflare-pp-cli accounts registrar domain-registration-get-status` — Returns the current status of a domain registration workflow. Use this endpoint to poll for completion when...
- `cloudflare-pp-cli accounts registrar domain-registration-get-update-status` — Returns the current status of a domain update workflow. Use this endpoint to poll for completion when the P...
- `cloudflare-pp-cli accounts registrar domain-registration-list` — Returns a paginated list of domain registrations owned by the account. This endpoint uses cursor-based pagi...
- `cloudflare-pp-cli accounts registrar domains-get-domain` — Show individual domain.
- `cloudflare-pp-cli accounts registrar domains-list-domains` — List domains handled by Registrar.
- `cloudflare-pp-cli accounts registrar domains-update-domain` — Update individual domain.

**accounts rulesets** — Manage rulesets

- `cloudflare-pp-cli accounts rulesets create-account-rule` — Adds a new rule to an account ruleset. The rule will be added to the end of the existing list of rules in t...
- `cloudflare-pp-cli accounts rulesets delete-account-rule` — Deletes an existing rule from an account ruleset.
- `cloudflare-pp-cli accounts rulesets delete-account-version` — Deletes an existing version of an account ruleset.
- `cloudflare-pp-cli accounts rulesets get-account-entrypoint` — Fetches the latest version of the account entry point ruleset for a given phase.
- `cloudflare-pp-cli accounts rulesets get-account-entrypoint-version` — Fetches a specific version of an account entry point ruleset.
- `cloudflare-pp-cli accounts rulesets get-account-version` — Fetches a specific version of an account ruleset.
- `cloudflare-pp-cli accounts rulesets list-account` — Fetches all rulesets at the account level.
- `cloudflare-pp-cli accounts rulesets list-account-entrypoint-versions` — Fetches the versions of an account entry point ruleset.
- `cloudflare-pp-cli accounts rulesets list-account-version-rules-by-tag` — Fetches the rules of a managed account ruleset version for a given tag.
- `cloudflare-pp-cli accounts rulesets list-account-versions` — Fetches the versions of an account ruleset.
- `cloudflare-pp-cli accounts rulesets update-account-entrypoint` — Updates an account entry point ruleset, creating a new version.
- `cloudflare-pp-cli accounts rulesets update-account-rule` — Updates an existing rule in an account ruleset.

**accounts storage** — Manage storage

- `cloudflare-pp-cli accounts storage workers-kv-namespace-delete-multiple-key-value-pairs` — Remove multiple KV pairs from the namespace. Body should be an array of up to 10,000 keys to be removed.
- `cloudflare-pp-cli accounts storage workers-kv-namespace-delete-multiple-key-value-pairs-deprecated` — Remove multiple KV pairs from the namespace. Body should be an array of up to 10,000 keys to be removed.
- `cloudflare-pp-cli accounts storage workers-kv-namespace-get-multiple-key-value-pairs` — Retrieve up to 100 KV pairs from the namespace. Keys must contain text-based values. JSON values can option...
- `cloudflare-pp-cli accounts storage workers-kv-namespace-list-a-namespace-s-keys` — Lists a namespace's keys.
- `cloudflare-pp-cli accounts storage workers-kv-namespace-list-namespaces` — Returns the namespaces owned by an account.
- `cloudflare-pp-cli accounts storage workers-kv-namespace-read-key-value-pair` — Returns the value associated with the given key in the given namespace. Use URL-encoding to use special cha...
- `cloudflare-pp-cli accounts storage workers-kv-namespace-read-the-metadata-for-a-key` — Returns the metadata associated with the given key in the given namespace. Use URL-encoding to use special...
- `cloudflare-pp-cli accounts storage workers-kv-namespace-remove-a-namespace` — Deletes the namespace corresponding to the given ID.
- `cloudflare-pp-cli accounts storage workers-kv-namespace-write-key-value-pair-with-metadata` — Write a value identified by a key. Use URL-encoding to use special characters (for example, `:`, `!`, `%`) ...
- `cloudflare-pp-cli accounts storage workers-kv-namespace-write-multiple-key-value-pairs` — Write multiple keys and values at once. Body should be an array of up to 10,000 key-value pairs to be store...

**accounts workers** — Manage workers

- `cloudflare-pp-cli accounts workers account-settings-create-account-settings` — Creates Worker account settings for an account.
- `cloudflare-pp-cli accounts workers account-settings-fetch-account-settings` — Fetches Worker account settings for an account.
- `cloudflare-pp-cli accounts workers assets-upload` — Upload assets ahead of creating a Worker version. To learn more about the direct uploads of assets, see...
- `cloudflare-pp-cli accounts workers create` — Create a new Worker.
- `cloudflare-pp-cli accounts workers create-version` — Create a new version.
- `cloudflare-pp-cli accounts workers cron-trigger-get-cron-triggers` — Fetches Cron Triggers for a Worker.
- `cloudflare-pp-cli accounts workers cron-trigger-update-cron-triggers` — Updates Cron Triggers for a Worker.
- `cloudflare-pp-cli accounts workers delete` — Delete a Worker and all its associated resources (versions, deployments, etc.).
- `cloudflare-pp-cli accounts workers delete-script-secret` — Remove a secret from a script.
- `cloudflare-pp-cli accounts workers delete-version` — Delete a version.
- `cloudflare-pp-cli accounts workers deployments-create-deployment` — Deployments configure how [Worker Versions](https://developers.cloudflare.com/api/operations/worker-version...
- `cloudflare-pp-cli accounts workers deployments-delete-deployment` — Delete a Worker Deployment. The latest deployment, which is actively serving traffic, cannot be deleted. Al...
- `cloudflare-pp-cli accounts workers deployments-get-deployment` — Get information about a Worker Deployment.
- `cloudflare-pp-cli accounts workers deployments-list-deployments` — List of Worker Deployments. The first deployment in the list is the latest deployment actively serving traf...
- `cloudflare-pp-cli accounts workers destination-create` — Create a new Workers Observability Telemetry Destination.
- `cloudflare-pp-cli accounts workers destination-list` — List your Workers Observability Telemetry Destinations.
- `cloudflare-pp-cli accounts workers destination-update` — Update an existing Workers Observability Telemetry Destination.
- `cloudflare-pp-cli accounts workers destinations-delete` — Delete a Workers Observability Telemetry Destination.
- `cloudflare-pp-cli accounts workers domains-delete` — Detaches a domain from a Worker. Both the Worker and all of its previews are no longer routable using this ...
- `cloudflare-pp-cli accounts workers domains-get` — Gets information about a domain.
- `cloudflare-pp-cli accounts workers domains-list` — Lists all domains for an account.
- `cloudflare-pp-cli accounts workers domains-update` — Attaches a domain that routes traffic to a Worker.
- `cloudflare-pp-cli accounts workers durable-objects-namespace-list-namespaces` — Returns the Durable Object namespaces owned by an account.
- `cloudflare-pp-cli accounts workers durable-objects-namespace-list-objects` — Returns the Durable Objects in a given namespace.
- `cloudflare-pp-cli accounts workers edit` — Perform a partial update on a Worker, where omitted properties are left unchanged from their current values.
- `cloudflare-pp-cli accounts workers environment-get-script-content` — Get script content from a worker with an environment.
- `cloudflare-pp-cli accounts workers environment-put-script-content` — Put script content from a worker with an environment.
- `cloudflare-pp-cli accounts workers get` — Get details about a specific Worker.
- `cloudflare-pp-cli accounts workers get-accounts` — Get list of tails currently deployed on a Worker.
- `cloudflare-pp-cli accounts workers get-script-secret` — Get a given secret binding (value omitted) on a script.
- `cloudflare-pp-cli accounts workers get-version` — Get details about a specific version.
- `cloudflare-pp-cli accounts workers list` — List all Workers for an account.
- `cloudflare-pp-cli accounts workers list-script-secrets` — List secrets bound to a script.
- `cloudflare-pp-cli accounts workers list-versions` — List all versions for a Worker.
- `cloudflare-pp-cli accounts workers namespace-create` — Create a new Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-delete-namespace` — Delete a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-delete-script-secret` — Remove a secret from a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-delete-script-tag` — Delete script tag for a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-delete-scripts` — Delete multiple scripts from a Workers for Platforms namespace based on optional tag filters.
- `cloudflare-pp-cli accounts workers namespace-get-namespace` — Get a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-get-script-bindings` — Fetch script bindings from a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-get-script-content` — Fetch script content from a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-get-script-secrets` — Get a given secret binding (value omitted) on a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-get-script-settings` — Get script settings from a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-get-script-tags` — Fetch tags from a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-list` — Fetch a list of Workers for Platforms namespaces.
- `cloudflare-pp-cli accounts workers namespace-list-script-secrets` — List secrets bound to a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-list-scripts` — Fetch a list of scripts uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-patch-namespace` — Patch a Workers for Platforms namespace. Omitted fields are left unchanged.
- `cloudflare-pp-cli accounts workers namespace-patch-script-secrets-bulk` — Create, update, or delete multiple secrets on a script in a single operation using JSON Merge Patch (RFC 73...
- `cloudflare-pp-cli accounts workers namespace-patch-script-settings` — Patch script metadata, such as bindings.
- `cloudflare-pp-cli accounts workers namespace-put-namespace` — Update a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-put-script-content` — Put script content for a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-put-script-secrets` — Add a secret to a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-put-script-tag` — Put a single tag on a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-put-script-tags` — Put script tags for a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-script-delete` — Delete a worker from a Workers for Platforms namespace. This call has no response body on a successful delete.
- `cloudflare-pp-cli accounts workers namespace-script-details` — Fetch information about a script uploaded to a Workers for Platforms namespace.
- `cloudflare-pp-cli accounts workers namespace-script-update-create-assets-upload-session` — Start uploading a collection of assets for use in a Worker version. To learn more about the direct uploads ...
- `cloudflare-pp-cli accounts workers namespace-script-upload-module` — Upload a worker module to a Workers for Platforms namespace. You can find more about the multipart metadata...
- `cloudflare-pp-cli accounts workers patch-script-secrets-bulk` — Create, update, or delete multiple secrets on a script in a single operation using JSON Merge Patch (RFC 73...
- `cloudflare-pp-cli accounts workers placement-list-regions` — Returns a list of available placement regions organized by cloud provider. These regions can be used to con...
- `cloudflare-pp-cli accounts workers put-script-secret` — Add a secret to a script.
- `cloudflare-pp-cli accounts workers queries-delete` — Delete a saved query.
- `cloudflare-pp-cli accounts workers queries-get` — Retrieve a saved query.
- `cloudflare-pp-cli accounts workers queries-list` — List saved queries.
- `cloudflare-pp-cli accounts workers queries-patch` — Update saved query.
- `cloudflare-pp-cli accounts workers queries-post` — Persist query for later use.
- `cloudflare-pp-cli accounts workers script-delete` — Delete your worker. This call has no response body on a successful delete.
- `cloudflare-pp-cli accounts workers script-delete-subdomain` — Disable all workers.dev subdomains for a Worker.
- `cloudflare-pp-cli accounts workers script-download` — Fetch raw script content for your worker. Note this is the original script content, not JSON encoded.
- `cloudflare-pp-cli accounts workers script-environment-get-settings` — Get script settings from a worker with an environment.
- `cloudflare-pp-cli accounts workers script-environment-patch-settings` — Patch script metadata, such as bindings.
- `cloudflare-pp-cli accounts workers script-fetch-usage-model` — Fetches the Usage Model for a given Worker.
- `cloudflare-pp-cli accounts workers script-get-content` — Fetch script content only.
- `cloudflare-pp-cli accounts workers script-get-settings` — Get metadata and config, such as bindings or usage model.
- `cloudflare-pp-cli accounts workers script-get-subdomain` — Get if the Worker is available on the workers.dev subdomain.
- `cloudflare-pp-cli accounts workers script-list` — Fetch a list of uploaded workers.
- `cloudflare-pp-cli accounts workers script-patch-settings` — Patch metadata or config, such as bindings or usage model.
- `cloudflare-pp-cli accounts workers script-post-subdomain` — Enable or disable the Worker on the workers.dev subdomain.
- `cloudflare-pp-cli accounts workers script-put-content` — Put script content without touching config or metadata.
- `cloudflare-pp-cli accounts workers script-search` — Search for Workers in an account.
- `cloudflare-pp-cli accounts workers script-settings-get-settings` — Get script-level settings when using [Worker Versions](https://developers.cloudflare.com/api/operations/wor...
- `cloudflare-pp-cli accounts workers script-settings-patch-settings` — Patch script-level settings when using [Worker Versions](https://developers.cloudflare.com/api/operations/w...
- `cloudflare-pp-cli accounts workers script-update-create-assets-upload-session` — Start uploading a collection of assets for use in a Worker version. To learn more about the direct uploads ...
- `cloudflare-pp-cli accounts workers script-update-usage-model` — Updates the Usage Model for a given Worker. Requires a Workers Paid subscription.
- `cloudflare-pp-cli accounts workers script-upload-module` — Upload a worker module. You can find more about the multipart metadata on our docs:...
- `cloudflare-pp-cli accounts workers subdomain-create-subdomain` — Creates a Workers subdomain for an account.
- `cloudflare-pp-cli accounts workers subdomain-delete-subdomain` — Deletes a Workers subdomain for an account.
- `cloudflare-pp-cli accounts workers subdomain-get-subdomain` — Returns a Workers subdomain for an account.
- `cloudflare-pp-cli accounts workers tail-logs-delete-tail` — Deletes a tail from a Worker.
- `cloudflare-pp-cli accounts workers tail-logs-start-tail` — Starts a tail that receives logs and exception from a Worker.
- `cloudflare-pp-cli accounts workers telemetry-keys-list` — List all the keys in your telemetry events.
- `cloudflare-pp-cli accounts workers telemetry-query` — Run a temporary or saved query.
- `cloudflare-pp-cli accounts workers telemetry-values-list` — List unique values found in your events.
- `cloudflare-pp-cli accounts workers update` — Perform a complete replacement of a Worker, where omitted properties are set to their default values. This ...
- `cloudflare-pp-cli accounts workers versions-get-version-detail` — Retrieves detailed information about a specific version of a Workers script.
- `cloudflare-pp-cli accounts workers versions-list-versions` — List of Worker Versions. The first version in the list is the latest version.
- `cloudflare-pp-cli accounts workers versions-upload-version` — Upload a Worker Version without deploying to Cloudflare's network. You can find more about the multipart me...

**certificates** — Manage certificates


**memberships** — Manage memberships


**organizations** — Manage organizations


**promoted internal** — Manage promoted internal


**promoted ips** — Manage promoted ips


**promoted live** — Manage promoted live


**promoted ready** — Manage promoted ready


**promoted signed-url** — Manage promoted signed-url


**promoted tenants** — Manage promoted tenants


**promoted users** — Manage promoted users


**promoted workers** — Manage promoted workers


**system secrets-store-delete-by-id** — Manage system secrets-store-delete-by-id


**system secrets-store-duplicate-by-id** — Manage system secrets-store-duplicate-by-id


**system secrets-store-get-store-by-id** — Manage system secrets-store-get-store-by-id


**system secrets-store-list** — Manage system secrets-store-list


**system secrets-store-secret-create** — Manage system secrets-store-secret-create


**system secrets-store-secret-delete-by-id** — Manage system secrets-store-secret-delete-by-id


**system secrets-store-secrets-list** — Manage system secrets-store-secrets-list


**tenants account-types** — Manage account types


**tenants accounts** — Manage accounts


**tenants entitlements** — Manage entitlements


**tenants memberships** — Manage memberships


**user** — Manage user


**user api-tokens-update-token** — Manage user api-tokens-update-token


**user api-tokens-verify-token** — Manage user api-tokens-verify-token


**user details** — Manage user details


**user edit** — Manage user edit


**user ip-access-rules-for-a-create-an-ip-access-rule** — Manage user ip-access-rules-for-a-create-an-ip-access-rule


**user ip-access-rules-for-a-delete-an-ip-access-rule** — Manage user ip-access-rules-for-a-delete-an-ip-access-rule


**user ip-access-rules-for-a-list-ip-access-rules** — Manage user ip-access-rules-for-a-list-ip-access-rules


**user ip-access-rules-for-a-update-an-ip-access-rule** — Manage user ip-access-rules-for-a-update-an-ip-access-rule


**zones** — Manage zones


**zones activation-check** — Manage activation check


**zones available-plans** — Manage available plans

- `cloudflare-pp-cli zones available-plans zone-rate-plan-list` — Lists available plans the zone can subscribe to.

**zones cache** — Manage cache

- `cloudflare-pp-cli zones cache origin-cloud-regions-delete` — Removes the cloud region mapping for a single origin IP address. The IP path parameter is normalized before...
- `cloudflare-pp-cli zones cache origin-cloud-regions-list` — Returns all IP-to-cloud-region mappings configured for the zone. Each mapping tells Cloudflare which cloud ...
- `cloudflare-pp-cli zones cache origin-cloud-regions-supported-regions` — Returns the cloud vendors and regions that are valid values for origin cloud region mappings. Each region i...
- `cloudflare-pp-cli zones cache origin-cloud-regions-upsert` — Adds or updates a single IP-to-cloud-region mapping for the zone. Unlike POST, this operation is idempotent...
- `cloudflare-pp-cli zones cache smart-tiered-create-smart-tiered-setting` — Smart Tiered Cache dynamically selects the single closest upper tier for each of your website's origins wit...
- `cloudflare-pp-cli zones cache smart-tiered-delete-smart-tiered-setting` — Smart Tiered Cache dynamically selects the single closest upper tier for each of your website’s origins wit...
- `cloudflare-pp-cli zones cache smart-tiered-get-smart-tiered-setting` — Smart Tiered Cache dynamically selects the single closest upper tier for each of your website’s origins wit...
- `cloudflare-pp-cli zones cache smart-tiered-patch-smart-tiered-setting` — Smart Tiered Cache dynamically selects the single closest upper tier for each of your website’s origins wit...
- `cloudflare-pp-cli zones cache zone-settings-change-origin-post-quantum-encryption-setting` — Instructs Cloudflare to use Post-Quantum (PQ) key agreement algorithms when connecting to your origin. Pref...
- `cloudflare-pp-cli zones cache zone-settings-change-regional-tiered-setting` — Instructs Cloudflare to check a regional hub data center on the way to your upper tier. This can help impro...
- `cloudflare-pp-cli zones cache zone-settings-change-reserve-setting` — Increase cache lifetimes by automatically storing all cacheable files into Cloudflare's persistent object s...
- `cloudflare-pp-cli zones cache zone-settings-change-variants-setting` — Variant support enables caching variants of images with certain file extensions in addition to the original...
- `cloudflare-pp-cli zones cache zone-settings-delete-variants-setting` — Variant support enables caching variants of images with certain file extensions in addition to the original...
- `cloudflare-pp-cli zones cache zone-settings-get-origin-post-quantum-encryption-setting` — Instructs Cloudflare to use Post-Quantum (PQ) key agreement algorithms when connecting to your origin. Pref...
- `cloudflare-pp-cli zones cache zone-settings-get-regional-tiered-setting` — Instructs Cloudflare to check a regional hub data center on the way to your upper tier. This can help impro...
- `cloudflare-pp-cli zones cache zone-settings-get-reserve-clear` — You can use Cache Reserve Clear to clear your Cache Reserve, but you must first disable Cache Reserve. In m...
- `cloudflare-pp-cli zones cache zone-settings-get-reserve-setting` — Increase cache lifetimes by automatically storing all cacheable files into Cloudflare's persistent object s...
- `cloudflare-pp-cli zones cache zone-settings-get-variants-setting` — Variant support enables caching variants of images with certain file extensions in addition to the original...
- `cloudflare-pp-cli zones cache zone-settings-start-reserve-clear` — You can use Cache Reserve Clear to clear your Cache Reserve, but you must first disable Cache Reserve. In m...

**zones certificate-authorities** — Manage certificate authorities


**zones dns-records** — Manage dns records

- `cloudflare-pp-cli zones dns-records for-a-zone-batch` — Send a Batch of DNS Record API calls to be executed together. Notes: - Although Cloudflare will execute the...
- `cloudflare-pp-cli zones dns-records for-a-zone-create` — Create a new DNS record for a zone. Notes: - A/AAAA records cannot exist on the same name as CNAME records....
- `cloudflare-pp-cli zones dns-records for-a-zone-export` — You can export your [BIND config](https://en.wikipedia.org/wiki/Zone_file 'Zone file') through this endpoin...
- `cloudflare-pp-cli zones dns-records for-a-zone-get-usage` — Get the current DNS record usage for a zone, including the number of records and the quota limit.
- `cloudflare-pp-cli zones dns-records for-a-zone-import` — You can upload your [BIND config](https://en.wikipedia.org/wiki/Zone_file 'Zone file') through this endpoin...
- `cloudflare-pp-cli zones dns-records for-a-zone-list` — List, search, sort, and filter a zones' DNS records.
- `cloudflare-pp-cli zones dns-records for-a-zone-review-dns-scan` — Retrieves the list of DNS records discovered up to this point by the asynchronous scan. These records are t...
- `cloudflare-pp-cli zones dns-records for-a-zone-scan` — Scan for common DNS records on your domain and automatically add them to your zone. Useful if you haven't u...
- `cloudflare-pp-cli zones dns-records for-a-zone-trigger-dns-scan` — Initiates an asynchronous scan for common DNS records on your domain. Note that this **does not** automatic...
- `cloudflare-pp-cli zones dns-records for-a-zone-update` — Overwrite an existing DNS record. Notes: - A/AAAA records cannot exist on the same name as CNAME records. -...

**zones dnssec** — Manage dnssec

- `cloudflare-pp-cli zones dnssec list-zsks` — List the Zone Signing Keys (ZSKs) that DNSSEC uses for the zone.

**zones origin** — Manage origin

- `cloudflare-pp-cli zones origin cloud-regions-v2-delete` — Removes the cloud region mapping for a single origin IP address. The IP path parameter is normalized before...
- `cloudflare-pp-cli zones origin cloud-regions-v2-list` — Returns all IP-to-cloud-region mappings configured for the zone with pagination support. Each mapping tells...
- `cloudflare-pp-cli zones origin cloud-regions-v2-supported-regions` — Returns the cloud vendors and regions that are valid values for origin cloud region mappings. Each region i...
- `cloudflare-pp-cli zones origin cloud-regions-v2-upsert` — Creates a new IP-to-cloud-region mapping or replaces the existing mapping for the specified IP. PUT is idem...

**zones origin-tls-client-auth** — Manage origin tls client auth

- `cloudflare-pp-cli zones origin-tls-client-auth per-hostname-authenticated-origin-pull-get-the-hostname-status-for-client-authentication` — Retrieves the client certificate authentication status for a specific hostname, showing whether authenticat...
- `cloudflare-pp-cli zones origin-tls-client-auth per-hostname-authenticated-origin-pull-list-certificates` — Lists all client certificates configured for per-hostname authenticated origin pulls on the zone.
- `cloudflare-pp-cli zones origin-tls-client-auth per-hostname-authenticated-origin-pull-list-hostname-associations` — List certificate ID - hostname associations for the given zone. Shows which hostnames are associated to whi...
- `cloudflare-pp-cli zones origin-tls-client-auth zone-level-authenticated-origin-pulls-delete-certificate` — Removes a client certificate used for zone-level authenticated origin pulls.
- `cloudflare-pp-cli zones origin-tls-client-auth zone-level-authenticated-origin-pulls-get-certificate-details` — Retrieves details for a specific client certificate used in zone-level authenticated origin pulls.
- `cloudflare-pp-cli zones origin-tls-client-auth zone-level-authenticated-origin-pulls-get-enablement-setting-for-zone` — Get whether zone-level authenticated origin pulls is enabled or not. It is false by default.
- `cloudflare-pp-cli zones origin-tls-client-auth zone-level-authenticated-origin-pulls-list-certificates` — Lists all client certificates configured for zone-level authenticated origin pulls.
- `cloudflare-pp-cli zones origin-tls-client-auth zone-level-authenticated-origin-pulls-set-enablement-for-zone` — Enable or disable zone-level authenticated origin pulls. 'enabled' should be set true either before/after t...
- `cloudflare-pp-cli zones origin-tls-client-auth zone-level-authenticated-origin-pulls-upload-certificate` — Upload your own certificate you want Cloudflare to use for edge-to-origin communication to override the sha...

**zones pagerules** — Manage pagerules

- `cloudflare-pp-cli zones pagerules page-rules-get-a-page-rule` — Fetches the details of a Page Rule.
- `cloudflare-pp-cli zones pagerules page-rules-list-page-rules` — Fetches Page Rules in a zone.
- `cloudflare-pp-cli zones pagerules page-rules-update-a-page-rule` — Replaces the configuration of an existing Page Rule. The configuration of the updated Page Rule will exactl...

**zones purge-cache** — Manage purge cache


**zones rulesets** — Manage rulesets

- `cloudflare-pp-cli zones rulesets create-zone-rule` — Adds a new rule to a zone ruleset. The rule will be added to the end of the existing list of rules in the r...
- `cloudflare-pp-cli zones rulesets delete-zone-rule` — Deletes an existing rule from a zone ruleset.
- `cloudflare-pp-cli zones rulesets delete-zone-version` — Deletes an existing version of a zone ruleset.
- `cloudflare-pp-cli zones rulesets get-zone-entrypoint` — Fetches the latest version of the zone entry point ruleset for a given phase.
- `cloudflare-pp-cli zones rulesets get-zone-entrypoint-version` — Fetches a specific version of a zone entry point ruleset.
- `cloudflare-pp-cli zones rulesets get-zone-version` — Fetches a specific version of a zone ruleset.
- `cloudflare-pp-cli zones rulesets list-zone` — Fetches all rulesets at the zone level.
- `cloudflare-pp-cli zones rulesets list-zone-entrypoint-versions` — Fetches the versions of a zone entry point ruleset.
- `cloudflare-pp-cli zones rulesets list-zone-version-rules-by-tag` — Fetches the rules of a managed zone ruleset version for a given tag.
- `cloudflare-pp-cli zones rulesets list-zone-versions` — Fetches the versions of a zone ruleset.
- `cloudflare-pp-cli zones rulesets update-zone-entrypoint` — Updates a zone entry point ruleset, creating a new version.
- `cloudflare-pp-cli zones rulesets update-zone-rule` — Updates an existing rule in a zone ruleset.

**zones settings** — Manage settings

- `cloudflare-pp-cli zones settings get-zones-zone-identifier-zaraz-config-history` — Gets a history of published Zaraz configurations by ID(s) for a zone.
- `cloudflare-pp-cli zones settings get-zones-zone-identifier-zaraz-default` — Gets default Zaraz configuration for a zone.
- `cloudflare-pp-cli zones settings get-zones-zone-identifier-zaraz-export` — Exports full current published Zaraz configuration for a zone, secret variables included.
- `cloudflare-pp-cli zones settings get-zones-zone-identifier-zaraz-history` — Lists a history of published Zaraz configuration records for a zone.
- `cloudflare-pp-cli zones settings get-zones-zone-identifier-zaraz-workflow` — Gets Zaraz workflow for a zone.
- `cloudflare-pp-cli zones settings put-zones-zone-identifier-zaraz-history` — Restores a historical published Zaraz configuration by ID for a zone.
- `cloudflare-pp-cli zones settings put-zones-zone-identifier-zaraz-workflow` — Updates Zaraz workflow for a zone.
- `cloudflare-pp-cli zones settings ssl-detector-automatic-mode-get-enrollment` — If the system is enabled, the response will include next_scheduled_scan, representing the next time this zo...
- `cloudflare-pp-cli zones settings ssl-detector-automatic-mode-patch-enrollment` — The automatic system is enabled when this endpoint is hit with value in the request body is set to 'auto', ...
- `cloudflare-pp-cli zones settings web-analytics-get-rum-status` — Retrieves RUM status for a zone.
- `cloudflare-pp-cli zones settings web-analytics-toggle-rum` — Toggles RUM on/off for an existing zone.
- `cloudflare-pp-cli zones settings zone-cache-change-aegis` — Aegis provides dedicated egress IPs (from Cloudflare to your origin) for your layer 7 WAF and CDN services....
- `cloudflare-pp-cli zones settings zone-cache-change-origin-h2-max-streams` — Origin H2 Max Streams configures the max number of concurrent requests that Cloudflare will send within the...
- `cloudflare-pp-cli zones settings zone-cache-change-origin-max-http-version` — Origin Max HTTP Setting Version sets the highest HTTP version Cloudflare will attempt to use with your orig...
- `cloudflare-pp-cli zones settings zone-cache-get-aegis` — Aegis provides dedicated egress IPs (from Cloudflare to your origin) for your layer 7 WAF and CDN services....
- `cloudflare-pp-cli zones settings zone-cache-get-origin-h2-max-streams` — Origin H2 Max Streams configures the max number of concurrent requests that Cloudflare will send within the...
- `cloudflare-pp-cli zones settings zone-cache-get-origin-max-http-version` — Origin Max HTTP Setting Version sets the highest HTTP version Cloudflare will attempt to use with your orig...
- `cloudflare-pp-cli zones settings zone-change-fonts` — Enhance your website's font delivery with Cloudflare Fonts. Deliver Google Hosted fonts from your own domai...
- `cloudflare-pp-cli zones settings zone-change-google-tag-gateway-config` — Updates the Google Tag Gateway configuration for a zone.
- `cloudflare-pp-cli zones settings zone-change-speed-brain` — Speed Brain lets compatible browsers speculate on content which can be prefetched or preloaded, making webs...
- `cloudflare-pp-cli zones settings zone-edit-single` — Updates a single zone setting by the identifier
- `cloudflare-pp-cli zones settings zone-edit-zone-info` — Edit multiple zone settings
- `cloudflare-pp-cli zones settings zone-get-all-zone` — Available settings for your user in relation to a zone.
- `cloudflare-pp-cli zones settings zone-get-fonts` — Enhance your website's font delivery with Cloudflare Fonts. Deliver Google Hosted fonts from your own domai...
- `cloudflare-pp-cli zones settings zone-get-google-tag-gateway-config` — Gets the Google Tag Gateway configuration for a zone.
- `cloudflare-pp-cli zones settings zone-get-single` — Fetch a single zone setting by name
- `cloudflare-pp-cli zones settings zone-get-speed-brain` — Speed Brain lets compatible browsers speculate on content which can be prefetched or preloaded, making webs...

**zones ssl** — Manage ssl

- `cloudflare-pp-cli zones ssl certificate-packs-get-certificate-pack-quotas` — For a given zone, list certificate pack quotas.
- `cloudflare-pp-cli zones ssl certificate-packs-list-certificate-packs` — For a given zone, list all active certificate packs.
- `cloudflare-pp-cli zones ssl certificate-packs-order-advanced-certificate-manager-certificate-pack` — For a given zone, order an advanced certificate pack.
- `cloudflare-pp-cli zones ssl tls-mode-recommendation-tls-recommendation` — Retrieve the SSL/TLS Recommender's recommendation for a zone.
- `cloudflare-pp-cli zones ssl universal-settings-for-a-zone-edit-universal-settings` — Patch Universal SSL Settings for a Zone.
- `cloudflare-pp-cli zones ssl universal-settings-for-a-zone-universal-settings-details` — Get Universal SSL Settings for a Zone.
- `cloudflare-pp-cli zones ssl verification-edit-certificate-pack-validation-method` — Edit SSL validation method for a certificate pack. A PATCH request will request an immediate validation che...
- `cloudflare-pp-cli zones ssl verification-verification-details` — Get SSL Verification Info for a Zone.

**zones workers** — Manage workers

- `cloudflare-pp-cli zones workers routes-list-routes` — Returns routes for a zone.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cloudflare-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### End-to-end zone setup

```bash
cloudflare-pp-cli setup_zone example.com --origin 203.0.113.10 --redirect-from "legacy.example.com/*" --json
```

Provisions A record, optional redirect Page Rule, SSL strict, and Always-Use-HTTPS in one shot.

### Audit zone drift before promoting staging to prod

```bash
cloudflare-pp-cli zones diff staging.example.com prod.example.com --json --select drift,settings,page_rules,dns
```

Returns settings/page-rule/DNS deltas with --select narrowing the response so agents see only the relevant nested fields.

### Find every place a domain is wired

```bash
cloudflare-pp-cli where-is example.com --json
```

Live API calls across DNS records, Worker routes, and Page Rules (no local cache required).

### Idempotent DNS provisioning from a script

```bash
cloudflare-pp-cli dns apply --zone example.com --type A --name @ --content 203.0.113.10 --dry-run
```

Shows what would change without applying. Drop --dry-run to commit.

### Cache purge with verification

```bash
cloudflare-pp-cli cache purge release --zone example.com --tags release-v1 --probe https://example.com/ --json
```

Purges by tag, then probes the URL and asserts cf-cache-status transitions from MISS to HIT.

## Auth Setup

Cloudflare uses scoped API tokens (Bearer) — preferred over the legacy email+global-key pair. Set `CLOUDFLARE_API_TOKEN` once; the CLI auto-resolves account_id and zone_id from names so most commands take human-readable inputs.

Run `cloudflare-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cloudflare-pp-cli accounts list --agent --select id,name,status
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
cloudflare-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cloudflare-pp-cli feedback --stdin < notes.txt
cloudflare-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.cloudflare-pp-cli/feedback.jsonl`. They are never POSTed unless `CLOUDFLARE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLOUDFLARE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
cloudflare-pp-cli profile save briefing --json
cloudflare-pp-cli --profile briefing accounts list
cloudflare-pp-cli profile list --json
cloudflare-pp-cli profile show briefing
cloudflare-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `cloudflare-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add cloudflare-pp-mcp -- cloudflare-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cloudflare-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cloudflare-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cloudflare-pp-cli <command> --help`.

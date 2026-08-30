---
name: pp-immich
description: "Agent-native control and careful personal-library rituals for a self-hosted Immich instance. Trigger phrases: `make a shared album from our beach weekend`, `find duplicate screenshots`, `show photos of me and Dad from past Julys`, `show my Immich memories`, `check my Immich server health`, `use Immich`."
author: "avanderheyde"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - immich-pp-cli
    install:
      - kind: go
        bins: [immich-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/immich/cmd/immich-pp-cli
---

# Immich — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `immich-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install immich --cli-only
   ```
2. Verify: `immich-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/immich/cmd/immich-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Covers Immich's official v3 API, the upload/import strengths of immich-go, and the broad raw operations of ImmichMCP, then adds safe personal workflows for events, duplicate cleanup, family time queries, memories, stacks, partners, and jobs.

## When to Use This CLI

Use this CLI for your own or a delegated self-hosted Immich library: finding and sharing event photos, safely dealing with duplicates, browsing people/time memories, and checking the health of the host.

## Anti-triggers

Do not use this CLI for:
- Do not use it against an Immich instance you are not authorized to access.
- Do not run duplicate resolution or asset deletion without reviewing the preview output.
- Do not expect it to replace a local backup of your original photos.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Shared events
- **`album event`** — Create or update a reviewable shared event album from explicit Immich search filters.

  _Use it when an agent needs to make a shareable event collection without manually copying asset IDs._

  ```bash
  immich-pp-cli album event beach-weekend --from 2025-07-01 --to 2025-07-07 --dry-run --agent
  ```

### Safe cleanup
- **`duplicates plan`** — Preview native duplicate groups and a deterministic keeper proposal without changing assets.

  _Use it before any destructive duplicate cleanup._

  ```bash
  immich-pp-cli duplicates plan --limit 20 --agent
  ```
- **`duplicates apply`** — Resolve selected native duplicate groups only after an explicit apply confirmation.

  _Use it only after reviewing a duplicate plan. Include that plan's exact `evidence` array for every selected group; when the server has no keeper recommendation, also include an explicit keeper. Apply rejects missing or changed evidence before choosing assets to trash._

  ```bash
  immich-pp-cli duplicates apply --groups '[{"group_id":"group-id","keeper":"asset-to-keep","evidence":["asset-to-keep","asset-to-trash"]}]' --apply --agent
  ```

### Family archive
- **`people july`** — Find one or two people across past Julys using real people and metadata search endpoints.

  _Use it for questions such as photos of me and Dad from past Julys._

  ```bash
  immich-pp-cli people july --person me --person dad --years 5 --agent
  ```
- **`memories review`** — List recent native memories with their dates and asset counts for an intentional review.

  _Use it to revisit what Immich surfaced without paging through the web UI._

  ```bash
  immich-pp-cli memories review --limit 12 --agent
  ```

### Library curation
- **`library review`** — Review bounded favorite and archived asset search results without mutating the library.

  _Use it to curate favorites and archived material deliberately._

  ```bash
  immich-pp-cli library review --mode favorites --limit 25 --agent
  ```
- **`stacks review`** — Inspect native stacks for empty, singleton, and unusually large groups without changing them.

  _Use it to understand burst and RAW/JPEG grouping before editing a stack._

  ```bash
  immich-pp-cli stacks review --limit 50 --agent
  ```

### Self-hosted maintenance
- **`library health`** — Report partner-sharing and worker-queue facts from the configured Immich server.

  _Use it when a self-hosted photo library feels stale or a shared library is not behaving as expected._

  ```bash
  immich-pp-cli library health --agent
  ```

## Command Reference

**activities** — An activity is a like or a comment made by a user on an asset or album.

- `immich-pp-cli activities create-activity` — Create a like or a comment for an album, or an asset in an album.
- `immich-pp-cli activities delete-activity` — Removes a like or comment from a given album or asset in an album.
- `immich-pp-cli activities get` — Returns a list of activities for the selected asset or album.
- `immich-pp-cli activities get-activity-statistics` — Returns the number of likes and comments for a given album or asset in an album.

**admin** — Manage admin

- `immich-pp-cli admin create-notification` — Create a new notification for a specific user.
- `immich-pp-cli admin create-user` — Create a new user.
- `immich-pp-cli admin delete-database-backup` — Delete a backup by its filename
- `immich-pp-cli admin delete-integrity-report` — Delete a given report item and perform corresponding deletion (e.g. trash asset, delete file)
- `immich-pp-cli admin delete-user` — Delete a user.
- `immich-pp-cli admin detect-prior-install` — Collect integrity checks and other heuristics about local data.
- `immich-pp-cli admin download-database-backup` — Downloads the database backup file
- `immich-pp-cli admin get-integrity-report` — Get all flagged items by integrity report type
- `immich-pp-cli admin get-integrity-report-csv` — Get all integrity report entries for a given type as a CSV
- `immich-pp-cli admin get-integrity-report-file` — Download the untracked/broken file if one exists
- `immich-pp-cli admin get-integrity-report-summary` — Get a count of the items flagged in each integrity report
- `immich-pp-cli admin get-maintenance-status` — Fetch information about the currently running maintenance action.
- `immich-pp-cli admin get-notification-template` — Retrieve a preview of the provided email template.
- `immich-pp-cli admin get-user` — Retrieve a specific user by their ID.
- `immich-pp-cli admin get-user-calendar-heatmap` — Retrieve activity counts for a specified period, in a calendar heatmap format.
- `immich-pp-cli admin get-user-preferences` — Retrieve the preferences of a specific user.
- `immich-pp-cli admin get-user-sessions` — Retrieve all sessions for a specific user.
- `immich-pp-cli admin get-user-statistics` — Retrieve asset statistics for a specific user.
- `immich-pp-cli admin list-database-backups` — Get the list of the successful and failed backups
- `immich-pp-cli admin maintenance-login` — Login with maintenance token or cookie to receive current information and perform further actions.
- `immich-pp-cli admin restore-user` — Restore a previously deleted user.
- `immich-pp-cli admin search-users` — Search for users.
- `immich-pp-cli admin send-test-email` — Send a test email using the provided SMTP configuration.
- `immich-pp-cli admin set-maintenance-mode` — Put Immich into or take it out of maintenance mode
- `immich-pp-cli admin start-database-restore-flow` — Put Immich into maintenance mode to restore a backup (Immich must not be configured)
- `immich-pp-cli admin unlink-all-oauth-accounts` — Unlinks all OAuth accounts associated with user accounts in the system.
- `immich-pp-cli admin update-user` — Update an existing user.
- `immich-pp-cli admin update-user-preferences` — Update the preferences of a specific user.
- `immich-pp-cli admin upload-database-backup` — Uploads .sql/.sql.gz file to restore backup from

**albums** — An album is a collection of assets that can be shared with other users or via shared links.

- `immich-pp-cli albums add-assets-to` — Send a list of asset IDs and album IDs to add each asset to each album.
- `immich-pp-cli albums create` — Create a new album. The album can also be created with initial users and assets.
- `immich-pp-cli albums delete` — Delete a specific album by its ID.
- `immich-pp-cli albums get-all` — Retrieve a list of albums available to the authenticated user.
- `immich-pp-cli albums get-info` — Retrieve information about a specific album by its ID.
- `immich-pp-cli albums get-statistics` — Returns statistics about the albums available to the authenticated user.
- `immich-pp-cli albums update-info` — Update the information of a specific album by its ID.

**api-keys** — An api key can be used to programmatically access the Immich API.

- `immich-pp-cli api-keys create` — Creates a new API key. It will be limited to the permissions specified.
- `immich-pp-cli api-keys delete` — Deletes an API key identified by its ID. The current user must own this API key.
- `immich-pp-cli api-keys get` — Retrieve all API keys of the current user.
- `immich-pp-cli api-keys get-apikeys` — Retrieve an API key by its ID. The current user must own this API key.
- `immich-pp-cli api-keys get-my` — Retrieve the API key that is used to access this endpoint.
- `immich-pp-cli api-keys update` — Updates the name and permissions of an API key by its ID. The current user must own this API key.

**assets** — An asset is an image or video that has been uploaded to Immich.

- `immich-pp-cli assets check-bulk-upload` — Determine which assets have already been uploaded to the server based on their SHA1 checksums.
- `immich-pp-cli assets copy` — Copy asset information like albums, tags, etc. from one asset to another.
- `immich-pp-cli assets delete` — Deletes multiple assets at the same time.
- `immich-pp-cli assets delete-bulk-metadata` — Delete metadata key-value pairs for multiple assets.
- `immich-pp-cli assets get-info` — Retrieve detailed information about a specific asset.
- `immich-pp-cli assets get-statistics` — Retrieve various statistics about the assets owned by the authenticated user.
- `immich-pp-cli assets run-jobs` — Run a specific job on a set of assets.
- `immich-pp-cli assets update` — Updates multiple assets at the same time.
- `immich-pp-cli assets update-bulk-metadata` — Upsert metadata key-value pairs for multiple assets.
- `immich-pp-cli assets update-id` — Update information of a specific asset.
- `immich-pp-cli assets upload` — Uploads a new asset to the server.

**download** — Endpoints for downloading assets or collections of assets.

- `immich-pp-cli download archive` — Download a ZIP archive containing the specified assets.
- `immich-pp-cli download get-info` — Retrieve information about how to request a download for the specified assets or album.

**duplicates** — Endpoints for managing and identifying duplicate assets.

- `immich-pp-cli duplicates delete` — Delete multiple duplicate assets specified by their IDs.
- `immich-pp-cli duplicates delete-id` — Dismiss a duplicate group by its ID, unlinking all assets in the group without deleting them.
- `immich-pp-cli duplicates get-asset` — Retrieve a list of duplicate assets available to the authenticated user.
- `immich-pp-cli duplicates resolve` — Resolve duplicate groups by synchronizing metadata across assets and deleting/trashing duplicates.

**faces** — A face is a detected human face within an asset, which can be associated with a person. Faces are normally detected via machine learning, but can also be created manually.

- `immich-pp-cli faces create` — Create a new face that has not been discovered by facial recognition.
- `immich-pp-cli faces delete` — Delete a face identified by the id. Optionally can be force deleted.
- `immich-pp-cli faces get` — Retrieve all faces belonging to an asset.
- `immich-pp-cli faces reassign-by-id` — Re-assign the face provided in the body to the person identified by the id in the path parameter.

**immich-auth** — Manage immich auth

- `immich-pp-cli immich-auth change-password` — Change the password of the current user.
- `immich-pp-cli immich-auth change-pin-code` — Change the pin code for the current user.
- `immich-pp-cli immich-auth get-status` — Get information about the current session, including whether the user has a password
- `immich-pp-cli immich-auth lock-session` — Remove elevated access to locked assets from the current session.
- `immich-pp-cli immich-auth login` — Login with username and password and receive a session token.
- `immich-pp-cli immich-auth logout` — Logout the current user and invalidate the session token.
- `immich-pp-cli immich-auth reset-pin-code` — Reset the pin code for the current user by providing the account password
- `immich-pp-cli immich-auth setup-pin-code` — Setup a new pin code for the current user.
- `immich-pp-cli immich-auth sign-up-admin` — Create the first admin user in the system.
- `immich-pp-cli immich-auth unlock-session` — Temporarily grant the session elevated access to locked assets by providing the correct PIN code.
- `immich-pp-cli immich-auth validate-access-token` — Validate the current authorization method is still valid.

**immich-jobs** — Manage immich jobs

- `immich-pp-cli immich-jobs create` — Run a specific job.
- `immich-pp-cli immich-jobs get-queues-legacy` — Retrieve the counts of the current queue, as well as the current status.
- `immich-pp-cli immich-jobs run-queue-command-legacy` — Queue all assets for a specific job type.

**immich-search** — Manage immich search

- `immich-pp-cli immich-search asset-statistics` — Retrieve statistical data about assets based on search criteria, such as the total matching count.
- `immich-pp-cli immich-search assets` — Search for assets based on various metadata criteria.
- `immich-pp-cli immich-search get-assets-by-city` — Retrieve a list of assets with each asset belonging to a different city.
- `immich-pp-cli immich-search get-explore-data` — Retrieve data for the explore section, such as popular people and places.
- `immich-pp-cli immich-search get-suggestions` — Retrieve search suggestions based on partial input. This endpoint is used for typeahead search features.
- `immich-pp-cli immich-search large-assets` — Search for assets that are considered large based on specified criteria.
- `immich-pp-cli immich-search person` — Search for people by name.
- `immich-pp-cli immich-search places` — Search for places by name.
- `immich-pp-cli immich-search random` — Retrieve a random selection of assets based on the provided criteria.
- `immich-pp-cli immich-search smart` — Perform a smart search for assets by using machine learning vectors to determine relevance.

**immich-sync** — Manage immich sync

- `immich-pp-cli immich-sync delete-ack` — Delete specific synchronization acknowledgments.
- `immich-pp-cli immich-sync get-ack` — Retrieve the synchronization acknowledgments for the current session.
- `immich-pp-cli immich-sync get-stream` — Retrieve a JSON lines streamed response of changes for synchronization.
- `immich-pp-cli immich-sync send-ack` — Send a list of synchronization acknowledgements to confirm that the latest changes have been received.

**libraries** — An external library is made up of input file paths or expressions that are scanned for asset files. Discovered files are automatically imported. Assets much be unique within a library, but can be duplicated across libraries. Each user has a default upload library, and can have one or more external libraries.

- `immich-pp-cli libraries create-library` — Create a new external library.
- `immich-pp-cli libraries delete-library` — Delete an external library by its ID.
- `immich-pp-cli libraries get-all` — Retrieve a list of external libraries.
- `immich-pp-cli libraries get-library` — Retrieve an external library by its ID.
- `immich-pp-cli libraries update-library` — Update an existing external library.

**map** — Map endpoints include supplemental functionality related to geolocation, such as reverse geocoding and retrieving map markers for assets with geolocation data.

- `immich-pp-cli map get-markers` — Retrieve a list of latitude and longitude coordinates for every asset with location data.
- `immich-pp-cli map reverse-geocode` — Retrieve location information (e.g., city, country) for given latitude and longitude coordinates.

**memories** — A memory is a specialized collection of assets with dedicated viewing implementations in the web and mobile clients. A memory includes fields related to visibility and are automatically generated per user via a background job.

- `immich-pp-cli memories create-memory` — Create a new memory by providing a name, description, and a list of asset IDs to include in the memory.
- `immich-pp-cli memories delete-memory` — Delete a specific memory by its ID.
- `immich-pp-cli memories get-memory` — Retrieve a specific memory by its ID.
- `immich-pp-cli memories search` — Retrieve a list of memories.
- `immich-pp-cli memories statistics` — Retrieve statistics about memories, such as total count and other relevant metrics.
- `immich-pp-cli memories update-memory` — Update an existing memory by its ID.

**notifications** — A notification is a specialized message sent to users to inform them of important events. Currently, these notifications are only shown in the Immich web application.

- `immich-pp-cli notifications delete` — Delete a list of notifications at once.
- `immich-pp-cli notifications delete-id` — Delete a specific notification.
- `immich-pp-cli notifications get` — Retrieve a list of notifications.
- `immich-pp-cli notifications get-id` — Retrieve a specific notification identified by id.
- `immich-pp-cli notifications update` — Update a list of notifications. Allows to bulk-set the read status of notifications.
- `immich-pp-cli notifications update-id` — Update a specific notification to set its read status.

**oauth** — Manage oauth

- `immich-pp-cli oauth finish` — Complete the OAuth authorization process by exchanging the authorization code for a session token.
- `immich-pp-cli oauth link-account` — Link an OAuth account to the authenticated user.
- `immich-pp-cli oauth logout` — Logout the OAuth account and invalidate the session specified by the sid claim or all sessions if the sid claim is not
- `immich-pp-cli oauth redirect-to-mobile` — Requests to this URL are automatically forwarded to the mobile app, and is used in some cases for OAuth redirecting.
- `immich-pp-cli oauth start` — Initiate the OAuth authorization process.
- `immich-pp-cli oauth unlink-account` — Unlink the OAuth account from the authenticated user.

**partners** — A partner is a link with another user that allows sharing of assets between two users.

- `immich-pp-cli partners create` — Create a new partner to share assets with.
- `immich-pp-cli partners create-deprecated` — Create a new partner to share assets with.
- `immich-pp-cli partners get` — Retrieve a list of partners with whom assets are shared.
- `immich-pp-cli partners remove` — Stop sharing assets with a partner.
- `immich-pp-cli partners update` — Specify whether a partner's assets should appear in the user's timeline.

**people** — A person is a collection of faces, which can be favorited and named. A person can also be merged into another person. People are automatically created via the face recognition job.

- `immich-pp-cli people create-person` — Create a new person that can have multiple faces assigned to them.
- `immich-pp-cli people delete` — Bulk delete a list of people at once.
- `immich-pp-cli people delete-person` — Delete an individual person.
- `immich-pp-cli people get-all` — Retrieve a list of all people.
- `immich-pp-cli people get-person` — Retrieve a person by id.
- `immich-pp-cli people update` — Bulk update multiple people at once.
- `immich-pp-cli people update-person` — Update an individual person.

**plugins** — A plugin is an installed module that makes filters and actions available for the workflow feature.

- `immich-pp-cli plugins get` — Retrieve information about a specific plugin by its ID.
- `immich-pp-cli plugins search` — Retrieve a list of plugins available to the authenticated user.
- `immich-pp-cli plugins search-methods` — Retrieve a list of plugin methods
- `immich-pp-cli plugins search-templates` — Retrieve workflow templates provided by installed plugins

**queues** — Queues and background jobs are used for processing tasks asynchronously. Queues can be paused and resumed as needed.

- `immich-pp-cli queues get` — Retrieves a list of queues.
- `immich-pp-cli queues get-name` — Retrieves a specific queue by its name.
- `immich-pp-cli queues update` — Change the paused status of a specific queue.

**server** — Information about the current server deployment, including version and build information, available features, supported media types, and more.

- `immich-pp-cli server delete-license` — Delete the currently set server product key.
- `immich-pp-cli server get-about-info` — Retrieve a list of information about the server.
- `immich-pp-cli server get-apk-links` — Retrieve links to the APKs for the current server version.
- `immich-pp-cli server get-config` — Retrieve the current server configuration.
- `immich-pp-cli server get-features` — Retrieve available features supported by this server.
- `immich-pp-cli server get-license` — Retrieve information about whether the server currently has a product key registered.
- `immich-pp-cli server get-statistics` — Retrieve statistics about the entire Immich instance such as asset counts.
- `immich-pp-cli server get-storage` — Retrieve the current storage utilization information of the server.
- `immich-pp-cli server get-supported-media-types` — Retrieve all media types supported by the server.
- `immich-pp-cli server get-version` — Retrieve the current server version in semantic versioning (semver) format.
- `immich-pp-cli server get-version-check` — Retrieve information about the last time the version check ran.
- `immich-pp-cli server get-version-history` — Retrieve a list of past versions the server has been on.
- `immich-pp-cli server ping` — Pong
- `immich-pp-cli server set-license` — Validate and set the server product key if successful.

**sessions** — A session represents an authenticated login session for a user. Sessions also appear in the web application as "Authorized devices".

- `immich-pp-cli sessions create` — Create a session as a child to the current session. This endpoint is used for casting.
- `immich-pp-cli sessions delete` — Delete a specific session by id.
- `immich-pp-cli sessions delete-all` — Delete all sessions for the user. This will not delete the current session.
- `immich-pp-cli sessions get` — Retrieve a list of sessions for the user.
- `immich-pp-cli sessions update` — Update a specific session identified by id.

**shared-links** — A shared link is a public url that provides access to a specific album, asset, or collection of assets. A shared link can be protected with a password, include a specific slug, allow or disallow downloads, and optionally include an expiration date.

- `immich-pp-cli shared-links create` — Create a new shared link.
- `immich-pp-cli shared-links get-all` — Retrieve a list of all shared links.
- `immich-pp-cli shared-links get-by-id` — Retrieve a specific shared link by its ID.
- `immich-pp-cli shared-links get-my` — Retrieve the current shared link associated with authentication method.
- `immich-pp-cli shared-links login` — Login to a password protected shared link
- `immich-pp-cli shared-links remove` — Delete a specific shared link by its ID.
- `immich-pp-cli shared-links update` — Update an existing shared link by its ID.

**stacks** — A stack is a group of related assets. One asset is the "primary" asset, and the rest are "child" assets. On the main timeline, stack parents are included by default, while child assets are hidden.

- `immich-pp-cli stacks create` — Create a new stack by providing a name and a list of asset IDs to include in the stack.
- `immich-pp-cli stacks delete` — Delete multiple stacks by providing a list of stack IDs.
- `immich-pp-cli stacks delete-id` — Delete a specific stack by its ID.
- `immich-pp-cli stacks get` — Retrieve a specific stack by its ID.
- `immich-pp-cli stacks search` — Retrieve a list of stacks.
- `immich-pp-cli stacks update` — Update an existing stack by its ID.

**system-config** — Endpoints to view, modify, and validate the system configuration settings.

- `immich-pp-cli system-config get-config` — Retrieve the current system configuration.
- `immich-pp-cli system-config get-config-defaults` — Retrieve the default values for the system configuration.
- `immich-pp-cli system-config get-storage-template-options` — Retrieve exemplary storage template options.
- `immich-pp-cli system-config update-config` — Update the system configuration with a new system configuration.

**system-metadata** — Endpoints to view, modify, and validate the system metadata, which includes information about things like admin onboarding status.

- `immich-pp-cli system-metadata get-admin-onboarding` — Retrieve the current admin onboarding status.
- `immich-pp-cli system-metadata get-reverse-geocoding-state` — Retrieve the current state of the reverse geocoding import.
- `immich-pp-cli system-metadata get-version-check-state` — Retrieve the current state of the version check process.
- `immich-pp-cli system-metadata update-admin-onboarding` — Update the admin onboarding status.

**tags** — A tag is a user-defined label that can be applied to assets for organizational purposes. Tags can also be hierarchical, allowing for parent-child relationships between tags.

- `immich-pp-cli tags bulk-assets` — Add multiple tags to multiple assets in a single request.
- `immich-pp-cli tags create` — Create a new tag by providing a name and optional color.
- `immich-pp-cli tags delete` — Delete a specific tag by its ID.
- `immich-pp-cli tags get-all` — Retrieve a list of all tags.
- `immich-pp-cli tags get-by-id` — Retrieve a specific tag by its ID.
- `immich-pp-cli tags update` — Update an existing tag identified by its ID.
- `immich-pp-cli tags upsert` — Create or update multiple tags in a single request.

**timeline** — Specialized endpoints related to the timeline implementation used in the web application. External applications or tools should not use or rely on these endpoints, as they are subject to change without notice.

- `immich-pp-cli timeline get-time-bucket` — Retrieve a string of all asset ids in a given time bucket.
- `immich-pp-cli timeline get-time-buckets` — Retrieve a list of all minimal time buckets.

**trash** — Endpoints for managing the trash can, which includes assets that have been discarded. Items in the trash are automatically deleted after a configured amount of time.

- `immich-pp-cli trash empty` — Permanently delete all items in the trash.
- `immich-pp-cli trash restore` — Restore all items in the trash.
- `immich-pp-cli trash restore-assets` — Restore specific assets from the trash.

**users** — Endpoints for viewing and updating the current users, including product key information, profile picture data, onboarding progress, and more.

- `immich-pp-cli users create-profile-image` — Upload and set a new profile image for the current user.
- `immich-pp-cli users delete-license` — Delete the registered product key for the current user.
- `immich-pp-cli users delete-onboarding` — Delete the onboarding status of the current user.
- `immich-pp-cli users delete-profile-image` — Delete the profile image of the current user.
- `immich-pp-cli users get` — Retrieve a specific user by their ID.
- `immich-pp-cli users get-license` — Retrieve information about whether the current user has a registered product key.
- `immich-pp-cli users get-my` — Retrieve information about the user making the API request.
- `immich-pp-cli users get-my-calendar-heatmap` — Retrieve activity counts for a specified period, in a calendar heatmap format.
- `immich-pp-cli users get-my-preferences` — Retrieve the preferences for the current user.
- `immich-pp-cli users get-onboarding` — Retrieve the onboarding status of the current user.
- `immich-pp-cli users search` — Retrieve a list of all users on the server.
- `immich-pp-cli users set-license` — Register a product key for the current user.
- `immich-pp-cli users set-onboarding` — Update the onboarding status of the current user.
- `immich-pp-cli users update-my` — Update the current user making the API request.
- `immich-pp-cli users update-my-preferences` — Update the preferences of the current user.

**view** — Endpoints for specialized views, such as the folder view.

- `immich-pp-cli view get-assets-by-original-path` — Retrieve assets that are children of a specific folder.
- `immich-pp-cli view get-unique-original-paths` — Retrieve a list of unique folder paths from asset original paths.

**workflows** — A workflow is a set of actions that run whenever a triggering event occurs. Workflows also can include filters to further limit execution.

- `immich-pp-cli workflows create` — Create a new workflow, the workflow can also be created with empty filters and actions.
- `immich-pp-cli workflows delete` — Delete a workflow by its ID.
- `immich-pp-cli workflows get` — Retrieve information about a specific workflow by its ID.
- `immich-pp-cli workflows get-triggers` — Retrieve a list of all available workflow triggers.
- `immich-pp-cli workflows search` — Retrieve a list of workflows available to the authenticated user.
- `immich-pp-cli workflows update` — Update the information of a specific workflow by its ID.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
immich-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Preview an event album

```bash
immich-pp-cli album event beach-weekend --from 2025-07-01 --to 2025-07-07 --dry-run --agent
```

See the event-album operation before it writes albums or membership.

### Plan cleanup

```bash
immich-pp-cli duplicates plan --limit 20 --agent
```

Inspect native duplicate recommendations in structured output.

### Review memories

```bash
immich-pp-cli memories review --limit 12 --agent
```

Read recent native memories as bounded library facts.

### Check self-hosted health

```bash
immich-pp-cli library health --agent
```

Combine partner and job queue state in one non-mutating report.

## Auth Setup

Set IMMICH_BASE_URL to the API root of your own Immich server and IMMICH_API_KEY to an API key with only the permissions your commands need. The CLI sends the key as x-api-key and never prints it.

Run `immich-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  immich-pp-cli activities get --album-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

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

- Use `--home <dir>` for one invocation, or set `IMMICH_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `IMMICH_CONFIG_DIR`, `IMMICH_DATA_DIR`, `IMMICH_STATE_DIR`, `IMMICH_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `IMMICH_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `immich-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "immich": {
        "command": "immich-pp-mcp",
        "env": {
          "IMMICH_HOME": "/srv/immich"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `IMMICH_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `IMMICH_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
immich-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "immich-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `immich-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `immich-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `immich-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
immich-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
immich-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
immich-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
immich-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`immich-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `IMMICH_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
immich-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
immich-pp-cli feedback --stdin < notes.txt
immich-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `IMMICH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IMMICH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
immich-pp-cli profile save briefing --json
immich-pp-cli --profile briefing activities get --album-id 550e8400-e29b-41d4-a716-446655440000
immich-pp-cli profile list --json
immich-pp-cli profile show briefing
immich-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `immich-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/immich/cmd/immich-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add immich-pp-mcp -- immich-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which immich-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   immich-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `immich-pp-cli <command> --help`.

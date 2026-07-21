# Immich CLI

**Agent-native control and careful personal-library rituals for a self-hosted Immich instance.**

Covers Immich's official v3 API, the upload/import strengths of immich-go, and the broad raw operations of ImmichMCP, then adds safe personal workflows for events, duplicate cleanup, family time queries, memories, stacks, partners, and jobs.

## Install

The recommended path installs both the `immich-pp-cli` binary and the `pp-immich` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install immich
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install immich --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install immich --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install immich --agent claude-code
npx -y @mvanhorn/printing-press-library install immich --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/immich/cmd/immich-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immich-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install immich --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-immich --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-immich --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install immich --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immich-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `IMMICH_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/immich/cmd/immich-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "immich": {
      "command": "immich-pp-mcp",
      "env": {
        "IMMICH_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set IMMICH_BASE_URL to the API root of your own Immich server and IMMICH_API_KEY to an API key with only the permissions your commands need. The CLI sends the key as x-api-key and never prints it.

## Quick Start

```bash
# Verify the configured self-hosted URL and API-key prerequisites without a request.
immich-pp-cli doctor --dry-run

# Preview a shared event-album plan before changing the library.
immich-pp-cli album event beach-weekend --from 2025-07-01 --to 2025-07-07 --dry-run --agent

# Review native duplicate groups before resolving anything.
immich-pp-cli duplicates plan --limit 20 --agent

# Check partner and queue facts on the current Immich server.
immich-pp-cli library health --agent

```

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

For every selected group, copy the `evidence` array from that reviewed plan
exactly. When the server has no keeper recommendation, also choose an explicit
keeper. Apply re-fetches the group and rejects missing or changed evidence before
it computes anything to trash.

```bash
immich-pp-cli duplicates apply --groups '[{"group_id":"group-id","keeper":"asset-to-keep","evidence":["asset-to-keep","asset-to-trash"]}]' --apply --agent
```

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

## Unique Features

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

  _Use it only after reviewing a duplicate plan._

  ```bash
  immich-pp-cli duplicates apply --groups '[{"group_id":"group-id","evidence":["server-reviewed-asset","other-reviewed-asset"]}]' --apply --agent
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

## Usage

Run `immich-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IMMICH_CONFIG_DIR`, `IMMICH_DATA_DIR`, `IMMICH_STATE_DIR`, or `IMMICH_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IMMICH_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IMMICH_HOME=/srv/immich
immich-pp-cli doctor
```

Under `IMMICH_HOME=/srv/immich`, the four dirs resolve to `/srv/immich/config`, `/srv/immich/data`, `/srv/immich/state`, and `/srv/immich/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `IMMICH_DATA_DIR` overrides an explicit `--home` for that kind. Use `IMMICH_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IMMICH_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `immich-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### activities

An activity is a like or a comment made by a user on an asset or album.

- **`immich-pp-cli activities create-activity`** - Create a like or a comment for an album, or an asset in an album.
- **`immich-pp-cli activities delete-activity`** - Removes a like or comment from a given album or asset in an album.
- **`immich-pp-cli activities get`** - Returns a list of activities for the selected asset or album. The activities are returned in sorted order, with the oldest activities appearing first.
- **`immich-pp-cli activities get-activity-statistics`** - Returns the number of likes and comments for a given album or asset in an album.

### admin

Manage admin

- **`immich-pp-cli admin create-notification`** - Create a new notification for a specific user.
- **`immich-pp-cli admin create-user`** - Create a new user.
- **`immich-pp-cli admin delete-database-backup`** - Delete a backup by its filename
- **`immich-pp-cli admin delete-integrity-report`** - Delete a given report item and perform corresponding deletion (e.g. trash asset, delete file)
- **`immich-pp-cli admin delete-user`** - Delete a user.
- **`immich-pp-cli admin detect-prior-install`** - Collect integrity checks and other heuristics about local data.
- **`immich-pp-cli admin download-database-backup`** - Downloads the database backup file
- **`immich-pp-cli admin get-integrity-report`** - Get all flagged items by integrity report type
- **`immich-pp-cli admin get-integrity-report-csv`** - Get all integrity report entries for a given type as a CSV
- **`immich-pp-cli admin get-integrity-report-file`** - Download the untracked/broken file if one exists
- **`immich-pp-cli admin get-integrity-report-summary`** - Get a count of the items flagged in each integrity report
- **`immich-pp-cli admin get-maintenance-status`** - Fetch information about the currently running maintenance action.
- **`immich-pp-cli admin get-notification-template`** - Retrieve a preview of the provided email template.
- **`immich-pp-cli admin get-user`** - Retrieve  a specific user by their ID.
- **`immich-pp-cli admin get-user-calendar-heatmap`** - Retrieve activity counts for a specified period, in a calendar heatmap format.
- **`immich-pp-cli admin get-user-preferences`** - Retrieve the preferences of a specific user.
- **`immich-pp-cli admin get-user-sessions`** - Retrieve all sessions for a specific user.
- **`immich-pp-cli admin get-user-statistics`** - Retrieve asset statistics for a specific user.
- **`immich-pp-cli admin list-database-backups`** - Get the list of the successful and failed backups
- **`immich-pp-cli admin maintenance-login`** - Login with maintenance token or cookie to receive current information and perform further actions.
- **`immich-pp-cli admin restore-user`** - Restore a previously deleted user.
- **`immich-pp-cli admin search-users`** - Search for users.
- **`immich-pp-cli admin send-test-email`** - Send a test email using the provided SMTP configuration.
- **`immich-pp-cli admin set-maintenance-mode`** - Put Immich into or take it out of maintenance mode
- **`immich-pp-cli admin start-database-restore-flow`** - Put Immich into maintenance mode to restore a backup (Immich must not be configured)
- **`immich-pp-cli admin unlink-all-oauth-accounts`** - Unlinks all OAuth accounts associated with user accounts in the system.
- **`immich-pp-cli admin update-user`** - Update an existing user.
- **`immich-pp-cli admin update-user-preferences`** - Update the preferences of a specific user.
- **`immich-pp-cli admin upload-database-backup`** - Uploads .sql/.sql.gz file to restore backup from

### albums

An album is a collection of assets that can be shared with other users or via shared links.

- **`immich-pp-cli albums add-assets-to`** - Send a list of asset IDs and album IDs to add each asset to each album.
- **`immich-pp-cli albums create`** - Create a new album. The album can also be created with initial users and assets.
- **`immich-pp-cli albums delete`** - Delete a specific album by its ID. Note the album is initially trashed and then immediately scheduled for deletion, but relies on a background job to complete the process.
- **`immich-pp-cli albums get-all`** - Retrieve a list of albums available to the authenticated user.
- **`immich-pp-cli albums get-info`** - Retrieve information about a specific album by its ID.
- **`immich-pp-cli albums get-statistics`** - Returns statistics about the albums available to the authenticated user.
- **`immich-pp-cli albums update-info`** - Update the information of a specific album by its ID. This endpoint can be used to update the album name, description, sort order, etc. However, it is not used to add or remove assets or users from the album.

### api-keys

An api key can be used to programmatically access the Immich API.

- **`immich-pp-cli api-keys create`** - Creates a new API key. It will be limited to the permissions specified.
- **`immich-pp-cli api-keys delete`** - Deletes an API key identified by its ID. The current user must own this API key.
- **`immich-pp-cli api-keys get`** - Retrieve all API keys of the current user.
- **`immich-pp-cli api-keys get-apikeys`** - Retrieve an API key by its ID. The current user must own this API key.
- **`immich-pp-cli api-keys get-my`** - Retrieve the API key that is used to access this endpoint.
- **`immich-pp-cli api-keys update`** - Updates the name and permissions of an API key by its ID. The current user must own this API key.

### assets

An asset is an image or video that has been uploaded to Immich.

- **`immich-pp-cli assets check-bulk-upload`** - Determine which assets have already been uploaded to the server based on their SHA1 checksums.
- **`immich-pp-cli assets copy`** - Copy asset information like albums, tags, etc. from one asset to another.
- **`immich-pp-cli assets delete`** - Deletes multiple assets at the same time.
- **`immich-pp-cli assets delete-bulk-metadata`** - Delete metadata key-value pairs for multiple assets.
- **`immich-pp-cli assets get-info`** - Retrieve detailed information about a specific asset.
- **`immich-pp-cli assets get-statistics`** - Retrieve various statistics about the assets owned by the authenticated user.
- **`immich-pp-cli assets run-jobs`** - Run a specific job on a set of assets.
- **`immich-pp-cli assets update`** - Updates multiple assets at the same time.
- **`immich-pp-cli assets update-bulk-metadata`** - Upsert metadata key-value pairs for multiple assets.
- **`immich-pp-cli assets update-id`** - Update information of a specific asset.
- **`immich-pp-cli assets upload`** - Uploads a new asset to the server.

### download

Endpoints for downloading assets or collections of assets.

- **`immich-pp-cli download archive`** - Download a ZIP archive containing the specified assets. The assets must have been previously requested via the "getDownloadInfo" endpoint.
- **`immich-pp-cli download get-info`** - Retrieve information about how to request a download for the specified assets or album. The response includes groups of assets that can be downloaded together.

### duplicates

Endpoints for managing and identifying duplicate assets.

- **`immich-pp-cli duplicates delete`** - Delete multiple duplicate assets specified by their IDs.
- **`immich-pp-cli duplicates delete-id`** - Dismiss a duplicate group by its ID, unlinking all assets in the group without deleting them.
- **`immich-pp-cli duplicates get-asset`** - Retrieve a list of duplicate assets available to the authenticated user.
- **`immich-pp-cli duplicates resolve`** - Resolve duplicate groups by synchronizing metadata across assets and deleting/trashing duplicates.

### faces

A face is a detected human face within an asset, which can be associated with a person. Faces are normally detected via machine learning, but can also be created manually.

- **`immich-pp-cli faces create`** - Create a new face that has not been discovered by facial recognition. The content of the bounding box is considered a face.
- **`immich-pp-cli faces delete`** - Delete a face identified by the id. Optionally can be force deleted.
- **`immich-pp-cli faces get`** - Retrieve all faces belonging to an asset.
- **`immich-pp-cli faces reassign-by-id`** - Re-assign the face provided in the body to the person identified by the id in the path parameter.

### immich-auth

Manage immich auth

- **`immich-pp-cli immich-auth change-password`** - Change the password of the current user.
- **`immich-pp-cli immich-auth change-pin-code`** - Change the pin code for the current user.
- **`immich-pp-cli immich-auth get-status`** - Get information about the current session, including whether the user has a password, and if the session can access locked assets.
- **`immich-pp-cli immich-auth lock-session`** - Remove elevated access to locked assets from the current session.
- **`immich-pp-cli immich-auth login`** - Login with username and password and receive a session token.
- **`immich-pp-cli immich-auth logout`** - Logout the current user and invalidate the session token.
- **`immich-pp-cli immich-auth reset-pin-code`** - Reset the pin code for the current user by providing the account password
- **`immich-pp-cli immich-auth setup-pin-code`** - Setup a new pin code for the current user.
- **`immich-pp-cli immich-auth sign-up-admin`** - Create the first admin user in the system.
- **`immich-pp-cli immich-auth unlock-session`** - Temporarily grant the session elevated access to locked assets by providing the correct PIN code.
- **`immich-pp-cli immich-auth validate-access-token`** - Validate the current authorization method is still valid.

### immich-jobs

Manage immich jobs

- **`immich-pp-cli immich-jobs create`** - Run a specific job. Most jobs are queued automatically, but this endpoint allows for manual creation of a handful of jobs, including various cleanup tasks, as well as creating a new database backup.
- **`immich-pp-cli immich-jobs get-queues-legacy`** - Retrieve the counts of the current queue, as well as the current status.
- **`immich-pp-cli immich-jobs run-queue-command-legacy`** - Queue all assets for a specific job type. Defaults to only queueing assets that have not yet been processed, but the force command can be used to re-process all assets.

### immich-search

Manage immich search

- **`immich-pp-cli immich-search asset-statistics`** - Retrieve statistical data about assets based on search criteria, such as the total matching count.
- **`immich-pp-cli immich-search assets`** - Search for assets based on various metadata criteria.
- **`immich-pp-cli immich-search get-assets-by-city`** - Retrieve a list of assets with each asset belonging to a different city. This endpoint is used on the places pages to show a single thumbnail for each city the user has assets in.
- **`immich-pp-cli immich-search get-explore-data`** - Retrieve data for the explore section, such as popular people and places.
- **`immich-pp-cli immich-search get-suggestions`** - Retrieve search suggestions based on partial input. This endpoint is used for typeahead search features.
- **`immich-pp-cli immich-search large-assets`** - Search for assets that are considered large based on specified criteria.
- **`immich-pp-cli immich-search person`** - Search for people by name.
- **`immich-pp-cli immich-search places`** - Search for places by name.
- **`immich-pp-cli immich-search random`** - Retrieve a random selection of assets based on the provided criteria.
- **`immich-pp-cli immich-search smart`** - Perform a smart search for assets by using machine learning vectors to determine relevance.

### immich-sync

Manage immich sync

- **`immich-pp-cli immich-sync delete-ack`** - Delete specific synchronization acknowledgments.
- **`immich-pp-cli immich-sync get-ack`** - Retrieve the synchronization acknowledgments for the current session.
- **`immich-pp-cli immich-sync get-stream`** - Retrieve a JSON lines streamed response of changes for synchronization. This endpoint is used by the mobile app to efficiently stay up to date with changes.
- **`immich-pp-cli immich-sync send-ack`** - Send a list of synchronization acknowledgements to confirm that the latest changes have been received.

### libraries

An external library is made up of input file paths or expressions that are scanned for asset files. Discovered files are automatically imported. Assets much be unique within a library, but can be duplicated across libraries. Each user has a default upload library, and can have one or more external libraries.

- **`immich-pp-cli libraries create-library`** - Create a new external library.
- **`immich-pp-cli libraries delete-library`** - Delete an external library by its ID.
- **`immich-pp-cli libraries get-all`** - Retrieve a list of external libraries.
- **`immich-pp-cli libraries get-library`** - Retrieve an external library by its ID.
- **`immich-pp-cli libraries update-library`** - Update an existing external library.

### map

Map endpoints include supplemental functionality related to geolocation, such as reverse geocoding and retrieving map markers for assets with geolocation data.

- **`immich-pp-cli map get-markers`** - Retrieve a list of latitude and longitude coordinates for every asset with location data.
- **`immich-pp-cli map reverse-geocode`** - Retrieve location information (e.g., city, country) for given latitude and longitude coordinates.

### memories

A memory is a specialized collection of assets with dedicated viewing implementations in the web and mobile clients. A memory includes fields related to visibility and are automatically generated per user via a background job.

- **`immich-pp-cli memories create-memory`** - Create a new memory by providing a name, description, and a list of asset IDs to include in the memory.
- **`immich-pp-cli memories delete-memory`** - Delete a specific memory by its ID.
- **`immich-pp-cli memories get-memory`** - Retrieve a specific memory by its ID.
- **`immich-pp-cli memories search`** - Retrieve a list of memories. Memories are sorted descending by creation date by default, although they can also be sorted in ascending order, or randomly.
- **`immich-pp-cli memories statistics`** - Retrieve statistics about memories, such as total count and other relevant metrics.
- **`immich-pp-cli memories update-memory`** - Update an existing memory by its ID.

### notifications

A notification is a specialized message sent to users to inform them of important events. Currently, these notifications are only shown in the Immich web application.

- **`immich-pp-cli notifications delete`** - Delete a list of notifications at once.
- **`immich-pp-cli notifications delete-id`** - Delete a specific notification.
- **`immich-pp-cli notifications get`** - Retrieve a list of notifications.
- **`immich-pp-cli notifications get-id`** - Retrieve a specific notification identified by id.
- **`immich-pp-cli notifications update`** - Update a list of notifications. Allows to bulk-set the read status of notifications.
- **`immich-pp-cli notifications update-id`** - Update a specific notification to set its read status.

### oauth

Manage oauth

- **`immich-pp-cli oauth finish`** - Complete the OAuth authorization process by exchanging the authorization code for a session token.
- **`immich-pp-cli oauth link-account`** - Link an OAuth account to the authenticated user.
- **`immich-pp-cli oauth logout`** - Logout the OAuth account and invalidate the session specified by the sid claim or all sessions if the sid claim is not present.
- **`immich-pp-cli oauth redirect-to-mobile`** - Requests to this URL are automatically forwarded to the mobile app, and is used in some cases for OAuth redirecting.
- **`immich-pp-cli oauth start`** - Initiate the OAuth authorization process.
- **`immich-pp-cli oauth unlink-account`** - Unlink the OAuth account from the authenticated user.

### partners

A partner is a link with another user that allows sharing of assets between two users.

- **`immich-pp-cli partners create`** - Create a new partner to share assets with.
- **`immich-pp-cli partners create-deprecated`** - Create a new partner to share assets with.
- **`immich-pp-cli partners get`** - Retrieve a list of partners with whom assets are shared.
- **`immich-pp-cli partners remove`** - Stop sharing assets with a partner.
- **`immich-pp-cli partners update`** - Specify whether a partner's assets should appear in the user's timeline.

### people

A person is a collection of faces, which can be favorited and named. A person can also be merged into another person. People are automatically created via the face recognition job.

- **`immich-pp-cli people create-person`** - Create a new person that can have multiple faces assigned to them.
- **`immich-pp-cli people delete`** - Bulk delete a list of people at once.
- **`immich-pp-cli people delete-person`** - Delete an individual person.
- **`immich-pp-cli people get-all`** - Retrieve a list of all people.
- **`immich-pp-cli people get-person`** - Retrieve a person by id.
- **`immich-pp-cli people update`** - Bulk update multiple people at once.
- **`immich-pp-cli people update-person`** - Update an individual person.

### plugins

A plugin is an installed module that makes filters and actions available for the workflow feature.

- **`immich-pp-cli plugins get`** - Retrieve information about a specific plugin by its ID.
- **`immich-pp-cli plugins search`** - Retrieve a list of plugins available to the authenticated user.
- **`immich-pp-cli plugins search-methods`** - Retrieve a list of plugin methods
- **`immich-pp-cli plugins search-templates`** - Retrieve workflow templates provided by installed plugins

### queues

Queues and background jobs are used for processing tasks asynchronously. Queues can be paused and resumed as needed.

- **`immich-pp-cli queues get`** - Retrieves a list of queues.
- **`immich-pp-cli queues get-name`** - Retrieves a specific queue by its name.
- **`immich-pp-cli queues update`** - Change the paused status of a specific queue.

### server

Information about the current server deployment, including version and build information, available features, supported media types, and more.

- **`immich-pp-cli server delete-license`** - Delete the currently set server product key.
- **`immich-pp-cli server get-about-info`** - Retrieve a list of information about the server.
- **`immich-pp-cli server get-apk-links`** - Retrieve links to the APKs for the current server version.
- **`immich-pp-cli server get-config`** - Retrieve the current server configuration.
- **`immich-pp-cli server get-features`** - Retrieve available features supported by this server.
- **`immich-pp-cli server get-license`** - Retrieve information about whether the server currently has a product key registered.
- **`immich-pp-cli server get-statistics`** - Retrieve statistics about the entire Immich instance such as asset counts.
- **`immich-pp-cli server get-storage`** - Retrieve the current storage utilization information of the server.
- **`immich-pp-cli server get-supported-media-types`** - Retrieve all media types supported by the server.
- **`immich-pp-cli server get-version`** - Retrieve the current server version in semantic versioning (semver) format.
- **`immich-pp-cli server get-version-check`** - Retrieve information about the last time the version check ran.
- **`immich-pp-cli server get-version-history`** - Retrieve a list of past versions the server has been on.
- **`immich-pp-cli server ping`** - Pong
- **`immich-pp-cli server set-license`** - Validate and set the server product key if successful.

### sessions

A session represents an authenticated login session for a user. Sessions also appear in the web application as "Authorized devices".

- **`immich-pp-cli sessions create`** - Create a session as a child to the current session. This endpoint is used for casting.
- **`immich-pp-cli sessions delete`** - Delete a specific session by id.
- **`immich-pp-cli sessions delete-all`** - Delete all sessions for the user. This will not delete the current session.
- **`immich-pp-cli sessions get`** - Retrieve a list of sessions for the user.
- **`immich-pp-cli sessions update`** - Update a specific session identified by id.

### shared-links

A shared link is a public url that provides access to a specific album, asset, or collection of assets. A shared link can be protected with a password, include a specific slug, allow or disallow downloads, and optionally include an expiration date.

- **`immich-pp-cli shared-links create`** - Create a new shared link.
- **`immich-pp-cli shared-links get-all`** - Retrieve a list of all shared links.
- **`immich-pp-cli shared-links get-by-id`** - Retrieve a specific shared link by its ID.
- **`immich-pp-cli shared-links get-my`** - Retrieve the current shared link associated with authentication method.
- **`immich-pp-cli shared-links login`** - Login to a password protected shared link
- **`immich-pp-cli shared-links remove`** - Delete a specific shared link by its ID.
- **`immich-pp-cli shared-links update`** - Update an existing shared link by its ID.

### stacks

A stack is a group of related assets. One asset is the "primary" asset, and the rest are "child" assets. On the main timeline, stack parents are included by default, while child assets are hidden.

- **`immich-pp-cli stacks create`** - Create a new stack by providing a name and a list of asset IDs to include in the stack. If any of the provided asset IDs are primary assets of an existing stack, the existing stack will be merged into the newly created stack.
- **`immich-pp-cli stacks delete`** - Delete multiple stacks by providing a list of stack IDs.
- **`immich-pp-cli stacks delete-id`** - Delete a specific stack by its ID.
- **`immich-pp-cli stacks get`** - Retrieve a specific stack by its ID.
- **`immich-pp-cli stacks search`** - Retrieve a list of stacks.
- **`immich-pp-cli stacks update`** - Update an existing stack by its ID.

### system-config

Endpoints to view, modify, and validate the system configuration settings.

- **`immich-pp-cli system-config get-config`** - Retrieve the current system configuration.
- **`immich-pp-cli system-config get-config-defaults`** - Retrieve the default values for the system configuration.
- **`immich-pp-cli system-config get-storage-template-options`** - Retrieve exemplary storage template options.
- **`immich-pp-cli system-config update-config`** - Update the system configuration with a new system configuration.

### system-metadata

Endpoints to view, modify, and validate the system metadata, which includes information about things like admin onboarding status.

- **`immich-pp-cli system-metadata get-admin-onboarding`** - Retrieve the current admin onboarding status.
- **`immich-pp-cli system-metadata get-reverse-geocoding-state`** - Retrieve the current state of the reverse geocoding import.
- **`immich-pp-cli system-metadata get-version-check-state`** - Retrieve the current state of the version check process.
- **`immich-pp-cli system-metadata update-admin-onboarding`** - Update the admin onboarding status.

### tags

A tag is a user-defined label that can be applied to assets for organizational purposes. Tags can also be hierarchical, allowing for parent-child relationships between tags.

- **`immich-pp-cli tags bulk-assets`** - Add multiple tags to multiple assets in a single request.
- **`immich-pp-cli tags create`** - Create a new tag by providing a name and optional color.
- **`immich-pp-cli tags delete`** - Delete a specific tag by its ID.
- **`immich-pp-cli tags get-all`** - Retrieve a list of all tags.
- **`immich-pp-cli tags get-by-id`** - Retrieve a specific tag by its ID.
- **`immich-pp-cli tags update`** - Update an existing tag identified by its ID.
- **`immich-pp-cli tags upsert`** - Create or update multiple tags in a single request.

### timeline

Specialized endpoints related to the timeline implementation used in the web application. External applications or tools should not use or rely on these endpoints, as they are subject to change without notice.

- **`immich-pp-cli timeline get-time-bucket`** - Retrieve a string of all asset ids in a given time bucket.
- **`immich-pp-cli timeline get-time-buckets`** - Retrieve a list of all minimal time buckets.

### trash

Endpoints for managing the trash can, which includes assets that have been discarded. Items in the trash are automatically deleted after a configured amount of time.

- **`immich-pp-cli trash empty`** - Permanently delete all items in the trash.
- **`immich-pp-cli trash restore`** - Restore all items in the trash.
- **`immich-pp-cli trash restore-assets`** - Restore specific assets from the trash.

### users

Endpoints for viewing and updating the current users, including product key information, profile picture data, onboarding progress, and more.

- **`immich-pp-cli users create-profile-image`** - Upload and set a new profile image for the current user.
- **`immich-pp-cli users delete-license`** - Delete the registered product key for the current user.
- **`immich-pp-cli users delete-onboarding`** - Delete the onboarding status of the current user.
- **`immich-pp-cli users delete-profile-image`** - Delete the profile image of the current user.
- **`immich-pp-cli users get`** - Retrieve a specific user by their ID.
- **`immich-pp-cli users get-license`** - Retrieve information about whether the current user has a registered product key.
- **`immich-pp-cli users get-my`** - Retrieve information about the user making the API request.
- **`immich-pp-cli users get-my-calendar-heatmap`** - Retrieve activity counts for a specified period, in a calendar heatmap format.
- **`immich-pp-cli users get-my-preferences`** - Retrieve the preferences for the current user.
- **`immich-pp-cli users get-onboarding`** - Retrieve the onboarding status of the current user.
- **`immich-pp-cli users search`** - Retrieve a list of all users on the server.
- **`immich-pp-cli users set-license`** - Register a product key for the current user.
- **`immich-pp-cli users set-onboarding`** - Update the onboarding status of the current user.
- **`immich-pp-cli users update-my`** - Update the current user making the API request.
- **`immich-pp-cli users update-my-preferences`** - Update the preferences of the current user.

### view

Endpoints for specialized views, such as the folder view.

- **`immich-pp-cli view get-assets-by-original-path`** - Retrieve assets that are children of a specific folder.
- **`immich-pp-cli view get-unique-original-paths`** - Retrieve a list of unique folder paths from asset original paths.

### workflows

A workflow is a set of actions that run whenever a triggering event occurs. Workflows also can include filters to further limit execution.

- **`immich-pp-cli workflows create`** - Create a new workflow, the workflow can also be created with empty filters and actions.
- **`immich-pp-cli workflows delete`** - Delete a workflow by its ID.
- **`immich-pp-cli workflows get`** - Retrieve information about a specific workflow by its ID.
- **`immich-pp-cli workflows get-triggers`** - Retrieve a list of all available workflow triggers.
- **`immich-pp-cli workflows search`** - Retrieve a list of workflows available to the authenticated user.
- **`immich-pp-cli workflows update`** - Update the information of a specific workflow by its ID. This endpoint can be used to update the workflow name, description, trigger type, filters and actions order, etc.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`immich-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`immich-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`immich-pp-cli learnings list`** - Inspect taught rows
- **`immich-pp-cli learnings forget <query>`** - Undo a teach
- **`immich-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`immich-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`immich-pp-cli teach-pattern`** - Install a query/resource template up front
- **`immich-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `IMMICH_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `immich-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
immich-pp-cli activities get --album-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
immich-pp-cli activities get --album-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
immich-pp-cli activities get --album-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
immich-pp-cli activities get --album-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
immich-pp-cli activities get --album-id 550e8400-e29b-41d4-a716-446655440000 --agent
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

## Health Check

```bash
immich-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `immich-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/immich-pp-cli/config.toml`; `--home`, `IMMICH_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `IMMICH_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `immich-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `immich-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $IMMICH_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 or 403 response** — Create an Immich API key with the least permissions required and export it as IMMICH_API_KEY.
- **The self-hosted server cannot be reached** — Set IMMICH_BASE_URL to the API root reachable from this machine, then run immich-pp-cli doctor.
- **A destructive operation was refused** — Run the corresponding preview command first, then supply the command's explicit apply flag only after reviewing the IDs.

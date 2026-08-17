# Pinecone CLI

**Every Pinecone API feature, plus local sync, snapshot history, and text-first search no other Pinecone tool has.**

The Pinecone CLI manages indexes, vectors, namespaces, backups, imports, inference, and admin — then goes further: sync index and record metadata into a local SQLite database for offline search, capture snapshot history to diff and project growth, and search by natural language without touching vectors. Agent-native --json/--select/csv output and typed exit codes make it scriptable in CI.

## Install

The recommended path installs both the `pinecone-pp-cli` binary and the `pp-pinecone` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pinecone
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pinecone --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pinecone --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pinecone --agent claude-code
npx -y @mvanhorn/printing-press-library install pinecone --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/pinecone/cmd/pinecone-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pinecone-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install pinecone --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pinecone --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pinecone --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install pinecone --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pinecone-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PINECONE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/pinecone/cmd/pinecone-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pinecone": {
      "command": "pinecone-pp-mcp",
      "env": {
        "PINECONE_ASSISTANT_HOST": "<assistant_host>",
        "PINECONE_INDEX_HOST": "<index_host>",
        "PINECONE_SCHEDULE_ID": "<schedule_id>",
        "PINECONE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Pinecone uses an API key sent as the `Api-Key` header, with the required `X-Pinecone-Api-Version` header set to the API version (2026-04). Set PINECONE_API_KEY=<key> or run `pinecone-pp-cli auth set-token`. Data-plane commands target the per-index host, which the CLI resolves automatically from `indexes describe-index` when you pass --index, or via PINECONE_INDEX_HOST.

## Quick Start

```bash
# Verify auth and connectivity without making a request
pinecone-pp-cli doctor --dry-run

# List your indexes and their hosts for data-plane targeting
pinecone-pp-cli indexes list --json --select name,host,dimension

# Describe an index and resolve its data-plane host
pinecone-pp-cli indexes describe-index travel-chat-embeddings --json --select name,host,dimension

# Record point-in-time index state for later diffing
pinecone-pp-cli snapshot travel-chat-embeddings --note weekly

# See what changed in the index over the last week
pinecone-pp-cli snapshot diff --index travel-chat-embeddings --since 7d

# Sync index metadata into the local SQLite store
pinecone-pp-cli sync --resources indexes

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Search that works the way humans think
- **`text-query`** — Search a dense index with a natural-language query — embeds the text via Pinecone Inference, then queries the index — without managing vectors yourself.

  _Use when an agent needs semantic retrieval from an index but only has a text query, not a precomputed vector._

  ```bash
  pinecone-pp-cli text-query travel-chat-embeddings --text "visa on arrival for thailand" --top-k 5 --json
  ```
- **`cascade`** — Run one semantic query across multiple indexes and merge the ranked results (deduped by vector ID, best score wins) into a single agent-ready list.

  _Use when an agent needs to search across several indexes (e.g. prod vs staging, multiple tenants) in one shot._

  ```bash
  pinecone-pp-cli cascade --indexes travel-chat-embeddings,travel-chat-embeddings-v2 --text "kyoto itinerary" --top-k 3 --json
  ```

### Local state that compounds
- **`snapshot`** — Capture a point-in-time state of any index (per-namespace vector counts, dimension, metric, host) into local SQLite, then diff against prior snapshots to see exactly what changed.

  _Use when an agent needs to answer "what changed in this index since last week?" for drift, growth, or incident review._

  ```bash
  pinecone-pp-cli snapshot travel-chat-embeddings --note weekly
  ```
- **`usage`** — Compute vector-count growth and per-namespace distribution shifts from snapshot history, with a projection to a future horizon.

  _Use when an agent needs to forecast index growth or spot a namespace ballooning before it hits quota._

  ```bash
  pinecone-pp-cli usage --index travel-chat-embeddings --since 30d --json
  ```
- **`coverage`** — Join backups, backup schedules, and restore jobs into a per-index protection matrix showing which indexes are backed up, scheduled, and restorable.

  _Use when an agent audits disaster-recovery posture across an index fleet._

  ```bash
  pinecone-pp-cli coverage --json
  ```

### Memory lifecycle
- **`prune`** — Find vectors whose local metadata timestamps are older than a threshold and delete them in batches — dry-run by default, with --apply to commit.

  _Use when an agent maintains a long-lived memory store and needs to expire stale chunks without a full resync._

  ```bash
  pinecone-pp-cli prune travel-chat-embeddings --namespace __default__ --older-than 90d --apply
  ```
- **`check-vectors`** — Validate a vectors JSON file against the index schema — dimension, duplicate/empty IDs, sparse/dense shape — before you upsert and burn write units on a rejected batch.

  _Use when an agent ingests data and wants to catch dimension or ID errors before a costly rejected batch._

  ```bash
  pinecone-pp-cli check-vectors --index travel-chat-embeddings --file vectors.json --json
  ```

## Recipes

### Semantic search over chat history

```bash
pinecone-pp-cli text-query travel-chat-embeddings --text "what did the group decide about kyoto" --top-k 5 --select matches.id,matches.metadata.sender,matches.score
```

Ask a natural-language question and get scored, deduped hits with metadata — no embedding step

### Weekly index health snapshot

```bash
pinecone-pp-cli snapshot travel-chat-embeddings --note weekly && pinecone-pp-cli snapshot diff --index travel-chat-embeddings --since 7d
```

Record index state weekly, then see exactly what changed: vector counts, namespaces, config drift

### Stale memory pruning dry-run

```bash
pinecone-pp-cli prune travel-chat-embeddings --namespace __default__ --older-than 90d
```

Preview which vectors would be deleted by age before committing with --apply

### Cascade search across environments

```bash
pinecone-pp-cli cascade --indexes travel-chat-embeddings,travel-chat-embeddings-v2 --text "trip itinerary" --top-k 3 --json
```

Search prod and staging indexes in one call and get a single merged, deduped ranked list

### Validate before you upsert

```bash
pinecone-pp-cli check-vectors --index travel-chat-embeddings --file vectors.json --json
```

Catch dimension and ID errors in a batch before it burns write units on a rejected upsert

### Agent-ready index inventory

```bash
pinecone-pp-cli indexes list --json --select name,host,dimension,spec.serverless.region | jq '.results[] | {name, host, dimension, region: .spec.serverless.region}'
```

Narrow a large response with --select and pipe to jq for a compact inventory

## Usage

Run `pinecone-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PINECONE_CONFIG_DIR`, `PINECONE_DATA_DIR`, `PINECONE_STATE_DIR`, or `PINECONE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PINECONE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PINECONE_HOME=/srv/pinecone
pinecone-pp-cli doctor
```

Under `PINECONE_HOME=/srv/pinecone`, the four dirs resolve to `/srv/pinecone/config`, `/srv/pinecone/data`, `/srv/pinecone/state`, and `/srv/pinecone/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "pinecone": {
      "command": "pinecone-pp-mcp",
      "env": {
        "PINECONE_HOME": "/srv/pinecone"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PINECONE_DATA_DIR` overrides an explicit `--home` for that kind. Use `PINECONE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PINECONE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `pinecone-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### admin

Manage admin

- **`pinecone-pp-cli admin create-api-key`** - Create an API key for a project to authenticate Data Plane and Control Plane requests.
- **`pinecone-pp-cli admin create-invite`** - Invite a user to the organization by email and grant their initial role bindings.
- **`pinecone-pp-cli admin create-project`** - Create a new project.
- **`pinecone-pp-cli admin create-role-binding`** - Grant a role to a principal at an organization or project scope.
- **`pinecone-pp-cli admin create-service-account`** - Create a service account with optional initial role bindings; the client secret is returned only once.
- **`pinecone-pp-cli admin delete-api-key`** - Delete an API key from a project.
- **`pinecone-pp-cli admin delete-invite`** - Delete a pending or expired invite and its role bindings; to remove an accepted user, delete the user instead.
- **`pinecone-pp-cli admin delete-organization`** - Delete an organization and all its configuration; delete all its projects first.
- **`pinecone-pp-cli admin delete-project`** - Delete a project and all its configuration; delete its indexes, assistants, backups, and collections first.
- **`pinecone-pp-cli admin delete-role-binding`** - Delete a role binding; permissions are revoked when the deletion completes.
- **`pinecone-pp-cli admin delete-service-account`** - Delete a service account and its role bindings; tokens it minted are revoked within a few seconds.
- **`pinecone-pp-cli admin delete-user`** - Remove a user from the organization and revoke their role bindings; their Pinecone account is not deleted.
- **`pinecone-pp-cli admin fetch-api-key`** - Get an API key's details, excluding its secret.
- **`pinecone-pp-cli admin fetch-invite`** - Get an invite in the caller's organization by ID.
- **`pinecone-pp-cli admin fetch-organization`** - Get an organization's details.
- **`pinecone-pp-cli admin fetch-project`** - Get a project's details.
- **`pinecone-pp-cli admin fetch-role-binding`** - Get a role binding in the caller's organization by ID.
- **`pinecone-pp-cli admin fetch-service-account`** - Get a service account by ID; the client secret is returned only from create and rotate-secret requests.
- **`pinecone-pp-cli admin fetch-user`** - Get a user in the caller's organization by ID.
- **`pinecone-pp-cli admin list-invites`** - List pending and expired invites in the caller's organization.
- **`pinecone-pp-cli admin list-organizations`** - List all organizations associated with an account.
- **`pinecone-pp-cli admin list-project-api-keys`** - List all API keys in a project.
- **`pinecone-pp-cli admin list-projects`** - List all projects in an organization.
- **`pinecone-pp-cli admin list-role-bindings`** - List role bindings in the caller's organization, optionally filtered by principal, resource, and role.
- **`pinecone-pp-cli admin list-service-accounts`** - List service accounts in the caller's organization.
- **`pinecone-pp-cli admin list-users`** - List users in the caller's organization, optionally filtered by email address.
- **`pinecone-pp-cli admin resend-invite`** - Resend the invite email and extend its expiration to 7 days from now; limited to 100 emails per hour per organization.
- **`pinecone-pp-cli admin rotate-service-account-secret`** - Rotate a service account's OAuth client secret; the previous secret and its tokens are revoked within seconds and the new secret is returned only once.
- **`pinecone-pp-cli admin update-api-key`** - Update an API key's name and roles.
- **`pinecone-pp-cli admin update-organization`** - Update an organization's name.
- **`pinecone-pp-cli admin update-project`** - Update a project's name, maximum number of Pods, or customer-managed encryption key (CMEK).
- **`pinecone-pp-cli admin update-service-account`** - Update a service account's name; role bindings are managed through the role-binding endpoints.

### assistants

Manage assistants

- **`pinecone-pp-cli assistants create`** - Create an assistant. This is where you specify the underlying training model, which cloud provider you would like to deploy with, and more.

For guidance and examples, see [Create an assistant](https://docs.pinecone.io/guides/assistant/create-assistant)
- **`pinecone-pp-cli assistants delete`** - Delete an existing assistant.

For guidance and examples, see [Manage assistants](https://docs.pinecone.io/guides/assistant/manage-assistants#delete-an-assistant)
- **`pinecone-pp-cli assistants get`** - Get the status of an assistant.

For guidance and examples, see [Manage assistants](https://docs.pinecone.io/guides/assistant/manage-assistants#get-the-status-of-an-assistant)
- **`pinecone-pp-cli assistants list`** - List of all assistants in a project.

For guidance and examples, see [Manage assistants](https://docs.pinecone.io/guides/assistant/manage-assistants#list-assistants-for-a-project).
- **`pinecone-pp-cli assistants update`** - Update an existing assistant. You can modify the assistant's instructions.

For guidance and examples, see [Manage assistants](https://docs.pinecone.io/guides/assistant/manage-assistants#add-instructions-to-an-assistant).

### backup-schedules

Manage backup schedules

- **`pinecone-pp-cli backup-schedules delete`** - Permanently remove a backup schedule.
- **`pinecone-pp-cli backup-schedules describe`** - Get a single backup schedule by ID.
- **`pinecone-pp-cli backup-schedules update`** - Update frequency, retention, or enabled state for a backup schedule.
Re-enabling a disabled schedule (`enabled: true`) enqueues a new backup operation.

### backups

Manage backups

- **`pinecone-pp-cli backups delete`** - Delete a backup.
- **`pinecone-pp-cli backups describe`** - Get a description of a backup.
- **`pinecone-pp-cli backups list-project`** - List backups for all indexes in a project

### bulk

Manage bulk

- **`pinecone-pp-cli bulk cancel-import`** - Cancel an import operation if it is not yet finished. It has no effect if the operation is already finished.

For guidance and examples, see [Import data](https://docs.pinecone.io/guides/index-data/import-data).
- **`pinecone-pp-cli bulk describe-import`** - Return details of a specific import operation.

For guidance and examples, see [Import data](https://docs.pinecone.io/guides/index-data/import-data).
- **`pinecone-pp-cli bulk list-imports`** - List all recent and ongoing import operations.

By default, `list_imports` returns up to 100 imports per page. If the `limit` parameter is set, `list` returns up to that number of imports instead. Whenever there are additional IDs to return, the response also includes a `pagination_token` that you can use to get the next batch of imports. When the response does not include a `pagination_token`, there are no more imports to return.

For guidance and examples, see [Import data](https://docs.pinecone.io/guides/index-data/import-data).
- **`pinecone-pp-cli bulk start-import`** - Start an asynchronous import of vectors from object storage into an index.

For guidance and examples, see [Import data](https://docs.pinecone.io/guides/index-data/import-data).

### chat

Manage chat

- **`pinecone-pp-cli chat <assistant_name>`** - Chat with an assistant and get back citations in structured form. 

This is the recommended way to chat with an assistant, as it offers more functionality and control over the assistant's responses and references than the OpenAI-compatible chat interface.

For guidance and examples, see [Chat with an assistant](https://docs.pinecone.io/guides/assistant/chat-with-assistant).

### collections

Manage collections

- **`pinecone-pp-cli collections create`** - Create a Pinecone collection.
  
Serverless indexes do not support collections.
- **`pinecone-pp-cli collections delete`** - Delete an existing collection.
Serverless indexes do not support collections.
- **`pinecone-pp-cli collections describe`** - Get a description of a collection.
Serverless indexes do not support collections.
- **`pinecone-pp-cli collections list`** - List all collections in a project.
Serverless indexes do not support collections.

### describe-index-stats

Manage describe index stats

- **`pinecone-pp-cli describe-index-stats`** - Return statistics about the contents of an index, including the vector count per namespace, the number of dimensions, and the index fullness.

Serverless indexes scale automatically as needed, so index fullness is relevant only for pod-based indexes.

### embed

Manage embed

- **`pinecone-pp-cli embed`** - Generate vector embeddings for input data. This endpoint uses Pinecone's [hosted embedding models](https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models).

### files

Manage files

- **`pinecone-pp-cli files delete`** - [Delete an uploaded file](https://docs.pinecone.io/guides/assistant/manage-files#delete-a-file) from an assistant.

This operation is asynchronous. The response includes an operation ID that can be used to poll for completion via the describe operation endpoint.
- **`pinecone-pp-cli files describe`** - [Get the current status and metadata of a file](https://docs.pinecone.io/guides/assistant/manage-files#get-the-status-of-a-file) uploaded to an assistant.
- **`pinecone-pp-cli files list`** - List all files in an assistant, with an option to filter files with metadata.

For guidance and examples, see [Manage files](https://docs.pinecone.io/guides/assistant/manage-files#list-files-in-an-assistant).
- **`pinecone-pp-cli files upload`** - Upload a file to the specified assistant.

An identifier will be generated. To specify a file identifier or to replace file content, use the upsert endpoint (`PUT /files/{assistant_name}/{assistant_file_id}`).

This operation is asynchronous. The response includes an operation ID that can be used to poll for completion via the describe operation endpoint.

For guidance and examples, see [Manage files](https://docs.pinecone.io/guides/assistant/manage-files#upload-a-local-file).
- **`pinecone-pp-cli files upsert`** - Create or replace a file in the specified assistant. If a file with the given `assistant_file_id` already exists, it will be replaced with the new file. If it doesn't exist, a new file will be created with that identifier.

This operation is asynchronous. The file processing will occur in the background.

For guidance and examples, see [Manage files](https://docs.pinecone.io/guides/assistant/manage-files#upload-a-local-file).

### indexes

Manage indexes

- **`pinecone-pp-cli indexes configure-index`** - Configure an existing index. For guidance and examples, see [Manage indexes](https://docs.pinecone.io/guides/manage-data/manage-indexes).
- **`pinecone-pp-cli indexes create-index`** - Create a Pinecone index. This is where you specify the measure of similarity, the dimension of vectors to be stored in the index, which cloud provider you would like to deploy with, and more.
To restore from a backup, set `spec.serverless.source_backup_id` and specify the target `cloud` and `region`. Same-cloud cross-region restore is supported when available for the backup's source region. Cross-cloud restore is not supported.
For guidance and examples, see [Create an index](https://docs.pinecone.io/guides/index-data/create-an-index).
- **`pinecone-pp-cli indexes create-index-for-model`** - Create an index with integrated embedding.
With this type of index, you provide source text, and  Pinecone uses a [hosted embedding model](https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models)  to convert the text automatically during [upsert](https://docs.pinecone.io/reference/api/2026-04/data-plane/upsert_records)  and [search](https://docs.pinecone.io/reference/api/2026-04/data-plane/search_records).  
For guidance and examples, see [Create an index](https://docs.pinecone.io/guides/index-data/create-an-index#integrated-embedding).
- **`pinecone-pp-cli indexes delete-index`** - Delete an existing index.
- **`pinecone-pp-cli indexes describe-index`** - Get a description of an index.
- **`pinecone-pp-cli indexes list`** - List all indexes in a project.

### metrics

Endpoints for accessing database metrics.

- **`pinecone-pp-cli metrics <project_id>`** - Get endpoints for Prometheus scraping.

### models

Manage models

- **`pinecone-pp-cli models get`** - Get a description of a model hosted by Pinecone. 

You can use hosted models as an integrated part of Pinecone operations or for standalone embedding and reranking. For more details, see [Vector embedding](https://docs.pinecone.io/guides/index-data/indexing-overview#vector-embedding) and [Rerank results](https://docs.pinecone.io/guides/search/rerank-results).
- **`pinecone-pp-cli models list`** - List the embedding and reranking models hosted by Pinecone. 

You can use hosted models as an integrated part of Pinecone operations or for standalone embedding and reranking. For more details, see [Vector embedding](https://docs.pinecone.io/guides/index-data/indexing-overview#vector-embedding) and [Rerank results](https://docs.pinecone.io/guides/search/rerank-results).

### namespaces

Manage namespaces

- **`pinecone-pp-cli namespaces create`** - Create a namespace in a serverless index.

For guidance and examples, see [Manage namespaces](https://docs.pinecone.io/guides/manage-data/manage-namespaces).

**Note:** This operation is not supported for pod-based indexes.
- **`pinecone-pp-cli namespaces delete`** - Delete a namespace from a serverless index. Deleting a namespace is irreversible; all data in the namespace is permanently deleted.

For guidance and examples, see [Manage namespaces](https://docs.pinecone.io/guides/manage-data/manage-namespaces).

**Note:** This operation is not supported for pod-based indexes.
- **`pinecone-pp-cli namespaces describe`** - Describe a namespace in a serverless index, including the total number of vectors in the namespace.

For guidance and examples, see [Manage namespaces](https://docs.pinecone.io/guides/manage-data/manage-namespaces).

**Note:** This operation is not supported for pod-based indexes.
- **`pinecone-pp-cli namespaces list-operation`** - List all namespaces in a serverless index.

Up to 100 namespaces are returned at a time by default, in sorted order (bitwise “C” collation). If the `limit` parameter is set, up to that number of namespaces are returned instead. Whenever there are additional namespaces to return, the response also includes a `pagination_token` that you can use to get the next batch of namespaces. When the response does not include a `pagination_token`, there are no more namespaces to return.

For guidance and examples, see [Manage namespaces](https://docs.pinecone.io/guides/manage-data/manage-namespaces).

**Note:** This operation is not supported for pod-based indexes.

### oauth

Authentication using the OAuth2 protocol.

- **`pinecone-pp-cli oauth`** - Obtain an access token for a service account using the OAuth2 client credentials flow. An access token is needed to authorize requests to the Pinecone Admin API.
The host domain for OAuth endpoints is `login.pinecone.io`.

### operations

Manage operations

- **`pinecone-pp-cli operations describe`** - Get the status of an operation.
- **`pinecone-pp-cli operations list`** - List all operations for an assistant. Returns operations that are in progress, as well as recently completed or failed operations.
Both successful and failed operations are retained for 30 days after completion.
Use the `operation_type` and `status` query parameters to filter results.

### query

Manage query

- **`pinecone-pp-cli query`** - Search a namespace using a query vector. It retrieves the ids of the most similar items in a namespace, along with their similarity scores.

For guidance, examples, and limits, see [Search](https://docs.pinecone.io/guides/search/search-overview).

### records

Manage records

- **`pinecone-pp-cli records search-namespace`** - Search a namespace with a query text, query vector, or record ID and return the most similar records, along with their similarity scores. Optionally, rerank the initial results based on their relevance to the query. 

Searching with text is supported only for indexes with [integrated embedding](https://docs.pinecone.io/guides/index-data/indexing-overview#vector-embedding). Searching with a query vector or record ID is supported for all indexes. 

For guidance and examples, see [Search](https://docs.pinecone.io/guides/search/search-overview).
- **`pinecone-pp-cli records upsert-namespace`** - Upsert text into a namespace. Pinecone converts the text to vectors automatically using the hosted embedding model associated with the index.

Upserting text is supported only for [indexes with integrated embedding](https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models).

For guidance, examples, and limits, see [Upsert data](https://docs.pinecone.io/guides/index-data/upsert-data).

### rerank

Manage rerank

- **`pinecone-pp-cli rerank`** - Rerank results according to their relevance to a query.

For guidance and examples, see [Rerank results](https://docs.pinecone.io/guides/search/rerank-results).

### restore-jobs

Manage restore jobs

- **`pinecone-pp-cli restore-jobs describe`** - Get a description of a restore job.
- **`pinecone-pp-cli restore-jobs list`** - List all restore jobs for a project.

### vectors

Manage vectors

- **`pinecone-pp-cli vectors delete`** - Delete records by id or by metadata from a single namespace.

For guidance and examples, see [Delete data](https://docs.pinecone.io/guides/manage-data/delete-data).
- **`pinecone-pp-cli vectors fetch`** - Look up and return records by ID from a single namespace. The returned records include the vector data and/or metadata.
For on-demand indexes, since vector values are retrieved from object storage, fetch operations may have increased latency. If you only need metadata or IDs, consider using the query operation with `includeValues` set to `false` instead.
For guidance and examples, see [Fetch data](https://docs.pinecone.io/guides/manage-data/fetch-data).
- **`pinecone-pp-cli vectors fetch-by-metadata`** - Look up and return records by metadata from a single namespace. The returned records include the vector data and metadata.
For guidance and examples, see [Fetch data](https://docs.pinecone.io/guides/manage-data/fetch-data).
- **`pinecone-pp-cli vectors list`** - List the IDs of records in a single namespace of a serverless index. An optional prefix can be passed to limit the results to IDs with a common prefix.

Returns up to 100 IDs at a time by default in sorted order (bitwise "C" collation). If the `limit` parameter is set, `list` returns up to that number of IDs instead. Whenever there are additional IDs to return, the response also includes a `pagination_token` that you can use to get the next batch of IDs. When the response does not include a `pagination_token`, there are no more IDs to return.

For guidance and examples, see [List record IDs](https://docs.pinecone.io/guides/manage-data/list-record-ids).

**Note:** `list` is supported only for serverless indexes.
- **`pinecone-pp-cli vectors update`** - Update records by ID or by metadata in a namespace. Updating by ID changes the vector and/or metadata of a single record. Updating by metadata changes metadata across multiple records using a metadata filter.
If a vector value is included, it will overwrite the previous value. If `set_metadata` is included, only the specified metadata fields are modified, and if a specified metadata field does not exist, it is added.
For guidance and examples, see [Update data](https://docs.pinecone.io/guides/manage-data/update-data).
- **`pinecone-pp-cli vectors upsert`** - Upsert records into a namespace. If a new value is upserted for an existing record ID, it will overwrite the previous value.

For guidance, examples, and limits, see [Upsert data](https://docs.pinecone.io/guides/index-data/upsert-data).


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`pinecone-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`pinecone-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`pinecone-pp-cli learnings list`** - Inspect taught rows
- **`pinecone-pp-cli learnings forget <query>`** - Undo a teach
- **`pinecone-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`pinecone-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`pinecone-pp-cli teach-pattern`** - Install a query/resource template up front
- **`pinecone-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PINECONE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `pinecone-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pinecone-pp-cli assistants list --x-pinecone-api-version example-value

# JSON for scripting and agents
pinecone-pp-cli assistants list --x-pinecone-api-version example-value --json
# Filter to specific fields
pinecone-pp-cli assistants list --x-pinecone-api-version example-value --json --select created_at,host,instructions

# Dry run — show the request without sending
pinecone-pp-cli assistants list --x-pinecone-api-version example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pinecone-pp-cli assistants list --x-pinecone-api-version example-value --agent
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
- `PINECONE_ASSISTANT_HOST` resolves `{assistant_host}`
- `PINECONE_INDEX_HOST` resolves `{index_host}`
- `PINECONE_SCHEDULE_ID` resolves `{schedule_id}`

Base URL: `https://api.pinecone.io`

## Health Check

```bash
pinecone-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `pinecone-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/pinecone-pp-cli/config.toml`; `--home`, `PINECONE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PINECONE_ASSISTANT_HOST` | endpoint | Yes |  |
| `PINECONE_INDEX_HOST` | endpoint | Yes |  |
| `PINECONE_SCHEDULE_ID` | endpoint | Yes |  |
| `PINECONE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `pinecone-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pinecone-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PINECONE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Missing api-key header** — Set PINECONE_API_KEY=<key> or run `pinecone-pp-cli auth set-token <key>`
- **429 TOO_MANY_REQUESTS (rate limit)** — Retry with exponential backoff; the CLI honors --rate-limit to pace requests
- **422 unknown field in request body** — Check the endpoint version header; pass --x-pinecone-api-version 2026-04
- **403 FORBIDDEN (quota/deletion protection)** — Check plan quota or disable deletion protection on the index

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**pinecone-io/cli**](https://github.com/pinecone-io/cli) — Go (22 stars)
- [**@pinecone-database/mcp**](https://github.com/pinecone-io/pinecone-mcp-server) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

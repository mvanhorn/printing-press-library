# Pinecone CLI

**Every Pinecone API endpoint, plus cross-index search, namespace drift detection, and offline cost estimation.**

Full coverage of Pinecone's control plane (indexes, collections, backups), data plane (query, upsert, fetch, delete), and inference (embed, rerank) — plus cross-index search, namespace drift detection, bulk-delete with dry-run, and an offline cost estimator. Built on a synced local cache so cron-driven monitors and AI agents can answer 'has this namespace grown today?' without re-running describe-stats every minute.

## Install

The recommended path installs both the `pinecone-pp-cli` binary and the `pp-pinecone` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install pinecone
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install pinecone --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pinecone-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pinecone --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pinecone --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-pinecone skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-pinecone. The skill defines how its required CLI can be installed.
```

## Authentication

Pinecone uses API-key auth. Set PINECONE_API_KEY in your environment or run `pinecone-pp-cli auth set-token`. The control plane uses https://api.pinecone.io; data-plane endpoints (query, upsert) use per-index hosts returned by `describe_index` — the CLI resolves these automatically.

## Quick Start

```bash
# Validates PINECONE_API_KEY and API reachability.
pinecone-pp-cli doctor --json


# Lists every index with dimension/metric/host.
pinecone-pp-cli inventory --json


# Per-namespace counts across all indexes.
pinecone-pp-cli ns-stats --json


# Single-index search via the cross-index command.
pinecone-pp-cli xindex --indexes client-impact --query 'time tracking' --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`ns-stats`** — Per-namespace record counts and size across all your indexes, in one query.

  _Use this monthly to find namespaces that have grown out of expected bounds before they hit quota._

  ```bash
  pinecone-pp-cli ns-stats --json --select index,namespace,record_count,vector_count
  ```
- **`csearch`** — Cascading search across multiple namespaces with local result merging and rerank.

  _Use this when you need cross-namespace results without paying for re-querying every time._

  ```bash
  pinecone-pp-cli csearch --index client-impact --namespaces features,tasks --query 'time tracking sync' --json
  ```
- **`drift`** — Compares the last cached describe_index_stats vs current — surfaces namespaces that grew/shrank significantly.

  _Use this for daily health checks to catch unexpected ingestion stalls or growth spikes._

  ```bash
  pinecone-pp-cli drift --json --threshold 100
  ```

### Agent-native plumbing
- **`inventory`** — All indexes with their dimension, metric, host, status, and last-modified time.

  _Use this in CI to verify expected indexes exist before running vector operations._

  ```bash
  pinecone-pp-cli inventory --json
  ```
- **`doctor-deep`** — Verifies API key works AND each index's data-plane host is reachable.

  _Use this in CI/cron to catch host-routing issues before they break data-plane operations._

  ```bash
  pinecone-pp-cli doctor-deep --json
  ```
- **`purge`** — Delete vectors matching a metadata filter across one or many namespaces with --dry-run preview.

  _Use this for cleanup with a confirmation step — destructive operations should never be one-step._

  ```bash
  pinecone-pp-cli purge --index client-impact --namespace tasks --filter '{"category":"obsolete"}' --dry-run --json
  ```
- **`estimate`** — Estimates inference cost for a batch of texts before submitting (input-token count × model price).

  _Use this before bulk embedding jobs to avoid surprise bills._

  ```bash
  pinecone-pp-cli estimate --model llama-text-embed-v2 --file inputs.txt --json
  ```
- **`xindex`** — Run the same query against multiple indexes in parallel, merge and rerank.

  _Use this when an entity could live in any of several indexes — single command, parallel queries._

  ```bash
  pinecone-pp-cli xindex --indexes client-impact,features --query 'time tracking' --top-k 5 --json
  ```

## Usage

Run `pinecone-pp-cli --help` for the full command reference and flag list.

## Commands

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

- **`pinecone-pp-cli describe-index-stats describe_index_stats`** - Return statistics about the contents of an index, including the vector count per namespace, the number of dimensions, and the index fullness.

Serverless indexes scale automatically as needed, so index fullness is relevant only for pod-based indexes.

### embed

Manage embed

- **`pinecone-pp-cli embed embed`** - Generate vector embeddings for input data. This endpoint uses Pinecone's [hosted embedding models](https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models).

### indexes

Manage indexes

- **`pinecone-pp-cli indexes configure-index`** - Configure an existing index. For guidance and examples, see [Manage indexes](https://docs.pinecone.io/guides/manage-data/manage-indexes).
- **`pinecone-pp-cli indexes create-index`** - Create a Pinecone index. This is where you specify the measure of similarity, the dimension of vectors to be stored in the index, which cloud provider you would like to deploy with, and more.
  
For guidance and examples, see [Create an index](https://docs.pinecone.io/guides/index-data/create-an-index).
- **`pinecone-pp-cli indexes create-index-for-model`** - Create an index with integrated embedding.
With this type of index, you provide source text, and  Pinecone uses a [hosted embedding model](https://docs.pinecone.io/guides/index-data/create-an-index#embedding-models)  to convert the text automatically during [upsert](https://docs.pinecone.io/reference/api/2026-04/data-plane/upsert_records)  and [search](https://docs.pinecone.io/reference/api/2026-04/data-plane/search_records).  
For guidance and examples, see [Create an index](https://docs.pinecone.io/guides/index-data/create-an-index#integrated-embedding).
- **`pinecone-pp-cli indexes delete-index`** - Delete an existing index.
- **`pinecone-pp-cli indexes describe-index`** - Get a description of an index.
- **`pinecone-pp-cli indexes list`** - List all indexes in a project.

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

### query

Manage query

- **`pinecone-pp-cli query vectors`** - Search a namespace using a query vector. It retrieves the ids of the most similar items in a namespace, along with their similarity scores.

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

- **`pinecone-pp-cli rerank rerank`** - Rerank results according to their relevance to a query.

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


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pinecone-pp-cli collections list

# JSON for scripting and agents
pinecone-pp-cli collections list --json

# Filter to specific fields
pinecone-pp-cli collections list --json --select id,name,status

# Dry run — show the request without sending
pinecone-pp-cli collections list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pinecone-pp-cli collections list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-pinecone -g
```

Then invoke `/pp-pinecone <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add pinecone pinecone-pp-mcp -e PINECONE_API_KEY=<your-key>
```

</details>

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


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pinecone": {
      "command": "pinecone-pp-mcp",
      "env": {
        "PINECONE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
pinecone-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/pinecone-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PINECONE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pinecone-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PINECONE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized** — Check PINECONE_API_KEY is set (`pinecone-pp-cli auth status`). Get a key from app.pinecone.io.
- **404 on /query or /vectors/*** — Data-plane endpoints need the per-index host. Run `pinecone-pp-cli inventory` to see hosts; the CLI auto-resolves but you can override with `--host`.
- **drift returns nothing** — Drift needs at least one prior `ns-stats` snapshot to compare against. Run `ns-stats` daily to seed the local cache.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Official Pinecone MCP**](https://github.com/pinecone-io/pinecone-mcp) — TypeScript
- [**pinecone-io/pinecone-ts-client**](https://github.com/pinecone-io/pinecone-ts-client) — TypeScript
- [**pinecone-io/pinecone-python-client**](https://github.com/pinecone-io/pinecone-python-client) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

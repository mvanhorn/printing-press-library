---
name: pp-pinecone
description: "Every Pinecone API endpoint, plus cross-index search, namespace drift detection, and offline cost estimation. Trigger phrases: `show pinecone namespaces`, `pinecone health check`, `search across pinecone indexes`, `estimate pinecone embedding cost`, `pinecone drift`, `use pinecone-pp`, `run pinecone-pp`."
author: "Dan Bronson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pinecone-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/pinecone/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Pinecone — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pinecone-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install pinecone --cli-only
   ```
2. Verify: `pinecone-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Full coverage of Pinecone's control plane (indexes, collections, backups), data plane (query, upsert, fetch, delete), and inference (embed, rerank) — plus cross-index search, namespace drift detection, bulk-delete with dry-run, and an offline cost estimator. Built on a synced local cache so cron-driven monitors and AI agents can answer 'has this namespace grown today?' without re-running describe-stats every minute.

## When to Use This CLI

Reach for pinecone-pp-cli when you need to operate Pinecone programmatically — namespace audits, cross-index search, drift detection, embedding-cost estimation, or bulk cleanup with dry-run safety. For one-off CRUD on a single index, the official Python/TS SDKs are fine; use this CLI when local cache, multi-index ops, or cron-driven monitoring matter.

## Unique Capabilities

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

## Command Reference

**backups** — Manage backups

- `pinecone-pp-cli backups delete` — Delete a backup.
- `pinecone-pp-cli backups describe` — Get a description of a backup.
- `pinecone-pp-cli backups list-project` — List backups for all indexes in a project

**bulk** — Manage bulk

- `pinecone-pp-cli bulk cancel-import` — Cancel an import operation if it is not yet finished. It has no effect if the operation is already finished. For...
- `pinecone-pp-cli bulk describe-import` — Return details of a specific import operation. For guidance and examples, see [Import...
- `pinecone-pp-cli bulk list-imports` — List all recent and ongoing import operations. By default, `list_imports` returns up to 100 imports per page. If the...
- `pinecone-pp-cli bulk start-import` — Start an asynchronous import of vectors from object storage into an index. For guidance and examples, see [Import...

**collections** — Manage collections

- `pinecone-pp-cli collections create` — Create a Pinecone collection. Serverless indexes do not support collections.
- `pinecone-pp-cli collections delete` — Delete an existing collection. Serverless indexes do not support collections.
- `pinecone-pp-cli collections describe` — Get a description of a collection. Serverless indexes do not support collections.
- `pinecone-pp-cli collections list` — List all collections in a project. Serverless indexes do not support collections.

**describe-index-stats** — Manage describe index stats

- `pinecone-pp-cli describe-index-stats` — Return statistics about the contents of an index, including the vector count per namespace, the number of...

**embed** — Manage embed

- `pinecone-pp-cli embed` — Generate vector embeddings for input data. This endpoint uses Pinecone's [hosted embedding...

**indexes** — Manage indexes

- `pinecone-pp-cli indexes configure-index` — Configure an existing index. For guidance and examples, see [Manage...
- `pinecone-pp-cli indexes create-index` — Create a Pinecone index. This is where you specify the measure of similarity, the dimension of vectors to be stored...
- `pinecone-pp-cli indexes create-index-for-model` — Create an index with integrated embedding. With this type of index, you provide source text, and Pinecone uses a...
- `pinecone-pp-cli indexes delete-index` — Delete an existing index.
- `pinecone-pp-cli indexes describe-index` — Get a description of an index.
- `pinecone-pp-cli indexes list` — List all indexes in a project.

**models** — Manage models

- `pinecone-pp-cli models get` — Get a description of a model hosted by Pinecone. You can use hosted models as an integrated part of Pinecone...
- `pinecone-pp-cli models list` — List the embedding and reranking models hosted by Pinecone. You can use hosted models as an integrated part of...

**namespaces** — Manage namespaces

- `pinecone-pp-cli namespaces create` — Create a namespace in a serverless index. For guidance and examples, see [Manage...
- `pinecone-pp-cli namespaces delete` — Delete a namespace from a serverless index. Deleting a namespace is irreversible; all data in the namespace is...
- `pinecone-pp-cli namespaces describe` — Describe a namespace in a serverless index, including the total number of vectors in the namespace. For guidance and...
- `pinecone-pp-cli namespaces list-operation` — List all namespaces in a serverless index. Up to 100 namespaces are returned at a time by default, in sorted order...

**query** — Manage query

- `pinecone-pp-cli query` — Search a namespace using a query vector. It retrieves the ids of the most similar items in a namespace, along with...

**records** — Manage records

- `pinecone-pp-cli records search-namespace` — Search a namespace with a query text, query vector, or record ID and return the most similar records, along with...
- `pinecone-pp-cli records upsert-namespace` — Upsert text into a namespace. Pinecone converts the text to vectors automatically using the hosted embedding model...

**rerank** — Manage rerank

- `pinecone-pp-cli rerank` — Rerank results according to their relevance to a query. For guidance and examples, see [Rerank...

**restore-jobs** — Manage restore jobs

- `pinecone-pp-cli restore-jobs describe` — Get a description of a restore job.
- `pinecone-pp-cli restore-jobs list` — List all restore jobs for a project.

**vectors** — Manage vectors

- `pinecone-pp-cli vectors delete` — Delete records by id or by metadata from a single namespace. For guidance and examples, see [Delete...
- `pinecone-pp-cli vectors fetch` — Look up and return records by ID from a single namespace. The returned records include the vector data and/or...
- `pinecone-pp-cli vectors fetch-by-metadata` — Look up and return records by metadata from a single namespace. The returned records include the vector data and...
- `pinecone-pp-cli vectors list` — List the IDs of records in a single namespace of a serverless index. An optional prefix can be passed to limit the...
- `pinecone-pp-cli vectors update` — Update records by ID or by metadata in a namespace. Updating by ID changes the vector and/or metadata of a single...
- `pinecone-pp-cli vectors upsert` — Upsert records into a namespace. If a new value is upserted for an existing record ID, it will overwrite the...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pinecone-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning namespace health check

```bash
pinecone-pp-cli ns-stats --json --select index,namespace,record_count,vector_count
```

Snapshots every namespace size across all indexes — pipe into your dashboard.

### Drift alert (cron)

```bash
pinecone-pp-cli drift --threshold 100 --json
```

Exits non-zero when any namespace changed by more than 100 records vs the last snapshot. Pair with `ns-stats` daily.

### Safe purge (deep --select preview)

```bash
pinecone-pp-cli purge --index client-impact --namespace features --filter @./filter.json --dry-run --json
```

Lists what would be deleted under the filter, with deep --select narrowing the metadata you actually want to see. Drop --dry-run to commit.

### Cross-index search

```bash
pinecone-pp-cli xindex --indexes client-impact,features --query 'sync' --top-k 5 --json --select index,id,score,metadata.title
```

Parallel query across indexes, merged + reranked locally.

### Cost estimate before batch embed

```bash
pinecone-pp-cli estimate --model llama-text-embed-v2 --file inputs.txt --json
```

Local tokenizer × published prices = estimated cost. No API hit.

## Auth Setup

Pinecone uses API-key auth. Set PINECONE_API_KEY in your environment or run `pinecone-pp-cli auth set-token`. The control plane uses https://api.pinecone.io; data-plane endpoints (query, upsert) use per-index hosts returned by `describe_index` — the CLI resolves these automatically.

Run `pinecone-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pinecone-pp-cli collections list --agent --select id,name,status
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
pinecone-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pinecone-pp-cli feedback --stdin < notes.txt
pinecone-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pinecone-pp-cli/feedback.jsonl`. They are never POSTed unless `PINECONE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PINECONE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pinecone-pp-cli profile save briefing --json
pinecone-pp-cli --profile briefing collections list
pinecone-pp-cli profile list --json
pinecone-pp-cli profile show briefing
pinecone-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pinecone-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add pinecone-pp-mcp -- pinecone-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pinecone-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pinecone-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pinecone-pp-cli <command> --help`.

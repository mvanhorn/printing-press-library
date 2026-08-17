---
name: pp-pinecone
description: "Every Pinecone API feature, plus local sync, snapshot history, and text-first search no other Pinecone tool has. Trigger phrases: `query my pinecone index`, `upsert vectors into pinecone`, `search pinecone embeddings`, `check pinecone index health`, `what changed in my pinecone index`, `use pinecone`, `run pinecone`."
author: "Som Samantray"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pinecone-pp-cli
    install:
      - kind: go
        bins: [pinecone-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/pinecone/cmd/pinecone-pp-cli
---

# Pinecone — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pinecone-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install pinecone --cli-only
   ```
2. Verify: `pinecone-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/pinecone/cmd/pinecone-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Pinecone CLI manages indexes, vectors, namespaces, backups, imports, inference, and admin — then goes further: sync index and record metadata into a local SQLite database for offline search, capture snapshot history to diff and project growth, and search by natural language without touching vectors. Agent-native --json/--select/csv output and typed exit codes make it scriptable in CI.

## When to Use This CLI

Use this CLI when you manage Pinecone indexes and vectors from a terminal or agent: create and scale indexes, ingest and query vectors, run semantic search over text, audit backup coverage, and track index growth over time. It is the right tool for RAG pipelines, embedding workflows, and index fleet administration.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for large bulk imports (10M+ records) — use object-storage import via the Pinecone console or SDK
- Do not use this CLI for real-time streaming ingestion at high throughput — use the Python/Go SDK with gRPC
- Do not use this CLI to manage non-Pinecone vector databases
- Do not use this CLI for integrated-embedding text search on indexes that do not have integrated inference configured

## Unique Capabilities

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

## Command Reference

**admin** — Manage admin

- `pinecone-pp-cli admin create-api-key` — Create an API key for a project to authenticate Data Plane and Control Plane requests.
- `pinecone-pp-cli admin create-invite` — Invite a user to the organization by email and grant their initial role bindings.
- `pinecone-pp-cli admin create-project` — Create a new project.
- `pinecone-pp-cli admin create-role-binding` — Grant a role to a principal at an organization or project scope.
- `pinecone-pp-cli admin create-service-account` — Create a service account with optional initial role bindings; the client secret is returned only once.
- `pinecone-pp-cli admin delete-api-key` — Delete an API key from a project.
- `pinecone-pp-cli admin delete-invite` — Delete a pending or expired invite and its role bindings; to remove an accepted user, delete the user instead.
- `pinecone-pp-cli admin delete-organization` — Delete an organization and all its configuration; delete all its projects first.
- `pinecone-pp-cli admin delete-project` — Delete a project and all its configuration; delete its indexes, assistants, backups, and collections first.
- `pinecone-pp-cli admin delete-role-binding` — Delete a role binding; permissions are revoked when the deletion completes.
- `pinecone-pp-cli admin delete-service-account` — Delete a service account and its role bindings; tokens it minted are revoked within a few seconds.
- `pinecone-pp-cli admin delete-user` — Remove a user from the organization and revoke their role bindings; their Pinecone account is not deleted.
- `pinecone-pp-cli admin fetch-api-key` — Get an API key's details, excluding its secret.
- `pinecone-pp-cli admin fetch-invite` — Get an invite in the caller's organization by ID.
- `pinecone-pp-cli admin fetch-organization` — Get an organization's details.
- `pinecone-pp-cli admin fetch-project` — Get a project's details.
- `pinecone-pp-cli admin fetch-role-binding` — Get a role binding in the caller's organization by ID.
- `pinecone-pp-cli admin fetch-service-account` — Get a service account by ID; the client secret is returned only from create and rotate-secret requests.
- `pinecone-pp-cli admin fetch-user` — Get a user in the caller's organization by ID.
- `pinecone-pp-cli admin list-invites` — List pending and expired invites in the caller's organization.
- `pinecone-pp-cli admin list-organizations` — List all organizations associated with an account.
- `pinecone-pp-cli admin list-project-api-keys` — List all API keys in a project.
- `pinecone-pp-cli admin list-projects` — List all projects in an organization.
- `pinecone-pp-cli admin list-role-bindings` — List role bindings in the caller's organization, optionally filtered by principal, resource, and role.
- `pinecone-pp-cli admin list-service-accounts` — List service accounts in the caller's organization.
- `pinecone-pp-cli admin list-users` — List users in the caller's organization, optionally filtered by email address.
- `pinecone-pp-cli admin resend-invite` — Resend the invite email and extend its expiration to 7 days from now; limited to 100 emails per hour per organization.
- `pinecone-pp-cli admin rotate-service-account-secret` — Rotate a service account's OAuth client secret
- `pinecone-pp-cli admin update-api-key` — Update an API key's name and roles.
- `pinecone-pp-cli admin update-organization` — Update an organization's name.
- `pinecone-pp-cli admin update-project` — Update a project's name, maximum number of Pods, or customer-managed encryption key (CMEK).
- `pinecone-pp-cli admin update-service-account` — Update a service account's name; role bindings are managed through the role-binding endpoints.

**assistants** — Manage assistants

- `pinecone-pp-cli assistants create` — Create an assistant.
- `pinecone-pp-cli assistants delete` — Delete an existing assistant. For guidance and examples, see [Manage assistants](https://docs.pinecone.
- `pinecone-pp-cli assistants get` — Get the status of an assistant. For guidance and examples, see [Manage assistants](https://docs.pinecone.
- `pinecone-pp-cli assistants list` — List of all assistants in a project. For guidance and examples, see [Manage assistants](https://docs.pinecone.
- `pinecone-pp-cli assistants update` — Update an existing assistant. You can modify the assistant's instructions.

**backup-schedules** — Manage backup schedules

- `pinecone-pp-cli backup-schedules delete` — Permanently remove a backup schedule.
- `pinecone-pp-cli backup-schedules describe` — Get a single backup schedule by ID.
- `pinecone-pp-cli backup-schedules update` — Update frequency, retention, or enabled state for a backup schedule.

**backups** — Manage backups

- `pinecone-pp-cli backups delete` — Delete a backup.
- `pinecone-pp-cli backups describe` — Get a description of a backup.
- `pinecone-pp-cli backups list-project` — List backups for all indexes in a project

**bulk** — Manage bulk

- `pinecone-pp-cli bulk cancel-import` — Cancel an import operation if it is not yet finished. It has no effect if the operation is already finished.
- `pinecone-pp-cli bulk describe-import` — Return details of a specific import operation. For guidance and examples, see [Import data](https://docs.pinecone.
- `pinecone-pp-cli bulk list-imports` — List all recent and ongoing import operations. By default, `list_imports` returns up to 100 imports per page.
- `pinecone-pp-cli bulk start-import` — Start an asynchronous import of vectors from object storage into an index.

**chat** — Manage chat

- `pinecone-pp-cli chat <assistant_name>` — Chat with an assistant and get back citations in structured form.

**collections** — Manage collections

- `pinecone-pp-cli collections create` — Create a Pinecone collection. Serverless indexes do not support collections.
- `pinecone-pp-cli collections delete` — Delete an existing collection. Serverless indexes do not support collections.
- `pinecone-pp-cli collections describe` — Get a description of a collection. Serverless indexes do not support collections.
- `pinecone-pp-cli collections list` — List all collections in a project. Serverless indexes do not support collections.

**describe-index-stats** — Manage describe index stats

- `pinecone-pp-cli describe-index-stats` — Return statistics about the contents of an index, including the vector count per namespace, the number of dimensions

**embed** — Manage embed

- `pinecone-pp-cli embed` — Generate vector embeddings for input data. This endpoint uses Pinecone's [hosted embedding models](https://docs.

**files** — Manage files

- `pinecone-pp-cli files delete` — [Delete an uploaded file](https://docs.pinecone.io/guides/assistant/manage-files#delete-a-file) from an assistant.
- `pinecone-pp-cli files describe` — [Get the current status and metadata of a file](https://docs.pinecone.
- `pinecone-pp-cli files list` — List all files in an assistant, with an option to filter files with metadata.
- `pinecone-pp-cli files upload` — Upload a file to the specified assistant. An identifier will be generated.
- `pinecone-pp-cli files upsert` — Create or replace a file in the specified assistant.

**indexes** — Manage indexes

- `pinecone-pp-cli indexes configure-index` — Configure an existing index. For guidance and examples, see [Manage indexes](https://docs.pinecone.
- `pinecone-pp-cli indexes create-index` — Create a Pinecone index.
- `pinecone-pp-cli indexes create-index-for-model` — Create an index with integrated embedding.
- `pinecone-pp-cli indexes delete-index` — Delete an existing index.
- `pinecone-pp-cli indexes describe-index` — Get a description of an index.
- `pinecone-pp-cli indexes list` — List all indexes in a project.

**metrics** — Endpoints for accessing database metrics.

- `pinecone-pp-cli metrics <project_id>` — Get endpoints for Prometheus scraping.

**models** — Manage models

- `pinecone-pp-cli models get` — Get a description of a model hosted by Pinecone.
- `pinecone-pp-cli models list` — List the embedding and reranking models hosted by Pinecone.

**namespaces** — Manage namespaces

- `pinecone-pp-cli namespaces create` — Create a namespace in a serverless index. For guidance and examples, see [Manage namespaces](https://docs.pinecone.
- `pinecone-pp-cli namespaces delete` — Delete a namespace from a serverless index.
- `pinecone-pp-cli namespaces describe` — Describe a namespace in a serverless index, including the total number of vectors in the namespace.
- `pinecone-pp-cli namespaces list-operation` — List all namespaces in a serverless index.

**oauth** — Authentication using the OAuth2 protocol.

- `pinecone-pp-cli oauth` — Obtain an access token for a service account using the OAuth2 client credentials flow.

**operations** — Manage operations

- `pinecone-pp-cli operations describe` — Get the status of an operation.
- `pinecone-pp-cli operations list` — List all operations for an assistant.

**query** — Manage query

- `pinecone-pp-cli query` — Search a namespace using a query vector.

**records** — Manage records

- `pinecone-pp-cli records search-namespace` — Search a namespace with a query text, query vector, or record ID and return the most similar records
- `pinecone-pp-cli records upsert-namespace` — Upsert text into a namespace.

**rerank** — Manage rerank

- `pinecone-pp-cli rerank` — Rerank results according to their relevance to a query. For guidance and examples, see [Rerank results](https://docs.

**restore-jobs** — Manage restore jobs

- `pinecone-pp-cli restore-jobs describe` — Get a description of a restore job.
- `pinecone-pp-cli restore-jobs list` — List all restore jobs for a project.

**vectors** — Manage vectors

- `pinecone-pp-cli vectors delete` — Delete records by id or by metadata from a single namespace. For guidance and examples, see [Delete data](https://docs.
- `pinecone-pp-cli vectors fetch` — Look up and return records by ID from a single namespace. The returned records include the vector data and/or metadata.
- `pinecone-pp-cli vectors fetch-by-metadata` — Look up and return records by metadata from a single namespace.
- `pinecone-pp-cli vectors list` — List the IDs of records in a single namespace of a serverless index.
- `pinecone-pp-cli vectors update` — Update records by ID or by metadata in a namespace.
- `pinecone-pp-cli vectors upsert` — Upsert records into a namespace.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pinecone-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Pinecone uses an API key sent as the `Api-Key` header, with the required `X-Pinecone-Api-Version` header set to the API version (2026-04). Set PINECONE_API_KEY=<key> or run `pinecone-pp-cli auth set-token`. Data-plane commands target the per-index host, which the CLI resolves automatically from `indexes describe-index` when you pass --index, or via PINECONE_INDEX_HOST.

Run `pinecone-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pinecone-pp-cli assistants list --x-pinecone-api-version example-value --agent --select created_at,host,instructions
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

- Use `--home <dir>` for one invocation, or set `PINECONE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `PINECONE_CONFIG_DIR`, `PINECONE_DATA_DIR`, `PINECONE_STATE_DIR`, `PINECONE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `PINECONE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `pinecone-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `PINECONE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `PINECONE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
pinecone-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "pinecone-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `pinecone-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `pinecone-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `pinecone-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
pinecone-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
pinecone-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
pinecone-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
pinecone-pp-cli playbook amend \
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

`pinecone-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `PINECONE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
pinecone-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pinecone-pp-cli feedback --stdin < notes.txt
pinecone-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `PINECONE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PINECONE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pinecone-pp-cli profile save briefing --json
pinecone-pp-cli --profile briefing assistants list --x-pinecone-api-version example-value
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

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/pinecone/cmd/pinecone-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add pinecone-pp-mcp -- pinecone-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pinecone-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pinecone-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pinecone-pp-cli <command> --help`.

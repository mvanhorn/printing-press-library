---
name: pp-openrouter-image
description: "Every image model on OpenRouter, one key: generate, rank, estimate, and batch with a local cost ledger. Trigger phrases: `generate an image`, `create a picture`, `which image model is cheapest`, `estimate the cost of this image`, `regenerate that image`, `batch of images`, `use openrouter-image`, `run openrouter-image`."
author: "neal-kyle"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - openrouter-image-pp-cli
    install:
      - kind: go
        bins: [openrouter-image-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/cmd/openrouter-image-pp-cli
---

# OpenRouter — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `openrouter-image-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install openrouter-image --cli-only
   ```
2. Verify: `openrouter-image-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/cmd/openrouter-image-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

OpenRouter's Image API fronts 40+ image models from every major lab. This CLI adds what the API alone lacks: offline model ranking by capability and budget, pre-spend cost estimates, deterministic re-generation from a local history ledger, budget-gated batch runs, and a weekly spend digest. Model selection is always explicit — every generation names its model.

## When to Use This CLI

Use this CLI when an AI agent or human needs to generate images on demand through OpenRouter with explicit model selection, wants to pick the cheapest capable provider before spending, run budgeted batches from a CSV, reproduce past generations exactly, or track image-generation spend over time. It is the tool for scheduled image pipelines, prompt iteration across models, and cost-aware image production.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for chat completions or text generation — use the existing openrouter-pp-cli (cost attribution) or a chat CLI
- Do not use this CLI for video generation — OpenRouter's video API is a different surface
- Do not use this CLI for embedding, reranking, or speech endpoints
- Do not use this CLI as a general HTTP client for arbitrary OpenRouter endpoints

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`models rank`** — Rank every image model+provider combo cheapest-first under your capability and budget constraints.

  _Pick the cheapest provider that meets your image constraints without paging through catalog JSON._

  ```bash
  openrouter-image-pp-cli models rank --image-to-image --resolution 4K --max-cost 0.10 --limit 5 --json
  ```
- **`cost-estimate`** — Estimate USD cost of a generation before spending credits, computed offline from synced per-endpoint pricing.

  _Agents can check the price of a planned image before spending credits._

  ```bash
  openrouter-image-pp-cli cost-estimate --model openai/gpt-image-1 --resolution 2K --quality high --n 4
  ```
- **`regenerate`** — Re-run a past generation with its exact stored parameters (model, seed, resolution, quality, references).

  _Reproduce or tweak a past image without re-typing the full flag set._

  ```bash
  openrouter-image-pp-cli regenerate gen-1234567890 --output winner.png
  ```
- **`usage digest`** — Period-over-period spend and volume summary: images generated, USD spent, top models, cost per image vs the prior window.

  _Budget owners get a machine-readable weekly cost report from the local ledger._

  ```bash
  openrouter-image-pp-cli usage digest --since 7d --agent
  ```

### Agent-native plumbing

- **`batch`** — Run many generations from a CSV with a hard USD budget: estimate first, abort before any spend if over, then execute and log each cost.

  _Cron pipelines can fire a batch with a hard spend cap and get typed exit codes instead of burning the whole balance._

  ```bash
  openrouter-image-pp-cli batch --spec batch.csv --budget 2.00 --dry-run
  ```

### Reachability mitigation

- **`models diff`** — See newly added, retired, and price-changed image models between syncs so pinned pipelines never break silently.

  _Catch a retired model before the next scheduled batch 404s._

  ```bash
  openrouter-image-pp-cli models diff --since 7d --json
  ```

## Command Reference

**activity** — Manage activity

- `openrouter-image-pp-cli activity` — Returns user activity data grouped by endpoint for the last 30 (completed) UTC days.

**audio** — Manage audio

- `openrouter-image-pp-cli audio create-speech` — Synthesizes audio from the input text. Returns a raw audio bytestream in the requested format (e.g. mp3, pcm, wav).
- `openrouter-image-pp-cli audio create-transcriptions` — Transcribes audio into text.

**benchmarks** — Benchmarks endpoints

- `openrouter-image-pp-cli benchmarks` — Unified benchmark endpoint that aggregates scores from multiple benchmark sources (Artificial Analysis, Design Arena

**byok** — BYOK endpoints

- `openrouter-image-pp-cli byok create-byokkey` — Create a new bring-your-own-key (BYOK) provider credential.
- `openrouter-image-pp-cli byok delete-byokkey` — Delete (soft-delete) a bring-your-own-key (BYOK) provider credential by its `id`.
- `openrouter-image-pp-cli byok get-byokkey` — Get a single bring-your-own-key (BYOK) provider credential by its `id`.
- `openrouter-image-pp-cli byok list-byokkeys` — List the bring-your-own-key (BYOK) provider credentials for the authenticated entity's default workspace.
- `openrouter-image-pp-cli byok update-byokkey` — Update an existing bring-your-own-key (BYOK) provider credential by its `id`.

**chat** — Chat completion endpoints

- `openrouter-image-pp-cli chat` — Sends a request for a model response for the given chat conversation. Supports both streaming and non-streaming modes.

**classifications** — Task classification market-share endpoints

- `openrouter-image-pp-cli classifications` — Returns the market-share breakdown of OpenRouter traffic by task classification (e.g.

**credits** — Credit management endpoints

- `openrouter-image-pp-cli credits create-coinbase-charge` — Deprecated.
- `openrouter-image-pp-cli credits get` — Get total credits purchased and used for the authenticated user.

**datasets** — Datasets endpoints

- `openrouter-image-pp-cli datasets get-app-rankings` — Returns the top public apps on OpenRouter ranked by token usage inside the requested date window
- `openrouter-image-pp-cli datasets get-rankings-daily` — Returns the top 50 public models per day by total token usage on OpenRouter

**embeddings** — Text embedding endpoints

- `openrouter-image-pp-cli embeddings create` — Submits an embedding request to the embeddings router
- `openrouter-image-pp-cli embeddings list-models` — Returns a list of all available embeddings models and their properties

**endpoints** — Endpoint information

- `openrouter-image-pp-cli endpoints` — Preview the impact of ZDR on the available endpoints

**files** — Files endpoints

- `openrouter-image-pp-cli files delete` — Deletes a file owned by the requesting workspace. Deletion is irreversible.
- `openrouter-image-pp-cli files get-metadata` — Retrieves metadata for a single file owned by the requesting workspace.
- `openrouter-image-pp-cli files list` — Lists files belonging to the workspace of the authenticating API key.
- `openrouter-image-pp-cli files upload` — Uploads a file to be referenced in future API calls.

**generation** — Generation history endpoints

- `openrouter-image-pp-cli generation get` — Get request & usage metadata for a generation
- `openrouter-image-pp-cli generation list-content` — Get stored prompt and completion content for a generation
- `openrouter-image-pp-cli generation submit-feedback` — Submit structured feedback on a generation the authenticated user made.

**guardrails** — Guardrails endpoints

- `openrouter-image-pp-cli guardrails create` — Create a new guardrail for the authenticated user.
- `openrouter-image-pp-cli guardrails delete` — Delete an existing guardrail. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-image-pp-cli guardrails get` — Get a single guardrail by ID. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-image-pp-cli guardrails list` — List all guardrails for the authenticated user.
- `openrouter-image-pp-cli guardrails list-key-assignments` — List all API key guardrail assignments for the authenticated user.
- `openrouter-image-pp-cli guardrails list-member-assignments` — List all organization member guardrail assignments for the authenticated user.
- `openrouter-image-pp-cli guardrails update` — Update an existing guardrail. Collection fields use replace semantics: send the full desired set on every update.

**images** — Images endpoints

- `openrouter-image-pp-cli images create` — Generates an image from a text prompt via the image generation router
- `openrouter-image-pp-cli images list-model-endpoints` — Returns the full per-endpoint records for an image model: each endpoint's definitive supported parameters, pricing
- `openrouter-image-pp-cli images list-models` — Lists every image generation model with its top-level supported-parameter superset and a URL to its full per-endpoint

**key** — Manage key

- `openrouter-image-pp-cli key` — Get information on the API key associated with the current authentication session

**keys** — Manage keys

- `openrouter-image-pp-cli keys create` — Create a new API key for the authenticated user. The plaintext `key` is returned only in this response.
- `openrouter-image-pp-cli keys delete` — Delete an existing API key. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-image-pp-cli keys get` — Get a single API key by hash. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-image-pp-cli keys list` — List all API keys for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-image-pp-cli keys update` — Update an existing API key. [Management key](/docs/guides/overview/auth/management-api-keys) required.

**messages** — Manage messages

- `openrouter-image-pp-cli messages` — Creates a message using the Anthropic Messages API format. Supports text, images, PDFs, tools, and extended thinking.

**model** — Model information endpoints

- `openrouter-image-pp-cli model <author> <slug>` — Returns full details for a single model identified by its author and slug (e.g. openai/gpt-4).

**models** — Model information endpoints

- `openrouter-image-pp-cli models get` — List all models and their properties
- `openrouter-image-pp-cli models list-count` — Get total count of available models
- `openrouter-image-pp-cli models list-user` — List models filtered by user provider preferences, [privacy settings](https://openrouter.

**observability** — Observability endpoints

- `openrouter-image-pp-cli observability create-destination` — Create a new observability destination. A maximum of 5 destinations per type is allowed.
- `openrouter-image-pp-cli observability delete-destination` — Delete an existing observability destination. This performs a soft delete.
- `openrouter-image-pp-cli observability get-destination` — Fetch a single observability destination by its UUID.
- `openrouter-image-pp-cli observability list-destinations` — List the observability destinations configured for the authenticated entity's default workspace.
- `openrouter-image-pp-cli observability update-destination` — Update an existing observability destination. Only the fields provided in the request body are updated.

**openrouter-analytics** — Manage openrouter analytics

- `openrouter-image-pp-cli openrouter-analytics get-meta` — Returns the available metrics, dimensions, filter operators, and granularities for the analytics query endpoint.
- `openrouter-image-pp-cli openrouter-analytics query` — Execute an analytics query with specified metrics, dimensions, filters, and time range.

**openrouter-auth** — Manage openrouter auth

- `openrouter-image-pp-cli openrouter-auth create-keys-code` — Create an authorization code for the PKCE flow to generate a user-controlled API key
- `openrouter-image-pp-cli openrouter-auth exchange-code-for-apikey` — Exchange an authorization code from the PKCE flow for a user-controlled API key

**organization** — Organization endpoints

- `openrouter-image-pp-cli organization` — List all members of the organization associated with the authenticated management key.

**presets** — Presets endpoints

- `openrouter-image-pp-cli presets get` — Retrieves a preset by its slug with its currently designated version inline.
- `openrouter-image-pp-cli presets list` — Lists all presets for the authenticated user, ordered by most recently updated first.

**providers** — Provider information endpoints

- `openrouter-image-pp-cli providers` — List all providers

**rerank** — Rerank endpoints

- `openrouter-image-pp-cli rerank` — Submits a rerank request to the rerank router

**responses** — OpenAI-compatible Responses API endpoints

- `openrouter-image-pp-cli responses create` — Creates a streaming or non-streaming response using OpenResponses API format
- `openrouter-image-pp-cli responses create-compact` — Rewrites a conversation into a smaller context window, returning the canonical next context window

**scim** — SCIM endpoints

- `openrouter-image-pp-cli scim create-group-mapping` — Create a SCIM group-to-workspace role mapping.
- `openrouter-image-pp-cli scim delete-group-mapping` — Delete a SCIM group-to-workspace mapping. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-image-pp-cli scim get-group-mapping` — Get a SCIM group-to-workspace mapping. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-image-pp-cli scim list-group-mappings` — List SCIM group-to-workspace mappings for the organization.
- `openrouter-image-pp-cli scim list-groups` — List SCIM groups for the organization. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-image-pp-cli scim update-group-mapping` — Update a SCIM group mapping role. [Management key](/docs/guides/overview/auth/management-api-keys) required.

**videos** — Manage videos

- `openrouter-image-pp-cli videos create` — Submits a video generation request and returns a polling URL to check status
- `openrouter-image-pp-cli videos get` — Returns job status and content URLs when completed
- `openrouter-image-pp-cli videos list-models` — Returns a list of all available video generation models and their properties

**workspaces** — Workspaces endpoints

- `openrouter-image-pp-cli workspaces create` — Create a new workspace for the authenticated user.
- `openrouter-image-pp-cli workspaces delete` — Delete an existing workspace. The default workspace cannot be deleted.
- `openrouter-image-pp-cli workspaces get` — Get a single workspace by ID or slug. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-image-pp-cli workspaces list` — List all workspaces for the authenticated user.
- `openrouter-image-pp-cli workspaces update` — Update an existing workspace by ID or slug. [Management key](/docs/guides/overview/auth/management-api-keys) required.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `OPENROUTER_IMAGE_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `openrouter-image-pp-cli activity`
- `openrouter-image-pp-cli activity get`
- `openrouter-image-pp-cli activity list`
- `openrouter-image-pp-cli activity search`
- `openrouter-image-pp-cli benchmarks`
- `openrouter-image-pp-cli benchmarks get`
- `openrouter-image-pp-cli benchmarks list`
- `openrouter-image-pp-cli benchmarks search`
- `openrouter-image-pp-cli byok`
- `openrouter-image-pp-cli byok get`
- `openrouter-image-pp-cli byok list`
- `openrouter-image-pp-cli byok search`
- `openrouter-image-pp-cli datasets`
- `openrouter-image-pp-cli datasets get`
- `openrouter-image-pp-cli datasets list`
- `openrouter-image-pp-cli datasets search`
- `openrouter-image-pp-cli datasets-rankings-daily`
- `openrouter-image-pp-cli datasets-rankings-daily get`
- `openrouter-image-pp-cli datasets-rankings-daily list`
- `openrouter-image-pp-cli datasets-rankings-daily search`
- `openrouter-image-pp-cli embeddings`
- `openrouter-image-pp-cli embeddings get`
- `openrouter-image-pp-cli embeddings list`
- `openrouter-image-pp-cli embeddings search`
- `openrouter-image-pp-cli endpoints`
- `openrouter-image-pp-cli endpoints get`
- `openrouter-image-pp-cli endpoints list`
- `openrouter-image-pp-cli endpoints search`
- `openrouter-image-pp-cli files`
- `openrouter-image-pp-cli files get`
- `openrouter-image-pp-cli files list`
- `openrouter-image-pp-cli files search`
- `openrouter-image-pp-cli generation`
- `openrouter-image-pp-cli generation get`
- `openrouter-image-pp-cli generation list`
- `openrouter-image-pp-cli generation search`
- `openrouter-image-pp-cli guardrails`
- `openrouter-image-pp-cli guardrails get`
- `openrouter-image-pp-cli guardrails list`
- `openrouter-image-pp-cli guardrails search`
- `openrouter-image-pp-cli guardrails-assignments-keys`
- `openrouter-image-pp-cli guardrails-assignments-keys get`
- `openrouter-image-pp-cli guardrails-assignments-keys list`
- `openrouter-image-pp-cli guardrails-assignments-keys search`
- `openrouter-image-pp-cli guardrails-assignments-members`
- `openrouter-image-pp-cli guardrails-assignments-members get`
- `openrouter-image-pp-cli guardrails-assignments-members list`
- `openrouter-image-pp-cli guardrails-assignments-members search`
- `openrouter-image-pp-cli images`
- `openrouter-image-pp-cli images get`
- `openrouter-image-pp-cli images list`
- `openrouter-image-pp-cli images search`
- `openrouter-image-pp-cli keys`
- `openrouter-image-pp-cli keys get`
- `openrouter-image-pp-cli keys list`
- `openrouter-image-pp-cli keys search`
- `openrouter-image-pp-cli models`
- `openrouter-image-pp-cli models get`
- `openrouter-image-pp-cli models list`
- `openrouter-image-pp-cli models search`
- `openrouter-image-pp-cli models-count`
- `openrouter-image-pp-cli models-count get`
- `openrouter-image-pp-cli models-count list`
- `openrouter-image-pp-cli models-count search`
- `openrouter-image-pp-cli models-user`
- `openrouter-image-pp-cli models-user get`
- `openrouter-image-pp-cli models-user list`
- `openrouter-image-pp-cli models-user search`
- `openrouter-image-pp-cli observability`
- `openrouter-image-pp-cli observability get`
- `openrouter-image-pp-cli observability list`
- `openrouter-image-pp-cli observability search`
- `openrouter-image-pp-cli organization`
- `openrouter-image-pp-cli organization get`
- `openrouter-image-pp-cli organization list`
- `openrouter-image-pp-cli organization search`
- `openrouter-image-pp-cli presets`
- `openrouter-image-pp-cli presets get`
- `openrouter-image-pp-cli presets list`
- `openrouter-image-pp-cli presets search`
- `openrouter-image-pp-cli providers`
- `openrouter-image-pp-cli providers get`
- `openrouter-image-pp-cli providers list`
- `openrouter-image-pp-cli providers search`
- `openrouter-image-pp-cli scim`
- `openrouter-image-pp-cli scim get`
- `openrouter-image-pp-cli scim list`
- `openrouter-image-pp-cli scim search`
- `openrouter-image-pp-cli scim-group-mappings`
- `openrouter-image-pp-cli scim-group-mappings get`
- `openrouter-image-pp-cli scim-group-mappings list`
- `openrouter-image-pp-cli scim-group-mappings search`
- `openrouter-image-pp-cli videos`
- `openrouter-image-pp-cli videos get`
- `openrouter-image-pp-cli videos list`
- `openrouter-image-pp-cli videos search`
- `openrouter-image-pp-cli workspaces`
- `openrouter-image-pp-cli workspaces get`
- `openrouter-image-pp-cli workspaces list`
- `openrouter-image-pp-cli workspaces search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
openrouter-image-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Cheapest image-to-image model under budget

```bash
openrouter-image-pp-cli models rank --image-to-image --max-cost 0.10 --limit 3 --json
```

Finds capable providers under a per-image budget, cheapest first

### Budget-gated batch from CSV

```bash
openrouter-image-pp-cli batch --spec batch.csv --budget 5.00 --dry-run
```

Dry-run estimates every row and aborts if the total exceeds the budget before any spend

### Reproduce last week's winner

```bash
openrouter-image-pp-cli regenerate gen-1234567890 --output winner-v2.png
```

Replays the exact stored model, seed, resolution, and quality of a past generation

### Pre-flight cost check for an agent

```bash
openrouter-image-pp-cli cost-estimate --model bytedance-seed/seedream-4.5 --resolution 2K --n 4 --json
```

Agents can gate generation on the quoted price before spending credits

### Narrow generation output for agents

```bash
openrouter-image-pp-cli generate --model google/gemini-2.5-flash-image --prompt 'a red panda astronaut floating in space, studio lighting' --json --agent --select data.0.media_type,usage.cost
```

Deeply nested generation responses collapse to the fields an agent needs

### Spot a retiring model before cron breaks

```bash
openrouter-image-pp-cli models diff --since 7d --json
```

Surfaces retired and price-changed models between syncs so pinned pipelines fail loudly, not silently

## Auth Setup

Set OPENROUTER_API_KEY in your environment. The key is read per command; nothing is persisted to disk. Run `openrouter-image-pp-cli doctor` to verify setup.

Run `openrouter-image-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  openrouter-image-pp-cli benchmarks --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `OPENROUTER_IMAGE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `OPENROUTER_IMAGE_CONFIG_DIR`, `OPENROUTER_IMAGE_DATA_DIR`, `OPENROUTER_IMAGE_STATE_DIR`, `OPENROUTER_IMAGE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `OPENROUTER_IMAGE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `openrouter-image-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "openrouter-image": {
        "command": "openrouter-image-pp-mcp",
        "env": {
          "OPENROUTER_IMAGE_HOME": "/srv/openrouter-image"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `OPENROUTER_IMAGE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `OPENROUTER_IMAGE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
openrouter-image-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "openrouter-image-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `openrouter-image-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `openrouter-image-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `openrouter-image-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
openrouter-image-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
openrouter-image-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
openrouter-image-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
openrouter-image-pp-cli playbook amend \
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

`openrouter-image-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `OPENROUTER_IMAGE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
openrouter-image-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
openrouter-image-pp-cli feedback --stdin < notes.txt
openrouter-image-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `OPENROUTER_IMAGE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPENROUTER_IMAGE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
openrouter-image-pp-cli profile save briefing --json
openrouter-image-pp-cli --profile briefing benchmarks
openrouter-image-pp-cli profile list --json
openrouter-image-pp-cli profile show briefing
openrouter-image-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `openrouter-image-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/cmd/openrouter-image-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add openrouter-image-pp-mcp -- openrouter-image-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which openrouter-image-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   openrouter-image-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `openrouter-image-pp-cli <command> --help`.

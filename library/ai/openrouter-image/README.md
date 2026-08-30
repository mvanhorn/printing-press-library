# OpenRouter CLI

**Every image model on OpenRouter, one key: generate, rank, estimate, and batch with a local cost ledger.**

OpenRouter's Image API fronts 40+ image models from every major lab. This CLI adds what the API alone lacks: offline model ranking by capability and budget, pre-spend cost estimates, deterministic re-generation from a local history ledger, budget-gated batch runs, and a weekly spend digest. Model selection is always explicit — every generation names its model.

Learn more at [OpenRouter](https://openrouter.ai/docs).

Created by [@neal-kyle](https://github.com/neal-kyle).

## Install

The recommended path installs both the `openrouter-image-pp-cli` binary and the `pp-openrouter-image` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install openrouter-image
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install openrouter-image --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install openrouter-image --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install openrouter-image --agent claude-code
npx -y @mvanhorn/printing-press-library install openrouter-image --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/cmd/openrouter-image-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openrouter-image-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install openrouter-image --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openrouter-image --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openrouter-image --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install openrouter-image --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openrouter-image-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OPENROUTER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/cmd/openrouter-image-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openrouter-image": {
      "command": "openrouter-image-pp-mcp",
      "env": {
        "OPENROUTER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set OPENROUTER_API_KEY in your environment. The key is read per command; nothing is persisted to disk. Run `openrouter-image-pp-cli doctor` to verify setup.

## Quick Start

```bash
# Verify the CLI is installed and the key is wired up
openrouter-image-pp-cli doctor --dry-run


# Pull the image model catalog and per-endpoint pricing into the local store
openrouter-image-pp-cli sync --resources images --full


# Pick the cheapest provider that supports image-to-image under your budget
openrouter-image-pp-cli models rank --image-to-image --max-cost 0.10 --limit 5


# Check the price before spending credits
openrouter-image-pp-cli cost-estimate --model openai/gpt-image-1 --quality high


# Generate an image and save it to disk
openrouter-image-pp-cli generate --model openai/gpt-image-1 --prompt 'a red panda astronaut' --output panda.png


# Review weekly spend and volume as agent-shaped JSON
openrouter-image-pp-cli usage digest --since 7d --agent

```

## Unique Features

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

## Usage

Run `openrouter-image-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `OPENROUTER_IMAGE_CONFIG_DIR`, `OPENROUTER_IMAGE_DATA_DIR`, `OPENROUTER_IMAGE_STATE_DIR`, or `OPENROUTER_IMAGE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `OPENROUTER_IMAGE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export OPENROUTER_IMAGE_HOME=/srv/openrouter-image
openrouter-image-pp-cli doctor
```

Under `OPENROUTER_IMAGE_HOME=/srv/openrouter-image`, the four dirs resolve to `/srv/openrouter-image/config`, `/srv/openrouter-image/data`, `/srv/openrouter-image/state`, and `/srv/openrouter-image/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `OPENROUTER_IMAGE_DATA_DIR` overrides an explicit `--home` for that kind. Use `OPENROUTER_IMAGE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `OPENROUTER_IMAGE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `openrouter-image-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### activity

Manage activity

- **`openrouter-image-pp-cli activity`** - Returns user activity data grouped by endpoint for the last 30 (completed) UTC days. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### audio

Manage audio

- **`openrouter-image-pp-cli audio create-speech`** - Synthesizes audio from the input text. Returns a raw audio bytestream in the requested format (e.g. mp3, pcm, wav).
- **`openrouter-image-pp-cli audio create-transcriptions`** - Transcribes audio into text. Accepts base64-encoded audio input as JSON or an OpenAI-style multipart/form-data file upload, and returns the transcribed text.

### benchmarks

Benchmarks endpoints

- **`openrouter-image-pp-cli benchmarks`** - Unified benchmark endpoint that aggregates scores from multiple benchmark sources (Artificial Analysis, Design Arena, and OpenRouter's own tau-bench and GPQA evals). Filter by source to reproduce the exact shapes from the legacy per-source endpoints, or use task_type to find models suited for specific workloads. Authenticate with any valid OpenRouter API key. Rate-limited to 30 requests/minute per key and 500 requests/day per account.

### byok

BYOK endpoints

- **`openrouter-image-pp-cli byok create-byokkey`** - Create a new bring-your-own-key (BYOK) provider credential. The raw key is encrypted at rest and never returned in API responses. Defaults to the authenticated entity's default workspace; use the `workspace_id` body field to scope to a different workspace. Treat the raw key as write-only; it is never returned after creation. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli byok delete-byokkey`** - Delete (soft-delete) a bring-your-own-key (BYOK) provider credential by its `id`. The encrypted key material is wiped and the record is marked as deleted. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli byok get-byokkey`** - Get a single bring-your-own-key (BYOK) provider credential by its `id`. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli byok list-byokkeys`** - List the bring-your-own-key (BYOK) provider credentials for the authenticated entity's default workspace. Use the `workspace_id` query parameter to scope the result to a different workspace, or the `provider` query parameter to filter by upstream provider. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli byok update-byokkey`** - Update an existing bring-your-own-key (BYOK) provider credential by its `id`. Include the `key` field to rotate the raw provider API key in-place (the previous key material is overwritten). [Management key](/docs/guides/overview/auth/management-api-keys) required.

### chat

Chat completion endpoints

- **`openrouter-image-pp-cli chat`** - Sends a request for a model response for the given chat conversation. Supports both streaming and non-streaming modes.

### classifications

Task classification market-share endpoints

- **`openrouter-image-pp-cli classifications`** - Returns the market-share breakdown of OpenRouter traffic by task classification
(e.g. code generation, web search, summarization) over a trailing time window.

Each classification reports its share of classified sampled requests (`usage_share`)
and classified sampled token volume (`token_share`) as fractions between 0 and 1.
The unclassified `other` bucket is excluded. Absolute volumes are not exposed
because the underlying data is sampled.

Each classification also includes a `models` array listing the top models by
request volume within that classification, with their within-tag usage and token shares.

Classifications are grouped into macro-categories (Code, Data, Agent, General)
with aggregate shares provided for each.

Authenticate with any valid OpenRouter API key (same key used for inference).
Rate-limited to 30 requests/minute per key and 500 requests/day per account.

When republishing or quoting this data, cite as:
"Source: OpenRouter (openrouter.ai/rankings), as of {as_of}."

### credits

Credit management endpoints

- **`openrouter-image-pp-cli credits create-coinbase-charge`** - Deprecated. The Coinbase APIs used by this endpoint have been deprecated, so Coinbase Commerce charges have been removed. Use the web credits purchase flow instead.
- **`openrouter-image-pp-cli credits get`** - Get total credits purchased and used for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### datasets

Datasets endpoints

- **`openrouter-image-pp-cli datasets get-app-rankings`** - Returns the top public apps on OpenRouter ranked by token usage inside the requested
date window, matching the public apps marketplace on openrouter.ai/apps. Token totals
are `prompt_tokens + completion_tokens`; hidden and private apps are excluded and
traffic from related app aliases is merged into the canonical visible app.

`sort=popular` (default) ranks by total token volume inside the window.
`sort=trending` ranks by absolute excess token growth: window volume minus the average
volume of the three equal-length periods immediately preceding the window. Apps with
no excess growth are omitted, so `trending` may return fewer than `limit` rows.

Filter with `category` (marketplace category group, e.g. `coding`) or `subcategory`
(e.g. `cli-agent`). Ranks are re-numbered 1..N after filtering. Page with `offset` —
`rank` stays absolute, so the first row of `offset=50` is `rank: 51`.

Authenticate with any valid OpenRouter API key (same key used for inference).
Rate-limited to 30 requests/minute per key and 500 requests/day per account.

When republishing or quoting this dataset, OpenRouter must be cited as:
"Source: OpenRouter (openrouter.ai/apps), as of {as_of}."

Token counts come from each upstream provider's own tokenizer, so a token attributed
to one app is not directly comparable to a token attributed to another app whose
traffic flows through a different provider.
- **`openrouter-image-pp-cli datasets get-rankings-daily`** - Returns the top 50 public models per day by total token usage on OpenRouter, plus a
single aggregated `other` row per day that sums every model outside that top 50.
Token totals are `prompt_tokens + completion_tokens`, matching the public rankings
chart on openrouter.ai/rankings.

Each row is a distinct `(date, model_permaslug)` pair. The `other` row uses the
reserved permaslug `other` and is always returned last within its date, so callers
can compute `top-50 traffic / total daily traffic` without a second request.

Optional filters slice the dataset. `period` (`day`/`week`/`month`) sets the time
grain. `modality` and `context_bucket` narrow the exact dataset by output/input
modality (or tool-calling activity) and request context length. `category` and
`language_type` instead read a sampled, upsampled dataset whose `total_tokens` are
weekly-grain estimates — they cannot be combined with each other or with the exact
filters, and reject `period=day` with a 400.

Authenticate with any valid OpenRouter API key (same key used for inference).
Rate-limited to 30 requests/minute per key and 500 requests/day per account.

When republishing or quoting this dataset, OpenRouter must be cited as:
"Source: OpenRouter (openrouter.ai/rankings), as of {as_of}."

Token counts come from each upstream provider's own tokenizer (Anthropic counts
are as reported by Anthropic, OpenAI counts are as reported by OpenAI, etc.), so
a token in one row is not directly comparable to a token in another row from a
different provider.

### embeddings

Text embedding endpoints

- **`openrouter-image-pp-cli embeddings create`** - Submits an embedding request to the embeddings router
- **`openrouter-image-pp-cli embeddings list-models`** - Returns a list of all available embeddings models and their properties

### endpoints

Endpoint information

- **`openrouter-image-pp-cli endpoints`** - Preview the impact of ZDR on the available endpoints

### files

Files endpoints

- **`openrouter-image-pp-cli files delete`** - Deletes a file owned by the requesting workspace. Deletion is irreversible.
- **`openrouter-image-pp-cli files get-metadata`** - Retrieves metadata for a single file owned by the requesting workspace.
- **`openrouter-image-pp-cli files list`** - Lists files belonging to the workspace of the authenticating API key.
- **`openrouter-image-pp-cli files upload`** - Uploads a file to be referenced in future API calls. The file is stored under the workspace of the authenticating API key. Maximum file size: 100 MB.

### generation

Generation history endpoints

- **`openrouter-image-pp-cli generation get`** - Get request & usage metadata for a generation
- **`openrouter-image-pp-cli generation list-content`** - Get stored prompt and completion content for a generation
- **`openrouter-image-pp-cli generation submit-feedback`** - Submit structured feedback on a generation the authenticated user made. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### guardrails

Guardrails endpoints

- **`openrouter-image-pp-cli guardrails create`** - Create a new guardrail for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli guardrails delete`** - Delete an existing guardrail. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli guardrails get`** - Get a single guardrail by ID. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli guardrails list`** - List all guardrails for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli guardrails list-key-assignments`** - List all API key guardrail assignments for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli guardrails list-member-assignments`** - List all organization member guardrail assignments for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli guardrails update`** - Update an existing guardrail. Collection fields use replace semantics: send the full desired set on every update. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### images

Images endpoints

- **`openrouter-image-pp-cli images create`** - Generates an image from a text prompt via the image generation router
- **`openrouter-image-pp-cli images list-model-endpoints`** - Returns the full per-endpoint records for an image model: each endpoint's definitive supported parameters, pricing, and passthrough allowlist.
- **`openrouter-image-pp-cli images list-models`** - Lists every image generation model with its top-level supported-parameter superset and a URL to its full per-endpoint records.

### key

Manage key

- **`openrouter-image-pp-cli key`** - Get information on the API key associated with the current authentication session

### keys

Manage keys

- **`openrouter-image-pp-cli keys create`** - Create a new API key for the authenticated user. The plaintext `key` is returned only in this response. Treat it as a write-only, sensitive value; it cannot be retrieved later. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli keys delete`** - Delete an existing API key. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli keys get`** - Get a single API key by hash. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli keys list`** - List all API keys for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli keys update`** - Update an existing API key. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### messages

Manage messages

- **`openrouter-image-pp-cli messages`** - Creates a message using the Anthropic Messages API format. Supports text, images, PDFs, tools, and extended thinking.

### model

Model information endpoints

- **`openrouter-image-pp-cli model <author> <slug>`** - Returns full details for a single model identified by its author and slug (e.g. openai/gpt-4). Supports variant suffixes (e.g. openai/gpt-4:free) and resolves known slug aliases.

### models

Model information endpoints

- **`openrouter-image-pp-cli models get`** - List all models and their properties
- **`openrouter-image-pp-cli models list-count`** - Get total count of available models
- **`openrouter-image-pp-cli models list-user`** - List models filtered by user provider preferences, [privacy settings](https://openrouter.ai/docs/guides/privacy/provider-logging), and [guardrails](https://openrouter.ai/docs/guides/features/guardrails). If requesting through `eu.openrouter.ai/api/v1/...` the results will be filtered to models that satisfy [EU in-region routing](https://openrouter.ai/docs/guides/privacy/provider-logging#enterprise-eu-in-region-routing).

### observability

Observability endpoints

- **`openrouter-image-pp-cli observability create-destination`** - Create a new observability destination. A maximum of 5 destinations per type is allowed. Defaults to the authenticated entity's default workspace; use the `workspace_id` body field to scope to a different workspace. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli observability delete-destination`** - Delete an existing observability destination. This performs a soft delete. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli observability get-destination`** - Fetch a single observability destination by its UUID. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli observability list-destinations`** - List the observability destinations configured for the authenticated entity's default workspace. Use the `workspace_id` query parameter to scope the result to a different workspace. Only destinations with stable release status are surfaced — destinations of other types are excluded. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli observability update-destination`** - Update an existing observability destination. Only the fields provided in the request body are updated. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### openrouter-analytics

Manage openrouter analytics

- **`openrouter-image-pp-cli openrouter-analytics get-meta`** - Returns the available metrics, dimensions, filter operators, and granularities for the analytics query endpoint. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli openrouter-analytics query`** - Execute an analytics query with specified metrics, dimensions, filters, and time range. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### openrouter-auth

Manage openrouter auth

- **`openrouter-image-pp-cli openrouter-auth create-keys-code`** - Create an authorization code for the PKCE flow to generate a user-controlled API key
- **`openrouter-image-pp-cli openrouter-auth exchange-code-for-apikey`** - Exchange an authorization code from the PKCE flow for a user-controlled API key

### organization

Organization endpoints

- **`openrouter-image-pp-cli organization`** - List all members of the organization associated with the authenticated management key. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### presets

Presets endpoints

- **`openrouter-image-pp-cli presets get`** - Retrieves a preset by its slug with its currently designated version inline.
- **`openrouter-image-pp-cli presets list`** - Lists all presets for the authenticated user, ordered by most recently updated first.

### providers

Provider information endpoints

- **`openrouter-image-pp-cli providers`** - List all providers

### rerank

Rerank endpoints

- **`openrouter-image-pp-cli rerank`** - Submits a rerank request to the rerank router

### responses

OpenAI-compatible Responses API endpoints

- **`openrouter-image-pp-cli responses create`** - Creates a streaming or non-streaming response using OpenResponses API format
- **`openrouter-image-pp-cli responses create-compact`** - Rewrites a conversation into a smaller context window, returning the canonical next context window: echoed input items followed by a `compaction` item that encodes the compacted history. Pass the returned `output` as the `input` of your next `/responses` request. Currently supported on OpenAI and Azure endpoints only; OpenRouter routes follow-up requests carrying the compaction item back to the producing endpoint.

### scim

SCIM endpoints

- **`openrouter-image-pp-cli scim create-group-mapping`** - Create a SCIM group-to-workspace role mapping. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli scim delete-group-mapping`** - Delete a SCIM group-to-workspace mapping. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli scim get-group-mapping`** - Get a SCIM group-to-workspace mapping. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli scim list-group-mappings`** - List SCIM group-to-workspace mappings for the organization. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli scim list-groups`** - List SCIM groups for the organization. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli scim update-group-mapping`** - Update a SCIM group mapping role. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### videos

Manage videos

- **`openrouter-image-pp-cli videos create`** - Submits a video generation request and returns a polling URL to check status
- **`openrouter-image-pp-cli videos get`** - Returns job status and content URLs when completed
- **`openrouter-image-pp-cli videos list-models`** - Returns a list of all available video generation models and their properties

### workspaces

Workspaces endpoints

- **`openrouter-image-pp-cli workspaces create`** - Create a new workspace for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli workspaces delete`** - Delete an existing workspace. The default workspace cannot be deleted. Workspaces with active API keys cannot be deleted; remove the keys first. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli workspaces get`** - Get a single workspace by ID or slug. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli workspaces list`** - List all workspaces for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-image-pp-cli workspaces update`** - Update an existing workspace by ID or slug. [Management key](/docs/guides/overview/auth/management-api-keys) required.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`openrouter-image-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`openrouter-image-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`openrouter-image-pp-cli learnings list`** - Inspect taught rows
- **`openrouter-image-pp-cli learnings forget <query>`** - Undo a teach
- **`openrouter-image-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`openrouter-image-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`openrouter-image-pp-cli teach-pattern`** - Install a query/resource template up front
- **`openrouter-image-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `OPENROUTER_IMAGE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `openrouter-image-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openrouter-image-pp-cli benchmarks

# JSON for scripting and agents
openrouter-image-pp-cli benchmarks --json

# Filter to specific fields
openrouter-image-pp-cli benchmarks --json --select id,name,status

# Dry run — show the request without sending
openrouter-image-pp-cli benchmarks --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openrouter-image-pp-cli benchmarks --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `OPENROUTER_IMAGE_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
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

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
openrouter-image-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `openrouter-image-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/openrouter-pp-cli/config.toml`; `--home`, `OPENROUTER_IMAGE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OPENROUTER_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `openrouter-image-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `openrouter-image-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OPENROUTER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports missing key** — export OPENROUTER_API_KEY=<your-key> and re-run doctor
- **generate returns 401** — Check OPENROUTER_API_KEY is set and valid; run `openrouter-image-pp-cli doctor`
- **model slug not found** — Run `openrouter-image-pp-cli sync --resources images --full` then `openrouter-image-pp-cli models list` to see current slugs; models retire without notice
- **batch spent more than expected** — Use `batch --dry-run` first; the estimate comes from synced pricing, so re-sync before a big batch
- **429 rate limited** — OpenRouter throttles per-key; wait and retry, or route to another provider via `generate --provider`

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**imagen-openrouter**](https://github.com/yusufipk/imagen-openrouter) — JavaScript (37 stars)
- [**jtxmp/openrouter-image-mcp**](https://github.com/jtxmp/openrouter-image-mcp) — TypeScript (3 stars)
- [**openrouter-image-mcp**](https://github.com/hamzatrq/openrouter-image-mcp) — TypeScript
- [**openrouter-pp-cli**](https://github.com/mvanhorn/printing-press-library) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

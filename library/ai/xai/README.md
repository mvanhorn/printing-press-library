# xAI CLI

**OpenAI-compatible xAI operations from a local, agent-ready CLI.**

Use xAI's REST API from scripts and agents with JSON-first output, local cache support, and MCP tooling. The CLI keeps the official bearer-token contract and exposes read-only inspection commands for model and credential health checks.

## Install

The recommended path installs both the `xai-pp-cli` binary and the `pp-xai` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install xai
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install xai --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install xai --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install xai --agent claude-code
npx -y @mvanhorn/printing-press-library install xai --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/xai-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install xai --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-xai --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-xai --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install xai --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/xai-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `XAI_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "xai": {
      "command": "xai-pp-mcp",
      "env": {
        "XAI_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify configuration and API reachability.
xai-pp-cli doctor

# Pick a live model for the task before sending work.
xai-pp-cli model-picker --task chat

# Estimate token cost from live model pricing metadata.
xai-pp-cli cost-preview --model grok-4.3 --input-tokens 1000 --output-tokens 300

# Prove inference works with a one-token chat request.
xai-pp-cli smoke-chat --yes --json --select model,success,content_preview
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Model planning
- **`model-picker`** — Rank live xAI models for a task using the authenticated model catalog.

  _Use this before generation, vision, embedding, image, or video workflows when an agent needs a valid model ID._

  ```bash
  xai-pp-cli model-picker --task reasoning --json --select recommendations.model,recommendations.why
  ```
- **`model-capabilities`** — Build a live capability matrix for the authenticated xAI model catalog.

  _Use this when an agent needs to choose between chat, reasoning, vision, embedding, image, and video surfaces._

  ```bash
  xai-pp-cli model-capabilities --json --select models.id,models.capabilities
  ```

### Cost and readiness
- **`cost-preview`** — Estimate xAI token cost from live model pricing metadata before sending a request.

  _Use this before a large request to estimate prompt and completion cost from xAI's current model metadata._

  ```bash
  xai-pp-cli cost-preview --task chat --input-tokens 2000 --output-tokens 500 --json
  ```
- **`smoke-chat`** — Verify that the authenticated key can run a tiny xAI chat completion.

  _Use this after auth setup to prove inference works, not only that the key is syntactically valid._

  ```bash
  xai-pp-cli smoke-chat --yes --json --select model,success,content_preview
  ```
- **`api-key`** — Inspect the authenticated xAI API key status and permissions without exposing the secret value.

  _Use this when a workflow fails with authorization errors and the agent needs a safe credential-health check._

  ```bash
  xai-pp-cli api-key --json --select api_key_blocked,api_key_disabled,acls
  ```

### Agent readiness
- **`api`** — Browse the complete generated xAI endpoint surface by interface for agent planning and fallback calls.

  _Use this when a task needs an endpoint that is not obvious from the top-level command list._

  ```bash
  xai-pp-cli api
  ```

## Usage

Run `xai-pp-cli --help` for the full command reference and flag list.

## Commands

### api-key

Manage api key

- **`xai-pp-cli api-key`** - Get information about an API key, including name, status, permissions and users who created or modified this key.

### chat

Manage chat

- **`xai-pp-cli chat handle-generic-completion-request`** - Create a chat response from text/image chat prompts. This is the endpoint for making requests to chat and image understanding models.
- **`xai-pp-cli chat handle-get-deferred-completion-request`** - Tries to fetch a result for a previously-started deferred completion. Returns `200 Success` with the response body, if the request has been completed. Returns `202 Accepted` when the request is pending processing.

### complete

Manage complete

- **`xai-pp-cli complete`** - (Legacy - Not supported by reasoning models) Create a text completion response. This endpoint is compatible with the Anthropic API.

### completions

Manage completions

- **`xai-pp-cli completions`** - (Legacy - Not supported by reasoning models) Create a text completion response for a given prompt. Replaced by /v1/chat/completions.

### documents

Manage documents

- **`xai-pp-cli documents`** - Search for content related to the query within the given collections.

### embedding-models

Manage embedding models

- **`xai-pp-cli embedding-models handle-get-request`** - Get full information about an embedding model with its model_id.
- **`xai-pp-cli embedding-models handle-list-request`** - List all embedding models available to the authenticating API key with full information. Additional information compared to /v1/models includes modalities, fingerprint and alias(es).

### embeddings

Manage embeddings

- **`xai-pp-cli embeddings`** - Create an embedding vector representation corresponding to the input text. This is the endpoint for making requests to embedding models.

### files

Manage files

- **`xai-pp-cli files handle-delete-request`** - Delete a file by ID. After this returns, the file no longer appears in
`GET /v1/files`, content download returns 404, and the ID can no longer
be referenced in chat attachments.
- **`xai-pp-cli files handle-list-request`** - List files owned by the authenticated team, paginated. The response
always returns a `pagination_token`; pass it back as a query parameter
to fetch the next page. The end of the list is reached when the
returned `data` array is shorter than `limit`.
- **`xai-pp-cli files handle-retrieve-request`** - Retrieve metadata for a single file by ID. Errors with 404 if the file
doesn't exist, has been deleted, or has passed its `expires_at`.
- **`xai-pp-cli files handle-upload-request`** - Upload a file to xAI's storage. Returns the file's metadata. Files can
be referenced by ID anywhere a `file_id` is accepted (e.g. chat
attachments). Maximum file size: 50 MB. Files are kept until you
delete them, or until `expires_after` elapses if set at upload time.

### image-generation-models

Manage image generation models

- **`xai-pp-cli image-generation-models handle-get-request`** - Get full information about an image generation model with its model_id.
- **`xai-pp-cli image-generation-models handle-list-request`** - List all image generation models available to the authenticating API key with full information. Additional information compared to /v1/models includes modalities, fingerprint and alias(es).

### images

Manage images

- **`xai-pp-cli images handle-edit-request`** - Edit an image based on a prompt. This is the endpoint for making edit requests to image generation models.
- **`xai-pp-cli images handle-generate-request`** - Generate an image based on a prompt. This is the endpoint for making generation requests to image generation models.

### language-models

Manage language models

- **`xai-pp-cli language-models handle-get-request`** - Get full information about a chat or image understanding model with its model_id.
- **`xai-pp-cli language-models handle-list-request`** - List all chat and image understanding models available to the authenticating API key with full information. Additional information compared to /v1/models includes modalities, fingerprint and alias(es).

### me

Manage me

- **`xai-pp-cli me`** - Get information about the currently authenticated caller.
Works with both API keys and OAuth tokens. Returns identity, team, and ZDR status.

### messages

Manage messages

- **`xai-pp-cli messages`** - Create a messages response. This endpoint is compatible with the Anthropic API.

### model-capabilities

Build a live capability matrix for the authenticated xAI model catalog.

- **`xai-pp-cli model-capabilities`** - List model IDs, families, input/output modalities, and inferred capabilities.

### model-picker

Rank live xAI models for a task using the authenticated model catalog.

- **`xai-pp-cli model-picker`** - Recommend models for chat, reasoning, vision, embedding, image, or video work.

### models

Manage models

- **`xai-pp-cli models handle-get-request`** - Get information about a model with its model_id, including pricing.
- **`xai-pp-cli models handle-list-request`** - List all models available to the authenticating API key, including model names (ID), creation times, and pricing.

### cost-preview

Estimate xAI token cost from live model pricing metadata.

- **`xai-pp-cli cost-preview`** - Estimate prompt and completion cost before sending a request.

### responses

Manage responses

- **`xai-pp-cli responses handle-compact-request`** - The client sends its current input (the same items that would be passed
to `POST /v1/responses`) and receives a compacted output window.
The output should be used **verbatim** as the `input` of the next
`/v1/responses` call (appending only the new user turn).

This generalizes the compaction approach used by the coding-agent
harness (`generate_session_compact` in xai-grok-shell):

1. Strip tool-result noise from the history.
2. Ask the model to produce a structured `<summary>` of the conversation.
3. Rebuild a compact window:  system message → summary → last user query.
4. Return that window to the client.
- **`xai-pp-cli responses handle-delete-stored-completion-request`** - Delete a previously generated response.
- **`xai-pp-cli responses handle-generic-model-request`** - Generates a response based on text or image prompts. The response ID can be used to retrieve the response later or to continue the conversation without repeating prior context. New responses will be stored for 30 days and then permanently deleted.
- **`xai-pp-cli responses handle-get-stored-completion-request`** - Retrieve a previously generated response.

### skills

Manage skills

- **`xai-pp-cli skills handle-delete-request`** - Handle delete request
- **`xai-pp-cli skills handle-list-request`** - Handle list request
- **`xai-pp-cli skills handle-retrieve-request`** - Handle retrieve request
- **`xai-pp-cli skills handle-upload-request`** - Handle upload request

### smoke-chat

Verify that the authenticated key can run a tiny xAI chat completion.

- **`xai-pp-cli smoke-chat`** - Send a one-token chat request after `--yes` confirmation.

### tokenize-text

Manage tokenize text

- **`xai-pp-cli tokenize-text`** - Tokenize text with the specified model

### video-generation-models

Manage video generation models

- **`xai-pp-cli video-generation-models handle-get-request`** - Get full information about a video generation model with its model_id.
- **`xai-pp-cli video-generation-models handle-list-request`** - List all video generation models available to the authenticating API key with full information.

### videos

Manage videos

- **`xai-pp-cli videos handle-edit-request`** - Edit a video based on a prompt.
This is an asynchronous operation that returns a request_id for polling.
- **`xai-pp-cli videos handle-extend-request`** - Extend a video by generating continuation content.
This is an asynchronous operation that returns a request_id for polling.
- **`xai-pp-cli videos handle-generate-request`** - Generate a video from a text prompt and optionally an image.
This is an asynchronous operation that returns a request_id for polling.
- **`xai-pp-cli videos handle-get-deferred-request`** - Returns the current status of a video generation job. When the job completes
successfully the response contains the generated video URL. When the job fails
the response contains a structured `error` object with a machine-readable
`code` and a human-readable `message`.

Both successful and failed completions return HTTP 200 — use the `status`
field (`"done"` or `"failed"`) to distinguish between the two.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
xai-pp-cli api-key

# JSON for scripting and agents
xai-pp-cli api-key --json

# Filter to specific fields
xai-pp-cli api-key --json --select id,name,status

# Dry run — show the request without sending
xai-pp-cli api-key --dry-run

# Agent mode — JSON + compact + no prompts in one flag
xai-pp-cli api-key --agent
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

## Health Check

```bash
xai-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/xai-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `XAI_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `xai-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `xai-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $XAI_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **doctor reports missing credentials** — Set XAI_API_KEY or run xai-pp-cli auth set-token <token>.
- **a model command returns an authorization error** — Run xai-pp-cli api-key --json to check whether the key is blocked, disabled, or permission-limited.

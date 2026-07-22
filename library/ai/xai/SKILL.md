---
name: pp-xai
description: "Printing Press CLI for xAI. REST API for xAI compatible with other providers."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - xai-pp-cli
---

# xAI — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `xai-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install xai --cli-only
   ```
2. Verify: `xai-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/xai/cmd/xai-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use xAI's REST API from scripts and agents with JSON-first output, local cache support, and MCP tooling. The CLI keeps the official bearer-token contract and exposes read-only inspection commands for model and credential health checks.

## Unique Capabilities

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

## Command Reference

**api-key** — Manage api key

- `xai-pp-cli api-key` — Get information about an API key, including name, status, permissions and users who created or modified this key.

**chat** — Manage chat

- `xai-pp-cli chat handle-generic-completion-request` — Create a chat response from text/image chat prompts.
- `xai-pp-cli chat handle-get-deferred-completion-request` — Tries to fetch a result for a previously-started deferred completion.

**complete** — Manage complete

- `xai-pp-cli complete` — (Legacy - Not supported by reasoning models) Create a text completion response.

**completions** — Manage completions

- `xai-pp-cli completions` — (Legacy - Not supported by reasoning models) Create a text completion response for a given prompt.

**documents** — Manage documents

- `xai-pp-cli documents` — Search for content related to the query within the given collections.

**embedding-models** — Manage embedding models

- `xai-pp-cli embedding-models handle-get-request` — Get full information about an embedding model with its model_id.
- `xai-pp-cli embedding-models handle-list-request` — List all embedding models available to the authenticating API key with full information.

**embeddings** — Manage embeddings

- `xai-pp-cli embeddings` — Create an embedding vector representation corresponding to the input text.

**files** — Manage files

- `xai-pp-cli files handle-delete-request` — Delete a file by ID.
- `xai-pp-cli files handle-list-request` — List files owned by the authenticated team, paginated.
- `xai-pp-cli files handle-retrieve-request` — Retrieve metadata for a single file by ID.
- `xai-pp-cli files handle-upload-request` — Upload a file to xAI's storage. Returns the file's metadata.

**image-generation-models** — Manage image generation models

- `xai-pp-cli image-generation-models handle-get-request` — Get full information about an image generation model with its model_id.
- `xai-pp-cli image-generation-models handle-list-request` — List all image generation models available to the authenticating API key with full information.

**images** — Manage images

- `xai-pp-cli images handle-edit-request` — Edit an image based on a prompt. This is the endpoint for making edit requests to image generation models.
- `xai-pp-cli images handle-generate-request` — Generate an image based on a prompt. This is the endpoint for making generation requests to image generation models.

**language-models** — Manage language models

- `xai-pp-cli language-models handle-get-request` — Get full information about a chat or image understanding model with its model_id.
- `xai-pp-cli language-models handle-list-request` — List all chat and image understanding models available to the authenticating API key with full information.

**me** — Manage me

- `xai-pp-cli me` — Get information about the currently authenticated caller. Works with both API keys and OAuth tokens.

**messages** — Manage messages

- `xai-pp-cli messages` — Create a messages response. This endpoint is compatible with the Anthropic API.

**model-capabilities** — Build a live capability matrix for the authenticated xAI model catalog

- `xai-pp-cli model-capabilities` — List model IDs, families, input/output modalities, and inferred capabilities.

**model-picker** — Rank live xAI models for a task using the authenticated model catalog

- `xai-pp-cli model-picker` — Recommend models for chat, reasoning, vision, embedding, image, or video work.

**models** — Manage models

- `xai-pp-cli models handle-get-request` — Get information about a model with its model_id, including pricing.
- `xai-pp-cli models handle-list-request` — List all models available to the authenticating API key, including model names (ID), creation times, and pricing.

**cost-preview** — Estimate xAI token cost from live model pricing metadata

- `xai-pp-cli cost-preview` — Estimate prompt and completion cost before sending a request.

**responses** — Manage responses

- `xai-pp-cli responses handle-compact-request` — The client sends its current input (the same items that would be passed to `POST /v1/responses`)
- `xai-pp-cli responses handle-delete-stored-completion-request` — Delete a previously generated response.
- `xai-pp-cli responses handle-generic-model-request` — Generates a response based on text or image prompts.
- `xai-pp-cli responses handle-get-stored-completion-request` — Retrieve a previously generated response.

**skills** — Manage skills

- `xai-pp-cli skills handle-delete-request` — Handle delete request
- `xai-pp-cli skills handle-list-request` — Handle list request
- `xai-pp-cli skills handle-retrieve-request` — Handle retrieve request
- `xai-pp-cli skills handle-upload-request` — Handle upload request

**smoke-chat** — Verify that the authenticated key can run a tiny xAI chat completion

- `xai-pp-cli smoke-chat` — Send a one-token chat request after `--yes` confirmation.

**tokenize-text** — Manage tokenize text

- `xai-pp-cli tokenize-text` — Tokenize text with the specified model

**video-generation-models** — Manage video generation models

- `xai-pp-cli video-generation-models handle-get-request` — Get full information about a video generation model with its model_id.
- `xai-pp-cli video-generation-models handle-list-request` — List all video generation models available to the authenticating API key with full information.

**videos** — Manage videos

- `xai-pp-cli videos handle-edit-request` — Edit a video based on a prompt. This is an asynchronous operation that returns a request_id for polling.
- `xai-pp-cli videos handle-extend-request` — Extend a video by generating continuation content.
- `xai-pp-cli videos handle-generate-request` — Generate a video from a text prompt and optionally an image.
- `xai-pp-cli videos handle-get-deferred-request` — Returns the current status of a video generation job.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
xai-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `xai-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
xai-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `XAI_API_KEY` as an environment variable.

Run `xai-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  xai-pp-cli api-key --agent --select id,name,status
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
xai-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
xai-pp-cli feedback --stdin < notes.txt
xai-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/xai-pp-cli/feedback.jsonl`. They are never POSTed unless `XAI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `XAI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
xai-pp-cli profile save briefing --json
xai-pp-cli --profile briefing api-key
xai-pp-cli profile list --json
xai-pp-cli profile show briefing
xai-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `xai-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add xai-pp-mcp -- xai-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which xai-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   xai-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `xai-pp-cli <command> --help`.

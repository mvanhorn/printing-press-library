# Gemini CLI

The Gemini API allows developers to build generative AI applications using Gemini models. Gemini is our most capable model, built from the ground up to be multimodal. It can generalize and seamlessly understand, operate across, and combine different types of information including language, images, audio, video, and code. You can use the Gemini API for use cases like reasoning across text and images, content generation, dialogue agents, summarization and classification systems, and more.

Learn more at [Gemini](https://developers.generativeai.google/api).

## Install

The recommended path installs both the `gemini-pp-cli` binary and the `pp-gemini` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install gemini
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install gemini --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/gemini/cmd/gemini-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gemini-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gemini --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gemini --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-gemini skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-gemini. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
gemini-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
gemini-pp-cli model-async-batch-embed-content mock-value
```

## Usage

Run `gemini-pp-cli --help` for the full command reference and flag list.

## Commands

### model-async-batch-embed-content

Manage model async batch embed content

- **`gemini-pp-cli model-async-batch-embed-content generativelanguage-tuned-models-async-batch-embed-content`** - Enqueues a batch of `EmbedContent` requests for batch processing. We have a `BatchEmbedContents` handler in `GenerativeService`, but it was synchronized. So we name this one to be `Async` to avoid confusion.

### model-batch-embed-contents

Manage model batch embed contents

- **`gemini-pp-cli model-batch-embed-contents generativelanguage-models-batch-embed-contents`** - Generates multiple embedding vectors from the input `Content` which consists of a batch of strings represented as `EmbedContentRequest` objects.

### model-batch-generate-content

Manage model batch generate content

- **`gemini-pp-cli model-batch-generate-content generativelanguage-tuned-models-batch-generate-content`** - Enqueues a batch of `GenerateContent` requests for batch processing.

### model-count-tokens

Manage model count tokens

- **`gemini-pp-cli model-count-tokens generativelanguage-models-count-tokens`** - Runs a model's tokenizer on input `Content` and returns the token count. Refer to the [tokens guide](https://ai.google.dev/gemini-api/docs/tokens) to learn more about tokens.

### model-embed-content

Manage model embed content

- **`gemini-pp-cli model-embed-content generativelanguage-models-embed-content`** - Generates a text embedding vector from the input `Content` using the specified [Gemini Embedding model](https://ai.google.dev/gemini-api/docs/models/gemini#text-embedding).

### model-generate-content

Manage model generate content

- **`gemini-pp-cli model-generate-content generativelanguage-tuned-models-generate-content`** - Generates a model response given an input `GenerateContentRequest`. Refer to the [text generation guide](https://ai.google.dev/gemini-api/docs/text-generation) for detailed usage information. Input capabilities differ between models, including tuned models. Refer to the [model guide](https://ai.google.dev/gemini-api/docs/models/gemini) and [tuning guide](https://ai.google.dev/gemini-api/docs/model-tuning) for details.

### model-stream-generate-content

Manage model stream generate content

- **`gemini-pp-cli model-stream-generate-content generativelanguage-tuned-models-stream-generate-content`** - Generates a [streamed response](https://ai.google.dev/gemini-api/docs/text-generation?lang=python#generate-a-text-stream) from the model given an input `GenerateContentRequest`.

### models

Models

- **`gemini-pp-cli models generativelanguage-list`** - Lists the [`Model`s](https://ai.google.dev/gemini-api/docs/models/gemini) available through the Gemini API.

### name-cancel

Manage name cancel

- **`gemini-pp-cli name-cancel generativelanguage-tuned-models-operations-cancel`** - Starts asynchronous cancellation on a long-running operation. The server makes a best effort to cancel the operation, but success is not guaranteed. If the server doesn't support this method, it returns `google.rpc.Code.UNIMPLEMENTED`. Clients can use Operations.GetOperation or other methods to check whether the cancellation succeeded or whether the operation completed despite cancellation. On successful cancellation, the operation is not deleted; instead, it becomes an operation with an Operation.error value with a google.rpc.Status.code of `1`, corresponding to `Code.CANCELLED`.

### name-update-embed-content-batch

Manage name update embed content batch

- **`gemini-pp-cli name-update-embed-content-batch generativelanguage-batches-update-embed-content-batch`** - Updates a batch of EmbedContent requests for batch processing.

### name-update-generate-content-batch

Manage name update generate content batch

- **`gemini-pp-cli name-update-generate-content-batch generativelanguage-batches-update-generate-content-batch`** - Updates a batch of GenerateContent requests for batch processing.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gemini-pp-cli model-async-batch-embed-content mock-value

# JSON for scripting and agents
gemini-pp-cli model-async-batch-embed-content mock-value --json

# Filter to specific fields
gemini-pp-cli model-async-batch-embed-content mock-value --json --select id,name,status

# Dry run — show the request without sending
gemini-pp-cli model-async-batch-embed-content mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gemini-pp-cli model-async-batch-embed-content mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-gemini -g
```

Then invoke `/pp-gemini <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/gemini/cmd/gemini-pp-mcp@latest
```

Then register it:

```bash
claude mcp add gemini gemini-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gemini-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/gemini/cmd/gemini-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gemini": {
      "command": "gemini-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
gemini-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/gemini-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

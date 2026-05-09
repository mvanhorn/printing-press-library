# Gemini CLI

The Gemini API allows developers to build generative AI applications using Gemini models. Gemini is our most capable model, built from the ground up to be multimodal. It can generalize and seamlessly understand, operate across, and combine different types of information including language, images, audio, video, and code. You can use the Gemini API for use cases like reasoning across text and images, content generation, dialogue agents, summarization and classification systems, and more.

Learn more at [Gemini](https://developers.generativeai.google/api).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/ai/gemini/cmd/gemini-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

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
gemini-pp-cli model-async-batch-embed-content list
```

## Usage

<!-- HELP_OUTPUT -->

## Commands

### model-async-batch-embed-content

Manage model async batch embed content

- **`gemini-pp-cli model-async-batch-embed-content generativelanguage-tuned-models-async-batch-embed-content`** - Enqueues a batch of `EmbedContent` requests for batch processing

### model-batch-embed-contents

Manage model batch embed contents

- **`gemini-pp-cli model-batch-embed-contents generativelanguage-models-batch-embed-contents`** - Generates multiple embedding vectors from the input `Content` which consists of a batch of strings r

### model-batch-generate-content

Manage model batch generate content

- **`gemini-pp-cli model-batch-generate-content generativelanguage-tuned-models-batch-generate-content`** - Enqueues a batch of `GenerateContent` requests for batch processing

### model-count-tokens

Manage model count tokens

- **`gemini-pp-cli model-count-tokens generativelanguage-models-count-tokens`** - Runs a model's tokenizer on input `Content` and returns the token count

### model-embed-content

Manage model embed content

- **`gemini-pp-cli model-embed-content generativelanguage-models-embed-content`** - Generates a text embedding vector from the input `Content` using the specified [Gemini Embedding mod

### model-generate-content

Manage model generate content

- **`gemini-pp-cli model-generate-content generativelanguage-tuned-models-generate-content`** - Generates a model response given an input `GenerateContentRequest`

### model-stream-generate-content

Manage model stream generate content

- **`gemini-pp-cli model-stream-generate-content generativelanguage-tuned-models-stream-generate-content`** - Generates a [streamed response](https://ai

### models

Models

- **`gemini-pp-cli models generativelanguage-list`** - Lists the [`Model`s](https://ai

### name-cancel

Manage name cancel

- **`gemini-pp-cli name-cancel generativelanguage-tuned-models-operations-cancel`** - Starts asynchronous cancellation on a long-running operation

### name-update-embed-content-batch

Manage name update embed content batch

- **`gemini-pp-cli name-update-embed-content-batch generativelanguage-batches-update-embed-content-batch`** - Updates a batch of EmbedContent requests for batch processing

### name-update-generate-content-batch

Manage name update generate content batch

- **`gemini-pp-cli name-update-generate-content-batch generativelanguage-batches-update-generate-content-batch`** - Updates a batch of GenerateContent requests for batch processing


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gemini-pp-cli model-async-batch-embed-content list

# JSON for scripting and agents
gemini-pp-cli model-async-batch-embed-content list --json

# Filter to specific fields
gemini-pp-cli model-async-batch-embed-content list --json --select id,name,status

# Dry run — show the request without sending
gemini-pp-cli model-async-batch-embed-content list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gemini-pp-cli model-async-batch-embed-content list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | gemini-pp-cli <resource> create --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add gemini gemini-pp-mcp
```

### Claude Desktop

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

## Cookbook

Common workflows and recipes:

```bash
# List resources as JSON for scripting
gemini-pp-cli model-async-batch-embed-content list --json

# Filter to specific fields
gemini-pp-cli model-async-batch-embed-content list --json --select id,name,status

# Dry run to preview the request
gemini-pp-cli model-async-batch-embed-content list --dry-run

# Sync data locally for offline search
gemini-pp-cli sync

# Search synced data
gemini-pp-cli search "query"

# Export for backup
gemini-pp-cli export --format jsonl > backup.jsonl
```

## Health Check

```bash
gemini-pp-cli doctor
```

<!-- DOCTOR_OUTPUT -->

## Configuration

Config file: `~/.config/gemini-pp-cli/config.toml`

Environment variables:

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `gemini-pp-cli doctor` to check credentials

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- If persistent, wait a few minutes and try again

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

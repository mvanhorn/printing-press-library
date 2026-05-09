---
name: pp-gemini
description: "Printing Press CLI for Gemini. The Gemini API allows developers to build generative AI applications using Gemini models. Gemini is our most capable..."
author: "Krishna Vardhan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gemini-pp-cli
    install:
      - kind: go
        bins: [gemini-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/gemini/cmd/gemini-pp-cli
---

# Gemini — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gemini-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install gemini --cli-only
   ```
2. Verify: `gemini-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/gemini/cmd/gemini-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Gemini API allows developers to build generative AI applications using Gemini models. Gemini is our most capable model, built from the ground up to be multimodal. It can generalize and seamlessly understand, operate across, and combine different types of information including language, images, audio, video, and code. You can use the Gemini API for use cases like reasoning across text and images, content generation, dialogue agents, summarization and classification systems, and more.

## Command Reference

**model-async-batch-embed-content** — Manage model async batch embed content

- `gemini-pp-cli model-async-batch-embed-content <model>` — Enqueues a batch of `EmbedContent` requests for batch processing. We have a `BatchEmbedContents` handler in...

**model-batch-embed-contents** — Manage model batch embed contents

- `gemini-pp-cli model-batch-embed-contents <model>` — Generates multiple embedding vectors from the input `Content` which consists of a batch of strings represented as...

**model-batch-generate-content** — Manage model batch generate content

- `gemini-pp-cli model-batch-generate-content <model>` — Enqueues a batch of `GenerateContent` requests for batch processing.

**model-count-tokens** — Manage model count tokens

- `gemini-pp-cli model-count-tokens <model>` — Runs a model's tokenizer on input `Content` and returns the token count. Refer to the [tokens...

**model-embed-content** — Manage model embed content

- `gemini-pp-cli model-embed-content <model>` — Generates a text embedding vector from the input `Content` using the specified [Gemini Embedding...

**model-generate-content** — Manage model generate content

- `gemini-pp-cli model-generate-content <model>` — Generates a model response given an input `GenerateContentRequest`. Refer to the [text generation...

**model-stream-generate-content** — Manage model stream generate content

- `gemini-pp-cli model-stream-generate-content <model>` — Generates a [streamed response](https://ai.google.dev/gemini-api/docs/text-generation?lang=python#generate-a-text-str...

**models** — Models

- `gemini-pp-cli models` — Lists the [`Model`s](https://ai.google.dev/gemini-api/docs/models/gemini) available through the Gemini API.

**name-cancel** — Manage name cancel

- `gemini-pp-cli name-cancel <name>` — Starts asynchronous cancellation on a long-running operation. The server makes a best effort to cancel the...

**name-update-embed-content-batch** — Manage name update embed content batch

- `gemini-pp-cli name-update-embed-content-batch <name>` — Updates a batch of EmbedContent requests for batch processing.

**name-update-generate-content-batch** — Manage name update generate content batch

- `gemini-pp-cli name-update-generate-content-batch <name>` — Updates a batch of GenerateContent requests for batch processing.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gemini-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `gemini-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gemini-pp-cli model-async-batch-embed-content mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
gemini-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gemini-pp-cli feedback --stdin < notes.txt
gemini-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.gemini-pp-cli/feedback.jsonl`. They are never POSTed unless `GEMINI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GEMINI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gemini-pp-cli profile save briefing --json
gemini-pp-cli --profile briefing model-async-batch-embed-content mock-value
gemini-pp-cli profile list --json
gemini-pp-cli profile show briefing
gemini-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `gemini-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/gemini/cmd/gemini-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add gemini-pp-mcp -- gemini-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gemini-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gemini-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gemini-pp-cli <command> --help`.

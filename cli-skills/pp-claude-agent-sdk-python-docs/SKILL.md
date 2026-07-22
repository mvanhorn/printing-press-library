---
name: pp-claude-agent-sdk-python-docs
description: "A source-grounded CLI for exploring and verifying the Claude Agent SDK Python docs. Trigger phrases: `look up Claude Agent SDK Python`, `verify this against Claude Agent SDK docs`, `ClaudeSDKClient docs`, `ClaudeAgentOptions reference`, `use claude agent sdk python docs`, `run claude agent sdk python docs`."
author: "Nik"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - claude-agent-sdk-python-docs-pp-cli
    install:
      - kind: go
        bins: [claude-agent-sdk-python-docs-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/claude-agent-sdk-python-docs/cmd/claude-agent-sdk-python-docs-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/claude-agent-sdk-python-docs/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Claude Agent SDK Python Docs — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `claude-agent-sdk-python-docs-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install claude-agent-sdk-python-docs --cli-only
   ```
2. Verify: `claude-agent-sdk-python-docs-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/claude-agent-sdk-python-docs/cmd/claude-agent-sdk-python-docs-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Fetch the raw Claude Code docs index, search the Python Agent SDK reference, and extract exact symbols, examples, and citations. Novel verification and context commands help agents use the SDK without guessing.

## When to Use This CLI

Use this CLI when you need exact Claude Agent SDK Python documentation, examples, signatures, or citations in an agent-friendly shape. It is strongest for implementation planning, code review, and preventing SDK hallucinations.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to run Claude Agent SDK applications.
- Do not use this CLI for TypeScript SDK questions unless the docs mirror has been expanded to TypeScript pages.
- Do not use this CLI as a replacement for live Claude Code authentication or account setup.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Docs-grounded verification
- **`verify`** — Check Python code imports and qualified names against documented Claude Agent SDK symbols.

  _Use this before trusting generated SDK code or reviewing a PR that imports claude_agent_sdk._

  ```bash
  claude-agent-sdk-python-docs-pp-cli verify ./src --agent
  ```
- **`diff`** — Compare current docs entity hashes against an optional baseline file.

  _Use this to detect whether SDK docs facts have changed against a saved baseline._

  ```bash
  claude-agent-sdk-python-docs-pp-cli diff --since latest --agent
  ```
- **`audit-links`** — Validate internal anchors, guide links, and section references in the fetched docs corpus.

  _Use this before relying on context bundles or citations in generated code reviews._

  ```bash
  claude-agent-sdk-python-docs-pp-cli audit-links --agent
  ```

### Agent-native context
- **`context`** — Build a compact source-cited docs bundle for one SDK implementation task.

  _Use this when an agent needs exact docs context without loading full Markdown pages._

  ```bash
  claude-agent-sdk-python-docs-pp-cli context "custom tools" --agent --select sections.title,examples.code,citations.url
  ```
- **`recipe`** — Compose a deterministic implementation scaffold from documented snippets and exact signatures.

  _Use this when you need a copyable starting point constrained to documented SDK patterns._

  ```bash
  claude-agent-sdk-python-docs-pp-cli recipe "streaming ClaudeSDKClient" --agent
  ```

### SDK surface intelligence
- **`map`** — Map functions, classes, types, options, tools, hooks, and message blocks by entity type.

  _Use this to discover the available SDK surface before choosing an implementation path._

  ```bash
  claude-agent-sdk-python-docs-pp-cli map --kind classes,types,options --agent
  ```
- **`coverage examples`** — Report which documented symbols have extracted examples and which do not.

  _Use this to find example-backed SDK APIs and documentation coverage gaps._

  ```bash
  claude-agent-sdk-python-docs-pp-cli coverage examples --agent
  ```

## Command Reference

**pages** — Fetch Claude Agent SDK documentation pages

- `claude-agent-sdk-python-docs-pp-cli pages custom-tools` — Fetch the Agent SDK custom tools guide
- `claude-agent-sdk-python-docs-pp-cli pages index` — Fetch the Claude Code documentation index
- `claude-agent-sdk-python-docs-pp-cli pages mcp` — Fetch the Agent SDK MCP guide
- `claude-agent-sdk-python-docs-pp-cli pages overview` — Fetch the Agent SDK overview
- `claude-agent-sdk-python-docs-pp-cli pages permissions` — Fetch the Agent SDK permissions guide
- `claude-agent-sdk-python-docs-pp-cli pages python` — Fetch the Python Agent SDK reference
- `claude-agent-sdk-python-docs-pp-cli pages quickstart` — Fetch the Agent SDK quickstart
- `claude-agent-sdk-python-docs-pp-cli pages sessions` — Fetch the Agent SDK sessions guide
- `claude-agent-sdk-python-docs-pp-cli pages structured-outputs` — Fetch the Agent SDK structured output guide


## Freshness Contract

The generated `pages` endpoints and hand-authored docs intelligence commands read the public Claude Code docs over HTTPS. The docs intelligence commands intentionally reject `--data-source local`; use `--data-source auto` or `--data-source live` for those commands. For structured retrieval, prefer `read`, `search`, `symbol`, `examples`, `guide`, or `context` over raw `pages` binary endpoints.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
claude-agent-sdk-python-docs-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find the right option fields for a client setup

```bash
claude-agent-sdk-python-docs-pp-cli search "ClaudeAgentOptions" --type pages --agent --select items.title,items.url,items.snippet
```

Returns only the high-signal fields an agent needs instead of the full reference page.

### Build a custom-tools implementation bundle

```bash
claude-agent-sdk-python-docs-pp-cli context "custom tools" --agent --select sections.title,examples.code,citations.url
```

Pairs `--agent` with `--select` so downstream agents get just examples, section titles, and citations.

### Verify a Python project against current docs

```bash
claude-agent-sdk-python-docs-pp-cli verify ./src --agent
```

Flags undocumented Claude Agent SDK Python identifiers with source citations.

### Inspect documented SDK surface area

```bash
claude-agent-sdk-python-docs-pp-cli map --kind classes,types,options --agent
```

Shows the SDK inventory by entity type before writing code.

## Auth Setup

No authentication required.

Run `claude-agent-sdk-python-docs-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  claude-agent-sdk-python-docs-pp-cli context "custom tools" --agent --select sections.title,examples.code,citations.url
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Agent-friendly** — docs intelligence commands return structured JSON with `--agent` and `--select`
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Output shape

Docs intelligence commands return command-specific JSON objects. Use `--select` to keep only the fields an agent needs, such as `sections.title`, `examples.code`, `citations.url`, `symbols.name`, or `unknown_symbols`.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
claude-agent-sdk-python-docs-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
claude-agent-sdk-python-docs-pp-cli feedback --stdin < notes.txt
claude-agent-sdk-python-docs-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/claude-agent-sdk-python-docs-pp-cli/feedback.jsonl`. They are never POSTed unless `CLAUDE_AGENT_SDK_PYTHON_DOCS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLAUDE_AGENT_SDK_PYTHON_DOCS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same docs command every run with the same output and filtering options.

```
claude-agent-sdk-python-docs-pp-cli profile save briefing --json
claude-agent-sdk-python-docs-pp-cli --profile briefing pages custom-tools
claude-agent-sdk-python-docs-pp-cli profile list --json
claude-agent-sdk-python-docs-pp-cli profile show briefing
claude-agent-sdk-python-docs-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `claude-agent-sdk-python-docs-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/claude-agent-sdk-python-docs/cmd/claude-agent-sdk-python-docs-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add claude-agent-sdk-python-docs-pp-mcp -- claude-agent-sdk-python-docs-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which claude-agent-sdk-python-docs-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   claude-agent-sdk-python-docs-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `claude-agent-sdk-python-docs-pp-cli <command> --help`.

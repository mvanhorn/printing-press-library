---
name: pp-notebooklm
description: "Printing Press CLI for NotebookLM. Query and manage Google NotebookLM notebooks from the terminal"
author: "Markimus"
license: "Apache-2.0"
argument-hint: "<command> [args]"
allowed-tools: "Read Bash"
---

# NotebookLM — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `notebooklm-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install notebooklm --cli-only
   ```
2. Verify: `notebooklm-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Query and manage Google NotebookLM notebooks from the terminal

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native notebook querying
- **`query`** — Ask a question against a notebook's sources with AI-powered answers and citations.

  _Use this when an agent needs to extract knowledge from a specific NotebookLM notebook._

  ```bash
  notebooklm-pp-cli query <notebook-id> "What are the key takeaways?" --agent
  ```

### Cross-notebook intelligence
- **`cross-query`** — Query across multiple notebooks with aggregated citations.

  _Use this when an agent needs to synthesize information from multiple research notebooks._

  ```bash
  notebooklm-pp-cli cross-query "compare approaches" --notebooks "id1,id2" --agent
  ```

### Offline knowledge cache
- **`sync`** — Cache notebook source content to local SQLite for instant offline search.

  _Use this when an agent will query the same notebooks repeatedly without burning API calls._

  ```bash
  notebooklm-pp-cli sync --notebook <id>
  ```

- **`search`** — Full-text search across synced notebook data using FTS5.

  ```bash
  notebooklm-pp-cli search "machine learning algorithms" --limit 10 --agent
  ```

### Source management
- **`sources add`** — Add URLs, text, files, or Drive docs to a notebook.

  ```bash
  notebooklm-pp-cli sources add <notebook-id> --url "https://example.com" --wait --agent
  ```

### Deep research
- **`research start`** — Start fast or deep research to discover new sources.

  ```bash
  notebooklm-pp-cli research start <notebook-id> "query" --mode deep
  ```

## Command Reference

**notebooks** — Manage NotebookLM notebooks

- `notebooklm-pp-cli notebooks list` — List all notebooks
- `notebooklm-pp-cli notebooks get` — Get notebook details
- `notebooklm-pp-cli notebooks create` — Create a new notebook
- `notebooklm-pp-cli notebooks describe` — AI summary and topics

**query** — Ask questions against notebook sources

- `notebooklm-pp-cli query <notebook-id> <question>` — Single-shot query

**sources** — Manage sources in notebooks

- `notebooklm-pp-cli sources list` — List sources
- `notebooklm-pp-cli sources add` — Add source (URL/text/file/drive)
- `notebooklm-pp-cli sources content` — Get raw source text

**studio** — Create artifacts

- `notebooklm-pp-cli studio create` — Create audio/video/report/etc.
- `notebooklm-pp-cli studio status` — Check artifact status

**research** — Discover new sources

- `notebooklm-pp-cli research start` — Start research task
- `notebooklm-pp-cli research import` — Import results as sources

**sync & search** — Offline capabilities

- `notebooklm-pp-cli sync` — Cache to SQLite
- `notebooklm-pp-cli search <query>` — Full-text search

## Auth Setup

This CLI wraps `nlm` (notebooklm-mcp-cli). Auth is handled by `nlm login` which opens a browser for Google SSO. Sessions last ~20 minutes.

Run `notebooklm-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `notebooklm-pp-cli --help` output
2. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## Direct Use

1. Check if installed: `which notebooklm-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   notebooklm-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `notebooklm-pp-cli <command> --help`.

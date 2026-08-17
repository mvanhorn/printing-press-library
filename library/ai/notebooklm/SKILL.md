---
name: pp-notebooklm
description: "Agent-native Gemini Notebook (NotebookLM) CLI with Chrome cookie auth, batchexecute RPC, offline notebook search, grounded chat, and Studio quiz generation. Trigger phrases: `notebooklm`, `use notebooklm-pp-cli`, `ask my notebook`, `notebooklm chat`."
author: "Som Samantray"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - notebooklm-pp-cli
    install:
      - kind: go
        bins: [notebooklm-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/notebooklm/cmd/notebooklm-pp-cli
---

# Gemini Notebook (NotebookLM) — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `notebooklm-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install notebooklm --cli-only
   ```
2. Verify: `notebooklm-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/notebooklm/cmd/notebooklm-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI to manage Gemini Notebook projects from the terminal: list/create notebooks, add URL sources, ask grounded questions with citations, generate Studio quizzes, cache notebooks locally for `search`, and run health checks before agent loops. There is no official API key — authenticate once with Chrome session cookies.

## Anti-triggers

Do not use this CLI for:
- Official Google-supported NotebookLM APIs (none exist publicly)
- Bulk scraping outside your own notebooks
- Sharing mutation workflows (only `share show` is implemented)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Auth
- **`auth login`** — Import Google session cookies from Chrome without manual DevTools copying.

  _Use once after logging into Google in Chrome to enable all live commands._

  ```bash
  notebooklm-pp-cli auth login --chrome
  ```

### Local cache
- **`search`** — Search synced notebooks locally by title or id without another API round-trip.

  _Use when you need fast recall across many notebooks in scripts._

  ```bash
  notebooklm-pp-cli sync --json && notebooklm-pp-cli search "quarterly" --json
  ```

### Chat
- **`chat ask`** — Ask questions against notebook sources via streamed batchexecute RPC.

  _Use for agent loops that need cited answers from uploaded sources._

  ```bash
  notebooklm-pp-cli chat ask "Research" "What are the main themes?" --json
  ```

### Studio
- **`studio generate-quiz`** — Kick off NotebookLM Studio quiz artifact generation and optionally wait for completion.

  _Use to produce study materials from a notebook's sources._

  ```bash
  notebooklm-pp-cli studio generate-quiz "Research" --wait --json
  ```

### Operations
- **`doctor`** — Validate config, cookie presence, and session bootstrap tokens before live calls.

  _Run after auth login or when list commands return empty results._

  ```bash
  notebooklm-pp-cli doctor --json
  ```

## Command Reference

**auth** — Manage Google session authentication

- `notebooklm-pp-cli auth login --chrome` — Import cookies from Chrome for `.google.com`
- `notebooklm-pp-cli auth status --json` — Show whether cookies are stored locally

**notebook** — Manage notebooks

- `notebooklm-pp-cli notebook list --json` — List recently viewed notebooks
- `notebooklm-pp-cli notebook create "Title" --json` — Create a notebook
- `notebooklm-pp-cli notebook get <id-or-title> --json` — Get notebook details and sources
- `notebooklm-pp-cli notebook rename <id-or-title> "New Title" --json` — Rename a notebook
- `notebooklm-pp-cli notebook delete <id-or-title> --json` — Delete a notebook

**source** — Manage notebook sources

- `notebooklm-pp-cli source list <notebook> --json` — List sources in a notebook
- `notebooklm-pp-cli source add <notebook> <url> --json` — Add a URL source
- `notebooklm-pp-cli source delete <notebook> <source-id> --json` — Delete a source

**chat** — Chat with a notebook

- `notebooklm-pp-cli chat ask <notebook> <question> --json` — Ask a grounded question
- `notebooklm-pp-cli chat history <notebook> --json` — List conversation history

**studio** — Studio artifacts

- `notebooklm-pp-cli studio list <notebook> --json` — List Studio artifacts
- `notebooklm-pp-cli studio generate-quiz <notebook> --wait --json` — Generate a quiz artifact

**share** — Notebook sharing

- `notebooklm-pp-cli share show <notebook> --json` — Get sharing status

**sync** — Refresh local notebook cache

- `notebooklm-pp-cli sync --json` — Sync notebooks to SQLite (use `--db` to override cache path)

**search** — Search cached notebooks

- `notebooklm-pp-cli search <query> --json` — Search local cache (run `sync` first)

**whoami** — Show account tier and output language

- `notebooklm-pp-cli whoami --json`

**doctor** — Health check

- `notebooklm-pp-cli doctor --json`

## Recipes

### Create notebook and add a URL source

```bash
notebooklm-pp-cli notebook create "Agent Research" --json && notebooklm-pp-cli source add "Agent Research" https://en.wikipedia.org/wiki/NotebookLM --json
```

Bootstrap a fresh notebook with a public web source.

### Search cached notebooks

```bash
notebooklm-pp-cli search "research" --json
```

Find notebooks by title after running sync.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `not authenticated` | `notebooklm-pp-cli auth login --chrome` |
| Empty `notebook list` or `null result` | Re-login in Chrome; run `doctor --json` |
| `search` returns nothing | Run `sync --json` first |

## Global flags

- `--json` — machine-readable JSON on stdout
- `--dry-run` — skip live API calls (for CI and verify)
- `--agent` — agent-friendly defaults (`--json --no-input --yes`)
- `--config` — alternate config path

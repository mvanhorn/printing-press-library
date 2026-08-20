# Gemini Notebook (NotebookLM) CLI

**Unofficial Go CLI for [Gemini Notebook](https://notebooklm.google.com/) — notebooks, sources, grounded chat, Studio artifacts, offline search, and MCP via batchexecute RPC with Chrome session auth.**

> Not affiliated with Google. Uses your personal Google session cookies. For personal use only.

Created by [@SomSamantray](https://github.com/SomSamantray).

## Install

```bash
npx -y @mvanhorn/printing-press-library install notebooklm --cli-only
```

Go fallback (CLI only):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/notebooklm/cmd/notebooklm-pp-cli@latest
```

From this repo:

```bash
cd library/ai/notebooklm
go build -o notebooklm-pp-cli ./cmd/notebooklm-pp-cli
go build -o notebooklm-pp-mcp ./cmd/notebooklm-pp-mcp
```

## Authentication

NotebookLM has no public API key. Authenticate once by importing Google session cookies from Chrome:

```bash
pip install pycookiecheat   # optional helper; browser-cookies also works
notebooklm-pp-cli auth login --chrome
notebooklm-pp-cli auth status --json
```

Cookies are stored at `~/.config/notebooklm-pp-cli/config.toml` (mode `0600`). Re-run login when `doctor` reports missing session tokens or live commands return empty results.

Alternative: `notebooklm-pp-cli auth login --cookies-file ./cookie-header.txt`

## Quick Start

```bash
# Import Google session cookies from Chrome
notebooklm-pp-cli auth login --chrome

# Verify auth and session bootstrap
notebooklm-pp-cli doctor --json

# List notebooks
notebooklm-pp-cli notebook list --json

# Cache notebooks locally for offline search
notebooklm-pp-cli sync --json

# Ask a grounded question with citations
notebooklm-pp-cli chat ask "My Notebook" "Summarize the sources" --json

```

## Known Gaps

- **`whoami`** may return `{}` when Google’s user-settings RPC yields a null frame; tier/language parsing is best-effort.
- **`chat history`** often returns `[]` immediately after `chat ask` because conversation indexing lags on Google’s side.
- **Studio** supports quiz generation only; audio, slides, mind map, and download/export are not implemented yet.
- **`share show`** is read-only; public/private mutations are not implemented yet.
- **Notes, file upload, YouTube/Drive sources, and research agents** are planned (see repo plan doc).

## Unique Features

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

## Commands

| Domain | Commands |
|--------|----------|
| Auth | `auth login`, `auth status` |
| Notebooks | `notebook list`, `create`, `get`, `rename`, `delete` |
| Sources | `source list`, `add`, `delete` |
| Chat | `chat ask`, `chat history` |
| Studio | `studio list`, `studio generate-quiz` |
| Share | `share show` |
| Cache | `sync`, `search` |
| Account | `whoami` |
| Health | `doctor`, `version` |
| Agent | `mcp stdio` (MCP server) |

## Output Formats

```bash
notebooklm-pp-cli notebook list --json
notebooklm-pp-cli notebook list --dry-run
notebooklm-pp-cli chat ask "NB" "question" --json --agent
```

## Agent Usage

- Add `--agent` for JSON + non-interactive defaults (`--json --no-input --yes`).
- Exit code `0` on success; non-zero on auth/RPC errors.
- Use `doctor --json` before long agent loops to catch expired cookies.

## Cookbook

```bash
# Create notebook and add Wikipedia source
notebooklm-pp-cli notebook create "Agent Research" --json
notebooklm-pp-cli source add "Agent Research" https://en.wikipedia.org/wiki/NotebookLM --json

# Grounded Q&A with citations
notebooklm-pp-cli chat ask "Agent Research" "List three facts from the sources" --json

# Generate quiz and wait for completion
notebooklm-pp-cli studio generate-quiz "Agent Research" --wait --json

# Rename then delete (cleanup)
notebooklm-pp-cli notebook rename "Agent Research" "Agent Research (done)" --json
notebooklm-pp-cli notebook delete "Agent Research (done)" --json

# Offline search after sync
notebooklm-pp-cli sync --db ~/.local/share/notebooklm-pp-cli/cache.db --json
notebooklm-pp-cli search "Agent" --json

# Share visibility check
notebooklm-pp-cli share show "My Notebook" --json

# MCP stdio server for Claude Desktop / agents
notebooklm-pp-mcp
```

## Health Check

```bash
$ notebooklm-pp-cli doctor --json
{
  "checks": [
    {"name": "go", "status": "ok", "detail": "go1.24.0"},
    {"name": "auth", "status": "ok", "detail": "/Users/you/.config/notebooklm-pp-cli/config.toml"},
    {"name": "session", "status": "ok", "detail": "bootstrap tokens"}
  ]
}
```

## Configuration

| Variable / file | Description |
|-----------------|-------------|
| `~/.config/notebooklm-pp-cli/config.toml` | Cookie auth header (`auth_header`) and optional `base_url` override |
| `~/.local/share/notebooklm-pp-cli/cache.db` | SQLite notebook cache (override with `sync --db` / `search --db`) |
| `--config` | Alternate config file path |

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `not authenticated` | `notebooklm-pp-cli auth login --chrome` |
| `null result` / empty lists | Re-login; confirm Google account has NotebookLM access in Chrome |
| `search` returns nothing | Run `sync --json` first |
| `pycookiecheat not installed` | `pip install pycookiecheat` or use `--cookies-file` |

### API-specific
- **notebook list returns empty or RPC errors** — Re-run `notebooklm-pp-cli auth login --chrome` after logging into Google in Chrome, then `doctor --json`.
- **search returns no results** — Run `notebooklm-pp-cli sync --json` first to populate the local cache.

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

## Development

```bash
go test ./...
go build ./cmd/notebooklm-pp-cli/
```

## License

Apache-2.0

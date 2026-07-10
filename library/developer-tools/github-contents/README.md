# GitHub Contents CLI

**Download any GitHub folder without cloning — plan it first, verify it after, re-sync only what changed.**

Addresses owner/repo/path#ref like degit, but streams big files past the 1 MB contents-API limit via the raw CDN, preserves folder structure, and gives every command --json for agents. The plan/verify/sync-dir loop — preview cost before downloading, prove local files match remote by git blob SHA, then fetch only the diff — exists in no other GitHub download tool.

## Install

The recommended path installs both the `github-contents-pp-cli` binary and the `pp-github-contents` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install github-contents
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install github-contents --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install github-contents --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install github-contents --agent claude-code
npx -y @mvanhorn/printing-press-library install github-contents --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/cmd/github-contents-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/github-contents-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install github-contents --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-github-contents --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-github-contents --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install github-contents --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/github-contents-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GITHUB_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/cmd/github-contents-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "github-contents": {
      "command": "github-contents-pp-mcp",
      "env": {
        "GITHUB_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check first — works without a token; GITHUB_TOKEN raises the rate limit from 60 to 5000 req/h
github-contents-pp-cli doctor --dry-run

# One API call: how big is this folder, what file types are in it
github-contents-pp-cli stats mjwoon/AI-readings/books

# Preview the download: every file, sizes, total bytes, API cost — nothing is written
github-contents-pp-cli plan mjwoon/AI-readings/books

# Download recursively, preserving folder structure; big files stream via the raw CDN
github-contents-pp-cli fetch mjwoon/AI-readings/books --out ./books

# Prove every local file matches the remote by git blob SHA — no re-download
github-contents-pp-cli verify ./books mjwoon/AI-readings/books

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Plan-verify-sync loop
- **`plan`** — Preview exactly what a fetch would download — file list, sizes, total bytes, API-request cost vs your remaining quota, and LFS-pointer warnings — before spending any bandwidth.

  _Run this before fetch to know a download is 122 files / 1.9 GB and costs only 2 API requests, instead of finding out mid-download._

  ```bash
  github-contents-pp-cli plan mjwoon/AI-readings/books --agent
  ```
- **`verify`** — Check whether a previously downloaded directory still matches the remote at a ref — match/changed/missing/extra per file — without re-downloading anything.

  _Proves 'this workspace matches ref X' cheaply — the post-download assertion CI and agents otherwise can't make without re-fetching._

  ```bash
  github-contents-pp-cli verify ./books mjwoon/AI-readings/books --agent
  ```
- **`sync-dir`** — Update an existing downloaded directory in place, fetching only files that changed upstream or are missing locally.

  _The weekly-mirror command: keeps a local collection current for the price of one API call plus only the changed bytes._

  ```bash
  github-contents-pp-cli sync-dir ./books mjwoon/AI-readings/books --agent
  ```

### Remote inspection
- **`stats`** — Size and file-count breakdown of any repo path by subfolder and extension, plus the largest files, from a single API request.

  _Answers 'how big is this directory and what is in it' in one call before committing to a download._

  ```bash
  github-contents-pp-cli stats mjwoon/AI-readings/books --agent --select by_folder
  ```
- **`search`** — Search previously fetched repo listings from the local store — find files by name or pattern with zero API calls.

  _Lets agents answer 'which repo path had file X' from data already on disk instead of re-walking the API._

  ```bash
  github-contents-pp-cli search "transformers" --limit 20
  ```

## Recipes

### Download a folder, keep the structure

```bash
github-contents-pp-cli fetch mjwoon/AI-readings/books --out ./books
```

Recursive download of one subdirectory; subfolders and filenames (including spaces) arrive intact.

### Preview cost before a big download

```bash
github-contents-pp-cli plan mjwoon/AI-readings/books --agent
```

Machine-readable plan: files, total_bytes, api_cost, lfs_pointers — decide before spending bandwidth.

### Only PDFs, nothing else

```bash
github-contents-pp-cli fetch mjwoon/AI-readings/books --include "*.pdf" --out ./pdfs
```

Glob filters restrict the download without changing the preserved structure.

### Narrow a huge tree listing for an agent

```bash
github-contents-pp-cli trees mjwoon AI-readings main --recursive --agent --select tree.path,tree.size
```

The recursive tree of a repo is thousands of entries; --select keeps only the two fields an agent needs.

### Keep a mirror current

```bash
github-contents-pp-cli sync-dir ./books mjwoon/AI-readings/books --agent
```

One API call to diff by blob SHA, then only changed or new files are streamed down.

## Usage

Run `github-contents-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GITHUB_CONTENTS_CONFIG_DIR`, `GITHUB_CONTENTS_DATA_DIR`, `GITHUB_CONTENTS_STATE_DIR`, or `GITHUB_CONTENTS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GITHUB_CONTENTS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GITHUB_CONTENTS_HOME=/srv/github-contents
github-contents-pp-cli doctor
```

Under `GITHUB_CONTENTS_HOME=/srv/github-contents`, the four dirs resolve to `/srv/github-contents/config`, `/srv/github-contents/data`, `/srv/github-contents/state`, and `/srv/github-contents/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "github-contents": {
      "command": "github-contents-pp-mcp",
      "env": {
        "GITHUB_CONTENTS_HOME": "/srv/github-contents"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GITHUB_CONTENTS_DATA_DIR` overrides an explicit `--home` for that kind. Use `GITHUB_CONTENTS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GITHUB_CONTENTS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `github-contents-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### blobs

Raw git blobs — fetch file content by SHA (works for files up to 100 MB)

- **`github-contents-pp-cli blobs <owner> <repo> <file_sha>`** - Get a blob (base64-encoded content) by its git SHA

### contents

List directories and read files via the repository contents API

- **`github-contents-pp-cli contents <owner> <repo> <path>`** - Get a directory listing (JSON array) or file metadata+content (JSON object, base64, files up to 1 MB). Use `trees <owner> <repo> <tree_sha> --recursive` for large/deep listings and `blobs <owner> <repo> <file_sha>` or download_url for files over 1 MB.

### rate-limit

API quota status — check headroom before and during bulk downloads

- **`github-contents-pp-cli rate-limit`** - Show remaining API quota (5000/h with a token, 60/h without)

### releases

Repository releases and their downloadable assets

- **`github-contents-pp-cli releases latest`** - Get the latest published release
- **`github-contents-pp-cli releases list`** - List releases with their assets
- **`github-contents-pp-cli releases download <owner> <repo>`** - Download release assets (latest, or `--tag`) matching `--pattern` into `--out`; `--list-only` previews matches without downloading

### repos

Repository metadata (default branch, visibility, size)

- **`github-contents-pp-cli repos branches`** - List branches of a repository
- **`github-contents-pp-cli repos commits`** - List commits, optionally filtered to a path (newest first)
- **`github-contents-pp-cli repos get`** - Get repository metadata including the default branch

### tarball

Full-repository snapshot as a .tar.gz

- **`github-contents-pp-cli tarball <owner/repo[#ref]>`** - Download the whole repo at a ref as a tarball via GitHub's archive endpoint (use `fetch` for a subdirectory)

### trees

Git trees — one-request recursive listings of an entire repo or subtree

- **`github-contents-pp-cli trees <owner> <repo> <tree_sha>`** - Get a git tree. With --recursive, returns every file under it in one request (check the truncated flag on huge repos)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`github-contents-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`github-contents-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`github-contents-pp-cli learnings list`** - Inspect taught rows
- **`github-contents-pp-cli learnings forget <query>`** - Undo a teach
- **`github-contents-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`github-contents-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`github-contents-pp-cli teach-pattern`** - Install a query/resource template up front
- **`github-contents-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GITHUB_CONTENTS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `github-contents-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
github-contents-pp-cli blobs mock-value mock-value mock-value

# JSON for scripting and agents
github-contents-pp-cli blobs mock-value mock-value mock-value --json

# Filter to specific fields
github-contents-pp-cli blobs mock-value mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
github-contents-pp-cli blobs mock-value mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
github-contents-pp-cli blobs mock-value mock-value mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
github-contents-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `github-contents-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/github-contents-pp-cli/config.toml`; `--home`, `GITHUB_CONTENTS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GITHUB_TOKEN` | per_call | No (raises rate limit from 60 to 5000 req/h) | Set to your API credential. Public repos work unauthenticated. |
| `GH_TOKEN` | per_call | No (raises rate limit from 60 to 5000 req/h) | Set to your API credential. Public repos work unauthenticated. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `github-contents-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `github-contents-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GITHUB_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **404 on a repo you know exists** — Private repos return 404 (not 403) without auth — export GITHUB_TOKEN=<token> and retry
- **403 or rate-limit errors during listings** — Run github-contents-pp-cli rate-limit show; unauthenticated is 60 req/h — set GITHUB_TOKEN for 5000 req/h (file downloads never count against the limit)
- **plan or fetch reports the tree was truncated** — The repo exceeds 100k entries or 7 MB of tree data — fetch a deeper subpath directly, e.g. fetch owner/repo/sub/dir
- **A downloaded file is a few lines of text starting with 'version https://git-lfs...'** — That file is stored in Git LFS; plan flags these as lfs_pointers — download it from the repo's web UI or with git lfs
- **Paths with spaces return 404** — Quote the whole path argument; the CLI URL-escapes each segment automatically

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**degit**](https://github.com/Rich-Harris/degit) — JavaScript (7300 stars)
- [**DownGit**](https://github.com/MinhasKamal/DownGit) — JavaScript (2000 stars)
- [**giget**](https://github.com/unjs/giget) — TypeScript (1600 stars)
- [**download-directory.github.io**](https://github.com/download-directory/download-directory.github.io) — JavaScript (1400 stars)
- [**tiged**](https://github.com/tiged/tiged) — JavaScript (900 stars)
- [**gitdir**](https://github.com/sdushantha/gitdir) — Python (500 stars)
- [**githubdl**](https://github.com/wilvk/githubdl) — Python
- [**github-mcp-server**](https://github.com/github/github-mcp-server) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

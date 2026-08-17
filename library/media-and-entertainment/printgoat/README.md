# PrintGoat CLI

**Search and reliably download 3D-print files across Printables and Thingiverse (plus search Cults3D) — with real resumable downloads and cross-site duplicate detection nothing else has.**

PrintGoat unifies three separate 3D-model communities into one searchable, scriptable tool instead of three browser tabs. It's the only tool that actually resumes interrupted downloads via HTTP byte-range requests rather than just skipping files that already exist by name, and the only one that tells you when the same design is free on one site and paid on another before you download the wrong copy.

## Install

The recommended path installs both the `printgoat-pp-cli` binary and the `pp-printgoat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install printgoat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install printgoat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install printgoat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install printgoat --agent claude-code
npx -y @mvanhorn/printing-press-library install printgoat --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/printgoat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install printgoat --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-printgoat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-printgoat --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install printgoat --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/printgoat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "printgoat": {
      "command": "printgoat-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Each source has a different auth story: Printables needs nothing at all (its GraphQL backend works fully anonymously for search and download). Thingiverse requires a free self-registered OAuth2 app token (`auth set-token --source thingiverse`). Cults3D requires your account handle plus a self-service API key via HTTP Basic Auth (`auth set-token --source cults3d`) — and by Cults3D's own design, its API can search and browse but never download other users' files, so Cults3D results are search/metadata only.

## Quick Start

```bash
# Health check that works with zero configuration — confirms which sources are reachable and which need auth.
printgoat-pp-cli doctor --dry-run

# Unified search across all three sources; works immediately since Printables needs no auth.
printgoat-pp-cli search "benchy" --agent

# List a model's files in one normalized shape regardless of source.
printgoat-pp-cli files printables:3161 --agent

# Download every file for a model, with real resume if interrupted.
printgoat-pp-cli download printables:3161 --all

# See the cross-site duplicate/price/license comparison that no other tool surfaces.
printgoat-pp-cli duplicates "raspberry pi case" --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-site intelligence
- **`duplicates`** — See instantly when the same design is listed free on one site and paid on another, before you download the wrong one.

  _Reach for this before downloading anything from a paid Cults3D listing — it tells you if the same file exists free elsewhere._

  ```bash
  printgoat-pp-cli duplicates "gopro mount" --agent
  ```
- **`feed`** — Follow a designer across all the sites they post to and see new uploads from any of them in one feed.

  _Check this instead of manually revisiting three separate designer profile pages to see what's new._

  ```bash
  printgoat-pp-cli feed --agent
  ```
- **`formats gaps`** — Find alternatives when a model only ships in one file format and you need another.

  _Use when a search result is STL-only and you need 3MF or STEP — this looks for tag-similar alternatives that include the missing format._

  ```bash
  printgoat-pp-cli formats gaps printables:3161 --agent
  ```

### Provenance, resume & integrity
- **`history diff`** — Know exactly what changed on a model since you last looked — new files, price changes, license changes.

  _Use this instead of re-fetching a model's full detail when you only need to know what's different since last time._

  ```bash
  printgoat-pp-cli history diff printables:3161 --agent
  ```
- **`license audit`** — Flag license conflicts across your entire local library against how you actually intend to use the files.

  _Run this before a commercial print job to catch license conflicts the site's own badge won't warn you about._

  ```bash
  printgoat-pp-cli license audit --library --intent commercial --agent
  ```
- **`job download`** — Batch-download across all three sites as one crash-safe job that actually resumes from where it stopped.

  _Use this for any multi-file or multi-source batch pull; it survives crashes and rate-limit backoff without restarting completed files._

  ```bash
  printgoat-pp-cli job resume job-20260721-01 --agent
  ```
- **`snapshot verify`** — Prove exactly which file version you used for a past print job, even if the upstream model has since changed.

  _Use this for provenance/reproducibility on any job whose output needs to match a prior run exactly._

  ```bash
  printgoat-pp-cli snapshot verify batch-march-orders --agent
  ```
- **`library doctor`** — Find orphaned files, missing files, silent duplicates, and remote listings that have been delisted (and can never be re-fetched).

  _Run periodically to catch link rot before a delisted source becomes unrecoverable if the local copy is ever lost._

  ```bash
  printgoat-pp-cli library doctor --agent
  ```

### Print outcome memory
- **`similar`** — Re-search for alternatives after a bad print, automatically excluding the model and designer that failed you.

  _Use after logging a failed print (`log fail`) instead of re-running a generic search that will surface the same bad result again._

  ```bash
  printgoat-pp-cli similar printables:3161 --agent
  ```
- **`designer stats`** — See your own success/failure rate with a specific designer, pooled across every site they publish on.

  _Check this before downloading a new model from a designer you've printed before, when site-level popularity counts don't reflect your own experience._

  ```bash
  printgoat-pp-cli designer stats "PrintedSolid" --agent
  ```

## Recipes

### Unified search with narrowed output

```bash
printgoat-pp-cli search "cable clip" --agent --select results.name,results.source,results.price,results.license
```

Search results are deeply nested per-source objects; --select trims the response to just the fields an agent needs for a quick comparison.

### Find the cheapest legitimate copy of a design

```bash
printgoat-pp-cli duplicates "phone stand" --agent
```

Surfaces the same design across sources with price and license side by side before you commit to a paid download.

### Resume an interrupted batch job

```bash
printgoat-pp-cli job resume job-20260721-01 --agent
```

Continues a multi-source download job from its last completed byte offset per file instead of restarting.

### Audit your library before a commercial print run

```bash
printgoat-pp-cli license audit --library --intent commercial --agent
```

Flags any locally downloaded file whose license doesn't permit the commercial use you've declared.

## Usage

Run `printgoat-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PRINTGOAT_CONFIG_DIR`, `PRINTGOAT_DATA_DIR`, `PRINTGOAT_STATE_DIR`, or `PRINTGOAT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PRINTGOAT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PRINTGOAT_HOME=/srv/printgoat
printgoat-pp-cli doctor
```

Under `PRINTGOAT_HOME=/srv/printgoat`, the four dirs resolve to `/srv/printgoat/config`, `/srv/printgoat/data`, `/srv/printgoat/state`, and `/srv/printgoat/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "printgoat": {
      "command": "printgoat-pp-mcp",
      "env": {
        "PRINTGOAT_HOME": "/srv/printgoat"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PRINTGOAT_DATA_DIR` overrides an explicit `--home` for that kind. Use `PRINTGOAT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PRINTGOAT_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `printgoat-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### categories

Thingiverse categories

- **`printgoat-pp-cli categories list`** - List top-level categories
- **`printgoat-pp-cli categories things`** - List things in a category

### collections

Thingiverse collections

- **`printgoat-pp-cli collections get`** - Get a collection by ID
- **`printgoat-pp-cli collections things`** - List things in a collection

### creations

Cults3D creations (designs)

- **`printgoat-pp-cli creations get`** - Get a Cults3D creation's detail
- **`printgoat-pp-cli creations search`** - Search Cults3D creations by keyword

### models

Printables 3D models (prints)

- **`printgoat-pp-cli models <id>`** - Get a Printables model's detail and file listing

### search_things

Search Thingiverse things

- **`printgoat-pp-cli search-things <term>`** - Search for things by keyword

### tags

Thingiverse tags

- **`printgoat-pp-cli tags <tag>`** - List things with a tag

### things

Thingiverse things (3D models)

- **`printgoat-pp-cli things categories`** - List categories for a thing
- **`printgoat-pp-cli things files`** - List files for a thing
- **`printgoat-pp-cli things get`** - Get a thing by ID
- **`printgoat-pp-cli things images`** - List images for a thing
- **`printgoat-pp-cli things tags`** - List tags for a thing

### users

Thingiverse users

- **`printgoat-pp-cli users collections`** - List a user's collections
- **`printgoat-pp-cli users get`** - Get a user's public profile
- **`printgoat-pp-cli users things`** - List a user's published things


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`printgoat-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`printgoat-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`printgoat-pp-cli learnings list`** - Inspect taught rows
- **`printgoat-pp-cli learnings forget <query>`** - Undo a teach
- **`printgoat-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`printgoat-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`printgoat-pp-cli teach-pattern`** - Install a query/resource template up front
- **`printgoat-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PRINTGOAT_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `printgoat-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
printgoat-pp-cli categories list

# JSON for scripting and agents
printgoat-pp-cli categories list --json

# Filter to specific fields
printgoat-pp-cli categories list --json --select id,name,status

# Dry run — show the request without sending
printgoat-pp-cli categories list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
printgoat-pp-cli categories list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure (`--allow-partial-failure` downgrades to a warning), `7` rate limited, `10` config error.

## Health Check

```bash
printgoat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `printgoat-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/printgoat-pp-cli/config.toml`; `--home`, `PRINTGOAT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Thingiverse requests suddenly all fail after working fine** — Thingiverse invalidates tokens outright on abusive rate-limit hits (300 req/5min) rather than just returning 429 — re-run `auth set-token --source thingiverse` with a fresh token.
- **Cults3D returns 401 even with an API key set** — Cults3D uses HTTP Basic Auth with your account handle as the username, not just the API key — set both via `auth set-token --source cults3d`.
- **Printables search intermittently times out or returns an HTML challenge page** — Printables' GraphQL backend sits behind Cloudflare; retry with backoff — `doctor` will report if it's currently blocking.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `PRINTGOAT_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**thingiverse_downloader**](https://github.com/jcarolinares/thingiverse_downloader) — Python (56 stars)
- [**CultsDL**](https://github.com/pattonwebz/CultsDL) — TypeScript (13 stars)
- [**printables-cli-api**](https://github.com/GhostTypes/printables-cli-api) — Python (8 stars)
- [**thingiverse-easy-download**](https://github.com/ajh1138/thingiverse-easy-download) — JavaScript (2 stars)
- [**thingfinder**](https://github.com/nukleas/thingfinder) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

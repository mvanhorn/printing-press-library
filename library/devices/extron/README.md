# Extron CLI

**Every Extron spec sheet and user manual, browsable and searchable offline with revision tracking no other Extron tool has.**

Extron publishes spec sheets and manuals at extron.com, but finding them means browser tabs, letter-scoped tables, and a WAF that blocks plain curl. extron-pp-cli syncs the whole literature catalog into a local database, then lets you list, search, download, and track revisions from the terminal — including what's new (literature recent), which downloaded docs went stale (literature updates), and which doc types a rack's models are missing (catalog completeness).

## Install

The recommended path installs both the `extron-pp-cli` binary and the `pp-extron` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install extron
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install extron --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install extron --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install extron --agent claude-code
npx -y @mvanhorn/printing-press-library install extron --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/extron/cmd/extron-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/extron-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine ./extron-pp-cli`. On Unix, mark it executable: `chmod +x ./extron-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install extron --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-extron --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-extron --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install extron --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/extron-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/extron/cmd/extron-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "extron": {
      "command": "extron-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check that works without credentials — confirms the CLI and its config are sane.
extron-pp-cli doctor --dry-run

# Fetch the Extron literature catalog into the local store — everything else reads it.
# This baseline pass takes the first index page per letter bucket; add --full for the
# complete catalog (see "catalog sync" under Commands).
extron-pp-cli catalog sync

# Browse the Manual category for M products (Matrix, MAV) straight from the official index.
extron-pp-cli literature list --category manual --letter m

# Resolve a product name to its official documents with revision and date metadata.
extron-pp-cli literature get "MAV Plus Series" --json

# Search the synced catalog offline — no website, no WAF.
extron-pp-cli search "DTP2" --type literature

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local catalog that compounds
- **`catalog sync`** — Build the local Extron literature catalog, walking the alphabetical index the site only exposes a letter at a time. Bare `catalog sync` fetches the first index page per letter bucket — a fast baseline of roughly 1,200 documents, with large categories truncated at the page-1 ceiling. `--full` follows each category's pagination and is what produces the complete catalog (roughly 3,600 documents and up). A failed letter bucket is retried and skipped rather than aborting the crawl, so a 36-bucket `--full` walk survives a flaky bucket.

  _Run this first — every local read depends on it, and it is not the same command as top-level `sync`._

  ```bash
  extron-pp-cli catalog sync --full --timeout 15m --max-duration 4h --json
  ```
- **`literature updates`** — See which downloaded spec sheets and manuals have a newer revision available upstream, so project shares never run stale docs.

  _Use this before commissioning or re-quoting to catch docs superseded since the last sync._

  ```bash
  extron-pp-cli literature updates --dir ./docs --json
  ```
- **`catalog completeness`** — Per-model gap report across Brochure, Declaration of Conformity, Design Guide, Product Guide, Manual, and Revit BIM — see which doc types each model is missing.

  _Use it before a bid submittal or commissioning checklist to catch missing compliance or reference docs._

  ```bash
  extron-pp-cli catalog completeness --bom ./rack.csv --json
  ```
- **`literature recent`** — Newest Extron literature across the whole library, ordered by date, filterable by category and age.

  _Use it to track manual/firmware-doc releases without re-checking the website._

  ```bash
  extron-pp-cli literature recent --days 30 --category manual --json --select title,date,category,url
  ```
- **`literature family`** — Browse every document for a product family (DTP, MAV, IPL, DVS, ...) across all alphabetical letters at once.

  _Use it to pull every doc for a family before writing control code or a design guide review._

  ```bash
  extron-pp-cli literature family dtp --json
  ```
- **`catalog verify`** — Compare local PDF sizes against the download ledger to flag truncated or mismatched downloads.

  _Use it after a batch download or sync to confirm every PDF landed complete and current._

  ```bash
  extron-pp-cli catalog verify --dir ./docs --json
  ```

### Project-driven assembly
- **`literature rack`** — Assemble the full official doc set for every model in a rack bill of materials — report or batch-download in one pass.

  _Use it when a project's doc binder must match the exact gear list on the job._

  ```bash
  extron-pp-cli literature rack --bom ./rack.csv --download --dir ./docs
  ```

## Recipes

### First run: build the catalog

Everything local — `search`, `literature list`, `catalog completeness`, `catalog verify` — reads the catalog this builds, so run it before trusting any local result. There are two passes, and the difference matters:

```bash
# Fast baseline: first index page per letter bucket, ~1,200 docs.
# Large categories are truncated here — this is NOT the complete catalog.
extron-pp-cli catalog sync

# Complete catalog: follows every category's pagination, ~3,600 docs and up.
extron-pp-cli catalog sync --full --timeout 15m --max-duration 4h --json
```

Run the `--full` pass before any answer that depends on completeness — a rack doc binder, a completeness report, a bid compliance check. If you only ran the baseline, a document that exists at Extron can be missing from local search with no error to tell you.

`--timeout` bounds each letter bucket, `--max-duration` bounds the whole crawl. Failed buckets are retried and then skipped; check `letters_failed` and `errors` in the summary, then re-run just those buckets:

```bash
extron-pp-cli catalog sync --letters A,Q --full --timeout 15m
```

### What did Extron release this month

```bash
extron-pp-cli literature recent --days 30 --json --select title,date,category,url
```

Agent-friendly list of the newest docs with a narrow --select so the payload stays small.

### Stale-doc sweep before commissioning

```bash
extron-pp-cli literature updates --dir ./docs --download --dry-run
```

See which project PDFs are superseded and preview the re-download without touching disk.

### Rack doc binder

```bash
extron-pp-cli literature rack --bom ./rack.csv --download --dir ./docs
```

Pull the full official doc set for every model on the job in one pass.

### Family reference pull

```bash
extron-pp-cli literature family dtp --json
```

All DTP-family docs across every letter of the index, in machine-readable form.

### Bid compliance check

```bash
extron-pp-cli catalog completeness --bom ./rack.csv --json
```

Which models are missing a Declaration of Conformity or manual before the submittal.

## Usage

Run `extron-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, and the download ledger |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `EXTRON_CONFIG_DIR`, `EXTRON_DATA_DIR`, `EXTRON_STATE_DIR`, or `EXTRON_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `EXTRON_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export EXTRON_HOME=/srv/extron
extron-pp-cli doctor
```

Under `EXTRON_HOME=/srv/extron`, the four dirs resolve to `/srv/extron/config`, `/srv/extron/data`, `/srv/extron/state`, and `/srv/extron/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "extron": {
      "command": "extron-pp-mcp",
      "env": {
        "EXTRON_HOME": "/srv/extron"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `EXTRON_DATA_DIR` overrides an explicit `--home` for that kind. Use `EXTRON_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `EXTRON_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. This CLI requires no credentials, so `credentials.toml` stays empty unless you write it yourself. Run `extron-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### catalog sync — build the local catalog (run this first)

Fetch the Extron literature catalog into the local store. Every local read — `search`, `literature list`, `catalog completeness`, `catalog verify`, `literature recent`, `literature updates` — depends on it.

- **`extron-pp-cli catalog sync`** - Fetch the **first index page per letter bucket** (0-9, A-Z). A fast baseline of roughly 1,200 documents, **not the complete catalog**: any category with more than one page of results is truncated at the page-1 ceiling.
- **`extron-pp-cli catalog sync --full`** - Also follow each category's pagination. **This is what produces the complete catalog** (roughly 3,600 documents and up).
- **`extron-pp-cli catalog sync --letters A,B,C`** - Narrow to specific letter buckets
- **`extron-pp-cli catalog sync --full --max-duration 4h --retries 3`** - Long crawl with a bigger overall budget and more per-letter retries

Use the baseline to get something searchable quickly; run `--full` before any answer that depends on the catalog being complete, such as a rack doc binder or a bid compliance check. `--max-pages` caps pagination per category and truncates in the same way, so leave it unset for a genuinely complete crawl.

A letter bucket that fails is retried (`--retries`, default 2) and then skipped, so one bad bucket does not discard the rest of the crawl. Skipped buckets appear in the summary's `errors` array and in `letters_failed`; the run exits non-zero only when every bucket failed, or when `--strict` is passed. The root `--timeout` bounds each letter bucket; `--max-duration` (default 30m) bounds the whole crawl.

> `catalog sync` is not the same command as the top-level `sync`. Top-level `sync` walks the generated `literature` endpoint resource and refreshes entity lookups; it does not build the catalog.

### literature

Extron literature library index (spec sheets, manuals, guides)

- **`extron-pp-cli literature`** - Fetch the alphabetical literature index for a letter
- **`extron-pp-cli literature list`** - List literature from the local catalog, filterable by category and letter
- **`extron-pp-cli literature get`** - Resolve a product or document name to its official Extron literature
- **`extron-pp-cli literature download`** - Download official Extron spec sheets and manuals as PDFs
- **`extron-pp-cli literature recent`** - Newest Extron literature across the whole library, ordered by date
- **`extron-pp-cli literature updates`** - See which downloaded docs have a newer revision available upstream
- **`extron-pp-cli literature family`** - Browse every document for a product family (DTP, MAV, IPL, DVS, ...)
- **`extron-pp-cli literature rack`** - Assemble the full official doc set for every model in a rack BOM

### catalog

Local-catalog reporting

- **`extron-pp-cli catalog completeness`** - Per-model gap report across Brochure, Declaration of Conformity, Design Guide, Product Guide, Manual, Revit BIM
- **`extron-pp-cli catalog verify`** - Compare local PDF sizes and revisions against the download ledger


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`extron-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`extron-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`extron-pp-cli learnings list`** - Inspect taught rows
- **`extron-pp-cli learnings forget <query>`** - Undo a teach
- **`extron-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`extron-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`extron-pp-cli teach-pattern`** - Install a query/resource template up front
- **`extron-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `EXTRON_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `extron-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
extron-pp-cli literature

# JSON for scripting and agents
extron-pp-cli literature --json

# Filter to specific fields
extron-pp-cli literature --json --select id,name,status

# Dry run — show the request without sending
extron-pp-cli literature --dry-run

# Agent mode — JSON + compact + no prompts in one flag
extron-pp-cli literature --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
extron-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `extron-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/extron-pp-cli/config.toml`; `--home`, `EXTRON_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Connection reset / read tcp ... connection reset by peer on sync or list** — Extron's WAF intermittently resets non-browser connections. Re-run the command — retries succeed. The CLI retries once automatically.
- **search returns nothing right after install** — Run `extron-pp-cli sync` first — search reads the local catalog, which starts empty.
- **literature updates shows everything as stale** — The drift check compares local filenames against the catalog; re-download with `extron-pp-cli literature updates --download` to normalize filenames.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**av-toolkit**](https://github.com/simonlpaige/av-toolkit) — Python
- [**pyextron**](https://github.com/zombielinux/pyextron) — Python
- [**ha-extron**](https://github.com/NitorCreations/ha-extron) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

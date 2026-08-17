# AVer USA CLI

**Every AVer USA manual, spec sheet, and white paper in a local, type-filtered catalog — with spec comparison and link audits no AVer site offers.**

averusa.com hides its manuals, spec sheets, and white papers behind a Salesforce portal maze and per-model PDFs. averusa-pp-cli syncs the whole catalog into a local database, then answers the questions integrators actually ask: which model fits (compare), what are its specs (specs), what docs exist per model (coverage, docs pack), what changed since last sync (whats-new), and which PDF links are dead (doctor).

## Install

The recommended path installs both the `averusa-pp-cli` binary and the `pp-averusa` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install averusa
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install averusa --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install averusa --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install averusa --agent claude-code
npx -y @mvanhorn/printing-press-library install averusa --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/averusa/cmd/averusa-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/averusa-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install averusa --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-averusa --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-averusa --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install averusa --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/averusa-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/averusa/cmd/averusa-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "averusa": {
      "command": "averusa-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check that works with no credentials — confirms the CLI and its config are sane.
averusa-pp-cli doctor --dry-run

# Build the local corpus from the support portal and product catalog — everything else reads it.
averusa-pp-cli harvest

# Type-filtered search over the synced catalog, offline.
averusa-pp-cli docs search "CAM570" --type user-manual

# Grab the whole offline doc bag for a model before heading to the jobsite.
averusa-pp-cli docs pack CAM570 --out ./job-570

# Side-by-side specs from the datasheets for a bid comparison.
averusa-pp-cli compare CAM570 CAM550 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local catalog that compounds
- **`compare`** — Side-by-side spec fields for two or more AVer models from their datasheets, ready for bid comparisons and RFP tables. Spec fields come from datasheet PDFs extracted by `harvest --with-specs`.

  _Use this instead of opening N datasheet PDFs to answer 'which model fits?' for a bid._

  ```bash
  averusa-pp-cli compare CAM570 CAM550 --agent
  ```
- **`specs`** — One model's full spec fields as clean text or JSON for spec-compliance tables and agent pipelines. Fields come from datasheet PDFs extracted by `harvest --with-specs`.

  _Use this to fill an RFP spec-compliance row without opening the PDF._

  ```bash
  averusa-pp-cli specs CAM570 --json --agent
  ```
- **`whats-new`** — List documents and products added or updated since the last sync, filterable by age.

  _Use this to track new manuals or firmware docs across a fleet without re-checking the website._

  ```bash
  averusa-pp-cli whats-new --since 30d --json
  ```
- **`coverage`** — Per-model doc-type availability matrix for a category, flagging which models are missing manuals, spec sheets, or white papers.

  _Use this before a recommendation or commissioning checklist to catch missing compliance docs._

  ```bash
  averusa-pp-cli coverage conference-camera
  ```

### Reachability mitigation
- **`docs audit`** — HEAD-checks every document URL in the catalog and flags 404s and soft-404 shells, caching last-checked status locally.

  _Use this before pushing docs to a shared drive to catch dead or mislinked PDFs while they are still cheap to fix._

  ```bash
  averusa-pp-cli docs audit
  ```

### Offline job-site workflow
- **`docs pack`** — Batch-download every document for a model into one offline folder with stable <model>-<type> names, with a --dry-run preview. Downloads need entityIds, which `harvest` resolves at sync time.

  _Use this to pre-stage a job bag of manuals before driving to a site with no signal._

  ```bash
  averusa-pp-cli docs pack CAM570 --out ./job-570 --dry-run
  ```

### Service-specific intelligence
- **`products status`** — Flag which models AVer lists as discontinued, filterable by category.

  _Use this before specing a model into a school bid so a discontinued unit never ships in the quote._

  ```bash
  averusa-pp-cli products status --category conference-camera --json
  ```

## Recipes

### Field kit before you drive out

```bash
averusa-pp-cli docs pack CAM570 --out ./job-570
```

One command assembles the whole offline doc bag for a model with stable names — no portal clicking at the jobsite.

### Bid spec table

```bash
averusa-pp-cli specs CAM550 --json --agent
```

Structured spec fields straight into an RFP compliance table or agent pipeline.

### Which model fits?

```bash
averusa-pp-cli compare CAM570 CAM550 --agent
```

Side-by-side datasheet specs so a recommendation never relies on opening two PDFs.

### Audit the shared drive

```bash
averusa-pp-cli docs audit
```

Catches dead and mislinked PDFs before installers do — the CAM570 page currently mislinks cam520pro3-datasheet.pdf.

### White papers for an eval

```bash
averusa-pp-cli docs search "white paper" --type white-paper --json --select title,doc_type,model
```

Evaluation material filtered to white papers, narrowed with --select so agents don't parse the full payload.

## Usage

Run `averusa-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AVERUSA_CONFIG_DIR`, `AVERUSA_DATA_DIR`, `AVERUSA_STATE_DIR`, or `AVERUSA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AVERUSA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AVERUSA_HOME=/srv/averusa
averusa-pp-cli doctor
```

Under `AVERUSA_HOME=/srv/averusa`, the four dirs resolve to `/srv/averusa/config`, `/srv/averusa/data`, `/srv/averusa/state`, and `/srv/averusa/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "averusa": {
      "command": "averusa-pp-mcp",
      "env": {
        "AVERUSA_HOME": "/srv/averusa"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AVERUSA_DATA_DIR` overrides an explicit `--home` for that kind. Use `AVERUSA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AVERUSA_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `averusa-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### docs

AVer USA support-portal knowledge articles and attached files

- **`averusa-pp-cli docs download`** - Download an article's attached file (PDF)
- **`averusa-pp-cli docs get`** - Fetch a knowledge article as clean text (crawler-UA SSR)
- **`averusa-pp-cli docs list`** - Fetch the support-portal article sitemap (all knowledge-article URLs)

### products

AVer USA product catalog and datasheets

- **`averusa-pp-cli products get`** - Fetch a product page as clean text (spec links, downloads)
- **`averusa-pp-cli products list`** - Fetch the main-site sitemap (product pages, categories)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`averusa-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`averusa-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`averusa-pp-cli learnings list`** - Inspect taught rows
- **`averusa-pp-cli learnings forget <query>`** - Undo a teach
- **`averusa-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`averusa-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`averusa-pp-cli teach-pattern`** - Install a query/resource template up front
- **`averusa-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `AVERUSA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `averusa-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
averusa-pp-cli docs list

# JSON for scripting and agents
averusa-pp-cli docs list --json

# Filter to specific fields
averusa-pp-cli docs list --json --select title,doc_type,model

# Dry run — show the request without sending
averusa-pp-cli docs list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
averusa-pp-cli docs list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select title,doc_type` returns only the fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
averusa-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `averusa-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/averusa-pp-cli/config.toml`; `--home`, `AVERUSA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **docs search returns nothing right after install** — Run `averusa-pp-cli harvest` first — search reads the local corpus, which starts empty.
- **docs download fails for an article with no known entityId** — fileField downloads need a Salesforce entityId, which `harvest` now resolves for all 737 articles; articles without an attached file (the majority) return no-body 204, and `docs audit` reports which URLs are unverifiable.
- **a PDF link returns a 200 HTML shell instead of a PDF** — averusa.com serves 61301-byte soft-404 shells for guessed paths. Run `averusa-pp-cli docs audit` to audit link health, then re-sync.

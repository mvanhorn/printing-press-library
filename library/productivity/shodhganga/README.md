# Shodhganga CLI

**Structured, scriptable, agent-native access to India's national reservoir of PhD theses — the machine API Shodhganga never shipped.**

Shodhganga hosts 600,000+ Indian doctoral theses from 900+ universities but exposes no OAI, REST, or OpenSearch API — only HTML. This CLI turns every thesis into a structured Dublin Core record you can search and export as JSON, then mirror a research area into a local database for instant offline analysis with guide-centric indexes, university profiles, and subject trends the web UI can't produce.

Learn more at [Shodhganga](https://shodhganga.inflibnet.ac.in).

## Install

The recommended path installs both the `shodhganga-pp-cli` binary and the `pp-shodhganga` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install shodhganga
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install shodhganga --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install shodhganga --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install shodhganga --agent claude-code
npx -y @mvanhorn/printing-press-library install shodhganga --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/cmd/shodhganga-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shodhganga-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install shodhganga --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-shodhganga --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-shodhganga --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install shodhganga --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shodhganga-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/cmd/shodhganga-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "shodhganga": {
      "command": "shodhganga-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the CLI is wired and the site is reachable before searching
shodhganga-pp-cli doctor --dry-run

# keyword-search all theses and get structured rows
shodhganga-pp-cli thesis search --query "black hole physics" --limit 10

# pull the full Dublin Core metadata record for one thesis (accepts 305247 or 10603/305247)
shodhganga-pp-cli thesis get 305247 --json

# mirror a research area into the local store for offline analysis
shodhganga-pp-cli harvest "machine learning" --limit 100

# list every thesis a supervisor guided (harvest that area first)
shodhganga-pp-cli guide "Ghosh, Sushant G" --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`harvest`** — Mirror a research area's theses into a local database so you can search and analyze offline without re-hitting the site.

  _Reach for this first when an agent will run many follow-up queries over the same research area — harvest once, then everything else is instant and offline._

  ```bash
  shodhganga-pp-cli harvest "machine learning" --limit 5
  ```
- **`guide`** — List every thesis supervised by a given research guide across the harvested corpus.

  _Use when mapping a supervisor's body of work or building academic-lineage graphs the site can't produce._

  ```bash
  shodhganga-pp-cli guide "Ghosh, Sushant G" --json
  ```

### Corpus analytics
- **`university stats`** — Aggregate a university's theses in the local store into a profile: thesis count, subject spread, and year range.

  _Use to compare institutions' doctoral output or profile a university's research strengths._

  ```bash
  shodhganga-pp-cli university stats "Jamia Millia Islamia" --json
  ```
- **`trends`** — Show how thesis counts for a subject change over completion year across the harvested corpus.

  _Use to spot rising or declining research areas in Indian doctoral output._

  ```bash
  shodhganga-pp-cli trends --subject Physics --json
  ```
- **`similar`** — Find theses in the local store that share the most subject keywords with a given thesis.

  _Use to expand a literature review outward from one relevant thesis._

  ```bash
  shodhganga-pp-cli similar 10603/305247 --json
  ```

## Recipes

### Structured metadata for a known thesis

```bash
shodhganga-pp-cli thesis get 305247 --json --select title,researcher,guides,university,keywords
```

Pull just the fields you need from the full Dublin Core record; deeply nested output narrows cleanly with --select.

### Harvest a research area for offline analysis

```bash
shodhganga-pp-cli harvest "quantum computing" --limit 200
```

Mirror a topic once; then trends, guide, similar, and 'search --data-source local' all run instantly against the local store.

### Map a supervisor's students

```bash
shodhganga-pp-cli guide "Ghosh, Sushant G" --json
```

After harvesting the relevant area, list every thesis a research guide supervised.

### Expand a literature review

```bash
shodhganga-pp-cli similar 305247
```

Find theses sharing the most subject keywords with a relevant one, ranked by overlap.

## Usage

Run `shodhganga-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SHODHGANGA_CONFIG_DIR`, `SHODHGANGA_DATA_DIR`, `SHODHGANGA_STATE_DIR`, or `SHODHGANGA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SHODHGANGA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SHODHGANGA_HOME=/srv/shodhganga
shodhganga-pp-cli doctor
```

Under `SHODHGANGA_HOME=/srv/shodhganga`, the four dirs resolve to `/srv/shodhganga/config`, `/srv/shodhganga/data`, `/srv/shodhganga/state`, and `/srv/shodhganga/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "shodhganga": {
      "command": "shodhganga-pp-mcp",
      "env": {
        "SHODHGANGA_HOME": "/srv/shodhganga"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SHODHGANGA_DATA_DIR` overrides an explicit `--home` for that kind. Use `SHODHGANGA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SHODHGANGA_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `shodhganga-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### browse

Browse theses by facet (title, author, subject, date)

- **`shodhganga-pp-cli browse`** - Browse theses by title, author, subject, keyword, or date

### thesis

Search and retrieve Indian PhD theses

- **`shodhganga-pp-cli thesis get`** - Get a thesis item page by handle number (the NNNNN in 10603/NNNNN)
- **`shodhganga-pp-cli thesis search`** - Search theses by keyword across all of Shodhganga

### university

Look up universities and collections (DSpace communities)

- **`shodhganga-pp-cli university <id>`** - Get a university/community page by handle number


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`shodhganga-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`shodhganga-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`shodhganga-pp-cli learnings list`** - Inspect taught rows
- **`shodhganga-pp-cli learnings forget <query>`** - Undo a teach
- **`shodhganga-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`shodhganga-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`shodhganga-pp-cli teach-pattern`** - Install a query/resource template up front
- **`shodhganga-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SHODHGANGA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `shodhganga-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
shodhganga-pp-cli browse

# JSON for scripting and agents
shodhganga-pp-cli browse --json

# Filter to specific fields
shodhganga-pp-cli browse --json --select id,name,status

# Dry run — show the request without sending
shodhganga-pp-cli browse --dry-run

# Agent mode — JSON + compact + no prompts in one flag
shodhganga-pp-cli browse --agent
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
shodhganga-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `shodhganga-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/shodhganga-pp-cli/config.toml`; `--home`, `SHODHGANGA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **thesis search returns no rows for a topic you expect results for** — Shodhganga tokenizes loosely; try a broader single-keyword query, or raise --limit
- **guide/similar/trends/university-stats return empty** — these read the local store — run 'shodhganga-pp-cli harvest <query>' first to populate it
- **thesis get says handle not found** — pass the bare number (305247), the full handle (10603/305247), or the hdl.handle.net URI; all are accepted
- **thesis search or harvest hangs or times out** — Shodhganga's search backend is occasionally overloaded and slow; item lookups (thesis get) stay fast. Retry, lower --limit, or raise --timeout (e.g. --timeout 90s).

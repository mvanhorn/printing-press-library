# New England Journal of Medicine CLI

**Browse, search, and triage New England Journal of Medicine articles from the terminal, backed by a local corpus and a browser-compatible transport that clears Cloudflare.**

NEJM has no public API. This CLI turns its public surfaces into an offline-queryable, agent-native corpus: the current issue and recently-published feeds, per-article abstracts and metadata by DOI, and full-text search over everything you have synced. Novel local-state commands like 'since', 'digest', 'reading-list', 'trends', and 'open-access' do things the website and PubMed wrappers cannot.

Created by [@laci141](https://github.com/laci141) (laci141).

## Install

The recommended path installs both the `nejm-pp-cli` binary and the `pp-nejm` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nejm
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nejm --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nejm --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nejm --agent claude-code
npx -y @mvanhorn/printing-press-library install nejm --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/nejm/cmd/nejm-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nejm-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install nejm --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nejm --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nejm --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install nejm --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nejm-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/nejm/cmd/nejm-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nejm": {
      "command": "nejm-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account or API key is required for the public surfaces this CLI uses (RSS feeds and server-rendered article metadata). Full article text remains subscription-gated on nejm.org; the CLI reports a free/paywalled flag and never circumvents the paywall.

## Quick Start

```bash
# Confirm the CLI is wired and NEJM is reachable via the Surf transport.
nejm-pp-cli doctor --dry-run


# Pull the current issue and recently-published feeds into the local store.
nejm-pp-cli sync


# List the current issue's articles with authors and pages.
nejm-pp-cli current


# Offline full-text search over the synced corpus.
nejm-pp-cli search "sepsis"


# Fetch one article's abstract and metadata by DOI.
nejm-pp-cli article 10.1056/NEJMoa2506905

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`since`** — Show NEJM articles first seen in your local corpus within a time window.

  _Reach for this when an agent needs only what changed since the last check, not the whole issue._

  ```bash
  nejm-pp-cli since 48h --agent
  ```
- **`digest`** — Triage the current issue grouped by specialty or type with abstracts and free/paywalled flags.

  _Use to summarize the week's NEJM issue in one structured payload instead of fetching each article._

  ```bash
  nejm-pp-cli digest --agent
  ```
- **`reading-list`** — Queue NEJM articles by DOI locally and track read/unread state.

  _Use to persist a clinician's to-read queue across sessions without an account._

  ```bash
  nejm-pp-cli reading-list add 10.1056/NEJMoa2506905
  ```

### Corpus analytics

- **`trends`** — Show how NEJM output distributes by specialty, article type, and issue across the synced corpus.

  _Use to quantify NEJM coverage patterns no single page exposes._

  ```bash
  nejm-pp-cli trends --agent
  ```
- **`open-access`** — Surface free full-text NEJM articles in the corpus, newest first.

  _Use for unsubscribed readers who can only act on freely-readable articles._

  ```bash
  nejm-pp-cli open-access --agent --limit 20
  ```

## Recipes


### Current issue, agent-native, narrowed fields

```bash
nejm-pp-cli current --agent --select articles.doi,articles.title,articles.authors
```

Return only the fields an agent needs from the current issue, as compact JSON.

### What's new in the last two days

```bash
nejm-pp-cli since 48h --agent
```

List articles first seen in the local corpus within the window.

### Triage the week by specialty

```bash
nejm-pp-cli digest --group specialty --agent
```

Group the current issue by specialty with one-line abstracts.

### Free full-text only

```bash
nejm-pp-cli open-access --limit 20 --agent
```

Surface freely-readable NEJM articles, newest first.

### Cite an article

```bash
nejm-pp-cli article cite 10.1056/NEJMoa2506905 --format bibtex
```

Emit a BibTeX entry from the local article record.

## Usage

Run `nejm-pp-cli --help` for the full command reference and flag list.

## Commands

### article

NEJM articles by DOI

- **`nejm-pp-cli article <doi>`** - Fetch an NEJM article's metadata and abstract by DOI (e.g. 10.1056/NEJMoa2506905)

### specialty

NEJM specialty sections

- **`nejm-pp-cli specialty`** - List NEJM specialty sections


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nejm-pp-cli article mock-value

# JSON for scripting and agents
nejm-pp-cli article mock-value --json

# Filter to specific fields
nejm-pp-cli article mock-value --json --select id,name,status

# Dry run — show the request without sending
nejm-pp-cli article mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nejm-pp-cli article mock-value --agent
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
nejm-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/nejm-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Empty results from current/search** — Run 'nejm-pp-cli sync' first to populate the local store.
- **Abstract/specialty/type fields are blank** — Fetch the article directly: `nejm-pp-cli article 10.1056/NEJMoaXXXXXXX` pulls abstract, type, specialties, and free/paywalled flag from the article detail page.
- **Plain HTTP returns 403 / 'Just a moment'** — The CLI uses Surf (Chrome-TLS) transport which clears Cloudflare; ensure you are on a current build.
- **Live keyword search against nejm.org fails** — NEJM's search endpoint is Cloudflare-hard; this CLI searches the local synced corpus instead. Sync more issues to widen coverage.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**openpharma-org/pubmed-mcp**](https://github.com/openpharma-org/pubmed-mcp) — TypeScript
- [**JamesANZ/medical-mcp**](https://github.com/JamesANZ/medical-mcp) — TypeScript
- [**Cicatriiz/healthcare-mcp-public**](https://github.com/Cicatriiz/healthcare-mcp-public) — TypeScript
- [**andybrandt/mcp-simple-pubmed**](https://github.com/andybrandt/mcp-simple-pubmed) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

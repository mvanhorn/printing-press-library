# Framer CLI

**Every Framer operation from your terminal — CMS sync, bulk upload, site migration, and a local database no other Framer tool has**

Framer's Server API is powerful but locked behind a JavaScript SDK with no CLI. framer-pp-cli wraps every API method with offline search, dry-run previews, multi-project management, and migration automation that turns site porting from days into hours.

Learn more at [Framer](https://www.framer.com).

Printed by [@ioncom](https://github.com/ioncom) (ioncom).

## Install

The recommended path installs both the `framer-pp-cli` binary and the `pp-framer` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install framer
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install framer --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install framer --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install framer --agent claude-code
npx -y @mvanhorn/printing-press install framer --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/framer/cmd/framer-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/framer-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-framer --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-framer --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-framer skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-framer. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/framer-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FRAMER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/framer/cmd/framer-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "framer": {
      "command": "framer-pp-mcp",
      "env": {
        "FRAMER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Framer uses per-project API keys generated in Site Settings. The CLI stores project profiles locally so you can switch between projects with --project aliases. Requires Node.js 18+ and the framer-api npm package as a runtime bridge.

## Quick Start

```bash
# Verify Node.js, framer-api, and connectivity
framer-pp-cli doctor


# Store your Framer API key
framer-pp-cli auth set-token YOUR_FRAMER_API_KEY


# See what CMS collections exist
framer-pp-cli cms-collections list --json


# Preview a CMS import before committing
framer-pp-cli cms-sync ./posts.csv --collection Blog --dry-run


# Create a preview deployment
framer-pp-cli publish preview

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`snapshot`** — Track your Framer project's evolution over time with full structural snapshots and visual diffs

  _When an agent needs to understand what changed in a project between two points in time, this is the only way_

  ```bash
  framer-pp-cli snapshot --label 'before-redesign' && framer-pp-cli diff latest~1 latest --json
  ```
- **`dashboard`** — Query across all registered Framer projects at once — stale CMS, unpublished changes, collection health

  _When an agent manages multiple client sites and needs a single command to find which projects need attention_

  ```bash
  framer-pp-cli dashboard --json
  ```
- **`cms-schema-diff`** — Declare CMS schema in YAML and diff it against the live project — infrastructure-as-code for Framer CMS

  _When an agent needs to verify CMS schema matches the expected structure before syncing content_

  ```bash
  framer-pp-cli cms-schema-diff ./schema.yaml --json
  ```
- **`cms-validate`** — Find broken collection references, orphan items, and circular refs across your CMS

  _When CMS grows past 3-4 collections, broken references become invisible — this surfaces them_

  ```bash
  framer-pp-cli cms-validate --json
  ```

### Agent-native plumbing
- **`cms-sync`** — Import CMS content from CSV/JSON/Sheets with a preview diff before committing changes

  _When an agent needs to bulk-update CMS content safely without risking accidental overwrites or deletions_

  ```bash
  framer-pp-cli cms-sync ./blog-posts.csv --collection Blog --dry-run --json
  ```
- **`nodes set`** — Read-back verification on every mutation catches Framer's silent attribute rejections

  _When an agent sets node attributes and needs to know whether they actually took effect_

  ```bash
  framer-pp-cli nodes set abc123 --json --dry-run
  ```
- **`publish`** — Pre-publish linting catches broken links, missing CMS references, orphan pages, and empty text nodes

  _When an agent is about to deploy and needs confidence that the site won't ship with broken references_

  ```bash
  framer-pp-cli publish preview --json
  ```
- **`code-push`** — Edit TSX components locally in your editor, then push to Framer — eliminates copy-paste workflow

  _When a developer needs to edit Framer components in their preferred editor without the copy-paste dance_

  ```bash
  framer-pp-cli code-pull HeroSection -o hero.tsx && vim hero.tsx && framer-pp-cli code-push hero.tsx --name HeroSection
  ```

### Migration automation
- **`migrate-scrape`** — Scrape an existing website and generate a complete Framer migration plan with pages, content, and assets

  _When porting an existing site to Framer, this automates the most tedious part of the migration_

  ```bash
  framer-pp-cli migrate-scrape https://old-site.com --depth 3 && framer-pp-cli migrate-apply manifest.json --dry-run
  ```
- **`assets upload`** — Upload a directory of images and auto-bind them to CMS items by filename-to-slug matching

  _When an agent needs to upload dozens of images and link them to the right CMS records in one operation_

  ```bash
  framer-pp-cli assets upload --dry-run --json
  ```
- **`i18n-push`** — Push and pull translations between standard i18n formats (CSV, PO, XLIFF) and Framer's localization system

  _When an agent manages multi-language sites and needs to sync translations from external translation tools_

  ```bash
  framer-pp-cli i18n-push translations.csv --format csv --dry-run
  ```
- **`redirects-generate`** — Auto-generate redirect map by crawling old site's sitemap and fuzzy-matching to Framer page slugs

  _Every site migration needs redirects — this automates what is otherwise a fully manual spreadsheet task_

  ```bash
  framer-pp-cli redirects-generate --old-sitemap https://old-site.com/sitemap.xml --json
  ```
- **`styles-import`** — Import CSS variables or Tailwind config as Framer color and text styles — no manual recreation

  _When porting a site with an existing design system, this eliminates hours of manual style recreation_

  ```bash
  framer-pp-cli styles-import --from tailwind.config.js --json
  ```

## Usage

Run `framer-pp-cli --help` for the full command reference and flag list.

## Commands

### assets

Asset upload and management

- **`framer-pp-cli assets add-image`** - Add image from URL
- **`framer-pp-cli assets upload`** - Upload an image or file asset

### changes

Change tracking between versions

- **`framer-pp-cli changes contributors`** - List contributors who made changes in a version range
- **`framer-pp-cli changes list`** - List added, removed, and modified paths since last publish

### cms-collections

CMS collection management

- **`framer-pp-cli cms-collections create`** - Create a new CMS collection
- **`framer-pp-cli cms-collections get`** - Get collection details including fields and item count
- **`framer-pp-cli cms-collections list`** - List all CMS collections

### cms-fields

CMS field management

- **`framer-pp-cli cms-fields`** - Add, remove, or reorder collection fields

### cms-items

CMS item management

- **`framer-pp-cli cms-items get`** - Get a CMS item with all field data
- **`framer-pp-cli cms-items list`** - List all items in a collection
- **`framer-pp-cli cms-items remove`** - Remove CMS items by ID
- **`framer-pp-cli cms-items upsert`** - Create or update CMS items in batch

### code

Code file management for React components and overrides

- **`framer-pp-cli code create`** - Create a new code file
- **`framer-pp-cli code get`** - Get code file content
- **`framer-pp-cli code list`** - List all code files in the project
- **`framer-pp-cli code typecheck`** - Run TypeScript type checking on a code file

### components

Component management

- **`framer-pp-cli components`** - Add a component instance by URL or name

### custom-code

Custom code injection into site head and body

- **`framer-pp-cli custom-code get`** - Get installed custom code snippets
- **`framer-pp-cli custom-code set`** - Install custom code at head or body insertion points

### fonts

Font management

- **`framer-pp-cli fonts`** - List all available fonts with weights and styles

### i18n

Localization management

- **`framer-pp-cli i18n groups`** - Get localization groups with translation status
- **`framer-pp-cli i18n locales`** - List all project locales

### nodes

Canvas node operations

- **`framer-pp-cli nodes add-svg`** - Add an SVG element to the canvas
- **`framer-pp-cli nodes children`** - Get child nodes of a node
- **`framer-pp-cli nodes clone`** - Clone a node
- **`framer-pp-cli nodes create-frame`** - Create a new frame node
- **`framer-pp-cli nodes create-text`** - Create a new text node
- **`framer-pp-cli nodes find`** - Find nodes by type, attribute, or name
- **`framer-pp-cli nodes get`** - Get node by ID with all attributes
- **`framer-pp-cli nodes remove`** - Remove a node
- **`framer-pp-cli nodes screenshot`** - Capture a screenshot of a node as PNG/JPEG
- **`framer-pp-cli nodes set`** - Set node attributes
- **`framer-pp-cli nodes set-text`** - Set text content of a text node

### pages

Page management

- **`framer-pp-cli pages create`** - Create a new page
- **`framer-pp-cli pages list`** - List all pages in the project

### project

Framer project management

- **`framer-pp-cli project info`** - Get project name, ID, and metadata
- **`framer-pp-cli project user`** - Get current authenticated user info

### publish

Publishing and deployment

- **`framer-pp-cli publish deploy`** - Promote a published deployment to production
- **`framer-pp-cli publish preview`** - Create a preview deployment with a shareable URL

### redirects

URL redirect management

- **`framer-pp-cli redirects add`** - Add redirects to the project
- **`framer-pp-cli redirects list`** - List all project redirects
- **`framer-pp-cli redirects remove`** - Remove redirects from the project

### styles-colors

Color style management

- **`framer-pp-cli styles-colors create`** - Create a new color style
- **`framer-pp-cli styles-colors list`** - List all color styles

### styles-text

Text style management

- **`framer-pp-cli styles-text create`** - Create a new text style
- **`framer-pp-cli styles-text list`** - List all text styles


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
framer-pp-cli changes list

# JSON for scripting and agents
framer-pp-cli changes list --json

# Filter to specific fields
framer-pp-cli changes list --json --select id,name,status

# Dry run — show the request without sending
framer-pp-cli changes list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
framer-pp-cli changes list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
framer-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/framer-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FRAMER_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `framer-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FRAMER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Connection timeout or 'WebSocket error'** — Verify FRAMER_API_KEY is valid and Node.js 18+ is installed: framer-pp-cli doctor
- **Node attributes silently rejected** — Use --guard flag to detect which attributes Framer dropped: framer-pp-cli nodes set <id> --guard
- **'framer-api not found' error** — Install the npm package globally: npm install -g framer-api
- **CMS sync creates duplicates** — Ensure your CSV has a unique 'slug' column matching existing item slugs for upsert behavior

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**framer-design-mcp-server**](https://github.com/superprat/framer-design-mcp-server) — TypeScript
- [**framer-mcp**](https://github.com/tmcpro/framer-mcp) — TypeScript
- [**framer-api**](https://www.npmjs.com/package/framer-api) — TypeScript
- [**framer-plugin-tools**](https://www.npmjs.com/package/framer-plugin-tools) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

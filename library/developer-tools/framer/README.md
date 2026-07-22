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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

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

Framer uses per-project API keys generated in Site Settings > General. Every live command needs two things in the environment: your `FRAMER_API_KEY` and the `FRAMER_PROJECT_URL` of the project you're targeting (copy it from the Framer editor's address bar). Save reusable flag sets with `framer-pp-cli profile save <name>` and apply them per command with `--profile <name>`; use the `dashboard` command for a cross-project status view. Live commands also require Node.js 18+, which the CLI drives as a runtime bridge (see [How it works](#how-it-works)).

## How it works

`framer-pp-cli` is a single Go binary, but Framer's Server API speaks over a WebSocket that only the official JavaScript SDK knows how to drive. To get the best of both, the binary orchestrates a small Node.js subprocess — the **bridge** (`bridge/framer-bridge.mjs`, built on the `framer-api` package) — and talks to it for every operation that touches your live project.

```
your terminal → framer-pp-cli (Go) → framer-bridge.mjs (Node) ⇄ Framer Server API (WebSocket)
```

This means:

- **Live/bridge commands** (`publish`, `changes`, `pages`, `nodes`, `assets`, `code`, `redirects`, `sync`, `doctor`, `components`, `collections`, `items`, `project`) require **Node.js 18+ at runtime** and both `FRAMER_API_KEY` and `FRAMER_PROJECT_URL`. This is true even when you install the binary via the Go (`go install`) path — the "Without Node" install means you don't need Node to *install* the CLI, not to *run live commands*.
- **Local-store commands** (`snapshot`, `diff`, `dashboard`, `search`, `cms-validate`, `cms-schema-diff`, `styles-import`, `code-pull`, `code-push`, `migrate-scrape`, `i18n-push`, `redirects-generate`, and `--dry-run` previews of `cms-sync`) work fully **offline** against a local SQLite store — no Node, no network.
- `framer-pp-cli doctor` verifies the whole chain: that Node.js is present, that the `framer-api` bridge can start, and that it can reach your project.

`FRAMER_BRIDGE_TIMEOUT` (a Go duration string like `5m`, default `120s`) caps how long any single bridge subprocess may run before the CLI gives up.

## Quick Start

```bash
# Required for every live command: your API key + the project you're targeting.
# Copy FRAMER_PROJECT_URL straight from the Framer editor's address bar.
export FRAMER_API_KEY=your_framer_api_key
export FRAMER_PROJECT_URL=https://framer.com/projects/Your-Project--abc123


# Verify Node.js, the framer-api bridge, and connectivity
framer-pp-cli doctor


# See what CMS collections exist
framer-pp-cli cms-collections list --json


# Preview a CMS import before committing (offline, dry-run)
framer-pp-cli cms-sync ./posts.csv --collection Blog --dry-run


# Create a preview deployment, then optionally promote to production
framer-pp-cli publish

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
- **`nodes set`** — Set node attributes through the live Framer bridge

  _When an agent needs to update canvas node attributes by ID_

  ```bash
  framer-pp-cli nodes set abc123 --attr width=400 --json
  ```
- **`publish`** — Create a preview deployment, then optionally promote it to production

  _When an agent needs a shareable preview URL or an explicit production deploy_

  ```bash
  framer-pp-cli publish --json
  ```
- **`code-push`** — Edit TSX components locally in your editor, then push to Framer — eliminates copy-paste workflow

  _When a developer needs to edit Framer components in their preferred editor without the copy-paste dance_

  ```bash
  framer-pp-cli code-pull HeroSection -o hero.tsx && vim hero.tsx && framer-pp-cli code-push hero.tsx --name HeroSection
  ```

### Migration automation
- **`migrate-scrape`** _(planned)_ — Crawl an existing website and generate a Framer migration manifest of pages, content, and assets

  _When porting an existing site to Framer, this is intended to automate the most tedious part of the migration. Site-migration scaffolding is a stub today and lands in a future version._

  ```bash
  framer-pp-cli migrate-scrape https://old-site.com --depth 3 --output manifest.json
  ```
- **`assets upload`** — Upload a single image, several images, or a whole directory and get back each Framer asset URL in one operation

  _When an agent needs to upload dozens of images and collect their Framer URLs without clicking through the editor_

  ```bash
  framer-pp-cli assets upload ./images/ --json
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

Live operations run through the Node bridge and need `FRAMER_API_KEY` + `FRAMER_PROJECT_URL`. Local-store and dry-run commands work offline.

### Live: project & deploy

- **`framer-pp-cli project get`** - Get project name, ID, and metadata
- **`framer-pp-cli project user`** - Get current authenticated user info
- **`framer-pp-cli publish`** - Create a preview deployment, then optionally promote it to production (`--yes` to auto-deploy)
- **`framer-pp-cli changes`** - List added, removed, and modified paths since the last publish
- **`framer-pp-cli pages`** - List all pages in the live project
- **`framer-pp-cli sync`** - Sync all project data from the Server API into the local store

### Live: canvas nodes

`nodes` is a command group; each operation is a subcommand.

- **`framer-pp-cli nodes get <id>`** - Get a node by ID with all attributes
- **`framer-pp-cli nodes children <id>`** - Get the child nodes of a node
- **`framer-pp-cli nodes set <id> --attr key=value`** - Set node attributes
- **`framer-pp-cli nodes create-frame`** - Create a new frame node
- **`framer-pp-cli nodes clone <id>`** - Clone a node
- **`framer-pp-cli nodes remove <id>`** - Remove a node

### Live: CMS

`collections` and `items` read live from the API; the `cms-*` groups manage structure.

- **`framer-pp-cli collections`** - List CMS collections directly from the live API
- **`framer-pp-cli items <collection-id>`** - List CMS items in a collection from the live API
- **`framer-pp-cli cms-collections list`** - List all CMS collections
- **`framer-pp-cli cms-collections get`** - Get collection details including fields and item count
- **`framer-pp-cli cms-collections create`** - Create a new CMS collection
- **`framer-pp-cli cms-fields`** - Add, remove, or reorder collection fields
- **`framer-pp-cli cms-items list`** - List all items in a collection
- **`framer-pp-cli cms-items get`** - Get a CMS item with all field data
- **`framer-pp-cli cms-items upsert`** - Create or update CMS items in batch
- **`framer-pp-cli cms-items remove`** - Remove CMS items by ID

### Live: code, components & assets

- **`framer-pp-cli code list`** - List all code files in the project
- **`framer-pp-cli code get`** - Get code file content
- **`framer-pp-cli components add --name <name>`** - Add a component instance by code-file name or `--id`
- **`framer-pp-cli assets upload <file-or-dir>`** - Upload an image (or a whole directory) and return each Framer asset URL
- **`framer-pp-cli fonts`** - List all available fonts with weights and styles

### Live: styles, custom code, i18n & redirects

- **`framer-pp-cli styles-colors list`** / **`styles-colors create`** - List or create color styles
- **`framer-pp-cli styles-text list`** / **`styles-text create`** - List or create text styles
- **`framer-pp-cli custom-code get`** / **`custom-code set`** - Read or install custom code at head/body insertion points
- **`framer-pp-cli i18n locales`** - List all project locales
- **`framer-pp-cli i18n groups`** - Get localization groups with translation status
- **`framer-pp-cli redirects list`** - List all project redirects
- **`framer-pp-cli redirects add --from <path> --to <path>`** - Add a redirect to the project

### Local store & migration (offline)

- **`framer-pp-cli snapshot`** - Save a labeled snapshot of all synced project data
- **`framer-pp-cli diff <a> <b>`** - Diff two snapshots to show what changed
- **`framer-pp-cli dashboard`** - Status across all synced data in the local store
- **`framer-pp-cli search`** - Full-text search across synced data
- **`framer-pp-cli cms-sync <file> --collection <id>`** - Import CMS content from CSV/JSON and diff against the local store
- **`framer-pp-cli cms-validate`** - Validate CMS referential integrity across collections
- **`framer-pp-cli cms-schema-diff <schema_file>`** - Diff a local YAML/JSON CMS schema against the synced collections
- **`framer-pp-cli code-pull <id-or-name> -o file.tsx`** - Pull a code file from the local store to disk
- **`framer-pp-cli code-push <file.tsx> --name <name>`** - Diff a local TSX/JS file against the stored version (dry-run preview)
- **`framer-pp-cli styles-import --from <file>`** - Preview CSS/Tailwind/JSON color tokens as Framer color styles
- **`framer-pp-cli i18n-push <file> --format csv`** - Push translations (CSV/PO/XLIFF) into Framer's localization system
- **`framer-pp-cli redirects-generate --old-sitemap <url>`** - Auto-generate a redirect map by fuzzy-matching old sitemap URLs to Framer slugs
- **`framer-pp-cli migrate-scrape <url>`** _(planned)_ - Crawl a site and generate a migration manifest (scaffolding is a stub today)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
framer-pp-cli changes

# JSON for scripting and agents
framer-pp-cli changes --json

# Filter to specific fields
framer-pp-cli changes --json --select id,name,status

# Dry run — show the request without sending
framer-pp-cli changes --dry-run

# Agent mode — JSON + compact + no prompts in one flag
framer-pp-cli changes --agent
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
| `FRAMER_API_KEY` | per_call | Yes | Your per-project API key from Site Settings > General. |
| `FRAMER_PROJECT_URL` | per_call | Yes (live commands) | The Framer project URL copied from the editor's address bar. Every bridge command (`publish`, `changes`, `pages`, `nodes`, `assets`, `code`, `sync`, `doctor`, `components`, `collections`, `items`, `redirects`, `project`) errors with `FRAMER_PROJECT_URL not set` without it. |
| `FRAMER_BRIDGE_TIMEOUT` | per_call | No | Go duration string (e.g. `5m`; default `120s`) capping how long a single Node bridge subprocess may run. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `framer-pp-cli doctor` to check credentials and the bridge
- Verify the environment variables are set: `echo $FRAMER_API_KEY` and `echo $FRAMER_PROJECT_URL`
**`FRAMER_PROJECT_URL not set`**
- Every live/bridge command needs `FRAMER_PROJECT_URL` in addition to `FRAMER_API_KEY`. Copy the URL from the Framer editor's address bar and `export FRAMER_PROJECT_URL=...` (see [Quick Start](#quick-start)).
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the corresponding `list` command to see available items

### API-specific

- **Connection timeout or 'WebSocket error'** — Verify `FRAMER_API_KEY` and `FRAMER_PROJECT_URL` are set, and that Node.js 18+ is installed: `framer-pp-cli doctor`
- **Node attributes silently rejected** — `nodes set` returns the bridge response, but it does not perform a follow-up read-back comparison. Run `framer-pp-cli nodes get <id>` after mutation when you need to confirm Framer persisted an attribute.
- **'framer-api not found' error** — `doctor` reports whether the Node bridge can start; ensure Node.js 18+ is on your `PATH` and re-run `framer-pp-cli doctor`
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

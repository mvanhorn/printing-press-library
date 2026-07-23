---
name: pp-framer
description: "Every Framer operation from your terminal — CMS sync, bulk upload, site migration, and a local database no other... Trigger phrases: `sync CMS content to Framer`, `import blog posts into Framer`, `publish my Framer site`, `migrate site to Framer`, `upload assets to Framer`, `check Framer project status`."
author: "ioncom"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - framer-pp-cli
    install:
      - kind: go
        bins: [framer-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/framer/cmd/framer-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/framer/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Framer — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `framer-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install framer --cli-only
   ```
2. Verify: `framer-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/framer/cmd/framer-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Framer's Server API is powerful but locked behind a JavaScript SDK with no CLI. framer-pp-cli wraps every API method with offline search, dry-run previews, multi-project management, and migration automation that turns site porting from days into hours.

## How It Works (Architecture)

`framer-pp-cli` is a Go binary that orchestrates a Node.js `framer-api` subprocess — the **bridge** (`bridge/framer-bridge.mjs`) — which speaks Framer's Server API over WebSocket. Any **live** command (anything that touches the project: `publish`, `changes`, `pages`, `nodes`, `assets`, `code`, `redirects`, `sync`, `doctor`, `components`, `collections`, `items`, `project`) shells out to this bridge, so **Node.js 18+ must be present at runtime**. The Go-only install path (`go install …`) still needs Node at runtime for these live commands — only the **local-store** commands (`snapshot`, `diff`, `dashboard`, `search`, `cms-sync`, `cms-validate`, `cms-schema-diff`, `styles-import`, `code-pull`, `code-push`, `migrate-scrape`, `i18n-push`, `redirects-generate`) work fully offline against the local SQLite store.

Run `framer-pp-cli doctor` to verify Node, the `framer-api` bridge, and live connectivity in one step.

## Environment Variables

Every **live / bridge** command reads its connection details from the environment. Set these before any command that talks to Framer, or the bridge errors out immediately.

| Variable | Required | Purpose |
|----------|----------|---------|
| `FRAMER_API_KEY` | **Yes** | Per-project API key from Framer **Site Settings → General**. Authenticates the bridge to the Server API. |
| `FRAMER_PROJECT_URL` | **Yes** (all live/bridge commands) | Your Framer project URL, copied from the Framer editor's address bar. Without it **every** bridge command (`publish`, `changes`, `pages`, `nodes`, `sync`, `doctor`, `redirects`, `collections`, `items`, `assets`, `code`, `components`, `project`) fails with `FRAMER_PROJECT_URL not set`. |
| `FRAMER_BRIDGE_TIMEOUT` | No | Go duration string (e.g. `5m`; default `120s`) capping how long a single bridge subprocess may run before it is killed. |

```bash
export FRAMER_API_KEY="$(your-secret-source)"
export FRAMER_PROJECT_URL="https://framer.com/projects/<your-project>"   # required for live commands
export FRAMER_BRIDGE_TIMEOUT="5m"                                        # optional

framer-pp-cli doctor   # confirms both vars + Node bridge + connectivity
```

Local-store commands (`snapshot`, `diff`, `dashboard`, `search`, etc.) do not require `FRAMER_PROJECT_URL` once data has been synced, but the initial `sync` that populates the store does.

## When to Use This CLI

Use framer-pp-cli when you need to automate Framer operations from scripts, CI/CD pipelines, or AI agents. Especially valuable for site migrations (importing content, uploading assets, generating redirects), multi-project management, and CMS content pipelines. Not for visual design work — use the Framer editor for that.

## Unique Capabilities

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
  framer-pp-cli nodes set abc123 --json --dry-run
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
- **`migrate-scrape`** — Planned stub for scraping a site into a Framer migration plan

  _Do not use for production migration planning yet; the command reports that this feature is planned_

  ```bash
  framer-pp-cli migrate-scrape https://old-site.com --depth 3 --dry-run --json
  ```
- **`assets upload`** — Upload a directory of images and auto-bind them to CMS items by filename-to-slug matching

  _When an agent needs to upload dozens of images and link them to the right CMS records in one operation_

  ```bash
  framer-pp-cli assets upload --dry-run --json
  ```
- **`i18n-push`** — Planned stub for syncing translations between standard i18n formats and Framer localization

  _Do not use for production translation sync yet; the command reports that this feature is planned_

  ```bash
  framer-pp-cli i18n-push translations.csv --format csv --dry-run
  ```
- **`redirects-generate`** — Planned stub for generating redirect maps from an old sitemap

  _Do not use for production redirect generation yet; the command reports that this feature is planned_

  ```bash
  framer-pp-cli redirects-generate --old-sitemap https://old-site.com/sitemap.xml --json
  ```
- **`styles-import`** — Import CSS variables or Tailwind config as Framer color and text styles — no manual recreation

  _When porting a site with an existing design system, this eliminates hours of manual style recreation_

  ```bash
  framer-pp-cli styles-import --from tailwind.config.js --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

The shipped binary exposes a **bridge-flattened** surface: the primary live operations are single leaf commands (no `list`/`preview`/`deploy` subcommands), while CMS, nodes, and a handful of others keep nested subcommands. Every command below exists in the binary — verify any of them with `framer-pp-cli <command> --help`.

### Live / bridge commands

These talk to the Framer Server API through the Node.js bridge. They require `FRAMER_API_KEY` and `FRAMER_PROJECT_URL`.

**publish** — Create a preview deployment, then optionally promote to production. Single command (no `preview`/`deploy` subcommands); pass `--yes` to auto-deploy.

- `framer-pp-cli publish` — Preview, print the URL + deployment ID, prompt to deploy
- `framer-pp-cli publish --yes` — Non-interactive: auto-deploy to production

**changes** — List added, removed, and modified paths since last publish (single command, no subcommands).

- `framer-pp-cli changes` — Show changed paths as a table or JSON

**pages** — List pages from the live project (single command, no subcommands).

- `framer-pp-cli pages` — List all pages

**nodes** — Canvas node operations.

- `framer-pp-cli nodes get <id>` — Get a node by ID with all attributes
- `framer-pp-cli nodes children <id>` — List children of a node
- `framer-pp-cli nodes set <id>` — Set attributes on a node
- `framer-pp-cli nodes move <id>` — Move a node to a new parent / reorder by index
- `framer-pp-cli nodes clone <id>` — Clone a node
- `framer-pp-cli nodes create-frame` — Create a new frame node on the canvas
- `framer-pp-cli nodes remove <id>` — Remove a node from the canvas

**assets** — Image asset upload.

- `framer-pp-cli assets upload` — Upload images to Framer and get back asset URLs

**code** — Code file management (live).

- `framer-pp-cli code list` — List all code files in the project
- `framer-pp-cli code get <id>` — Get a code file's content

**components** — Component operations.

- `framer-pp-cli components add` — Add a code component instance to the canvas by URL or name

**redirects** — URL redirect management (live).

- `framer-pp-cli redirects list` — List all project redirects
- `framer-pp-cli redirects add --from /old --to /new` — Add a redirect

**collections** — List CMS collections directly from the Framer API (live).

- `framer-pp-cli collections` — List collections with fields and item counts

**items** — List CMS items in a collection (live).

- `framer-pp-cli items` — List items in the specified collection

**cms-fields** — Add, remove, or reorder collection fields.

- `framer-pp-cli cms-fields --collection-id <id>` — Manage a collection's fields

**fonts** — Font management.

- `framer-pp-cli fonts` — List all available fonts with weights and styles

**project** — Framer project management.

- `framer-pp-cli project get` — Get project name, ID, and metadata
- `framer-pp-cli project user` — Get current authenticated user info

**sync** — Sync all Framer project data into the local store (single command).

- `framer-pp-cli sync` — Pull collections, items, pages, code, etc. into local SQLite

**doctor** — Verify the live API connection via the Node.js bridge.

- `framer-pp-cli doctor` — Check Node, the `framer-api` bridge, and connectivity

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
framer-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `framer-pp-cli dashboard` — Multi-project status dashboard showing stale CMS, unpublished changes, and collection health
- `framer-pp-cli snapshot` — Take a full structural snapshot of the current project into local SQLite
- `framer-pp-cli diff <snapshot_a> <snapshot_b>` — Diff two project snapshots to see structural changes over time
- `framer-pp-cli migrate-scrape <url>` — Scrape an existing website to generate a Framer migration manifest (currently a stub)
- `framer-pp-cli cms-sync <source_file>` — Sync CMS content from CSV or JSON with dry-run preview
- `framer-pp-cli cms-schema-diff <schema.yaml>` — Compare a local CMS schema definition against live Framer collections
- `framer-pp-cli cms-validate` — Find broken collection references, orphan items, and circular refs across CMS
- `framer-pp-cli styles-import <file>` — Import CSS variables or Tailwind config as Framer color and text styles (local preview/dry-run)
- `framer-pp-cli code-push <file>` — Push a local TSX/JS file or preview the change with `--dry-run`
- `framer-pp-cli code-pull <code_file_id>` — Pull a Framer code file from the local store to a local TSX file
- `framer-pp-cli redirects-generate` — Planned stub for redirect map generation
- `framer-pp-cli i18n-push <translations_file>` — Planned stub for Framer localization sync

## Recipes


### Bulk import blog posts from CSV

```bash
framer-pp-cli cms-sync ./posts.csv --collection Blog --dry-run && framer-pp-cli cms-sync ./posts.csv --collection Blog
```

Preview the import diff first, then commit the changes

### Port design tokens from Tailwind

```bash
framer-pp-cli styles-import --from tailwind.config.js --json --select name,value
```

Import your entire Tailwind color palette as Framer styles

### Edit a component locally

```bash
framer-pp-cli code-pull HeroSection --output hero.tsx
```

Pull a Framer code file to edit locally, then push back with code-push

### Preview planned migration redirects

```bash
framer-pp-cli redirects-generate --dry-run --json
```

Show the current planned-stub response for redirect map generation

### Create a preview deployment

```bash
framer-pp-cli publish --json
```

Create a preview deployment and get the shareable URL (add `--yes` to promote to production)

## Auth Setup

Framer uses per-project API keys generated in **Site Settings → General**. Provide the key and project URL via the environment (see [Environment Variables](#environment-variables)): set `FRAMER_API_KEY` and `FRAMER_PROJECT_URL` before any live command. Live commands require Node.js 18+ and the `framer-api` npm package as a runtime bridge.

To reuse the same flag set across invocations (e.g. a scheduled agent), save a **named profile** and apply it with `--profile <name>` — see [Named Profiles](#named-profiles). For status across multiple synced projects at once, use `framer-pp-cli dashboard`.

Run `framer-pp-cli doctor` to verify setup (Node + bridge + connectivity + env vars).

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  framer-pp-cli changes --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
framer-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
framer-pp-cli feedback --stdin < notes.txt
framer-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.framer-pp-cli/feedback.jsonl`. They are never POSTed unless `FRAMER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FRAMER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
framer-pp-cli profile save briefing --json
framer-pp-cli --profile briefing changes
framer-pp-cli profile list --json
framer-pp-cli profile show briefing
framer-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `framer-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/framer/cmd/framer-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add framer-pp-mcp -- framer-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which framer-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   framer-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `framer-pp-cli <command> --help`.

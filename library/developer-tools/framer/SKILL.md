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

# Framer — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `framer-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install framer --cli-only
   ```
2. Verify: `framer-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/framer/cmd/framer-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Framer's Server API is powerful but locked behind a JavaScript SDK with no CLI. framer-pp-cli wraps every API method with offline search, dry-run previews, multi-project management, and migration automation that turns site porting from days into hours.

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

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**assets** — Asset upload and management

- `framer-pp-cli assets add-image` — Add image from URL
- `framer-pp-cli assets upload` — Upload an image or file asset

**changes** — Change tracking between versions

- `framer-pp-cli changes contributors` — List contributors who made changes in a version range
- `framer-pp-cli changes list` — List added, removed, and modified paths since last publish

**cms-collections** — CMS collection management

- `framer-pp-cli cms-collections create` — Create a new CMS collection
- `framer-pp-cli cms-collections get` — Get collection details including fields and item count
- `framer-pp-cli cms-collections list` — List all CMS collections

**cms-fields** — CMS field management

- `framer-pp-cli cms-fields` — Add, remove, or reorder collection fields

**cms-items** — CMS item management

- `framer-pp-cli cms-items get` — Get a CMS item with all field data
- `framer-pp-cli cms-items list` — List all items in a collection
- `framer-pp-cli cms-items remove` — Remove CMS items by ID
- `framer-pp-cli cms-items upsert` — Create or update CMS items in batch

**code** — Code file management for React components and overrides

- `framer-pp-cli code create` — Create a new code file
- `framer-pp-cli code get` — Get code file content
- `framer-pp-cli code list` — List all code files in the project
- `framer-pp-cli code typecheck` — Run TypeScript type checking on a code file

**components** — Component management

- `framer-pp-cli components` — Add a component instance by URL or name

**custom-code** — Custom code injection into site head and body

- `framer-pp-cli custom-code get` — Get installed custom code snippets
- `framer-pp-cli custom-code set` — Install custom code at head or body insertion points

**fonts** — Font management

- `framer-pp-cli fonts` — List all available fonts with weights and styles

**i18n** — Localization management

- `framer-pp-cli i18n groups` — Get localization groups with translation status
- `framer-pp-cli i18n locales` — List all project locales

**nodes** — Canvas node operations

- `framer-pp-cli nodes add-svg` — Add an SVG element to the canvas
- `framer-pp-cli nodes children` — Get child nodes of a node
- `framer-pp-cli nodes clone` — Clone a node
- `framer-pp-cli nodes create-frame` — Create a new frame node
- `framer-pp-cli nodes create-text` — Create a new text node
- `framer-pp-cli nodes find` — Find nodes by type, attribute, or name
- `framer-pp-cli nodes get` — Get node by ID with all attributes
- `framer-pp-cli nodes remove` — Remove a node
- `framer-pp-cli nodes screenshot` — Capture a screenshot of a node as PNG/JPEG
- `framer-pp-cli nodes set` — Set node attributes
- `framer-pp-cli nodes set-text` — Set text content of a text node

**pages** — Page management

- `framer-pp-cli pages create` — Create a new page
- `framer-pp-cli pages list` — List all pages in the project

**project** — Framer project management

- `framer-pp-cli project info` — Get project name, ID, and metadata
- `framer-pp-cli project user` — Get current authenticated user info

**publish** — Publishing and deployment

- `framer-pp-cli publish deploy` — Promote a published deployment to production
- `framer-pp-cli publish preview` — Create a preview deployment with a shareable URL

**redirects** — URL redirect management

- `framer-pp-cli redirects add` — Add redirects to the project
- `framer-pp-cli redirects list` — List all project redirects
- `framer-pp-cli redirects remove` — Remove redirects from the project

**styles-colors** — Color style management

- `framer-pp-cli styles-colors create` — Create a new color style
- `framer-pp-cli styles-colors list` — List all color styles

**styles-text** — Text style management

- `framer-pp-cli styles-text create` — Create a new text style
- `framer-pp-cli styles-text list` — List all text styles


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
- `framer-pp-cli migrate-scrape <url>` — Scrape an existing website to generate a Framer migration manifest
- `framer-pp-cli migrate-apply <manifest>` — Apply a migration manifest to create pages, content, and assets in Framer
- `framer-pp-cli cms-sync <source_file>` — Sync CMS content from CSV, JSON, or Google Sheets with dry-run preview
- `framer-pp-cli cms-schema-diff <schema.yaml>` — Compare a local CMS schema definition against live Framer collections
- `framer-pp-cli cms-validate` — Find broken collection references, orphan items, and circular refs across CMS
- `framer-pp-cli api-probe` — Probe which node attributes Framer actually accepts vs silently rejects
- `framer-pp-cli styles-import <file>` — Import CSS variables or Tailwind config as Framer color and text styles
- `framer-pp-cli code-push <file>` — Push a local TSX file to a Framer code file
- `framer-pp-cli code-pull <code_file_id>` — Pull a Framer code file to a local TSX file for editing
- `framer-pp-cli redirects-generate` — Auto-generate redirect map from old site sitemap to Framer page slugs
- `framer-pp-cli i18n-push <translations_file>` — Push translations from CSV, PO, or XLIFF into Framer localization
- `framer-pp-cli i18n-pull` — Pull Framer translations into standard i18n format files
- `framer-pp-cli assets-bulk-upload <directory>` — Bulk upload assets from a directory and optionally map to CMS collection fields

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

### Generate migration redirects

```bash
framer-pp-cli redirects-generate --dry-run --json
```

Preview redirect map generation from old site sitemap

### Pre-publish health check

```bash
framer-pp-cli publish preview --json
```

Create a preview deployment and get the shareable URL

## Auth Setup

Framer uses per-project API keys generated in Site Settings. The CLI stores project profiles locally so you can switch between projects with --project aliases. Requires Node.js 18+ and the framer-api npm package as a runtime bridge.

Run `framer-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  framer-pp-cli changes list --agent --select id,name,status
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
framer-pp-cli --profile briefing changes list
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

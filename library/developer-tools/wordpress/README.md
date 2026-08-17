# WordPress CLI

**WP-CLI-grade control over any WordPress site through plain HTTPS — no SSH, no server-side plugin, one static binary.**

WP-CLI is the power tool but it runs on the server and needs SSH, which most shared hosting does not give you. The REST API reaches any site from anywhere with an Application Password, yet every existing client is abandoned, read-only, or hardcoded to a 2018 view of the API. This CLI covers the full modern core surface, adapts to the live route table of whatever site you point it at, and keeps a local SQLite mirror so cross-site and historical questions become one query instead of hundreds of paginated requests.

## Install

The recommended path installs both the `wordpress-pp-cli` binary and the `pp-wordpress` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wordpress
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wordpress --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wordpress --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wordpress --agent claude-code
npx -y @mvanhorn/printing-press-library install wordpress --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/cmd/wordpress-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wordpress-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install wordpress --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wordpress --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wordpress --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install wordpress --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wordpress-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WORDPRESS_USER` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/cmd/wordpress-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wordpress": {
      "command": "wordpress-pp-mcp",
      "env": {
        "WORDPRESS_USER": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

WordPress authenticates CLI clients with Application Passwords over HTTP Basic. Create one at Users -> Profile -> Application Passwords in wp-admin (the REST root also advertises the exact authorize-application.php URL for the site), then set WORDPRESS_USER and WORDPRESS_APP_PASSWORD. Public read endpoints work with no credentials at all, so posts, pages, and taxonomies are browsable immediately. Two things commonly break this and neither is a wrong password: Wordfence disables Application Passwords by default on its 5,000,000+ installs, and many Apache and CGI hosts strip the Authorization header entirely. Run 'diagnose' to tell those apart.

## Quick Start

```bash
# confirm the CLI is wired up before pointing it at a site
wordpress-pp-cli doctor --dry-run

# public reads need no credentials, so this works with zero configuration
wordpress-pp-cli posts list --per-page 5 --agent

# the first thing to run against any new site — says whether REST is reachable and, if not, exactly why
wordpress-pp-cli diagnose --agent

# what your credentials may actually do here, discovered without a single write
wordpress-pp-cli caps --agent

# build the local mirror; incremental via modified_after, not a full re-pull
wordpress-pp-cli sync --resources posts,pages,media --since 30d

# what is stuck in the editorial pipeline and for how long
wordpress-pp-cli queue --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Managing many sites at once
- **`fleet`** — See core version, pending updates, admin-account count and sync age for every site you manage, in one table.

  _Reach for this when the question spans more than one site. Every other command answers for a single site._

  ```bash
  wordpress-pp-cli fleet --agent
  ```

### Diagnosing a real site
- **`diagnose`** — Say in one command why a site's REST API is failing: path-based block, app-layer filter, bot challenge, renamed prefix, or stripped Authorization header.

  _Run this first whenever a WordPress request fails. A 401 can mean bad credentials, a security plugin, or a host stripping the auth header, and the status code alone cannot tell you which._

  ```bash
  wordpress-pp-cli diagnose --agent
  ```
- **`caps`** — Report what your credentials are actually allowed to do on each route, with zero writes.

  _Use this before pointing an unattended job at a site, instead of discovering permissions by attempting a write in production._

  ```bash
  wordpress-pp-cli caps --agent
  ```
- **`schema`** — Find out why a post type, field, or meta key is missing from the REST surface.

  _This is the show_in_rest blind spot. Reach for it when content exists in wp-admin but never appears in an API response._

  ```bash
  wordpress-pp-cli schema post --agent
  ```

### Editorial hygiene from the local mirror
- **`queue`** — See drafts, pending, and scheduled posts bucketed by how long they have been sitting in that state.

  _Answers 'what has been stuck in pending for three weeks', which wp-admin cannot express at all._

  ```bash
  wordpress-pp-cli queue --agent
  ```
- **`audit`** — Find posts and pages missing a featured image, a real category, an excerpt, or tags.

  _Use this for defects on content itself. It reports offending IDs, so the output feeds directly into a bulk fix._

  ```bash
  wordpress-pp-cli audit --agent
  ```
- **`orphans`** — List media files that no post or page references and that are nobody's featured image.

  _Reach for this when auditing storage or preparing a migration; it is the only way to size unused uploads without shell access._

  ```bash
  wordpress-pp-cli orphans --agent
  ```

## Recipes

### Triage a site that just started failing

```bash
wordpress-pp-cli diagnose --agent
```

Classifies the failure as a path-based block, an app-layer filter, a bot challenge, a renamed REST prefix, or a stripped Authorization header, instead of leaving you with a bare 401.

### Pull only the fields you need from a verbose endpoint

```bash
wordpress-pp-cli posts list --per-page 20 --agent --select id,title.rendered,status,modified
```

WordPress post objects carry 29 properties plus embedded links; narrowing with --select keeps large listings from flooding an agent's context.

### Check what a credential may do before automating with it

```bash
wordpress-pp-cli caps --agent
```

Walks the live route table reading the Allow header with and without authentication, so permissions are discovered without performing a single write.

### Find content defects across a synced site

```bash
wordpress-pp-cli audit --agent
```

Joins posts against media and term links locally to report missing featured images, categories, excerpts, and tags with the offending IDs.

### See what this specific site actually exposes

```bash
wordpress-pp-cli schema post --agent
```

Reconciles the post-types registry, the live route table, and the OPTIONS schema against real rows, surfacing fields registered without show_in_rest.

## Usage

Run `wordpress-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WORDPRESS_CONFIG_DIR`, `WORDPRESS_DATA_DIR`, `WORDPRESS_STATE_DIR`, or `WORDPRESS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WORDPRESS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WORDPRESS_HOME=/srv/wordpress
wordpress-pp-cli doctor
```

Under `WORDPRESS_HOME=/srv/wordpress`, the four dirs resolve to `/srv/wordpress/config`, `/srv/wordpress/data`, `/srv/wordpress/state`, and `/srv/wordpress/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "wordpress": {
      "command": "wordpress-pp-mcp",
      "env": {
        "WORDPRESS_HOME": "/srv/wordpress"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WORDPRESS_DATA_DIR` overrides an explicit `--home` for that kind. Use `WORDPRESS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WORDPRESS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `wordpress-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### abilities

WordPress Abilities API — the platform-native agent surface (WP 6.9+)

- **`wordpress-pp-cli abilities ability-categories`** - List ability categories
- **`wordpress-pp-cli abilities ability-category`** - Describe one ability category
- **`wordpress-pp-cli abilities get`** - Describe one ability, including its input schema
- **`wordpress-pp-cli abilities list`** - List abilities this site exposes to agents
- **`wordpress-pp-cli abilities run`** - Execute an ability with a JSON input payload

### application-passwords

Application passwords for a user

- **`wordpress-pp-cli application-passwords create`** - 
- **`wordpress-pp-cli application-passwords delete`** - 
- **`wordpress-pp-cli application-passwords get`** - 
- **`wordpress-pp-cli application-passwords introspect`** - 
- **`wordpress-pp-cli application-passwords list`** - 
- **`wordpress-pp-cli application-passwords update`** - 

### batch

Atomic multi-request batching (core /batch/v1)

- **`wordpress-pp-cli batch`** - Execute up to 25 write requests in one round trip

### block-directory

Search the block directory

- **`wordpress-pp-cli block-directory`** - 

### block-patterns

Block pattern categories

- **`wordpress-pp-cli block-patterns categories`** - 
- **`wordpress-pp-cli block-patterns list`** - 

### block-renderer

Render a block server-side

- **`wordpress-pp-cli block-renderer <name>`** - 

### block-revisions

Blocks revisions

- **`wordpress-pp-cli block-revisions delete`** - 
- **`wordpress-pp-cli block-revisions get`** - 
- **`wordpress-pp-cli block-revisions list`** - 

### block-types

Registered block types

- **`wordpress-pp-cli block-types get`** - 
- **`wordpress-pp-cli block-types list`** - 

### blocks

Reusable blocks (wp_block)

- **`wordpress-pp-cli blocks create`** - 
- **`wordpress-pp-cli blocks delete`** - 
- **`wordpress-pp-cli blocks get`** - 
- **`wordpress-pp-cli blocks list`** - 
- **`wordpress-pp-cli blocks update`** - 

### blocks-autosaves

Blocks autosaves

- **`wordpress-pp-cli blocks-autosaves <parent> <id>`** - 

### categories

Categories

- **`wordpress-pp-cli categories create`** - 
- **`wordpress-pp-cli categories delete`** - 
- **`wordpress-pp-cli categories get`** - 
- **`wordpress-pp-cli categories list`** - 
- **`wordpress-pp-cli categories update`** - 

### comments

Comments

- **`wordpress-pp-cli comments create`** - 
- **`wordpress-pp-cli comments delete`** - 
- **`wordpress-pp-cli comments get`** - 
- **`wordpress-pp-cli comments list`** - 
- **`wordpress-pp-cli comments update`** - 

### font-collections

Font collections

- **`wordpress-pp-cli font-collections get`** - 
- **`wordpress-pp-cli font-collections list`** - 

### font-faces

Font faces within a font family

- **`wordpress-pp-cli font-faces create`** - 
- **`wordpress-pp-cli font-faces delete`** - 
- **`wordpress-pp-cli font-faces get`** - 
- **`wordpress-pp-cli font-faces list`** - 

### font-families

Font families

- **`wordpress-pp-cli font-families create`** - 
- **`wordpress-pp-cli font-families delete`** - 
- **`wordpress-pp-cli font-families get`** - 
- **`wordpress-pp-cli font-families list`** - 
- **`wordpress-pp-cli font-families update`** - 

### global-styles

Global styles (theme.json)

- **`wordpress-pp-cli global-styles get`** - 
- **`wordpress-pp-cli global-styles theme`** - 
- **`wordpress-pp-cli global-styles update`** - 
- **`wordpress-pp-cli global-styles variations`** - 

### global-styles-revisions

Global-Styles revisions

- **`wordpress-pp-cli global-styles-revisions get`** - 
- **`wordpress-pp-cli global-styles-revisions list`** - 

### media

Media library items

- **`wordpress-pp-cli media create`** - 
- **`wordpress-pp-cli media delete`** - 
- **`wordpress-pp-cli media edit`** - 
- **`wordpress-pp-cli media get`** - 
- **`wordpress-pp-cli media list`** - 
- **`wordpress-pp-cli media post-process`** - 
- **`wordpress-pp-cli media update`** - 

### menu-items

Navigation menu items

- **`wordpress-pp-cli menu-items create`** - 
- **`wordpress-pp-cli menu-items delete`** - 
- **`wordpress-pp-cli menu-items get`** - 
- **`wordpress-pp-cli menu-items list`** - 
- **`wordpress-pp-cli menu-items update`** - 

### menu-items-autosaves

Menu-Items autosaves

- **`wordpress-pp-cli menu-items-autosaves <parent> <id>`** - 

### menu-locations

Registered menu locations

- **`wordpress-pp-cli menu-locations get`** - 
- **`wordpress-pp-cli menu-locations list`** - 

### menus

Navigation menus

- **`wordpress-pp-cli menus create`** - 
- **`wordpress-pp-cli menus delete`** - 
- **`wordpress-pp-cli menus get`** - 
- **`wordpress-pp-cli menus list`** - 
- **`wordpress-pp-cli menus update`** - 

### navigation

Navigation post type (wp_navigation)

- **`wordpress-pp-cli navigation create`** - 
- **`wordpress-pp-cli navigation delete`** - 
- **`wordpress-pp-cli navigation get`** - 
- **`wordpress-pp-cli navigation list`** - 
- **`wordpress-pp-cli navigation update`** - 

### navigation-autosaves

Navigation autosaves

- **`wordpress-pp-cli navigation-autosaves <parent> <id>`** - 

### navigation-revisions

Navigation revisions

- **`wordpress-pp-cli navigation-revisions delete`** - 
- **`wordpress-pp-cli navigation-revisions get`** - 
- **`wordpress-pp-cli navigation-revisions list`** - 

### oembed

oEmbed discovery and proxying (core /oembed/1.0)

- **`wordpress-pp-cli oembed get`** - oEmbed response for a URL on this site
- **`wordpress-pp-cli oembed proxy`** - Proxy an external oEmbed request through this site

### page-revisions

Pages revisions

- **`wordpress-pp-cli page-revisions delete`** - 
- **`wordpress-pp-cli page-revisions get`** - 
- **`wordpress-pp-cli page-revisions list`** - 

### pages

Pages

- **`wordpress-pp-cli pages create`** - 
- **`wordpress-pp-cli pages delete`** - 
- **`wordpress-pp-cli pages get`** - 
- **`wordpress-pp-cli pages list`** - 
- **`wordpress-pp-cli pages update`** - 

### pages-autosaves

Pages autosaves

- **`wordpress-pp-cli pages-autosaves <parent> <id>`** - 

### pattern-categories

Block pattern categories (taxonomy)

- **`wordpress-pp-cli pattern-categories create`** - 
- **`wordpress-pp-cli pattern-categories delete`** - 
- **`wordpress-pp-cli pattern-categories get`** - 
- **`wordpress-pp-cli pattern-categories list`** - 
- **`wordpress-pp-cli pattern-categories update`** - 

### pattern-directory

Patterns from the pattern directory

- **`wordpress-pp-cli pattern-directory`** - 

### plugins

Installed plugins

- **`wordpress-pp-cli plugins create`** - 
- **`wordpress-pp-cli plugins delete`** - 
- **`wordpress-pp-cli plugins get`** - 
- **`wordpress-pp-cli plugins list`** - 
- **`wordpress-pp-cli plugins update`** - 

### post-revisions

Posts revisions

- **`wordpress-pp-cli post-revisions delete`** - 
- **`wordpress-pp-cli post-revisions get`** - 
- **`wordpress-pp-cli post-revisions list`** - 

### posts

Blog posts

- **`wordpress-pp-cli posts create`** - 
- **`wordpress-pp-cli posts delete`** - 
- **`wordpress-pp-cli posts get`** - 
- **`wordpress-pp-cli posts list`** - 
- **`wordpress-pp-cli posts update`** - 

### posts-autosaves

Posts autosaves

- **`wordpress-pp-cli posts-autosaves <parent> <id>`** - 

### settings

Site settings

- **`wordpress-pp-cli settings get`** - 
- **`wordpress-pp-cli settings update`** - 

### sidebars

Widget sidebars

- **`wordpress-pp-cli sidebars get`** - 
- **`wordpress-pp-cli sidebars list`** - 
- **`wordpress-pp-cli sidebars update`** - 

### site-health

WordPress Site Health checks

- **`wordpress-pp-cli site-health authorization-header`** - 
- **`wordpress-pp-cli site-health background-updates`** - 
- **`wordpress-pp-cli site-health directory-sizes`** - 
- **`wordpress-pp-cli site-health dotorg-communication`** - 
- **`wordpress-pp-cli site-health https-status`** - 
- **`wordpress-pp-cli site-health loopback-requests`** - 
- **`wordpress-pp-cli site-health page-cache`** - 

### statuses

Registered post statuses

- **`wordpress-pp-cli statuses get`** - 
- **`wordpress-pp-cli statuses list`** - 

### tags

Tags

- **`wordpress-pp-cli tags create`** - 
- **`wordpress-pp-cli tags delete`** - 
- **`wordpress-pp-cli tags get`** - 
- **`wordpress-pp-cli tags list`** - 
- **`wordpress-pp-cli tags update`** - 

### taxonomies

Registered taxonomies

- **`wordpress-pp-cli taxonomies get`** - 
- **`wordpress-pp-cli taxonomies list`** - 

### template-part-revisions

Template-Parts revisions

- **`wordpress-pp-cli template-part-revisions delete`** - 
- **`wordpress-pp-cli template-part-revisions get`** - 
- **`wordpress-pp-cli template-part-revisions list`** - 

### template-parts

Block template parts

- **`wordpress-pp-cli template-parts create`** - 
- **`wordpress-pp-cli template-parts delete`** - 
- **`wordpress-pp-cli template-parts get`** - 
- **`wordpress-pp-cli template-parts list`** - 
- **`wordpress-pp-cli template-parts lookup`** - 
- **`wordpress-pp-cli template-parts update`** - 

### template-parts-autosaves

Template-Parts autosaves

- **`wordpress-pp-cli template-parts-autosaves <parent> <id>`** - 

### template-revisions

Templates revisions

- **`wordpress-pp-cli template-revisions delete`** - 
- **`wordpress-pp-cli template-revisions get`** - 
- **`wordpress-pp-cli template-revisions list`** - 

### templates

Block templates

- **`wordpress-pp-cli templates create`** - 
- **`wordpress-pp-cli templates delete`** - 
- **`wordpress-pp-cli templates get`** - 
- **`wordpress-pp-cli templates list`** - 
- **`wordpress-pp-cli templates lookup`** - 
- **`wordpress-pp-cli templates update`** - 

### templates-autosaves

Templates autosaves

- **`wordpress-pp-cli templates-autosaves <parent> <id>`** - 

### themes

Installed themes

- **`wordpress-pp-cli themes get`** - 
- **`wordpress-pp-cli themes list`** - 

### users

Users

- **`wordpress-pp-cli users create`** - 
- **`wordpress-pp-cli users delete`** - 
- **`wordpress-pp-cli users get`** - 
- **`wordpress-pp-cli users list`** - 
- **`wordpress-pp-cli users update`** - 

### widget-types

Registered widget types

- **`wordpress-pp-cli widget-types encode`** - 
- **`wordpress-pp-cli widget-types get`** - 
- **`wordpress-pp-cli widget-types list`** - 
- **`wordpress-pp-cli widget-types render`** - 

### widgets

Widgets

- **`wordpress-pp-cli widgets create`** - 
- **`wordpress-pp-cli widgets delete`** - 
- **`wordpress-pp-cli widgets get`** - 
- **`wordpress-pp-cli widgets list`** - 
- **`wordpress-pp-cli widgets update`** - 

### wp_search

Search across post types

- **`wordpress-pp-cli wp-search`** - 

### wp_types

Registered post types

- **`wordpress-pp-cli wp-types get`** - 
- **`wordpress-pp-cli wp-types list`** - 


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`wordpress-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`wordpress-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`wordpress-pp-cli learnings list`** - Inspect taught rows
- **`wordpress-pp-cli learnings forget <query>`** - Undo a teach
- **`wordpress-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`wordpress-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`wordpress-pp-cli teach-pattern`** - Install a query/resource template up front
- **`wordpress-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WORDPRESS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `wordpress-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wordpress-pp-cli abilities list

# JSON for scripting and agents
wordpress-pp-cli abilities list --json

# Filter to specific fields
wordpress-pp-cli abilities list --json --select id,name,status

# Dry run — show the request without sending
wordpress-pp-cli abilities list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wordpress-pp-cli abilities list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `WORDPRESS_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `wordpress-pp-cli abilities`
- `wordpress-pp-cli abilities get`
- `wordpress-pp-cli abilities list`
- `wordpress-pp-cli abilities search`
- `wordpress-pp-cli abilities-categories`
- `wordpress-pp-cli abilities-categories get`
- `wordpress-pp-cli abilities-categories list`
- `wordpress-pp-cli abilities-categories search`
- `wordpress-pp-cli block-patterns`
- `wordpress-pp-cli block-patterns get`
- `wordpress-pp-cli block-patterns list`
- `wordpress-pp-cli block-patterns search`
- `wordpress-pp-cli block-patterns-categories`
- `wordpress-pp-cli block-patterns-categories get`
- `wordpress-pp-cli block-patterns-categories list`
- `wordpress-pp-cli block-patterns-categories search`
- `wordpress-pp-cli block-types`
- `wordpress-pp-cli block-types get`
- `wordpress-pp-cli block-types list`
- `wordpress-pp-cli block-types search`
- `wordpress-pp-cli blocks`
- `wordpress-pp-cli blocks get`
- `wordpress-pp-cli blocks list`
- `wordpress-pp-cli blocks search`
- `wordpress-pp-cli categories`
- `wordpress-pp-cli categories get`
- `wordpress-pp-cli categories list`
- `wordpress-pp-cli categories search`
- `wordpress-pp-cli comments`
- `wordpress-pp-cli comments get`
- `wordpress-pp-cli comments list`
- `wordpress-pp-cli comments search`
- `wordpress-pp-cli font-collections`
- `wordpress-pp-cli font-collections get`
- `wordpress-pp-cli font-collections list`
- `wordpress-pp-cli font-collections search`
- `wordpress-pp-cli font-families`
- `wordpress-pp-cli font-families get`
- `wordpress-pp-cli font-families list`
- `wordpress-pp-cli font-families search`
- `wordpress-pp-cli media`
- `wordpress-pp-cli media get`
- `wordpress-pp-cli media list`
- `wordpress-pp-cli media search`
- `wordpress-pp-cli menu-items`
- `wordpress-pp-cli menu-items get`
- `wordpress-pp-cli menu-items list`
- `wordpress-pp-cli menu-items search`
- `wordpress-pp-cli menu-locations`
- `wordpress-pp-cli menu-locations get`
- `wordpress-pp-cli menu-locations list`
- `wordpress-pp-cli menu-locations search`
- `wordpress-pp-cli menus`
- `wordpress-pp-cli menus get`
- `wordpress-pp-cli menus list`
- `wordpress-pp-cli menus search`
- `wordpress-pp-cli navigation`
- `wordpress-pp-cli navigation get`
- `wordpress-pp-cli navigation list`
- `wordpress-pp-cli navigation search`
- `wordpress-pp-cli pages`
- `wordpress-pp-cli pages get`
- `wordpress-pp-cli pages list`
- `wordpress-pp-cli pages search`
- `wordpress-pp-cli pattern-categories`
- `wordpress-pp-cli pattern-categories get`
- `wordpress-pp-cli pattern-categories list`
- `wordpress-pp-cli pattern-categories search`
- `wordpress-pp-cli pattern-directory`
- `wordpress-pp-cli pattern-directory get`
- `wordpress-pp-cli pattern-directory list`
- `wordpress-pp-cli pattern-directory search`
- `wordpress-pp-cli plugins`
- `wordpress-pp-cli plugins get`
- `wordpress-pp-cli plugins list`
- `wordpress-pp-cli plugins search`
- `wordpress-pp-cli posts`
- `wordpress-pp-cli posts get`
- `wordpress-pp-cli posts list`
- `wordpress-pp-cli posts search`
- `wordpress-pp-cli sidebars`
- `wordpress-pp-cli sidebars get`
- `wordpress-pp-cli sidebars list`
- `wordpress-pp-cli sidebars search`
- `wordpress-pp-cli statuses`
- `wordpress-pp-cli statuses get`
- `wordpress-pp-cli statuses list`
- `wordpress-pp-cli statuses search`
- `wordpress-pp-cli tags`
- `wordpress-pp-cli tags get`
- `wordpress-pp-cli tags list`
- `wordpress-pp-cli tags search`
- `wordpress-pp-cli taxonomies`
- `wordpress-pp-cli taxonomies get`
- `wordpress-pp-cli taxonomies list`
- `wordpress-pp-cli taxonomies search`
- `wordpress-pp-cli template-parts`
- `wordpress-pp-cli template-parts get`
- `wordpress-pp-cli template-parts list`
- `wordpress-pp-cli template-parts search`
- `wordpress-pp-cli templates`
- `wordpress-pp-cli templates get`
- `wordpress-pp-cli templates list`
- `wordpress-pp-cli templates search`
- `wordpress-pp-cli themes`
- `wordpress-pp-cli themes get`
- `wordpress-pp-cli themes list`
- `wordpress-pp-cli themes search`
- `wordpress-pp-cli users`
- `wordpress-pp-cli users get`
- `wordpress-pp-cli users list`
- `wordpress-pp-cli users search`
- `wordpress-pp-cli widget-types`
- `wordpress-pp-cli widget-types get`
- `wordpress-pp-cli widget-types list`
- `wordpress-pp-cli widget-types search`
- `wordpress-pp-cli widgets`
- `wordpress-pp-cli widgets get`
- `wordpress-pp-cli widgets list`
- `wordpress-pp-cli widgets search`
- `wordpress-pp-cli wp_types`
- `wordpress-pp-cli wp_types get`
- `wordpress-pp-cli wp_types list`
- `wordpress-pp-cli wp_types search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
wordpress-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `wordpress-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/wordpress-pp-cli/config.toml`; `--home`, `WORDPRESS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WORDPRESS_USER` | per_call | Yes | Set to your API credential. |
| `WORDPRESS_APP_PASSWORD` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `wordpress-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `wordpress-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WORDPRESS_USER`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 rest_forbidden even though the password is correct** — Run 'wordpress-pp-cli diagnose'. The usual causes are Wordfence disabling Application Passwords site-side or the host stripping the Authorization header; the site-health authorization-header test distinguishes them.
- **404 rest_no_route on every endpoint** — Permalinks are set to Plain, or an edge rule blocks the literal path. Both are handled by the ?rest_route= fallback, which 'diagnose' probes automatically.
- **403 with an HTML body instead of JSON** — A bot-mitigation layer (commonly Cloudflare) is challenging the request above WordPress. This cannot be fixed client-side; the site owner must allowlist the path.
- **400 rest_invalid_param on a list command** — Read data.details in the JSON error — it names the offending parameter and the accepted range. per_page is hard-capped at 100.
- **A custom post type or ACF field never appears in responses** — It was registered without show_in_rest. Run 'wordpress-pp-cli schema <type>' to see exactly which declared fields never carry data.
- **Editing a page turns it into one invalid block in the editor** — Never write back content.rendered. Read with --context edit to get content.raw, which preserves the block markers, and write that.
- **batch run returns 403 or 404** — Hosts and WAFs began blocking /wp-json/batch/v1 after CVE-2026-63030. Update WordPress to 6.9.5 or 7.0.2 and ask the host to allowlist the route, or let the command fall back to sequential requests.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wp-cli/restful**](https://github.com/wp-cli/restful) — PHP (155 stars)
- [**claudeus-wp-mcp**](https://github.com/deus-h/claudeus-wp-mcp) — TypeScript (155 stars)
- [**sogko/go-wordpress**](https://github.com/sogko/go-wordpress) — Go (141 stars)
- [**server-wp-mcp**](https://github.com/emzimmer/server-wp-mcp) — JavaScript (115 stars)
- [**mcp-wordpress**](https://github.com/docdyhr/mcp-wordpress) — TypeScript (102 stars)
- [**mcp-wp**](https://github.com/InstaWP/mcp-wp) — TypeScript (90 stars)
- [**robbiet480/go-wordpress**](https://github.com/robbiet480/go-wordpress) — Go (19 stars)
- [**claude-wp-cli**](https://github.com/thenguyenvn90/claude-wp-cli) — Python (14 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

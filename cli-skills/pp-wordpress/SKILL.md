---
name: pp-wordpress
description: "WP-CLI-grade control over any WordPress site through plain HTTPS — no SSH, no server-side plugin, one static binary. Trigger phrases: `check my WordPress site`, `why is my wp-json returning 401`, `list posts from my WordPress site`, `publish this to WordPress`, `which of my WordPress sites need updates`, `find posts without a featured image`, `use wordpress`, `run wordpress-pp-cli`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - wordpress-pp-cli
    install:
      - kind: go
        bins: [wordpress-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/cmd/wordpress-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/wordpress/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# WordPress — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `wordpress-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install wordpress --cli-only
   ```
2. Verify: `wordpress-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/cmd/wordpress-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

WP-CLI is the power tool but it runs on the server and needs SSH, which most shared hosting does not give you. The REST API reaches any site from anywhere with an Application Password, yet every existing client is abandoned, read-only, or hardcoded to a 2018 view of the API. This CLI covers the full modern core surface, adapts to the live route table of whatever site you point it at, and keeps a local SQLite mirror so cross-site and historical questions become one query instead of hundreds of paginated requests.

## When to Use This CLI

Use this CLI to read from or write to any WordPress site over HTTPS when you have a site URL and an Application Password. It is the right tool for content operations at scale (bulk publishing, editorial queue triage, taxonomy and media hygiene), for fleet maintenance across many client sites without SSH, and for diagnosing a WordPress REST API that is returning 401, 403, or rest_no_route. It is also the right tool when you need to know what a site's API actually exposes, since it reads the live route table rather than assuming a fixed endpoint list.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for database work — export, import, search-replace, or arbitrary SQL. WordPress exposes none of that over REST; use WP-CLI over SSH.
- Do not use it to install, update, or activate a theme. The core themes controller is read-only over REST; theme activation is not possible, and tools that advertise it are overclaiming.
- Do not use it to read or write arbitrary options. The settings endpoint exposes roughly twenty whitelisted core options under renamed keys; anything else needs register_setting with show_in_rest.
- Do not use it for WP-Cron, transients, object cache, rewrite rules, wp-config.php, roles and capabilities, or multisite network administration — none have a core REST surface.
- Do not use it to round-trip block-editor content through content.rendered; that strips the block markers and corrupts the page. Use --context edit and content.raw.
- Do not use it as a WordPress.com or Jetpack client. This targets the self-hosted /wp-json surface.

## Unique Capabilities

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

## Command Reference

**abilities** — WordPress Abilities API — the platform-native agent surface (WP 6.9+)

- `wordpress-pp-cli abilities ability-categories` — List ability categories
- `wordpress-pp-cli abilities ability-category` — Describe one ability category
- `wordpress-pp-cli abilities get` — Describe one ability, including its input schema
- `wordpress-pp-cli abilities list` — List abilities this site exposes to agents
- `wordpress-pp-cli abilities run` — Execute an ability with a JSON input payload

**application-passwords** — Application passwords for a user

- `wordpress-pp-cli application-passwords create` — 
- `wordpress-pp-cli application-passwords delete` — 
- `wordpress-pp-cli application-passwords get` — 
- `wordpress-pp-cli application-passwords introspect` — 
- `wordpress-pp-cli application-passwords list` — 
- `wordpress-pp-cli application-passwords update` — 

**batch** — Atomic multi-request batching (core /batch/v1)

- `wordpress-pp-cli batch` — Execute up to 25 write requests in one round trip

**block-directory** — Search the block directory

- `wordpress-pp-cli block-directory` — 

**block-patterns** — Block pattern categories

- `wordpress-pp-cli block-patterns categories` — 
- `wordpress-pp-cli block-patterns list` — 

**block-renderer** — Render a block server-side

- `wordpress-pp-cli block-renderer <name>` — 

**block-revisions** — Blocks revisions

- `wordpress-pp-cli block-revisions delete` — 
- `wordpress-pp-cli block-revisions get` — 
- `wordpress-pp-cli block-revisions list` — 

**block-types** — Registered block types

- `wordpress-pp-cli block-types get` — 
- `wordpress-pp-cli block-types list` — 

**blocks** — Reusable blocks (wp_block)

- `wordpress-pp-cli blocks create` — 
- `wordpress-pp-cli blocks delete` — 
- `wordpress-pp-cli blocks get` — 
- `wordpress-pp-cli blocks list` — 
- `wordpress-pp-cli blocks update` — 

**blocks-autosaves** — Blocks autosaves

- `wordpress-pp-cli blocks-autosaves <parent> <id>` — 

**categories** — Categories

- `wordpress-pp-cli categories create` — 
- `wordpress-pp-cli categories delete` — 
- `wordpress-pp-cli categories get` — 
- `wordpress-pp-cli categories list` — 
- `wordpress-pp-cli categories update` — 

**comments** — Comments

- `wordpress-pp-cli comments create` — 
- `wordpress-pp-cli comments delete` — 
- `wordpress-pp-cli comments get` — 
- `wordpress-pp-cli comments list` — 
- `wordpress-pp-cli comments update` — 

**font-collections** — Font collections

- `wordpress-pp-cli font-collections get` — 
- `wordpress-pp-cli font-collections list` — 

**font-faces** — Font faces within a font family

- `wordpress-pp-cli font-faces create` — 
- `wordpress-pp-cli font-faces delete` — 
- `wordpress-pp-cli font-faces get` — 
- `wordpress-pp-cli font-faces list` — 

**font-families** — Font families

- `wordpress-pp-cli font-families create` — 
- `wordpress-pp-cli font-families delete` — 
- `wordpress-pp-cli font-families get` — 
- `wordpress-pp-cli font-families list` — 
- `wordpress-pp-cli font-families update` — 

**global-styles** — Global styles (theme.json)

- `wordpress-pp-cli global-styles get` — 
- `wordpress-pp-cli global-styles theme` — 
- `wordpress-pp-cli global-styles update` — 
- `wordpress-pp-cli global-styles variations` — 

**global-styles-revisions** — Global-Styles revisions

- `wordpress-pp-cli global-styles-revisions get` — 
- `wordpress-pp-cli global-styles-revisions list` — 

**media** — Media library items

- `wordpress-pp-cli media create` — 
- `wordpress-pp-cli media delete` — 
- `wordpress-pp-cli media edit` — 
- `wordpress-pp-cli media get` — 
- `wordpress-pp-cli media list` — 
- `wordpress-pp-cli media post-process` — 
- `wordpress-pp-cli media update` — 

**menu-items** — Navigation menu items

- `wordpress-pp-cli menu-items create` — 
- `wordpress-pp-cli menu-items delete` — 
- `wordpress-pp-cli menu-items get` — 
- `wordpress-pp-cli menu-items list` — 
- `wordpress-pp-cli menu-items update` — 

**menu-items-autosaves** — Menu-Items autosaves

- `wordpress-pp-cli menu-items-autosaves <parent> <id>` — 

**menu-locations** — Registered menu locations

- `wordpress-pp-cli menu-locations get` — 
- `wordpress-pp-cli menu-locations list` — 

**menus** — Navigation menus

- `wordpress-pp-cli menus create` — 
- `wordpress-pp-cli menus delete` — 
- `wordpress-pp-cli menus get` — 
- `wordpress-pp-cli menus list` — 
- `wordpress-pp-cli menus update` — 

**navigation** — Navigation post type (wp_navigation)

- `wordpress-pp-cli navigation create` — 
- `wordpress-pp-cli navigation delete` — 
- `wordpress-pp-cli navigation get` — 
- `wordpress-pp-cli navigation list` — 
- `wordpress-pp-cli navigation update` — 

**navigation-autosaves** — Navigation autosaves

- `wordpress-pp-cli navigation-autosaves <parent> <id>` — 

**navigation-revisions** — Navigation revisions

- `wordpress-pp-cli navigation-revisions delete` — 
- `wordpress-pp-cli navigation-revisions get` — 
- `wordpress-pp-cli navigation-revisions list` — 

**oembed** — oEmbed discovery and proxying (core /oembed/1.0)

- `wordpress-pp-cli oembed get` — oEmbed response for a URL on this site
- `wordpress-pp-cli oembed proxy` — Proxy an external oEmbed request through this site

**page-revisions** — Pages revisions

- `wordpress-pp-cli page-revisions delete` — 
- `wordpress-pp-cli page-revisions get` — 
- `wordpress-pp-cli page-revisions list` — 

**pages** — Pages

- `wordpress-pp-cli pages create` — 
- `wordpress-pp-cli pages delete` — 
- `wordpress-pp-cli pages get` — 
- `wordpress-pp-cli pages list` — 
- `wordpress-pp-cli pages update` — 

**pages-autosaves** — Pages autosaves

- `wordpress-pp-cli pages-autosaves <parent> <id>` — 

**pattern-categories** — Block pattern categories (taxonomy)

- `wordpress-pp-cli pattern-categories create` — 
- `wordpress-pp-cli pattern-categories delete` — 
- `wordpress-pp-cli pattern-categories get` — 
- `wordpress-pp-cli pattern-categories list` — 
- `wordpress-pp-cli pattern-categories update` — 

**pattern-directory** — Patterns from the pattern directory

- `wordpress-pp-cli pattern-directory` — 

**plugins** — Installed plugins

- `wordpress-pp-cli plugins create` — 
- `wordpress-pp-cli plugins delete` — 
- `wordpress-pp-cli plugins get` — 
- `wordpress-pp-cli plugins list` — 
- `wordpress-pp-cli plugins update` — 

**post-revisions** — Posts revisions

- `wordpress-pp-cli post-revisions delete` — 
- `wordpress-pp-cli post-revisions get` — 
- `wordpress-pp-cli post-revisions list` — 

**posts** — Blog posts

- `wordpress-pp-cli posts create` — 
- `wordpress-pp-cli posts delete` — 
- `wordpress-pp-cli posts get` — 
- `wordpress-pp-cli posts list` — 
- `wordpress-pp-cli posts update` — 

**posts-autosaves** — Posts autosaves

- `wordpress-pp-cli posts-autosaves <parent> <id>` — 

**settings** — Site settings

- `wordpress-pp-cli settings get` — 
- `wordpress-pp-cli settings update` — 

**sidebars** — Widget sidebars

- `wordpress-pp-cli sidebars get` — 
- `wordpress-pp-cli sidebars list` — 
- `wordpress-pp-cli sidebars update` — 

**site-health** — WordPress Site Health checks

- `wordpress-pp-cli site-health authorization-header` — 
- `wordpress-pp-cli site-health background-updates` — 
- `wordpress-pp-cli site-health directory-sizes` — 
- `wordpress-pp-cli site-health dotorg-communication` — 
- `wordpress-pp-cli site-health https-status` — 
- `wordpress-pp-cli site-health loopback-requests` — 
- `wordpress-pp-cli site-health page-cache` — 

**statuses** — Registered post statuses

- `wordpress-pp-cli statuses get` — 
- `wordpress-pp-cli statuses list` — 

**tags** — Tags

- `wordpress-pp-cli tags create` — 
- `wordpress-pp-cli tags delete` — 
- `wordpress-pp-cli tags get` — 
- `wordpress-pp-cli tags list` — 
- `wordpress-pp-cli tags update` — 

**taxonomies** — Registered taxonomies

- `wordpress-pp-cli taxonomies get` — 
- `wordpress-pp-cli taxonomies list` — 

**template-part-revisions** — Template-Parts revisions

- `wordpress-pp-cli template-part-revisions delete` — 
- `wordpress-pp-cli template-part-revisions get` — 
- `wordpress-pp-cli template-part-revisions list` — 

**template-parts** — Block template parts

- `wordpress-pp-cli template-parts create` — 
- `wordpress-pp-cli template-parts delete` — 
- `wordpress-pp-cli template-parts get` — 
- `wordpress-pp-cli template-parts list` — 
- `wordpress-pp-cli template-parts lookup` — 
- `wordpress-pp-cli template-parts update` — 

**template-parts-autosaves** — Template-Parts autosaves

- `wordpress-pp-cli template-parts-autosaves <parent> <id>` — 

**template-revisions** — Templates revisions

- `wordpress-pp-cli template-revisions delete` — 
- `wordpress-pp-cli template-revisions get` — 
- `wordpress-pp-cli template-revisions list` — 

**templates** — Block templates

- `wordpress-pp-cli templates create` — 
- `wordpress-pp-cli templates delete` — 
- `wordpress-pp-cli templates get` — 
- `wordpress-pp-cli templates list` — 
- `wordpress-pp-cli templates lookup` — 
- `wordpress-pp-cli templates update` — 

**templates-autosaves** — Templates autosaves

- `wordpress-pp-cli templates-autosaves <parent> <id>` — 

**themes** — Installed themes

- `wordpress-pp-cli themes get` — 
- `wordpress-pp-cli themes list` — 

**users** — Users

- `wordpress-pp-cli users create` — 
- `wordpress-pp-cli users delete` — 
- `wordpress-pp-cli users get` — 
- `wordpress-pp-cli users list` — 
- `wordpress-pp-cli users update` — 

**widget-types** — Registered widget types

- `wordpress-pp-cli widget-types encode` — 
- `wordpress-pp-cli widget-types get` — 
- `wordpress-pp-cli widget-types list` — 
- `wordpress-pp-cli widget-types render` — 

**widgets** — Widgets

- `wordpress-pp-cli widgets create` — 
- `wordpress-pp-cli widgets delete` — 
- `wordpress-pp-cli widgets get` — 
- `wordpress-pp-cli widgets list` — 
- `wordpress-pp-cli widgets update` — 

**wp_search** — Search across post types

- `wordpress-pp-cli wp-search` — 

**wp_types** — Registered post types

- `wordpress-pp-cli wp-types get` — 
- `wordpress-pp-cli wp-types list` — 


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `WORDPRESS_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

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

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
wordpress-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

WordPress authenticates CLI clients with Application Passwords over HTTP Basic. Create one at Users -> Profile -> Application Passwords in wp-admin (the REST root also advertises the exact authorize-application.php URL for the site), then set WORDPRESS_USER and WORDPRESS_APP_PASSWORD. Public read endpoints work with no credentials at all, so posts, pages, and taxonomies are browsable immediately. Two things commonly break this and neither is a wrong password: Wordfence disables Application Passwords by default on its 5,000,000+ installs, and many Apache and CGI hosts strip the Authorization header entirely. Run 'diagnose' to tell those apart.

Run `wordpress-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  wordpress-pp-cli abilities list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `WORDPRESS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WORDPRESS_CONFIG_DIR`, `WORDPRESS_DATA_DIR`, `WORDPRESS_STATE_DIR`, `WORDPRESS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WORDPRESS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `wordpress-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WORDPRESS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WORDPRESS_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
wordpress-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "wordpress-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `wordpress-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `wordpress-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `wordpress-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
wordpress-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
wordpress-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
wordpress-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
wordpress-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`wordpress-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `WORDPRESS_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
wordpress-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
wordpress-pp-cli feedback --stdin < notes.txt
wordpress-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WORDPRESS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WORDPRESS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
wordpress-pp-cli profile save briefing --json
wordpress-pp-cli --profile briefing abilities list
wordpress-pp-cli profile list --json
wordpress-pp-cli profile show briefing
wordpress-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `wordpress-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/cmd/wordpress-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add wordpress-pp-mcp -- wordpress-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which wordpress-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   wordpress-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `wordpress-pp-cli <command> --help`.

## Works with the web stack (same machine)

This CLI owns the **content** layer of a WordPress site. Sibling pp CLIs own the other layers — route by layer, not by brand. Full map + choreographies: `~/docs/runbooks/pp-web-stack.md`.

- **Hosting** (which install serves the site, cache, backups, SSL, PHP): `wpengine-pp-cli`. After content mutations on a WP Engine-hosted site, purge so it stops serving stale pages: `wpengine-pp-cli installs purge-cache create <install_id> --type page`. Before bulk content migration: `wpengine-pp-cli guard <install>` (checkpoint backup, CI gate).
- **Commerce** (orders, products, refunds, stock): `woocommerce-pp-cli` — not this CLI.
- **SEO** (backlinks/keywords): `ahrefs-pp-cli`; (Google search): `google-search-console-pp-cli`.
- **Agent-readiness** of the site: `isitagentready-pp-cli`.

Before improvising a cross-layer flow, run `wordpress-pp-cli recall "<question>"` — cross-CLI choreographies are recorded in the learn loop.

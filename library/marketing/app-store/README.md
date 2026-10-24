# App Store CLI

**Apple App Store charts, listings, ratings and reviews for any storefront from one Go binary, with a local history that turns repeated captures into rank, listing-change and traction trends.**

Pull 200-rank free and paid charts (top-grossing: up to 100 rows from Apple's legacy feed), listings with privacy labels, version history, in-app purchase prices (the page's top 10) and in-app events, and both review feeds, for any app in any storefront, with no key and no account. Every capture lands in SQLite, so traction, impact, changes, keyword-share and breakouts answer week-over-week questions once you have captures from more than one day.

## Install

The recommended path installs both the `app-store-pp-cli` binary and the `pp-app-store` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install app-store
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install app-store --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install app-store --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install app-store --agent claude-code
npx -y @mvanhorn/printing-press-library install app-store --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/app-store/cmd/app-store-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/app-store-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine app-store-pp-cli`. On Unix, mark it executable: `chmod +x app-store-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install app-store --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-app-store --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-app-store --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install app-store --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/app-store-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/app-store/cmd/app-store-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "app-store": {
      "command": "app-store-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No API key or Apple account. By default commands read Apple's public iTunes Lookup, Search and RSS feeds and public App Store web pages. suggest calls Apple's undocumented search-hints endpoint, and --source web (search-store, keyword-rank) and --source api (reviews) reach robots-disallowed surfaces: use those only when asked. Data is per storefront: pass --country with a two-letter code; ratings, reviews, iap, prices and compare, and the local readers traction, impact, changes, breakouts and review-digest, also take --countries for a list. Other commands read one storefront per run.

Run `app-store-pp-cli doctor` to verify setup (one paced iTunes Lookup).

Pacing: apps.apple.com pages go about 20 s apart across runs, other surfaces about 1 request per second (Search about 20 per minute). The first 403/429 stops the run (exit 7) and holds that host for at least 5 minutes, across runs; never retry in a loop.

## Quick Start

```bash
# Check configuration, resolved paths and reachability (one paced iTunes Lookup).
app-store-pp-cli doctor

# Fetch one full listing (Spotify) and record a snapshot.
app-store-pp-cli app 324684580 --country us --agent

# Read a games chart and store it for movers and rank history.
app-store-pp-cli top --chart top-free --genre 6014 --country pk --limit 50

# Archive the newest reviews from two storefronts.
app-store-pp-cli reviews 6448311069 --countries us,gb --sort recent

# Read per-country demand proxies once some history exists.
app-store-pp-cli traction 324684580 --countries us,pk --period daily

```

## Unique Features

### Local history that compounds
- **`traction`** — See per-country demand signals for any app, rolled up daily, monthly, annually or all-time: new ratings, chart rank and review volume, clearly labelled as proxies.

  _Reach for this when asked for downloads, installs or demand by country over time; it returns honest proxies instead of invented download counts._

  ```bash
  app-store-pp-cli traction 324684580 --countries us,gb,pk --period monthly --agent
  ```
- **`impact`** — Compare rank, new ratings, review stars and keyword positions in equal windows before and after a release, listing change or date.

  _Use it to check whether a release or metadata change moved anything, with co-movement numbers and no causal claims._

  ```bash
  app-store-pp-cli impact 324684580 --at latest --window 7d --agent
  ```
- **`changes`** — One dated feed of every change across your tracked apps: new versions, title and subtitle edits, price and in-app purchase price changes, privacy labels, events and featuring.

  _Use it for a weekly competitor review instead of opening every listing._

  ```bash
  app-store-pp-cli changes --since 7d --apps 553834731,850417475 --agent
  ```

### Market and keyword intelligence
- **`keyword-share`** — See which apps hold the top search results across a set of tracked terms and how that share moved since the last capture.

  _Use it for keyword reports across a basket of terms rather than one term at a time._

  ```bash
  app-store-pp-cli keyword-share "music streaming" "podcast player" --country us --top 10 --app 324684580 --agent
  ```
- **`breakouts`** — Find apps climbing or entering the same chart in several countries at once.

  _Use it to separate a multi-market breakout from noise in one storefront._

  ```bash
  app-store-pp-cli breakouts --genre 6014 --chart top-free --since 7d --min-markets 3 --agent
  ```

## Recipes

### Top 200 free games in Pakistan

```bash
app-store-pp-cli top --chart top-free --genre 6014 --country pk --limit 200 --agent --select rank,title,developerName,ratingCount
```

One chart page plus two Lookup requests (150 + 50 ids) return 200 ranked games with exact rating counts.

### In-app purchase prices for a game

```bash
app-store-pp-cli app 553834731 --full --agent --select trackName,inAppPurchases.name,inAppPurchases.price
```

Narrows the full listing to the in-app purchase names and prices (the page's top 10) with dotted paths.

### Weekly competitor change feed

```bash
app-store-pp-cli changes --since 7d --apps 553834731,850417475 --agent
```

Lists every recorded version, price, IAP, privacy, event and featuring change for the tracked apps.

### Monthly demand proxies by country

```bash
app-store-pp-cli traction 324684580 --countries us,gb,pk --period monthly --agent
```

New ratings, best chart rank and review volume per storefront, labelled as proxies.

## Usage

Run `app-store-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state: the learn journal, `teach.log`, and `appstore-governor.json` (cross-run pacing slots and 403/429 holds; a different `--home` or `APP_STORE_HOME` has its own governor) |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `APP_STORE_CONFIG_DIR`, `APP_STORE_DATA_DIR`, `APP_STORE_STATE_DIR`, or `APP_STORE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `APP_STORE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export APP_STORE_HOME=/srv/app-store
app-store-pp-cli doctor
```

Under `APP_STORE_HOME=/srv/app-store`, the four dirs resolve to `/srv/app-store/config`, `/srv/app-store/data`, `/srv/app-store/state`, and `/srv/app-store/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:APP_STORE_HOME = "C:\srv\app-store"
app-store-pp-cli doctor
```

Under `$env:APP_STORE_HOME = "C:\srv\app-store"`, the four dirs resolve to `C:\srv\app-store\config`, `C:\srv\app-store\data`, `C:\srv\app-store\state`, and `C:\srv\app-store\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "app-store": {
      "command": "app-store-pp-mcp",
      "env": {
        "APP_STORE_HOME": "/srv/app-store"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `APP_STORE_DATA_DIR` overrides an explicit `--home` for that kind. Use `APP_STORE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `APP_STORE_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `app-store-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

Live commands call Apple and store what they read in the local SQLite store. History commands read only that store: they return empty results until the captures that feed them exist, and movement needs captures from two or more days. Run any command with `--help` for its flags and examples.

### Live App Store reads

- **`app-store-pp-cli app <app>`** - One app's listing in one storefront: Lookup fields plus the product page's subtitle, star histogram, chart badge and Featured In; `--full` adds description, versions, privacy, in-app purchase prices, events and similar apps. Two requests.
- **`app-store-pp-cli top`** - One chart: top-free and top-paid have 200 ranks from the chart page; top-grossing has at most 100 rows from Apple's legacy RSS feed (Mac grossing feeds are empty). Rows carry exact rating counts; each run is stored for movers, rank-history and breakouts.
- **`app-store-pp-cli reviews <app>`** - Customer reviews from Apple's reviews RSS (50 per page, up to 10 pages per sort; `--sort both` by default). `--source api` is the robots-disallowed opt-in.
- **`app-store-pp-cli ratings <app>`** - Exact rating count and average per storefront (`--countries`); `--histogram` adds the star split from each product page.
- **`app-store-pp-cli compare <app> <app> [app...]`** - Apps side by side per storefront: developer, genre, price, ratings, version, size and release dates.
- **`app-store-pp-cli prices <app> [app...]`** - Price and availability per storefront, one Lookup per storefront.
- **`app-store-pp-cli iap <app>`** - In-app purchase prices from the product page (Apple lists the top 10) per storefront.
- **`app-store-pp-cli privacy <app>`** - App Privacy labels from the product page.
- **`app-store-pp-cli versions <app>`** - Version history with release notes, newest first (the page shows the 25 newest).
- **`app-store-pp-cli events <app>`** - In-app events on the product page.
- **`app-store-pp-cli similar <app>`** - The product page's You Might Also Like apps.
- **`app-store-pp-cli developer <developer-id-or-app>`** - A developer's apps (up to 200 from Lookup) and the shelves on the developer page.
- **`app-store-pp-cli featured`** - The Today tab: stories and the apps they feature.
- **`app-store-pp-cli search-store <term>`** - A live store search in one storefront. `--source web` is the robots-disallowed opt-in.
- **`app-store-pp-cli suggest <term>`** - Search suggestions from Apple's undocumented search-hints endpoint; run it only when asked.
- **`app-store-pp-cli keyword-rank <term>`** - One live search, the target app's rank (`--app`) and the stored result list for keyword-history and keyword-share. `--source web` is the opt-in.

### Local history readers

- **`app-store-pp-cli movers`** - Apps that climbed, dropped, entered or left one chart between its two latest captures (fed by `top`).
- **`app-store-pp-cli rank-history <app>`** - An app's rank over time in one chart (fed by `top`).
- **`app-store-pp-cli keyword-history <term>`** - A term's stored captures: rank over time and who held the top results (fed by `keyword-rank`).
- **`app-store-pp-cli watch-listing <app>`** - Field-by-field diff of an app's two latest stored listing snapshots (fed by `app`, `prices`, `compare`).
- **`app-store-pp-cli review-digest <app>`** - A mechanical digest of stored reviews: star histogram, volume over time, common words (fed by `reviews`).
- **`app-store-pp-cli search <query>`** - Offline search of stored reviews and app listings.

### categories

List App Store chart categories and their genre ids. By default from the built-in genre table (no request); `--live` reads the public chart page (one paced request).

- **`app-store-pp-cli categories games`** - Game subcategories (genre ids 7001-7019 such as 7012 Puzzle) with chart links
- **`app-store-pp-cli categories list`** - App categories (genre ids such as 6014 Games, 6026 Developer Tools, Kids age bands) with chart links


### breakouts


- **`app-store-pp-cli breakouts`** - Find apps climbing or entering the same chart in several countries at once.

Fed by `top` captures of one chart in several storefronts.


### changes


- **`app-store-pp-cli changes`** - One dated feed of every change across your tracked apps: new versions, title and subtitle edits, price and in-app purchase price changes, privacy labels, events and featuring.

Fed by `app`, `iap`, `privacy`, `events` and `featured` captures (and the lookup snapshots `prices` and `compare` store).


### impact


- **`app-store-pp-cli impact`** - Compare rank, new ratings, review stars and keyword positions in equal windows before and after a release, listing change or date.

Fed by `app`, `ratings`, `top`, `reviews` and `keyword-rank` captures on both sides of the date.


### keyword-share


- **`app-store-pp-cli keyword-share`** - See which apps hold the top search results across a set of tracked terms and how that share moved since the last capture.

Fed by `keyword-rank` captures of each term.


### traction


- **`app-store-pp-cli traction`** - See per-country demand signals for any app, rolled up daily, monthly, annually or all-time: new ratings, chart rank and review volume, clearly labelled as proxies.

Fed by `ratings`, `top` and `reviews` captures; Apple publishes no download numbers, so these are proxies, never downloads.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`app-store-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`app-store-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`app-store-pp-cli learnings list`** - Inspect taught rows
- **`app-store-pp-cli learnings forget <query>`** - Undo a teach
- **`app-store-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`app-store-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`app-store-pp-cli teach-pattern`** - Install a query/resource template up front
- **`app-store-pp-cli teach-lookup`** - Add an entity mapping (e.g. a storefront code or a genre alias) for pattern substitution

Pass `--no-learn` or set `APP_STORE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `app-store-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
app-store-pp-cli categories list

# JSON for scripting and agents
app-store-pp-cli categories list --json
# Filter to specific fields by name
app-store-pp-cli categories list --json --select genreId,name

# Dry run: exit before any request or store write (prints the command it would run)
app-store-pp-cli categories list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
app-store-pp-cli categories list --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select genreId,name` returns only the fields you need; dotted paths reach nested lists (`inAppPurchases.price`)
- **Previewable** - `--dry-run` exits before any request or store write and prints the command it would run (it does not show URLs or validate arguments)
- **Read-only toward Apple** - this CLI never changes anything on the App Store; it writes only its local store, cache and learn files
- **Offline-friendly** - the history commands (traction, impact, changes, keyword-share, breakouts, movers, rank-history, keyword-history, watch-listing, review-digest, search) read only the local SQLite store; live commands always call Apple
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` invalid invocation of a framework flag (an unknown `--data-source`, `--profile` or `--deliver` value, or `doctor --fail-on` tripped), `2` usage error, `3` not found (an app or developer id that is not sold in, or not found in, the storefront), `5` API error, `7` rate limited or paced (Apple answered 403/429, or the next paced request slot lies past `--timeout`; nothing more was sent), `10` config or local store error.

## Health Check

```bash
app-store-pp-cli doctor
```

Shows the resolved config, data, state and cache paths and checks reachability with one paced iTunes Lookup.

## Configuration

Run `app-store-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/app-store-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\app-store-pp-cli\config.toml` on Windows; `--home`, `APP_STORE_HOME`, and per-kind env vars can relocate it.

Static request headers (`headers`), `--ca-cert` / `APP_STORE_CA_CERT` / `ca_cert` and `--tls-server-name` / `APP_STORE_TLS_SERVER_NAME` / `tls_server_name` apply to the generated client only: `doctor`'s reachability probe, `export` and the MCP server's typed `categories` tools. The App Store data commands use their own client with the system trust store, the standard `HTTPS_PROXY` / `HTTP_PROXY` variables and a fixed Chrome User-Agent. Precedence for the generated client's settings is flag, then environment, then the config file; a config file other users can change does not supply them.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the app id (numeric, id123, an App Store URL or a bundle id) and `--country`: an app not sold in that storefront exits 3
- Find ids with `app-store-pp-cli search-store <term>`

### API-specific
- **Exit code 7 or 'rate limited' after a burst of requests** — Wait a few minutes, then retry with a narrower --countries list or a lower --rate-limit; the client stops at the first 403 or 429 on purpose and holds that host for at least 5 minutes, across runs. A 'may go only in Ns' message is the CLI's own pacing (apps.apple.com pages go about 20 s apart): nothing was sent, so rerun after that wait or raise --timeout.
- **reviews returns no rows for an app that has reviews** — Apple's review feed arrives in lagged batches and is sometimes empty; retry later, and keep the default --sort both (or drop an explicit --sort recent) so the helpful feed is included.
- **impact reports insufficient_data, or traction returns empty results or null newRatings** — These read only your local captures. Run the capture commands on more than one day: ratings, top and reviews for traction; app, ratings, top, reviews and keyword-rank for impact; app, iap, privacy, events and featured for changes; keyword-rank for keyword-share; top in several storefronts for breakouts. Schedule them daily with cron or launchd.
- **You need real download or sales numbers** — Apple publishes none for public apps; for apps you own, use the asc CLI (App Store Connect).

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**DIYgod/RSSHub**](https://github.com/DIYgod/RSSHub) — TypeScript (46615 stars)
- [**facundoolano/app-store-scraper**](https://github.com/facundoolano/app-store-scraper) — JavaScript (1436 stars)
- [**respectlytics/respectaso**](https://github.com/respectlytics/respectaso) — Python (529 stars)
- [**appreply-co/mcp-appstore**](https://github.com/appreply-co/mcp-appstore) — JavaScript (78 stars)
- [**plahteenlahti/app-store-scraper**](https://github.com/plahteenlahti/app-store-scraper) — TypeScript (19 stars)
- [**hristo2612/aso-cli**](https://github.com/hristo2612/aso-cli) — TypeScript (19 stars)
- [**drewster99/appstore-mcp-server**](https://github.com/drewster99/appstore-mcp-server) — Swift (7 stars)
- [**google-play (Printing Press)**](https://github.com/mvanhorn/printing-press-library/tree/main/library/marketing/google-play) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

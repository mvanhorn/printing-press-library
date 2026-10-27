# AppTweak CLI

**Every AppTweak endpoint in a local-first CLI — keyword rank fan-outs, competitor metadata diffs, and review mining that no MCP server can do.**

The only CLI for AppTweak. Track keyword rankings across countries, diff competitor metadata changes, mine reviews by sentiment, and monitor paid vs organic keyword gaps — all stored in SQLite for offline analysis and CSV export. The AppTweak MCP servers cover individual queries; this CLI covers the workflows that require persistence, loops, and aggregation.

Created by [@rk-labs-tech](https://github.com/rk-labs-tech) (Ryan Kelley).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `apptweak-pp-cli` binary and the `pp-apptweak` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install apptweak
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install apptweak --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install apptweak --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install apptweak --agent claude-code
npx -y @mvanhorn/printing-press-library install apptweak --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apptweak/cmd/apptweak-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apptweak-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install apptweak --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-apptweak --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-apptweak --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install apptweak --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apptweak-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `APPTWEAK_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apptweak/cmd/apptweak-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "apptweak": {
      "command": "apptweak-pp-mcp",
      "env": {
        "APPTWEAK_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

### Remote MCP over HTTP

`apptweak-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `APPTWEAK_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `APPTWEAK_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export APPTWEAK_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
apptweak-pp-mcp --transport http
```

Set `APPTWEAK_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

Set APPTWEAK_API_KEY to your AppTweak API token. The key is sent as the x-apptweak-key header on every request. Get your key at https://developers.apptweak.com.

## Quick Start

```bash
# Verify auth and API reachability
apptweak-pp-cli doctor --dry-run


# Check keyword volume and difficulty
apptweak-pp-cli keywords get-metrics-current --keywords 'photo editor,camera' --country us --device iphone --json


# Pull app metadata
apptweak-pp-cli apps get-metadata --apps 284882215 --country us --device iphone --json


# Fan out keyword rank check across 4 countries
apptweak-pp-cli keywords rank-fanout --app 284882215 --keywords 'photo editor' --countries us,gb,de,fr --device iphone --json


# Find ASA opportunities competitor exploits
apptweak-pp-cli keywords paid-organic-gap --competitor 389801252 --app 284882215 --country us --device iphone --agent

```

## Unique Features

### Multi-country intelligence

- **`keywords rank-fanout`** — Check keyword rankings across many countries in one command — no MCP tool or competitor offers multi-country fan-out.

  _Use this when an ASO team needs a country-sweep ranking report without running the same query manually for each market._

  ```bash
  apptweak-pp-cli keywords rank-fanout --app 284882215 --keywords 'photo editor,camera' --countries us,gb,de,fr,jp --device iphone --json
  ```

### Competitor intelligence

- **`apps metadata-diff`** — Compare metadata between two app versions or two competitor apps to spot ASO experiments.

  _Use this to detect when a competitor changed their title, subtitle, or description — the primary signal of an ASO experiment._

  ```bash
  apptweak-pp-cli apps metadata-diff --app 284882215 --vs 389801252 --country us --device iphone --fields title,subtitle,description
  ```
- **`keywords paid-organic-gap`** — Find keywords a competitor bids on where your app doesn't rank organically — ASA opportunity map.

  _Use this to find Apple Search Ads opportunities where competitors are spending but your organic rank is absent._

  ```bash
  apptweak-pp-cli keywords paid-organic-gap --competitor 389801252 --app 284882215 --country us --device iphone --agent
  ```

### Local state that compounds

- **`keywords track`** — Sync and alert on ranking changes for a saved keyword basket across apps and countries.

  _Use this to monitor ranking changes for your core keyword set without re-querying or manually comparing runs._

  ```bash
  apptweak-pp-cli keywords track --app 284882215 --keywords 'photo,camera,selfie' --country us --device iphone --since 7d
  ```
- **`reviews sentiment`** — Aggregate review ratings by keyword term — find what topics drive 1-star vs 5-star reviews.

  _Use this to surface the specific complaint terms driving low ratings — faster than reading individual reviews._

  ```bash
  apptweak-pp-cli reviews sentiment --app 284882215 --country us --device iphone --word 'crash' --json
  ```

### Agent-native plumbing

- **`credits status`** — Check remaining API credits and burn rate before running expensive bulk queries.

  _Use this before bulk syncs to avoid hitting credit limits mid-run._

  ```bash
  apptweak-pp-cli credits status --json
  ```

## Recipes


### Multi-country keyword rank sweep

```bash
apptweak-pp-cli keywords rank-fanout --app 284882215 --keywords 'photo editor,camera filter' --countries us,gb,de,fr,jp,kr --device iphone --json --select results.country,results.keyword,results.rank
```

Fan out keyword rank checks across 6 countries and extract just country/keyword/rank fields.

### Competitor metadata change detection

```bash
apptweak-pp-cli apps metadata-diff --app 284882215 --vs 389801252 --country us --device iphone --fields title,subtitle
```

Compare title and subtitle between two apps to spot competitor ASO experiments.

### Review sentiment for a complaint term

```bash
apptweak-pp-cli reviews sentiment --app 284882215 --country us --device iphone --word 'crash' --json --agent
```

Aggregate rating distribution for reviews mentioning 'crash' — identify if a term drives negative sentiment.

### Paid vs organic gap report

```bash
apptweak-pp-cli keywords paid-organic-gap --competitor 389801252 --app 284882215 --country us --device iphone --agent --select gap_keywords
```

Find keywords where the competitor bids but your app has no organic ranking — prime Apple Search Ads targets.

## Usage

Run `apptweak-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `APPTWEAK_CONFIG_DIR`, `APPTWEAK_DATA_DIR`, `APPTWEAK_STATE_DIR`, or `APPTWEAK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `APPTWEAK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export APPTWEAK_HOME=/srv/apptweak
apptweak-pp-cli doctor
```

Under `APPTWEAK_HOME=/srv/apptweak`, the four dirs resolve to `/srv/apptweak/config`, `/srv/apptweak/data`, `/srv/apptweak/state`, and `/srv/apptweak/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:APPTWEAK_HOME = "C:\srv\apptweak"
apptweak-pp-cli doctor
```

Under `$env:APPTWEAK_HOME = "C:\srv\apptweak"`, the four dirs resolve to `C:\srv\apptweak\config`, `C:\srv\apptweak\data`, `C:\srv\apptweak\state`, and `C:\srv\apptweak\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "apptweak": {
      "command": "apptweak-pp-mcp",
      "env": {
        "APPTWEAK_HOME": "/srv/apptweak"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `APPTWEAK_DATA_DIR` overrides an explicit `--home` for that kind. Use `APPTWEAK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `APPTWEAK_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `apptweak-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `apptweak-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `apptweak-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `APPTWEAK_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `APPTWEAK_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### apps

Manage apps

- **`apptweak-pp-cli apps get-category-ranking-current`** - Current free, paid, or grossing chart position for one or more apps.
- **`apptweak-pp-cli apps get-category-ranking-history`** - Track category chart position over time.
- **`apptweak-pp-cli apps get-keyword-rankings-current`** - Rank position for a list of keywords across a country and device.
- **`apptweak-pp-cli apps get-keyword-rankings-history`** - Track keyword rank positions over time — core ASO monitoring endpoint.
- **`apptweak-pp-cli apps get-metadata`** - Title, subtitle, description, screenshots, ratings, developer info, languages, in-app purchases, and more.
- **`apptweak-pp-cli apps get-metadata-history`** - Track title, subtitle, description changes over a date range.
- **`apptweak-pp-cli apps get-metrics-current`** - Estimated downloads, revenues, and App Power score.
- **`apptweak-pp-cli apps get-metrics-history`** - Downloads, revenues, and App Power score over a date range.
- **`apptweak-pp-cli apps get-paid-keywords`** - Keywords an app bids on in Apple Search Ads — competitor paid strategy intelligence.
- **`apptweak-pp-cli apps get-review-stats`** - Rating distribution, average rating, review count over a date range.
- **`apptweak-pp-cli apps get-reviews-displayed`** - Reviews currently displayed in the App Store or Google Play for an app.
- **`apptweak-pp-cli apps get-twin`** - Find the Android equivalent of an iOS app or vice versa.
- **`apptweak-pp-cli apps search-reviews`** - Search through review text, filter by rating or date range. Requires tracked app.

### apptweak

Manage apptweak

- **`apptweak-pp-cli apptweak get-dna-list`** - AppTweak's proprietary DNA sub-genre taxonomy identifiers.
- **`apptweak-pp-cli apptweak get-supported-countries`** - List all countries supported for a given store.
- **`apptweak-pp-cli apptweak get-supported-languages`** - List all supported language codes.

### categories

Manage categories

- **`apptweak-pp-cli categories get-category-metrics`** - Aggregate downloads, revenue, and top performers for an App Store category.
- **`apptweak-pp-cli categories get-conversion-rate-benchmarks`** - Industry average conversion rates by category — benchmark your app's store listing performance.

### charts

Manage charts

- **`apptweak-pp-cli charts get-dna-current`** - AppTweak's proprietary DNA sub-genre taxonomy charts — more granular than official categories.
- **`apptweak-pp-cli charts get-top-current`** - Current free, paid, or grossing chart for a category and country.
- **`apptweak-pp-cli charts get-top-history`** - How the free/paid/grossing chart changed over a date range.

### featured-content

Manage featured content

- **`apptweak-pp-cli featured-content`** - When and in which editorial lists an app was featured (iOS only).

### keywords

Manage keywords

- **`apptweak-pp-cli keywords get-live-search-ads-current`** - Apps running Apple Search Ads for a keyword right now.
- **`apptweak-pp-cli keywords get-live-search-current`** - Apps appearing in search results right now for a given keyword.
- **`apptweak-pp-cli keywords get-metrics-current`** - Search volume, difficulty score, and brand score for keywords.
- **`apptweak-pp-cli keywords get-metrics-history`** - Search volume and difficulty score trends over time.
- **`apptweak-pp-cli keywords get-search-history`** - How search results for a keyword changed over time.
- **`apptweak-pp-cli keywords get-share-of-voice`** - Ad impression share across all bidders for a keyword — identify dominant advertisers.
- **`apptweak-pp-cli keywords get-suggestions-by-app`** - Keyword ideas based on an app's install patterns — top installs, trending, or volume change.
- **`apptweak-pp-cli keywords get-suggestions-by-category`** - Highest-volume and trending keywords for an App Store category.


### apps


- **`apptweak-pp-cli apps metadata-diff`** - Compare metadata between two app versions or two competitor apps to spot ASO experiments.


### credits


- **`apptweak-pp-cli credits status`** - Check remaining API credits and burn rate before running expensive bulk queries.


### keywords


- **`apptweak-pp-cli keywords rank-fanout`** - Check keyword rankings across many countries in one command — no MCP tool or competitor offers multi-country fan-out.

- **`apptweak-pp-cli keywords track`** - Sync and alert on ranking changes for a saved keyword basket across apps and countries.

- **`apptweak-pp-cli keywords paid-organic-gap`** - Find keywords a competitor bids on where your app doesn't rank organically — ASA opportunity map.


### reviews


- **`apptweak-pp-cli reviews sentiment`** - Aggregate review ratings by keyword term — find what topics drive 1-star vs 5-star reviews.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`apptweak-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`apptweak-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`apptweak-pp-cli learnings list`** - Inspect taught rows
- **`apptweak-pp-cli learnings forget <query>`** - Undo a teach
- **`apptweak-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`apptweak-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`apptweak-pp-cli teach-pattern`** - Install a query/resource template up front
- **`apptweak-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `APPTWEAK_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `apptweak-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
apptweak-pp-cli apps get-category-ranking-current --apps 324684580 --country us --device iphone

# JSON for scripting and agents
apptweak-pp-cli apps get-category-ranking-current --apps 324684580 --country us --device iphone --json
# Filter to specific fields by name
apptweak-pp-cli apps get-category-ranking-current --apps 324684580 --country us --device iphone --json --select <field>[,<field>...]

# Dry run — show the request without sending
apptweak-pp-cli apps get-category-ranking-current --apps 324684580 --country us --device iphone --dry-run

# Agent mode — JSON + compact + no prompts in one flag
apptweak-pp-cli apps get-category-ranking-current --apps 324684580 --country us --device iphone --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
apptweak-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `apptweak-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/apptweak-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\apptweak-pp-cli\config.toml` on Windows; `--home`, `APPTWEAK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `APPTWEAK_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `APPTWEAK_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `APPTWEAK_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `apptweak-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `apptweak-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $APPTWEAK_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized** — export APPTWEAK_API_KEY=<your-token> — check at developers.apptweak.com
- **403 Forbidden** — Insufficient credits — check apptweak-pp-cli credits status
- **422 Unprocessable Entity** — Invalid parameter — check country/device/date format in the error response
- **reviews search returns empty** — Reviews search requires a tracked app — run apptweak-pp-cli tracked-apps create first

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ngo275/app-agent**](https://github.com/ngo275/app-agent) — TypeScript (177 stars)
- [**semihcihan/App-Store-Optimization-CLI**](https://github.com/semihcihan/App-Store-Optimization-CLI) — TypeScript (90 stars)
- [**ckz/apptweak-mcp**](https://github.com/ckz/apptweak-mcp) — TypeScript (1 stars)
- [**abhi11/apptweak**](https://github.com/abhi11/apptweak) — Go (1 stars)
- [**Nikita-Guzenko/apptweak-mcp**](https://github.com/Nikita-Guzenko/apptweak-mcp) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

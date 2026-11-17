# Autom CLI

**Every Autom search engine from the terminal, plus saved rank history, cross-engine and cross-country comparison, and per-engine credit spend from a local ledger.**

Search Google, Bing, Brave, DuckDuckGo and Yahoo through one key, with every documented parameter as a flag. rank check, batch run and the --save flag of the compare commands save each results page locally, so rank movers and visibility can answer what changed and where a domain stands, and credits shows where your credits went. Read-only: it searches and reads account usage; nothing on the Autom account is created or changed.

## Install

The recommended path installs both the `autom-pp-cli` binary and the `pp-autom` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install autom
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install autom --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install autom --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install autom --agent claude-code
npx -y @mvanhorn/printing-press-library install autom --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/autom/cmd/autom-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/autom-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install autom --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-autom --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-autom --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install autom --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/autom-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AUTOM_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/autom/cmd/autom-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "autom": {
      "command": "autom-pp-mcp",
      "env": {
        "AUTOM_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

### Remote MCP over HTTP

`autom-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `AUTOM_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `AUTOM_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export AUTOM_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
autom-pp-mcp --transport http
```

Set `AUTOM_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Quick Start

```bash
# Confirm the CLI is installed (no network, no credits)
autom-pp-cli doctor --dry-run

# Free: see your remaining quota, packs and credits
autom-pp-cli usage

# Where a domain ranks for a keyword, saved for later comparisons
autom-pp-cli rank check autom.dev --query "serp api pricing" --country fr

# Same query on several engines at once
autom-pp-cli engines compare "serp api pricing" --engines google,bing,brave --domain autom.dev

# What moved since last week, from the local history
autom-pp-cli rank movers --since 7d

```

## Unique Features

### Search across engines and countries
- **`engines compare`** — See how one query ranks across Google, Bing, Brave, DuckDuckGo and Yahoo: shared domains, domains unique to each engine, and your domain's position on each. Costs 1 credit per engine.

  _Reach for this when a question is about visibility beyond Google or about which engines disagree._

  ```bash
  autom-pp-cli engines compare "serp api pricing" --engines google,bing,brave --domain autom.dev --agent
  ```
- **`geo compare`** — Run one query on one engine across several countries and see your domain's position and the shared results per country. Costs 1 credit per country.

  _Use this when the question is how rankings differ between markets._

  ```bash
  autom-pp-cli geo compare "serp api pricing" --countries fr,be,ca --domain autom.dev --agent
  ```

### Rank tracking from saved pages
- **`rank movers`** — List what moved across every tracked keyword since a cutoff: gains, drops, and entries into or exits from the top results. Reads the pages saved earlier by rank check, batch run or compare --save.

  _Use this for a weekly ranking report once pages have been saved on both sides of the cutoff; on a fresh store it returns a note._

  ```bash
  autom-pp-cli rank movers --since 7d --domain autom.dev --agent
  ```
- **`visibility`** — See one domain's footprint across every saved keyword: where it appears, average and best position, and top-3 and top-N (--top) counts, optionally against rival domains given with --vs. Reads the pages saved by rank check, batch run or compare --save.

  _Use this to compare a brand with competitors across a whole keyword set that you have already saved._

  ```bash
  autom-pp-cli visibility autom.dev --top 10 --agent
  ```

### Credit control
- **`credits`** — See your remaining quota, packs and credits plus where your credits went, by keyword, command or engine, with burn rate and days left. Spend covers only the calls made by this CLI; async searches show as pending.

  _Check this before and after large batches to predict spend and find the expensive workload._

  ```bash
  autom-pp-cli credits --by engine --since 30d --agent
  ```

### Monitoring
- **`news watch`** — Get only the news articles about a brand or keyword that earlier watches have not shown. Returned articles are marked as seen; --peek previews without marking. Costs 1 credit.

  _Schedule this for brand monitoring without re-reading the same articles._

  ```bash
  autom-pp-cli news watch "Autom" --country fr --agent
  ```

## Recipes

### Narrow a Google search to links

```bash
autom-pp-cli google search --query "model context protocol" --agent --select organic_results.position,organic_results.link
```

Keep only positions and links from a large results payload. Costs 1 credit.

### Check and save a ranking

```bash
autom-pp-cli rank check autom.dev --query "serp api pricing" --country fr
```

Position of a domain for one keyword; the page is saved for rank history and movers. Costs 1 credit.

### Track a keyword list

```bash
autom-pp-cli batch run --keywords "serp api pricing,google search scraper" --domain autom.dev --agent
```

Checks the balance first, then saves every page. Costs 1 credit per distinct keyword/engine/country/lang.

### Weekly ranking report

```bash
autom-pp-cli rank movers --since 7d --domain autom.dev --agent
```

Gains, drops, entries and exits versus pages saved 7+ days ago by rank check or batch run. Free: reads saved pages.

### Compare markets

```bash
autom-pp-cli geo compare "serp api pricing" --countries fr,be,ca --domain autom.dev
```

Position per country and the results the countries share. Costs 3 credits (one per country).

### Brand news without repeats

```bash
autom-pp-cli news watch "Autom" --country fr
```

Only articles earlier watches have not shown, then marks them as seen. Costs 1 credit.

## Usage

Run `autom-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AUTOM_CONFIG_DIR`, `AUTOM_DATA_DIR`, `AUTOM_STATE_DIR`, or `AUTOM_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AUTOM_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AUTOM_HOME=/srv/autom
autom-pp-cli doctor
```

Under `AUTOM_HOME=/srv/autom`, the four dirs resolve to `/srv/autom/config`, `/srv/autom/data`, `/srv/autom/state`, and `/srv/autom/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:AUTOM_HOME = "C:\srv\autom"
autom-pp-cli doctor
```

Under `$env:AUTOM_HOME = "C:\srv\autom"`, the four dirs resolve to `C:\srv\autom\config`, `C:\srv\autom\data`, `C:\srv\autom\state`, and `C:\srv\autom\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "autom": {
      "command": "autom-pp-mcp",
      "env": {
        "AUTOM_HOME": "/srv/autom"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AUTOM_DATA_DIR` overrides an explicit `--home` for that kind. Use `AUTOM_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AUTOM_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `autom-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `autom-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `autom-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Purging the store also deletes the saved results pages (rank history, movers, visibility) and the news watch memory. Config, credentials and the credit ledger (`credit-ledger.jsonl` in the data directory) are kept.
- **Run without a local copy:** set `AUTOM_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. rank check, batch run and compare --save do not save pages, news watch remembers nothing between runs, and the credit ledger is not written. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `AUTOM_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### bing

Bing web search

- **`autom-pp-cli bing`** - Query Bing Search via API and retrieve structured organic results (title, link, snippet, position).

Costs 1 credit per successful call.

### brave

Brave web, image and video search

- **`autom-pp-cli brave images`** - Search Brave Images via API and retrieve structured image results with pagination and SafeSearch.

Costs 1 credit per successful call.
- **`autom-pp-cli brave search`** - Search Brave Search via API and retrieve structured organic results with pagination.

Costs 1 credit per successful call.
- **`autom-pp-cli brave videos`** - Search Brave Videos via API and retrieve structured video results with pagination and SafeSearch.

Costs 1 credit per successful call.

### duckduckgo

DuckDuckGo web search

- **`autom-pp-cli duckduckgo`** - Query DuckDuckGo via API and retrieve structured organic results with region and language.

Costs 1 credit per successful call.

### finder

Free lookups of Google country, language and location codes

- **`autom-pp-cli finder google-countries`** - Search Google countries and retrieve country codes (free Finder endpoint).
- **`autom-pp-cli finder google-languages`** - Search Google languages and retrieve language codes (free Finder endpoint).
- **`autom-pp-cli finder google-locations`** - Find supported Google locations and retrieve location IDs and country codes (free endpoint).

### google

Google search, news, images, videos, shopping, maps, autocomplete and link resolving

- **`autom-pp-cli google autocomplete`** - Get Google Search autocomplete suggestions via API for a given query.

Costs 1 credit per successful call.
- **`autom-pp-cli google images`** - Search Google Images via API and retrieve image results (URLs, titles, sources, dimensions).

Costs 1 credit per successful call.
- **`autom-pp-cli google maps`** - Search Google Maps via API and retrieve structured place results (address, website, phone).

Costs 1 credit per successful call.
- **`autom-pp-cli google news`** - Search Google News via API and retrieve structured news results with pagination.

Costs 1 credit per successful call.
- **`autom-pp-cli google resolve`** - Resolve google.com/goto links from organic search results. Destinations replace /goto URLs; unresolved links stay with retryAfter.

Status: Experimental
- **`autom-pp-cli google search`** - Scrape Google Search results via API and retrieve structured SERP data with pagination.

Costs 1 credit per successful call.
- **`autom-pp-cli google search-light`** - Scrape lightweight Google Search results via API: fast, cost-efficient, organic results only.

Costs 1 credit per successful call.
- **`autom-pp-cli google shopping`** - Scrape Google Shopping results via API and retrieve structured product listings with prices, ratings, and merchant data.

Costs 1 credit per successful call.
- **`autom-pp-cli google videos`** - Search Google Videos via API and retrieve structured video results with metadata.

Costs 1 credit per successful call.

### job-results

Read an async job by id (`jobs get --wait` also polls until it finishes)

- **`autom-pp-cli job-results <id>`** - Poll the status, progress (0–100) and result of a job started with is_async=true. Polling is free. Poll until status is terminal: CompletedJob, Error, NotFound or BadRequest.

### stats

Free engine health over the last hour (needs --engine or --path)

- **`autom-pp-cli stats`** - Infrastructure health for an engine or endpoint over the last hour. Not scoped to your account or tenant. Enabled per account: HTTP 403 means contact support to activate it.

### usage

Free account balance, quotas and rate limits

- **`autom-pp-cli usage`** - Retrieve real-time usage statistics and credit balance for your Autom account.

### web-search

One multi-engine search: Autom walks its engines in order and returns the first organic results (404 when none has a result)

- **`autom-pp-cli web-search`** - One call walks engines in order and returns the first organic results. engines=auto tries Google, then Brave, Bing, DuckDuckGo, Yahoo, and Baidu. Omitted gl is us and omitted hl is en. There is no page parameter: the response is always page 1. An empty query returns 400. If every engine misses, the call returns 404. Send header x-api-key. 1 credit on HTTP 200.

Costs 1 credit per successful call.

### yahoo

Yahoo web search

- **`autom-pp-cli yahoo`** - Query Yahoo via API and retrieve structured organic results with country and language.

Costs 1 credit per successful call.


### credits


- **`autom-pp-cli credits`** - See your remaining quota, packs and credits plus where your credits went, by keyword, command or engine, with burn rate and days left. Spend covers only the calls made by this CLI; async searches show as pending.


### engines


- **`autom-pp-cli engines compare`** - See how one query ranks across Google, Bing, Brave, DuckDuckGo and Yahoo: shared domains, domains unique to each engine, and your domain's position on each. Costs 1 credit per engine.


### geo


- **`autom-pp-cli geo compare`** - Run one query on one engine across several countries and see your domain's position and the shared results per country. Costs 1 credit per country.


### news


- **`autom-pp-cli news watch`** - Get only the news articles about a brand or keyword that earlier watches have not shown. Returned articles are marked as seen; --peek previews without marking. Costs 1 credit.


### rank


- **`autom-pp-cli rank movers`** - List what moved across every tracked keyword since a cutoff: gains, drops, and entries into or exits from the top results. Reads the pages saved earlier by rank check, batch run or compare --save.
- **`autom-pp-cli rank check`** - Check where a domain ranks for a keyword on one engine and save the results page. Costs 1 credit.
- **`autom-pp-cli rank history`** - A domain's saved positions over time, per keyword. Reads saved pages; export with --csv.
- **`autom-pp-cli rank competitors`** - Who holds the top results for one keyword, and what changed since the previous saved page. Reads saved pages.


### visibility


- **`autom-pp-cli visibility`** - See one domain's footprint across every saved keyword: where it appears, average and best position, and top-3 and top-N (--top) counts, optionally against rival domains given with --vs. Reads the pages saved by rank check, batch run or compare --save.




### serp


- **`autom-pp-cli serp list`** - List saved results pages and their ids.
- **`autom-pp-cli serp diff`** - Which URLs entered, left or moved between two saved pages of a keyword.


### batch


- **`autom-pp-cli batch run`** - Check a keyword list (file, stdin or --keywords) after checking the balance, and save every page. Costs 1 credit per distinct keyword/engine/country/lang.


### ground


- **`autom-pp-cli ground`** - One numbered, deduplicated source list for a question across engines (and Google News with --news). Costs 1 credit per engine, plus 1 with --news.


### jobs


- **`autom-pp-cli jobs get`** - Read an async job started with --is-async; --wait polls until it finishes. Polling is free.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`autom-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`autom-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`autom-pp-cli learnings list`** - Inspect taught rows
- **`autom-pp-cli learnings forget <query>`** - Undo a teach
- **`autom-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`autom-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`autom-pp-cli teach-pattern`** - Install a query/resource template up front
- **`autom-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code fr ↔ France, engine alias ddg ↔ duckduckgo) for pattern substitution

Pass `--no-learn` or set `AUTOM_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `autom-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
autom-pp-cli brave search --query "serp api pricing"

# JSON for scripting and agents
autom-pp-cli brave search --query "serp api pricing" --json
# Filter to specific fields
autom-pp-cli brave search --query "serp api pricing" --json --select organic_results,search_parameters

# Dry run — show the request without sending
autom-pp-cli brave search --query "serp api pricing" --dry-run

# Agent mode — JSON + compact + no prompts in one flag
autom-pp-cli brave search --query "serp api pricing" --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only** - searches and reads account usage; nothing on the Autom account is created or changed
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Offline history** - results pages saved by rank check, batch run and compare --save are read offline by rank history, rank movers, rank competitors, serp list/diff and visibility
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
autom-pp-cli doctor
```

Reports configuration, paths, whether a key is set, and whether the API is reachable. It does not validate the key: run `autom-pp-cli usage` (free) for that.

## Configuration

Run `autom-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/autom-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\autom-pp-cli\config.toml` on Windows; `--home`, `AUTOM_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `AUTOM_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `AUTOM_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AUTOM_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `autom-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `autom-pp-cli doctor` to check credentials
- See which key the CLI uses with `autom-pp-cli auth status`, then confirm it works with `autom-pp-cli usage` (free)
**Not found errors (exit code 3)**
- `web-search` answers 404 when no engine has a result for the query
- `jobs get` and `job-results` answer 404 for an unknown or expired job id

### API-specific
- **HTTP 401 with "Rate limit exceeded"** — Autom signals rate limits with 401, and the CLI exits 7 rather than 4: wait and retry, or lower batch concurrency with --concurrency 1
- **HTTP 401 "Invalid API Key"** — Set AUTOM_API_KEY to a valid key from the Autom dashboard, then run autom-pp-cli usage (free) to confirm the key
- **HTTP 402 Payment required** — Out of credits: check autom-pp-cli credits, top up, or enable Auto Top-Up in the dashboard
- **Bing returns HTTP 500 when paging** — Drop --page on autom-pp-cli bing; the Autom endpoint currently fails when page is set
- **Requests time out before 60 seconds** — Autom can take up to 60 seconds; raise --timeout, or add --is-async and poll autom-pp-cli jobs get <job-id> --wait
- **sync does nothing and search costs credits** — Every Autom endpoint needs a query, so sync has nothing to crawl and search runs a billed live web-search in the default auto mode; pass --data-source local to read only saved data, and use rank history, rank movers, visibility or serp diff for offline work

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**seo-position-tracker**](https://github.com/dimitryzub/seo-position-tracker) — Python (22 stars)
- [**Decodo rank-tracker**](https://github.com/Decodo/rank-tracker) — Python
- [**Serply CLI**](https://github.com/mvanhorn/printing-press-library/tree/main/library/ai/serply) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

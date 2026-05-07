# Contact Goat CLI

Super LinkedIn for the terminal. Search, enrich, and map warm-intro paths
across LinkedIn, Happenstance, and Deepline from one SQLite-backed CLI.

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-contact-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-contact-goat --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-contact-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-contact-goat. The skill defines how its required CLI can be installed.
```

## Install

### Go

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/contact-goat-current).

## Authentication

Contact Goat reaches three sources. Each has its own auth path; all three
are optional — the CLI runs with whichever sources you've authenticated.

### Two auth surfaces (Happenstance)

Happenstance has two parallel auth paths and the CLI supports both. The
auto router prefers the cookie surface (free monthly allocation) and
falls back to the bearer surface (paid credits) only when the cookie
quota is exhausted.

| Surface | Auth | Cost | Default? |
|---------|------|------|----------|
| Cookie web app | Chrome session cookies | Free monthly allocation | YES (auto-prefer) |
| Public REST API | HAPPENSTANCE_API_KEY (Bearer) | 2 credits/search, 1 credit/research | Fallback only |

Pass `--source api` on `coverage`, `hp people`, `prospect`, or
`warm-intro` to opt into the bearer surface explicitly (richer schema,
group-scoped searches). The `api hpn *` subcommands always use the
bearer surface. Provision and rotate keys at
https://happenstance.ai/settings/api-keys.

### Happenstance (free web-app allocation, cookie auth)

Happenstance uses your logged-in Chrome session — no API key, no credit
burn. On macOS, run:

```bash
contact-goat-pp-cli auth login --chrome --service happenstance
```

Cookies are written to `~/.config/contact-goat-pp-cli/cookies-happenstance.json`
(mode 0600). The file contains plaintext cookie values — treat it like any
other credential file. For scripted environments, set the env var instead:

```bash
export HAPPENSTANCE_WEB_APP_COOKIE_AUTH="<your-cookie-auth>"
```

### LinkedIn (scraper subprocess, browser session)

Commands under `linkedin` and the cross-source commands (`warm-intro`,
`coverage`, `prospect`) call the
[stickerdaniel/linkedin-mcp-server](https://github.com/stickerdaniel/linkedin-mcp-server)
via `uvx`. Log in once:

```bash
uvx linkedin-scraper-mcp@latest --login
```

### Deepline (paid, credit-based)

Only needed for `waterfall`, `dossier --enrich-email`, `prospect --deepline`,
and `budget`. Get a key from [code.deepline.com](https://code.deepline.com):

```bash
export DEEPLINE_API_KEY="dlp_..."
```

If you have the official [Deepline CLI](https://code.deepline.com) installed
and authenticated (`deepline auth register` or `deepline auth status`),
contact-goat auto-discovers the saved key from
`~/.local/deepline/<host>/.env` — you do **not** need to re-export it into
your shell. The resolver checks `--deepline-key` first, then
`DEEPLINE_API_KEY` env, then the sibling-CLI file (mode 0600 or 0400
required, value must start with `dlp_`, no symlinks outside `$HOME`).
`contact-goat-pp-cli doctor` reports the resolution source as
`set (env)` / `set (flag)` / `set (file:~/.local/...)` / `not set`.

Every Deepline-touching command surfaces estimated credit cost before
execution and requires `--yes` to spend.

### Self-hosted Happenstance

Override the base URL with `CONTACT_GOAT_BASE_URL=https://your-host`.

## Quick Start

```bash
# 1. Check your setup (shows which sources are authenticated)
contact-goat-pp-cli doctor

# 2. Import Happenstance cookies from Chrome (macOS)
contact-goat-pp-cli auth login --chrome --service happenstance

# 3. See who you already know at a company
contact-goat-pp-cli coverage stripe

# 4. Find warm intros to a target person
contact-goat-pp-cli warm-intro "Patrick Collison"

# 5. What's changed in your network in the last week
contact-goat-pp-cli since 7d
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`warm-intro <target>`** — Given a target person, find mutual connections across your LinkedIn 1st-degree AND your Happenstance network who can introduce you.
- **`coverage <company>`** — For any company, see who you already know there via LinkedIn 1st-degree or Happenstance friends, ranked by relationship strength.
- **`prospect <query>`** — Fan-out search across LinkedIn, Happenstance, and Deepline (budget-gated). Dedupes by LinkedIn URL, ranks by network strength plus relevance.
- **`dossier <person>`** — Compose a single view of a person from LinkedIn profile, Happenstance research, and optionally Deepline enrichment.
- **`budget`** — Aggregate Deepline spend this month, recent high-cost calls, enforce limits.
- **`intersect`** — People who appear in BOTH your LinkedIn 1st-degree AND your Happenstance network. Your highest-signal warm intros.
- **`since <duration>`** — Time-windowed diff across LinkedIn connections, Happenstance feed, and new research.
- **`graph export`** — Export your full cross-source network as GEXF, DOT, or JSON for Gephi or Graphviz.
- **`waterfall <target>`** — Clay-style enrichment that walks LinkedIn → Happenstance → Deepline BYOK → Deepline managed, stopping at the first hit to minimize spend.

## Commands

### Cross-source (the reason to install)

| Command | What it does |
|---------|--------------|
| `warm-intro <target>` | Find mutuals across LinkedIn + Happenstance who can intro |
| `coverage <company>` | Who you know at a company, across sources |
| `prospect <query>` | Fan-out search, dedupe, rank by network strength |
| `dossier <person>` | Unified profile from all configured sources |
| `intersect` | People in both LinkedIn 1st-degree AND Happenstance |
| `since <duration>` | Time-windowed diff of new items |
| `engagement <person>` | Score last-touch engagement across all sources |
| `stale` | Warm intros going cold — researched but not followed up |
| `graph export` | Export the unified contact graph as GEXF/DOT/JSON |
| `waterfall <target>` | Progressive enrichment, cheapest source first |

### Happenstance (cookie auth)

| Command | What it does |
|---------|--------------|
| `feed` | Latest posts from your Happenstance network |
| `friends` | Your Happenstance friends (top connectors) |
| `groups` | Your Happenstance groups |
| `notifications` | New friend posts, research completions, search results |
| `research` | List recent research dossiers |
| `clerk --referrer-id` | Resolve a referrer UUID to a user profile |
| `dynamo --request-id` | Look up an async search by request_id |
| `user get` | Current authenticated user |
| `user get-limits` | Searches remaining, renewal date |
| `uploads` | Status of uploaded data sources (LinkedIn, Gmail, etc) |

### LinkedIn (scraper subprocess)

| Command | What it does |
|---------|--------------|
| `linkedin` | LinkedIn scraper powered by stickerdaniel/linkedin-mcp-server |

### Deepline (paid)

| Command | What it does |
|---------|--------------|
| `deepline` | Contact-data API: email, phone, company enrichment |
| `budget` | Credit spend this month, top tools, recent calls |

### Data layer (local SQLite)

| Command | What it does |
|---------|--------------|
| `sync` | Sync API data to local SQLite for offline search |
| `search <query>` | Full-text search across synced data or live API |
| `analytics` | Run analytics queries on locally synced data |
| `export` | Export data to JSONL or JSON for backup |
| `import` | Import data from JSONL via API create/upsert |
| `tail` | Stream NEW items across LinkedIn + Happenstance |

### Utilities

| Command | What it does |
|---------|--------------|
| `doctor` | Check CLI health, auth, and connectivity |
| `auth login --chrome --service happenstance` | Import Chrome cookies |
| `config byok set <provider> <env-var-name>` | Register BYOK keys for Deepline |
| `api` | Browse all API endpoints by interface name |
| `workflow` | Compound workflows that combine API operations |

Run any command with `--help` for full flag documentation.

## Output Formats

```bash
# Human-readable table (default in terminal)
contact-goat-pp-cli coverage stripe

# JSON when piped (automatic) or explicit
contact-goat-pp-cli coverage stripe --json

# Filter to specific fields
contact-goat-pp-cli friends --json --select name,company,connection_count

# CSV for spreadsheet import
contact-goat-pp-cli coverage openai --csv

# Compact — drop nulls and verbose fields
contact-goat-pp-cli dossier "Satya Nadella" --json --compact

# Dry run — preview without spending credits or making API calls
contact-goat-pp-cli prospect "CTO fintech" --deepline --budget 5 --dry-run

# Agent mode — sets --json --compact --no-input --no-color --yes
contact-goat-pp-cli warm-intro "Patrick Collison" --agent
```

## Agent Usage

Contact Goat is designed for AI agent consumption:

- **Non-interactive** — never prompts when `--no-input` or `--agent` is set
- **Pipeable** — `--json` to stdout, errors to stderr, one record per line where sensible
- **Filterable** — `--select field1,field2` returns only requested fields
- **Previewable** — `--dry-run` on every paid/mutating command
- **Cacheable** — GET responses cached; bypass with `--no-cache`
- **Confirmable** — `--yes` for explicit acknowledgement of Deepline credit spend
- **Typed exit codes** — `0` success, `2` usage, `3` not found, `4` auth, `5` API, `7` rate limited, `10` config

All agent defaults in one flag: `--agent`.

## Cookbook

```bash
# Find warm intros to a target (ranked by source strength)
contact-goat-pp-cli warm-intro "Patrick Collison" --json --limit 5

# Who do I know at OpenAI? Happenstance only, no LinkedIn scraper call.
contact-goat-pp-cli coverage "OpenAI" --source hp --limit 10

# Prospect without spending Deepline credits
contact-goat-pp-cli prospect "VP engineering fintech" --limit 50

# Prospect WITH Deepline, hard-capped at 5 credits
contact-goat-pp-cli prospect "Director Product" --deepline --budget 5 --yes

# One-shot unified profile
contact-goat-pp-cli dossier https://www.linkedin.com/in/patrickcollison --json

# Enrich a bare email via the cheapest source that has it
contact-goat-pp-cli waterfall alice@stripe.com --max-cost 2 --json

# Prefer BYOK keys for Deepline steps; fail if none configured
contact-goat-pp-cli waterfall "Brian Chesky" --company airbnb.com --byok

# What's new in the last 24 hours, JSON
contact-goat-pp-cli since 24h --json

# People in BOTH LinkedIn 1st-degree AND Happenstance friends (your strongest signals)
contact-goat-pp-cli intersect --json

# Export the unified graph for Gephi
contact-goat-pp-cli graph export --format gexf > network.gexf

# Stream NEW items across sources (watch mode)
contact-goat-pp-cli tail --sources li,hp

# Search locally synced data with FTS5
contact-goat-pp-cli search "fintech" --data-source local --limit 20

# Deepline spend this month
contact-goat-pp-cli budget --json

# Sync, then search, with a dedicated DB path
contact-goat-pp-cli sync --resources friends
contact-goat-pp-cli search "stripe" --data-source local

# Register a BYOK key so waterfall uses your Hunter subscription
contact-goat-pp-cli config byok set hunter HUNTER_API_KEY
```

## Health Check

```bash
contact-goat-pp-cli doctor
```

Sample output:

```
  OK Config: ok
  OK Auth: configured
  OK API: reachable
  OK Happenstance: cookies: found: 14 cookies (~/.config/contact-goat-pp-cli/cookies-happenstance.json)
  OK Happenstance: session JWT: valid
  OK LinkedIn: Python: ok (3.14.3)
  OK LinkedIn: uvx: ok
  OK LinkedIn: binary: will launch via `uvx linkedin-scraper-mcp@latest`
  WARN LinkedIn: profile: not logged in — run `uvx linkedin-scraper-mcp@latest --login`
  OK Deepline: DEEPLINE_API_KEY: set (file:~/.local/deepline/code-deepline-com/.env)
  OK Deepline: prefix: ok (dlp_)
  OK Deepline: CLI on PATH: /Users/you/.local/bin/deepline
  config_path: ~/.config/contact-goat-pp-cli/config.toml
  base_url: https://happenstance.ai
  version: 0.1.0
```

`WARN` rows are OK — they just mean that source isn't wired up yet.

## Configuration

Config file: `~/.config/contact-goat-pp-cli/config.toml`

Environment variables:

| Variable | Purpose |
|----------|---------|
| `HAPPENSTANCE_WEB_APP_COOKIE_AUTH` | Cookie auth blob for Happenstance (alternative to `auth login --chrome`) |
| `HAPPENSTANCE_API_KEY` | Bearer key for the Happenstance public REST API (`hpn_live_personal_...`). Provision at https://happenstance.ai/settings/api-keys |
| `DEEPLINE_API_KEY` | Deepline API key (`dlp_...`). Required for paid commands |
| `CONTACT_GOAT_BASE_URL` | Override Happenstance base URL (self-hosted) |
| `CONTACT_GOAT_CONFIG` | Config file path override |
| `HUNTER_API_KEY`, `APOLLO_API_KEY`, ... | BYOK keys registered via `config byok set` |
| `NO_COLOR` | Disable colored output (standard) |

## Use as MCP Server

Contact Goat ships a companion MCP server for Claude Desktop, Cursor, and
other MCP-compatible tools.

### Claude Code

```bash
claude mcp add contact-goat contact-goat-pp-mcp \
  -e HAPPENSTANCE_WEB_APP_COOKIE_AUTH="$HAPPENSTANCE_WEB_APP_COOKIE_AUTH" \
  -e DEEPLINE_API_KEY="$DEEPLINE_API_KEY"
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "contact-goat": {
      "command": "contact-goat-pp-mcp",
      "env": {
        "HAPPENSTANCE_WEB_APP_COOKIE_AUTH": "<your-cookie-auth>",
        "DEEPLINE_API_KEY": "<your-deepline-key>"
      }
    }
  }
}
```

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `contact-goat-pp-cli doctor` to see which source is unauthenticated
- For Happenstance: `contact-goat-pp-cli auth login --chrome --service happenstance`
- For LinkedIn: `uvx linkedin-scraper-mcp@latest --login`
- For Deepline: `export DEEPLINE_API_KEY=dlp_...`

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- Default rate limit is 2 requests/second — raise with `--rate-limit 5`
- Happenstance free tier is 14 searches/month; `user get-limits` shows remaining

**Deepline credit errors**
- `waterfall`, `prospect --deepline`, and `dossier --enrich-email` require
  both `--budget > 0` and `--yes` to spend
- `contact-goat-pp-cli budget` shows your month-to-date spend

**Not found errors (exit code 3)**
- LinkedIn URLs and slugs are validated strictly — check for typos
- Happenstance research dossiers expire; re-run `dossier` with `--no-cache`

## Rate Limits

- LinkedIn scraper: governed by your own browser session; no documented limit
- Happenstance web app: 14 searches/month on free tier (your actual limit is in `user get-limits`)
- Deepline: credit-based, see your balance at [code.deepline.com](https://code.deepline.com)

## Data Sources

This CLI unifies three distinct sources:

- **LinkedIn** via [stickerdaniel/linkedin-mcp-server](https://github.com/stickerdaniel/linkedin-mcp-server) (Python MCP, subprocess)
- **Happenstance** via sniffed web-app endpoints (Clerk session cookies, free tier)
- **Deepline** via [code.deepline.com](https://code.deepline.com) (Bearer token, credit-based)

The web-app Happenstance endpoints are DISTINCT from the public REST API at
`api.happenstance.ai/v1` — that one uses Bearer-token auth and burns paid
credits. We target the web app your browser already uses.

---

## Sources & Inspiration

Built by studying these projects:

- [**stickerdaniel/linkedin-mcp-server**](https://github.com/stickerdaniel/linkedin-mcp-server) — Python (1610 stars)
- [**joeyism/linkedin_scraper**](https://github.com/joeyism/linkedin_scraper) — Python (3969 stars)
- [**tomquirk/linkedin-api**](https://github.com/tomquirk/linkedin-api) — Python
- [**Happenstance hpn**](https://developer.happenstance.ai/cli/install) — Python
- [**Deepline CLI**](https://code.deepline.com) — Shell

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).

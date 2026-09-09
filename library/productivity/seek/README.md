# SEEK CLI

**Every SEEK search and job-detail feature, plus a local job history, salary distributions, and hiring trends no other SEEK tool has.**

seek-pp-cli searches Australian and New Zealand job listings, pulls full job details through SEEK's own JSON endpoints (never the bot-gated HTML), and keeps every job it has seen in a local SQLite store. On top of that it computes salary percentiles (salary), hiring-volume trends over time (trends), per-company hiring scans (company), and runs all your saved searches for new postings in one command (me new-jobs). No API key; no hosted scraper.

Learn more at [SEEK](https://au.seek.com).

Created by [@polsiola-dot](https://github.com/polsiola-dot) (Paul Siola).

## Install

The recommended path installs both the `seek-pp-cli` binary and the `pp-seek` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install seek
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install seek --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install seek --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install seek --agent claude-code
npx -y @mvanhorn/printing-press-library install seek --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/seek/cmd/seek-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seek-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install seek --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-seek --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-seek --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install seek --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
seek-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seek-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/seek/cmd/seek-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "seek": {
      "command": "seek-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Job search, job details, salary data, and company profiles need no authentication. The me commands (saved searches, saved jobs, applied status) read your SEEK session: run `seek-pp-cli auth login --chrome` once while logged in to au.seek.com in Chrome and the CLI imports the session cookie. Nothing is written to your SEEK account.

## Quick Start

```bash
# confirm the CLI can reach SEEK before anything else
seek-pp-cli doctor --dry-run

# the core workflow: keyword + location + recency
seek-pp-cli listings search --keywords "software engineer" --where "Sydney NSW" --posted-within-days 7

# full description, salary, apply link and company for one job
seek-pp-cli listings get 94483533

# shape of the market with zero rows fetched
seek-pp-cli listings facets --keywords "software engineer" --where "Sydney NSW" --group-by classification

# salary percentiles — the command fetches and caches listings itself
seek-pp-cli salary "software engineer" --where "Sydney NSW" --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`salary`** — Get salary percentiles, a histogram, and the pay-disclosure rate for a role and location by sampling matching listings (which it also caches locally).

  _Reach for this instead of fetching hundreds of listings and computing pay statistics yourself._

  ```bash
  seek-pp-cli salary "registered nurse" --where "Melbourne VIC" --agent
  ```
- **`trends`** — See how job-listing volume for a classification, region, or work arrangement changes over time, bucketed from the listing dates of every job the CLI has cached, with period-over-period deltas.

  _Use this when the question is 'is hiring up or down', not 'what is open right now'._

  ```bash
  seek-pp-cli trends --by classification --since 90d --agent
  ```

### Cross-entity joins
- **`company`** — Profile one advertiser: their company details and review ratings plus every current opening and a local history of how many roles they've posted.

  _Reach for this for 'who is hiring for X' market scans and interview prep instead of scrolling a company's SEEK page._

  ```bash
  seek-pp-cli company "Atlassian" --active --agent
  ```
- **`me new-jobs`** — Run every one of your SEEK saved searches and return only the postings that aren't already in your local store, tagged by which search matched.

  _This is the daily-driver command for an active job hunt; use it instead of re-scrolling the SEEK site._

  ```bash
  seek-pp-cli me new-jobs --since 7d --agent
  ```

### Agent-native plumbing
- **`listings facets`** — Break a live query down by classification, work type, pay band, or region by tallying the sampled result pages — no local store needed.

  _Use this for a fast shape-of-the-market answer before committing to a full search or a sync._

  ```bash
  seek-pp-cli listings facets --keywords "data analyst" --where "Brisbane QLD" --group-by classification --agent
  ```
- **`classifications`** — Browse or resolve SEEK's numeric classification and subclassification IDs so filtered searches can target them precisely.

  _Call this first when you need a --classification value for listings search._

  ```bash
  seek-pp-cli classifications software
  ```

## Recipes

### Daily new-jobs digest

```bash
seek-pp-cli me new-jobs --since 24h --agent
```

Runs every saved search and returns only postings not already in the local store.

### Salary benchmark for a role

```bash
seek-pp-cli salary "data engineer" --where "All Australia" --agent
```

Percentiles and disclosure rate over the listings the command samples and caches for that role and location.

### Competitor hiring snapshot

```bash
seek-pp-cli company "Canva" --agent --select openings.title,openings.location,openings.listingDate
```

All current openings for one advertiser with just the fields an agent needs from a large nested response.

### Is ICT hiring cooling?

```bash
seek-pp-cli trends --by classification --since 180d --agent
```

Period-over-period listing counts bucketed from the listing dates of jobs cached locally.

### Shape of a market before a full search

```bash
seek-pp-cli listings facets --keywords "registered nurse" --where "Perth WA" --group-by salary --agent
```

Per-facet counts tallied from the sampled result pages, zero extra rows fetched.

## Usage

Run `seek-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SEEK_CONFIG_DIR`, `SEEK_DATA_DIR`, `SEEK_STATE_DIR`, or `SEEK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SEEK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SEEK_HOME=/srv/seek
seek-pp-cli doctor
```

Under `SEEK_HOME=/srv/seek`, the four dirs resolve to `/srv/seek/config`, `/srv/seek/data`, `/srv/seek/state`, and `/srv/seek/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "seek": {
      "command": "seek-pp-mcp",
      "env": {
        "SEEK_HOME": "/srv/seek"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SEEK_DATA_DIR` overrides an explicit `--home` for that kind. Use `SEEK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SEEK_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `seek-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### listings

Search and inspect SEEK job listings

- **`seek-pp-cli listings`** - Search job listings by keyword, location, and filters

### me

Your SEEK account: saved searches, saved jobs, applied jobs (needs `auth login --chrome`)

- **`seek-pp-cli me job-status`** - Check which of the given job IDs you've saved or already applied to
- **`seek-pp-cli me saved-jobs`** - List jobs you've saved on SEEK
- **`seek-pp-cli me saved-searches`** - List your saved searches / job alerts


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`seek-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`seek-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`seek-pp-cli learnings list`** - Inspect taught rows
- **`seek-pp-cli learnings forget <query>`** - Undo a teach
- **`seek-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`seek-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`seek-pp-cli teach-pattern`** - Install a query/resource template up front
- **`seek-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SEEK_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `seek-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
seek-pp-cli listings

# JSON for scripting and agents
seek-pp-cli listings --json
# Filter to specific fields
seek-pp-cli listings --json --select id,title,teaser

# Dry run — show the request without sending
seek-pp-cli listings --dry-run

# Agent mode — JSON + compact + no prompts in one flag
seek-pp-cli listings --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
seek-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `seek-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/seek-cli/config.toml`; `--home`, `SEEK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SEEK_COOKIE` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `seek-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `seek-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SEEK_COOKIE`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **listings get or a company scan returns nothing for a valid job ID** — SEEK's GraphQL schema may have shifted; re-run with --json to see the raw error, then trim the query fields the error names.
- **me new-jobs or me saved-searches says you are not authenticated** — run `seek-pp-cli auth login --chrome` while logged in to au.seek.com in Chrome; only the me commands need this.
- **salary or trends shows a thin distribution or no history** — these commands accumulate listings locally on each run; run the same query a few times over days, or widen --where / drop filters for a bigger sample.
- **search returns AU jobs when you wanted NZ** — pass --site NZ-Main --locale en-NZ (both are needed).

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://au.seek.com/
- Capture coverage: 12 API entries from 60 total network entries
- Reachability: standard_http (90% confidence)
- Protocols: rest_json (95% confidence), graphql (95% confidence), ssr_embedded_data (80% confidence)
- Auth signals: cookie — cookies: SEEK_SESSION; none
- Protection signals: cloudflare (60% confidence)
- Generation hints: primary transport: standard_http against https://au.seek.com, search surface is REST JSON at /api/jobsearch/v5/search; job-detail and account surfaces are GraphQL at POST /graphql, graphql endpoint rejects introspection and enforces a real schema; jobDetails(id: ID!) and jobSearchV7(params: JobSearchV7QueryInput!) verified working with hand-written queries, AU and NZ share the au.seek.com host; switch via siteKey=AU-Main|NZ-Main and locale=en-AU|en-NZ, authenticated surface uses same-origin session cookies on .seek.com (no Authorization header, no CSRF token on read queries); emit auth login --chrome / press-auth cookie companion, requires_browser_auth for the me/* commands only; all jobs/* commands are unauthenticated
- Candidate command ideas: search — GET /api/jobsearch/v5/search — keyword+location+filter job search, the primary workflow; get — POST /graphql jobDetails(id) — full job description, salary, apply link; count — POST /graphql jobSearchV7 JobCountV7 — result count for a filter combination without fetching rows; recommended — POST /graphql JobDetailsRecommendedJobs(jobDetailsId) — similar jobs; saved-searches — viewer -> ApacSavedSearch{id,name,query,createdDate,newToYouCountLabel,subscribeToNewJobs} (cookie auth); saved-jobs — viewer.savedJobs(first) (cookie auth); applied-jobs — viewer.searchAppliedJobs / applied job history (cookie auth)

Warnings from discovery:
- hand_written_queries: GraphQL operation bodies were reconstructed by the agent and verified against the live endpoint (HTTP 200); they are leaner than the SPA's persisted queries. jobDetailsPersonalised/GetMatchedQualities field selections were observed but not fully transcribed.
- cookie_replay_unverified: Cookie auth confirmed working in-browser (viewer queries returned data). Replay outside the browser was not validated this run because session cookie values were deliberately not extracted; Phase 5 live smoke or first auth login --chrome will confirm.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**seek-scraper (BlackFalconData)**](https://github.com/BlackFalconData-org/seek-scraper) — TypeScript
- [**SeekSpider**](https://github.com/qinscode/SeekSpider) — Python
- [**seek-com-au-api**](https://github.com/tomquirk/seek-com-au-api) — Python
- [**job-hunter-mcp**](https://github.com/patrickvicente/job-hunter-mcp) — Python
- [**jobapply-mcp-server**](https://github.com/Sakshee5/jobapply-mcp-server) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

# Amazon Jobs CLI

**Every Amazon.jobs feature, plus a local job store, new-since diffing, and aggregation Amazon's own site can't do — no login required.**

amazon-jobs turns Amazon's careers site from a page you refresh into a queryable, watchable dataset. It uses the same unauthenticated JSON the site does, so there is no API key and no scraping fragility. Beyond search and filters, it keeps a local SQLite mirror that powers `new` (reqs unseen since your last check), `stats` (counts by city/team/category the empty server facets can't give), and `skills` (which teams demand a given qualification).

Created by [@qazmataz](https://github.com/qazmataz) (qazmataz).

## Install

The recommended path installs both the `amazon-jobs-pp-cli` binary and the `pp-amazon-jobs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install amazon-jobs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install amazon-jobs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install amazon-jobs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install amazon-jobs --agent claude-code
npx -y @mvanhorn/printing-press-library install amazon-jobs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/cmd/amazon-jobs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amazon-jobs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install amazon-jobs --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-amazon-jobs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-amazon-jobs --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install amazon-jobs --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amazon-jobs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/cmd/amazon-jobs-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "amazon-jobs": {
      "command": "amazon-jobs-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the API is reachable (no auth needed).
amazon-jobs-pp-cli doctor


# Search live listings by keyword and country.
amazon-jobs-pp-cli find "software engineer" --country USA --limit 10


# Agent-native output, narrowed to the fields that matter (skips the huge description).
amazon-jobs-pp-cli find "solutions architect" --city Seattle --agent --select title,location,posted_date


# Persist a named search to track over time.
amazon-jobs-pp-cli save sde-seattle "software engineer" --city Seattle --country USA


# Mirror listings into the local store.
amazon-jobs-pp-cli sync engineer --max-pages 5


# Aggregate the synced store by city — counts Amazon's site never shows.
amazon-jobs-pp-cli stats --by city

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`new`** — See only the Amazon reqs that appeared since you last synced a saved search — no more re-scanning the whole list every morning.

  _Reach for this when a user tracks a role over time and wants the delta, not the full list._

  ```bash
  amazon-jobs-pp-cli new sde-seattle --agent
  ```
- **`save`** — Persist a named query plus its filters and diff cursor so a search and its new-since state survive between runs.

  _Use this to set up a repeatable watch that `new` and `sync` then operate on by name._

  ```bash
  amazon-jobs-pp-cli save sde-seattle "software engineer" --city Seattle --country USA
  ```

### Aggregation the API can't do

- **`stats`** — Count synced reqs grouped by city, state, team, or category — the aggregation Amazon's own empty facets[] never returns.

  _Use this to answer 'which cities/teams have the most open reqs' without paging every result._

  ```bash
  amazon-jobs-pp-cli stats --by city --agent
  ```
- **`skills`** — Rank teams and cities by how many synced reqs demand a given skill keyword in their basic/preferred qualifications.

  _Reach for this for labor-market questions like 'who is hiring for X skill' rather than retrieving individual reqs._

  ```bash
  amazon-jobs-pp-cli skills Rust --agent
  ```

### Honest client-side filtering

- **`find`** — Filter live results by intern, manager, university, and schedule type — fields the .json endpoint silently ignores as server params.

  _Use these flags when a user wants senior-IC, non-intern, or schedule-specific roles that Amazon can't filter server-side._

  ```bash
  amazon-jobs-pp-cli find "software engineer" --manager=false --intern=false --agent
  ```

### True recency, not the re-index clock

- **`find --posted-within`** — Filter on `posted_date`, the real posting date, with `24h` / `3d` / `2w` style windows.

  Amazon's `updated_time` field looks like a freshness signal and is not one: it tracks the last edit or re-index of any kind. In a 1000-req live sample, **514 reqs were posted more than 14 days ago and every one of them reported an `updated_time` inside 48 hours** — including a req posted in August 2025 still reading "about 21 hours". Anyone sorting or filtering on `updated_time` to "apply the fastest" is reading a re-index clock.

  `find` and `new` now print both dates side by side and mark the rows where they disagree badly with `(edited)`; JSON output carries `"updated_diverged": true` on those rows.

  `posted_date` is day-granular — the API exposes no sub-day posting timestamp — so `--posted-within` is inclusive **by date, not by clock**: `--posted-within 7d` means "posted on or after (today − 7 days)", counting whole dates.

  ```bash
  amazon-jobs-pp-cli find "program manager" --country GBR --posted-within 7d
  ```

### Search the fine print

- **`find --description-contains` / `--description-not-contains`** — Case-insensitive regex over `description` + `basic_qualifications` + `preferred_qualifications`, with HTML stripped before matching.

  _This is where the disqualifiers hide: visa/sponsorship language, relocation terms (which cut both ways — "Relocation assistance is NOT provided" vs. "Relocation benefits are offered"), and language requirements like Mandarin or Japanese N1. None of it is a server-side facet._

  Patterns that aren't valid regex syntax are matched literally, so `C++` works as typed.

  ```bash
  amazon-jobs-pp-cli find "" --country SGP --description-not-contains "without sponsorship"
  ```

## Recipes


### Track new SDE roles in Seattle each morning

```bash
amazon-jobs-pp-cli save sde-seattle "software engineer" --city Seattle --country USA && amazon-jobs-pp-cli new sde-seattle --agent
```

Persist a named search once, then run `new` to see only reqs that appeared since your last sync.

### Agent-native search narrowed to key fields

```bash
amazon-jobs-pp-cli find "solutions architect" --country USA --agent --select title,location,posted_date,job_path
```

Amazon reqs carry huge description/qualification text; --select trims the payload to the fields an agent needs.

### Where is AWS hiring most right now?

```bash
amazon-jobs-pp-cli sync aws --max-pages 10 && amazon-jobs-pp-cli stats --by city
```

Mirror AWS reqs locally, then aggregate by city — counts the empty server facets never return.

### Which teams demand a specific skill

```bash
amazon-jobs-pp-cli sync engineer --max-pages 10 && amazon-jobs-pp-cli skills Rust --agent
```

Scan synced qualification text for a keyword and rank teams/cities by demand.

### Senior IC roles only (no manager, no intern)

```bash
amazon-jobs-pp-cli find "software engineer" --country USA --manager=false --intern=false
```

Client-side NULL-safe filters for fields Amazon can't filter server-side.

### Only reqs actually posted this week

```bash
amazon-jobs-pp-cli find "program manager" --country GBR --posted-within 7d --max-scan-pages 10
```

Filters on the true `posted_date`. Rows printed with `(edited)` were posted long ago and merely re-indexed — `updated_time` alone would have made them look brand new.

### Screen out roles that won't sponsor a visa

```bash
amazon-jobs-pp-cli find "" --country SGP --description-not-contains "without sponsorship" --posted-within 2w --agent
```

Sponsorship, relocation, and language requirements live only in the description text. Combine the text filter with a recency window to get a shortlist worth applying to; raise `--max-scan-pages` when the match rate is low.

## Usage

Run `amazon-jobs-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AMAZON_JOBS_CONFIG_DIR`, `AMAZON_JOBS_DATA_DIR`, `AMAZON_JOBS_STATE_DIR`, or `AMAZON_JOBS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AMAZON_JOBS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AMAZON_JOBS_HOME=/srv/amazon-jobs
amazon-jobs-pp-cli doctor
```

Under `AMAZON_JOBS_HOME=/srv/amazon-jobs`, the four dirs resolve to `/srv/amazon-jobs/config`, `/srv/amazon-jobs/data`, `/srv/amazon-jobs/state`, and `/srv/amazon-jobs/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "amazon-jobs": {
      "command": "amazon-jobs-pp-mcp",
      "env": {
        "AMAZON_JOBS_HOME": "/srv/amazon-jobs"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AMAZON_JOBS_DATA_DIR` overrides an explicit `--home` for that kind. Use `AMAZON_JOBS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AMAZON_JOBS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `amazon-jobs-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### postings

Search Amazon job listings

- **`amazon-jobs-pp-cli postings`** - Search Amazon job listings by keyword, location, and sort order


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`amazon-jobs-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`amazon-jobs-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`amazon-jobs-pp-cli learnings list`** - Inspect taught rows
- **`amazon-jobs-pp-cli learnings forget <query>`** - Undo a teach
- **`amazon-jobs-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`amazon-jobs-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`amazon-jobs-pp-cli teach-pattern`** - Install a query/resource template up front
- **`amazon-jobs-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `AMAZON_JOBS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `amazon-jobs-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
amazon-jobs-pp-cli postings

# JSON for scripting and agents
amazon-jobs-pp-cli postings --json

# Filter to specific fields
amazon-jobs-pp-cli postings --json --select id,name,status

# Dry run — show the request without sending
amazon-jobs-pp-cli postings --dry-run

# Agent mode — JSON + compact + no prompts in one flag
amazon-jobs-pp-cli postings --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
amazon-jobs-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `amazon-jobs-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/amazon-jobs-pp-cli/config.toml`; `--home`, `AMAZON_JOBS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

### Overriding the API host

The CLI talks to `https://www.amazon.jobs` by default. Override it with the
`base_url` key in `config.toml`, or per-invocation with `AMAZON_JOBS_BASE_URL`:

```bash
AMAZON_JOBS_BASE_URL=https://www.amazon.jobs amazon-jobs-pp-cli doctor
```

This is the escape hatch when a local resolver, split-DNS setup, or corporate
proxy will not answer for `www.amazon.jobs` — point the CLI at a mirror or a
local forwarding proxy without editing config. `doctor` prints the effective
`base_url`, so it is the fastest way to confirm the override took effect.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **A search with filters returns zero results but the same query without filters returns thousands.** — Always pass --limit >= 1; the API returns 0 hits when result_limit is 0 combined with a location filter.
- **`new` or `stats` says the local mirror is empty.** — Run `amazon-jobs-pp-cli sync engineer` (or `sync` a saved search) first to populate the store.
- **Job descriptions are full of <br/> and HTML entities.** — Add --plain to `find`/`get` to strip HTML into readable text.
- **Broad searches always report about 10000 hits.** — That is Amazon's server-side cap for broad queries; narrow with keyword/location filters, or sync and use `stats` for exact local counts.
- **`doctor` reports `cannot resolve host "www.amazon.jobs"` and every command fails immediately.** — This is your machine's DNS, not the API. Some home routers and split-DNS/VPN setups refuse the `amazon.jobs` zone; confirm with `host amazon.jobs` versus `host amazon.jobs 1.1.1.1`. Switch to a public resolver (1.1.1.1 or 8.8.8.8), disconnect the VPN, or set `AMAZON_JOBS_BASE_URL` to a reachable host. Unresolvable hosts fail fast by design — the CLI does not retry a name the resolver has already refused.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**shubhtoy/amazon-jobs-scraper**](https://github.com/shubhtoy/amazon-jobs-scraper) — Python
- [**marcogdepinto/amazon-jobs-scraper**](https://github.com/marcogdepinto/amazon-jobs-scraper) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

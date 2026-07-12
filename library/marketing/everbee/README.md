# EverBee CLI

**Etsy niche research whose scores come with their evidence attached — seeded search, honest confidence, and a local store that turns repeat research into trends.**

EverBee's data is the best Etsy research signal available, but its API will happily answer a question you did not ask: the default suggestion feeds return unranked filler regardless of your seed. This CLI queries the endpoints EverBee's own search boxes call, then stamps every returned row with a relevance score, an evidence count, and provenance. Confidence tracks evidence coverage, so a niche with no keyword support cannot come back looking confident. Use 'research niche' for a defensible verdict on one seed, 'research subniches' to rank a whole family of them, and 'selftest' to prove the data path is semantically sound before you trust a batch run.

Learn more at [EverBee](https://api.everbee.com).

Created by [@horknfbr](https://github.com/horknfbr) (horknfbr).

## Install

The recommended path installs both the `everbee-pp-cli` binary and the `pp-everbee` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install everbee
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install everbee --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install everbee --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install everbee --agent claude-code
npx -y @mvanhorn/printing-press-library install everbee --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/everbee-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install everbee --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-everbee --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-everbee --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install everbee --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/everbee-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EVERBEE_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "everbee": {
      "command": "everbee-pp-mcp",
      "env": {
        "EVERBEE_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

EverBee authenticates with a session token minted by Google SSO — there is no API-key page. Run 'everbee-pp-cli auth setup' for the steps to obtain a token, then store it with 'everbee-pp-cli auth set-token <token>', or set EVERBEE_ACCESS_TOKEN directly. Tokens expire; a 401 means the token needs replacing, not that the CLI is broken. Your EverBee plan gates research volume: the free Hobby plan allows only 10 keyword searches per month, and this CLI reports that cap as a typed error rather than as an empty result.

## Quick Start

```bash
# Confirm config, token, and API reachability before spending any research quota.
everbee-pp-cli doctor --dry-run

# Prove the research path returns semantically relevant data, not just HTTP 200s.
everbee-pp-cli selftest --agent

# The core workflow: a scored niche verdict with its evidence and confidence attached.
everbee-pp-cli research niche "dad shirt" --agent

# Rank the child niches under a parent, with digital SVG/PNG listings excluded.
everbee-pp-cli research subniches --parent dad --product apparel --exclude-svg-png --agent

# Persist research locally so drift baselines and offline search have something to compare against.
everbee-pp-cli sync --resources products,keyword_research,shops

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Evidence-aware research
- **`research niche`** — Score an Etsy niche from a seed keyword and get the evidence behind the score, not just the number.

  _Reach for this instead of a raw keyword call when you need to defend a low-competition claim: every verdict carries its evidence count, provenance, and an honest confidence._

  ```bash
  everbee-pp-cli research niche "dad shirt" --agent
  ```
- **`research subniches`** — Expand a parent niche into child niches and rank them on comparable, normalized scores.

  _Use this when the task is 'find me the least-crowded corner of X' rather than 'tell me about X'._

  ```bash
  everbee-pp-cli research subniches --parent dad --product apparel --exclude-svg-png --agent
  ```
- **`research competitors`** — Get the market shape of a niche: result count, median price, review and sales density, listing-age quartiles.

  _Answers 'who would I be competing against, and how entrenched are they' before any design work starts._

  ```bash
  everbee-pp-cli research competitors "dad shirt" --agent
  ```
- **`research tags`** — See which tags and title tokens the winning listings in a niche agree on, and whether demand is seasonal or evergreen when EverBee supplies trend data.

  _Use before writing a listing: it gives you the consensus vocabulary of the niche. The seasonality verdict is reported as 'unknown' when EverBee returns no trend data, which is common — it never guesses._

  ```bash
  everbee-pp-cli research tags "dad shirt" --agent
  ```

### Local state that compounds
- **`research drift`** — Compare a niche against a saved baseline to see what actually moved since last time.

  _Turns repeated research into a trend instead of a series of disconnected screenshots._

  ```bash
  everbee-pp-cli research drift "dad shirt" --agent
  ```
- **`research listing`** — Resolve an Etsy listing URL or ID to what we actually know about it, and say so plainly when we know nothing.

  _Distinguishes 'this listing does not exist' from 'we have no data on it yet' — the two failures an agent must never conflate._

  ```bash
  everbee-pp-cli research listing 4515173344 --agent
  ```

### Agent-native plumbing
- **`selftest`** — Check that the research path is not just reachable but actually returning relevant data.

  _Run this first in any automated session: it is the difference between 'the API answered' and 'the answer means something'._

  ```bash
  everbee-pp-cli selftest --agent
  ```

## Recipes

### Defend a low-competition claim

```bash
everbee-pp-cli research niche "dad shirt" --agent
```

Returns demand, competition, saturation, price band, evidence count, and an opportunity score, plus the provenance of each metric so the verdict can be audited rather than trusted.

### Find the least-crowded corner of a theme

```bash
everbee-pp-cli research subniches --parent dad --product apparel --exclude-svg-png --limit 20 --agent
```

Expands the parent into child niches from EverBee's own suggestion engine, drops nothing but flags product type, and normalizes scores so the children are actually comparable.

### Narrow a verbose product payload for an agent

```bash
everbee-pp-cli products search --search-term "dad shirt" --agent --select results.title,results.price,results.listing_type,results.cached_est_mo_revenue
```

Product rows carry 68 fields each; --select trims the payload to the four that drive a decision, keeping agent context small.

### Size up the competition before designing

```bash
everbee-pp-cli research competitors "dad shirt" --agent
```

Reports result count, median price, review and sales density, and listing-age quartiles, with the raw rows printed alongside so the statistics can be checked.

### Track a niche week over week

```bash
everbee-pp-cli research drift "dad shirt" --save-baseline --agent
```

Saves a snapshot to the local store; re-running later diffs against it and reports what actually moved, with both fetch timestamps in provenance.

## Usage

Run `everbee-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `EVERBEE_CONFIG_DIR`, `EVERBEE_DATA_DIR`, `EVERBEE_STATE_DIR`, or `EVERBEE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `EVERBEE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export EVERBEE_HOME=/srv/everbee
everbee-pp-cli doctor
```

Under `EVERBEE_HOME=/srv/everbee`, the four dirs resolve to `/srv/everbee/config`, `/srv/everbee/data`, `/srv/everbee/state`, and `/srv/everbee/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "everbee": {
      "command": "everbee-pp-mcp",
      "env": {
        "EVERBEE_HOME": "/srv/everbee"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `EVERBEE_DATA_DIR` overrides an explicit `--home` for that kind. Use `EVERBEE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `EVERBEE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `everbee-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

EverBee account plan and research quota

- **`everbee-pp-cli account`** - Show the EverBee account's current plan, research quota, and usage. Use this to check remaining keyword-search quota before a batch run — the free Hobby plan allows only 10 keyword searches per month.

### keyword_research

Etsy keyword research — volume, competition, score, CPC, and trend

- **`everbee-pp-cli keyword-research list`** - Browse EverBee's default keyword feed (what the UI shows before you search). This is a discovery/browse surface, NOT a search: it ignores any seed and returns EverBee's unranked default suggestions. To research a specific keyword use `keyword-research search --keyword`.
- **`everbee-pp-cli keyword-research search`** - Seeded keyword search. Returns keywords related to the seed with volume, competition, score, CPC, and trend, plus the seed's own demand-vs-competition metrics under `searched_keyword`. This is the endpoint the EverBee UI search box calls. Comma-join the keyword value to research multiple seeds at once.

### products

Etsy product/listing research — sales, revenue, tags, price, listing type, and age

- **`everbee-pp-cli products`** - Browse EverBee's default product feed (what the UI shows before you search). This is a discovery/browse surface, NOT a search: it ignores any search term. To research a specific product niche use `products search --search-term`.

### shops

Etsy competitor shop research — revenue, sales, listing counts, conversion, and reviews

- **`everbee-pp-cli shops resolve`** - Resolve an Etsy shop handle to its EverBee identity (shop_id, exact shop name, rating, review count, year created). Use this before any shop workflow so an unresolved handle is reported as unresolved rather than as a shop with zero research evidence.
- **`everbee-pp-cli shops search`** - Search EverBee's Etsy shop database. Pass a search term to find a specific competitor shop, or omit it to browse top shops by revenue.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`everbee-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`everbee-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`everbee-pp-cli learnings list`** - Inspect taught rows
- **`everbee-pp-cli learnings forget <query>`** - Undo a teach
- **`everbee-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`everbee-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`everbee-pp-cli teach-pattern`** - Install a query/resource template up front
- **`everbee-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `EVERBEE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `everbee-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
everbee-pp-cli keyword-research list

# JSON for scripting and agents
everbee-pp-cli keyword-research list --json

# Filter to specific fields
everbee-pp-cli keyword-research list --json --select results.title,results.price

# Dry run — show the request without sending
everbee-pp-cli keyword-research list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
everbee-pp-cli keyword-research list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `EVERBEE_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `everbee-pp-cli keyword-research`
- `everbee-pp-cli keyword-research list`
- `everbee-pp-cli keyword-research search`
- `everbee-pp-cli products`
- `everbee-pp-cli products search`
- `everbee-pp-cli products search`
- `everbee-pp-cli shops`
- `everbee-pp-cli shops search`
- `everbee-pp-cli shops search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
everbee-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `everbee-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/everbee-pp-cli/config.toml`; `--home`, `EVERBEE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EVERBEE_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `everbee-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `everbee-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EVERBEE_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every command returns 401 Could not authenticate with the provided credentials.** — The EverBee session token expired. Re-capture it with 'everbee-pp-cli auth setup', or export a fresh EVERBEE_ACCESS_TOKEN.
- **Keyword research returns a typed plan-cap error instead of results.** — The free Hobby plan allows 10 keyword searches per month. Check remaining quota with 'everbee-pp-cli account --json'.
- **Results look unrelated to the seed — mugs and posters for an apparel search.** — You are reading a trending/browse feed, not a search. Use 'research niche' or 'products search --search-term', never the unranked browse feeds ('products' and 'keyword-research list'), to answer a query.
- **A niche verdict reports confidence 0 and zero evidence.** — That is an honest no-evidence result, not a failure. EverBee returned nothing relevant for that seed; widen the seed or lower --min-relevance to inspect the near-misses.
- **research drift reports no baseline.** — Save one first: 'everbee-pp-cli research drift "dad shirt" --save-baseline', then re-run later to see movement.

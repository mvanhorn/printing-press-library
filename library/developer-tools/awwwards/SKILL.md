---
name: pp-awwwards
description: "Query Awwwards jury scores, palettes, and tech stacks from a local SQLite mirror - multi-filter search, trend deltas, and one-shot design briefings the site itself can't do. Trigger phrases: `get design inspiration for a landing page`, `what do award-winning sites look like`, `design context for this brief`, `current web design trends`, `find sites with this color palette`, `use awwwards`, `run awwwards`."
author: ""
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - awwwards-pp-cli
    install:
      - kind: go
        bins: [awwwards-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/cmd/awwwards-pp-cli
---

# Awwwards — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `awwwards-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install awwwards --cli-only
   ```
2. Verify: `awwwards-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/cmd/awwwards-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Awwwards jury-scores the best web design in the world, but that intelligence is locked in server-rendered HTML with one-filter-at-a-time browsing. This CLI mirrors cards, scores, palettes, and tags into local SQLite, then answers questions the site cannot: multi-filter intersections via 'find', trend deltas via 'trends', and one-shot agent design briefings via 'context-pack'.

## When to Use This CLI

Use this CLI whenever an agent or designer needs grounded evidence about what award-winning web design actually looks like: assembling reference sets for a brief, checking current tag/color/tech trends, benchmarking a design direction against jury scores, or pulling palettes and section-level patterns before writing any CSS. It is the right tool when the question starts with "what does great look like for...".

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to submit sites, vote, or manage an Awwwards account - authenticated features are out of scope
- Do not use it to capture full-page screenshots of live sites - it returns Awwwards' own thumbnails and data, not a rendering engine
- Do not use it for bulk archival scraping of awwwards.com - it is an interactive tool with conservative rate limits
- Do not use it for general web search or non-design research; its universe is Awwwards award entries

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Trend and profile analytics
- **`trends`** — Quantify what's rising and falling in award-winning design: tag, color-family, and tech frequency over a time window, with window-over-window deltas.

  _Reach for this when you need current design-language evidence ("is dark mode still dominant?") instead of training-data taste. Prerequisite: run 'awwwards-pp-cli mirror --pages 10' first (add --details for the color axis)._

  ```bash
  awwwards-pp-cli trends --by tag --since 90d --agent
  ```
- **`studio`** — Aggregated award profile of one agency or studio: wins by tier, average dimension scores, and dominant tags and tech.

  _Use to study how a specific top studio wins: their score profile and signature techniques in one call. Prerequisite: credits come from detail pages - run 'awwwards-pp-cli mirror --details' first._

  ```bash
  awwwards-pp-cli studio obys --json
  ```

### Design context for agents
- **`context-pack`** — One-shot design briefing for a build: top-scoring reference sites, dominant palettes, recurring tech, co-occurring style tags, and jury-score benchmarks for a category or style.

  _Run this first when designing a site: it turns "what does great look like for this kind of page" into machine-readable input. Prerequisite: run 'awwwards-pp-cli mirror --pages 10 --details' first; palettes and score benchmarks come from detail data._

  ```bash
  awwwards-pp-cli context-pack --category e-commerce --agent
  ```
- **`palette-match`** — Find award-winning sites whose palette contains a color near a target hex, ranked by RGB distance.

  _Use when a brand color is fixed and you need proof of how top-rated designs deploy colors like it. Prerequisite: palette rows come from detail pages - run 'awwwards-pp-cli mirror --details' first._

  ```bash
  awwwards-pp-cli palette-match "#0F4C81" --distance 25 --json
  ```
- **`elements-top`** — Section-level inspiration ranked by quality: heroes, footers, or 404 pages from sites whose jury score clears your bar.

  _Use before building a specific page section: it returns only the sections from provably high-scoring sites. Prerequisite: run 'awwwards-pp-cli mirror --elements <type> --details' first._

  ```bash
  awwwards-pp-cli elements-top hero --dim design --min 8 --json
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 14 total network entries
- Protocols: html_scrape (55% confidence)
- Auth signals: none

## Command Reference

**design intelligence (hand-built)** — the flagship surface; all but `latest`/`inspect` read the local mirror

- `awwwards-pp-cli mirror` — Feed the local design mirror: listing cards, detail scores/palettes/credits (`--details`), and section screenshots (`--elements hero,footer`)
- `awwwards-pp-cli latest` — The newest award entries as parsed JSON cards (live)
- `awwwards-pp-cli find` — Multi-filter AND search across the mirror (tags, tech, color, award, text, min-score)
- `awwwards-pp-cli top` — Rank mirrored winners by any jury dimension
- `awwwards-pp-cli inspect <slug>` — Full design profile of one site: scores, jury votes, palette, tech, credits (live, cached locally)
- `awwwards-pp-cli trends` — Tag/color/tech frequency over a time window, with deltas
- `awwwards-pp-cli context-pack` — One-shot design briefing (sites + palettes + tech + benchmarks)
- `awwwards-pp-cli palette-match <hex>` — Fuzzy palette search by RGB distance
- `awwwards-pp-cli elements-top <type>` — Section screenshots ranked by parent-site score
- `awwwards-pp-cli studio <name>` — Aggregated award profile of one maker

**collections** — Curated theme boards (dark-mode, hot-right-now, ai-powered-web-projects...)

- `awwwards-pp-cli collections get` — Fetch one curated collection's site grid (owner username + collection slug)
- `awwwards-pp-cli collections list` — List curated collections

**directory** — Agencies and freelancers directory with country/specialty filters

- `awwwards-pp-cli directory browse` — Browse the directory by one filter: specialty (freelance, agency-studio, art-direction, graphic-design, interactive)
- `awwwards-pp-cli directory list` — List top agencies and freelancers

**elements** — Section-level design inspiration (heroes, footers, 404 pages, navigation...)

- `awwwards-pp-cli elements <type>` — Browse tagged screenshots of individual page sections by type: hero, footer, 404_page, about_us, animation, branding

**sites** — Individual award-winning site detail pages (scores, jury notes, palette, tags)

- `awwwards-pp-cli sites content` — Fetch the lightweight content partial for a site (same data, less page chrome)
- `awwwards-pp-cli sites get` — Fetch a site's detail page: overall + per-dimension scores, jury votes, color palette, tags, tech stack

**websites** — Browse award-winning websites (listings with embedded card data)

- `awwwards-pp-cli websites browse` — Browse websites by one filter: award tier (sites_of_the_day, sites_of_the_month, sites_of_the_year, nominees
- `awwwards-pp-cli websites list` — List the latest websites; supports text search and pagination


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
awwwards-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Prime the local design mirror (run once before any analytics)

```bash
awwwards-pp-cli mirror --pages 5 --details --elements hero
```

Mirrors cards, jury scores, palettes, credits, and hero elements into local SQLite - every analytics recipe below reads this mirror.

### Ground an agent before designing a landing page

```bash
awwwards-pp-cli context-pack --category e-commerce --agent
```

Returns top reference sites, dominant palettes, recurring tech, and score benchmarks as one JSON document an agent can design from.

### Check what's trending in award-winning design

```bash
awwwards-pp-cli trends --by tech --since 90d --vs 90d --json
```

Tech frequency this quarter vs last - cite actual counts, not vibes.

### Narrow winners across filters, agent-shaped

```bash
awwwards-pp-cli find --tag dark --tech gsap --agent --select items.slug,items.title,items.tags,items.thumbnail_url
```

Client-side AND-intersection across filters with dotted-path field narrowing so the agent parses only what it needs.

### Steal the palette strategy of a fixed brand color

```bash
awwwards-pp-cli palette-match "#0F4C81" --distance 25 --json
```

Finds winners whose extracted palette contains a near-match, ranked by RGB distance.

### Study only the best heroes before building one

```bash
awwwards-pp-cli elements-top hero --dim design --min 8 --json
```

Section screenshots joined to parent-site jury scores - inspiration filtered by proof of quality.

## Auth Setup

No authentication required.

Run `awwwards-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  awwwards-pp-cli collections list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Generated endpoint commands (`websites`, `sites`, `elements`, `collections`, `directory`) wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. The design-analytics commands (`find`, `top`, `trends`, `context-pack`, `palette-match`, `elements-top`, `studio`, `latest`, `inspect`) emit bare JSON objects instead - their field names are shown in each command's help and recipes (e.g. `find` returns `{items, matched, mirror_total}`). A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `AWWWARDS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `AWWWARDS_CONFIG_DIR`, `AWWWARDS_DATA_DIR`, `AWWWARDS_STATE_DIR`, `AWWWARDS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `AWWWARDS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `data.db` (the local design mirror). `state` contains runtime state. `cache` contains regenerable HTTP/cache files. This CLI has no auth, so no credentials are ever stored.
- Run `awwwards-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "awwwards": {
        "command": "awwwards-pp-mcp",
        "env": {
          "AWWWARDS_HOME": "/srv/awwwards"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `AWWWARDS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `AWWWARDS_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
awwwards-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
awwwards-pp-cli feedback --stdin < notes.txt
awwwards-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `AWWWARDS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AWWWARDS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration.

```
awwwards-pp-cli profile save briefing --json
awwwards-pp-cli --profile briefing collections list
awwwards-pp-cli profile list --json
awwwards-pp-cli profile show briefing
awwwards-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 4 | Permission denied or blocked by the site (HTTP 401/403) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `awwwards-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/cmd/awwwards-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add awwwards-pp-mcp -- awwwards-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which awwwards-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   awwwards-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `awwwards-pp-cli <command> --help`.

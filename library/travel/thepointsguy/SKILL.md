---
name: pp-thepointsguy
description: "The first CLI and local database for The Points Guy: search every article, look up any card's real terms, and turn points valuations into answers you can script. Trigger phrases: `what are my points worth`, `should I use points or cash`, `look up the amex platinum card`, `search the points guy for`, `compare these two credit cards`, `use the points guy`, `run thepointsguy`."
author: "megumikuo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - thepointsguy-pp-cli
    install:
      - kind: go
        bins: [thepointsguy-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/cmd/thepointsguy-pp-cli
---

# The Points Guy — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `thepointsguy-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install thepointsguy --cli-only
   ```
2. Verify: `thepointsguy-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/cmd/thepointsguy-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Points Guy is the reference for points valuations and credit-card terms, but that data is buried in long articles and JS-rendered pages. This CLI mirrors TPG's valuations, card database, and content into a local SQLite store so you can search (live Algolia or offline FTS), value a balance with 'worth', decide points-vs-cash with 'redeem-check', and compare cards with 'cards compare' — all agent-native with --json and --select.

## When to Use This CLI

Use this CLI when an agent or script needs The Points Guy's points valuations, credit-card terms, or article content as structured data: valuing a balance, deciding points vs cash for a booking, comparing cards, or pulling the latest travel-rewards news. It is read-only and needs no account.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to apply for a credit card or access affiliate/apply links.
- Do not use it to track your personal loyalty account balances automatically (use AwardWallet); it works from balances you supply.
- Do not treat its valuations as offers from card issuers; they are TPG editorial estimates.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Points math that compounds
- **`redeem-check`** — Tells you whether to use points or pay cash for a specific booking, using TPG's valuation as the baseline.

  _Reach for this to answer 'points or cash?' for a concrete fare or rate instead of eyeballing a valuation table._

  ```bash
  thepointsguy-pp-cli redeem-check --program "Chase Ultimate Rewards" --points 60000 --cash 900 --agent
  ```
- **`worth`** — Converts a points/miles balance into an estimated dollar value using TPG's monthly valuation.

  _Reach for this to turn a balance into dollars in one call._

  ```bash
  thepointsguy-pp-cli worth --program "American AAdvantage" --points 75000 --agent
  ```
- **`portfolio`** — Values many balances across different programs at once from stdin or a file and totals them.

  _Reach for this to get a single dollar total across every loyalty currency you hold._

  ```bash
  thepointsguy-pp-cli portfolio "Amex Membership Rewards=120000" "United MileagePlus=50000" --agent
  ```

### Local state that compounds
- **`cards compare`** — Compares two or more credit cards across annual fee, APRs, welcome bonus, and rewards.

  _Reach for this to line up two cards' real terms without opening two tabs._

  ```bash
  thepointsguy-pp-cli cards compare chase-sapphire-preferred-card chase-sapphire-reserve --agent
  ```
- **`valuations drift`** — Shows how a program's cents-per-point value changed month over month.

  _Reach for this to see whether a currency is being devalued over time._

  ```bash
  thepointsguy-pp-cli valuations drift --program "Marriott Bonvoy" --months 6 --agent
  ```
- **`since`** — Lists everything TPG published in the last N hours or days across all categories.

  _Reach for this to catch up on new deals and news since you last checked._

  ```bash
  thepointsguy-pp-cli since 24h --agent
  ```

## Command Reference

**articles** — The Points Guy articles and news

- `thepointsguy-pp-cli articles <section> <slug>` — Fetch an article's structured page data by section and slug

**cards** — The Points Guy credit-card database

- `thepointsguy-pp-cli cards <slug>` — Fetch a credit card's structured page data (fees, APRs, welcome bonus) by slug


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
thepointsguy-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Value a balance

```bash
thepointsguy-pp-cli worth --program "American AAdvantage" --points 75000 --agent
```

Turns a mileage balance into an estimated dollar value using TPG's valuation.

### Points or cash?

```bash
thepointsguy-pp-cli redeem-check --program "Chase Ultimate Rewards" --points 60000 --cash 900 --agent
```

Verdict on whether a specific redemption beats paying cash.

### Narrow a big search response

```bash
thepointsguy-pp-cli search "lounge access" --agent --select hits.title,hits.url,hits.category
```

Uses --select on the nested Algolia response so an agent only reads the fields it needs.

### Compare two cards

```bash
thepointsguy-pp-cli cards compare chase-sapphire-preferred-card chase-sapphire-reserve --agent
```

Side-by-side annual fee, APRs, welcome bonus, and rewards from the local mirror.

## Auth Setup

No authentication required.

Run `thepointsguy-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  thepointsguy-pp-cli articles mock-value mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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

- Use `--home <dir>` for one invocation, or set `THEPOINTSGUY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `THEPOINTSGUY_CONFIG_DIR`, `THEPOINTSGUY_DATA_DIR`, `THEPOINTSGUY_STATE_DIR`, `THEPOINTSGUY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `THEPOINTSGUY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `thepointsguy-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "thepointsguy": {
        "command": "thepointsguy-pp-mcp",
        "env": {
          "THEPOINTSGUY_HOME": "/srv/thepointsguy"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `THEPOINTSGUY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `THEPOINTSGUY_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
thepointsguy-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
thepointsguy-pp-cli feedback --stdin < notes.txt
thepointsguy-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `THEPOINTSGUY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `THEPOINTSGUY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
thepointsguy-pp-cli profile save briefing --json
thepointsguy-pp-cli --profile briefing articles mock-value mock-value
thepointsguy-pp-cli profile list --json
thepointsguy-pp-cli profile show briefing
thepointsguy-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `thepointsguy-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/cmd/thepointsguy-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add thepointsguy-pp-mcp -- thepointsguy-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which thepointsguy-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   thepointsguy-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `thepointsguy-pp-cli <command> --help`.

---
name: pp-google-trends
description: "Every Google Trends workflow pytrends offered, plus the local history and rate-limit resilience that archived, frozen wrapper never had. Trigger phrases: `check google trends for`, `interest over time for`, `what's trending right now`, `compare search interest`, `related queries for`, `search interest by region`, `use google-trends`, `run google-trends-pp-cli`."
author: "Kerry Morrison"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-trends-pp-cli
    install:
      - kind: go
        bins: [google-trends-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/google-trends/cmd/google-trends-pp-cli
---

# Google Trends — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-trends-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install google-trends --cli-only
   ```
2. Verify: `google-trends-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-trends/cmd/google-trends-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

pytrends — the de-facto standard Python wrapper — was archived in April 2025, and Google Trends throttles anonymous access aggressively and unpredictably. This CLI absorbs the full explore-to-widget contract with built-in retry/backoff, caches every query locally so history compounds instead of evaporating, and answers questions the live site simply cannot: what was trending three weeks ago, what changed since your last check, and which related terms are actually worth a content brief.

## When to Use This CLI

Use this CLI for programmatic Google Trends research: interest-over-time pulls, multi-keyword and cross-geo comparisons, related/rising query discovery for content planning, and tracking trending topics over time via the local cache. It is the right tool whenever you need history Google Trends' own UI discards, or when you're automating what used to be a manual pytrends script.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for absolute search volume — Google Trends only exposes relative 0-100 interest, never real query counts.
- Do not use this CLI for real-time (sub-hourly) trend monitoring at scale — the API's aggressive rate limiting makes tight polling loops unreliable.
- Do not use this CLI for authenticated Google Ads Keyword Planner-style volume data — that requires a separate Google Ads account and API, not Trends.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`trends trending at`** — See what was trending on a specific past date and geo — a question Google Trends' own live UI cannot answer.

  _Reach for this when reconstructing what was trending on a specific past day instead of scraping the live site, which cannot show historical trending snapshots at all._

  ```bash
  google-trends-pp-cli trends trending at --date 2026-06-15 --geo US --agent
  ```
- **`trends history search`** — Full-text search across every related-term and trending-topic result you've ever synced, offline.

  _Use this to check whether a term you tracked weeks ago ever surfaced as related or trending, without re-querying the rate-limited live API._

  ```bash
  google-trends-pp-cli trends history search "electric vehicle" --table related --agent
  ```
- **`trends changes`** — See exactly what changed for a keyword's related terms and interest score since your last sync.

  _Reach for this instead of re-eyeballing a chart when you need to know precisely what's new since last week._

  ```bash
  google-trends-pp-cli trends changes coffee --since 7d --agent
  ```
- **`trends opportunities`** — Ranks a keyword's related/rising terms by rising-momentum times parent-topic growth, surfacing the best content ideas first.

  _Use this when you need a shortlist of content ideas ranked by momentum, not the raw related-queries dump._

  ```bash
  google-trends-pp-cli trends opportunities "meal prep" --agent
  ```

### Computed insight
- **`trends seasonality`** — Computes monthly averages, peak month, and a seasonality strength score for a keyword from cached history.

  _Reach for this to detect a keyword's seasonal pattern from cached history instead of eyeballing a chart by hand._

  ```bash
  google-trends-pp-cli trends seasonality "pumpkin spice" --geo US --agent
  ```
- **`trends geo-gap`** — Ranks the regions where two keywords' interest diverges most, instead of raw per-region values you'd have to compare by hand.

  _Use this for a ranked list of the biggest regional gaps between two keywords, not the raw per-region compare._

  ```bash
  google-trends-pp-cli trends geo-gap nike adidas --resolution REGION --agent
  ```

### Reachability mitigation
- **`trends stale`** — Lists tracked keywords whose local data hasn't been refreshed recently, so you know what to re-sync before you hit a rate limit.

  _Check this before a bulk re-sync so you refresh only what's actually stale, given this API's confirmed rate-limit fragility._

  ```bash
  google-trends-pp-cli trends stale --older-than 14d --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 29 API entries from 226 total network entries
- Protocols: google_batchexecute (95% confidence), rpc_envelope (90% confidence), rest_json (75% confidence)
- Auth signals: api_key — headers: X-Goog-Api-Key; api_key — query: key, token
- Generation hints: browser_http_transport, has_rpc_envelope, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_GetAsyncData — Derived from observed POST /$rpc/google.internal.onegoogle.asyncdata.v1.AsyncDataService/GetAsyncData traffic.; create_batchexecute — Derived from observed POST /_/TrendsUi/data/batchexecute traffic.; create_browserinfo — Derived from observed POST /_/TrendsUi/browserinfo traffic.; create_explore — Derived from observed POST /trends/api/explore traffic.; create_reload — Derived from observed POST /recaptcha/enterprise/reload traffic.; create_startup_config — Derived from observed POST /v1/survey/startup_config traffic.; create_trigger_anonymous — Derived from observed POST /v1/survey/trigger/trigger_anonymous traffic.; create_ubd — Derived from observed POST /recaptcha/api2/ubd traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**trends** — Core Google Trends explore/widget flow. explore and pickers_* are directly callable; widget_* endpoints require a token minted by a prior explore call (chained by hand-written wrapper commands in Priority 0/1 — see the absorb manifest). All responses are prefixed with Google's XSSI defense string )]}', which must be stripped before JSON decoding.


- `google-trends-pp-cli trends explore` — Requests per-widget tokens for a comparison of up to 5 keywords.
- `google-trends-pp-cli trends pickers-category` — Full category id-to-name lookup tree. Openly accessible, no clearance cookie required.
- `google-trends-pp-cli trends pickers-geo` — Full geo code-to-name lookup tree (country/region/DMA/city). Openly accessible, no clearance cookie required.
- `google-trends-pp-cli trends widget-comparedgeo` — Interest-by-region data for the GEO_MAP widget.
- `google-trends-pp-cli trends widget-multiline` — Interest-over-time data for the TIMESERIES widget.
- `google-trends-pp-cli trends widget-relatedsearches` — Related queries and related topics (both top and rising) for the RELATED_QUERIES/RELATED_TOPICS widgets.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-trends-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Compare a brand against two competitors

```bash
google-trends-pp-cli trends interest nike --compare adidas,puma --geo US --timeframe "today 3-m" --json
```

Returns all three keywords on the same 0-100 scale in one call, the shared-scale comparison pytrends required manual payload construction for.

### Find this week's best content ideas from a topic

```bash
google-trends-pp-cli trends opportunities "home workout" --agent
```

Ranks rising related terms by momentum instead of dumping the raw related-queries list.

### See what changed since last week without re-scraping

```bash
google-trends-pp-cli trends changes coffee --since 7d --agent
```

Diffs the last two local snapshots — a question the live site cannot answer since it has no history.

### Pull a large regional breakdown and narrow to the fields that matter

```bash
google-trends-pp-cli trends region coffee --resolution CITY --geo US --agent --select region_name,value
```

City-level breakdowns return dozens of rows with extra metadata; --select trims the response to just the two fields an agent typically needs.

### Check what was trending on a specific past date

```bash
google-trends-pp-cli trends trending at --date 2026-06-01 --geo US --json
```

Answers a question the live Trending Now page cannot — it only ever shows the current window.

## Auth Setup

Google Trends has no login and no API key. It authenticates via an anonymous NID cookie, but both plain HTTP and Chrome-fingerprinted clients get blocked (HTTP 429) on first contact — confirmed live during generation. Run `google-trends-pp-cli auth login --chrome` once to import a clearance cookie from your logged-in Chrome session; the CLI refreshes it automatically as needed.

Run `google-trends-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-trends-pp-cli trends explore --hl en-US --tz 42 --req '{}' --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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

- Use `--home <dir>` for one invocation, or set `GOOGLE_TRENDS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `GOOGLE_TRENDS_CONFIG_DIR`, `GOOGLE_TRENDS_DATA_DIR`, `GOOGLE_TRENDS_STATE_DIR`, `GOOGLE_TRENDS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `GOOGLE_TRENDS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `google-trends-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "google-trends": {
        "command": "google-trends-pp-mcp",
        "env": {
          "GOOGLE_TRENDS_HOME": "/srv/google-trends"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `GOOGLE_TRENDS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `GOOGLE_TRENDS_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
google-trends-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-trends-pp-cli feedback --stdin < notes.txt
google-trends-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `GOOGLE_TRENDS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_TRENDS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
google-trends-pp-cli profile save briefing --json
google-trends-pp-cli --profile briefing trends explore --hl en-US --tz 42 --req '{}'
google-trends-pp-cli profile list --json
google-trends-pp-cli profile show briefing
google-trends-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `google-trends-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/google-trends/cmd/google-trends-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add google-trends-pp-mcp -- google-trends-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-trends-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-trends-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-trends-pp-cli <command> --help`.

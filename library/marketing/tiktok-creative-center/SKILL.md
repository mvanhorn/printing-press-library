---
name: pp-tiktok-creative-center
description: "TikTok trending hashtags, viral content Trigger phrases: `tiktok trending hashtags`, `tiktok niche research`, `tiktok competitor ads`, `tiktok top ads`, `creative center trends`, `use tiktok creative center`, `run tiktok creative center`."
author: "Jon"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tiktok-creative-center-pp-cli
    install:
      - kind: go
        bins: [tiktok-creative-center-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/tiktok-creative-center/cmd/tiktok-creative-center-pp-cli
---

# TikTok Creative Center — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tiktok-creative-center-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tiktok-creative-center --cli-only
   ```
2. Verify: `tiktok-creative-center-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/tiktok-creative-center/cmd/tiktok-creative-center-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Replays TikTok Creative Center's real endpoints — trending hashtags with popularity curves and audience demographics, representative videos, and the Top Contents (Top Ads) library — and syncs them to SQLite. Then it layers on cross-entity niche briefs, growth-velocity diffing, competitor sweeps, an underserved/viral opportunity score, and a `decide` command that synthesizes all of it into a concrete content + account recommendation. The end goal isn't trends — it's the decision of what to make.

## When to Use This CLI

Use this CLI for TikTok niche research (trending hashtags, their audience demographics, representative creators/videos) and competitor research (the Top Ads / top-performing content library). Ideal when an agent needs structured, offline-queryable trend data or cross-entity reports the web UI can't produce.

## Anti-triggers

Do not use this CLI for:
- Posting or managing TikTok ads (use the official Marketing API)
- Account-level ad performance metrics for your own campaigns
- Downloading video files at scale

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-entity research

- **`niche`** — Get a one-command brief for a niche: trending hashtags, their top creators, and representative videos ranked together.

  _When you need to understand a niche fast instead of clicking through five Creative Center pages._

  ```bash
  tiktok-creative-center-pp-cli niche "marvel rivals" --region US --days 7 --agent
  ```
- **`competitor`** — Summarize a competitor's top-performing content and which trending hashtags they ride.

  _When sizing up a competitor's creative strategy and trend positioning in one pass._

  ```bash
  tiktok-creative-center-pp-cli competitor "myketowersmtyk" --region US --agent
  ```
- **`similar`** — Surface similar or co-rising hashtags to one you specify, using stored industries and shared creators.

  _Expand a winning tag into its cluster of related tags._

  ```bash
  tiktok-creative-center-pp-cli similar "marvelrivalss9" --region US --agent
  ```

### Trend intelligence

- **`velocity`** — Measure which tracked hashtags are accelerating by diffing popularity curves across syncs.

  _Spot rising trends before they peak, not after._

  ```bash
  tiktok-creative-center-pp-cli velocity --region US --top 10 --agent
  ```
- **`since`** — Show hashtags and top content that are newly trending since your last sync.

  _Catch what changed overnight without re-reading the whole feed._

  ```bash
  tiktok-creative-center-pp-cli since 24h --region US --agent
  ```
- **`watch`** — Track hashtags and report which crossed a popularity threshold since the last snapshot.

  _Hands-off monitoring of the tags you care about._

  ```bash
  tiktok-creative-center-pp-cli watch add "gaming" --threshold 80
  ```

### Decision intelligence

- **`viral`** — Rank hashtags and content by opportunity = high popularity + low publish count. Find what's rising but not yet saturated — the signal for what to create before everyone piles in.

  _Tells you WHERE the white space is — the single most useful signal for deciding what to create._

  ```bash
  tiktok-creative-center-pp-cli viral --region US --days 7 --top 20 --agent
  ```
- **`content`** — One ranked feed of trending/viral content pulled from the Top Ads library, representative videos in hashtag detail, and top creators' work — across all three in one list.

  _See what's actually going viral in the niche — not just the tags, the content itself._

  ```bash
  tiktok-creative-center-pp-cli content "marvel rivals" --region US --days 7 --agent
  ```
- **`decide`** — Given a niche, output a concrete recommendation: which hashtags to ride, what content formats are working, where the saturation gaps are, how competitors are positioned, and what account angle is still open.

  _This is the actual goal: decide what content and account to create. Every other command feeds this one._

  ```bash
  tiktok-creative-center-pp-cli decide "marvel rivals" --region US --days 7 --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 783 API entries from 2047 total network entries
- Protocols: google_batchexecute (95% confidence), rpc_envelope (90% confidence), rest_json (75% confidence)
- Auth signals: api_key — query: key, msToken, tcc_key, token
- Generation hints: has_rpc_envelope, weak_schema_confidence
- Candidate command ideas: create_GetHashtagDetail — Derived from observed POST /CreativeOne/KnowledgeAPI/GetHashtagDetail traffic.; create_GetHashtagList — Derived from observed POST /CreativeOne/KnowledgeAPI/GetHashtagList traffic.; create_abtest_config — Derived from observed POST /service/{service_id}/abtest_config/ traffic.; create_act — Derived from observed POST /api/v2/pixel/act traffic.; create_batch — Derived from observed POST /monitor_browser/collect/batch/ traffic.; create_batchexecute — Derived from observed POST /v3/signin/_/AccountsSignInUi/data/batchexecute traffic.; create_browserinfo — Derived from observed POST /v3/signin/_/AccountsSignInUi/browserinfo traffic.; create_check — Derived from observed POST /api/v3/self_serve/feature_gating/check/ traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**hashtags** — Trending hashtags with popularity curves, publish counts, and top creators

- `tiktok-creative-center-pp-cli hashtags detail` — Get a hashtag's popularity curve, audience age/geo profile, and representative videos
- `tiktok-creative-center-pp-cli hashtags list` — List trending hashtags for a country and time range

**top-ads** — Top Contents (Top Ads) library for competitor research

- `tiktok-creative-center-pp-cli top-ads list` — Search the Top Ads / top-performing content library by region, metric, and period
- `tiktok-creative-center-pp-cli top-ads overview` — Get Top Ads filter metadata (industries, objectives, regions)

**trends** — Trends portal metadata and reference data

- `tiktok-creative-center-pp-cli trends` — Trends portal configuration and reference data


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tiktok-creative-center-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### The decision

```bash
tiktok-creative-center-pp-cli decide "marvel rivals" --region US --days 7 --agent
```

The whole point: a concrete recommendation of what content to make and what account angle is open, synthesized from trends + viral scores + competitors.

### Find white space

```bash
tiktok-creative-center-pp-cli viral --region US --days 7 --top 20 --agent
```

Ranks hashtags by opportunity (high popularity, low publish count) so you create before it saturates.

### See what's going viral

```bash
tiktok-creative-center-pp-cli content "marvel rivals" --region US --days 7 --agent
```

One ranked feed of representative + top-ad + creator videos across the niche.

### Size up a competitor

```bash
tiktok-creative-center-pp-cli competitor "myketowersmtyk" --region US --agent
```

Summarize a competitor's top-performing content and the trending hashtags they ride.

### Narrow verbose output

```bash
tiktok-creative-center-pp-cli hashtags list --region US --agent --select items.hashtagName,items.publishCnt,items.rankIndex
```

Use --select with dotted paths to trim large nested responses for agent context.

### Trend velocity

```bash
tiktok-creative-center-pp-cli velocity --region US --top 10 --agent
```

After two syncs, diff popularity curves to find accelerating hashtags.

## Auth Setup

No authentication required.

Run `tiktok-creative-center-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tiktok-creative-center-pp-cli hashtags list --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tiktok-creative-center-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tiktok-creative-center-pp-cli feedback --stdin < notes.txt
tiktok-creative-center-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/tiktok-creative-center-pp-cli/feedback.jsonl`. They are never POSTed unless `TIKTOK_CREATIVE_CENTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TIKTOK_CREATIVE_CENTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tiktok-creative-center-pp-cli profile save briefing --json
tiktok-creative-center-pp-cli --profile briefing hashtags list
tiktok-creative-center-pp-cli profile list --json
tiktok-creative-center-pp-cli profile show briefing
tiktok-creative-center-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `tiktok-creative-center-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/tiktok-creative-center/cmd/tiktok-creative-center-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tiktok-creative-center-pp-mcp -- tiktok-creative-center-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tiktok-creative-center-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tiktok-creative-center-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tiktok-creative-center-pp-cli <command> --help`.

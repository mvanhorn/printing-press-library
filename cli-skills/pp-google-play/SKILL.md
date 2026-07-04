---
name: pp-google-play
description: "Every public Google Play surface in one Go binary Trigger phrases: `top grossing games on google play`, `google play app details for`, `track this app's rank over time`, `what changed on this play store listing`, `reviews for this android app`, `use google-play`, `run google-play`."
author: "Hamza Qazi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-play-pp-cli
    install:
      - kind: go
        bins: [google-play-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/google-play/cmd/google-play-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/google-play/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Google Play — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-play-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install google-play --cli-only
   ```
2. Verify: `google-play-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-play/cmd/google-play-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Pull app details, top charts, search, reviews, similar apps, developer portfolios, permissions, and data safety from the public Play Store with no API key. Then go further than any existing scraper: snapshot charts and listings into a local database so 'movers', 'rank-history', 'keyword-history', and 'watch-listing' can answer week-over-week questions Google never exposes and commercial tools paywall.

## When to Use This CLI

Use this CLI when you need structured, scriptable Google Play Store data without an API key: profiling an app and its competitive set, tracking chart and keyword rank over time, detecting listing changes, or mining reviews. It is built for mobile game and app market intelligence, ASO research, and AI agents that need clean JSON and a typed rate-limit signal.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to manage your own published apps (releases, store listings, replying to reviews) — that needs the Google Play Developer API and a service-account key, not the public store.
- Do not use it for download or revenue estimates, DAU/MAU, retention, or demographics — those come from panels and models, not the public store, and this CLI does not fabricate them.
- Do not point it at the Apple App Store; it only reads Google Play.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`movers`** — See which games climbed, dropped, entered, or fell off a Play chart between two snapshots.

  _Reach for this when an agent needs week-over-week rank movement that Play never exposes and Sensor Tower paywalls._

  ```bash
  google-play-pp-cli movers --collection TOP_GROSSING --category GAME_PUZZLE --country us --agent
  ```
- **`rank-history`** — Show one app's rank trajectory over time within a chart, including first-seen, peak, and last-seen rank.

  _Pick this when you need the trend line for a single title, not the whole chart._

  ```bash
  google-play-pp-cli rank-history com.dreamgames.royalkingdom --collection TOP_GROSSING --category GAME --country us --agent
  ```
- **`watch-listing`** — Diff the two latest snapshots of a listing to surface what changed: title, icon, version, IAP range, ads flag, price, screenshots.

  _Use this to catch a competitor's monetization or positioning shift the moment it ships, instead of eyeballing screenshots._

  ```bash
  google-play-pp-cli watch-listing com.yalla.yallagames --agent
  ```

### ASO rank tracking
- **`keyword-rank`** — Run a live store search for a term and record where a target app ranks, persisting the data point for trend analysis.

  _Reach for this to log today's keyword position so tomorrow's metadata change can be measured._

  ```bash
  google-play-pp-cli keyword-rank "merge puzzle" --country us --app com.yalla.yallagames --agent
  ```
- **`keyword-history`** — Show the rank-over-time series for a term, app, and country from captured keyword snapshots.

  _Use this to prove whether a listing change actually moved a keyword over time._

  ```bash
  google-play-pp-cli keyword-history "merge puzzle" --country us --app com.yalla.yallagames --agent
  ```

### Review intelligence
- **`review-digest`** — Aggregate synced reviews into star and per-version histograms, developer reply rate, and complaint-term frequency, with no NLP.

  _Pick this for mechanical post-update sentiment stats; pipe the output to an LLM if you want prose._

  ```bash
  google-play-pp-cli review-digest com.yalla.yallagames --agent
  ```
- **`compare`** — Fetch details for several apps and lay their key fields side by side in one table.

  _Use this to benchmark a title against its competitive set in a single call._

  ```bash
  google-play-pp-cli compare com.yalla.yallagames com.dreamgames.royalkingdom --agent --select items.appId,items.score,items.installs,items.offersIAP
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 6 API entries from 8 total network entries
- Protocols: ssr_embedded_data (95% confidence), google_batchexecute (95% confidence)
- Generation hints: standard_http: no browser transport or clearance cookie needed in the printed CLI, responses are positional protojson (anonymous nested arrays) under AF_initDataCallback or batchexecute framing; the printed CLI must hand-parse with index paths and fallbacks, not generic JSON field mapping, no auth: all surfaces answer anonymously; rate-limit risk via 429 / 503+captcha / PlayGatewayError-in-200, mitigate with throttle + backoff + caching
- Caveats: : batchexecute responses use the )]}' length-prefixed double-encoded JSON envelope; positional index paths shift on store redesigns roughly once a year

## Command Reference

**categories** — Enumerate Google Play app/game categories from the public store

- `google-play-pp-cli categories` — List Google Play category slugs (GAME_ACTION, GAME_PUZZLE, ...) scraped from the store nav


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-play-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Profile an app for an agent

```bash
google-play-pp-cli app com.dreamgames.royalkingdom --agent --select appId,title,developer,score,realInstalls,offersIAP,containsAds
```

Narrow a verbose detail payload to the fields an agent actually needs.

### This week's grossing movers

```bash
google-play-pp-cli movers --collection TOP_GROSSING --category GAME --country us --agent
```

Diff the two most recent chart snapshots to see who climbed and who fell off.

### Track a keyword over time

```bash
google-play-pp-cli keyword-rank "merge puzzle" --country us --app com.yalla.yallagames && google-play-pp-cli keyword-history "merge puzzle" --country us --app com.yalla.yallagames --agent
```

Capture today's rank, then read the trend series back.

### Narrow a big reviews payload

```bash
google-play-pp-cli reviews com.dreamgames.royalkingdom --limit 100 --agent --select userName,score,text,at
```

Reviews return tens of KB; --select with --agent keeps only the fields you parse.

## Auth Setup

No authentication. Every command reads the public Play Store anonymously. The CLI throttles to about two requests per second by default (set with --rate-limit) and backs off on rate-limit responses, including the PlayGatewayError that Google returns inside an HTTP 200 body.

Run `google-play-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-play-pp-cli categories --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
google-play-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-play-pp-cli feedback --stdin < notes.txt
google-play-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/google-play-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLE_PLAY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_PLAY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
google-play-pp-cli profile save briefing --json
google-play-pp-cli --profile briefing categories
google-play-pp-cli profile list --json
google-play-pp-cli profile show briefing
google-play-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `google-play-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/google-play/cmd/google-play-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add google-play-pp-mcp -- google-play-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-play-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-play-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-play-pp-cli <command> --help`.

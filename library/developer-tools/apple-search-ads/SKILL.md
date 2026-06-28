---
name: pp-apple-search-ads
description: "Every Apple Search Ads feature, plus local cost analytics, bid optimization Trigger phrases: `check my Apple Search Ads campaigns`, `list ASA keywords`, `optimize Apple Ads bids`, `apple search ads report`, `use apple-search-ads`, `run apple search ads`."
author: "Ryan Kelley"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - apple-search-ads-pp-cli
    install:
      - kind: go
        bins: [apple-search-ads-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/apple-search-ads/cmd/apple-search-ads-pp-cli
---

# Apple Search Ads — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `apple-search-ads-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install apple-search-ads --cli-only
   ```
2. Verify: `apple-search-ads-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apple-search-ads/cmd/apple-search-ads-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

apple-search-ads-pp-cli is a native Go CLI covering all campaign, keyword, and reporting operations plus a local SQLite cache for offline analytics. UA teams get bid optimization suggestions, budget pacing forecasts, and cross-org template sync without hitting API rate limits.

## When to Use This CLI

Use this CLI when managing Apple Search Ads campaigns from the command line, automating bid adjustments, analyzing keyword performance at scale, or building UA automation pipelines.

## Anti-triggers

Do not use this CLI for:
- App Store Connect app metadata or screenshots (use App Store Connect CLI or Fastlane)
- TestFlight beta management
- Apple developer account management
- Attribution analytics (use MMP tools like AppsFlyer or Adjust)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`analytics query`** — Query locally cached reporting data without API rate limit friction.

  _Use when you need cross-campaign aggregations or historical trending without burning API quota._

  ```bash
  apple-search-ads-pp-cli analytics query --group-by match_type --limit 20 --agent
  ```
- **`keywords auto-promote`** — Batch analyze high-converting search terms and promote them to keywords with smart match-type routing.

  _Use weekly to surface converting search terms that should become targeted keywords._

  ```bash
  apple-search-ads-pp-cli keywords auto-promote --campaign-id 12345 --min-installs 3 --dry-run
  ```
- **`templates apply`** — Version-control campaign structures and apply them across multiple org IDs with a diff preview.

  _Use when managing multiple apps to ensure consistent campaign structure across all orgs._

  ```bash
  apple-search-ads-pp-cli templates apply brand-baseline --org-ids 111,222 --diff --dry-run
  ```

### Agent-native plumbing
- **`optimize suggest`** — Get CPA/ROAS-driven bid adjustment suggestions with revenue impact forecast before applying.

  _Use before bulk bid changes to preview expected spend delta and install rate impact._

  ```bash
  apple-search-ads-pp-cli optimize suggest --metric cpa --target 2.50 --agent
  ```
- **`campaigns forecast-spend`** — Predict daily/monthly spend and flag campaigns on track to overshoot budget caps.

  _Use at start of month to catch pacing issues before they become wasted budget._

  ```bash
  apple-search-ads-pp-cli campaigns forecast-spend --days 30 --alert-threshold 95 --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**adgroups** — Operations on find

- `apple-search-ads-pp-cli adgroups` — POST /adgroups/find

**ads** — Operations on find

- `apple-search-ads-pp-cli ads` — POST /ads/find

**apps** — Operations on find

- `apple-search-ads-pp-cli apps create-find` — POST /apps/{id}/eligibilities/find
- `apple-search-ads-pp-cli apps create-find-2` — POST /apps/{id}/assets/find
- `apple-search-ads-pp-cli apps get-apps` — GET /apps/{id}
- `apple-search-ads-pp-cli apps get-locale-details` — GET /apps/{id}/locale-details
- `apple-search-ads-pp-cli apps get-locale-details-2` — GET /apps/{id}/product-pages/{id}/locale-details
- `apple-search-ads-pp-cli apps get-product-pages` — GET /apps/{id}/product-pages/{id}
- `apple-search-ads-pp-cli apps get-product-pages-2` — GET /apps/{id}/product-pages

**attribution** — Operations on identify

- `apple-search-ads-pp-cli attribution list-identify` — GET /attribution/device/identify
- `apple-search-ads-pp-cli attribution list-send-attribute` — GET /attribution/device/send-attribute
- `apple-search-ads-pp-cli attribution list-tracking-consent` — GET /attribution/device/tracking-consent

**budgetorders** — Operations on budgetorders

- `apple-search-ads-pp-cli budgetorders create-budgetorders` — POST /budgetorders
- `apple-search-ads-pp-cli budgetorders get-budgetorders` — GET /budgetorders/{id}
- `apple-search-ads-pp-cli budgetorders list-budgetorders` — GET /budgetorders
- `apple-search-ads-pp-cli budgetorders update-budgetorders` — PUT /budgetorders/{id}

**campaigns** — Operations on find

- `apple-search-ads-pp-cli campaigns create-adgroups` — POST /campaigns/{id}/adgroups
- `apple-search-ads-pp-cli campaigns create-ads` — POST /campaigns/{id}/adgroups/{id}/ads
- `apple-search-ads-pp-cli campaigns create-bulk` — POST /campaigns/{id}/adgroups/{id}/targetingkeywords/delete/bulk
- `apple-search-ads-pp-cli campaigns create-bulk-2` — POST /campaigns/{id}/adgroups/{id}/targetingkeywords/bulk
- `apple-search-ads-pp-cli campaigns create-campaigns` — POST /campaigns
- `apple-search-ads-pp-cli campaigns create-find` — POST /campaigns/{id}/adgroups/find
- `apple-search-ads-pp-cli campaigns create-find-2` — POST /campaigns/{id}/ads/find
- `apple-search-ads-pp-cli campaigns create-find-3` — POST /campaigns/find
- `apple-search-ads-pp-cli campaigns create-find-4` — POST /campaigns/{id}/adgroups/targetingkeywords/find
- `apple-search-ads-pp-cli campaigns create-find-5` — POST /campaigns/{id}/adgroups/negativekeywords/find
- `apple-search-ads-pp-cli campaigns create-find-6` — POST /campaigns/{id}/negativekeywords/find
- `apple-search-ads-pp-cli campaigns delete-adgroups` — DELETE /campaigns/{id}/adgroups/{id}
- `apple-search-ads-pp-cli campaigns delete-ads` — DELETE /campaigns/{id}/adgroups/{id}/ads/{id}
- `apple-search-ads-pp-cli campaigns delete-campaigns` — DELETE /campaigns/{id}
- `apple-search-ads-pp-cli campaigns get-adgroups` — GET /campaigns/{id}/adgroups
- `apple-search-ads-pp-cli campaigns get-adgroups-2` — GET /campaigns/{id}/adgroups/{id}
- `apple-search-ads-pp-cli campaigns get-ads` — GET /campaigns/{id}/adgroups/{id}/ads
- `apple-search-ads-pp-cli campaigns get-ads-2` — GET /campaigns/{id}/adgroups/{id}/ads/{id}
- `apple-search-ads-pp-cli campaigns get-campaigns` — GET /campaigns/{id}
- `apple-search-ads-pp-cli campaigns get-targetingkeywords` — GET /campaigns/{id}/adgroups/{id}/targetingkeywords
- `apple-search-ads-pp-cli campaigns get-targetingkeywords-2` — GET /campaigns/{id}/adgroups/{id}/targetingkeywords/{id}
- `apple-search-ads-pp-cli campaigns list-campaigns` — GET /campaigns
- `apple-search-ads-pp-cli campaigns update-adgroups` — PUT /campaigns/{id}/adgroups/{id}
- `apple-search-ads-pp-cli campaigns update-ads` — PUT /campaigns/{id}/adgroups/{id}/ads/{id}
- `apple-search-ads-pp-cli campaigns update-bulk` — PUT /campaigns/{id}/adgroups/{id}/targetingkeywords/bulk
- `apple-search-ads-pp-cli campaigns update-campaigns` — PUT /campaigns/{id}

**countries-or-regions** — Operations on countries-or-regions

- `apple-search-ads-pp-cli countries-or-regions` — GET /countries-or-regions

**creativeappmappings** — Operations on devices

- `apple-search-ads-pp-cli creativeappmappings` — GET /creativeappmappings/devices

**creatives** — Operations on find

- `apple-search-ads-pp-cli creatives create-creatives` — POST /creatives
- `apple-search-ads-pp-cli creatives create-find` — POST /creatives/find
- `apple-search-ads-pp-cli creatives get-creatives` — GET /creatives/{id}
- `apple-search-ads-pp-cli creatives list-creatives` — GET /creatives

**custom-reports** — Operations on custom-reports

- `apple-search-ads-pp-cli custom-reports create-custom-reports` — POST /custom-reports
- `apple-search-ads-pp-cli custom-reports get-custom-reports` — GET /custom-reports/{id}
- `apple-search-ads-pp-cli custom-reports list-custom-reports` — GET /custom-reports

**deferred-deep-link** — Operations on resolve

- `apple-search-ads-pp-cli deferred-deep-link list-resolve` — GET /deferred-deep-link/resolve
- `apple-search-ads-pp-cli deferred-deep-link list-store` — GET /deferred-deep-link/store

**me** — Operations on me

- `apple-search-ads-pp-cli me` — GET /me

**product-page-reasons** — Operations on product-page-reasons

- `apple-search-ads-pp-cli product-page-reasons create-find` — POST /product-page-reasons/find
- `apple-search-ads-pp-cli product-page-reasons get-product-page-reasons` — GET /product-page-reasons/{id}

**reports** — Operations on adgroups

- `apple-search-ads-pp-cli reports create-adgroups` — POST /reports/campaigns/{id}/adgroups
- `apple-search-ads-pp-cli reports create-ads` — POST /reports/campaigns/{id}/ads
- `apple-search-ads-pp-cli reports create-campaigns` — POST /reports/campaigns
- `apple-search-ads-pp-cli reports create-keywords` — POST /reports/campaigns/{id}/keywords
- `apple-search-ads-pp-cli reports create-searchterms` — POST /reports/campaigns/{id}/searchterms

**search_resource** — Operations on apps

- `apple-search-ads-pp-cli search-resource create-geo` — POST /search/geo
- `apple-search-ads-pp-cli search-resource list-apps` — GET /search/apps
- `apple-search-ads-pp-cli search-resource list-geo` — GET /search/geo


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
apple-search-ads-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find underperforming keywords

```bash
apple-search-ads-pp-cli campaigns get-targetingkeywords 12345 --json
```

Surface keywords with low install rate for bid review or pausing

### Promote converting search terms

```bash
apple-search-ads-pp-cli keywords auto-promote --campaign-id 12345 --min-installs 3 --dry-run
```

Preview search terms that qualify for keyword promotion before committing

### Check budget pacing

```bash
apple-search-ads-pp-cli campaigns forecast-spend --days 7 --alert-threshold 90 --agent
```

Surface campaigns projected to exhaust budget before week end

### Apply CPA-optimized bids

```bash
apple-search-ads-pp-cli optimize suggest --metric cpa --target 2.50 --apply --agent
```

Apply CPA-optimized bid suggestions across all active keywords

### Cross-campaign keyword analytics

```bash
apple-search-ads-pp-cli analytics query --group-by keyword_text --limit 20 --agent
```

Query local cache for top-20 keywords by CPA without API calls

## Auth Setup

Apple Search Ads uses OAuth 2.0 with a private key JWT flow. You need a client ID, team ID, key ID, org ID, and a .p8 private key file. Run `apple-search-ads-pp-cli auth setup` to configure these, or set ASA_CLIENT_ID, ASA_TEAM_ID, ASA_KEY_ID, ASA_ORG_ID, and ASA_PRIVATE_KEY_PATH in your environment.

Run `apple-search-ads-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  apple-search-ads-pp-cli adgroups --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
apple-search-ads-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
apple-search-ads-pp-cli feedback --stdin < notes.txt
apple-search-ads-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/apple-search-ads-pp-cli/feedback.jsonl`. They are never POSTed unless `APPLE_SEARCH_ADS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `APPLE_SEARCH_ADS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
apple-search-ads-pp-cli profile save briefing --json
apple-search-ads-pp-cli --profile briefing adgroups
apple-search-ads-pp-cli profile list --json
apple-search-ads-pp-cli profile show briefing
apple-search-ads-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `apple-search-ads-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/apple-search-ads/cmd/apple-search-ads-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add apple-search-ads-pp-mcp -- apple-search-ads-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which apple-search-ads-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   apple-search-ads-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `apple-search-ads-pp-cli <command> --help`.

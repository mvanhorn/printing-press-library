---
name: pp-appsflyer
description: "Ad-hoc pulls from AppsFlyer's V2 API, rate-budget aware: one facade command for any breakdown, a morning standup... Trigger phrases: `appsflyer standup`, `morning ROAS across my apps`, `appsflyer pull`, `fetch appsflyer data for last week`, `appsflyer raw events`, `use appsflyer`, `run appsflyer-pp-cli`."
author: "Matt Latimer"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - appsflyer-pp-cli
---

# AppsFlyer — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `appsflyer-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install appsflyer --cli-only
   ```
2. Verify: `appsflyer-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Two power commands on top of the V2 surface. `pull` gives you a single facade over the Aggregate Pull V2 reports with friendly source names and channel-group rollups. `standup` pivots yesterday / WTD / MTD across all apps in your account under the Pull API daily budget. Built for analysts who want fast answers without thinking about which AppsFlyer endpoint to call. (For Master, Cohort, SKAN, or Raw Data pulls, use the typed `master`, `cohort`, `skan`, and `raw` subcommands directly.)

## When to Use This CLI

Reach for appsflyer-pp-cli when you want a fast answer to a specific paid-acquisition question across one or more of your AppsFlyer apps. The CLI is built for ad-hoc pulls and a daily standup, not full data warehousing — pair it with Singer/Airbyte/Fivetran if you need nightly EL.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Morning workflow
- **`standup`** — Cross-app pivot showing yesterday vs week-to-date vs month-to-date ROAS, spend, and installs — optionally grouped by channel-group (social, programmatic, OEM, rewarded).

  _Run this every morning to see how the portfolio's paid spend is performing without scrolling N dashboards._

  ```bash
  appsflyer-pp-cli standup --app-id id123456 --app-id id654321 --json
  ```

### Ad-hoc workflow
- **`pull`** — One command, rich flags. Specify date range, media source (canonical or friendly), channel group, campaign, breakdown, metrics, currency, and timezone — the CLI routes to the right underlying AppsFlyer endpoint and applies friendly-name resolution.

  _Use this when you have a specific ad-hoc question (e.g. 'how did Facebook campaigns perform last week by campaign?') without thinking about which AppsFlyer endpoint to call._

  ```bash
  appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --source facebook --breakdown campaign --json
  ```

## Command Reference

**agg** — Aggregate Pull API V2 — campaign-day-source breakdowns

- `appsflyer-pp-cli agg daily` — Daily report — aggregate across all media sources by day
- `appsflyer-pp-cli agg geo` — Geo report — country breakdown
- `appsflyer-pp-cli agg geo_by_date` — Geo-by-date — country breakdown with daily granularity
- `appsflyer-pp-cli agg partners` — Partners report — media-source breakdown for date range
- `appsflyer-pp-cli agg partners_by_date` — Partners-by-date — daily granularity per media source

**apps** — AppsFlyer apps in this account

- `appsflyer-pp-cli apps` — List apps registered to the account

**cohort** — Cohort API V1 — D1/D3/D7/D30/D90 retention + LTV by cohort

- `appsflyer-pp-cli cohort` — Cohort retention + LTV. Body controls cohort length, KPIs, groupings, partial_data.

**master** — Master API V2 — combined dimensions in one call

- `appsflyer-pp-cli master` — Master API V2 — combined groupings + KPIs per request

**raw** — Raw Data Pull API V2 — per-install / per-event CSV exports

- `appsflyer-pp-cli raw in_app_events` — Per-event raw report (CSV)
- `appsflyer-pp-cli raw installs` — Per-install raw report (CSV)
- `appsflyer-pp-cli raw organic_in_app_events` — Organic per-event raw report (CSV)
- `appsflyer-pp-cli raw organic_installs` — Per-install organic raw report (CSV)
- `appsflyer-pp-cli raw uninstalls` — Per-uninstall raw report (CSV)

**skan** — SKAdNetwork API V1 — aggregate SKAN data, install-date and postback-arrival

- `appsflyer-pp-cli skan` — SKAN aggregated install-date report. Note: SKAN data lags ~2 days.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
appsflyer-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning standup, agent-friendly subset

```bash
appsflyer-pp-cli standup --app-id id123456 --app-id id654321 --agent --select apps[].app_id,apps[].yesterday.roas,apps[].wtd.roas,apps[].mtd.roas
```

Pull the cross-app standup scoped to just the ROAS fields a triage agent needs across three time windows.

### Last-week campaign performance for one channel

```bash
appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --source facebook --breakdown campaign --metrics installs,revenue,roas --json
```

Ad-hoc pull with friendly source resolution (facebook → facebook_int).

### Channel-group rollup across the portfolio

```bash
appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --channel-group social --breakdown media_source --metrics installs,revenue --json
```

Roll up Meta + TikTok + Snap + Reddit + Pinterest + X into one social group using the channels.yaml mapping.

### Doctor with full diagnostic JSON

```bash
appsflyer-pp-cli doctor --json
appsflyer-pp-cli doctor --json --probe-families   # consumes one budget call per family
```

The default returns dotenv path, token fingerprint, and remaining Pull API budget. Adding `--probe-families` actively probes each report family and surfaces 200/401/403 per family (one API call each — opt-in so doctor doesn't silently burn budget).

### Resolve a friendly source name to canonical

```bash
appsflyer-pp-cli sources resolve tiktok
```

Maps TikTok → tiktokglobal_int so you can paste the canonical ID into other tools.

## Auth Setup

AppsFlyer V2 uses a single account-level Bearer token (Security Center → AppsFlyer API tokens). The CLI loads APPSFLYER_API_TOKEN from ~/.config/appsflyer-pp-cli/.env via joho/godotenv; process env wins over the file. Token scopes (Master, Cohort, SKAN, Raw Data) depend on your subscription — appsflyer-pp-cli doctor probes each family. The CLI also tracks your Pull API daily-call budget (configurable, default 20) so you never blow the cap mid-day.

Run `appsflyer-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  appsflyer-pp-cli apps --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
appsflyer-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
appsflyer-pp-cli feedback --stdin < notes.txt
appsflyer-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.appsflyer-pp-cli/feedback.jsonl`. They are never POSTed unless `APPSFLYER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `APPSFLYER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
appsflyer-pp-cli profile save briefing --json
appsflyer-pp-cli --profile briefing apps
appsflyer-pp-cli profile list --json
appsflyer-pp-cli profile show briefing
appsflyer-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `appsflyer-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add appsflyer-pp-mcp -- appsflyer-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which appsflyer-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   appsflyer-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `appsflyer-pp-cli <command> --help`.

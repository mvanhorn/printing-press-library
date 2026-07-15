---
name: pp-oura
description: "Printing Press CLI for Oura. The Oura API allows Oura users and partner applications to improve their user experience with Oura data."
author: "ryanc00per"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - oura-pp-cli
    install:
      - kind: go
        bins: [oura-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/oura/cmd/oura-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/health/oura/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Oura — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `oura-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install oura --cli-only
   ```
2. Verify: `oura-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/health/oura/cmd/oura-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

# Overview 
The Oura API allows Oura users and partner applications to improve their user experience with Oura data.
This document describes the Oura API Version 2 (V2), which is the only available integration point for Oura data. The previous V1 API has been sunset.
# Getting Started 
## What is an API?
An API (Application Programming Interface) allows different software applications to communicate with each other. The Oura API enables you to access your Oura Ring data programmatically.
## Quick Start Guide
1. Register an [API Application](https://cloud.ouraring.com/oauth/applications) and implement OAuth2
2. **Make Your First API Call**:
   ```
   curl -X GET https://api.ouraring.com/v2/usercollection/personal_info \
   -H "Authorization: Bearer YOUR_TOKEN_HERE"
   ```
3. **Explore Data Types**:
   - Browse the available endpoints in this documentation to discover what data you can access
   - Each endpoint includes example requests and responses
4. **Set Up Webhooks (Strongly Recommended)**:
   - Webhooks are the preferred way to consume Oura data
   - We have not had customers hit rate limits with webhooks properly implemented
   - Make a single request for historical data when a user first connects, then use webhooks for ongoing updates
   - Webhook notifications come approximately 30 seconds after data syncs from the mobile app
   - [Set up webhooks](#tag/Webhook-Subscription-Routes) to receive notifications when data changes
## Common Questions
- **Data Delay**: Different data types sync at different times - sleep data requires users to open the Oura app, while daily activity and stress may sync in the background
# Data Access
In order to access data, a registered [API Application](https://cloud.ouraring.com/oauth/applications) is required.
 API Applications are limited to **10** users before requiring approval from Oura. There is no limit once an application is approved.
 Additionally, Oura users **must provide consent** to share each data type an API Application has access to.
All data access requests through the Oura API require [Authentication](https://cloud.ouraring.com/docs/authentication).
Additionally, we recommend that Oura users keep their mobile app updated to support API access for the latest data types.
# Authentication
The Oura Cloud API supports authentication through the industry-standard OAuth2 protocol. For more information, see our [Authentication instructions](https://cloud.ouraring.com/docs/authentication).
Access tokens must be included in the request header as follows:
```http
GET /v2/usercollection/personal_info HTTP/1.1
Host: api.ouraring.com
Authorization: Bearer <token>
```
Please note that personal access tokens were deprecated in December 2025 and are no longer available for use.
# Oura HTTP Response Codes
| Response Code                        | Description |
| ------------------------------------ | - |
| 200 OK                               | Successful Response         |
| 400 Query Parameter Validation Error | The request contains query parameters that are invalid or incorrectly formatted. |
| 401 Unauthorized                     | Invalid or expired authentication token. |
| 403 Forbidden                        | The requested resource requires additional permissions or the user's Oura subscription has expired. |
| 429 Too Many Requests                | Rate limit exceeded. See response headers for retry guidance. |

## Rate Limits
The API enforces rate limits at two layers to ensure fair access across all applications:
- a per-access-token limit, which throttles single-token floods, and
- a per-application limit, which caps the aggregate traffic across all of an application's end-user tokens so one fan-out app can't dominate shared capacity.

A request that trips either layer receives a `429 Too Many Requests`. The `X-RateLimit-Tier` response header identifies which layer fired.

If your application regularly approaches rate limits, [webhooks](#tag/Webhook-Subscription-Routes) are strongly recommended — most applications that implement webhooks correctly do not encounter rate limit issues.

## Rate Limit Response Headers
When a `429 Too Many Requests` response is returned, five headers are included to guide retries. Prefer these over fixed-interval backoff:
- **`Retry-After`** — integer seconds to wait before retrying. RFC 7231-compliant; safe to feed directly into your client's backoff logic.
- **`X-RateLimit-Limit`** — the request ceiling for the current window.
- **`X-RateLimit-Window`** — the rolling window length in seconds that the ceiling applies to.
- **`X-RateLimit-Reset`** — Unix epoch (seconds) at which the window resets and quota is fully restored.
- **`X-RateLimit-Tier`** — identifies which limit was exceeded, useful when contacting support.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`sync`** — Mirror Oura usercollection data into a local SQLite store for offline search and analysis
- **`search`** — FTS5 full-text search across locally synced Oura records
- **`analytics`** — Aggregate group-by and count queries over locally synced health data
- **`workflow`** — Chain multiple Oura API operations into one agent-friendly command

## Recipes

### 

```bash
oura-pp-cli usercollection multiple-heartrate-documents --json
```

### 

```bash
oura-pp-cli usercollection multiple-workout-documents --json
```

### 

```bash
oura-pp-cli usercollection single-personal-info-document --json
```

## Command Reference

**sandbox** — Manage sandbox

- `oura-pp-cli sandbox multiple-daily-activity-documents` — Sandbox - Multiple Daily Activity Documents
- `oura-pp-cli sandbox multiple-daily-cardiovascular-age-documents` — Sandbox - Multiple Daily Cardiovascular Age Documents
- `oura-pp-cli sandbox multiple-daily-readiness-documents` — Sandbox - Multiple Daily Readiness Documents
- `oura-pp-cli sandbox multiple-daily-resilience-documents` — Sandbox - Multiple Daily Resilience Documents
- `oura-pp-cli sandbox multiple-daily-sleep-documents` — Sandbox - Multiple Daily Sleep Documents
- `oura-pp-cli sandbox multiple-daily-spo2-documents` — Sandbox - Multiple Daily Spo2 Documents
- `oura-pp-cli sandbox multiple-daily-stress-documents` — Sandbox - Multiple Daily Stress Documents
- `oura-pp-cli sandbox multiple-enhanced-tag-documents` — Sandbox - Multiple Enhanced Tag Documents
- `oura-pp-cli sandbox multiple-heartrate-documents` — Sandbox - Multiple Heartrate Documents
- `oura-pp-cli sandbox multiple-rest-mode-period-documents` — Sandbox - Multiple Rest Mode Period Documents
- `oura-pp-cli sandbox multiple-ring-battery-level-documents` — Sandbox - Multiple Ring Battery Level Documents
- `oura-pp-cli sandbox multiple-ring-configuration-documents` — Sandbox - Multiple Ring Configuration Documents
- `oura-pp-cli sandbox multiple-session-documents` — Sandbox - Multiple Session Documents
- `oura-pp-cli sandbox multiple-sleep-documents` — Sandbox - Multiple Sleep Documents
- `oura-pp-cli sandbox multiple-sleep-time-documents` — Sandbox - Multiple Sleep Time Documents
- `oura-pp-cli sandbox multiple-tag-documents` — Sandbox - Multiple Tag Documents
- `oura-pp-cli sandbox multiple-v-o2-max-documents` — Sandbox - Multiple Vo2 Max Documents
- `oura-pp-cli sandbox multiple-workout-documents` — Sandbox - Multiple Workout Documents
- `oura-pp-cli sandbox single-daily-activity-document` — Sandbox - Single Daily Activity Document
- `oura-pp-cli sandbox single-daily-cardiovascular-age-document` — Sandbox - Single Daily Cardiovascular Age Document
- `oura-pp-cli sandbox single-daily-readiness-document` — Sandbox - Single Daily Readiness Document
- `oura-pp-cli sandbox single-daily-resilience-document` — Sandbox - Single Daily Resilience Document
- `oura-pp-cli sandbox single-daily-sleep-document` — Sandbox - Single Daily Sleep Document
- `oura-pp-cli sandbox single-daily-spo2-document` — Sandbox - Single Daily Spo2 Document
- `oura-pp-cli sandbox single-daily-stress-document` — Sandbox - Single Daily Stress Document
- `oura-pp-cli sandbox single-enhanced-tag-document` — Sandbox - Single Enhanced Tag Document
- `oura-pp-cli sandbox single-rest-mode-period-document` — Sandbox - Single Rest Mode Period Document
- `oura-pp-cli sandbox single-ring-configuration-document` — Sandbox - Single Ring Configuration Document
- `oura-pp-cli sandbox single-session-document` — Sandbox - Single Session Document
- `oura-pp-cli sandbox single-sleep-document` — Sandbox - Single Sleep Document
- `oura-pp-cli sandbox single-sleep-time-document` — Sandbox - Single Sleep Time Document
- `oura-pp-cli sandbox single-tag-document` — Sandbox - Single Tag Document
- `oura-pp-cli sandbox single-v-o2-max-document` — Sandbox - Single Vo2 Max Document
- `oura-pp-cli sandbox single-workout-document` — Sandbox - Single Workout Document

**usercollection** — Manage usercollection

- `oura-pp-cli usercollection multiple-daily-activity-documents` — Multiple Daily Activity Documents
- `oura-pp-cli usercollection multiple-daily-cardiovascular-age-documents` — Multiple Daily Cardiovascular Age Documents
- `oura-pp-cli usercollection multiple-daily-readiness-documents` — Multiple Daily Readiness Documents
- `oura-pp-cli usercollection multiple-daily-resilience-documents` — Multiple Daily Resilience Documents
- `oura-pp-cli usercollection multiple-daily-sleep-documents` — Multiple Daily Sleep Documents
- `oura-pp-cli usercollection multiple-daily-spo2-documents` — Multiple Daily Spo2 Documents
- `oura-pp-cli usercollection multiple-daily-stress-documents` — Multiple Daily Stress Documents
- `oura-pp-cli usercollection multiple-enhanced-tag-documents` — Multiple Enhanced Tag Documents
- `oura-pp-cli usercollection multiple-heartrate-documents` — Multiple Heartrate Documents
- `oura-pp-cli usercollection multiple-rest-mode-period-documents` — Multiple Rest Mode Period Documents
- `oura-pp-cli usercollection multiple-ring-battery-level-documents` — Multiple Ring Battery Level Documents
- `oura-pp-cli usercollection multiple-ring-configuration-documents` — Multiple Ring Configuration Documents
- `oura-pp-cli usercollection multiple-session-documents` — Multiple Session Documents
- `oura-pp-cli usercollection multiple-sleep-documents` — Multiple Sleep Documents
- `oura-pp-cli usercollection multiple-sleep-time-documents` — Multiple Sleep Time Documents
- `oura-pp-cli usercollection multiple-tag-documents` — Multiple Tag Documents
- `oura-pp-cli usercollection multiple-v-o2-max-documents` — Multiple Vo2 Max Documents
- `oura-pp-cli usercollection multiple-workout-documents` — Multiple Workout Documents
- `oura-pp-cli usercollection single-daily-activity-document` — Single Daily Activity Document
- `oura-pp-cli usercollection single-daily-cardiovascular-age-document` — Single Daily Cardiovascular Age Document
- `oura-pp-cli usercollection single-daily-readiness-document` — Single Daily Readiness Document
- `oura-pp-cli usercollection single-daily-resilience-document` — Single Daily Resilience Document
- `oura-pp-cli usercollection single-daily-sleep-document` — Single Daily Sleep Document
- `oura-pp-cli usercollection single-daily-spo2-document` — Single Daily Spo2 Document
- `oura-pp-cli usercollection single-daily-stress-document` — Single Daily Stress Document
- `oura-pp-cli usercollection single-enhanced-tag-document` — Single Enhanced Tag Document
- `oura-pp-cli usercollection single-personal-info-document` — Single Personal Info Document
- `oura-pp-cli usercollection single-rest-mode-period-document` — Single Rest Mode Period Document
- `oura-pp-cli usercollection single-ring-configuration-document` — Single Ring Configuration Document
- `oura-pp-cli usercollection single-session-document` — Single Session Document
- `oura-pp-cli usercollection single-sleep-document` — Single Sleep Document
- `oura-pp-cli usercollection single-sleep-time-document` — Single Sleep Time Document
- `oura-pp-cli usercollection single-tag-document` — Single Tag Document
- `oura-pp-cli usercollection single-v-o2-max-document` — Single Vo2 Max Document
- `oura-pp-cli usercollection single-workout-document` — Single Workout Document

**webhook** — Manage webhook

- `oura-pp-cli webhook create` — Create Webhook Subscription
- `oura-pp-cli webhook delete` — Delete Webhook Subscription
- `oura-pp-cli webhook get-subscription` — Get Webhook Subscription
- `oura-pp-cli webhook list-subscriptions` — List Webhook Subscriptions
- `oura-pp-cli webhook renew-subscription` — Renew Webhook Subscription
- `oura-pp-cli webhook update` — Update Webhook Subscription


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
oura-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `oura-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
oura-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `OURA_OAUTH2` as an environment variable.

Run `oura-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  oura-pp-cli sandbox multiple-daily-activity-documents --agent --select id,name,status
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
oura-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
oura-pp-cli feedback --stdin < notes.txt
oura-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/oura-pp-cli/feedback.jsonl`. They are never POSTed unless `OURA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OURA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
oura-pp-cli profile save briefing --json
oura-pp-cli --profile briefing sandbox multiple-daily-activity-documents
oura-pp-cli profile list --json
oura-pp-cli profile show briefing
oura-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `oura-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/health/oura/cmd/oura-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add oura-pp-mcp -- oura-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which oura-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   oura-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `oura-pp-cli <command> --help`.

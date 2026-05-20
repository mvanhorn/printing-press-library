---
name: pp-posthog
description: "Every PostHog resource in one CLI — with offline search, agent-native output, and cross-resource analytics no... Trigger phrases: `check my PostHog feature flags`, `query PostHog events`, `show experiment results in PostHog`, `what errors are spiking in PostHog`, `LLM costs in PostHog`, `is it safe to ramp this flag`, `use posthog`."
author: "riteshtiwari"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - posthog-pp-cli
---

# PostHog — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `posthog-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install posthog --cli-only
   ```
2. Verify: `posthog-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/posthog/cmd/posthog-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

posthog-pp-cli sync your flags, insights, experiments, persons, errors, and LLM traces to a local SQLite store. Query anything offline, run compound analytics across resources the UI keeps separate, and pipe results directly to agents or scripts.

## When to Use This CLI

Use posthog-pp-cli when an agent task requires querying, diffing, or combining PostHog data across more than one resource type — flag state vs error events, LLM costs vs variant performance, experiment exposure vs significance. It is faster than the web UI for bulk reads and produces structured JSON that agents can consume directly without parsing HTML.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Flag safety
- **`flags blast-radius`** — Find every insight, dashboard, experiment, and survey that references a flag before you archive or rename it.

  _Use before archiving or renaming a flag to prevent breaking dashboards and experiments silently._

  ```bash
  posthog-pp-cli flags blast-radius --key my-checkout-v2 --json
  ```
- **`flags rollout-health`** — Go/no-go confidence for a flag ramp — error rate and key metric movement correlated with flag exposure.

  _Use before ramping a flag to 100% to catch regressions that aren't visible in overall metrics._

  ```bash
  posthog-pp-cli flags rollout-health --key new-checkout --window 24h --agent
  ```
- **`flags stale`** — List flags that haven't been evaluated in N days — cleanup candidates before they accumulate.

  _Use in quarterly flag cleanup sprints to identify dead code paths safely._

  ```bash
  posthog-pp-cli flags stale --days 30 --json
  ```

### LLM observability
- **`llm cost-attribution`** — Break down LLM spend by feature flag variant — see whether the expensive model variant pays for itself.

  _Use when evaluating whether to promote a more expensive LLM variant based on actual cost-per-conversion._

  ```bash
  posthog-pp-cli llm cost-attribution --flag model-tier --agent
  ```

### Local state that compounds
- **`persons at-risk`** — Surface which users in a cohort are going quiet and recently hit errors — before they churn.

  _Use in weekly retention reviews to prioritize proactive outreach before churn events._

  ```bash
  posthog-pp-cli persons at-risk --cohort paying-users --silent-days 14 --json
  ```
- **`events property-drift`** — Catch tracking regressions — properties that silently disappeared from an event between two time windows.

  _Use after a deploy to catch silent schema changes that corrupt ongoing experiments and dashboards._

  ```bash
  posthog-pp-cli events property-drift checkout_completed --agent
  ```
- **`experiments pre-check`** — Know today whether an experiment will reach significance this sprint, or needs traffic adjustment now.

  _Use at the start of each sprint to surface experiments that need traffic changes before they run out of time._

  ```bash
  posthog-pp-cli experiments pre-check --agent
  ```
- **`dashboard health`** — Find broken dashboards before a stakeholder meeting does — stale data, deleted cohorts, archived flags.

  _Use before weekly business reviews to catch broken insight tiles that would embarrass the data team._

  ```bash
  posthog-pp-cli dashboard health --stale-days 7 --agent
  ```

## Command Reference

**projects** — Manage projects


**public-hog-function-templates** — Manage public hog function templates

- `posthog-pp-cli public-hog-function-templates` — List

**user-home-settings** — Manage user home settings

- `posthog-pp-cli user-home-settings partial-update` — Update the authenticated user's pinned sidebar tabs and/or homepage for the current team. Pass `@me` as the UUID....
- `posthog-pp-cli user-home-settings retrieve` — Get the authenticated user's pinned sidebar tabs and configured homepage for the current team. Pass `@me` as the UUID.

**users** — Manage users

- `posthog-pp-cli users cancel-email-change-request-partial-update` — Cancel email change request partial update
- `posthog-pp-cli users destroy` — Destroy
- `posthog-pp-cli users list` — List
- `posthog-pp-cli users partial-update` — Update one or more of the authenticated user's profile fields or settings.
- `posthog-pp-cli users request-email-verification-create` — Request email verification create
- `posthog-pp-cli users retrieve` — Retrieve a user's profile and settings. Pass `@me` as the UUID to fetch the authenticated user; non-staff callers...
- `posthog-pp-cli users update` — Replace the authenticated user's profile and settings. Pass `@me` as the UUID to update the authenticated user....
- `posthog-pp-cli users verify-email-create` — Verify email create


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
posthog-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find flags safe to archive

```bash
posthog-pp-cli flags stale --days 60 --json | jq '.[].key'
```

List flag keys with no evaluation events in 60 days — safe cleanup candidates

### Pre-ramp safety check

```bash
posthog-pp-cli flags rollout-health --key new-checkout --window 48h --agent --select flag_key,error_rate_delta,metric_delta
```

Check error rate and purchase metric delta for flag-exposed users before ramping to 100%

### Weekly experiment briefing

```bash
posthog-pp-cli experiments pre-check --json | jq '.[] | {key, winner, significance, days_remaining}'
```

Cross-experiment pre-check summary for Monday standup — surface experiments needing traffic adjustment

### LLM cost by flag variant

```bash
posthog-pp-cli llm cost-attribution --flag model-tier --days 30 --agent --select variant,total_cost_usd,avg_cost_per_call
```

Compare LLM spend across A/B variants to decide if GPT-4o outperforms GPT-4o-mini in your context

### Dashboard audit before all-hands

```bash
posthog-pp-cli dashboard health --stale-days 7 --json | jq '.[] | select(.issues | length > 0)'
```

Surface dashboards with broken filters or stale data before a stakeholder presentation

## Auth Setup

Uses your PostHog personal API key (phx_...). Set POSTHOG_API_KEY or run `posthog-pp-cli auth set-token`. Supports both US (app.posthog.com) and EU (eu.posthog.com) instances via POSTHOG_HOST.

Run `posthog-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  posthog-pp-cli flags stale --agent --select key,days_stale
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
posthog-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
posthog-pp-cli feedback --stdin < notes.txt
posthog-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.posthog-pp-cli/feedback.jsonl`. They are never POSTed unless `POSTHOG_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `POSTHOG_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
posthog-pp-cli profile save briefing --json
posthog-pp-cli --profile briefing flags stale
posthog-pp-cli profile list --json
posthog-pp-cli profile show briefing
posthog-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `posthog-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add posthog-pp-mcp -- posthog-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which posthog-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   posthog-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `posthog-pp-cli <command> --help`.

---
name: pp-whoop
description: "Printing Press CLI for WHOOP. Official WHOOP Developer Platform v2 API. Provides programmatic access to a user's cycles, sleep, recovery,... Trigger phrases: `my sleep score`, `strain today`, `weekly recovery trend`, `whoop analytics`, `use whoop`, `whoop sleep debt`, `whoop overtraining`, `correlate my whoop`, `why was my recovery`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - whoop-pp-cli
---

# WHOOP — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `whoop-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install whoop --cli-only
   ```
2. Verify: `whoop-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/whoop/cmd/whoop-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

WHOOP gives you a number every morning. whoop-pp-cli gives you the *why*. It syncs your full WHOOP history into a local SQLite database, runs cross-resource joins (sleep ⋈ recovery ⋈ workouts ⋈ cycles), and surfaces trends, correlations, and overtraining alerts no live API call can compute. Everything is agent-native: JSON output, --select field filtering, --agent mode for Claude Code, plus an MCP server for Claude Desktop.

## When to Use This CLI

Use whoop-pp-cli when you want to ask questions about your own WHOOP data that the WHOOP mobile app cannot answer — multi-week trends, correlations between metrics, custom SQL queries, overtraining detection, or sleep-debt tracking. Especially good with Claude Code: install the pp-whoop skill and ask in natural language ("how was my recovery this week vs last", "correlate my sleep consistency with morning recovery over 90 days", "flag overtraining"). Skip it if you just want today's score — the WHOOP app is fine for that.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`analyze efficiency`** — Buckets cycles by strain (0-5, 5-10, 10-15, 15-21) and shows mean recovery per bucket vs. the prior equivalent window.
- **`analyze sleep-debt`** — Cumulative sum of need_from_sleep_debt_milli weekly, with trend slope and human-friendly interpretation.
- **`analyze overtraining`** — Flags days where strain exceeds N sigma above the 90-day mean and shows the recovery delta vs. window mean.
- **`analyze correlate`** — Pearson correlation between any two whitelisted WHOOP metrics over a chosen window.
- **`analyze why-today`** — Ranks today's recovery, HRV, RHR, sleep consistency, and prior-day strain by abs(z-score) vs. personal 14-day baseline.
- **`sql`** — Execute read-only SELECT/WITH queries (or read-only PRAGMAs like table_info, table_list, foreign_key_list for schema introspection) against the local SQLite store. Accept the query as a positional arg or via --query.
- **`search`** — FTS5 full-text search across all synced resources (cycle, sleep, recovery, workouts).

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**activity** — Manage activity

- `whoop-pp-cli activity get-sleep` — Get a single sleep by UUID
- `whoop-pp-cli activity get-workout` — Get a single workout by UUID
- `whoop-pp-cli activity list-sleeps` — List sleep activities
- `whoop-pp-cli activity list-workouts` — List workouts

**activity-mapping** — V1 -> V2 identifier mapping helper.

- `whoop-pp-cli activity-mapping <v1Id>` — Translate a v1 sleep/workout id to a v2 UUID

**cycle** — Physiological cycles (a WHOOP "day").

- `whoop-pp-cli cycle get` — Get a single cycle by id
- `whoop-pp-cli cycle list` — Returns the user's cycles (a WHOOP day) ordered by start time descending.

**recovery** — Recovery scores for each cycle.

- `whoop-pp-cli recovery` — List recovery records

**user** — User profile and body measurements.

- `whoop-pp-cli user get-body-measurement` — Get user body measurement
- `whoop-pp-cli user get-profile` — Get user profile (basic)
- `whoop-pp-cli user revoke-oauth-access` — Revoke the user's OAuth access (delete token grants)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
whoop-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily morning check

```bash

```

### Sleep debt across the last month

```bash

```

### Correlate sleep consistency with recovery

```bash

```

### Detect overtraining trend

```bash

```

### Agent-mode query for Claude Code

```bash

```

## Auth Setup

WHOOP uses OAuth 2.0 with PKCE — there is no static API key. You register an app at developer.whoop.com, get a Client ID and Client Secret, then run `whoop-pp-cli auth login`. The CLI opens your browser, you approve the requested scopes (sleep, recovery, workout, cycles, profile, body measurement, plus offline so we can refresh), and WHOOP redirects back to http://localhost:8085/callback where the CLI is listening. Tokens are stored under ~/.config/whoop-pp-cli/tokens.json and auto-refreshed 60 seconds before expiry. For non-interactive contexts (CI, serverless), you can skip the flow and just set WHOOP_ACCESS_TOKEN (or WHOOP_OAUTH for back-compat with the prior 1.0 release).

Run `whoop-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  whoop-pp-cli activity-mapping mock-value --type example-value --agent --select id,name,status
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
whoop-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
whoop-pp-cli feedback --stdin < notes.txt
whoop-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.whoop-pp-cli/feedback.jsonl`. They are never POSTed unless `WHOOP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WHOOP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
whoop-pp-cli profile save briefing --json
whoop-pp-cli --profile briefing activity-mapping mock-value --type example-value
whoop-pp-cli profile list --json
whoop-pp-cli profile show briefing
whoop-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `whoop-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add whoop-pp-mcp -- whoop-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which whoop-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   whoop-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `whoop-pp-cli <command> --help`.

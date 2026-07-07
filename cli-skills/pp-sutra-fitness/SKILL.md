---
name: pp-sutra-fitness
description: "Every Sutra (Arketa) Partner API resource plus a local SQLite mirror and the studio analytics the dashboard buries — no-show rates, capacity, churn, revenue, and instructor scorecards. Trigger phrases: `no-show rate by instructor`, `which memberships expire this week`, `studio class utilization`, `at-risk clients for my studio`, `studio revenue versus last week`, `use sutra fitness`, `run sutra`."
author: "adam-birddog"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sutra-fitness-pp-cli
    install:
      - kind: go
        bins: [sutra-fitness-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/cmd/sutra-fitness-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/sutra-fitness/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Sutra — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sutra-fitness-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install sutra-fitness --cli-only
   ```
2. Verify: `sutra-fitness-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/cmd/sutra-fitness-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Sutra/Arketa Partner API is pure CRUD with zero reporting endpoints, so studio operators are stuck with canned vendor reports they cannot customize and a client list the vendor is cagey about exporting. This CLI syncs your studio's full dataset (locations, classes, clients, purchases, referrals, reservations) into a local SQLite database you own, then answers the questions the dashboard hides: instructor scorecards, no-show rates, capacity utilization, expiring memberships, churn risk, referral conversion, client LTV, and revenue with prior-period comparison. The daily front-desk loop (book, cancel, check-in) is in here too, all offline-queryable and agent-native.

## When to Use This CLI

Use this CLI when an operator or agent needs to answer studio-operations questions the Arketa dashboard cannot: no-show rates by instructor, capacity utilization across the schedule, which memberships expire this week, churn risk, referral conversion, client lifetime value, or revenue with prior-period comparison. It is also the fastest path for the front-desk loop of listing rosters, booking, canceling, and checking in reservations. Sync first, then query the local mirror offline.

## Anti-triggers

Do not use this CLI for:
- Charging cards, processing payments, or issuing arbitrary refunds — the API exposes purchases read-only and only supports reservation cancel/refund flags, not money movement.
- Creating or editing classes, schedules, rooms, or locations — these are read-only in the Partner API.
- Creating or editing client profiles — clients are read-only; only reservations can be created, canceled, or checked in.
- Real-time live dashboards or streaming monitors — this is a sync-then-query tool, not a live feed.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Studio analytics the dashboard buries
- **`scorecard`** — Rank instructors by class fill rate, no-show rate, and check-in rate across your synced schedule.

  _Reach for this when you need to compare teacher performance to decide who to coach or feature, not raw class rows._

  ```bash
  sutra-fitness-pp-cli scorecard --agent
  ```
- **`no-shows`** — Surface no-show rates grouped by instructor, class, or client from synced reservations.

  _Use when you need to quantify and pivot no-shows, e.g. which instructor or client drives the most absences._

  ```bash
  sutra-fitness-pp-cli no-shows --group-by instructor --json
  ```
- **`utilization`** — Compute fill ratio (booked vs capacity) per class, instructor, time-slot, or location over a date window.

  _Reach for this to find under-filled slots and over-subscribed classes before schedule planning._

  ```bash
  sutra-fitness-pp-cli utilization --group-by instructor
  ```
- **`revenue`** — Sum purchase revenue by membership type for a window and show the delta versus the prior equal window.

  _Use for the Monday revenue review: is this week up or down versus last, by membership type._

  ```bash
  sutra-fitness-pp-cli revenue --group-by type --compare-prior
  ```
- **`ltv`** — Rank clients by total purchase spend with tenure since signup.

  _Use to identify your highest-value members for VIP outreach or to understand revenue concentration._

  ```bash
  sutra-fitness-pp-cli ltv --limit 25
  ```

### Retention and renewals
- **`expiring`** — List active memberships and class-packs expiring within a window or running low on credits, with client contact info.

  _Use for deterministic renewal outreach: who needs to re-up this week, with their email and phone._

  ```bash
  sutra-fitness-pp-cli expiring --within 7d --low-credits
  ```
- **`churn`** — Flag non-removed clients with no recent check-in and/or an expired plan using a mechanical recency threshold.

  _Reach for this to build a win-back list of members drifting away, distinct from hard date-based expiry._

  ```bash
  sutra-fitness-pp-cli churn --inactive-days 30 --json
  ```
- **`referral-funnel`** — Trace referrals to whether the referred client signed up, purchased, and attended, and rank top referrers.

  _Reach for this to measure whether your referral program actually converts and who your best referrers are._

  ```bash
  sutra-fitness-pp-cli referral-funnel --json
  ```

## Command Reference

**classes** — Class management operations

- `sutra-fitness-pp-cli classes get-partner` — Retrieve a paginated list of partner classes with optional filtering
- `sutra-fitness-pp-cli classes get-partner-class` — Retrieve details for a specific class

**clients** — Client management operations

- `sutra-fitness-pp-cli clients get-partner` — Retrieve a paginated list of partner clients with optional filtering
- `sutra-fitness-pp-cli clients get-partner-clients` — Retrieve details for a specific client

**locations** — Location management operations

- `sutra-fitness-pp-cli locations <partnerId>` — Retrieve a paginated list of partner locations with optional filtering

**purchases** — Purchase management operations

- `sutra-fitness-pp-cli purchases <partnerId>` — Retrieve a paginated list of partner purchases with optional filtering

**referrals** — Referral management operations

- `sutra-fitness-pp-cli referrals <partnerId>` — Retrieve a paginated list of partner referrals with optional filtering


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sutra-fitness-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Monday revenue review

```bash
sutra-fitness-pp-cli revenue --group-by type --compare-prior
```

Totals revenue by membership type for the current window and shows the delta versus the prior equal window.

### Renewal outreach list

```bash
sutra-fitness-pp-cli expiring --within 7d --low-credits --csv
```

Exports a CSV of members expiring this week or low on credits, with contact info, ready for an email or SMS campaign.

### Agent-friendly instructor ranking

```bash
sutra-fitness-pp-cli scorecard --agent --select instructors.name,instructors.fill_rate,instructors.no_show_rate
```

Returns a narrowed, machine-readable instructor ranking so an agent can reason over fill and no-show rates without parsing the full nested payload.

### At-risk client sweep

```bash
sutra-fitness-pp-cli churn --inactive-days 30 --json
```

Lists members with no check-in in 30 days or an expired plan as a JSON win-back list.

## Auth Setup

Authentication uses a partner API key plus your partner ID. Set SUTRA_API_KEY to the key Arketa issued you (sent as the X-API-Key header) and SUTRA_PARTNER_ID to your partner identifier, which scopes every request. Run 'sutra-fitness-pp-cli doctor' to confirm both are set and the API is reachable before syncing.

Run `sutra-fitness-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sutra-fitness-pp-cli classes get-partner mock-value mock-value --agent --select id,name,status
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
sutra-fitness-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sutra-fitness-pp-cli feedback --stdin < notes.txt
sutra-fitness-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/sutra-fitness-pp-cli/feedback.jsonl`. They are never POSTed unless `SUTRA_FITNESS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUTRA_FITNESS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sutra-fitness-pp-cli profile save briefing --json
sutra-fitness-pp-cli --profile briefing classes get-partner mock-value mock-value
sutra-fitness-pp-cli profile list --json
sutra-fitness-pp-cli profile show briefing
sutra-fitness-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sutra-fitness-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/cmd/sutra-fitness-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add sutra-fitness-pp-mcp -- sutra-fitness-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sutra-fitness-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sutra-fitness-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sutra-fitness-pp-cli <command> --help`.

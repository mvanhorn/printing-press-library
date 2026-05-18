---
name: pp-brainos
description: "Your AI infrastructure CLI: offline analytics across trading, memory, agents, and MCP — all from one binary. Trigger phrases: `check my trading session`, `what did agents do overnight`, `MCP health check`, `trading drift`, `brain status`, `use brainos`, `run brainos`."
author: "Devin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - brainos-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/ai/brainos/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# BrainOS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `brainos-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install brainos --cli-only
   ```
2. Verify: `brainos-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/brainos/cmd/brainos-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Run `brainos-pp-cli sync` to mirror your Supabase backend to a local SQLite store, enabling sub-second cross-domain queries that no single API call or dashboard can answer. Trading drift alerts, MCP blast radius traces, agent deadlock detection, and thought-to-task latency — all offline, all composable with jq.

## When to Use This CLI

Use brainos-pp-cli when you need to query your AI infrastructure backend without opening a dashboard. Best for morning standups (trading pulse, brain since), incident response (mcp blast-radius, agents deadlock), and weekly reviews (trading drift, skills gap). It is the only tool that joins trading, memory, MCP, and agent data in a single offline query.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Trading intelligence
- **`trading pulse`** — Instant overview of today's trading session: wins, losses, running PnL, regime, and setup distribution.

  _Use when you need a pre-market or mid-session status check without opening a dashboard._

  ```bash
  brainos-pp-cli trading pulse --json
  ```
- **`trading calibrate`** — Current win rate by setup type and market regime from calibration history + recent sessions.

  _Use before sizing a trade to confirm calibration data supports the setup in the current regime._

  ```bash
  brainos-pp-cli trading calibrate --setup gap-go --regime trending --json
  ```
- **`trading drift`** — Detects when your premortem setup distribution is drifting toward lower expected-value setups before P&L shows it.

  _Use weekly to catch unconscious setup drift before it becomes a drawdown._

  ```bash
  brainos-pp-cli trading drift --weeks 2 --json
  ```

### MCP infrastructure
- **`mcp reliability`** — Latency percentiles and error rates per MCP server, with auth error overlay.

  _Use when diagnosing agent failures that may be caused by a degraded MCP backend._

  ```bash
  brainos-pp-cli mcp reliability --top 5 --json
  ```
- **`mcp blast-radius`** — Shows which NatureOS tasks failed downstream of MCP auth errors within a time window.

  _Use when agents are failing and you need to trace whether an MCP auth error is the root cause._

  ```bash
  brainos-pp-cli mcp blast-radius --since 1h --json
  ```

### Brain and memory
- **`memory load`** — Which agents have the most active memories, ranked by importance, with expiry alerts.

  _Use when an agent is behaving oddly to check if it is over- or under-loaded with context._

  ```bash
  brainos-pp-cli memory load --agent navigator --json
  ```
- **`brain since`** — All thoughts, agent messages, and shared_state changes in a time window — cross-domain activity snapshot.

  _Use at start of day to catch up on what agents did overnight._

  ```bash
  brainos-pp-cli brain since 2h --json
  ```
- **`brain lag`** — How long insights sit in thoughts before becoming NatureOS agent tasks, by topic.

  _Use to identify which thought topics are being ignored by the agent system._

  ```bash
  brainos-pp-cli brain lag --topic ops --json
  ```
- **`brain anomalies`** — Detects unusual spikes: MCP auth error floods, memory expiry cliffs, task queue backlogs — all at once.

  _Use when something feels off but you are not sure where — surfaces the domain with the highest deviation._

  ```bash
  brainos-pp-cli brain anomalies --hours 1 --json
  ```

### Agent coordination
- **`agents deadlock`** — Finds agents holding shared_state locks while sending no messages — potential deadlock.

  _Use when agents seem stuck but no error is surfaced — often a silent lock._

  ```bash
  brainos-pp-cli agents deadlock --json
  ```
- **`agents throughput`** — NatureOS task completion rates by agent type over a rolling window, with backlog depth.

  _Use for daily agent health check to see if any agent type is falling behind._

  ```bash
  brainos-pp-cli agents throughput --hours 24 --json
  ```
- **`skills gap`** — Which skill domains have low proficiency relative to tool usage frequency.

  _Use monthly to identify which capability areas need reinforcement based on actual workload._

  ```bash
  brainos-pp-cli skills gap --domain AI --json
  ```

## Command Reference

**active-memory** — Manage active memory

- `brainos-pp-cli active-memory create` — Create
- `brainos-pp-cli active-memory delete` — Delete
- `brainos-pp-cli active-memory list` — List
- `brainos-pp-cli active-memory update` — Update

**agent-messages** — Manage agent messages

- `brainos-pp-cli agent-messages create` — 1) pg_notify (instant via trigger) 2) Supabase Realtime WebSocket subscription 3) 5-min poll fallback (trigger...
- `brainos-pp-cli agent-messages delete` — 1) pg_notify (instant via trigger) 2) Supabase Realtime WebSocket subscription 3) 5-min poll fallback (trigger...
- `brainos-pp-cli agent-messages list` — 1) pg_notify (instant via trigger) 2) Supabase Realtime WebSocket subscription 3) 5-min poll fallback (trigger...
- `brainos-pp-cli agent-messages update` — 1) pg_notify (instant via trigger) 2) Supabase Realtime WebSocket subscription 3) 5-min poll fallback (trigger...

**agent-messages-status-summary** — Manage agent messages status summary

- `brainos-pp-cli agent-messages-status-summary` — Summary view showing message count by status for monitoring three-layer sync architecture health.

**brain-config** — Manage brain config

- `brainos-pp-cli brain-config create` — Create
- `brainos-pp-cli brain-config delete` — Delete
- `brainos-pp-cli brain-config list` — List
- `brainos-pp-cli brain-config update` — Update

**brain-email-capture-log** — Manage brain email capture log

- `brainos-pp-cli brain-email-capture-log create` — Create
- `brainos-pp-cli brain-email-capture-log delete` — Delete
- `brainos-pp-cli brain-email-capture-log list` — List
- `brainos-pp-cli brain-email-capture-log update` — Update

**brain-project-status** — Manage brain project status

- `brainos-pp-cli brain-project-status create` — Create
- `brainos-pp-cli brain-project-status delete` — Delete
- `brainos-pp-cli brain-project-status list` — List
- `brainos-pp-cli brain-project-status update` — Update

**broker-config** — Manage broker config

- `brainos-pp-cli broker-config create` — Sentinel v1.1 Phase 8: tracks each broker account post-PDT framework implementation (realtime vs EOD margin,...
- `brainos-pp-cli broker-config delete` — Sentinel v1.1 Phase 8: tracks each broker account post-PDT framework implementation (realtime vs EOD margin,...
- `brainos-pp-cli broker-config list` — Sentinel v1.1 Phase 8: tracks each broker account post-PDT framework implementation (realtime vs EOD margin,...
- `brainos-pp-cli broker-config update` — Sentinel v1.1 Phase 8: tracks each broker account post-PDT framework implementation (realtime vs EOD margin,...

**cron-jobs** — Manage cron jobs

- `brainos-pp-cli cron-jobs create` — Create
- `brainos-pp-cli cron-jobs delete` — Delete
- `brainos-pp-cli cron-jobs list` — List
- `brainos-pp-cli cron-jobs update` — Update

**documents** — Manage documents

- `brainos-pp-cli documents create` — Create
- `brainos-pp-cli documents delete` — Delete
- `brainos-pp-cli documents list` — List
- `brainos-pp-cli documents update` — Update

**executor-briefs** — Manage executor briefs

- `brainos-pp-cli executor-briefs create` — Create
- `brainos-pp-cli executor-briefs delete` — Delete
- `brainos-pp-cli executor-briefs list` — List
- `brainos-pp-cli executor-briefs update` — Update

**margin-log** — Manage margin log

- `brainos-pp-cli margin-log create` — Sentinel v1.1 Phase 8: intraday margin excess snapshots; populated only when broker provides realtime margin feed.
- `brainos-pp-cli margin-log delete` — Sentinel v1.1 Phase 8: intraday margin excess snapshots; populated only when broker provides realtime margin feed.
- `brainos-pp-cli margin-log list` — Sentinel v1.1 Phase 8: intraday margin excess snapshots; populated only when broker provides realtime margin feed.
- `brainos-pp-cli margin-log update` — Sentinel v1.1 Phase 8: intraday margin excess snapshots; populated only when broker provides realtime margin feed.

**mcp-activity-logs** — Manage mcp activity logs

- `brainos-pp-cli mcp-activity-logs create` — Create
- `brainos-pp-cli mcp-activity-logs delete` — Delete
- `brainos-pp-cli mcp-activity-logs list` — List
- `brainos-pp-cli mcp-activity-logs update` — Update

**mcp-api-keys** — Manage mcp api keys

- `brainos-pp-cli mcp-api-keys create` — Create
- `brainos-pp-cli mcp-api-keys delete` — Delete
- `brainos-pp-cli mcp-api-keys list` — List
- `brainos-pp-cli mcp-api-keys update` — Update

**mcp-auth-errors** — Manage mcp auth errors

- `brainos-pp-cli mcp-auth-errors create` — Create
- `brainos-pp-cli mcp-auth-errors delete` — Delete
- `brainos-pp-cli mcp-auth-errors list` — List
- `brainos-pp-cli mcp-auth-errors update` — Update

**mcp-oauth-states** — Manage mcp oauth states

- `brainos-pp-cli mcp-oauth-states create` — Create
- `brainos-pp-cli mcp-oauth-states delete` — Delete
- `brainos-pp-cli mcp-oauth-states list` — List
- `brainos-pp-cli mcp-oauth-states update` — Update

**mcp-servers** — Manage mcp servers

- `brainos-pp-cli mcp-servers create` — Create
- `brainos-pp-cli mcp-servers delete` — Delete
- `brainos-pp-cli mcp-servers list` — List
- `brainos-pp-cli mcp-servers update` — Update

**mcp-sessions** — Manage mcp sessions

- `brainos-pp-cli mcp-sessions create` — Create
- `brainos-pp-cli mcp-sessions delete` — Delete
- `brainos-pp-cli mcp-sessions list` — List
- `brainos-pp-cli mcp-sessions update` — Update

**mcp-tools** — Manage mcp tools

- `brainos-pp-cli mcp-tools create` — Create
- `brainos-pp-cli mcp-tools delete` — Delete
- `brainos-pp-cli mcp-tools list` — List
- `brainos-pp-cli mcp-tools update` — Update

**mcpapikeys** — Manage mcpapikeys

- `brainos-pp-cli mcpapikeys create` — Create
- `brainos-pp-cli mcpapikeys delete` — Delete
- `brainos-pp-cli mcpapikeys list` — List
- `brainos-pp-cli mcpapikeys update` — Update

**mcpsessions** — Manage mcpsessions

- `brainos-pp-cli mcpsessions create` — Create
- `brainos-pp-cli mcpsessions delete` — Delete
- `brainos-pp-cli mcpsessions list` — List
- `brainos-pp-cli mcpsessions update` — Update

**natureos-agent-checkpoints** — Manage natureos agent checkpoints

- `brainos-pp-cli natureos-agent-checkpoints create` — Create
- `brainos-pp-cli natureos-agent-checkpoints delete` — Delete
- `brainos-pp-cli natureos-agent-checkpoints list` — List
- `brainos-pp-cli natureos-agent-checkpoints update` — Update

**natureos-agent-memory** — Manage natureos agent memory

- `brainos-pp-cli natureos-agent-memory create` — Create
- `brainos-pp-cli natureos-agent-memory delete` — Delete
- `brainos-pp-cli natureos-agent-memory list` — List
- `brainos-pp-cli natureos-agent-memory update` — Update

**natureos-allowlist** — Manage natureos allowlist

- `brainos-pp-cli natureos-allowlist create` — Create
- `brainos-pp-cli natureos-allowlist delete` — Delete
- `brainos-pp-cli natureos-allowlist list` — List
- `brainos-pp-cli natureos-allowlist update` — Update

**natureos-dual-model-tasks** — Manage natureos dual model tasks

- `brainos-pp-cli natureos-dual-model-tasks create` — Create
- `brainos-pp-cli natureos-dual-model-tasks delete` — Delete
- `brainos-pp-cli natureos-dual-model-tasks list` — List
- `brainos-pp-cli natureos-dual-model-tasks update` — Update

**natureos-task-queue** — Manage natureos task queue

- `brainos-pp-cli natureos-task-queue create` — Create
- `brainos-pp-cli natureos-task-queue delete` — Delete
- `brainos-pp-cli natureos-task-queue list` — List
- `brainos-pp-cli natureos-task-queue update` — Update

**rpc** — Manage rpc

- `brainos-pp-cli rpc create` — Create
- `brainos-pp-cli rpc create-cleanupexpiredcheckpoints` — Create cleanupexpiredcheckpoints
- `brainos-pp-cli rpc create-cleanupexpiredsessions` — Create cleanupexpiredsessions
- `brainos-pp-cli rpc create-cleanupoldlogs` — Create cleanupoldlogs
- `brainos-pp-cli rpc create-enqueuetasks` — Create enqueuetasks
- `brainos-pp-cli rpc create-getrunresults` — Create getrunresults
- `brainos-pp-cli rpc create-isvalidsession` — Create isvalidsession
- `brainos-pp-cli rpc create-matchthoughts` — Create matchthoughts
- `brainos-pp-cli rpc create-readagentmemory` — Create readagentmemory
- `brainos-pp-cli rpc create-searchdocuments` — Create searchdocuments
- `brainos-pp-cli rpc create-searchdocumentskeyword` — Create searchdocumentskeyword
- `brainos-pp-cli rpc create-searchdocumentsvector` — Create searchdocumentsvector
- `brainos-pp-cli rpc create-searchthoughtshybrid` — Create searchthoughtshybrid
- `brainos-pp-cli rpc create-showlimit` — Create showlimit
- `brainos-pp-cli rpc create-showtrgm` — Create showtrgm
- `brainos-pp-cli rpc create-upsertagentmemory` — Create upsertagentmemory
- `brainos-pp-cli rpc create-upsertcheckpoint` — Create upsertcheckpoint
- `brainos-pp-cli rpc list` — List
- `brainos-pp-cli rpc list-searchdocumentskeyword` — List searchdocumentskeyword
- `brainos-pp-cli rpc list-searchdocumentsvector` — List searchdocumentsvector
- `brainos-pp-cli rpc list-searchthoughtshybrid` — List searchthoughtshybrid
- `brainos-pp-cli rpc list-showlimit` — List showlimit
- `brainos-pp-cli rpc list-showtrgm` — List showtrgm

**shared-state** — Manage shared state

- `brainos-pp-cli shared-state create` — Create
- `brainos-pp-cli shared-state delete` — Delete
- `brainos-pp-cli shared-state list` — List
- `brainos-pp-cli shared-state update` — Update

**skills** — Manage skills

- `brainos-pp-cli skills create` — Create
- `brainos-pp-cli skills delete` — Delete
- `brainos-pp-cli skills list` — List
- `brainos-pp-cli skills update` — Update

**thoughts** — Manage thoughts

- `brainos-pp-cli thoughts create` — Create
- `brainos-pp-cli thoughts delete` — Delete
- `brainos-pp-cli thoughts list` — List
- `brainos-pp-cli thoughts update` — Update

**tool-usage-stats** — Manage tool usage stats

- `brainos-pp-cli tool-usage-stats` — List

**trading-calibration** — Manage trading calibration

- `brainos-pp-cli trading-calibration create` — Sentinel v1.1 Wave 2: Brier score + win rate + expectancy by setup_type x regime x window; NULL setup_type/regime =...
- `brainos-pp-cli trading-calibration delete` — Sentinel v1.1 Wave 2: Brier score + win rate + expectancy by setup_type x regime x window; NULL setup_type/regime =...
- `brainos-pp-cli trading-calibration list` — Sentinel v1.1 Wave 2: Brier score + win rate + expectancy by setup_type x regime x window; NULL setup_type/regime =...
- `brainos-pp-cli trading-calibration update` — Sentinel v1.1 Wave 2: Brier score + win rate + expectancy by setup_type x regime x window; NULL setup_type/regime =...

**trading-postmortems** — Manage trading postmortems

- `brainos-pp-cli trading-postmortems create` — Sentinel v1.1 Wave 2: closed-trade outcome + lessons; mirrors state/postmortem-*.jsonl; FK to trading_premortems for...
- `brainos-pp-cli trading-postmortems delete` — Sentinel v1.1 Wave 2: closed-trade outcome + lessons; mirrors state/postmortem-*.jsonl; FK to trading_premortems for...
- `brainos-pp-cli trading-postmortems list` — Sentinel v1.1 Wave 2: closed-trade outcome + lessons; mirrors state/postmortem-*.jsonl; FK to trading_premortems for...
- `brainos-pp-cli trading-postmortems update` — Sentinel v1.1 Wave 2: closed-trade outcome + lessons; mirrors state/postmortem-*.jsonl; FK to trading_premortems for...

**trading-premortems** — Manage trading premortems

- `brainos-pp-cli trading-premortems create` — Sentinel v1.1 Wave 2: pre-trade hypothesis + sized-risk capture before order entry; mirrors state/premortem-*.jsonl.
- `brainos-pp-cli trading-premortems delete` — Sentinel v1.1 Wave 2: pre-trade hypothesis + sized-risk capture before order entry; mirrors state/premortem-*.jsonl.
- `brainos-pp-cli trading-premortems list` — Sentinel v1.1 Wave 2: pre-trade hypothesis + sized-risk capture before order entry; mirrors state/premortem-*.jsonl.
- `brainos-pp-cli trading-premortems update` — Sentinel v1.1 Wave 2: pre-trade hypothesis + sized-risk capture before order entry; mirrors state/premortem-*.jsonl.

**trading-regime-log** — Manage trading regime log

- `brainos-pp-cli trading-regime-log create` — Sentinel v1.1 Wave 2 + Sentinel v4 (shared): daily SPX/VIX regime classification; A=trending bull, B=late-cycle,...
- `brainos-pp-cli trading-regime-log delete` — Sentinel v1.1 Wave 2 + Sentinel v4 (shared): daily SPX/VIX regime classification; A=trending bull, B=late-cycle,...
- `brainos-pp-cli trading-regime-log list` — Sentinel v1.1 Wave 2 + Sentinel v4 (shared): daily SPX/VIX regime classification; A=trending bull, B=late-cycle,...
- `brainos-pp-cli trading-regime-log update` — Sentinel v1.1 Wave 2 + Sentinel v4 (shared): daily SPX/VIX regime classification; A=trending bull, B=late-cycle,...

**trading-sessions** — Manage trading sessions

- `brainos-pp-cli trading-sessions create` — Sentinel v1.1 Wave 2: per-day rollup of trades + operator state + halt status; mirrors state/session-summary-*.txt;...
- `brainos-pp-cli trading-sessions delete` — Sentinel v1.1 Wave 2: per-day rollup of trades + operator state + halt status; mirrors state/session-summary-*.txt;...
- `brainos-pp-cli trading-sessions list` — Sentinel v1.1 Wave 2: per-day rollup of trades + operator state + halt status; mirrors state/session-summary-*.txt;...
- `brainos-pp-cli trading-sessions update` — Sentinel v1.1 Wave 2: per-day rollup of trades + operator state + halt status; mirrors state/session-summary-*.txt;...

**webauthn-credentials** — Manage webauthn credentials

- `brainos-pp-cli webauthn-credentials create` — Create
- `brainos-pp-cli webauthn-credentials delete` — Delete
- `brainos-pp-cli webauthn-credentials list` — List
- `brainos-pp-cli webauthn-credentials update` — Update


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
brainos-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning standup

```bash
brainos-pp-cli trading pulse --json && brainos-pp-cli brain since 8h --json --select thoughts.content,agent_messages.sender
```

Get today's trading status and overnight agent activity in one pipeline.

### Incident triage

```bash
brainos-pp-cli mcp blast-radius --since 2h --json | jq '.[] | select(.task_failures > 0)'
```

Find which MCP errors caused downstream task failures in the last 2 hours.

### Weekly trading review

```bash
brainos-pp-cli trading drift --weeks 2 --json && brainos-pp-cli trading calibrate --json --select setup_type,win_rate,expected_value
```

Check if setup distribution is drifting and confirm calibration still supports your edge.

### Agent health check

```bash
brainos-pp-cli agents deadlock --json && brainos-pp-cli agents throughput --hours 24 --json
```

Detect stuck agents and confirm throughput is normal.

### Brain context dump

```bash
brainos-pp-cli brain since 24h --agent --select thoughts.content,thoughts.topics,agent_messages.content,shared_state.key,shared_state.value
```

Full context window of system activity for agent consumption.

## Auth Setup

No authentication required.

Run `brainos-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  brainos-pp-cli active-memory list --agent --select id,name,status
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
brainos-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
brainos-pp-cli feedback --stdin < notes.txt
brainos-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.brainos-pp-cli/feedback.jsonl`. They are never POSTed unless `BRAINOS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BRAINOS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
brainos-pp-cli profile save briefing --json
brainos-pp-cli --profile briefing active-memory list
brainos-pp-cli profile list --json
brainos-pp-cli profile show briefing
brainos-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `brainos-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add brainos-pp-mcp -- brainos-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which brainos-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   brainos-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `brainos-pp-cli <command> --help`.

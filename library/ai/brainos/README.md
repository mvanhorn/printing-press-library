# BrainOS CLI

**Your AI infrastructure CLI: offline analytics across trading, memory, agents, and MCP — all from one binary.**

brainos-pp-cli syncs your Supabase backend to a local SQLite store, enabling sub-second cross-domain queries that no single API call or dashboard can answer. Trading drift alerts, MCP blast radius traces, agent deadlock detection, and thought-to-task latency — all offline, all composable with jq.

## Install

The recommended path installs both the `brainos-pp-cli` binary and the `pp-brainos` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install brainos
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install brainos --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/brainos-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-brainos --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-brainos --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-brainos skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-brainos. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# verify auth and API reachability
brainos-pp-cli doctor


# mirror all key tables to local SQLite
brainos-pp-cli sync --full


# today's session overview
brainos-pp-cli trading pulse --json


# what happened in the last 2 hours
brainos-pp-cli brain since 2h --json


# MCP server health report
brainos-pp-cli mcp reliability --json

```

## Authentication

BrainOS connects to your Supabase project using service-role or anon keys from the PostgREST API.

```bash
# Use the service-role key (full access — recommended for local tools)
export BRAINOS_SERVICE_KEY=your-service-role-key

# Or use the anon key (public schema access only)
export BRAINOS_ANON_KEY=your-anon-key
```

Get your keys from the **Supabase Dashboard → Project Settings → API** panel.

To point at a non-default Supabase project:

```bash
export BRAINOS_BASE_URL=https://your-project-ref.supabase.co/rest/v1
```

You can also persist credentials in the config file (`~/.config/brainos-pp-cli/config.toml`):

```toml
base_url = "https://your-project-ref.supabase.co/rest/v1"
auth_header = "your-service-role-key"
```

Run `brainos-pp-cli doctor` after setting credentials to verify connectivity.

## Unique Features

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

## Cookbook

### First-time setup

```bash
# 1. Set your credentials
export BRAINOS_SERVICE_KEY=your-service-role-key

# 2. Verify connectivity
brainos-pp-cli doctor

# 3. Mirror all tables to local SQLite
brainos-pp-cli sync --full
```

### Morning trading review

```bash
# Today's session snapshot
brainos-pp-cli trading pulse --json

# Win rate filtered to gap-go setups in trending regime
brainos-pp-cli trading calibrate --setup gap-go --regime trending --json

# Is my setup mix drifting toward lower-EV trades?
brainos-pp-cli trading drift --weeks 4 --json
```

### Incident triage: something went wrong in the last hour

```bash
# Cross-domain activity snapshot
brainos-pp-cli brain since 1h --json

# Detect spikes: MCP errors, memory expiry, task backlog
brainos-pp-cli brain anomalies --hours 1 --json

# Which tasks failed downstream of MCP auth errors?
brainos-pp-cli mcp blast-radius --since 1h --json

# Which MCP servers are slowest or erroring?
brainos-pp-cli mcp reliability --top 5 --json
```

### Agent health check

```bash
# Any agents stuck holding a shared_state lock?
brainos-pp-cli agents deadlock --json

# Task completion rate by agent type (last 24h)
brainos-pp-cli agents throughput --hours 24 --json

# Which agents have the most active memories with imminent expiry?
brainos-pp-cli memory load --json
```

### Thought-to-task lag analysis

```bash
# How long before thoughts become NatureOS tasks?
brainos-pp-cli brain lag --json

# Filter by topic
brainos-pp-cli brain lag --topic trading --json
```

### Search and export

```bash
# Full-text search across all locally synced data
brainos-pp-cli search "MCP auth error" --json

# Search within a specific resource type
brainos-pp-cli search "critical" --type thoughts --data-source local --json

# Export all trading sessions to JSONL for backup
brainos-pp-cli export --type trading-sessions > trading-sessions.jsonl

# Count agent messages by status
brainos-pp-cli analytics --type agent-messages --group-by status --json
```

### Stream live changes

```bash
# Tail the thoughts table in real time
brainos-pp-cli tail --type thoughts --json

# Watch MCP auth errors as they arrive
brainos-pp-cli tail --type mcp-auth-errors --json
```

### Agent-mode pipeline (for Claude Code / scripts)

```bash
# Compact JSON for agent consumption
brainos-pp-cli trading pulse --agent

# Route output to a file for downstream processing
brainos-pp-cli brain since 2h --agent --deliver file:brain-snapshot.json

# Save a profile for your daily morning workflow
brainos-pp-cli profile save morning --json --compact
brainos-pp-cli trading pulse --profile morning
```

## Usage

Run `brainos-pp-cli --help` for the full command reference and flag list.

## Commands

### active-memory

Manage active memory

- **`brainos-pp-cli active-memory create`** - Create
- **`brainos-pp-cli active-memory delete`** - Delete
- **`brainos-pp-cli active-memory list`** - List
- **`brainos-pp-cli active-memory update`** - Update

### agent-messages

Manage agent messages

- **`brainos-pp-cli agent-messages create`** - 1) pg_notify (instant via trigger) 
   2) Supabase Realtime WebSocket subscription 
   3) 5-min poll fallback (trigger f42a538a) 
   
   The processed_at column prevents double-processing between Realtime and Poll layers.
- **`brainos-pp-cli agent-messages delete`** - 1) pg_notify (instant via trigger) 
   2) Supabase Realtime WebSocket subscription 
   3) 5-min poll fallback (trigger f42a538a) 
   
   The processed_at column prevents double-processing between Realtime and Poll layers.
- **`brainos-pp-cli agent-messages list`** - 1) pg_notify (instant via trigger) 
   2) Supabase Realtime WebSocket subscription 
   3) 5-min poll fallback (trigger f42a538a) 
   
   The processed_at column prevents double-processing between Realtime and Poll layers.
- **`brainos-pp-cli agent-messages update`** - 1) pg_notify (instant via trigger) 
   2) Supabase Realtime WebSocket subscription 
   3) 5-min poll fallback (trigger f42a538a) 
   
   The processed_at column prevents double-processing between Realtime and Poll layers.

### agent-messages-status-summary

Manage agent messages status summary

- **`brainos-pp-cli agent-messages-status-summary list`** - Summary view showing message count by status for monitoring three-layer sync architecture health.

### brain-config

Manage brain config

- **`brainos-pp-cli brain-config create`** - Create
- **`brainos-pp-cli brain-config delete`** - Delete
- **`brainos-pp-cli brain-config list`** - List
- **`brainos-pp-cli brain-config update`** - Update

### brain-email-capture-log

Manage brain email capture log

- **`brainos-pp-cli brain-email-capture-log create`** - Create
- **`brainos-pp-cli brain-email-capture-log delete`** - Delete
- **`brainos-pp-cli brain-email-capture-log list`** - List
- **`brainos-pp-cli brain-email-capture-log update`** - Update

### brain-project-status

Manage brain project status

- **`brainos-pp-cli brain-project-status create`** - Create
- **`brainos-pp-cli brain-project-status delete`** - Delete
- **`brainos-pp-cli brain-project-status list`** - List
- **`brainos-pp-cli brain-project-status update`** - Update

### broker-config

Manage broker config

- **`brainos-pp-cli broker-config create`** - Sentinel v1.1 Phase 8: tracks each broker account post-PDT framework implementation (realtime vs EOD margin, legacy_pdt vs post_pdt).
- **`brainos-pp-cli broker-config delete`** - Sentinel v1.1 Phase 8: tracks each broker account post-PDT framework implementation (realtime vs EOD margin, legacy_pdt vs post_pdt).
- **`brainos-pp-cli broker-config list`** - Sentinel v1.1 Phase 8: tracks each broker account post-PDT framework implementation (realtime vs EOD margin, legacy_pdt vs post_pdt).
- **`brainos-pp-cli broker-config update`** - Sentinel v1.1 Phase 8: tracks each broker account post-PDT framework implementation (realtime vs EOD margin, legacy_pdt vs post_pdt).

### cron-jobs

Manage cron jobs

- **`brainos-pp-cli cron-jobs create`** - Create
- **`brainos-pp-cli cron-jobs delete`** - Delete
- **`brainos-pp-cli cron-jobs list`** - List
- **`brainos-pp-cli cron-jobs update`** - Update

### documents

Manage documents

- **`brainos-pp-cli documents create`** - Create
- **`brainos-pp-cli documents delete`** - Delete
- **`brainos-pp-cli documents list`** - List
- **`brainos-pp-cli documents update`** - Update

### executor-briefs

Manage executor briefs

- **`brainos-pp-cli executor-briefs create`** - Create
- **`brainos-pp-cli executor-briefs delete`** - Delete
- **`brainos-pp-cli executor-briefs list`** - List
- **`brainos-pp-cli executor-briefs update`** - Update

### margin-log

Manage margin log

- **`brainos-pp-cli margin-log create`** - Sentinel v1.1 Phase 8: intraday margin excess snapshots; populated only when broker provides realtime margin feed.
- **`brainos-pp-cli margin-log delete`** - Sentinel v1.1 Phase 8: intraday margin excess snapshots; populated only when broker provides realtime margin feed.
- **`brainos-pp-cli margin-log list`** - Sentinel v1.1 Phase 8: intraday margin excess snapshots; populated only when broker provides realtime margin feed.
- **`brainos-pp-cli margin-log update`** - Sentinel v1.1 Phase 8: intraday margin excess snapshots; populated only when broker provides realtime margin feed.

### mcp-activity-logs

Manage mcp activity logs

- **`brainos-pp-cli mcp-activity-logs create`** - Create
- **`brainos-pp-cli mcp-activity-logs delete`** - Delete
- **`brainos-pp-cli mcp-activity-logs list`** - List
- **`brainos-pp-cli mcp-activity-logs update`** - Update

### mcp-api-keys

Manage mcp api keys

- **`brainos-pp-cli mcp-api-keys create`** - Create
- **`brainos-pp-cli mcp-api-keys delete`** - Delete
- **`brainos-pp-cli mcp-api-keys list`** - List
- **`brainos-pp-cli mcp-api-keys update`** - Update

### mcp-auth-errors

Manage mcp auth errors

- **`brainos-pp-cli mcp-auth-errors create`** - Create
- **`brainos-pp-cli mcp-auth-errors delete`** - Delete
- **`brainos-pp-cli mcp-auth-errors list`** - List
- **`brainos-pp-cli mcp-auth-errors update`** - Update

### mcp-oauth-states

Manage mcp oauth states

- **`brainos-pp-cli mcp-oauth-states create`** - Create
- **`brainos-pp-cli mcp-oauth-states delete`** - Delete
- **`brainos-pp-cli mcp-oauth-states list`** - List
- **`brainos-pp-cli mcp-oauth-states update`** - Update

### mcp-servers

Manage mcp servers

- **`brainos-pp-cli mcp-servers create`** - Create
- **`brainos-pp-cli mcp-servers delete`** - Delete
- **`brainos-pp-cli mcp-servers list`** - List
- **`brainos-pp-cli mcp-servers update`** - Update

### mcp-sessions

Manage mcp sessions

- **`brainos-pp-cli mcp-sessions create`** - Create
- **`brainos-pp-cli mcp-sessions delete`** - Delete
- **`brainos-pp-cli mcp-sessions list`** - List
- **`brainos-pp-cli mcp-sessions update`** - Update

### mcp-tools

Manage mcp tools

- **`brainos-pp-cli mcp-tools create`** - Create
- **`brainos-pp-cli mcp-tools delete`** - Delete
- **`brainos-pp-cli mcp-tools list`** - List
- **`brainos-pp-cli mcp-tools update`** - Update

### mcpapikeys

Manage mcpapikeys

- **`brainos-pp-cli mcpapikeys create`** - Create
- **`brainos-pp-cli mcpapikeys delete`** - Delete
- **`brainos-pp-cli mcpapikeys list`** - List
- **`brainos-pp-cli mcpapikeys update`** - Update

### mcpsessions

Manage mcpsessions

- **`brainos-pp-cli mcpsessions create`** - Create
- **`brainos-pp-cli mcpsessions delete`** - Delete
- **`brainos-pp-cli mcpsessions list`** - List
- **`brainos-pp-cli mcpsessions update`** - Update

### natureos-agent-checkpoints

Manage natureos agent checkpoints

- **`brainos-pp-cli natureos-agent-checkpoints create`** - Create
- **`brainos-pp-cli natureos-agent-checkpoints delete`** - Delete
- **`brainos-pp-cli natureos-agent-checkpoints list`** - List
- **`brainos-pp-cli natureos-agent-checkpoints update`** - Update

### natureos-agent-memory

Manage natureos agent memory

- **`brainos-pp-cli natureos-agent-memory create`** - Create
- **`brainos-pp-cli natureos-agent-memory delete`** - Delete
- **`brainos-pp-cli natureos-agent-memory list`** - List
- **`brainos-pp-cli natureos-agent-memory update`** - Update

### natureos-allowlist

Manage natureos allowlist

- **`brainos-pp-cli natureos-allowlist create`** - Create
- **`brainos-pp-cli natureos-allowlist delete`** - Delete
- **`brainos-pp-cli natureos-allowlist list`** - List
- **`brainos-pp-cli natureos-allowlist update`** - Update

### natureos-dual-model-tasks

Manage natureos dual model tasks

- **`brainos-pp-cli natureos-dual-model-tasks create`** - Create
- **`brainos-pp-cli natureos-dual-model-tasks delete`** - Delete
- **`brainos-pp-cli natureos-dual-model-tasks list`** - List
- **`brainos-pp-cli natureos-dual-model-tasks update`** - Update

### natureos-task-queue

Manage natureos task queue

- **`brainos-pp-cli natureos-task-queue create`** - Create
- **`brainos-pp-cli natureos-task-queue delete`** - Delete
- **`brainos-pp-cli natureos-task-queue list`** - List
- **`brainos-pp-cli natureos-task-queue update`** - Update

### rpc

Manage rpc

- **`brainos-pp-cli rpc create`** - Create
- **`brainos-pp-cli rpc create-cleanupexpiredcheckpoints`** - Create cleanupexpiredcheckpoints
- **`brainos-pp-cli rpc create-cleanupexpiredsessions`** - Create cleanupexpiredsessions
- **`brainos-pp-cli rpc create-cleanupoldlogs`** - Create cleanupoldlogs
- **`brainos-pp-cli rpc create-enqueuetasks`** - Create enqueuetasks
- **`brainos-pp-cli rpc create-getrunresults`** - Create getrunresults
- **`brainos-pp-cli rpc create-isvalidsession`** - Create isvalidsession
- **`brainos-pp-cli rpc create-matchthoughts`** - Create matchthoughts
- **`brainos-pp-cli rpc create-readagentmemory`** - Create readagentmemory
- **`brainos-pp-cli rpc create-searchdocuments`** - Create searchdocuments
- **`brainos-pp-cli rpc create-searchdocumentskeyword`** - Create searchdocumentskeyword
- **`brainos-pp-cli rpc create-searchdocumentsvector`** - Create searchdocumentsvector
- **`brainos-pp-cli rpc create-searchthoughtshybrid`** - Create searchthoughtshybrid
- **`brainos-pp-cli rpc create-showlimit`** - Create showlimit
- **`brainos-pp-cli rpc create-showtrgm`** - Create showtrgm
- **`brainos-pp-cli rpc create-upsertagentmemory`** - Create upsertagentmemory
- **`brainos-pp-cli rpc create-upsertcheckpoint`** - Create upsertcheckpoint
- **`brainos-pp-cli rpc list`** - List
- **`brainos-pp-cli rpc list-searchdocumentskeyword`** - List searchdocumentskeyword
- **`brainos-pp-cli rpc list-searchdocumentsvector`** - List searchdocumentsvector
- **`brainos-pp-cli rpc list-searchthoughtshybrid`** - List searchthoughtshybrid
- **`brainos-pp-cli rpc list-showlimit`** - List showlimit
- **`brainos-pp-cli rpc list-showtrgm`** - List showtrgm

### shared-state

Manage shared state

- **`brainos-pp-cli shared-state create`** - Create
- **`brainos-pp-cli shared-state delete`** - Delete
- **`brainos-pp-cli shared-state list`** - List
- **`brainos-pp-cli shared-state update`** - Update

### skills

Manage skills

- **`brainos-pp-cli skills create`** - Create
- **`brainos-pp-cli skills delete`** - Delete
- **`brainos-pp-cli skills list`** - List
- **`brainos-pp-cli skills update`** - Update

### thoughts

Manage thoughts

- **`brainos-pp-cli thoughts create`** - Create
- **`brainos-pp-cli thoughts delete`** - Delete
- **`brainos-pp-cli thoughts list`** - List
- **`brainos-pp-cli thoughts update`** - Update

### tool-usage-stats

Manage tool usage stats

- **`brainos-pp-cli tool-usage-stats list`** - List

### trading-calibration

Manage trading calibration

- **`brainos-pp-cli trading-calibration create`** - Sentinel v1.1 Wave 2: Brier score + win rate + expectancy by setup_type x regime x window; NULL setup_type/regime = aggregate across that dimension.
- **`brainos-pp-cli trading-calibration delete`** - Sentinel v1.1 Wave 2: Brier score + win rate + expectancy by setup_type x regime x window; NULL setup_type/regime = aggregate across that dimension.
- **`brainos-pp-cli trading-calibration list`** - Sentinel v1.1 Wave 2: Brier score + win rate + expectancy by setup_type x regime x window; NULL setup_type/regime = aggregate across that dimension.
- **`brainos-pp-cli trading-calibration update`** - Sentinel v1.1 Wave 2: Brier score + win rate + expectancy by setup_type x regime x window; NULL setup_type/regime = aggregate across that dimension.

### trading-postmortems

Manage trading postmortems

- **`brainos-pp-cli trading-postmortems create`** - Sentinel v1.1 Wave 2: closed-trade outcome + lessons; mirrors state/postmortem-*.jsonl; FK to trading_premortems for hypothesis-vs-result calibration.
- **`brainos-pp-cli trading-postmortems delete`** - Sentinel v1.1 Wave 2: closed-trade outcome + lessons; mirrors state/postmortem-*.jsonl; FK to trading_premortems for hypothesis-vs-result calibration.
- **`brainos-pp-cli trading-postmortems list`** - Sentinel v1.1 Wave 2: closed-trade outcome + lessons; mirrors state/postmortem-*.jsonl; FK to trading_premortems for hypothesis-vs-result calibration.
- **`brainos-pp-cli trading-postmortems update`** - Sentinel v1.1 Wave 2: closed-trade outcome + lessons; mirrors state/postmortem-*.jsonl; FK to trading_premortems for hypothesis-vs-result calibration.

### trading-premortems

Manage trading premortems

- **`brainos-pp-cli trading-premortems create`** - Sentinel v1.1 Wave 2: pre-trade hypothesis + sized-risk capture before order entry; mirrors state/premortem-*.jsonl.
- **`brainos-pp-cli trading-premortems delete`** - Sentinel v1.1 Wave 2: pre-trade hypothesis + sized-risk capture before order entry; mirrors state/premortem-*.jsonl.
- **`brainos-pp-cli trading-premortems list`** - Sentinel v1.1 Wave 2: pre-trade hypothesis + sized-risk capture before order entry; mirrors state/premortem-*.jsonl.
- **`brainos-pp-cli trading-premortems update`** - Sentinel v1.1 Wave 2: pre-trade hypothesis + sized-risk capture before order entry; mirrors state/premortem-*.jsonl.

### trading-regime-log

Manage trading regime log

- **`brainos-pp-cli trading-regime-log create`** - Sentinel v1.1 Wave 2 + Sentinel v4 (shared): daily SPX/VIX regime classification; A=trending bull, B=late-cycle, C=correction, D=bear/crisis, E=range. v4 swing writes Sunday; intraday detectors read latest.
- **`brainos-pp-cli trading-regime-log delete`** - Sentinel v1.1 Wave 2 + Sentinel v4 (shared): daily SPX/VIX regime classification; A=trending bull, B=late-cycle, C=correction, D=bear/crisis, E=range. v4 swing writes Sunday; intraday detectors read latest.
- **`brainos-pp-cli trading-regime-log list`** - Sentinel v1.1 Wave 2 + Sentinel v4 (shared): daily SPX/VIX regime classification; A=trending bull, B=late-cycle, C=correction, D=bear/crisis, E=range. v4 swing writes Sunday; intraday detectors read latest.
- **`brainos-pp-cli trading-regime-log update`** - Sentinel v1.1 Wave 2 + Sentinel v4 (shared): daily SPX/VIX regime classification; A=trending bull, B=late-cycle, C=correction, D=bear/crisis, E=range. v4 swing writes Sunday; intraday detectors read latest.

### trading-sessions

Manage trading sessions

- **`brainos-pp-cli trading-sessions create`** - Sentinel v1.1 Wave 2: per-day rollup of trades + operator state + halt status; mirrors state/session-summary-*.txt; one row per trading day.
- **`brainos-pp-cli trading-sessions delete`** - Sentinel v1.1 Wave 2: per-day rollup of trades + operator state + halt status; mirrors state/session-summary-*.txt; one row per trading day.
- **`brainos-pp-cli trading-sessions list`** - Sentinel v1.1 Wave 2: per-day rollup of trades + operator state + halt status; mirrors state/session-summary-*.txt; one row per trading day.
- **`brainos-pp-cli trading-sessions update`** - Sentinel v1.1 Wave 2: per-day rollup of trades + operator state + halt status; mirrors state/session-summary-*.txt; one row per trading day.

### webauthn-credentials

Manage webauthn credentials

- **`brainos-pp-cli webauthn-credentials create`** - Create
- **`brainos-pp-cli webauthn-credentials delete`** - Delete
- **`brainos-pp-cli webauthn-credentials list`** - List
- **`brainos-pp-cli webauthn-credentials update`** - Update


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
brainos-pp-cli active-memory list

# JSON for scripting and agents
brainos-pp-cli active-memory list --json

# Filter to specific fields
brainos-pp-cli active-memory list --json --select id,name,status

# Dry run — show the request without sending
brainos-pp-cli active-memory list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
brainos-pp-cli active-memory list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-brainos -g
```

Then invoke `/pp-brainos <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add brainos brainos-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/brainos-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "brainos": {
      "command": "brainos-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
brainos-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/brainos-pp-cli/config.toml`

| Environment Variable  | Description                                                                 |
| --------------------- | --------------------------------------------------------------------------- |
| `BRAINOS_SERVICE_KEY` | Supabase service-role key (full access). Takes precedence over `ANON_KEY`. |
| `BRAINOS_ANON_KEY`    | Supabase anon key (public schema access only).                              |
| `BRAINOS_BASE_URL`    | Override the default API base URL (default: your project's PostgREST URL). |
| `BRAINOS_CONFIG`      | Override the config file path.                                              |

Config file fields (TOML):

```toml
base_url = "https://your-project-ref.supabase.co/rest/v1"
auth_header = "your-service-role-key"

[headers]
# Optional: static headers sent with every request
x-custom-header = "value"
```

Static request headers can be configured under `[headers]`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized** — export BRAINOS_SERVICE_KEY=<your-service-role-key>
- **Empty results after sync** — brainos-pp-cli sync --full --table thoughts to force full resync
- **mcp blast-radius returns nothing** — Extend window: brainos-pp-cli mcp blast-radius --since 6h

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**supabase/cli**](https://github.com/supabase/cli) — Go (5000 stars)
- [**brain-mcp**](https://github.com/dbostwick/brain-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

# Best Food Trucks CLI

**The only Best Food Trucks client that can tell you where a truck goes, not just what's parked outside today.**

Best Food Trucks has no public API, no SDK, and no way to ask 'where else does this truck park' or 'summarize this week for my team.' This CLI talks directly to the same GraphQL backend the website and mobile apps use, then adds schedule digests, cuisine search, truck-centric reverse lookup, and cross-lot views the live site was never built to answer.

Learn more at [Best Food Trucks](https://api.bestfoodtrucks.com).

Created by [@enlewof](https://github.com/enlewof) (Allen Lew).

## Install

The recommended path installs both the `bestfoodtrucks-pp-cli` binary and the `pp-bestfoodtrucks` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install bestfoodtrucks
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install bestfoodtrucks --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install bestfoodtrucks --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install bestfoodtrucks --agent claude-code
npx -y @mvanhorn/printing-press-library install bestfoodtrucks --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/cmd/bestfoodtrucks-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bestfoodtrucks-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install bestfoodtrucks --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bestfoodtrucks --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bestfoodtrucks --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install bestfoodtrucks --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bestfoodtrucks-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/cmd/bestfoodtrucks-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bestfoodtrucks": {
      "command": "bestfoodtrucks-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirms the CLI can reach the API. No login or API key needed — the read surface is fully anonymous.
bestfoodtrucks-pp-cli doctor --dry-run

# See the full upcoming schedule for a specific lot by its URL slug.
bestfoodtrucks-pp-cli lot schedule playa-district

# Get the same schedule as ready-to-paste announcement text.
bestfoodtrucks-pp-cli lot digest playa-district

# Look up a specific scheduled truck visit's full menu, prices, and hours.
bestfoodtrucks-pp-cli shift get 179609

# Reverse-lookup: see every lot a specific truck visits, a view the website itself never shows.
bestfoodtrucks-pp-cli truck schedule 11869

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-schedule synthesis
- **`lot digest`** — Turns a lot's upcoming schedule into ready-to-paste announcement text instead of raw structured data.

  _Reach for this when the task is 'summarize this week's schedule for humans,' not 'give me structured data.'_

  ```bash
  bestfoodtrucks-pp-cli lot digest playa-district
  ```
- **`trucks find`** — Finds every upcoming shift at a lot matching a cuisine, without opening each shift page one at a time.

  _Use this when a user names a cuisine and a lot rather than a specific date — it walks the whole visible schedule window for you._

  ```bash
  bestfoodtrucks-pp-cli trucks find --cuisine Thai --lot playa-district --json
  ```
- **`lots digest`** — Combines multiple lots' schedules into one view in a single command instead of visiting each lot's page separately.

  _Use this when a user tracks more than one regular lot (e.g., office campus plus a nearby favorite) and wants one combined answer._

  ```bash
  bestfoodtrucks-pp-cli lots digest --lots playa-district,at-t-los-angeles --json
  ```

### Truck-centric views the website never built
- **`truck schedule`** — Shows every lot a specific truck visits, past and future — a view the Best Food Trucks website itself never built.

  _Use this to answer 'when does my favorite truck come back' or 'what other lots does this truck serve' — impossible from the live site's own navigation._

  ```bash
  bestfoodtrucks-pp-cli truck schedule 11869 --json --select locations.records.startTime,locations.records.lot.name
  ```
- **`market hotlist`** — Ranks trucks operating in a city by review signal, a cross-truck aggregate the site never computes.

  _Use this for 'what's the best-rated truck in this city' rather than checking trucks one at a time._

  ```bash
  bestfoodtrucks-pp-cli market hotlist los-angeles --limit 10
  ```

## Recipes

### What's at my office today

```bash
bestfoodtrucks-pp-cli lot schedule playa-district --json --select locationSchedule.dateAlias,locationSchedule.locations.truck.name
```

Pulls just today/tomorrow's truck name from the full schedule payload, instead of parsing the whole nested response by hand.

### Paste this week's schedule into Slack

```bash
bestfoodtrucks-pp-cli lot digest playa-district
```

Produces the announcement text directly — no manual copy-paste from the website required.

### Find every Thai truck coming to my lot

```bash
bestfoodtrucks-pp-cli trucks find --cuisine Thai --lot playa-district --json
```

Walks the visible schedule window and filters by cuisine tag, something no single API call does server-side.

### Track a favorite truck across every lot it visits

```bash
bestfoodtrucks-pp-cli truck schedule 11869 --agent --select locations.records.startTime,locations.records.lot.name
```

Reverse lookup with --select narrows a potentially large history-plus-future list down to just the two fields an agent needs to answer 'when and where.'

### Combine two lots you care about into one view

```bash
bestfoodtrucks-pp-cli lots digest --lots playa-district,at-t-los-angeles
```

One command instead of visiting two separate lot pages and manually merging the results.

## Usage

Run `bestfoodtrucks-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `data.db` and the local learning store. This CLI needs no Best Food Trucks credentials — its read surface is fully anonymous — so no `credentials.toml` or auth sidecar files are written here |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BESTFOODTRUCKS_CONFIG_DIR`, `BESTFOODTRUCKS_DATA_DIR`, `BESTFOODTRUCKS_STATE_DIR`, or `BESTFOODTRUCKS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BESTFOODTRUCKS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BESTFOODTRUCKS_HOME=/srv/bestfoodtrucks
bestfoodtrucks-pp-cli doctor
```

Under `BESTFOODTRUCKS_HOME=/srv/bestfoodtrucks`, the four dirs resolve to `/srv/bestfoodtrucks/config`, `/srv/bestfoodtrucks/data`, `/srv/bestfoodtrucks/state`, and `/srv/bestfoodtrucks/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "bestfoodtrucks": {
      "command": "bestfoodtrucks-pp-mcp",
      "env": {
        "BESTFOODTRUCKS_HOME": "/srv/bestfoodtrucks"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `BESTFOODTRUCKS_DATA_DIR` overrides an explicit `--home` for that kind. Use `BESTFOODTRUCKS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `BESTFOODTRUCKS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. This CLI stores no Best Food Trucks API credentials — the read surface is fully anonymous. Run `bestfoodtrucks-pp-cli doctor --fail-on warn` to check path and connectivity warnings in automation.

## Commands

### graphql

Raw GraphQL passthrough against the Best Food Trucks API. This is a generic escape-hatch endpoint; Phase 3 hand-writes typed, well-named commands (lot get/schedule/digest, shift get, truck schedule, market list/hotlist, trucks find, lots digest) on top of a hand-authored GraphQL client rather than relying on this endpoint's auto-emitted command surface. The CLI always sends full query text (no extensions.persistedQuery hash) to avoid depending on the server's Apollo Automatic Persisted Query cache.


- **`bestfoodtrucks-pp-cli graphql`** - Execute a raw, read-only GraphQL query against the Best Food Trucks API (advanced escape hatch; mutations such as ordering, subscribing, or checkout are out of scope for this CLI and require a customer login this build does not implement)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`bestfoodtrucks-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`bestfoodtrucks-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`bestfoodtrucks-pp-cli learnings list`** - Inspect taught rows
- **`bestfoodtrucks-pp-cli learnings forget <query>`** - Undo a teach
- **`bestfoodtrucks-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`bestfoodtrucks-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`bestfoodtrucks-pp-cli teach-pattern`** - Install a query/resource template up front
- **`bestfoodtrucks-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `BESTFOODTRUCKS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `bestfoodtrucks-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bestfoodtrucks-pp-cli graphql --query example-value

# JSON for scripting and agents
bestfoodtrucks-pp-cli graphql --query example-value --json
# Filter to specific fields by name (example uses a real curated command + real field path)
bestfoodtrucks-pp-cli lot schedule playa-district --json --select locationSchedule.dateAlias,locationSchedule.locations.truck.name

# Dry run — show the request without sending
bestfoodtrucks-pp-cli graphql --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bestfoodtrucks-pp-cli graphql --query example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by every curated command** - every command in "Unique Features" and every absorbed lot/shift/truck/market command above is a read-only lookup. The generic framework `import`/`graphql` commands can technically issue writes if deliberately misused with a mutation query, but no shipped feature does so, and the Best Food Trucks API requires a customer login this build does not implement for any real write (subscribe, order-ahead, checkout)
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
bestfoodtrucks-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `bestfoodtrucks-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `BESTFOODTRUCKS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **lot get returns 'not found' for a slug you copied from a URL** — Use market list <city> to find the correct seoName slug — some lots have display names that differ from their URL slug.
- **shift get returns an empty menu** — The shift may not have started publishing its menu yet (workStatusHuman: 'Not Started'); menus typically populate closer to the shift's start time.
- **market hotlist returns no ranking** — The market has zero trucks on record, or every truck lookup in the fan-out failed — check the fetch_failures field in --json output for per-truck error detail.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.bestfoodtrucks.com/lots/playa-district
- Capture coverage: 25 API entries from 196 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: graphql (92% confidence), graphql_persisted_query (90% confidence), rest_json (75% confidence)
- Protection signals: cloudflare (90% confidence)
- Generation hints: browser_http_transport, graphql_persisted_query, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_graphql — Derived from observed POST /graphql traffic.; create_track_referrer.json — Derived from observed POST /track-referrer.json traffic.; create_track_request — Derived from observed POST /api/v1/intent_pixel/track_request traffic.; head_schedule.json — Derived from observed HEAD /_next/data/-dh7Qe2PYMUxWzoOQbyo3/lots/playa-district/schedule.json traffic.; list_179609_playa_district_on_2026_08_26.json — Derived from observed GET /_next/data/-dh7Qe2PYMUxWzoOQbyo3/shifts/179609-playa-district-on-2026-08-26.json traffic.; list_attribution_trigger — Derived from observed GET /attribution_trigger traffic.; list_can_track_visitor — Derived from observed GET /api/v1/intent_pixel/can_track_visitor traffic.; list_j — Derived from observed GET /j traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

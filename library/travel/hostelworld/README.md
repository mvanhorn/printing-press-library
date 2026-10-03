# Hostelworld CLI

**Compare dated dorm beds and private rooms with explicit quantities, prices and terms.**

Resolve destinations, inspect property rules and fetch actual room plans for your dates. Source prices retain their per-bed or per-room basis; party totals are explicitly derived estimates.

Learn more at [Hostelworld](https://prod.apigee.hostelworld.com).

Created by [@zjsng](https://github.com/zjsng) (zjsng).

## Install

The recommended path installs both the `hostelworld-pp-cli` binary and the `pp-hostelworld` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install hostelworld
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install hostelworld --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install hostelworld --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install hostelworld --agent claude-code
npx -y @mvanhorn/printing-press-library install hostelworld --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hostelworld/cmd/hostelworld-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hostelworld-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install hostelworld --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hostelworld --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hostelworld --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install hostelworld --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hostelworld-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hostelworld/cmd/hostelworld-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hostelworld": {
      "command": "hostelworld-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account, cookies or personal API key required. Search fetches Hostelworld’s public anonymous application identifier from its current first-party config into memory; it is never stored or hardcoded. Property and availability requests are public. Changed config fails clearly.

## Quick Start

```bash
# Check command readiness.
hostelworld-pp-cli doctor --dry-run

# Resolve the source city ID.
hostelworld-pp-cli destinations search Tokyo --agent

# Find a bounded dated shortlist.
hostelworld-pp-cli hostels search --city-id 452 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --limit 5 --agent

# Inspect actual bed-level plans.
hostelworld-pp-cli hostels offers 67481 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --agent

```

## Recipes

### Find a destination

```bash
hostelworld-pp-cli destinations search Osaka --agent
```

Keep the exact city ID and distinguish similarly named cities.

### Inspect source rules

```bash
hostelworld-pp-cli hostels inspect 67481 --agent --select id,name,city,check_in,check_out,policies
```

Read source check-in and tax facts without verbose descriptions.

### Compare bed and room costs

```bash
hostelworld-pp-cli hostels compare 67481 15725 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --agent
```

Compare the same dates and guests, keeping source units and derived quantities visible.

### Check alternative dates

```bash
hostelworld-pp-cli hostels dates 67481 --starts 2026-11-10,2026-11-11 --nights 3 --guests 2 --agent
```

Each date window is independently requested.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Dated hostel decisions
- **`hostels compare`** — Compare actual dated dorm beds and private rooms with explicit source units.

  _Compare actual dated dorm beds and private rooms with explicit source units._

  ```bash
  hostelworld-pp-cli hostels compare 67481 15725 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --agent
  ```
- **`hostels dates`** — Check actual availability for each alternative check-in date.

  _Check actual availability for each alternative check-in date._

  ```bash
  hostelworld-pp-cli hostels dates 67481 --starts 2026-11-10,2026-11-11 --nights 3 --guests 2 --agent
  ```
- **`hostels offers`** — Keep nightly bed, room and occupancy-slot price bases explicit.

  _Keep nightly bed, room and occupancy-slot price bases explicit._

  ```bash
  hostelworld-pp-cli hostels offers 15725 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --agent
  ```
- **`hostels offers`** — See rate terms and exact cancellation deadline alongside the price.

  _See rate terms and exact cancellation deadline alongside the price._

  ```bash
  hostelworld-pp-cli hostels offers 67481 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --agent
  ```

### Saved evidence
- **`hostels saved`** — Re-read your bounded planning snapshots offline.

  _Re-read your bounded planning snapshots offline._

  ```bash
  hostelworld-pp-cli hostels saved --agent
  ```

## Usage

Run `hostelworld-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HOSTELWORLD_CONFIG_DIR`, `HOSTELWORLD_DATA_DIR`, `HOSTELWORLD_STATE_DIR`, or `HOSTELWORLD_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HOSTELWORLD_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HOSTELWORLD_HOME=/srv/hostelworld
hostelworld-pp-cli doctor
```

Under `HOSTELWORLD_HOME=/srv/hostelworld`, the four dirs resolve to `/srv/hostelworld/config`, `/srv/hostelworld/data`, `/srv/hostelworld/state`, and `/srv/hostelworld/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "hostelworld": {
      "command": "hostelworld-pp-mcp",
      "env": {
        "HOSTELWORLD_HOME": "/srv/hostelworld"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `HOSTELWORLD_DATA_DIR` overrides an explicit `--home` for that kind. Use `HOSTELWORLD_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HOSTELWORLD_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `hostelworld-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### source

Observed read-only Hostelworld source contracts

- **`hostelworld-pp-cli source availability`** - Dated per-bed/per-room availability; source totals retain their units
- **`hostelworld-pp-cli source city-search`** - Dated bounded city discovery; summary from-prices are not room quotes
- **`hostelworld-pp-cli source locations`** - Public destination suggestions; application identifier bootstrapped transiently
- **`hostelworld-pp-cli source property`** - Public property facts and source rules


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`hostelworld-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`hostelworld-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`hostelworld-pp-cli learnings list`** - Inspect taught rows
- **`hostelworld-pp-cli learnings forget <query>`** - Undo a teach
- **`hostelworld-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`hostelworld-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`hostelworld-pp-cli teach-pattern`** - Install a query/resource template up front
- **`hostelworld-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `HOSTELWORLD_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `hostelworld-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hostelworld-pp-cli hostels offers 67481 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2

# JSON for scripting and agents
hostelworld-pp-cli hostels offers 67481 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --json
# Filter to specific fields
hostelworld-pp-cli hostels offers 67481 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --json --select property_id,offers.source_stay_amount,offers.free_cancellation_status

# Dry run — show the request without sending
hostelworld-pp-cli hostels offers 67481 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hostelworld-pp-cli hostels offers 67481 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - hostels saved and local search/SQL can read explicitly saved planning evidence; generic provider sync is unsupported
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
hostelworld-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `hostelworld-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/hostelworld-pp-cli/config.toml`; `--home`, `HOSTELWORLD_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use `destinations search Tokyo` to resolve a city, then `hostels search --city-id 452 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2` to discover property IDs

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://prod.apigee.hostelworld.com/autocomplete-service/v1/autocomplete/web
- Capture coverage: 4 API entries from 4 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Candidate command ideas: get_availability — Derived from observed GET /legacy-hwapi-service/2.2/properties/{property_id}/availability/ traffic.; get_properties — Derived from observed GET /legacy-hwapi-service/2.2/cities/{city_id}/properties/ traffic.; list_web — Derived from observed GET /autocomplete-service/v1/autocomplete/web traffic.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**trvl**](https://github.com/MikkoParkkola/trvl) — Go
- [**Hostelworld-Finder**](https://github.com/NavyTitanium/Hostelworld-Finder) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Price units and limits

`hostels offers` retains the source stay amount and average nightly price separately from any derived party estimate. Dorm prices are per bed. Private stay totals are per entire room, even when your party uses fewer occupancy slots. A private nightly breakdown can be per occupancy slot: the CLI checks the sum against the room stay total before identifying its basis. An unknown basis remains explicit. Required quantities and available beds/rooms remain visible; a derived party estimate is not a checkout quote.

Source cancellation availability, deadline/timezone, deposit percentage, payment text and rate rules are reported together. Some source payment descriptions describe a different booking product from the free-cancellation text; confirm the selected rate at the handoff. Source gender labels and property age/group rules do not establish eligibility or guarantee that dorm beds are allocated to one room.

City search fetches one explicit page and preserves source ranking, which may include promotions. Its dated summary prices are from-prices; fetch actual room plans before comparing party costs. Compare accepts at most five properties; dates accepts at most five explicit starts and requests every window separately. Stays are bounded to 30 nights and ten guests, subject to stricter property rules. Network bodies are bounded, source errors stay distinct from no matching offers, and `--timeout` bounds planning commands.

`--save` stores only normalized planning evidence in the local SQLite cache, capped at 200 snapshots. `hostels saved` works offline and marks dated prices stale. Re-run the live command to refresh. No booking, payment, account, chat, contributor-profile or full-review workflow is exposed. Public property responses are pruned before generic CLI/MCP output and caching.

Search obtains the anonymous site's public application identifier from one known first-party configuration literal. The identifier remains in memory and is never hardcoded, persisted, printed, or exposed in upstream error bodies. Changed configuration fails clearly; fetched JavaScript is never executed. No user cookies or login are imported.

Generic provider `sync` is hidden from normal help and MCP. Calling it explicitly initializes only the local SQLite cache and returns `local_cache_only` with `provider_snapshot_refreshed: false`. The local population path is `hostels inspect/offers --save`; `hostels saved` reports stale saved observations.

City `--kind` and cancellation filters apply to the fetched source page; a filtered empty page does not mean the city is sold out. MCP `hostels_compare` accepts distinct numeric `id` and `id2`, with optional `id3`, `id4`, and `id5`, plus explicit stay flags.

### Planning terms and saved observations

The availability-wide free-cancellation signal does not prove that every rate is refundable. Deposit-only terms that require selecting an optional flexible booking are `conditional`; the free-cancellation filter excludes conditional and unknown plans. Output preserves the source-wide signal and deadline scope separately from rate-level evidence.

Manual saved observations and their search index retain at most 200 entries within the verified client profile. Check-in dates are source-local calendar dates; the provider decides same-day acceptance when the destination timezone is not yet known.

Cached reads fail with `cache_visibility_unavailable` while a WAL or rollback journal exists; close other cache writers, then retry, or use live planning without `--save`. The source guard resolves symlinks, rejects ambiguous hard links and SQLite URI metacharacters, and checks selected-file identity. Reads use an automatically deleted private snapshot (0700 directory,0600 file,512MiB maximum) from a verified descriptor so a delayed SQL open cannot select a replacement file. Saves use ordinary SQLite transactions on the canonical filename with one connection, reject ambiguous aliases before preflight, and check detected path retargets around the write. Cooperating SQLite writers are supported; cached reads reject while a WAL exists; retry after writer close. External renaming, unlinking, or replacing an in-use database during a write is unsupported, as documented by [SQLite sections2.5–2.6](https://www.sqlite.org/howtocorrupt.html). No custom overwrite or alternative locking protocol is used.

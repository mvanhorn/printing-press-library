# Sunflower Ferry CLI

**Plan dated Sunflower ferry sailings, cabin fares and port access from official sources.**

Query the anonymous fare simulator, compare seasonal calendar dates and retain exact overnight arrivals. Discover cabins and terminal access, then use the official booking handoff.

Learn more at [Sunflower Ferry](https://www.ferry-sunflower.co.jp).

## Planning commands and source coverage

This build covers Osaka–Beppu, Kobe–Oita and Osaka–Shibushi in both directions. Osaka Terminal 1 serves Beppu; Terminal 2 serves Shibushi. Oarai–Tomakomai uses a separate booking system and is outside this CLI's verified scope.

| Command | Result |
|---|---|
| `routes list`, `routes search Beppu` | Offline route, Japanese port name and source direction-code reference |
| `routes show --route osaka-shibushi --direction inbound` | Published normal weekday timetable, including source rowspan and next-morning rules |
| `sailings --route osaka-beppu --date 2026-10-15` | Official date lookup: ship and precise departure/arrival timestamps in JST |
| `calendar --route osaka-beppu --from 2026-10-01 --to 2026-10-31` | Explicit fare bands, published date coverage, missing dates and discount conditions |
| `quote --route kobe-oita --date 2026-10-15 --adults 2 --children 1 --mode car --car-category lt5m` | Source cabin price matrix for that entered one-way party and vehicle category |
| `cabins --route osaka-shibushi` | Source occupancy and room type; shared-room capacity retains the source wording |
| `ports --route osaka-beppu` | Distinct departure/arrival terminal addresses and published access guidance |
| `conditions` | Common passenger cancellation, change, baggage, school-stage and vehicle conditions |
| `handoff --route kobe-oita --date 2026-10-15` | Canonical booking URL and planning context; the page is not prefilled |

Replace example dates with a future Japan boarding date within the source's three-month booking window. `--direction outbound` means the route's first named port to its second; `inbound` reverses it. Source line IDs `11/12`, `21/22`, and `31/32` already select a direction. Calendar band **E** is a special daytime cruise; ordinary overnight timetable rules must not be projected onto it.

Quotes replay the official anonymous form in a fresh, in-memory session and stop at its fare table. Prices are the source's displayed values for the entered party/category; no passenger, room or vehicle arithmetic is invented. Fuel adjustment, taxes and fees are not itemized there. Availability symbols are observed snapshots and do not hold or guarantee a cabin or vehicle. Intro marketing prices dated July 2025 are excluded. No account, reservation, standby, cancellation or payment operation is provided.

The [official FAQ](https://www.ferry-sunflower.co.jp/faq/) lists daily booking-system maintenance from 03:00 to 05:00 JST. During source maintenance, `quote` and `sailings` return an explicit error; inventory remains unknown. Try the read-only lookup again after the published window and confirm actual recovery.

`foot`, `car`, and `bike` modes are supported. Car categories are strictly less than 3/4/5/6 metres. Bike categories are `over750cc`, `le750cc`, `le125cc`, and `bicycle`; specify `--bikes` and no more vehicles than adults. The source limits online parties to 14 adults plus children, one passenger car/room and up to two medium pet cages. Trucks, unsupported dimensions, special discounts, room charges, roundtrip/Dangan and other phone-only cases require the reservation center. An absent room does not establish whether it is ineligible or sold out.

Planning commands emit bounded JSON by default, including `data`, `meta.observed_at`, canonical source URLs, request count and warnings. Under `--agent`, the generated agent wrapper puts this document inside `results`. For example, `--agent --select data.quote.sailings` narrows a quote. The command deadline honors `--timeout` and is capped at 45 seconds; each typed planning response is capped at 2 MiB and each typed planning command at 10 HTTP requests including redirects. Calendar scans at most 93 inclusive dates; cabin output is capped at 40 rows per sailing. 429 responses remain errors. Sessions, cookies and anti-forgery values are never saved or emitted.

The English guide recommends reaching the terminal 60 minutes before departure and warns boarding may be refused within 30 minutes; confirm seasonal/vehicle instructions. Its cancellation summary omits the day-before band: the linked Japanese common passenger conditions supply the normal-ticket band and minimum fee. Baggage's 30 kg per-item definition differs from the 20 kg aggregate free allowance. Access times/fares are undated published guidance, so confirm current ground transport. Agency, campaign and vehicle ticket conditions can differ.

For this local build, run `go build -o sunflower-ferry-pp-cli ./cmd/sunflower-ferry-pp-cli`. Public installer instructions below apply after the separate publication step.

## Install

The recommended path installs both the `sunflower-ferry-pp-cli` binary and the `pp-sunflower-ferry` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sunflower-ferry
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sunflower-ferry --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sunflower-ferry --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sunflower-ferry --agent claude-code
npx -y @mvanhorn/printing-press-library install sunflower-ferry --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/cmd/sunflower-ferry-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sunflower-ferry-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine sunflower-ferry-pp-cli`. On Unix, mark it executable: `chmod +x sunflower-ferry-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sunflower-ferry --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sunflower-ferry --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sunflower-ferry --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sunflower-ferry --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sunflower-ferry-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install both binaries and configure MCP manually. The planning tools execute the companion CLI, so both must be discoverable on PATH.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/cmd/sunflower-ferry-pp-cli@latest
go install github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/cmd/sunflower-ferry-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sunflower-ferry": {
      "command": "sunflower-ferry-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public-source commands need no API key or login. Fare searches create an ephemeral anonymous session and stop at the fare/availability table.

## Quick Start

```bash
# Preview the doctor action without network.
sunflower-ferry-pp-cli doctor --dry-run

# Discover stable route and port identifiers.
sunflower-ferry-pp-cli routes list --agent

# Compare published fare seasons.
sunflower-ferry-pp-cli calendar --route osaka-beppu --from 2026-10-15 --to 2026-10-17 --agent

# Read source cabin fares without selecting a room.
sunflower-ferry-pp-cli quote --route osaka-beppu --date 2026-10-15 --adults 1 --mode foot --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Official ferry planning
- **`sailings`** — Turns source boarding-date results into dated JST arrival; handles daytime E cruises without projecting ordinary timetables.

  _Turns source boarding-date results into dated JST arrival; handles daytime E cruises without projecting ordinary timetables._

  ```bash
  sunflower-ferry-pp-cli sailings --route osaka-beppu --date 2026-10-15 --agent
  ```
- **`quote`** — Presents every source-eligible cabin fare for the entered party/vehicle without hand-made fare arithmetic.

  _Presents every source-eligible cabin fare for the entered party/vehicle without hand-made fare arithmetic._

  ```bash
  sunflower-ferry-pp-cli quote --route osaka-beppu --date 2026-10-15 --agent
  ```
- **`calendar`** — Filters bounded published date ranges with explicit coverage and special sailing warnings.

  _Filters bounded published date ranges with explicit coverage and special sailing warnings._

  ```bash
  sunflower-ferry-pp-cli calendar --route osaka-beppu --agent
  ```
- **`cabins`** — Extracts source room occupancy so agents can distinguish dormitory capacity from private room occupancy.

  _Extracts source room occupancy so agents can distinguish dormitory capacity from private room occupancy._

  ```bash
  sunflower-ferry-pp-cli cabins --route osaka-beppu --agent
  ```
- **`ports`** — Keeps Osaka Terminal1 and Terminal2 distinct with concise source address/access/checkin evidence.

  _Keeps Osaka Terminal1 and Terminal2 distinct with concise source address/access/checkin evidence._

  ```bash
  sunflower-ferry-pp-cli ports --route osaka-beppu --agent
  ```
- **`conditions`** — Preserves source cancellation/baggage and quote assumptions with explicit missing English day-before band.

  _Preserves source cancellation/baggage and quote assumptions with explicit missing English day-before band._

  ```bash
  sunflower-ferry-pp-cli conditions --agent
  ```

## Recipes

### Fare season window

```bash
sunflower-ferry-pp-cli calendar --route osaka-beppu --from 2026-10-15 --to 2026-10-17 --agent --select data
```

Keep only published seasonal dates.

### Overnight arrival

```bash
sunflower-ferry-pp-cli sailings --route osaka-beppu --date 2026-10-15 --agent
```

Read the actual source sailing for an exact boarding date.

### Vehicle party quote

```bash
sunflower-ferry-pp-cli quote --route kobe-oita --date 2026-10-15 --adults 2 --children 1 --mode car --car-category lt5m --agent
```

Return the source total per eligible cabin for this input party.

## Usage

Run `sunflower-ferry-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SUNFLOWER_FERRY_CONFIG_DIR`, `SUNFLOWER_FERRY_DATA_DIR`, `SUNFLOWER_FERRY_STATE_DIR`, or `SUNFLOWER_FERRY_CACHE_DIR`
2. `--home /tmp/sunflower-ferry-example` for this invocation
3. `SUNFLOWER_FERRY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SUNFLOWER_FERRY_HOME=/srv/sunflower-ferry
sunflower-ferry-pp-cli doctor
```

Under `SUNFLOWER_FERRY_HOME=/srv/sunflower-ferry`, the four dirs resolve to `/srv/sunflower-ferry/config`, `/srv/sunflower-ferry/data`, `/srv/sunflower-ferry/state`, and `/srv/sunflower-ferry/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "sunflower-ferry": {
      "command": "sunflower-ferry-pp-mcp",
      "env": {
        "SUNFLOWER_FERRY_HOME": "/srv/sunflower-ferry"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SUNFLOWER_FERRY_DATA_DIR` overrides an explicit `--home` for that kind. Use `SUNFLOWER_FERRY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SUNFLOWER_FERRY_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `sunflower-ferry-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### source

Official public source metadata; use the planning commands for structured facts

- **`sunflower-ferry-pp-cli source cabins`** - Read the official cabin description page
- **`sunflower-ferry-pp-cli source conditions`** - Read the official English boarding and cancellation policy
- **`sunflower-ferry-pp-cli source fares`** - Read the official fare and seasonal-calendar page
- **`sunflower-ferry-pp-cli source terminal`** - Read the official port access page
- **`sunflower-ferry-pp-cli source timetable`** - Read the official published weekday timetable page


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`sunflower-ferry-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`sunflower-ferry-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`sunflower-ferry-pp-cli learnings list`** - Inspect taught rows
- **`sunflower-ferry-pp-cli learnings forget <query>`** - Undo a teach
- **`sunflower-ferry-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`sunflower-ferry-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`sunflower-ferry-pp-cli teach-pattern`** - Install a query/resource template up front
- **`sunflower-ferry-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SUNFLOWER_FERRY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `sunflower-ferry-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Bounded planning JSON
sunflower-ferry-pp-cli cabins --route osaka-beppu

# JSON for scripting and agents
sunflower-ferry-pp-cli cabins --route osaka-beppu --json
# Filter to specific fields by name
sunflower-ferry-pp-cli cabins --route osaka-beppu --json --select data.cabins

# Dry run — show the request without sending
sunflower-ferry-pp-cli cabins --route osaka-beppu --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sunflower-ferry-pp-cli cabins --route osaka-beppu --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select data.cabins` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline reference** - routes list/search use the local verified registry; dynamic planning commands require live sources.
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
sunflower-ferry-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `sunflower-ferry-pp-cli doctor` to see the resolved config, data, state, and cache directories. `--home`, `SUNFLOWER_FERRY_HOME`, and per-kind env vars can relocate the resolved paths. This no-auth CLI requires no token or saved session.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://booking.ferry-sunflower.co.jp/web/yoyaku/Reserve0000/IndexEnglish
- Capture coverage: 0 API entries from 3 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: html_scrape (55% confidence)
- Auth signals: none

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Known Gaps

The typed ferry planning commands and their MCP command mirrors are the verified interface. Generated low-level `source_*` MCP tools return raw HTML instead of the page metadata emitted by matching `source` CLI commands. Their generic generated HTTP client also lacks the ferry client's 2 MiB ordinary-body cap. Use `routes`, `calendar`, `sailings`, `quote`, `cabins`, `ports`, `conditions` and `handoff` for bounded planning facts. These two generator issues are recorded in the independent review as Printing Press retro candidates; no ferry booking feature depends on them.

The mandatory gosec scan also reports 30 findings in shared generated framework code, including optional HTTP MCP server and local tooling helpers. No finding touches the native ferry client or its planning handlers. The proof ledger records their individual dispositions as generator retro candidates; this is not represented as a clean whole-repository security scan.

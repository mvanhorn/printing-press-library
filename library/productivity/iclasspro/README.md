# iClassPro CLI

**The only iClassPro tool that remembers yesterday — a queryable local mirror of any gym's catalog, with the search and filter surface the portal keeps to itself.**

iClassPro powers thousands of gymnastics, swim, dance, and cheer businesses and ships no developer API at all. This CLI treats the portal's own endpoints as one: it works against any account by its portal slug, keeps every sync in local SQLite, and answers the questions the upstream API structurally cannot — what changed since last week (drift), how fast a class is filling (fill-rate), and what registration opens next (opens-soon). It also exposes the filters the portal UI uses but nobody documented, including free-text search and openings-only.

## Install

The recommended path installs both the `iclasspro-pp-cli` binary and the `pp-iclasspro` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install iclasspro
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install iclasspro --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install iclasspro --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install iclasspro --agent claude-code
npx -y @mvanhorn/printing-press-library install iclasspro --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/cmd/iclasspro-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/iclasspro-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install iclasspro --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-iclasspro --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-iclasspro --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install iclasspro --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/iclasspro-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/cmd/iclasspro-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "iclasspro": {
      "command": "iclasspro-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Most accounts need no credentials for public catalog reads. Customer-gated catalogs use 'auth login'. If a stored customer token expires, the CLI retries the same read anonymously so an optional stale login cannot break a public catalog; a genuinely gated catalog still reports that a fresh login is required. Staff-side reads use a separate Office Portal session created by 'auth staff-login' from ICLASSPRO_STAFF_USERNAME and ICLASSPRO_STAFF_PASSWORD. Credentials are never accepted as flags, passwords are never persisted, and only server-issued session material is stored in the private 0600 session file. The admin surface is an explicit read-only allow-list and does not cache Office Portal responses.

## Quick Start

```bash
# Confirm the binary, config path, and local database are wired before touching the network
iclasspro-pp-cli doctor --dry-run

# Find out whether this account is open, sign-in-gated, or plan-gated before assuming an empty result means empty
iclasspro-pp-cli tenant scottsdalegymnastics

# Every other command needs a location id, and this is where they come from
iclasspro-pp-cli locations scottsdalegymnastics

# Free-text search plus openings-only, both honored server-side
iclasspro-pp-cli classes list scottsdalegymnastics --q ninja --openings 1

# Populate the local mirror; drift and fill-rate need at least two syncs before they can say anything
iclasspro-pp-cli sync scottsdalegymnastics --resources classes,camps

# Registration opening or closing in the next two weeks, which the portal never shows
iclasspro-pp-cli opens-soon scottsdalegymnastics --days 14

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local history the API forgets
- **`watch`** — Get told the moment a spot frees up in a class or camp that is currently full.

  _Reach for this instead of re-polling a list endpoint yourself: it stores every observation, so it reports the transition rather than the current value._

  ```bash
  iclasspro-pp-cli watch scottsdalegymnastics --class 8357 --agent
  ```
- **`drift`** — See what changed between syncs: classes and camps added, removed, retimed, or newly marked deleted.

  _Use this to answer 'what is different since last week' without re-reading the whole catalog into context._

  ```bash
  iclasspro-pp-cli drift scottsdalegymnastics --since 7d --agent
  ```
- **`opens-soon`** — Find registration that has not opened yet or is about to close, across every synced class and camp.

  _This is the only way to see a camp before its registration opens, which is when popular sessions are still winnable._

  ```bash
  iclasspro-pp-cli opens-soon scottsdalegymnastics --days 14 --agent
  ```
- **`fill-rate`** — Show how fast classes are filling over time, by class or by program.

  _Answers whether a class is trending toward full or toward cancellation, which no upstream call can express._

  ```bash
  iclasspro-pp-cli fill-rate scottsdalegymnastics --programs 589 --agent
  ```

### Catalog hygiene and publishing
- **`calendar`** — Export synced camps and classes to an RFC 5545 calendar file, one event per session.

  _Turns a portal catalog into something a calendar app, a website, or a newsletter can consume directly._

  ```bash
  iclasspro-pp-cli calendar scottsdalegymnastics --format ics --out fall-camps.ics
  ```
- **`lint`** — Flag catalog quality problems: missing descriptions or images, expired registration windows, deleted-but-listed programs.

  _Use before publishing a schedule to a website; it catches the records that will render blank or dead._

  ```bash
  iclasspro-pp-cli lint scottsdalegymnastics --agent
  ```

### Multi-tenant reach
- **Expired customer-session fallback** — Retry the anonymous Open API when a stored customer JWT returns HTTP 401.

  _A login that was once needed cannot later take down a catalog the account publishes publicly. Sign-in-gated accounts still surface their login requirement._

  ```bash
  iclasspro-pp-cli locations examplegym --agent
  ```
- **`tenant`** — Report which surfaces an account actually exposes: open, sign-in-gated, or plan-gated.

  _Run this first against any new account; it prevents mistaking a sign-in gate for a gym with no classes._

  ```bash
  iclasspro-pp-cli tenant examplegym --agent
  ```
- **`compare`** — Compare the same kind of program across several gyms at once.

  _Built for franchise and agency operators who track many gyms and need one table instead of many tabs._

  ```bash
  iclasspro-pp-cli compare --accounts scottsdalegymnastics,oasisgymnastics,tigar --agent
  ```

### Read-only Office Portal access
- **`admin`** — Read an explicit allow-list of authenticated staff resources without mutation, export, or response caching.

  _Provides staff-side visibility without giving an agent a generic request escape hatch or any write capability. Attendance reads discover the internal timeslot from the class and date when it is unambiguous._

  ```bash
  iclasspro-pp-cli admin families examplegym --q smith --limit 25 --agent
  iclasspro-pp-cli admin attendance examplegym 12345 2026-08-12 --agent
  ```

## Recipes

### Keep public reads working after a customer session expires

```bash
iclasspro-pp-cli locations examplegym --agent
```

The CLI tries a stored customer session first. If iClassPro rejects that JWT with HTTP 401, it automatically retries the same read through the anonymous Open API. Public catalogs continue normally; gated catalogs return their sign-in message so you can run `auth login` again.

### Narrow a large class list down to just what an agent needs

```bash
iclasspro-pp-cli classes list oasisgymnastics --openings 1 --limit 50 --agent --select id,name,openings,allowWaitlist,schedule.dayName,schedule.startTime
```

Some gyms return well over a hundred classes with nested schedule arrays; selecting dotted paths keeps the payload small enough to reason over without paging through raw JSON.

### Resolve camp type ids the correct way, then list that type

```bash
iclasspro-pp-cli bookings scottsdalegymnastics 1 --agent --select title,target,targetParams.typeId
```

The booking menu is the only authoritative source of camp type ids; feeding a programId from 'programs camps' into --type-id silently returns nothing.

### Publish a gym's camp schedule to a calendar file

```bash
iclasspro-pp-cli calendar scottsdalegymnastics --format ics --out fall-camps.ics
```

Builds one event per camp session from the block and schedule arrays, with the portal registration link attached to each event.

### Check several gyms for openings in one pass

```bash
iclasspro-pp-cli compare --accounts scottsdalegymnastics,oasisgymnastics,tigar --agent
```

Joins the synced copies of each account locally, which is the only way to line up programs whose names and ids differ per gym.

### Audit a catalog before pushing it to a client website

```bash
iclasspro-pp-cli lint scottsdalegymnastics --agent
```

Flags the records that will render blank or dead on a site: missing descriptions, missing images, expired registration windows, and programs already marked deleted upstream.

### Establish a staff session without putting credentials in shell history

```bash
ICLASSPRO_STAFF_USERNAME=staff-user ICLASSPRO_STAFF_PASSWORD='...' iclasspro-pp-cli auth staff-login examplegym
```

The password is used only for login and is never persisted. Verify with 'auth staff-status'; status output never exposes the cookie.

### Record an authoritative catalog snapshot

```bash
iclasspro-pp-cli sync examplegym --resources classes,camps --agent
```

Only a complete classes-and-camps walk replaces the snapshot used by `drift`. A classes-only or camps-only run still updates openings history and the search cache, but it cannot make the omitted resource type look deleted.

### Read staff-side operational data

```bash
iclasspro-pp-cli admin families examplegym --q smith --limit 25 --agent
```

The admin group covers dashboard, families, students, class search, enrollments, attendance, transactions, and report definitions through an explicit read-only endpoint allow-list.

### Read attendance without hunting for an internal timeslot ID

```bash
iclasspro-pp-cli admin attendance examplegym 12345 2026-08-12 --agent
```

The CLI resolves the date's unique `tsId` through the Office Portal schedule endpoint before reading the roster. If a class has multiple events that day, the error lists their non-sensitive timeslot IDs so you can pass the intended one as the optional fourth argument.

## Usage

Run `iclasspro-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ICLASSPRO_CONFIG_DIR`, `ICLASSPRO_DATA_DIR`, `ICLASSPRO_STATE_DIR`, or `ICLASSPRO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ICLASSPRO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ICLASSPRO_HOME=/srv/iclasspro
iclasspro-pp-cli doctor
```

Under `ICLASSPRO_HOME=/srv/iclasspro`, the four dirs resolve to `/srv/iclasspro/config`, `/srv/iclasspro/data`, `/srv/iclasspro/state`, and `/srv/iclasspro/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "iclasspro": {
      "command": "iclasspro-pp-mcp",
      "env": {
        "ICLASSPRO_HOME": "/srv/iclasspro"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ICLASSPRO_DATA_DIR` overrides an explicit `--home` for that kind. Use `ICLASSPRO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ICLASSPRO_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `iclasspro-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### bookings

The portal booking menu — the authoritative source of camp typeIds

- **`iclasspro-pp-cli bookings <account> <locationId>`** - Booking menu tiles for a location; camp tiles carry the typeId that 'camps list' requires

### camps

Camps and events — open gyms, clinics, kids night out, school-break camps

- **`iclasspro-pp-cli camps get`** - Full camp detail including HTML description, per-session blocks, room, instructors, and deletion/expiry flags
- **`iclasspro-pp-cli camps list`** - List camps for one camp type. typeId comes from 'bookings', NOT from 'programs camps'

### classes

Ongoing classes — the primary catalog

- **`iclasspro-pp-cli classes get`** - Full class detail including the HTML description the list endpoint omits
- **`iclasspro-pp-cli classes list`** - List classes with server-side filtering. Only the flags below are honored upstream; any other filter is applied locally

### instructors

Instructors teaching classes at a location

- **`iclasspro-pp-cli instructors <account> <locationId>`** - Instructors who teach classes; ids are valid values for 'classes list --instructors'

### levels

Skill levels used to band classes

- **`iclasspro-pp-cli levels <account> <locationId>`** - Active skill levels with display colors; ids are valid values for 'classes list --levels'

### locations

Physical locations for an account

- **`iclasspro-pp-cli locations <account>`** - List every location on an iClassPro account, with contact details and portal branding

### news

Portal news articles

- **`iclasspro-pp-cli news <account> <articleId>`** - Fetch a single portal news article by id

### parties

Birthday party booking availability

- **`iclasspro-pp-cli parties <account> <locationId>`** - Dates a party can be booked at a location

### products

ProShop retail catalog

- **`iclasspro-pp-cli products <account> <locationId>`** - Retail products with pricing, sale state, variations, and inventory

### programs

Program categories for classes, camps, and appointments

- **`iclasspro-pp-cli programs appointments`** - Appointment program categories; returns a plan-gate message on accounts without the appointments subscription
- **`iclasspro-pp-cli programs camps`** - Camp program categories. These ids are programIds, NOT the typeIds 'camps list' needs — use 'bookings' for those
- **`iclasspro-pp-cli programs classes`** - Class program categories; ids are valid values for 'classes list --programs'

### sessions

Enrollment sessions (date-bounded terms)

- **`iclasspro-pp-cli sessions <account>`** - Sessions for an account; ids are valid values for 'classes list --sessions'


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`iclasspro-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`iclasspro-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`iclasspro-pp-cli learnings list`** - Inspect taught rows
- **`iclasspro-pp-cli learnings forget <query>`** - Undo a teach
- **`iclasspro-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`iclasspro-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`iclasspro-pp-cli teach-pattern`** - Install a query/resource template up front
- **`iclasspro-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ICLASSPRO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `iclasspro-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
iclasspro-pp-cli bookings mock-value mock-value

# JSON for scripting and agents
iclasspro-pp-cli bookings mock-value mock-value --json

# Filter to specific fields
iclasspro-pp-cli bookings mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
iclasspro-pp-cli bookings mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
iclasspro-pp-cli bookings mock-value mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
iclasspro-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `iclasspro-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/iclasspro-pp-cli/config.toml`; `--home`, `ICLASSPRO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **classes list returns an empty array but the gym clearly has classes** — Run 'iclasspro-pp-cli tenant <slug>' — the account probably hides its catalog behind customer sign-in, which the API reports as HTTP 200 with an empty list.
- **A filter flag seems to do nothing and the result count never changes** — Only --q, --openings, --programs, --levels, --instructors, --days, --ages, --genders, --sessions, --location-id, --limit and --page are honored upstream; anything else is applied locally and the CLI warns on stderr when it does so.
- **camps list returns 'No camps found' for a type id that appeared in programs camps** — Use 'bookings <slug> <locationId>' for type ids. The ids from 'programs camps' are programIds and never match --type-id.
- **drift or fill-rate report nothing at all** — Both read local history. Run 'iclasspro-pp-cli sync <account> --resources classes,camps' at least twice, spaced apart, before either has anything to compare.
- **programs appointments returns a plan message instead of data** — That account does not have the appointments subscription. The message is returned by iClassPro, not by this CLI, and there is no workaround.
- **Organization not found** — The portal slug is wrong. It is the path segment in portal.iclasspro.com/<slug>, not the gym's brand name or website domain.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**master-events-calendar**](https://github.com/Jaymelynng/master-events-calendarMASTER) — JavaScript
- [**iclasspro-driver**](https://github.com/johnmarcovici/iclasspro-driver) — Python
- [**icp-widget**](https://github.com/DevCabin/icp-widget) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

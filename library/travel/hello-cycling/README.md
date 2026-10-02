# HELLO CYCLING CLI

**Find compatible HELLO CYCLING pickup and return stations with timestamped counts and published pricing.**

Search Japanese station names and addresses, rank explicit-coordinate pickup/return choices, and compare compatible trip endpoints. Source timestamps, missing counts, and model-dependent pricing constraints remain attached to every result.

Learn more at [HELLO CYCLING](https://www.hellocycling.jp/) and the [provider-published GBFS feed](https://api-public.odpt.org/api/v4/gbfs/hellocycling/gbfs.json).

## Local build status


## Station planning and source limits

Use `stations find`, `stations show`, and `stations nearby` for bounded Japanese station discovery. Nearby requires explicit latitude and longitude and computes distances locally from the country-wide feed. `trip compare` requires both coordinates and an explicit GBFS class ID; `vehicles rules` lists current classes and checks the provider's published special-vehicle restrictions. Class `2` currently represents a generic electric-assist bicycle and cannot select city/sports/e-Bike models or prove electric-cycle support.

Results carry `meta.source`, UTC and JST observation timestamps, per-feed Unix timestamps/TTL, source bytes/request count, license and attribution. Station counts are nullable observations. States distinguish `available`, `empty`, `full`, `closed`, `uninstalled`, `incompatible`, `stale`, `unknown`, and `source_missing`. Default freshness threshold is five minutes; `--status-max-age` sets an explicit alternative. `--available-only` and trip comparisons exclude uncertain/stale stations. Counts do not reserve a bike or return space.

The source publishes whole-country feeds: a station query makes four GETs (about 12.4 MB combined when verified), with a 16 MiB cap per feed, no retries, 20-second request deadlines and a 60-second operation cap. Output is bounded to 50 stations/changes, or 10 trip pairs chosen from at most 121 pair candidates. If station status is temporarily unavailable, station identity can still return with null counts and a source warning; auth/access/throttle failures remain errors. Pricing uses two page GETs, each at most 1 MiB.

`--data-source local` and `--offline` read the explicit station snapshot; `--data-source live` requires provider data. Auto can use a saved snapshot after a transient live failure with a warning, preserved timestamps and local provenance; access/throttle failures stay errors. Pricing, vehicle rules, sync and changes require live data.

`stations sync` explicitly saves an atomic SQLite snapshot. Add `--offline` to station discovery or trip comparison to read it with original timestamps; old counts remain stale. `stations changes` compares a saved snapshot with a new observation and does not overwrite the baseline. Use `--snapshot-db` to choose a local cache path.

`pricing areas` discovers current published area URLs. `pricing show --area tokyo` retains Tokyo base, Chiyoda and Itabashi tables separately. A `not_set` price is null, never zero. Price bands are not quotes: exact vehicle model and area exceptions require app confirmation. The CLI returns source rental/app URLs for final handoff.

GBFS data is attributed to HELLO CYCLING / OpenStreet Co., Ltd. via ODPT under the provider's offered [CC BY 4.0 license](https://d1yl7kw204zjxn.cloudfront.net/gbfs/v2/public/hellocycling_gbfs_licence.txt). Data is normalized and locally ranked. Source count/type feeds and first-party pricing/rules were observed on 2026-10-02/03 JST; freshness is reported on each run.

## Install

The recommended path installs both the `hello-cycling-pp-cli` binary and the `pp-hello-cycling` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install hello-cycling
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install hello-cycling --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install hello-cycling --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install hello-cycling --agent claude-code
npx -y @mvanhorn/printing-press-library install hello-cycling --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/cmd/hello-cycling-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hello-cycling-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install hello-cycling --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hello-cycling --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hello-cycling --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install hello-cycling --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hello-cycling-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/cmd/hello-cycling-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hello-cycling": {
      "command": "hello-cycling-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account, API key or location permission is required. Anonymous provider-published GBFS and first-party price pages are read through HTTP. Reserve rides only in the official app.

## Quick Start

```bash
# Verify local setup without making a ride request.
hello-cycling-pp-cli doctor --dry-run

# Resolve stable Japanese station IDs.
hello-cycling-pp-cli stations find --query 東新宿 --limit 5 --agent

# Read current nearby pickup observations.
hello-cycling-pp-cli stations nearby --lat 35.697315 --lon 139.704995 --purpose pickup --vehicle-type 2 --limit 5 --agent

# Inspect model and municipal rate exceptions.
hello-cycling-pp-cli pricing show --area tokyo --agent

```

## Unique Features

These workflows combine source observations with explicit planning constraints.

### Station planning
- **`stations nearby`** — Find nearby pickup or return choices while retaining closed, empty, full, stale and unknown states.

  _Find nearby pickup or return choices while retaining closed, empty, full, stale and unknown states._

  ```bash
  hello-cycling-pp-cli stations nearby --lat 35.697315 --lon 139.704995 --purpose pickup --vehicle-type 2 --agent
  ```
- **`trip compare`** — Compare pickup and dropoff pairs with per-type return capacity and straight-line access distances.

  _Compare pickup and dropoff pairs with per-type return capacity and straight-line access distances._

  ```bash
  hello-cycling-pp-cli trip compare --from-lat 35.697315 --from-lon 139.704995 --to-lat 35.707252 --to-lon 139.777587 --vehicle-type 2 --agent
  ```
- **`stations sync`** — Save a provider snapshot for offline station discovery with original source timestamps.

  _Save a provider snapshot for offline station discovery with original source timestamps._

  ```bash
  hello-cycling-pp-cli stations sync --agent
  ```
- **`stations changes`** — Inspect count and operational changes since an explicitly saved snapshot.

  _Inspect count and operational changes since an explicitly saved snapshot._

  ```bash
  hello-cycling-pp-cli stations changes --query 新宿 --agent
  ```
- **`pricing show`** — Read current source tables including municipal exceptions without asserting a station bike price.

  _Read current source tables including municipal exceptions without asserting a station bike price._

  ```bash
  hello-cycling-pp-cli pricing show --area tokyo --agent
  ```

## Recipes

### Find station IDs

```bash
hello-cycling-pp-cli stations find --query 新宿 --limit 5 --agent --select results.id,results.name,results.rental_state,results.return_state
```

Narrow Japanese name/address matches to station identity and observed state.

### Inspect one station

```bash
hello-cycling-pp-cli stations show --id 5112 --agent
```

Read counts, per-type dock compatibility and original timestamps.

### Compare both ends

```bash
hello-cycling-pp-cli trip compare --from-lat 35.697315 --from-lon 139.704995 --to-lat 35.707252 --to-lon 139.777587 --vehicle-type 2 --agent
```

Choose short access distances with compatible snapshot capacity.

### Read published Tokyo prices

```bash
hello-cycling-pp-cli pricing show --area tokyo --agent
```

Keep Chiyoda and Itabashi exceptions separate.

## Usage

Run `hello-cycling-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.json` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HELLO_CYCLING_CONFIG_DIR`, `HELLO_CYCLING_DATA_DIR`, `HELLO_CYCLING_STATE_DIR`, or `HELLO_CYCLING_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HELLO_CYCLING_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HELLO_CYCLING_HOME=/srv/hello-cycling
hello-cycling-pp-cli doctor
```

Under `HELLO_CYCLING_HOME=/srv/hello-cycling`, the four dirs resolve to `/srv/hello-cycling/config`, `/srv/hello-cycling/data`, `/srv/hello-cycling/state`, and `/srv/hello-cycling/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "hello-cycling": {
      "command": "hello-cycling-pp-mcp",
      "env": {
        "HELLO_CYCLING_HOME": "/srv/hello-cycling"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `HELLO_CYCLING_DATA_DIR` overrides an explicit `--home` for that kind. Use `HELLO_CYCLING_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HELLO_CYCLING_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `hello-cycling-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### feeds

Inspect the provider-published GBFS discovery and vehicle type metadata.

- **`hello-cycling-pp-cli feeds discovery`** - Get advertised anonymous HELLO CYCLING GBFS feed URLs.
- **`hello-cycling-pp-cli feeds vehicles`** - Get generic source vehicle classes without assigning station bike models.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`hello-cycling-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`hello-cycling-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`hello-cycling-pp-cli learnings list`** - Inspect taught rows
- **`hello-cycling-pp-cli learnings forget <query>`** - Undo a teach
- **`hello-cycling-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`hello-cycling-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`hello-cycling-pp-cli teach-pattern`** - Install a query/resource template up front
- **`hello-cycling-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `HELLO_CYCLING_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `hello-cycling-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hello-cycling-pp-cli feeds discovery

# JSON for scripting and agents
hello-cycling-pp-cli feeds discovery --json
# Filter to specific fields
hello-cycling-pp-cli feeds discovery --json --select last_updated,ttl,version

# Dry run — show the request without sending
hello-cycling-pp-cli feeds discovery --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hello-cycling-pp-cli feeds discovery --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
hello-cycling-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `hello-cycling-pp-cli doctor` to see the resolved config, data, state, and cache directories. The config location is platform-resolved; inspect `doctor --json` for active directories; `--home`, `HELLO_CYCLING_HOME`, and per-kind env vars can relocate it.

The no-auth station planner does not require custom headers. JSON configuration and profiles remain available for the generated framework commands.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `stations find --query 新宿` to see available items

## HTTP Transport

Provider feed and website reads use standard HTTP and require no browser process. A preserved shared client hook caps generated raw metadata responses at 256 KiB and rejects unexpected encodings/redirects; the station planner separately caps whole-country feeds at 16 MiB each.

Provider reads verify TLS certificates. Standard source transport is enforced by the shared metadata hook and the station client.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

Status-feed outages trigger the saved-snapshot fallback in auto mode when it is usable. Explicit live/no-cache discovery retains station identity with unknown counts and source_missing states. Sync and changes reject a missing status feed, preserving the saved baseline. Change inspection evaluates baseline usability at its original observation time and reports freshness/compatibility state transitions even when counts remain equal.

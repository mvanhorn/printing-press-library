# TeslaTracker CLI

**Every used Tesla TeslaTracker lists, in a local store that answers questions the site cannot — price paths, cohort placement, warranty left, and what already left the lot.**

TeslaTracker aggregates used Teslas across Tesla, Carvana, CarMax and private sellers. This CLI mirrors that inventory into VIN-keyed SQLite and adds the layer the website has no way to offer: a recorded price path per car (price-history), where a price sits in its cohort (comps), how much warranty is genuinely left at delivery (warranty), and which cars quietly left inventory (gone). Every money figure is landed cost, including the real per-car transport fee.

Learn more at [TeslaTracker](https://teslatracker.com).

Created by [@michegz](https://github.com/michegz) (michegz).

## Install

The recommended path installs both the `teslatracker-pp-cli` binary and the `pp-teslatracker` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install teslatracker
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install teslatracker --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install teslatracker --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install teslatracker --agent claude-code
npx -y @mvanhorn/printing-press-library install teslatracker --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/teslatracker/cmd/teslatracker-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/teslatracker-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install teslatracker --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-teslatracker --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-teslatracker --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install teslatracker --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/teslatracker-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/teslatracker/cmd/teslatracker-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "teslatracker": {
      "command": "teslatracker-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the CLI can reach the source before anything else
teslatracker-pp-cli doctor --dry-run

# your constraints live in a saved search and become the default for watch run
teslatracker-pp-cli watch add mine --max-landed 30000 --model "Model 3" --max-miles 60000

# build the local mirror; this is what every derived command reads
teslatracker-pp-cli sync

# turn each listing link into a full vehicle record; every derived command needs this
teslatracker-pp-cli hydrate

# confirm the mirror is complete before trusting any aggregate
teslatracker-pp-cli coverage

# long-listed and never cut — the strongest negotiation profile
teslatracker-pp-cli stale --limit 10

# from now on, see only what changed since last time
teslatracker-pp-cli watch run mine

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Coverage and condition
- **`warranty`** — See exactly how many months and miles of Tesla warranty are left on the day you would take delivery, and which limit runs out first.

  _Reach for this when the question is coverage or risk on one specific car, not price._

  ```bash
  teslatracker-pp-cli warranty 5YJ3E1EA7LF745758 --agent
  ```
- **`degradation`** — Compare a car's actual range against its rated range, and see where that gap sits among every similar car in your local store.

  _Use for range condition. This is the source's published spread, not a measured battery test._

  ```bash
  teslatracker-pp-cli degradation 5YJ3E1EA7LF745758 --agent
  ```

### Price intelligence
- **`comps`** — See where one car's landed cost sits in the distribution of comparable cars, with the sample size and the arithmetic shown.

  _The honest answer to 'is this price good' — placement and arithmetic, never a verdict._

  ```bash
  teslatracker-pp-cli comps 5YJ3E1EA7LF745758 --agent
  ```
- **`premium`** — See what a configuration attribute actually costs across the market, computed within matched cohorts.

  _Answers 'can I afford HW4 at my ceiling'. Observational, not causal._

  ```bash
  teslatracker-pp-cli premium --by hardwareVersion --agent
  ```
- **`stale`** — Rank the corpus by days listed and flag cars that are both long-listed and have never had a price cut.

  _Finds negotiation candidates. Deliberately does not judge whether the price is good._

  ```bash
  teslatracker-pp-cli stale --limit 20 --agent
  ```
- **`radius`** — See how the price floor and median change as you widen your search radius, with real transport fees included at every step.

  _Answers 'is shopping farther actually worth it' in dollars. Returns a curve, not cars._

  ```bash
  teslatracker-pp-cli radius --lat 30.2241 --lon -92.0198 --agent
  ```

### Local state that compounds
- **`gone`** — See which cars left inventory, at what landed price, and after how many days listed.

  _Calibrates a ceiling against what actually clears. A departure is not a sale._

  ```bash
  teslatracker-pp-cli gone --agent
  ```
- **`price-history`** — See a single car's full observed price path: every cut, its size, and how long it has held the current price.

  _Use for one car's own history; use stale to find candidates across the corpus._

  ```bash
  teslatracker-pp-cli price-history 5YJ3E1EA7LF745758 --agent
  ```
- **`watch`** — Save a named search and see only what is new, price-changed, or departed since the last time you ran it.

  _The right entry point for a recurring session instead of re-running search._

  ```bash
  teslatracker-pp-cli watch run --all --agent
  ```

### Agent-native plumbing
- **`coverage`** — See how complete and how fresh your local mirror is, field by field, including whether any pages were dropped during sync.

  _Call before asserting any aggregate. Reports on the store, never on cars._

  ```bash
  teslatracker-pp-cli coverage --agent
  ```

## Recipes

### Rank the corpus by how long cars have been sitting

```bash
teslatracker-pp-cli stale --limit 20 --agent --select vin,model,year,mileage,landed_usd
```

Ranks the local mirror by days listed, with landed cost attached. Staleness is a negotiating signal, not a price judgement — a car can sit simply because it is overpriced, so pair a VIN from here with `comps` before making an offer.

### Full-text search the local mirror

```bash
teslatracker-pp-cli search "model 3 long range" --limit 20 --agent
```

FTS5 over everything synced, offline, no API call.

### Is this price defensible?

```bash
teslatracker-pp-cli comps 5YJ3E1EA7LF745758 --agent
```

Returns the percentile, the median, the dollar gap and the sample size — never a bare score.

### How much car is left

```bash
teslatracker-pp-cli warranty 5YJ3E1EA7LF745758 --annual-miles 12000 --agent
```

Months and miles remaining at delivery, naming whichever of the four limits binds first.

### What did I miss since yesterday

```bash
teslatracker-pp-cli watch run --all --agent
```

New, price-changed and departed since each saved search's own cursor.

### Does the HW4 premium fit my budget

```bash
teslatracker-pp-cli premium --by hardwareVersion --agent
```

The landed-price difference between hardware versions within matched model/year/trim cohorts, with n per cell.

## Usage

Run `teslatracker-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TESLATRACKER_CONFIG_DIR`, `TESLATRACKER_DATA_DIR`, `TESLATRACKER_STATE_DIR`, or `TESLATRACKER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TESLATRACKER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TESLATRACKER_HOME=/srv/teslatracker
teslatracker-pp-cli doctor
```

Under `TESLATRACKER_HOME=/srv/teslatracker`, the four dirs resolve to `/srv/teslatracker/config`, `/srv/teslatracker/data`, `/srv/teslatracker/state`, and `/srv/teslatracker/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "teslatracker": {
      "command": "teslatracker-pp-mcp",
      "env": {
        "TESLATRACKER_HOME": "/srv/teslatracker"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TESLATRACKER_DATA_DIR` overrides an explicit `--home` for that kind. Use `TESLATRACKER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TESLATRACKER_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `teslatracker-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### inventory

Used Tesla listings, VIN-keyed

- **`teslatracker-pp-cli inventory get`** - 
- **`teslatracker-pp-cli inventory list`** - 
- **`teslatracker-pp-cli inventory report`** - 


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`teslatracker-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`teslatracker-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`teslatracker-pp-cli learnings list`** - Inspect taught rows
- **`teslatracker-pp-cli learnings forget <query>`** - Undo a teach
- **`teslatracker-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`teslatracker-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`teslatracker-pp-cli teach-pattern`** - Install a query/resource template up front
- **`teslatracker-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TESLATRACKER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `teslatracker-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
teslatracker-pp-cli inventory list

# JSON for scripting and agents
teslatracker-pp-cli inventory list --json
# Filter to specific fields by name
teslatracker-pp-cli inventory list --json --select <field>[,<field>...]

# Dry run — show the request without sending
teslatracker-pp-cli inventory list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
teslatracker-pp-cli inventory list --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `TESLATRACKER_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `teslatracker-pp-cli inventory`
- `teslatracker-pp-cli inventory get`
- `teslatracker-pp-cli inventory list`

`teslatracker-pp-cli inventory report` is a read but is **not** on the freshness hook; it always goes to the API under `--data-source auto`. The derived commands (`warranty`, `comps`, `degradation`, `price-history`, `gone`, `stale`, `premium`, `radius`, `watch`, `coverage`) read the local mirror only and are refreshed by `sync` + `hydrate`, not by this hook.

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
teslatracker-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `teslatracker-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `TESLATRACKER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every price looks 100x too high** — The source returns cents. Re-sync — conversion happens once at ingest, so a mixed store must be rebuilt.
- **comps refuses to return a percentile** — The cohort is below the sample floor. Widen with --mileage-band or sync more pages.
- **Mileage-capped search returns cars over the cap** — Run coverage. A null mileage is absent, not zero, and is excluded rather than silently passing.
- **gone returns nothing on first run** — There is no prior sync to compare against. Run sync twice, some hours apart.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**teslahunt/inventory**](https://github.com/teslahunt/inventory) — JavaScript
- [**kaedenbrinkman/tesla-inventory**](https://github.com/kaedenbrinkman/tesla-inventory) — Python
- [**JumpBearCode/TeslaWebScrape**](https://github.com/JumpBearCode/TeslaWebScrape) — Python
- [**robcerda/tesla-mcp-server**](https://github.com/robcerda/tesla-mcp-server) — Python
- [**tdorssers/TeslaPy**](https://github.com/tdorssers/TeslaPy) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

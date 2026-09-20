# NEPRA CLI

**Pakistan's electricity regulator publishes the country's only plant-level power data as Excel-exported HTML that reads as empty and PDFs nobody extracts — this turns it into a queryable panel and names every gap.**

NEPRA is the sole public source for monthly plant-by-plant generation and per-distribution-company reliability in Pakistan, and there is no API, no dataset and no package for any of it. This CLI extracts those releases live from the published documents, keeps unreported months distinct from measured zeros, follows plants across years despite renamed rows and re-sorted row numbers, and surfaces the places the regulator's own documents disagree with themselves instead of quietly picking one.

## Install

The recommended path installs both the `nepra-pp-cli` binary and the `pp-nepra` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nepra
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nepra --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nepra --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nepra --agent claude-code
npx -y @mvanhorn/printing-press-library install nepra --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/nepra/cmd/nepra-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nepra-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install nepra --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nepra --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nepra --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install nepra --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nepra-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/nepra/cmd/nepra-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nepra": {
      "command": "nepra-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the site is reachable before anything else; no credential is needed anywhere in this CLI
nepra-pp-cli doctor

# see which fiscal years actually exist upstream, and which are listed but unreachable
nepra-pp-cli sources --kind gen

# see every surface this CLI serves, how old each measurement is, and which command answers it; exits non-zero past the window so a cron can gate on it. Makes no request
nepra-pp-cli coverage --check-stale

# the plant-by-month grid for one fiscal year as rows
nepra-pp-cli gen --fy 2023-24 --agent

# prove the published bytes and the 32-column layout still match what this build expects. It is a FETCH gate, not a cache gate: nothing is stored locally
nepra-pp-cli verify --fy 2023-24

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### The panel nobody else has
- **`gen`** — Pull every power plant's monthly generation and utilisation for a fiscal year as rows you can pipe, with unreported months kept distinct from real zeros.

  _Reach for this when you need the whole fiscal year at plant granularity; it is the only machine-readable form of this table that exists._

  ```bash
  nepra-pp-cli gen --fy 2023-24 --agent
  ```
- **`fleet`** — Follow one plant or a listed operator's whole fleet across the fiscal years the crosswalk has observed, through renames and re-sorted row numbers.

  _Use this for any question spanning more than one fiscal year, and for mapping plants to listed operators. meta reports observed_fys against published_fys: the embedded crosswalk does not cover every published year, and the gap is stated rather than hidden._

  ```bash
  nepra-pp-cli fleet --parent HUBC --agent
  ```
- **`capacity`** — Installed and dependable capacity broken out by plant status and system, so each headline megawatt figure is labelled and derivable.

  _Use this when a capacity number has to reconcile; it shows which definition produces which total._

  ```bash
  nepra-pp-cli capacity --as-of 2024-06-30 --by status --agent
  ```

### Trust the number or don't
- **`conflicts`** — List every place NEPRA's own published figures disagree — two sources on one key, an unexplained step inside a single series, or a row that fails its own arithmetic — showing both values and both citations without picking a winner.

  _Reach for this before quoting any NEPRA figure. `--kind conflict` is the two-source case; `--kind break` is a step inside one series (those entries carry same_key=false, because there is no second source to cite); `--kind arithmetic` is a row that contradicts itself. Default is all three._

  ```bash
  nepra-pp-cli conflicts --surface per --agent
  ```
- **`disco`** — Per-distribution-company transmission losses, recovery, interruption frequency and duration against the regulator's own targets, across report years.

  _Use this for any per-DISCO performance question; the national averages everyone quotes hide the spread._

  ```bash
  nepra-pp-cli disco --metric saidi --fy 2024-25 --agent
  ```
- **`verify`** — Assert every published NEPRA surface against status, byte floor, content checksum, in-document year token and the frozen column fingerprint, exiting non-zero when stale or drifted.

  _Use this before trusting any extract. It is a FETCH gate, not a cache gate — nothing is stored locally — and it catches the decoys this site serves under an HTTP 200._

  ```bash
  nepra-pp-cli verify --fy 2023-24 --agent
  ```

### Find the document
- **`events`** — A dated feed of tariff determinations and orders by company, distribution company or docket, with the verbatim document link.

  _Use this to find which determination applies to a company and when; it returns document identity, not rates._

  ```bash
  nepra-pp-cli events --surface ipp-thermal --docket TRF-71 --agent
  ```
- **`licence`** — The licence register with gross capacity, plant type, fuel and the modification trail, searchable by name or fuel.

  _Reach for this for licensed-capacity and plant-type questions. Search by --search or --fuel; lookup by PSX ticker is refused with exit 2 because the crosswalk resolves only 28 of 335 register names, and fleet --parent is the right instrument for an operator's generation panel._

  ```bash
  nepra-pp-cli licence --fuel Coal --agent
  ```
- **`fca`** — The monthly requested-versus-allowed fuel cost adjustment series with cumulative disallowance, bounded to the window the source actually covers.

  _Use this to see how much of a requested fuel adjustment the regulator actually allowed, month by month._

  ```bash
  nepra-pp-cli fca --entity cppag --cumulative --agent
  ```
- **`sources`** — Catalogue every reachable published document with size, coverage window, text-layer flag and reachability state, and diff it against the shipped manifest.

  _Use this to discover what exists before fetching, and to tell an unreachable document apart from one with no data._

  ```bash
  nepra-pp-cli sources --kind per --agent
  ```

## Recipes

### One operator's fleet across every published year

```bash
nepra-pp-cli fleet --parent HUBC --agent
```

Follows a listed operator's plants through renames and re-sorted rows, narrowing the payload to the four fields an availability model needs.

### Which distribution company misses its loss target by most

```bash
nepra-pp-cli disco --metric tnd --fy 2024-25 --agent
```

Returns actual reported losses alongside the level allowed in tariff and the breach, per company, which no published page tabulates together.

### Check a figure is not contested before quoting it

```bash
nepra-pp-cli conflicts --surface per --agent
```

Lists same-key disagreements between two published documents with both citations, so a quoted number can be defended or avoided.

### See which fiscal years and surfaces actually exist upstream

```bash
nepra-pp-cli sources --diff --agent
```

Diffs the live catalogue against the shipped manifest, separating genuinely unreachable documents from ones with no data.

### Gate a published surface on bytes, decoys and the column fingerprint

```bash
nepra-pp-cli verify --fy 2023-24 --check-stale --agent
```

Emits a citable checksum manifest and exits non-zero if any artifact drifted or aged past its staleness window.

## Known Gaps

Measured on this build. Each is a real limit, not a caveat about the upstream data.

- **`disco` provenance captions are garbled.** Each row's `citation.caption` carries the table caption exactly as NEPRA's Performance Evaluation Report PDFs expose it, and that text layer splits words at arbitrary points — e.g. `Table 0 6:SystemAverageInterruptionDur ationIndex(SAIDI )`. This is verbatim provenance and is safe to cite as such, but it is not a clean label and should not be parsed. Use `metric` (`saidi`) or `metric_title` (`System Average Interruption Duration Index (SAIDI - Min)`) for display, and `citation.table` + `citation.page` to locate the source.
- **`--max-age` is accepted but does nothing.** The flag is declared on the root command, but this CLI has no `sync` command for it to gate, so no staleness hint is ever emitted. Use `coverage --check-stale [--stale-after <days>]` for a staleness gate over every surface, or `verify --check-stale` for one surface family.

## Usage

Run `nepra-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `NEPRA_CONFIG_DIR`, `NEPRA_DATA_DIR`, `NEPRA_STATE_DIR`, or `NEPRA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `NEPRA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export NEPRA_HOME=/srv/nepra
nepra-pp-cli doctor
```

Under `NEPRA_HOME=/srv/nepra`, the four dirs resolve to `/srv/nepra/config`, `/srv/nepra/data`, `/srv/nepra/state`, and `/srv/nepra/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "nepra": {
      "command": "nepra-pp-mcp",
      "env": {
        "NEPRA_HOME": "/srv/nepra"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `NEPRA_DATA_DIR` overrides an explicit `--home` for that kind. Use `NEPRA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `NEPRA_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `nepra-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### events

Tariff determinations and Authority orders

- **`nepra-pp-cli events`** - The determinations feed. REQUIRES gzip: without Accept-Encoding this page silently truncates under a 30 s timeout and returns a partial list under an HTTP 200.

### fca

Fuel cost adjustment: monthly requested versus allowed, CPPA-G and K-Electric

- **`nepra-pp-cli fca`** - The consolidated FCA table. Coverage is hard-bounded Jul-2018 to Jun-2022; negatives are written in accounting parentheses.

### generation

Plant-level installed and dependable capacity with monthly generation and utilisation, per fiscal year

- **`nepra-pp-cli generation index`** - The site's own generation index. INCOMPLETE — links 5 of the 7 reachable years; never use as the sole enumerator.
- **`nepra-pp-cli generation year`** - Fetch one fiscal year's generation workbook payload. windows-1252 encoded; the HTTP header declares no charset.
- **`nepra-pp-cli generation plants`** - One row per plant for a fiscal year: installed and dependable capacity, technology, fuel and status, filterable with `--technology` and `--fuel`. This is the route to capacity by fuel. No capacity factor or utilisation is emitted at any grain.
- **`nepra-pp-cli generation monthly`** - Twelve rows per plant for a fiscal year, with the same filters. Unfiltered it is byte-identical to `gen --fy <year>`.
- **`nepra-pp-cli coverage`** - The coverage ledger: all 104 surfaces this CLI can serve, the command that answers for each, and the age of every measurement. Makes no request. `--check-stale` exits 6 past the window.

### hydel

Hydel generation and dependable capacity

- **`nepra-pp-cli hydel`** - Hydel data. Dependable capacities are pinned upstream to 'July 2018 - June 2022'.

### licence

Generation licence register: gross capacity, licence number, plant type, fuel, modification trail

- **`nepra-pp-cli licence`** - The licence register accordion. Key lookup must be typo-aliased: the page misspells its own column as 'Gross Capacityy', hiding the largest entry.

### quarterly

Quarterly data for ex-WAPDA DISCOs and K-Electric

- **`nepra-pp-cli quarterly`** - Quarterly index. FROZEN upstream at 'April 2022- June 2022 (4th Quarter)' with 'Not Yet Issued' against every DISCO; nothing on the page says it is stale.

### reliability

DISCO Performance Evaluation Reports: T&D losses against allowed target, recovery, SAIFI, SAIDI, complaints, safety

- **`nepra-pp-cli reliability <path>`** - Fetch one Performance Evaluation Report PDF. Filenames are unconstructible across years — always scrape from sources, never build the path.

### sir

State of Industry Report PDFs

- **`nepra-pp-cli sir <year>`** - One State of Industry Report, returned as PDF bytes. Very large: 2025 is ~316 MB.

### sro

SRO notifications

- **`nepra-pp-cli sro`** - SRO notification index.

### tariff

Per-distribution-company notified tariff pages

- **`nepra-pp-cli tariff <disco>`** - One DISCO's tariff page. UTF-8 on this surface, unlike the generation workbooks. Rate schedules themselves are image-only PDFs and are NOT extractable.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`nepra-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`nepra-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`nepra-pp-cli learnings list`** - Inspect taught rows
- **`nepra-pp-cli learnings forget <query>`** - Undo a teach
- **`nepra-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`nepra-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`nepra-pp-cli teach-pattern`** - Install a query/resource template up front
- **`nepra-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `NEPRA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `nepra-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nepra-pp-cli events

# JSON for scripting and agents
nepra-pp-cli events --json
# Filter to specific fields by name
nepra-pp-cli events --json --select <field>[,<field>...]

# Dry run — show the request without sending
nepra-pp-cli events --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nepra-pp-cli events --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only** - this CLI has no write path to NEPRA at all. It does not create, update, delete, publish, send or mutate anything, and needs no credential
- **Live-only** - every data command reads the published documents on each invocation, or computes from an embedded catalogue. There is no sync command and no local NEPRA store; `--data-source local` is refused rather than answered emptily
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **One envelope, two documented shapes** - every command emits `{meta, results}`. On the row-returning commands — `capacity`, `coverage`, `events`, `fca`, `gen`, `hydel`, `licence`, `quarterly`, `sources` and `sro` — `results` is a row array, so `jq '.results[]'` iterates rows on all of them alike. On the four *report* commands — `verify`, `fleet`, `conflicts` and `disco` — `results` is a keyed object of named sections (`surfaces`, `operators`, `entries`, `report_years`, …), because those commands report sections rather than a row list; iterate the section you want, e.g. `jq '.results.surfaces[]'`. No command nests a second envelope, and none emits a bare top-level array or object

`meta` always carries the command's own provenance. Where a generic transport claim disagrees with the command's measured origin — `source: "live"` meaning this run touched the network, versus `source: "catalogue"` meaning the answer came from an embedded catalogue — the command's origin wins and the transport claim is preserved at `meta.envelope_meta` rather than dropped. The two are different axes and are never collapsed.

Exit codes: `0` success, `1` a completeness assertion failed under `--strict`, `2` usage error, `3` not published, `5` API or parse error, `6` a staleness or fetch gate refused, `7` rate limited, `10` config error.

## Health Check

```bash
nepra-pp-cli doctor
```

Verifies configuration and connectivity to nepra.org.pk. No credential is involved: this CLI has no auth.

## Configuration

Run `nepra-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/nepra-pp-cli/config.toml`; `--home`, `NEPRA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not published (exit code 3)**
- This CLI has no resource IDs. Exit 3 means NEPRA does not publish what was asked for at the path this build knows — most often a fiscal year outside the reachable window.
- Run `nepra-pp-cli sources` to see which documents are published, reachable or unavailable, and `nepra-pp-cli coverage` for the surfaces each command serves and how old each measurement is.
- `unpublished` and `empty` are deliberately different findings: a year that exists and reports nothing returns rows, not exit 3.

### API-specific
- **A fiscal year returns no rows and no error** — Run 'nepra-pp-cli sources --kind gen' — several years are listed by the site's own index but return 404, and are reported as unreachable rather than empty.
- **Generation figures look like every month is zero** — Unreported months are stored distinct from measured zeros. Pass --agent and read value_state; a blank month is absence of measurement, not zero output.
- **A plant disappears between fiscal years** — Row numbers are not stable across years and names drift. Use 'nepra-pp-cli fleet --plant' which resolves aliases, rather than joining on the row number.
- **Two NEPRA documents give different values for the same metric** — That is expected and recorded. Run 'nepra-pp-cli conflicts' to see both values with both citations; the CLI never reconciles them for you.
- **A distribution company is missing from reliability output** — The regulator excludes one company from these tables on the record, citing unreliable metering. It returns as unavailable with that reason rather than as an empty row.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**bijlicheck**](https://github.com/Muhammad-Sohair/bijlicheck) — TypeScript
- [**ebillpakistan.pk**](https://github.com/atifjan2019/ebillpakistan.pk) — JavaScript
- [**ha_wapda_peak_hours**](https://github.com/farhanshahlabs/ha_wapda_peak_hours) — Python
- [**VoltaIQ**](https://github.com/Noor-Rehman/VoltaIQ) — Python
- [**NEPRA-Prosumer-Bill-Calculator-2025**](https://github.com/AE-Usama/NEPRA-Prosumer-Bill-Calculator-2025) — TypeScript
- [**Solar-NetBilling-model**](https://github.com/harisahmadkhan/Solar-NetBilling-model-) — TypeScript
- [**nepradataset**](https://github.com/balochasif2021-arch/nepradataset) — None

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

# Coin Locker Navi CLI

**Walk-up coin lockers in Japan: size, price, IC card, hours, gate side, and live empties where published.**

Search コインロッカーなび by station name or coordinates and get normalized, source-dated JSON with explicit unknowns. near --live attaches live per-size empties from Multi Ekicube and the Maihama map; vacancy gives the full live board for one station. Live empties and gate side exist only where Multi Ekicube (JR East) banks or the Maihama Station map publish them; elsewhere those fields are null.

## Install

The recommended path installs both the `coinlocker-navi-pp-cli` binary and the `pp-coinlocker-navi` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install coinlocker-navi
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install coinlocker-navi --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install coinlocker-navi --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install coinlocker-navi --agent claude-code
npx -y @mvanhorn/printing-press-library install coinlocker-navi --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/cmd/coinlocker-navi-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/coinlocker-navi-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install coinlocker-navi --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-coinlocker-navi --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-coinlocker-navi --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install coinlocker-navi --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/coinlocker-navi-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/cmd/coinlocker-navi-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "coinlocker-navi": {
      "command": "coinlocker-navi-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Check setup without a request
coinlocker-navi-pp-cli doctor --dry-run

# Large walk-up lockers that take IC cards
coinlocker-navi-pp-cli near 東京駅 --size L --ic --agent

# Live reservable empties outside the gates
coinlocker-navi-pp-cli vacancy 東京駅 --gate outside --agent

# One locker with neighbours
coinlocker-navi-pp-cli locker 2685 --agent

```

## Unique Features

### Walk-up decision
- **`near`** — See walk-up lockers near a station with live per-size empties attached where Multi Ekicube (nearest bank within 30 m, reservable counts) or the Maihama map publishes them; other lockers have no live field.

  _Use it when an agent must decide where to drop a bag without a booking and wants live space evidence in one call._

  ```bash
  coinlocker-navi-pp-cli near 東京駅 --live --agent
  ```
- **`near`** — Keep only lockers open across your drop and pick-up times; unknown hours stay visible and flagged.

  _Use it to avoid a locker that closes before the user's last train._

  ```bash
  coinlocker-navi-pp-cli near 新宿 --open-during 10:00-21:30 --agent
  ```
- **`near`** — Filter lockers by inside or outside the ticket gates using Multi Ekicube data (exactly one bank within 15 m); unknown gate side never matches, so areas without Multi Ekicube banks return no matches.

  _Use it when the user will not enter the station paid area._

  ```bash
  coinlocker-navi-pp-cli near --lat 35.6896 --lon 139.7006 --gate outside --limit 3 --agent
  ```

## Recipes

### Large lockers with live space

```bash
coinlocker-navi-pp-cli near 新宿 --size L --live --agent --select meta.fetched_at,meta.notes,results.id,results.name,results.sizes,results.live
```

Keep the decision fields plus the observation time and caveat notes.

### Open across a time window

```bash
coinlocker-navi-pp-cli near 新大阪 --open-during 10:00-21:30 --agent
```

Keeps lockers whose published hours cover the window; train_hours and unknown hours stay flagged unless --strict.

### Disney day

```bash
coinlocker-navi-pp-cli vacancy --station maihama --agent
```

Installed and empty boxes per size for each Maihama Station block, with the source as-of time.

## Usage

Run `coinlocker-navi-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Local data such as `feedback.jsonl` (no locker data is stored) |
| `state` | Runtime state |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `COINLOCKER_NAVI_CONFIG_DIR`, `COINLOCKER_NAVI_DATA_DIR`, `COINLOCKER_NAVI_STATE_DIR`, or `COINLOCKER_NAVI_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `COINLOCKER_NAVI_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export COINLOCKER_NAVI_HOME=/srv/coinlocker-navi
coinlocker-navi-pp-cli doctor
```

Under `COINLOCKER_NAVI_HOME=/srv/coinlocker-navi`, the four dirs resolve to `/srv/coinlocker-navi/config`, `/srv/coinlocker-navi/data`, `/srv/coinlocker-navi/state`, and `/srv/coinlocker-navi/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "coinlocker-navi": {
      "command": "coinlocker-navi-pp-mcp",
      "env": {
        "COINLOCKER_NAVI_HOME": "/srv/coinlocker-navi"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `COINLOCKER_NAVI_DATA_DIR` overrides an explicit `--home` for that kind. Use `COINLOCKER_NAVI_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `COINLOCKER_NAVI_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Run `coinlocker-navi-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

| Command | What it does |
|---------|--------------|
| `coinlocker-navi-pp-cli near <keyword>` or `near --lat <lat> --lon <lon>` | Walk-up lockers by station/area keyword or coordinates. Filters: `--size S\|M\|L\|XL`, `--ic`, `--walk-up-only`, `--gate inside\|outside`, `--open-during HH:MM-HH:MM [--strict]`. `--live` attaches live empties where a source publishes them. |
| `coinlocker-navi-pp-cli locker <id>` | One coinlocker-navi locker in full (sizes, prices, payment, change machine, hours, coordinates) with its five nearest neighbours. |
| `coinlocker-navi-pp-cli vacancy <keyword>` or `vacancy --lat <lat> --lon <lon>` | Live reservable empties per size at Multi Ekicube banks, with gate side and English + Japanese names. |
| `coinlocker-navi-pp-cli vacancy --station maihama` | Installed and empty boxes per size for each Maihama Station block, with the source as-of time. |
| `coinlocker-navi-pp-cli source search\|locker\|ekicube` | Raw source pages (HTML or JSON) for debugging. Prefer the commands above. |

## Data Honesty

- **Source dates.** Coin Locker Navi pages give no per-record update date. Every response has `meta.fetched_at` (observation time, JST) and a note that says so. Maihama data has `source_as_of` from the page.
- **Unknowns stay null.** When the source shows 情報なし, the field is `null` (for example `hours.kind: "unknown"`, `change_machine: null`, `sizes_known: false`). Filters never show an unknown value as a match; `meta.excluded` counts each record dropped for an unknown. The one exception is `--open-during` without `--strict`: `train_hours` and `unknown` records stay in the list, flagged in `open_during`.
- **Gate side.** `gate` comes only from Multi Ekicube structured fields (`inside_ticket_gate` / `outside_ticket_gate`). On a Coin Locker Navi record, `gate` is set only when exactly one Multi Ekicube bank is within 15 m (`gate_source: multiecube_single_bank_within_15m`). The CLI never takes gate side from Japanese names or notes; when a record's own text says the other side (改札内/改札外), `gate` stays null and `gate_conflict` explains why. `--gate` drops records with unknown gate side (`meta.excluded.gate_unknown`). Multi Ekicube pages are read nearest first and paging stops once `--limit` matches are confirmed, so records farther out are counted as `gate_not_checked`.
- **Hours window.** `open_during` is `yes`, `no`, `train_hours` (source says first to last train) or `unknown`. Without `--strict`, `train_hours` and `unknown` records stay in the list, flagged.
- **Sizes.** Coin Locker Navi labels 小/中/大/特大 map to S/M/L/XL. Multi Ekicube sizes are grouped into `size_class` S/M/L/XL by this CLI (the raw `size_key` is kept). `--size L` also keeps XL.
- **Live empties.** Multi Ekicube counts reservable boxes only: 0 reservable does not mean full. `usage_fee_yen` and `reservation_fee_yen` (¥500, only for online booking) are separate fields; the source does not state the walk-up price at the machine. A Coin Locker Navi record gets Multi Ekicube live data from the nearest bank within 30 m whose gate side does not contradict the record's own 改札内/改札外 text; it may be a neighbouring bank (`match_basis` says so, and the table LIVE cell names the bank area). Maihama blocks match by exact locker id and show physical empties.
- **Fair use.** The CLI makes on-demand requests only, about one request per second per host, with no bulk sync. The Coin Locker Navi terms forbid copying the data beyond private use.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
coinlocker-navi-pp-cli near 東京駅 --limit 5

# JSON for scripting and agents
coinlocker-navi-pp-cli near 東京駅 --limit 5 --json
# Filter to specific fields
coinlocker-navi-pp-cli near 東京駅 --limit 5 --json --select meta.fetched_at,results.id,results.name,results.sizes

# Dry run — show the requests without sending
coinlocker-navi-pp-cli near 東京駅 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
coinlocker-navi-pp-cli near 東京駅 --limit 5 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only** - this CLI never changes source data; only `--deliver webhook:` and opt-in `feedback --send` POST output elsewhere
- **Live only** - every lookup goes to the source on demand; there is no bulk sync
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited or blocked, `10` config error.

## Health Check

```bash
coinlocker-navi-pp-cli doctor
```

Verifies configuration and connectivity to coinlocker-navi.com.

## Configuration

Run `coinlocker-navi-pp-cli doctor` to see the resolved config, data, state, and cache directories. `--home`, `COINLOCKER_NAVI_HOME`, and per-kind env vars can relocate it.

Static request headers under `headers` apply only to the raw `source` commands; `near`, `locker` and `vacancy` send fixed browser-like headers.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the locker id is correct (digits only, e.g. 2685)
- Use `near <keyword>` to find locker ids

### API-specific
- **Most fields are null** — The source shows 情報なし for that locker; check locker <id> source_url on site
- **vacancy shows 0 empty but lockers look free** — Multi Ekicube counts reservable boxes only; walk-up boxes can still be free on site
- **near --gate returns few or no rows** — Gate side is known only where exactly one Multi Ekicube bank is within 15 m; other rows are dropped and counted in meta.excluded.gate_unknown
- **Exit code 7 (rate limited or blocked)** — The source throttled the request; wait a minute and retry with a smaller --limit or --max-pages

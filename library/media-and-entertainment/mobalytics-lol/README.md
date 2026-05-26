# Mobalytics LoL CLI

**Every League of Legends aggregator, plus a local SQLite for cross-champion queries no single page surfaces.**

mobalytics-lol-pp-cli pulls champion data, builds, runes, counters, and tier ratings from Mobalytics and Riot Data Dragon, hydrates them into a local SQLite store, then exposes cross-champion SQL queries no aggregator page surfaces: counter-pool matrices, patch meta-shift deltas, head-to-head compares, region-split tier views, and ARAM batch item-set exports for the LoL client. Free, offline-after-sync, agent-native (--json + --select + --agent), and the only LoL data source agentically accessible without a headless browser.

Learn more at [Mobalytics LoL](https://mobalytics.gg/lol).

Printed by [@QuantumGlitch](https://github.com/QuantumGlitch) (QuantumGlitch).

## Install

The recommended path installs both the `mobalytics-lol-pp-cli` binary and the `pp-mobalytics-lol` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install mobalytics-lol
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install mobalytics-lol --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install mobalytics-lol --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install mobalytics-lol --agent claude-code
npx -y @mvanhorn/printing-press-library install mobalytics-lol --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/cmd/mobalytics-lol-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mobalytics-lol-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-mobalytics-lol --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-mobalytics-lol --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-mobalytics-lol skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-mobalytics-lol. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mobalytics-lol-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/cmd/mobalytics-lol-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mobalytics-lol": {
      "command": "mobalytics-lol-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Hydrate the local store with the latest patch's champions, items, runes, tier snapshots, builds, and matchups
mobalytics-lol-pp-cli sync --full


# Look up the recommended Aatrox top build for Emerald+ — the single most common ritual
mobalytics-lol-pp-cli champion build aatrox


# Diamond+ mid-lane tier list with WR/PR/BR
mobalytics-lol-pp-cli tier-list --role mid


# Pool-vs-pool matchup matrix coaches do in their head
mobalytics-lol-pp-cli counter-pool --our darius,aatrox,garen --their fiora,sett,renekton


# What moved up or down since two patches ago
mobalytics-lol-pp-cli meta-shift --since-patch 14.10 --agent --select winner,loser,delta

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-pool draft analysis
- **`counter-pool`** — Given our champion pool and the enemy pool, return the full matchup matrix ranked by WR delta with sample-size floor — the SQL coaches do in their head.

  _Reach for this when an agent or coach needs to know 'given two confirmed champion pools, who picks into whom.' Avoids N×M page loads._

  ```bash
  mobalytics-lol-pp-cli counter-pool --our darius,aatrox,garen --their fiora,sett,renekton --agent
  ```
- **`meta-shift`** — Diff two tier_snapshots and list champions that gained or lost ≥1 tier or ≥2% WR since a prior patch, with sample-size guard.

  _Use when an agent or coach needs to answer 'what changed this patch' without manually diffing tier-list screenshots._

  ```bash
  mobalytics-lol-pp-cli meta-shift --since-patch 14.10 --role top --agent --select winner,loser,delta
  ```
- **`compare`** — Side-by-side join across tier, build, counters, and matchups for two champions — including item-overlap percentage.

  _Resolves draft-time 'which of these two is better right now' questions in one command._

  ```bash
  mobalytics-lol-pp-cli compare aatrox darius
  ```
- **`duo-finder`** — Best support pairings for a given ADC, restricted to a candidate support pool — coaches' player pools are fixed.

  _Use when the support pool is fixed (coach, duo queue, ranked flex) and you need WR-ranked picks from that subset._

  ```bash
  mobalytics-lol-pp-cli duo-finder --bot jinx --supports-from lulu,nami,leona
  ```

### Build artifact export
- **`item-set`** — Write LoL client item-set JSON for N champions in ARAM mode in one command, refreshing all stale files in the client config folder.

  _Use when an ARAM player wants game-ready item-sets across their reroll pool without alt-tabbing between champ select and a website._

  ```bash
  mobalytics-lol-pp-cli item-set --aram aatrox,jinx,lulu,yasuo,sett --to client
  ```

### Daily-ritual collapse
- **`pool-digest`** — For each champion in your pool: current tier, WR delta since last patch, top-1 counter, top-1 synergy — all in one composite query.

  _Use as a morning ritual or pre-queue command to surface what changed about your specific champions._

  ```bash
  mobalytics-lol-pp-cli pool-digest --pool darius,aatrox,garen --agent
  ```

### Mobalytics signature features
- **`power-spike`** — List champions ranked by early/mid/late spike strength for a chosen role — inverts Mobalytics's per-champion data into a leaderboard.

  _Use when picking around a teammate's pick (ARAM, draft) to find spike-aligned champions._

  ```bash
  mobalytics-lol-pp-cli power-spike --phase early --role jungle --top 20 --agent
  ```

### Draft toolkit
- **`flex`** — Champions that are ≥A-tier in 2+ roles for the same patch and rank — the SQL no aggregator surfaces because they index by role first.

  _Use when drafting champions who don't tip your hand about role — invaluable for fearless-draft formats and amateur scrims._

  ```bash
  mobalytics-lol-pp-cli flex --min-roles 2
  ```
- **`tier-list`** — Same patch, same rank — three regions side-by-side. Surfaces pick-priority drift between KR/EUW/NA that no aggregator defaults to.

  _Use to spot regional meta divergence — KR is usually 2 weeks ahead of NA, and this surfaces it in one view._

  ```bash
  mobalytics-lol-pp-cli tier-list --compare-regions kr,euw,na
  ```

## Usage

Run `mobalytics-lol-pp-cli --help` for the full command reference and flag list.

## Commands

### cdn

Manage cdn


### versions-json

Manage versions json

- **`mobalytics-lol-pp-cli versions-json`** - Returns the full list of LoL patch versions known to Data Dragon, newest
first. The first element is the current patch.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
mobalytics-lol-pp-cli versions-json

# JSON for scripting and agents
mobalytics-lol-pp-cli versions-json --json

# Filter to specific fields
mobalytics-lol-pp-cli versions-json --json --select id,name,status

# Dry run — show the request without sending
mobalytics-lol-pp-cli versions-json --dry-run

# Agent mode — JSON + compact + no prompts in one flag
mobalytics-lol-pp-cli versions-json --agent
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
mobalytics-lol-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/mobalytics-lol-cli-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Empty results from any Mobalytics command** — Run `mobalytics-lol-pp-cli sync --full` once after first install; the local store has to be hydrated before queries return data.
- **tier-list returns stale tier ratings** — Run `mobalytics-lol-pp-cli sync --resources tier_snapshots --since 1d` to refresh; tier data moves daily as games are played.
- **HTTP 422 when calling Mobalytics commands** — Mobalytics GraphQL rejects requests without `Origin: https://mobalytics.gg`; verify with `mobalytics-lol-pp-cli doctor`.
- **item-set --to client doesn't show in League client** — Restart the League client after writing item sets; the client only reads the config folder on startup. Use `--to file` for ad-hoc JSON export instead.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://ddragon.leagueoflegends.com/api/versions.json
- Capture coverage: 6 API entries from 6 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Candidate command ideas: list_Aatrox.json — Derived from observed GET /cdn/16.10.1/data/en_US/champion/Aatrox.json traffic.; list_champion.json — Derived from observed GET /cdn/16.10.1/data/en_US/champion.json traffic.; list_item.json — Derived from observed GET /cdn/16.10.1/data/en_US/item.json traffic.; list_runesReforged.json — Derived from observed GET /cdn/16.10.1/data/en_US/runesReforged.json traffic.; list_summoner.json — Derived from observed GET /cdn/16.10.1/data/en_US/summoner.json traffic.; list_versions.json — Derived from observed GET /api/versions.json traffic.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Championify**](https://github.com/dustinblackman/Championify) — JavaScript (800 stars)
- [**golio**](https://github.com/KnutZuidema/golio) — Go (89 stars)
- [**op.gg**](https://op.gg/lol/champions) — Web
- [**u.gg**](https://u.gg/) — Web
- [**lolalytics.com**](https://lolalytics.com/) — Web
- [**League of Graphs**](https://www.leagueofgraphs.com/champions/stats) — Web
- [**League-of-Legends-MCP**](https://github.com/kostadindev/League-of-Legends-MCP) — TypeScript
- [**mcp-riot**](https://github.com/jifrozen0110/mcp-riot) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

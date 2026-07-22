---
name: pp-mobalytics-lol
description: "Every League of Legends aggregator, plus a local SQLite for cross-champion queries no single page surfaces. Trigger phrases: `look up the build for {champion}`, `what's the tier list for {role}`, `who counters {champion} in {role}`, `compare {champion1} and {champion2}`, `what changed in the meta this patch`, `use mobalytics-lol`, `run mobalytics-lol-pp-cli`."
author: "QuantumGlitch"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - mobalytics-lol-pp-cli
    install:
      - kind: go
        bins: [mobalytics-lol-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/cmd/mobalytics-lol-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/mobalytics-lol/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Mobalytics LoL — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `mobalytics-lol-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install mobalytics-lol --cli-only
   ```
2. Verify: `mobalytics-lol-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/cmd/mobalytics-lol-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent needs aggregator-quality LoL champion data without a headless browser, when you want to compose cross-champion queries (pool vs pool, patch diffs, flex pickers) no aggregator page surfaces, or when you want ARAM/SR item-sets exported to the LoL client in batch. Reach for it during draft analysis, agentic Discord-bot prompts about 'who beats X this patch', morning-ritual pool digests, or coach pre-scrim preparation. Don't use it for live game data (Riot's match API is the right source) or personal summoner profiles (Mobalytics Plus's overlay is the right tool).

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 6 API entries from 6 total network entries
- Protocols: rest_json (75% confidence)
- Candidate command ideas: list_Aatrox.json — Derived from observed GET /cdn/16.10.1/data/en_US/champion/Aatrox.json traffic.; list_champion.json — Derived from observed GET /cdn/16.10.1/data/en_US/champion.json traffic.; list_item.json — Derived from observed GET /cdn/16.10.1/data/en_US/item.json traffic.; list_runesReforged.json — Derived from observed GET /cdn/16.10.1/data/en_US/runesReforged.json traffic.; list_summoner.json — Derived from observed GET /cdn/16.10.1/data/en_US/summoner.json traffic.; list_versions.json — Derived from observed GET /api/versions.json traffic.

## Command Reference

**cdn** — Manage cdn


**versions-json** — Manage versions json

- `mobalytics-lol-pp-cli versions-json` — Returns the full list of LoL patch versions known to Data Dragon, newest first. The first element is the current patch.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
mobalytics-lol-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Diamond+ jungle picks that spike early

```bash
mobalytics-lol-pp-cli power-spike --phase early --role jungle --top 10 --agent
```

Inverts Mobalytics's per-champion power-spike data into a leaderboard scoped by role and rank — useful when picking around a teammate's pick or when an agent needs to recommend early-game junglers for a side comp.

### Pre-scrim pool-vs-pool matrix as JSON

```bash
mobalytics-lol-pp-cli counter-pool --our darius,aatrox,garen --their fiora,sett,renekton --agent --select our,their,wr,sample
```

Coaches' classic question — given our 3 picks and their 3 picks in top lane, what's the full 3×3 matchup matrix with sample sizes. The --select keeps only the four columns an agent needs.

### Export ARAM item-sets to the LoL client for tonight's session

```bash
mobalytics-lol-pp-cli item-set --aram aatrox,jinx,lulu,yasuo,sett,thresh,brand --to client
```

Replaces the dead Championify workflow for ARAM: one command writes 7 game-ready item-set JSON files into the LoL client config folder. No browser, no copy-paste, restart client to use.

### What moved up since last patch in mid lane

```bash
mobalytics-lol-pp-cli meta-shift --since-patch 16.9 --role mid --agent
```

Diffs the current tier_snapshots against the snapshot from patch 16.9. The --agent flag returns structured JSON of winners (gained ≥1 tier) and losers, with sample guard so noise gets filtered.

### Agent draft prompt — head-to-head with field narrowing

```bash
mobalytics-lol-pp-cli compare aatrox darius --agent --select tier,winrate,top_counter,top_synergy,build_overlap_pct
```

A Discord-bot or Claude-driven draft assistant pipes this through jq to surface 'which is the better top pick right now' in one call. The --select keeps the payload tight enough for agent context windows even when both champions have 50+ matchup rows in the underlying tables.

## Auth Setup

No authentication required.

Run `mobalytics-lol-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  mobalytics-lol-pp-cli versions-json --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
mobalytics-lol-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
mobalytics-lol-pp-cli feedback --stdin < notes.txt
mobalytics-lol-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.mobalytics-lol-pp-cli/feedback.jsonl`. They are never POSTed unless `MOBALYTICS_LOL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MOBALYTICS_LOL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
mobalytics-lol-pp-cli profile save briefing --json
mobalytics-lol-pp-cli --profile briefing versions-json
mobalytics-lol-pp-cli profile list --json
mobalytics-lol-pp-cli profile show briefing
mobalytics-lol-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `mobalytics-lol-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/cmd/mobalytics-lol-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add mobalytics-lol-pp-mcp -- mobalytics-lol-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which mobalytics-lol-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   mobalytics-lol-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `mobalytics-lol-pp-cli <command> --help`.

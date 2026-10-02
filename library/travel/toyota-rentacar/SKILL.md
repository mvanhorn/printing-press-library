---
name: pp-toyota-rentacar
description: "Live Japan rental class estimates, distinct shops, and a careful booking handoff. Trigger phrases: `Toyota rental quote for my dates`, `Toyota pickup and return shops`, `Toyota one-way surcharge`, `Toyota ETC and child-seat fees`, `use toyota-rentacar`, `run toyota-rentacar`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - toyota-rentacar-pp-cli
    install:
      - kind: go
        bins: [toyota-rentacar-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/cmd/toyota-rentacar-pp-cli
---

# TOYOTA Rent a Car — Printing Press CLI

This is a local verified build. If the binary is missing,build it from the Toyota checkout with `go build -o toyota-rentacar-pp-cli ./cmd/toyota-rentacar-pp-cli` and use that binary. The public-library commands below apply only after publication.

## Prerequisites: Install the CLI

This skill drives the `toyota-rentacar-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install toyota-rentacar --cli-only
   ```
2. Verify: `toyota-rentacar-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/cmd/toyota-rentacar-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Resolve pickup and return shops, compare real dated class offers, and inspect Toyota’s one-way surcharge and option policies. Final totals and individual driving eligibility remain explicit until Toyota verifies them.

## When to Use This CLI

Use for Toyota Japan rental planning: distinct shops,dated class estimates,one-way fee simulation and official option/document guidance. Recheck live inventory before booking.

## Anti-triggers

Do not use this CLI for:
- Creating,cancelling or modifying reservations
- Payment,customer profiles or entering personal documents
- Guaranteed vehicle model or final inclusive price
- Other rental suppliers or road toll calculations

## Unique Capabilities

These commands provide Toyota-specific rental planning flows.

### Rental planning
- **`shops search`** — Resolve distinct Toyota branches and return restrictions.

  _Preserves first-party branch identities and Japanese names._

  ```bash
  toyota-rentacar-pp-cli shops search --keyword "Kyoto Station" --limit 3 --agent
  ```
- **`cars quote`** — Compare real dated class offers with explicit price assumptions.

  _Verifies source-echoed dates and shops before reporting offers._

  ```bash
  toyota-rentacar-pp-cli cars quote --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --category compact --agent
  ```
- **`rental options`** — Read live option and waiver fees with source units.

  _Separates optional fee estimates from unknown final totals._

  ```bash
  toyota-rentacar-pp-cli rental options --agent
  ```
- **`oneway quote`** — Run Toyota’s route surcharge simulator.

  _Keeps date-independent fees separate from dated class stock._

  ```bash
  toyota-rentacar-pp-cli oneway quote --pickup-shop 63601:01V --dropoff-shop 63601:095 --family standard --agent
  ```
- **`booking handoff`** — Print a canonical booking link and reusable rental checklist.

  _Date and options survive as a checklist rather than false session-bound links._

  ```bash
  toyota-rentacar-pp-cli booking handoff --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --agent
  ```

## Planning contract

Resolve shops first; preserve the full company:branch ID and Japanese name. Use future JST dates with30-minute increments,within three months and at most one calendar month per rental. Replace example dates with the actual trip dates. Source operating windows do not imply vehicle stock slots.

`cars quote` reports source class estimates and explicit available/fully_booked/unknown status. Full confirmed totals stay null,and representative models are not guaranteed. Optional waiver/NOC,ETC/JAF,child seats,tires,model selection,one-way fees,tolls and fuel must be checked in Toyota's final breakdown. `rental options` gives live policy fees and basis,not option inventory. `oneway quote` is date-independent and does not guarantee a bookable route/class. `rental eligibility` is document guidance; individual eligibility is not assessed.

Use `booking handoff` for the canonical pickup-shop link and exact date/options checklist. Its URL does not preserve dates,return shop,class or extras; re-enter those on Toyota. Complete customer details,terms and payment yourself. No CLI command books,pays or accepts terms.

Live result metadata includes fetched_at,upstream_requests,response_bytes_read,elapsed_ms and the request cap. Use `--select` to keep just the needed fields. `--data-source local` is rejected by live Toyota planning commands. Anonymous session/form state stays in memory and is never saved.

## Command Reference

**source** — Public first-party source documents; use shops/cars/rental/oneway for structured planning

- `toyota-rentacar-pp-cli source eligibility` — Read official driving-document guidance
- `toyota-rentacar-pp-cli source insurance` — Read insurance and NOC policy source
- `toyota-rentacar-pp-cli source locations` — Read public keyword shop document
- `toyota-rentacar-pp-cli source one-way` — Read one-way restrictions and simulator link
- `toyota-rentacar-pp-cli source options` — Read option policy source


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
toyota-rentacar-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Resolve shops

```bash
toyota-rentacar-pp-cli shops search --keyword "Tokyo Station" --limit 3 --agent --select shops.id,shops.name,shops.name_jp,shops.oneway_returns
```

Keep branch identities and return restrictions.

### Dated compact offers

```bash
toyota-rentacar-pp-cli cars quote --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --category compact --agent
```

Class prices are source estimates; models are representative.

### One-way surcharge

```bash
toyota-rentacar-pp-cli oneway quote --pickup-shop 63601:01V --dropoff-shop 63601:095 --family standard --agent
```

Calculator fee has no dated inventory guarantee.

### Eligibility guidance

```bash
toyota-rentacar-pp-cli rental eligibility --agent
```

Read first-party document rules;Toyota verifies the real documents.

## Auth Setup

Public read-only HTTP. No API key,account login,browser dependency,or saved cookies.

Run `toyota-rentacar-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  toyota-rentacar-pp-cli source eligibility --agent
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `TOYOTA_RENTACAR_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TOYOTA_RENTACAR_CONFIG_DIR`, `TOYOTA_RENTACAR_DATA_DIR`, `TOYOTA_RENTACAR_STATE_DIR`, `TOYOTA_RENTACAR_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TOYOTA_RENTACAR_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `toyota-rentacar-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "toyota-rentacar": {
        "command": "toyota-rentacar-pp-mcp",
        "env": {
          "TOYOTA_RENTACAR_HOME": "/srv/toyota-rentacar"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TOYOTA_RENTACAR_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TOYOTA_RENTACAR_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

The generated framework can remember public query structures locally. Use `--no-learn` or `TOYOTA_RENTACAR_NO_LEARN=true` for deterministic flows. Learning and stored output never replace a fresh Toyota inventory check. Inspect the current framework commands with `--help` before using them.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
toyota-rentacar-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
toyota-rentacar-pp-cli feedback --stdin < notes.txt
toyota-rentacar-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TOYOTA_RENTACAR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TOYOTA_RENTACAR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
toyota-rentacar-pp-cli profile save briefing --json
toyota-rentacar-pp-cli --profile briefing source eligibility
toyota-rentacar-pp-cli profile list --json
toyota-rentacar-pp-cli profile show briefing
toyota-rentacar-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `toyota-rentacar-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/cmd/toyota-rentacar-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add toyota-rentacar-pp-mcp -- toyota-rentacar-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which toyota-rentacar-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   toyota-rentacar-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `toyota-rentacar-pp-cli <command> --help`.

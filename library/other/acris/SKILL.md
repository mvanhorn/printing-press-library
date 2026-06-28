---
name: pp-acris
description: "Query NYC property records — deeds, mortgages, parties, and full BBL history — straight from NYC Open Data, with cross-dataset joins no raw ACRIS query can do. Trigger phrases: `who owns this NYC property`, `mortgage history for this address`, `ACRIS document search`, `find deeds recorded by this LLC`, `NYC property records lookup`, `use acris`, `run acris`."
author: "not0xjarvis"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - acris-pp-cli
    install:
      - kind: go
        bins: [acris-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/acris/cmd/acris-pp-cli
---

# ACRIS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `acris-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install acris --cli-only
   ```
2. Verify: `acris-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/acris/cmd/acris-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

ACRIS publishes its property records as four separate NYC Open Data datasets (Master, Legals, Parties, Document Control Codes). This CLI gives you direct, scriptable access to each, plus joined commands — `bbl`, `debt`, `document`, `party-search` — that chain the datasets the way a title search actually needs.

## When to Use This CLI

Use this CLI when an agent needs NYC real-property record data: who owns or recorded against a property, when it last traded, what mortgages or debt instruments are recorded, or which documents name a given party. It is the right tool for title-research, due-diligence, and portfolio-monitoring workflows over NYC deeds and mortgages.

## Anti-triggers

Do not use this CLI for:
- Do not use for property valuations, assessments, or tax bills — that is NYC DOF assessment data, not ACRIS.
- Do not use for zoning, permits, or violations — those are separate NYC datasets (Zola, DOB).
- Do not use for property records outside the five NYC boroughs — ACRIS is NYC-only.
- Do not treat results as legal title opinion — ACRIS is the recording index, not a guarantee of title.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-dataset joins
- **`bbl`** — Resolve a borough/block/lot to its full recorded-document history in one call.

  _When an agent has an address or BBL and needs the chain of title and recordings, this is the single command that returns it._

  ```bash
  acris-pp-cli bbl --borough 1 --block 852 --lot 134 --json
  ```
- **`debt`** — List the mortgage and debt-instrument recordings for a BBL with amounts and dates.

  _Surfaces apparent debt attached to a property without manually cross-referencing three datasets._

  ```bash
  acris-pp-cli debt --borough 1 --block 852 --lot 134 --json
  ```
- **`document`** — Assemble one document's master record, all its property (BBL) legals, and all its parties into a single object.

  _Gives an agent the complete picture of a single recording in one call instead of three._

  ```bash
  acris-pp-cli document 2023012300123001 --json
  ```

### Entity discovery
- **`party-search`** — Find recorded documents by partial party name (grantor, grantee, mortgagor, mortgagee).

  _Entity-driven discovery: find every document an LLC or individual is named on without knowing the exact recorded spelling._

  ```bash
  acris-pp-cli party-search --name "MADISON" --limit 25 --json
  ```

## Command Reference

**doc-types** — Document type code lookup — translate ACRIS doc_type codes to descriptions and classes (ACRIS Document Control Codes, 7isb-wh4c)

- `acris-pp-cli doc-types` — List document control codes, optionally filtered by class

**documents** — Recorded document master records — deeds, mortgages, assignments, satisfactions (ACRIS Real Property Master, bnx9-e6tj)

- `acris-pp-cli documents get` — Fetch a single document master record by document ID
- `acris-pp-cli documents list` — List or filter recorded documents by type, borough, CRFN, or SoQL query

**legals** — Document-to-property (borough/block/lot) mappings — resolve a BBL to its recorded documents (ACRIS Real Property Legals, 8h5j-fqxa)

- `acris-pp-cli legals` — List property legal records by BBL, address, or document ID

**parties** — Parties (grantors, grantees, mortgagors, mortgagees) named on recorded documents (ACRIS Real Property Parties, 636b-3b5g)

- `acris-pp-cli parties` — List parties by document ID, exact name, or party type


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
acris-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Mortgage history for a property

```bash
acris-pp-cli debt --borough 1 --block 852 --lot 134 --json
```

Lists mortgage and debt-instrument recordings for the BBL with amounts and dates.

### Find documents naming a party

```bash
acris-pp-cli party-search --name "MADISON" --limit 25 --json
```

Substring search over recorded party names; returns the documents they appear on.

### Recent high-value deeds in Manhattan

```bash
acris-pp-cli documents list --doc-type DEED --recorded-borough 1 --where "document_amt > 10000000" --order "recorded_datetime DESC" --json --select document_id,document_amt,recorded_datetime
```

Combines a server-side SoQL filter with --select to keep the payload small for agents.

### Full picture of one recording

```bash
acris-pp-cli document 2023012300123001 --json
```

Merges the master record, all property legals (BBLs), and all parties for a single document.

## Auth Setup
Run `acris-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export ACRIS_APP_TOKEN="<your-key>"
```

To persist credentials, use `acris-pp-cli auth set-token <token>`. Stored secrets live in `credentials.toml` under the data dir, not in `config.toml`.

Run `acris-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  acris-pp-cli doc-types --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `ACRIS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ACRIS_CONFIG_DIR`, `ACRIS_DATA_DIR`, `ACRIS_STATE_DIR`, `ACRIS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ACRIS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `acris-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "acris": {
        "command": "acris-pp-mcp",
        "env": {
          "ACRIS_HOME": "/srv/acris"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ACRIS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ACRIS_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
acris-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
acris-pp-cli feedback --stdin < notes.txt
acris-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ACRIS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ACRIS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
acris-pp-cli profile save briefing --json
acris-pp-cli --profile briefing doc-types
acris-pp-cli profile list --json
acris-pp-cli profile show briefing
acris-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `acris-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/acris/cmd/acris-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add acris-pp-mcp -- acris-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which acris-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   acris-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `acris-pp-cli <command> --help`.

---
name: pp-woot
description: "Printing Press CLI for Woot. Search live deals, sync them locally, and query the read-only browser GraphQL API."
author: "Matthew Vassallo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - woot-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/woot/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Woot — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `woot-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install woot --cli-only
   ```
2. Verify: `woot-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/woot/cmd/woot-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Find offers that are buried deep in Woot's paginated All Deals catalog, then save a locally searchable snapshot with explicit completeness metadata. The CLI exposes the browser-observed searchOffers query without allowing GraphQL mutations.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Live deal discovery
- **`deals`** — Scan Woot's paged All Deals results and filter live offers by keyword.

  _Use this when Woot's visible All Deals pages contain an offer that a single GraphQL page or direct offer lookup would miss; scan metadata reports short windows, duplicate IDs, and rows without identities._

  ```bash
  woot-pp-cli deals rayon --limit 10000 --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 7 API entries from 7 total network entries
- Protocols: graphql (92% confidence)
- Auth signals: api_key — headers: x-api-key
- Candidate command ideas: list_graphql — Derived from observed GET /graphql traffic.

## Recipes

### Search a filtered slice of All Deals

```bash
woot-pp-cli deals laptop --category computers --price-range 50-100 --limit 500 --agent
```

Scan current computer deals priced from $50 to $100 and return title matches as compact JSON.

## Command Reference

**deals** — Search Woot All Deals offers

- `woot-pp-cli deals [keyword]` — scan Woot's paged All Deals GraphQL results up to `--limit`, then filter titles, slugs, and item attributes locally.
- `woot-pp-cli deals [keyword] --from-url <woot-alldeals-url>` — reuse category, price, and page filters from a copied Woot `/alldeals` URL.
- `woot-pp-cli deals [keyword] --category sport --price-range under-25 --price-range 25-50 --page 13` — reproduce visible All Deals filters from CLI flags.

Machine output reports `scanned`, `unique_scanned`, `duplicate_rows`, `missing_id_rows`, `expected_scan`, and `incomplete`. The completeness flag covers the requested live scan window; it does not claim that a smaller `--limit` covered Woot's entire catalog. Repeated IDs are omitted from results, and human table output warns when a requested window is incomplete.

**graphql** — Run read-only Woot GraphQL queries

- `woot-pp-cli graphql` — fetch one current All Deals offer with a default `searchOffers` query.
- `woot-pp-cli graphql --query '<query>'` — run a custom read-only Woot GraphQL query; mutation and subscription documents are rejected.

**sync and search** — Build and query an offline Woot deal index

- `woot-pp-cli sync --full` — start at the head of All Deals, store normalized offers and prices, and prune expired rows only after two consecutive full scans return the same complete set of deal IDs. If Woot changes between scans, the CLI preserves existing rows and marks the snapshot incomplete for another pass.
- `woot-pp-cli search '<query>' --type deals --data-source local` — search the local deal catalog without another Woot request.

Woot's mutable offset feed can repeat IDs or return fewer unique rows than `TotalHits`. Live scans expose this explicitly, and an incomplete sync warning is a deliberate non-success: local matches remain useful, but missing local matches are not authoritative until a later sync verifies the catalog.

`--no-prune` deliberately retains rows outside the verified live ID set. The store remains marked incomplete while those rows exist; use `sync --full` without `--no-prune` to publish an exact current local snapshot.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
woot-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Capture the x-api-key value from a successful graphql request made by your own Woot All Deals browser session, then provide it through D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY or auth set-token.

Run `woot-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  woot-pp-cli graphql --agent --select data.searchOffers.TotalHits
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "local", "synced_at": "...", "reason": "...", "incomplete": true, "resume_cursor": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. When `.meta.incomplete` is true, run sync again before treating a missing local match as authoritative; `.meta.resume_cursor` records where that retry will begin. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `WOOT_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WOOT_CONFIG_DIR`, `WOOT_DATA_DIR`, `WOOT_STATE_DIR`, `WOOT_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WOOT_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `woot-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "woot": {
        "command": "woot-pp-mcp",
        "env": {
          "WOOT_HOME": "/srv/woot"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WOOT_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WOOT_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
woot-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
woot-pp-cli feedback --stdin < notes.txt
woot-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WOOT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WOOT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
woot-pp-cli profile save briefing --json
woot-pp-cli --profile briefing graphql
woot-pp-cli profile list --json
woot-pp-cli profile show briefing
woot-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `woot-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add woot-pp-mcp -- woot-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which woot-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   woot-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `woot-pp-cli <command> --help`.

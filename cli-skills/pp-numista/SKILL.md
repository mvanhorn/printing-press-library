---
name: pp-numista
description: "Every Numista catalogue and collection feature, with offline FTS5 search and a monthly-quota tracker no Numista SDK has. Trigger phrases: `look up a coin on numista`, `what is my coin collection worth`, `track numista prices`, `check my numista quota`, `import my coins into numista`, `browse numista series`, `use numista-pp-cli`, `run numista-pp-cli`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - numista-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/numista/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Numista — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `numista-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install numista --cli-only
   ```
2. Verify: `numista-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/numista/cmd/numista-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for numista-pp-cli when you have a Numista API key and need quota-aware automation over the catalogue and your collection. Ideal for: bulk-importing a collection from another tracker, running scheduled price refreshes on watched coins, computing a current valuation of an entire collection, or pulling a full series (every year, every grade) for one type into a local store for analysis. Skip it when one ad-hoc lookup is all you need — the Numista web UI is fine for that.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Quota economics
- **`--quota`** — Print the current month's used/remaining/reset Numista quota and exit, with zero API calls.

  _Reach for this before any batch or crawl to know whether the operation fits today's budget — zero quota cost._

  ```bash
  numista-pp-cli --quota --json
  ```
- **`audit`** — Query the local lookup_log directly to see every API call, its endpoint, duration, and cache-hit status — aggregated by day, endpoint, or type ID.

  _Use this to diagnose unexpected quota consumption, find duplicate lookups worth caching, or audit which endpoints power which workflows._

  ```bash
  numista-pp-cli audit --by-endpoint --json
  ```
- **`types batch`** — Parse a CSV / JSONL / text list of Numista type IDs (N# numbers) and look up each one against the API — with cache reuse, --dry-run cost forecast, and --resumable splitting across UTC months.

  _Use any time you have more than one type ID to look up. Dry-run is always cheap; resumable is essential for >2K-item lists._

  ```bash
  numista-pp-cli types batch --file ./type-ids.csv --dry-run --json
  ```

### Local state that compounds
- **`types series`** — For one type (e.g., N#11013 — Australia 3 pence George VI), pull every year-of-issue, every issue's prices across all grades, and persist the full price/mintage curve to the local store — then print the scarcity and price-evolution table.

  _Reach for this when a user wants the full picture of one coin type — every year it was struck, every grade's current value — in one command instead of dozens of individual calls._

  ```bash
  numista-pp-cli types series 11013 --json --select issues.year,issues.mintage,prices.grade,prices.price
  ```
- **`collection value`** — Sum the current estimated value of every item in a user's Numista collection, fetching missing prices on demand, and emit a per-item breakdown sorted by value.

  _Use to answer 'what is my collection worth right now?' without scrolling through Numista's web UI. Refuses to start when remaining quota is less than the number of items needing fresh prices._

  ```bash
  numista-pp-cli collection value 12345 --json
  ```
- **`refresh`** — Refresh cached types by re-fetching only fields that actually change (prices, mintage) while leaving cataloger-set identity fields untouched. --dry-run --older 30d lists what needs refresh without spending a call.

  _Use weekly or monthly to keep cached prices current without re-pulling the entire catalogue. --dry-run --older makes the cost predictable._

  ```bash
  numista-pp-cli refresh --all --older 30d --json
  ```
- **`crawl issuer`** — Crawl every type from one issuer (e.g., 'australia') matching a year range, persist to local store, and print a summary table. Forecasts call cost as %-of-monthly-quota and requires confirmation before crawling.

  _Reach for this when starting research on an issuer or period — pay the call cost once, then ask the local store any question for free._

  ```bash
  numista-pp-cli crawl issuer australia_section --years 1900-1950 --dry-run
  ```
- **`watchlist`** — Track price changes for a set of types over time. `watchlist add N#` registers a type; `watchlist check` refreshes prices, snapshots them to the prices table with a timestamp, and prints the diff since the last snapshot.

  _Run on a cron after adding a few types; the CLI surfaces material price moves without polluting your inbox._

  ```bash
  numista-pp-cli watchlist check --json
  ```

### Agent-native plumbing
- **`users collected-items add --from-file`** — Import a list of new collected items from CSV / JSONL with --dry-run cost forecast. Idempotent on (user_id, type_id, issue_id, grade) so re-running the same file is safe.

  _Use to migrate a collection from another tracker, or to bulk-add a haul from a coin show without dozens of UI clicks._

  ```bash
  numista-pp-cli users collected-items add 12345 --from-file imports.csv --dry-run
  ```
- **`users collections hydrate`** — Given a collection-folder-id, fan out get-item for every item, optionally fan out get-prices, and persist everything to the local store. Refuses to start when remaining quota is less than the item count.

  _Use once after `auth set-token` (with an OAuth bearer) to populate the local store with everything in your collection — then most subsequent reads are quota-free._

  ```bash
  numista-pp-cli users collections hydrate 12345 --with-prices --json
  ```

## Command Reference

**catalogues** — The API endpoints in this section allow to access the data of the Numista catalogue of coins, banknotes and exonumia.

- `numista-pp-cli catalogues` — Retrieve the list of all the reference catalogues used for cross-reference in the catalogue

**issuers** — Manage issuers

- `numista-pp-cli issuers` — Retrieve the details about all the issuing countries and territories

**mints** — Manage mints

- `numista-pp-cli mints get` — Retrieve the details about all the mints
- `numista-pp-cli mints get-mintid` — Retrieve the details about a specific mint.

**oauth-token** — Manage oauth token

- `numista-pp-cli oauth-token` — In order to access the data of a Numista user, you will need to authenticate using the OAuth 2.0 protocol. See the...

**publications** — Manage publications

- `numista-pp-cli publications <id>` — Retrieve the details about a specific item in the literature catalogue.

**types** — Manage types

- `numista-pp-cli types get` — Retrieve the details about a specific type in the catalogue.
- `numista-pp-cli types search` — Search the catalogue for coin, banknote, and exonumia types. At least one of the following parameters should be...

**users** — The API endpoints in this section allow to access data about the Numista users and their collection.

- `numista-pp-cli users <user_id>` — Get details about a user


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
numista-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Identify a coin from a fuzzy description

```bash
numista-pp-cli types search --q 'Australia 3 pence George VI' --json --select types.id,types.title,types.min_year,types.max_year
```

Narrow with `--select` to keep the payload tight; the dotted-path projection turns ~50 lines of JSON per result into 4.

### See what a coin is worth in every grade

```bash
numista-pp-cli types series 11013 --json
```

One command pulls every year + every grade's price into the local store; subsequent reads are free.

### Estimate this month's batch cost before running it

```bash
numista-pp-cli types batch --file ./watchlist.csv --dry-run --json
```

--dry-run forecasts live calls vs cache hits vs %-of-quota; never spends a call.

### Refresh only stale prices in your cache

```bash
numista-pp-cli refresh --all --older 30d --json
```

Re-fetches only types whose prices are older than 30 days; never touches identity fields.

### Total my collection's value right now

```bash
numista-pp-cli collection value 12345 --json --select totals.estimated_value,totals.currency,items.id,items.grade,items.estimated_value
```

Joins your locally-cached collection against the prices table; refuses to start if remaining quota would be exceeded.

### If your input is a PCGS cert

Pair with [`pcgs-pp-cli`](https://github.com/mvanhorn/printing-press-library/tree/main/library/other/pcgs) when you're starting from a PCGS-graded coin and need its Numista catalogue ID (N#). PCGS is one of Numista's reference catalogues — registered as catalogue id `1856` (code `PCGS`, title "PCGS CoinFacts") — so a PCGSNo resolves to an N# in one API call with no text-match guessing.

**Direct cross-walk (recommended when you have a PCGSNo).**

```bash
# 1. Get the PCGSNo from PCGS (no Numista quota cost).
pcgs-pp-cli coin facts-cert <cert-number> --json --select PCGSNo,Name,Year
# → e.g. {"PCGSNo":"7130","Name":"1881-S Morgan Dollar","Year":"1881"}

# 2. Look up the Numista N# directly via the catalogue cross-reference.
numista-pp-cli types search --catalogue 1856 --number 7130 \
  --agent --select types.id,types.title
# → {"results":{"types":[{"id":1492,"title":"1 Dollar \"Morgan Dollar\""}]}}
```

Verify the catalogue id at any time with `numista-pp-cli catalogues find pcgs` (local-only, no quota cost; requires that `numista-pp-cli catalogues` has been run at least once to populate the local cache).

**Text-search fallback (no PCGSNo, or the catalogue lookup misses).** The PCGS CoinFacts reference catalogue doesn't index every cert. When `--catalogue 1856 --number <PCGSNo>` returns no types, fall back to text search:

```bash
pcgs-pp-cli coin facts-cert <cert-number> --json --select Name,Year,CountryName
# → e.g. {"Name":"1881-S Morgan Dollar","Year":"1881","CountryName":"United States"}

numista-pp-cli types search --q "morgan dollar" --issuer united-states --date 1881 \
  --agent --select types.id,types.title
# → top result is usually the Numista N# you want.
```

Use `--date` (Gregorian calendar year) for the year PCGS returns, not `--year` — Numista's `--year` is the year *as written on the item* (relevant for Hijri / Republican / other non-Gregorian dating on world coins; for US coins they coincide).

Tips:

- Issuer slugs are hyphenated (`united-states`, `south-africa`). Run `numista-pp-cli issuers find <name>` to look one up — local-only, no quota cost.
- This CLI does NOT depend on `pcgs-pp-cli`. No shell-out, no auto-detection — install it separately when you want grade / cert / population data on the coin Numista returned.

## Auth Setup

Set `NUMISTA_API_KEY` in your environment (request one at https://en.numista.com/api/index.php — the free plan is 2000 calls/month). Catalogue and reference endpoints work with the API key alone. User-collection commands (`users collected-items add`, `collection value`, `users collections hydrate`) need an OAuth token — request an OAuth bearer with `numista-pp-cli oauth-token --grant-type client_credentials --scope view_collection` and save it via `numista-pp-cli auth set-token <token>` to grant the CLI access to your own account; the token is stored at ~/.numista-pp-cli/auth.json (mode 0600).

Run `numista-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  numista-pp-cli catalogues --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
numista-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
numista-pp-cli feedback --stdin < notes.txt
numista-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.numista-pp-cli/feedback.jsonl`. They are never POSTed unless `NUMISTA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NUMISTA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
numista-pp-cli profile save briefing --json
numista-pp-cli --profile briefing catalogues
numista-pp-cli profile list --json
numista-pp-cli profile show briefing
numista-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `numista-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add numista-pp-mcp -- numista-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which numista-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   numista-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `numista-pp-cli <command> --help`.

---
name: pp-drivethrurpg
description: "DriveThruRPG catalog search and authenticated library download CLI."
author: "Jason Holt"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - drivethrurpg-pp-cli
---

# DriveThruRPG - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `drivethrurpg-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install drivethrurpg --cli-only
   ```
2. Verify: `drivethrurpg-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drivethrurpg/cmd/drivethrurpg-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search the public DriveThruRPG catalog and download purchased files from an authenticated library.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Catalog discovery
- **`search`** — Search DriveThruRPG's public product catalog by keyword while keeping JSON output agent-friendly.

  _Agents can answer store-discovery questions before asking the user for credentials._

  ```bash
  drivethrurpg-pp-cli search "Cyberpunk" --agent --limit 5 --data-source live
  ```

### Authenticated library
- **`library`** — List products in your DriveThruRPG library, including file indexes for downloads.

  _Agents can inspect owned purchases and choose the right file index without scraping website pages._

  ```bash
  drivethrurpg-pp-cli library --agent --page-size 10
  ```
- **`download`** — Prepare, poll, and save a purchased DriveThruRPG file from a single command.

  _Agents can execute the full authenticated file retrieval workflow without hand-rolling polling logic._

  ```bash
  drivethrurpg-pp-cli download LIBRARY_PRODUCT_ID --dry-run --agent
  ```

## Command Reference

**catalog search** — Public DriveThruRPG store discovery

- `drivethrurpg-pp-cli search <query>` — Search public products through the live API or local synced data
- `drivethrurpg-pp-cli products search --keyword <term>` — Search public product cards with pagination and publisher filters
- `drivethrurpg-pp-cli products get <productId>` — Get public product details
- `drivethrurpg-pp-cli search-ahead --keyword <term>` — Search ahead across products, publishers, categories, and filters
- `drivethrurpg-pp-cli special-offers` — List active public special offers

**authenticated library** — Owned products and downloads

- `drivethrurpg-pp-cli auth login` — Exchange a DriveThruRPG Library App Application Key for a saved token
- `drivethrurpg-pp-cli auth status` — Show auth state without printing secrets
- `drivethrurpg-pp-cli library` — List products in your authenticated DriveThruRPG library
- `drivethrurpg-pp-cli download <libraryProductId> [fileIndex]` — Prepare, poll, and save a purchased file
- `drivethrurpg-pp-cli library prepare-download <libraryProductId> --index <n>` — Advanced raw prepare step
- `drivethrurpg-pp-cli library check-download <libraryProductId> --index <n>` — Advanced raw poll step

**categories** — Manage categories

- `drivethrurpg-pp-cli categories get-category` — Get public category details
- `drivethrurpg-pp-cli categories list` — List public categories

**filters** — Manage filters

- `drivethrurpg-pp-cli filters get` — Get public filter details
- `drivethrurpg-pp-cli filters list` — List public filters

**publishers** — Manage publishers

- `drivethrurpg-pp-cli publishers <publisherId>` — Get public publisher details

**reviews** — Manage reviews

- `drivethrurpg-pp-cli reviews` — List public reviews, optionally for a product

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
drivethrurpg-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Public catalog commands do not need credentials. Library and download commands need a DriveThruRPG Library App Application Key with My Library Access enabled.

Preferred setup:

```bash
export DRIVETHRURPG_APPLICATION_KEY="<application-key>"
drivethrurpg-pp-cli auth login
drivethrurpg-pp-cli auth status
```

For ephemeral agents or MCP hosts that already have an exchanged token:

```bash
export DRIVETHRURPG_DTRPG_TOKEN="<exchanged-token>"
```

Do not print tokens. Run `drivethrurpg-pp-cli doctor --json` to verify setup.

## Cookbook

```bash
# Search the public catalog
drivethrurpg-pp-cli search "Legend of the Five Rings core rulebook" --agent --limit 5 --data-source live

# Store search with product filters
drivethrurpg-pp-cli products search --keyword "Cyberpunk" --page-size 5 --agent

# List owned library products and file indexes
drivethrurpg-pp-cli library --page-size 10 --agent

# Download the first file for a product in your library
drivethrurpg-pp-cli download <libraryProductId> 0 --output-dir ~/Downloads --agent

# Prepare without downloading
drivethrurpg-pp-cli download <libraryProductId> 0 --url-only --agent

# Inspect public reviews
drivethrurpg-pp-cli reviews --product-id <productId> --agent
```

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  drivethrurpg-pp-cli categories list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
drivethrurpg-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
drivethrurpg-pp-cli feedback --stdin < notes.txt
drivethrurpg-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.drivethrurpg-pp-cli/feedback.jsonl`. They are never POSTed unless `DRIVETHRURPG_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DRIVETHRURPG_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
drivethrurpg-pp-cli profile save briefing --json
drivethrurpg-pp-cli --profile briefing categories list
drivethrurpg-pp-cli profile list --json
drivethrurpg-pp-cli profile show briefing
drivethrurpg-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `drivethrurpg-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add drivethrurpg-pp-mcp -- drivethrurpg-pp-mcp
```

Stdio is the default. For hosts that support remote streamable HTTP, run:

```bash
drivethrurpg-pp-mcp --transport http --listen 127.0.0.1:8080
```

The server also exposes goal-oriented MCP tools. Prefer these before raw endpoint mirrors when they match the user's request:

- `job_catalog_search`
- `job_catalog_price_scan`
- `job_product_brief`
- `job_library_list`
- `job_library_find`
- `job_file_manifest`
- `job_prepare_file_download`
- `job_cache_status`

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which drivethrurpg-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   drivethrurpg-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `drivethrurpg-pp-cli <command> --help`.

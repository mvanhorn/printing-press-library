# Toretabi CLI

**Inspect Japan rail-ticket conditions and linked official operator rules with explicit uncertainty.**

Discover a bounded regional ticket shortlist, read the linked official operator document, and compare source periods and conditions. Japanese evidence, independent source clocks, conflicts and unknowns stay visible.

## Install

The recommended path installs both the `toretabi-pp-cli` binary and the `pp-toretabi` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install toretabi
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install toretabi --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install toretabi --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install toretabi --agent claude-code
npx -y @mvanhorn/printing-press-library install toretabi --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/toretabi/cmd/toretabi-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/toretabi-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine ./toretabi-pp-cli`. On Unix, mark it executable: `chmod +x ./toretabi-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install toretabi --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-toretabi --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-toretabi --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install toretabi --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/toretabi-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/toretabi/cmd/toretabi-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "toretabi": {
      "command": "toretabi-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Ordinary anonymous public Toretabi and linked official operator documents; no credentials or running browser.

## Evidence and coverage

Toretabi's public `/ticket/` directory supplies editorial discovery and detail evidence. Live `tickets get` and `tickets compare` also inspect exactly one official operator URL linked by that detail. The operator read stays inside these command paths. `ticket.observed_at` and `ticket.operator.observed_at` are separate retrieval clocks; neither is a publisher revision time.

The operator object preserves its source/effective URL, HTTP status, Japanese identity evidence, periods, ticket-fare category/unit evidence, purchase channels, eligibility, supplements, exclusions and referenced PDF URLs. It accepts bounded HTML from the six supported JR document hosts listed below, binds evidence to a unique product container and its own identity, and compares explicit date endpoints and validity days against publisher facts. `rule_checks` distinguishes `corroborated`, `conflict`, `operator_only` and `unknown`; fare categories from two sources require an explicit category comparison. Publisher facts are never overwritten by operator facts. `all_rules_verified` remains false: calendar exceptions, traveler eligibility, complete train coverage, referenced documents and inventory remain unresolved.

Supported linked hosts are `railway.jr-central.co.jp`, `www.jrhokkaido.co.jp`, `www.jreast.co.jp`, `tickets.jr-odekake.net`, `www.jrkyushu-kippu.jp` and `www.jr-eki.com`. Other hosts return `unsupported_operator` without a request. Missing links, access errors, non-200 responses, oversized documents, unsupported formats, PDFs, mismatched identity and insufficient labelled rules have distinct statuses. A fetched HTML page or a title match alone never confirms rules. PDFs are identified but their text is not extracted; linked PDFs remain under `pdf_references`. Access failure does not mean a ticket is unavailable, sold out or closed.

`tickets list` scans native area/type pages, then applies a literal Japanese name/tag query. `--max-pages` defaults to 2 (maximum 5), independently of `--limit` default 10 (maximum 50). Coverage reports inspected routes, scanned/returned counts and continuation. `complete` concerns those native-filtered pages; returned rows can still be capped. A zero match has that bounded scope. Use get before comparing conditions.

`tickets get` preserves sale/use/validity evidence and conditions from both sources. Prices stay source-specific: publisher `price.status: unknown` is normal when no fare row is present; official fare tables can supply `operator.price`. Each operator amount retains passenger/product category, JPY currency basis and `per_ticket` unit. No trip total or savings is calculated. A 1,000 JPY voucher remains a benefit. Airline proof can qualify a voucher rather than pass eligibility; U25 evidence concerns age at purchase and required documents/channels.

`tickets compare` inspects 2..4 distinct IDs. `--use-on` is the intended first use date; `--as-of` defaults to Asia/Tokyo today for sales and edition checks. Publisher and operator date checks stay separate. `closed` means after an explicit published deadline, `not_started` before an explicit start, and `within_published_window` only within known bounds. Weekdays, holidays, sale lead times and yearless blackout spans remain conditional. `archived_use_window` means an explicit use deadline precedes as-of; fetching an old edition today does not make it current. Inventory and confirmed traveler eligibility stay unknown. Partial publisher read failures appear under `fetch_failures`; an operator failure stays inside its successfully fetched ticket.

Live details save normalized observations unless `--no-cache` is set. `--data-source live` reads both sources; `local` reads saved evidence and performs no operator refresh. Auto prefers live and can use an explicitly warned saved fallback for other publisher read failures, while publisher 403/404 and rate limits remain errors. Operator 403/404 are evidence-access failures inside the ticket. Original source clocks and their cache ages remain attached; `stale` is recomputed independently from each original clock using root `--max-age` (zero disables); an absent operator clock has `freshness: unknown_clock`. Missing caches return an empty result; `tickets cached` neither refreshes nor creates/migrates a database.

Cache capacity is 100 inspected tickets, 32 KiB per normalized payload, 50 output records and an 8 MiB read-file bound. Normal SQLite transactions preserve committed WAL rows. A delayed older save cannot overwrite a newer publisher observation. The cache is `tickets.sqlite` under the resolved cache directory; framework learning data and generic SQL use a separate data directory and do not query these ticket observations.

Every live domain command passes the root timeout context. The shared polite limiter allows at most 2 requests/second; each document is capped at 512 KiB, with bounded charset decoding for operator HTML. Requests are anonymous GETs; the CLI accepts stable ticket IDs, not arbitrary URLs. Tourist-train timetables/live seats, exhaustive archives, route optimization, eligibility certification, purchases and payments are unsupported.

## Quick Start

```bash
# Discover native-filtered regional ticket candidates.
toretabi-pp-cli tickets list --area 1 --ticket-type 4 --max-pages 1 --limit 3 --agent

# Inspect publisher facts and bounded linked official operator rules.
toretabi-pp-cli tickets get hokkaido_028 --agent

# Compare published date evidence without assuming seat stock.
toretabi-pp-cli tickets compare tokai_043 east_027 --use-on 2026-10-10 --as-of 2026-10-04 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Japan rail-ticket evidence
- **`tickets list`** — Report native-filtered routes, scanned records and continuation so zero matches have a scope.

  _Report native-filtered routes, scanned records and continuation so zero matches have a scope._

  ```bash
  toretabi-pp-cli tickets list --area 1 --ticket-type 4 --max-pages 1 --limit 3 --agent
  ```
- **`tickets compare`** — Compare publisher and linked operator periods, corroboration and conflicts while preserving calendar uncertainty.

  _Compare publisher and linked operator periods, corroboration and conflicts while preserving calendar uncertainty._

  ```bash
  toretabi-pp-cli tickets compare tokai_043 east_027 --use-on 2026-10-10 --as-of 2026-10-04 --agent
  ```
- **`tickets compare`** — Contrast publisher and linked operator supplements and excluded trains with source clocks.

  _Contrast publisher and linked operator supplements and excluded trains with source clocks._

  ```bash
  toretabi-pp-cli tickets compare tokai_043 east_027 --agent
  ```
- **`tickets get`** — Inspect linked operator fares, eligibility and purchase channels while keeping missing facts explicit.

  _Inspect linked operator fares, eligibility and purchase channels while keeping missing facts explicit._

  ```bash
  toretabi-pp-cli tickets get hokkaido_028 --agent
  ```
- **`tickets cached`** — Read saved normalized evidence without refreshing the source.

  _Read saved normalized evidence without refreshing the source._

  ```bash
  toretabi-pp-cli tickets cached --limit 3 --agent
  ```

## Command reference

| Command | Inputs and result |
|---|---|
| `tickets list` | Native area/type, literal query, bounded listing and coverage |
| `tickets get` | One ID, publisher detail and linked operator rule evidence |
| `tickets compare` | 2..4 IDs, use/as-of dates, separate source checks and conflicts |
| `tickets cached` | Saved details, original source clocks and cache ages |

```bash
toretabi-pp-cli tickets list --area 1 --ticket-type 4 --max-pages 1 --limit 3 --agent
toretabi-pp-cli tickets get hokkaido_028 --data-source live --agent
toretabi-pp-cli tickets compare tokai_043 hokkaido_028 --use-on 2026-10-10 --as-of 2026-10-04 --agent
toretabi-pp-cli tickets cached --limit 3 --agent
toretabi-pp-cli which "ticket periods" --json
```

## Output and MCP

`--agent` enables JSON, compact output, no input and no color. Agent envelopes contain `{meta, results}`: inspect `results.ticket`, `results.tickets` or `results.comparisons`. Plain `--json` exposes the same decision fields without that wrapper. Diagnostics go to stderr. `--select` supports dotted projections; `--csv`, `--plain` and `--quiet` are formatting choices, not evidence checks. `--dry-run` previews a command without source reads.

```bash
toretabi-pp-cli tickets get tokai_043 --agent --select ticket.name_ja,ticket.operator,ticket.observed_at
```

MCP exposes the same four domain commands through the sibling CLI over stdio. For `tickets_compare`, pass `ids` as the comma-separated string `tokai_043,hokkaido_028`. Listing inputs include `area`, `ticket-type`, `query`, `max-pages` and `limit`; native numbered pages are bounded internally. There is no cursor/after parameter, ticket sync or ticket full-text search. Narrow IDs or projections if a response exceeds the MCP capture bound.

## Local paths

`--home /tmp/toretabi-session` or `TORETABI_HOME` relocates config, data, state and cache directories. Per-kind overrides are `TORETABI_CONFIG_DIR`, `TORETABI_DATA_DIR`, `TORETABI_STATE_DIR` and `TORETABI_CACHE_DIR`; precedence is per-kind override, command home, TORETABI_HOME, XDG, platform default. Config stores framework profiles, data stores local learning and cache stores ticket observations. No source secrets, login or cookies are needed.

```bash
toretabi-pp-cli doctor --json
toretabi-pp-cli agent-context --pretty
```

For MCP relocation, set `TORETABI_HOME` in the host environment; CLI flags are not inherited by the server.

## Automatic learning

The optional framework recall/teach loop writes only local learning data. Before discovery, check `recall` for a reusable ticket mapping; fetch returned IDs live before treating saved guidance as current rules. On an empty store, proceed with ordinary discovery. Candidate actions need a successful trial before confirmation; no candidate can certify a fare or traveler condition. Teach a structural question after a successful answer, using concrete resource IDs and `ticket` as the resource type.

```bash
toretabi-pp-cli recall "Hokkaido free ticket conditions" --agent
toretabi-pp-cli teach --query "Hokkaido free ticket conditions" --resource-type ticket --resource hokkaido_028
toretabi-pp-cli learnings list --agent
toretabi-pp-cli learnings candidates --agent
toretabi-pp-cli learnings stats --agent
```

Pass arbitrary user text as an argv/MCP value, never interpolate it into shell source. Confirm/reject only concrete candidate IDs returned by this installed binary. Use `--no-learn` or `TORETABI_NO_LEARN=true` for deterministic runs. The local schema stamp is one-way: an older binary can refuse a store opened by this one.

## Decision limits

An answer is complete when it returns a bounded shortlist or inspected evidence with both source clocks, explicit verification results and unresolved fields. Recheck the linked operator's full document before purchase or boarding where rules remain unknown, contradictory, inaccessible or in PDFs. The CLI never books or sends messages.

## Development

Run `go test ./...`, `go vet ./...`, and build both command packages. Keep recorded local customizations in `.printing-press-patches/`. Runtime version `0.0.0-dev` remains unstamped until the public library release workflow assigns it after merge.

## Recipes

### Bounded coverage audit

```bash
toretabi-pp-cli tickets list --area 1 --ticket-type 4 --max-pages 1 --limit 3 --agent
```

Report native-filtered routes, scanned records and continuation so zero matches have a scope.

### Date evidence comparison

```bash
toretabi-pp-cli tickets compare tokai_043 east_027 --use-on 2026-10-10 --as-of 2026-10-04 --agent
```

Compare publisher and linked operator periods, corroboration and conflicts while preserving calendar uncertainty.

### Supplement and exception contrast

```bash
toretabi-pp-cli tickets compare tokai_043 east_027 --agent
```

Contrast publisher and linked operator supplements and excluded trains with source clocks.

### Purchase prerequisites

```bash
toretabi-pp-cli tickets get hokkaido_028 --agent
```

Inspect linked operator fares, eligibility and purchase channels while keeping missing facts explicit.

### Timestamped offline shortlist

```bash
toretabi-pp-cli tickets cached --limit 3 --agent
```

Read saved normalized evidence without refreshing the source.

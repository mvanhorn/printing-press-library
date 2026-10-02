# JR East Status CLI

**Compare JR East line impacts with bilingual identity, source freshness and reporting limits.**

Discover lines and inspect suspensions, affected sections and planned notices. Compare itinerary lines and follow currently published certificate links with explicit unknowns.

Learn more at [JR East Status](https://traininfo.jreast.co.jp).

Contributors: [@zjsng](https://github.com/zjsng) (zjsng).
## Install

This build is locally verified. Public-library installer/release commands below apply after this CLI is published. To use the current source checkout:

```bash
go build -o ./jr-east-status-pp-cli ./cmd/jr-east-status-pp-cli
./jr-east-status-pp-cli areas --agent
```


The recommended path installs both the `jr-east-status-pp-cli` binary and the `pp-jr-east-status` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install jr-east-status
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install jr-east-status --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install jr-east-status --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install jr-east-status --agent claude-code
npx -y @mvanhorn/printing-press-library install jr-east-status --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/cmd/jr-east-status-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

After publication, download a pre-built binary for your platform from the [release](https://github.com/mvanhorn/printing-press-library/releases/tag/jr-east-status-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install jr-east-status --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-jr-east-status --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-jr-east-status --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install jr-east-status --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. After publication, download the `.mcpb` for your platform from the [release](https://github.com/mvanhorn/printing-press-library/releases/tag/jr-east-status-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/cmd/jr-east-status-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "jr-east-status": {
      "command": "jr-east-status-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Anonymous first-party source reads; no key or login required.

## Quick Start

```bash
# Check the local install.
jr-east-status-pp-cli doctor --dry-run

# Discover region/service summaries.
jr-east-status-pp-cli areas --agent

# Resolve native IDs.
jr-east-status-pp-cli lines --region kanto --query Yamanote --agent

# Inspect source-backed line facts.
jr-east-status-pp-cli status --line sobuline --region kanto --agent

```

## Unique Features

These commands add source-aware decisions to the underlying public-page reads.

### Source-aware decisions
- **`status`** — Reconcile native Japanese IDs with English row labels, keeping multiple notices and source uncertainty.

  _Reconcile native Japanese IDs with English row labels, keeping multiple notices and source uncertainty._

  ```bash
  jr-east-status-pp-cli status --line sobuline --region kanto --agent
  ```
- **`impact`** — Compare selected itinerary lines with per-region source timing and affected sections.

  _Compare selected itinerary lines with per-region source timing and affected sections._

  ```bash
  jr-east-status-pp-cli impact --lines kanto:yamanoteline,kanto:sobuline --agent
  ```
- **`coverage`** — Explain reporting hours, threshold conflicts, midnight service-day rollover and remaining unknowns.

  _Explain reporting hours, threshold conflicts, midnight service-day rollover and remaining unknowns._

  ```bash
  jr-east-status-pp-cli coverage --agent
  ```

### Planning handoffs
- **`planned`** — Inspect construction evidence without promoting future notices to current service suspension.

  _Inspect construction evidence without promoting future notices to current service suspension._

  ```bash
  jr-east-status-pp-cli planned --line koumiline --region kanto --agent
  ```
- **`certificates`** — Return actual published route/time-slot links and source coverage limits.

  _Return actual published route/time-slot links and source coverage limits._

  ```bash
  jr-east-status-pp-cli certificates --line yamanoteline --agent
  ```

## Commands

| Command | Purpose | Request cap |
|---|---|---:|
| `areas` | Current area/service aggregate labels and source URLs | 1 |
| `lines --region kanto --query Narita` | Native ID/name/code discovery in one region | 2 |
| `status --line kanto:sobuline` | Selected line facts; express groups include service details | 4 |
| `impact --lines kanto:yamanoteline,kanto:sobuline` | Up to eight line components, grouping region reads | 18 |
| `planned --line koumiline --region kanto` | Selected construction heading/table and line notice facts | 3 |
| `certificates --line yamanoteline --slot 02` | Actual current published handoff for a source time slot | 1 |
| `coverage --at 2026-10-03T01:30:00+09:00` | Embedded reporting-policy classifier | 0 |
| `sources` | Official guide links; use domain commands for operational facts | 1 |

`lines --limit` defaults to 20 and accepts 1–100. `status --limit` limits notice facts (default 10, maximum 20). `planned --limit` defaults to 4, maximum 6. `certificates --limit` defaults to 20, maximum 30; without a line it lists reference identities/counts, while selecting a line returns time-slot links. Result truncation is explicit in `meta.truncated`. Each regional page scans at most 512 rows; the construction parser scans at most 100 sections and retains at most 12 table rows per matching section.

Domain source bodies are capped at 1 MiB, requests at 10 seconds each and total command work at 30 seconds. The root `--timeout` can shorten that deadline. Source requests are paced at at most two per second per process; 429 returns a typed error without automatic retries. `--max-source-age 15m` on line/status/impact commands controls the accepted page age and accepts 1s–24h. Operational commands support `--data-source auto` or `live`; `local` is rejected. They read through to the source on each invocation.

The MCP server mirrors the seven bounded domain commands and the CLI `sources` guide, which extracts links. The raw HTML typed endpoint is hidden from MCP.

## Reading results

Output is bounded facts and source links, rather than full incident articles. `meta.observed_at` is the CLI observation; each `meta.sources[].source_updated_at` is a page timestamp. The page timestamp is not necessarily an incident update or a live train position. Missing, future or stale timestamps remain explicit. If the original Japanese page explicitly closes reporting and contains no line rows, the CLI returns catalogue identities with unknown operational state and does not request its English translation.

The [English status source](https://traininfo.jreast.co.jp/train_info/e/) reports anticipated/actual delays **in excess of 30 minutes**, from **04:00 until 02:00 the next day JST**. The [Japanese source](https://traininfo.jreast.co.jp/train_info/) says **30 minutes or more**. `coverage` retains that language difference. From 02:00–04:00 JST, reporting is closed; the reporting service day rolls at 04:00. BRT coverage is long suspensions only.

`normal_label_only` is a general source label. It does not prove zero delay or punctuality for an individual train. `actual_delay_minutes` stays null. `source_groups` such as “Bound for Sobu” are source navigation categories, while a notice's `direction` is extracted from its text. Unknown directions/sections stay unknown. English uses AI translation; status-label disagreement is flagged, and notice facts retain their source language. Japanese planned dates/times are used because the English Koumi construction sentence was visibly scrambled during discovery.

Express lines are service groups. `source_service_name` identifies an individual named service within the group, so a cancelled Sazanami service does not imply that the Wakashio service is cancelled. A resumption-month estimate remains an estimate and does not establish a date.

`impact` checks the supplied lines only. It does not evaluate a connection, timetable or route alternative. `meta.fetch_failures` and explicit `source_error`/`line_not_found` components preserve partial gaps; all failed components exit nonzero after emitting the uncertainty envelope.

## Planned work and certificates

[Planned-work notices](https://www.jreast.co.jp/suspend/) can change. The CLI preserves the source's date expressions/table facts. A missing calendar year is null; a current page timestamp does not silently supply it. No matching heading proves only that this bounded source search found none. Read the handoff for complex schedules and the latest source changes.

The [certificate page](https://traininfo.jreast.co.jp/delay_certificate/e/) covers ordinary conventional trains with approximately five-minute-plus delays. Certificates describe a rounded **maximum for a covered route/time slot**, not an individual train, and do not prove boarding. Publication follows confirmation near 07:00, 10:00, 16:00, 21:00 and 02:00 following day JST. A dash or absent URL is `not_published_or_below_threshold`, with no zero-delay claim. Displayed “61 minutes or more” remains a lower bound.

Only links actually present on the current source page are returned; the URL's own date controls midnight rollover. Historical dates use the official website handoff. [Section coverage and through-service rules](https://traininfo.jreast.co.jp/delay_certificate/e/rosen.html) govern the travelled segment. Shōnan-Shinjuku, Ueno-Tōkyō and Sotetsu through-service names require choosing the actual segment's certificate; Takasaki Tokyo–Omiya uses Utsunomiya. Sagami and Ōme beyond Ōme use the official DOKOTORE handoff. The CLI does not print certificates or check DOKOTORE inventory.

## Agent Usage

```bash
jr-east-status-pp-cli lines --region kanto --query Narita --agent --select id,name_ja,name_en
jr-east-status-pp-cli impact --lines kanto:yamanoteline,kanto:sobuline --agent
jr-east-status-pp-cli status --line sobuline --dry-run --agent
```

`--json`, `--agent`, `--select`, `--compact`, `--csv` and `--quiet` use the generated output layer. JSON is an envelope with `meta` and `results`; empty results are `[]`. Always inspect source timing, reporting state, truncation and fetch failures before planning. The learning store holds reusable identity/workflow hints; it is not a current operational status cache. Use `--no-learn` for deterministic runs.

## Health Check

```bash
jr-east-status-pp-cli doctor --json
jr-east-status-pp-cli doctor --dry-run
```

`doctor` checks the generated transport/install. A useful line read is a stronger parser check than a successful HTTP status alone: the auxiliary `traininfomulti/kanto.json` feed observed in Chrome contained only a dummy notice.

## Troubleshooting

| Symptom | Action |
|---|---|
| Source HTTP error, throttle or changed HTML | Retry later or open the emitted first-party URL; no normal-service conclusion is returned. |
| Source is stale, future dated or lacks a timestamp | Keep the explicit unknown state; use the official source handoff. |
| No matching line | Choose its native ID with `lines` and the correct region. |
| Construction result is empty | Read the official planned-work page; the selected heading search is bounded. |
| Certificate is absent | Check covered segment and publication timing on the official page; absence does not establish zero delay. |
| Curl gets 403 while the CLI works | The source accepted normal Go HTTP with the declared CLI headers during live verification. Browser discovery is not the runtime. |

Exit codes: 0 for completed reads/computed results (partial gaps can remain explicit); 2 for invalid input; 3 for a missing selected line; 5 for a source/parse failure; 7 for source throttling. Generated framework commands expose their own documented codes.

### API-specific
- **Source unavailable or stale** — Retry once later or open the reported source URL; do not infer normal operation.
- **Certificate cell is a dash** — Use the source handoff; a missing published link does not establish zero delay.

## Cookbook

```bash
# Compare a local line and a Shinkansen component.
jr-east-status-pp-cli impact --lines kanto:yamanoteline,shinkansen:tohokushinkansen --agent

# Planned Koumi closures, keeping source date expressions.
jr-east-status-pp-cli planned --line koumiline --region kanto --agent

# Only the second published certificate slot, without opening a browser.
jr-east-status-pp-cli certificates --line yamanoteline --slot 02 --agent

# Verify the overnight reporting service day.
jr-east-status-pp-cli coverage --at 2026-10-03T01:30:00+09:00 --agent
```

## Development

```bash
go test -count=1 ./...
go vet ./...
go build ./...
```

Fixture tests cover native IDs across reordered multilingual rows, multiple notices, reporting boundaries, stale/error pages, planned dates/sections, actual certificate URLs, typed throttle and request/body caps. Read-only live evidence and review receipts are maintained in the isolated Printing Press run and archived manuscripts; `FINAL-REPORT.md` records their exact paths. No notifications, subscriptions, bookings, refunds or account mutations are implemented.

## Recipes

### Itinerary impact

```bash
jr-east-status-pp-cli impact --lines kanto:yamanoteline,kanto:sobuline --agent
```

Compare selected lines with coverage and timestamps.

### Compact line discovery

```bash
jr-east-status-pp-cli lines --region kanto --query Narita --agent --select id,name_ja,name_en
```

Keep only line identifiers and bilingual names.

### Published certificate

```bash
jr-east-status-pp-cli certificates --line yamanoteline --agent
```

Follow published links; delay is a route maximum.

### Planned closure

```bash
jr-east-status-pp-cli planned --line koumiline --region kanto --agent
```

Inspect scheduled-work evidence and official handoff.

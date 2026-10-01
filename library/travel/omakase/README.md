# OMAKASE CLI

**Discover premium Japanese restaurants with source courses, release rules and honest availability states.**

Read-only public OMAKASE planning. Compare courses and cancellation terms, then use canonical booking links.

Created by [@zjsng-trav](https://github.com/zjsng-trav) (zjsng).

## Install

The recommended path installs both the `omakase-pp-cli` binary and the `pp-omakase` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install omakase
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install omakase --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install omakase --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install omakase --agent claude-code
npx -y @mvanhorn/printing-press-library install omakase --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/omakase/cmd/omakase-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/omakase-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install omakase --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-omakase --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-omakase --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install omakase --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Authentication

Public discovery and detail need no account. Exact seats require login; Premium search and calendars remain gated. No paid account required by this CLI.

## Quick Start

```bash
# Check local setup
omakase-pp-cli doctor --dry-run

# Relevant public names
omakase-pp-cli restaurants find --query Sugita --agent

# Inspect source detail
omakase-pp-cli restaurants show hc778124 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Restaurant planning
- **`courses`** — Inspect public course price floors, units, service charges, reservation fee and cancellation caveats.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli courses hc778124 --agent
  ```
- **`release`** — Read source release timing in JST; distinguish scheduled, irregular and undetermined releases from seats.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli release jv742052 --agent
  ```
- **`availability`** — Inspect a planning date and party with unknown exact seats and query_evaluated false when public access is gated.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli availability hc778124 --date 2026-11-01 --party 2 --agent
  ```
- **`compare`** — Compare public terms for two to five restaurant IDs with explicit per-ID failures.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli compare hc778124 qt951856 --agent
  ```
- **`inventory`** — Explicitly refresh bounded summary pages and inspect or search local inventory coverage.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli inventory --agent
  ```

## Agent Usage

Planning commands return compact JSON by default, including explicit `null` values. `--agent` remains compatible. Narrow output with field projection:

```sh
./omakase-pp-cli compare hc778124 qt951856 --agent --select results.id,results.name,results.courses
./omakase-pp-cli restaurants find --query Sugita --limit 5 --select results.id,results.name,results.url
```

The `meta` envelope reports request count, cache hits, bytes, latency, partial coverage and source evidence. `fetched_at` is retrieval time, not a provider update timestamp. Projection intentionally removes fields not selected, including metadata.

## Cookbook

```sh
./omakase-pp-cli filters
./omakase-pp-cli restaurants find --area kansai --cuisine sushi --limit 10
./omakase-pp-cli restaurants find --page 2 --offset 0
./omakase-pp-cli courses hc778124 --lang ja
./omakase-pp-cli inventory refresh --pages 2
./omakase-pp-cli inventory find --query Konno
./omakase-pp-cli inventory status
./omakase-pp-cli membership --refresh
```

Discovery reads one source page of up to 32 cards. Default output is 10 matches, maximum 50. `next_offset` continues matches on the same page; after that use `next_page`. The provider search is fuzzy; `--query` applies a literal, case-insensitive name substring filter afterward. `source_total` counts the provider's broad results, not exact name matches. Unseen pages can contain more matches.

Inventory refresh defaults to one page, caps at 25 pages and replaces the snapshot only after successful retrieval. It stores summaries without fetching every restaurant detail. There is no automatic inventory refresh. Increase the whole-command `--timeout` explicitly for a larger refresh.

## Reservation states and access

Public release schedules are distinct from seats. `release.state` can be `scheduled`, `undetermined`, `irregular`, `schedule_text` or `unknown`. `next_round_at` is present only when the source supplies a parseable dated release; recurrence text is preserved without inventing a date.

Booking actions can identify `request`, `waitlist` or `lottery`; these do not prove seats. An anonymous booking button also does not prove availability. Exact date/party seats remain `state: unknown`, `seats: null`, `query_evaluated: false`; supplied dates and parties are planning context and are not falsely presented as evaluated source queries. Amamoto explicitly requires login to check availability. Premium advanced seat search, labels and release calendars are paid/member-only; Gold is invitation-only. `membership` reads current public prices and feature text, with account eligibility unknown. No paid account is required or acquired.

A restaurant's total seating capacity under `information` is unrelated to available seats. Course amounts are JPY with a stated basis; minimum/variable values and source text are retained. Service charge and reservation fees remain separate. Cancellation rows and advance-payment caveats are preserved rather than converted into guessed refund calculations.

## Cache and limits

Normalized public documents are cached for 30 minutes by default; raw HTML, cookies and credentials are not stored. `--refresh` or `--data-source live` forces HTTP; `--no-cache` bypasses reads and writes. `--offline` or `--data-source local` reads only the existing document cache, marks stale evidence and fails on a miss. Online failures are explicit; they do not silently become stale successes.

The cache caps at 128 documents and responses at 2 MiB. HTTP concurrency is one, pacing defaults to 500 ms, and throttled/5xx reads receive at most one retry. Per-request timeout is 15 seconds; root `--timeout` bounds the whole command (default 60 seconds). Only observed first-party public paths are fetched. No browser runtime is needed.

`--home /absolute/path` isolates all application state. Planning cache lives under its `cache/omakase/documents`, inventory under `data/omakase/inventory.json`; runtime `agent-context` reports platform paths. Generated local learning/profile tools are optional; `--no-learn` keeps planning flows free of learning writes. `pages` exposes source metadata, while the planning commands above provide normalized domain results.

## Health Check

```sh
./omakase-pp-cli doctor --json
./omakase-pp-cli doctor --dry-run
```

## Troubleshooting

Usage errors exit 2; not found exits 3; access/403 exits 4; source/network/structure failures exit 5; exhausted throttling exits 7; local configuration errors exit 10. Diagnostics use stderr. Empty valid searches exit 0 and return `[]`. Comparisons surface per-ID errors and exit 5 unless `--allow-partial` is explicit. Exact seats being unknown is a successful bounded observation, not a sold-out claim.

Cloudflare can reject some clients or pages. The CLI uses identified HTTP request headers that passed live public reads. A challenge, login page, changed HTML structure or non-HTML response fails explicitly. Use the canonical website to inspect gated flows. No challenge solving or cookie extraction is included.

## Verification

[Evidence report](evidence/FINAL.md) records live source checks, cold/warm metrics, independent review, deterministic tests, Press shipcheck and full live dogfood. [Requirements](evidence/requirements.md) distinguish shipped public scope from inaccessible member scope. Offline test documents are synthetic and are never labeled live evidence.

## Recipes

### Compare courses

```bash
omakase-pp-cli compare hc778124 qt951856 --agent --select results.id,results.name,results.courses
```

Bounded public comparison.

### Inspect release

```bash
omakase-pp-cli release jv742052 --agent
```

Release timing is separate from seats.

### Inspect availability boundary

```bash
omakase-pp-cli availability hc778124 --date 2026-11-01 --party 2 --agent
```

Report unknown when login is required.

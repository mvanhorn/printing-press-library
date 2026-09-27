---
name: pp-rakuten-travel
description: Search Japan accommodation on Rakuten Travel, inspect room plans for explicit dates and party, or compare hotel/date alternatives before booking handoff.
author: "zjsng"
license: "Apache-2.0"
argument-hint: "hotel search, dated room offers, or date comparison"
allowed-tools: "Read Bash"
---

# Rakuten Travel

## Prerequisites: Install the CLI

This skill drives the `rakuten-travel-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install rakuten-travel --cli-only
   ```
2. Verify: `rakuten-travel-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/cmd/rakuten-travel-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search Rakuten Travel public pages, inspect property details and compare bounded date alternatives. Keep room and plan identities, source price labels and booking links intact.

## Build from source

From this repository directory:

```sh
go build -o bin/rakuten-travel-pp-cli ./cmd/rakuten-travel-pp-cli
export PATH="$PWD/bin:$PATH"
```

The public website backend needs no API credentials or browser.

## When to Use This CLI

Use for accommodation discovery in Japan, dated room-plan inspection, or bounded hotel/date comparisons on Rakuten Travel. The user completes booking on Rakuten.

## Anti-triggers

Use another workflow for cross-provider comparisons, reservation/payment/cancellation execution, account management, automatic destination translation, radius search, or unequal party allocations across rooms.

## Workflow

1. Establish destination or hotel, check-in/out dates, room count, adults per room and the six child categories. Resolve missing details before inventory search. Counts are per room; rooms must share the same party. Confirm category choices rather than infer ages.
2. Resolve source IDs with `areas list` or `hotels search`. Keep ambiguous area choices visible. Japanese labels and English literal keywords may return different coverage. Continue when candidates have hotel IDs and source URLs.
3. Inspect shortlisted properties with `hotels show`, including amenities, access and fee/policy notes. Continue when material requirements are verified or explicitly unknown.
4. Run `offers search` with explicit dates and party. Preserve hotel/plan/room IDs and source quote units. Follow returned within-page offset before the next source page when more results are needed; keep the request budget bounded.
5. Inspect a chosen tuple with `offers show`. Use `compare` only for an explicit matrix of hotel IDs and check-in dates with equal stay length and party, at most nine cells. Keep failed and empty cells distinct.
6. Present source-labelled prices, dates, per-room party, meals, material unknown fees/policies, freshness and the returned Rakuten link. Complete when each recommended option has enough evidence for the traveler to confirm and book on Rakuten.

For price units, cache states, policy scope or ambiguous empty results, read [the data contract](docs/data-contract.md) before interpreting output.

## Unique Capabilities

- **Date alternatives** — `compare` preserves every bounded matrix cell's status.
- **Whole-stay quote evidence** — `offers search` retains source room-stay totals.
- **Child-aware occupancy** — `offers search` keeps six child categories separate.
- **Policy and fee inspection** — `offers show` separates property notes from unknown plan rules.
- **Exact offer handoff** — `offers show` preserves the room/plan tuple and dated source link.


## Recipes

### Compact candidates

```bash
rakuten-travel-pp-cli hotels search --query 品川 --limit 3 --agent --select results.hotel_id,results.name
```

Resolve IDs with a bounded payload.

### Property notes

```bash
rakuten-travel-pp-cli hotels show --hotel 51870
```

Inspect amenities, access and property fee notes.

### Date alternatives

```bash
rakuten-travel-pp-cli compare --hotels 51870 --checkins 2026-11-08,2026-11-09 --nights 2 --rooms 1 --adults-per-room 2
```

Compare explicit equal-length stays with the same party.

## Agent Mode

Default stdout is compact JSON. `--agent` retains the focused result envelope; `--select` narrows fields. Read `.results` for data and `.meta` for query, source/freshness, coverage and request statistics. Diagnostics are on stderr.

Use `--help` for current flags and limits. Use `--dry-run` to inspect local command wiring without source requests. Live inventory is uncached by default; explicit inventory caching is short and timestamped. A page-contract error or throttle is not evidence of sold-out inventory.

## Auth Setup

The public website backend requires no API credentials or browser. The separately documented official APIs require an application ID and access key and are not used by these commands.

Run `rakuten-travel-pp-cli doctor` to verify setup.

## Direct Use

```sh
rakuten-travel-pp-cli doctor --json
rakuten-travel-pp-cli offers search --help
```

`doctor` checks setup. The live dated offer command checks inventory. Verification evidence is in [docs/verification.md](docs/verification.md).

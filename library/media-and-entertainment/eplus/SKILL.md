---
name: pp-eplus
description: "Search Japan concerts, theatre, sports and cultural events on eplus; inspect sessions, lottery deadlines, international ticket terms, or compare booking links."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - eplus-pp-cli
    install:
      - kind: go
        bins: [eplus-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/cmd/eplus-pp-cli
---

# eplus discovery

## Prerequisites

This local CLI requires Go 1.26.6 or newer. Build from its source root:

```bash
go build -o build/stage/bin/eplus-pp-cli ./cmd/eplus-pp-cli
./build/stage/bin/eplus-pp-cli --version
```

Use the built path or its directory on PATH. No credentials are needed for public discovery. Catalog installation requires the publication PR to merge first.

## Catalog installation after publication

The generator-owned block below applies after publication. Until then, use the local build above.

## Prerequisites: Install the CLI

This skill drives the `eplus-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install eplus --cli-only
   ```
2. Verify: `eplus-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/cmd/eplus-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Discover Japanese performances and plan source-backed ticket sale windows.

## Workflow

1. Use `eplus-pp-cli which "<capability>" --json` and the command's help when the capability is unfamiliar.
2. Search domestic performances with bounded filters. Start with a broad source region and Japanese venue/prefecture names.
3. Inspect a returned canonical detail URL for exact sessions and each sale round.
4. Search international offerings separately if the traveler needs overseas access. Inspect the specific product URL and its conditions.
5. Compare two to four detail IDs and return source-backed booking links to the user.

```bash
eplus-pp-cli events search --artist Radiohead --limit 3 --agent
eplus-pp-cli events search --from 2026-11-01 --to 2026-11-07 --category theatre --region kanto --agent
eplus-pp-cli events detail 4592490001-P0030001P021003 --agent
eplus-pp-cli international search --category concert --limit 3 --agent
eplus-pp-cli international detail 7078 --agent
eplus-pp-cli compare 7078 7079 --source international --agent
eplus-pp-cli policies --agent
```

## Interpretation

`compare` requires `--limit` to cover at least one session per input ID. Bounded results select sessions round-robin across successful inputs and retain partial coverage metadata.

Read `results` and `meta`. `meta.partial`, scan counts, observations and fetch failures describe coverage and freshness. Narrow fields using `--select id,name,date,sales`; increase `--pages` for domestic scans or `--max-scan` for international tour scans only when needed. Detail is lazy. Use `--fresh` for deadline/inventory checks and state when the data was fetched.

`date` is the source service date, even when 25:30 normalizes to the next calendar day. Times are JST. Keep event, performance and sale-round identities distinct, including source codes and URLs. Search round codes and detail booking selectors occupy different namespaces.

Lottery `accepting` means applications are open, with inventory `unknown`; first-come and general sale/presale are separate concepts. Preserve the deadline and eligibility. A domestic listing never proves overseas bookability. International `conditional` requires checking product country/residency/ID/collection terms; generic FAQ guidance cannot override them. Empty terms arrays with unknown/partial status mean unobserved terms, not unrestricted access. Static JPY 0 may be a selection placeholder.

## Boundaries and errors

Discovery and booking-link handoff only. Keep checkout, lottery entry, purchases, reservations and account changes in the user's browser. Public discovery has no offline mirror; `--data-source local` fails clearly. Unavailable source contracts fail instead of returning invented listings. Empty bounded scans are real empty results and may require widening the scan.

Usage failures exit 2, exhausted rate limits exit 7, source/network/parser failures exit nonzero. Diagnostics are stderr; JSON remains on stdout. Check `eplus-pp-cli doctor --json` for connectivity and `eplus-pp-cli agent-context --json` for the current tree. Sandbox DNS failures describe the environment, not provider access policy.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`events detail`** — Inspect separate performance and sale-round identities, exact JST windows and lottery deadlines while keeping unknown inventory and overseas access explicit.

  _An accepting lottery is an application window, not an available seat; source-specific identities and conditions must survive compact output._

  ```bash
  eplus-pp-cli events detail 4592490001-P0030001P021003 --agent
  ```

## Recipes

### Inspect sale rounds

```bash
eplus-pp-cli events detail 4592490001-P0030001P021003 --agent
```

Keep each performance and lottery deadline separate; available seats remain unknown for lotteries.

### Check international terms

```bash
eplus-pp-cli international detail 7078 --agent
```

Inspect the selected product price and event-specific overseas access conditions.

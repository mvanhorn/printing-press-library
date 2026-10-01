---
name: pp-yamap
description: "Discover Japan hikes on YAMAP, inspect planned model courses and recorded trip metrics, read dated contributor reports, or check source map coverage. Use for YAMAP hiking discovery, recent reports, use yamap, or run yamap."
author: zjsng
license: Apache-2.0
allowed-tools: Read Bash
---

# YAMAP public hiking discovery

## Prerequisites: Install the CLI

This skill drives the `yamap-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install yamap --cli-only
   ```
2. Verify: `yamap-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/yamap/cmd/yamap-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search named mountains, planned model courses, recorded trips and map areas. Keep source metrics and contributor observations separate from closure and safety decisions.

## When to Use This CLI

Use Japanese names, preserve source IDs and prefectures, then fetch one detail lazily. Public anonymous first-party reads need no paid account. Installation from the public catalog becomes available after maintainer merge.

## Anti-triggers

Do not use for safety certification, authoritative closure clearance, GPX/offline navigation, account data, purchases, bookings or route creation.

## Recipes

### Compact mountain candidates

```bash
yamap-pp-cli mountains search 高尾山 --agent --select id,name,prefectures,url
```

Keep source IDs and Japanese names.

### Recent contributor reports

```bash
yamap-pp-cli reports recent 高尾山 --limit 3 --agent
```

Activity-date filter within bounded candidate coverage.

### Inspect map coverage

```bash
yamap-pp-cli maps coverage 77 --agent
```

Area bounds do not establish route safety or offline availability.

## Unique Capabilities

Focused helpers preserve the evidence boundaries described above.

### Hiking evidence
- **`reports recent`** — Find recorded activity evidence by trip date in a bounded scan


  ```bash
  yamap-pp-cli reports recent 高尾山 --limit 3 --agent
  ```
- **`reports observations`** — Read bounded contributor observations with trip dates and source links


  ```bash
  yamap-pp-cli reports observations 51497803 --agent
  ```
- **`routes compare`** — Compare planned metrics with recorded metrics without assuming track equivalence


  ```bash
  yamap-pp-cli routes compare 1771 51497803 --agent
  ```
- **`maps coverage`** — Inspect map area bounds and limits of map coverage


  ```bash
  yamap-pp-cli maps coverage 77 --agent
  ```
- **`inventory status`** — Inspect cached request inventory and freshness without a crawl


  ```bash
  yamap-pp-cli inventory status --agent
  ```

## Auth Setup

Anonymous public first-party JSON reads; no account or paid key. Membership-only GPX and multi-landmark operations excluded.

Run `yamap-pp-cli doctor` to verify setup.

## Agent Mode

Focused commands emit compact `{meta,results}` JSON by default. Add `--select` for result fields; metadata always retains coverage/provenance. Distances/elevation use meters; durations seconds. UTC timestamps and Japan trip dates are explicit. Unavailable values are null.

Model courses are planned references. Report summaries use legacy metrics and may omit planned status; detail prefers regularized elapsed/active/rest totals. Exact moving time is unavailable. Contributor text and publisher cautions retain separate evidence classes. Neither user reports nor false publisher closure flags establish safe/open status. Linked authoritative notices are not independently fetched. Map bounds do not prove offline/track/trail coverage.

## Bounded reads and freshness

One page/five results default, limit at most twenty. Recent scans default twenty candidates, at most five pages. Sort/recency guarantees apply only within the scan; older seasons remain dated. Empty results cannot establish no reports or safe/open trails. Exact-request cache TTL is fifteen minutes. Use `--refresh` explicitly, `--no-cache` for no persistence, or `--data-source local` for exact cached reads with stale metadata. Isolate paths with `--home` when needed. `--diagnostics` writes costs to stderr.

## Troubleshooting

Read stderr and exit status: 2 usage, 3 absent source/cache, 4 restricted, 5 network/challenge/schema, 7 throttled, 10 config/path. Failures emit no result JSON. Inspect source links before interpreting observations. Run `yamap-pp-cli doctor --agent` for connectivity and `yamap-pp-cli inventory status --agent` for cache coverage.

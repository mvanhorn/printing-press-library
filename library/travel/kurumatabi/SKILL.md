---
name: pp-kurumatabi
description: Discover Japanese RV parks and permitted overnight stops, compare vehicle limits and facilities, screen services or prepare booking contacts through Kurumatabi. Use for RV Park / くるま旅 searches, vehicle-size checks, dump-station evidence, or `use kurumatabi`.
license: Apache-2.0
author: "zjsng"
allowed-tools: Read Bash
metadata:
  openclaw:
    requires:
      bins: [kurumatabi-pp-cli]
---

# Kurumatabi / くるま旅

## Source checkout before catalog release

This contribution awaits manual review. Until it is merged and indexed, build from its source checkout with `go build -o kurumatabi-pp-cli ./cmd/kurumatabi-pp-cli` and place the binary on the runtime PATH. The canonical public installer instructions below apply after the library catalog includes this CLI. Verify with `command -v kurumatabi-pp-cli` and plain `kurumatabi-pp-cli version`.

## Prerequisites: Install the CLI

This skill drives the `kurumatabi-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install kurumatabi --cli-only
   ```
2. Verify: `kurumatabi-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/cmd/kurumatabi-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search the official RV Park and Kurumatabi ecosystem, inspect vehicle limits and facility fees, then compare evidence and prepare a host handoff. Cached observations support proximity and required-service screening without claiming live vacancies or dated stay totals.

## When to Use This CLI

Use for official RV Park / Kurumatabi search, vehicle compatibility evidence, facility and membership comparison, cached waypoint proximity and booking preparation. Read public Japanese records and preserve their evidence.

## Anti-triggers

Choose another tool or a human workflow for reservations/payments, account operations, dated live vacancy, inclusive stay quotes, road routing/clearance, or general outdoor-camping permission. The CLI returns contact links without sending messages or booking.

## Unique Capabilities

These commands derive decisions across published evidence and cached observations.

### Vehicle and service decisions
- **`parks fit`** — Compare your vehicle with one park’s published dimensions and membership rules.

  _Large-motorhome service-stop planner can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks fit rvpark/1086 --length-m 6 --width-m 2.2 --height-m 3 --vehicle van --membership nonmember --agent
  ```
- **`parks match`** — Screen cached parks into proven, uncertain and ruled-out service matches.

  _Pet-owning van planner; Large-motorhome service-stop planner can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks match --require electricity,water,pets --membership nonmember --agent
  ```

### Observed park shortlists
- **`parks compare`** — Compare park evidence without inventing total prices or unknown facilities.

  _Pet-owning van planner; Club and nonmember tariff comparer can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks compare rvpark/1086 yypark/213 --agent
  ```
- **`parks near`** — Rank cached parks by straight-line distance from an explicit waypoint.

  _Pet-owning van planner can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks near --latitude 36.7 --longitude 138.2 --limit 5 --agent
  ```
- **`parks audit`** — Surface disabled-icon conflicts, stale observations and qualified opening notes.

  _Club and nonmember tariff comparer; Large-motorhome service-stop planner can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks audit rvpark/712 --agent
  ```

## Command Reference

Start with `kurumatabi-pp-cli parks --help`; use each leaf's help for flags. `parks filters` returns exact prefecture, type, vehicle, facility and opening-period vocabularies. Source campground slugs are `camp3000` and `campjrva`; RV parks use `rvpark`, 湯YOU parks `yypark`.

A park identity is `kind/numeric-id` or its canonical `https://www.kurumatabi.com/park/kind/id.html` URL. Keep Japanese names and source URLs in the answer.

## Recipes

### Bounded park shortlist

```bash
kurumatabi-pp-cli parks search --prefecture nagano --type rvpark --agent --select results.id,results.name,meta
```

Keep identifiers, Japanese names and source coverage.

### Vehicle evidence

```bash
kurumatabi-pp-cli parks fit rvpark/1086 --length-m 6 --width-m 2.2 --height-m 3 --vehicle van --membership nonmember --agent
```

Unknown constraints require host confirmation.

### Park comparison

```bash
kurumatabi-pp-cli parks compare rvpark/1086 yypark/213 --agent
```

Tariff audiences and fees remain separate.

### Offline proximity

```bash
kurumatabi-pp-cli parks near --latitude 36.7 --longitude 138.2 --limit 5 --agent
```

Ranks only cached observed parks; distance is straight line.

### Required services

```bash
kurumatabi-pp-cli parks match --require electricity,water,pets --membership nonmember --agent
```

Unknown evidence never becomes a proven match.

## Evidence interpretation

- Availability is yes/no/unknown; fee kind is free/paid/included/unknown. Conditional or contradictory fee text stays unknown. `_off` legacy icons override positive alt labels. Generic dump-station presence leaves black/grey permission unknown until explicit evidence exists.
- Preserve metre units, missing dimension bounds and explicit unrestricted height. Category plus dimensions are separate checks. Extra-pitch or overhang permission requires confirmation. A compatible published fit is conditional evidence, not a host acceptance or road-clearance guarantee.
- Per-record membership conditions override general type policy. Discount badges are benefits, not proof that membership is required. Separate audience tariff cases; do not sum an unverified dated total.
- Same-day reservation acceptance is not vacancy. Broad opening labels do not override seasonal closure notes. Vehicle overnight permission does not establish outdoor-camping permission; cite the requested activity's actual rule.
- `observed_at` is JST retrieval time; `source_updated_date_jst` is the provider publication date. Confirm current operation and material conditions directly with the host.

## Auth Setup

Public search and detail require no account. The native search session is memory-only and is used only to preserve filter pagination.

Run `kurumatabi-pp-cli doctor` to verify setup.

## Agent Mode

Use `--agent` for compact JSON and `--select` to preserve just the fields needed. `--json` preserves full structured detail. Arrays stay `[]` when empty. Missing dimension/coordinate values use `null`; facilities, fee kinds and live vacancy have explicit unknown states. Machine stdout contains only the selected output, while hints/errors go to stderr.

Search has separate output/scan caps: `--limit` (1..100) and `--max-scan-pages` (1..5). Read the provider total, scanned denominator, completion/truncation and note from metadata. Cached search/near/match are observed-pool results, not nationwide discovery. Near distances are straight line; match partitions contain only displayed candidate IDs. Compare records exclude failed fetches and include `meta.fetch_failures`.

## Paths and state

Use `kurumatabi-pp-cli agent-context --json` for actual runtime paths. Domain cache defaults follow the runtime data path; `--db` chooses an explicit SQLite observation store. `--no-cache` skips live cache writes and auto fallback. Search does not overwrite richer cached detail. `--max-age` controls stale-read hints; `audit` also flags old publication/observation evidence.

## Automatic learning

The generated `recall`, `teach`, `learnings` and `playbook` commands operate on local state. Use `recall` before repeated discovery when that state is populated; always verify recalled IDs against fresh source data for travel decisions. Teach structural questions after a useful workflow; strip personal identifiers and sensitive data. Pass arbitrary user text as argv/tool input or read it from a file rather than interpolating it into shell source. `--no-learn` or `KURUMATABI_NO_LEARN=true` disables the loop for deterministic verification.

## Agent Feedback

Use `kurumatabi-pp-cli feedback --help` to inspect the local feedback command. Report provider drift with the actual canonical URL and bounded evidence; keep credential/session values out of reports.

## Output Delivery

Ordinary output goes to stdout. Inspect `kurumatabi-pp-cli --help` for shared output/delivery options before selecting an external destination; this skill authorizes read-only planning only.

## Named Profiles

Inspect `kurumatabi-pp-cli profile --help` for the generated local profile commands. Preserve existing profiles and prefer explicit `--db` or runtime home settings for isolated checks.

## Exit Codes

Domain commands use 0 success, 2 usage/input error, 3 uncached ID, 5 source failure, including a partial comparison, and 7 throttling. Unknown source evidence can be a successful JSON result; inspect its decision and predicates instead of treating exit 0 as permission.

## Argument Parsing

Validate positive finite vehicle measurements in metres, finite coordinate ranges and supported filter labels before network work. Negative numeric flags should use `--longitude=-120` form. `--dry-run` short-circuits before source/cache I/O.

## MCP Server Installation

The source tree includes an MCP build target mirroring the nine park planning commands with read-only hints. Build it with `go build -o kurumatabi-pp-mcp ./cmd/kurumatabi-pp-mcp`. Raw HTML source diagnostics are excluded from MCP. No remote listener is deployed. Inspect the README/build tree before choosing an MCP installation surface.

## Direct Use

Run `kurumatabi-pp-cli parks filters --json` or `kurumatabi-pp-cli parks --help` to discover the current domain surface. Detailed semantics and current bounds are in the source README and command help.

A partial comparison is emitted on stdout and returns exit5. Explicit file/webhook delivery occurs only after success; failed comparisons do not deliver the partial buffer.

Search caches every observation in the bounded page scan before applying the returned-row limit. Detail, fit, compare, audit and handoff require detailed observations for local reads or automatic fallback; refresh a search-only card with `parks detail ID --data-source live`. Multiple bath/onsen icons retain every source observation and qualified fee evidence.

MCP comparison failures retain bounded partial JSON in the first content block, with `isError: true` and a separate diagnostic capped at 4,000 bytes. Oversized evidence uses an explicit preview; total failure text is bounded to 64,000 bytes. `learnings forget` atomically removes matched teachings and affected inferred query/resource/venue families. Manually taught and unrelated rules remain. Verified prefix patterns require one literal match, including `%` and `_` characters.

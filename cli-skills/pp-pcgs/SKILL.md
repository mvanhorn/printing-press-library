---
name: pp-pcgs
description: "The first CLI for the PCGS Public API — cert lookup, full CoinFacts extraction, and the 1,000-call-per-day budget... Trigger phrases: `verify a pcgs cert`, `look up a pcgs cert number`, `extract pcgs coinfacts`, `pcgs population report`, `pcgs price guide`, `use pcgs`, `run pcgs`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pcgs-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/pcgs/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# PCGS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pcgs-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install pcgs --cli-only
   ```
2. Verify: `pcgs-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/pcgs/cmd/pcgs-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

PCGS gives you 1,000 API calls per day per token. This CLI tracks every call locally, forecasts batch cost before you spend a single one, syncs only mutable market fields on refresh, and ingests CSV / JSON / JSONL / plain-text cert lists straight into a local SQLite cache. No community wrapper exists. This is the only PCGS CLI.

## When to Use This CLI

Use this CLI when you need to verify PCGS certs at scale, extract full CoinFacts metadata for downstream import, or keep a local snapshot of population / price-guide / auction data refreshed under a tight 1,000-call daily budget. It is upstream of any personal collection system, not a replacement for one. Reach for it when an agent needs the raw PCGS record, not a curated subset.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Quota-aware orchestration
- **`coin batch`** — Parse a CSV / JSON wrapper / JSONL / plain-text cert list and look up every cert against PCGS. --dry-run forecasts cost (live calls, cache hits, %-of-quota) without spending a single call. --resumable + --checkpoint split a list larger than the 1,000/day cap across UTC days. --list-certs emits the parsed cert list to stdout without calling. Non-cert input columns round-trip to output as `_keep.<col>` for downstream re-keying.

  _Use this any time you have more than one cert to look up. The dry-run mode answers 'does this fit today's quota?' for free; --resumable makes multi-day batches idempotent._

  ```bash
  pcgs-pp-cli coin batch --file ./pcgs-coin-list.csv --dry-run --json
  ```
- **`refresh`** — Refresh cached coins by updating only the fields PCGS can actually change — Population, PopHigher, PriceGuideValue, AuctionList, Images, CoinFactsNotes — while leaving cert identity fields untouched. Emits a per-field diff. --dry-run --older 30d --field price-guide lists which cached coins need refresh without calling the API.

  _This is the safe way to keep market data current without putting cert identity at risk. Pair --dry-run --older with --field to find what to refresh before spending quota._

  ```bash
  pcgs-pp-cli refresh --all --older 7d --json
  ```

### PCGS-specific content patterns
- **`coin pop-curve`** — Pull every grade 1–70 (and PlusGrade variants, plus the 82–98 Details codes when --include-details) for one PCGSNo in one command, persist the full population curve to local store, and print the scarcity table.

  _Use this when you need scarcity context across grades — dealer pricing, key-date analysis, or seeing where Details-grade coins fit relative to numerical grades._

  ```bash
  pcgs-pp-cli coin pop-curve 7356 --plus --include-details --json
  ```
- **`order hydrate`** — Take a PCGS submission number, fetch the order, then fan out CoinFacts (and optionally images) for every cert in the order. Respects cache and refuses to start when remaining quota is less than the cert count.

  _Use this the moment a PCGS submission posts. It turns one submission number into a fully-cached, fully-hydrated set of coins in one command._

  ```bash
  pcgs-pp-cli order hydrate 12345678 --with-images --json
  ```

### Local state that compounds
- **`audit`** — Query the lookup_log table directly: every API call, its endpoint, IsValidRequest, ServerMessage, and request hash. Aggregate by day, by endpoint, or by cert; filter to failed calls only.

  _Reach for this when quota usage looks off, when you want to spot which certs return IsValidRequest=false, or when triaging a sync diff._

  ```bash
  pcgs-pp-cli audit --since 7d --failed --by-endpoint --json
  ```
- **`search`** — FTS5 + numeric filters over your local cache: search by Name, Country, SeriesName, Category, Designer, MintLocation, or variety fields, plus range filters on Year, Grade, PriceGuideValue, Population, PopHigher, Mintage, Weight, Diameter. --max-pop and --top-pct flags expose continuous rarity slicing (e.g. --top-pct 5 = top 5% rarest in the scoped cohort) — no API call.

  _Use this any time you need to slice your cached collection without burning quota. --top-pct 1 surfaces apex-rarity coins; --max-pop N for explicit thresholds._

  ```bash
  pcgs-pp-cli search --text "morgan dollar" --year 1881 --top-pct 5 --json --select Name,Year,Grade,Population,PriceGuideValue
  ```

## Command Reference

**banknote** — PCGS Banknote lookups: facts and images. Same shape as coins, separate endpoint family.

- `pcgs-pp-cli banknote facts-cert` — Full banknote metadata for one PCGS Banknote cert. Optional language code for translated text.
- `pcgs-pp-cli banknote facts-grade` — Banknote snapshot for a (PCGSNo, GradeNo) tuple.
- `pcgs-pp-cli banknote images` — Image URLs for one PCGS Banknote cert.

**coin** — PCGS-graded coin lookups: CoinFacts metadata, Auction Prices Realized (APR), and images.

- `pcgs-pp-cli coin apr-barcode` — Auction Prices Realized by holder barcode with optional date window.
- `pcgs-pp-cli coin apr-cert` — Auction Prices Realized for one PCGS cert number.
- `pcgs-pp-cli coin apr-grade` — Auction Prices Realized for a (PCGSNo, GradeNo, PlusGrade) tuple with optional date window and result limit.
- `pcgs-pp-cli coin facts-barcode` — CoinFacts metadata by holder barcode. Supports PCGS and competitor-service barcodes (NGC, ANACS, ICG, SEGS).
- `pcgs-pp-cli coin facts-cert` — Full CoinFacts metadata for one PCGS cert number. The IsValidRequest + ServerMessage envelope tells you if the cert...
- `pcgs-pp-cli coin facts-grade` — CoinFacts snapshot for a (PCGSNo, GradeNo, PlusGrade) tuple. Used by pop-curve to fan grades 1-70 + Plus + Details...
- `pcgs-pp-cli coin images` — TrueView and stock images for one PCGS cert (URLs only; no binary download).

**order** — PCGS submission and order lookups for submitters.

- `pcgs-pp-cli order range` — Orders within a date window (paginated).
- `pcgs-pp-cli order submission` — Orders associated with one PCGS submission number.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pcgs-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Verify one cert and dump every field

```bash
pcgs-pp-cli coin facts-cert 53972744 --json --select Name,Year,Grade,Population,PopHigher,PriceGuideValue,CoinFactsLink,IsValidRequest,ServerMessage
```

Single live call. The --json --select pair gives a deeply nested response trimmed to import-friendly fields without losing the IsValidRequest envelope.

### Plan a 500-cert batch

```bash
pcgs-pp-cli coin batch --file examples-pcgs-coin-list.csv --dry-run --json
```

Reads the local cache, dedupes against the fixture, and tells you whether the batch fits today's remaining quota — zero API calls spent.

### Mixed-shape fixture to clean cert list

```bash
pcgs-pp-cli coin batch --file examples-pcgs-coin-list.csv --list-certs --json
```

Auto-detects the {slabs:[…]} wrapper, strips plus-grade slab IDs like 7130.67/51225377 down to bare cert numbers, and emits them to stdout — pipe straight into coin batch.

### Pop curve for a Peace Dollar (with Details grades)

```bash
pcgs-pp-cli coin pop-curve 7356 --plus --include-details --json
```

One command pulls every grade across the 1-70 + Plus axis and the 82-98 Details codes, persists the curve, and prints the scarcity table. Hand-fans 70+ separate API calls behind the scenes with cache reuse.

### Local SQL over cached coins

```bash
pcgs-pp-cli search --text "morgan" --year 1881 --min-pop 100 --json --select Name,Year,Grade,Population,PopHigher,PriceGuideValue
```

No API call. Uses FTS5 + numeric filters against the local cache, with --select narrowing the response so an agent sees only the fields that matter.

### Find which cached coins need a sync

```bash
pcgs-pp-cli refresh --dry-run --field price-guide --older 30d --json
```

Picks the right input to sync: only the cached coins whose PriceGuideValue has not been refreshed in 30 days. Pipes directly into `sync --cert -`.

### Bullion-floor analysis (compose with spot prices)

```bash
pcgs-pp-cli coin facts-cert 53972744 --json | jq '.MetalContent, .Weight'
```

Recipe R1 — pair this output with current Pt/Au/Ag/Pd spot prices to compute the bullion floor. The CLI gives you metal content and weight; you multiply by spot. See article: market-101-silver-dollars-on-the-move.

### Cross-house auction spread (compose with Heritage / Stacks-Bowers / GreatCollections)

```bash
pcgs-pp-cli coin apr-cert 53972744 --json
```

Recipe R2 — PCGS APR is one realized-price snapshot. Pair it with the auction houses' own sold-lot APIs to spot lots that traded below PCGS APR or to find cross-house arbitrage. See article: keeping-the-pcgs-price-guide-updated.

### Survival-rate scarcity (compose with US Mint mintage)

```bash
pcgs-pp-cli coin pop-curve 7356 --json
```

Recipe R3 — PCGS pop ÷ original mintage = survival rate, a more honest scarcity signal than raw pop. The CLI gives you pop across grades; you cross-walk PCGSNo to mintage from US Mint records. See article: collecting-us-gold-coins-by-year.

### True-market scarcity (compose with NGC pop data)

```bash
pcgs-pp-cli coin pop-curve 7356 --json
```

Recipe R4 — join PCGS pop with NGC pop on (Year, Denom, Mint, Grade) to compute the full-market scarcity and the PCGS-only premium.

### Retail-vs-auction spread (compose with eBay sold listings)

```bash
pcgs-pp-cli coin apr-cert 53972744 --json
```

Recipe R5 — pair PCGS APR with eBay completed-sales to spot retail-vs-auction spreads and counterfeit-cheap PCGS-labeled coins. See: Cfomodz/what-bot.

### Inflation-adjusted realized prices (compose with FRED CPI)

```bash
pcgs-pp-cli coin apr-grade --pcgs-no 7356 --grade 65 --plus false --start 2000-01-01 --end 2026-01-01 --json
```

Recipe R6 — apply CPI ratio to APR Date+Price pairs to get real-dollar realized prices over time. See article: secrets-of-valuing-us-coins.

### Set Registry path planning (compose with PCGS Set Registry)

```bash
pcgs-pp-cli search --text "morgan" --min-grade 65 --json
```

Recipe R7 — cross-walk your cached cert list against PCGS Set Registry slot requirements to find which coins would qualify for top sets and the cheapest path to a top-100 set.

### Pre-show inventory refresh (compose with PCGS show calendar)

```bash
pcgs-pp-cli refresh --dry-run --field price-guide --older 30d --json
```

Recipe R8 — narrow refresh --dry-run output to series you're bringing to an upcoming show; pipe into refresh so the Price Guide values are current when you walk in.

### Holder-type premium (compose with holder-generation catalog)

```bash
pcgs-pp-cli coin facts-cert 53972744 --json
```

Recipe R9 — tag cached records by holder generation (OGH / rattler / blue label / gold-shield) using a cert-range + image lookup table. Keeping-the-pcgs-price-guide-updated notes holder-type premia.

### Personal collection diff (compose with your downstream system)

```bash
pcgs-pp-cli coin batch --file examples-pcgs-coin-list.csv --list-certs --json
```

Recipe R10 — diff coin batch --list-certs's normalized cert list against your downstream collection's cert export to find mismatches, missing certs, or certs that no longer match PCGS truth.

## Auth Setup

Set `PCGS_AUTH_TOKEN` to the bearer token generated at https://www.pcgs.com/publicapi. Run `pcgs-pp-cli doctor` to confirm reachability. All commands authenticate identically — there is one auth mode.

Run `pcgs-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pcgs-pp-cli banknote facts-cert mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
pcgs-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pcgs-pp-cli feedback --stdin < notes.txt
pcgs-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pcgs-pp-cli/feedback.jsonl`. They are never POSTed unless `PCGS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PCGS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pcgs-pp-cli profile save briefing --json
pcgs-pp-cli --profile briefing banknote facts-cert mock-value
pcgs-pp-cli profile list --json
pcgs-pp-cli profile show briefing
pcgs-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pcgs-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add pcgs-pp-mcp -- pcgs-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pcgs-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pcgs-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pcgs-pp-cli <command> --help`.

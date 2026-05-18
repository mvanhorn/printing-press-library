---
name: pp-miami-dade-clerk
description: "Pull the full lien chain on any Miami-Dade property in 30 seconds — every deed, mortgage, satisfaction, lis... Trigger phrases: `lien chain on this property`, `what liens survive this foreclosure`, `chain of title for this folio`, `what else does this owner control`, `federal tax liens on this folio`, `use miami-dade-clerk`, `run miami-dade-clerk`."
author: "Alex Kleis"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - miami-dade-clerk-pp-cli
---

# Miami-Dade Clerk Official Records — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `miami-dade-clerk-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install miami-dade-clerk --cli-only
   ```
2. Verify: `miami-dade-clerk-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Miami-Dade Clerk portal indexes deeds by property and everything else (mortgages, satisfactions, liens, lis pendens, judgments) by name. This CLI is the only tool that pivots property → owners → bounded name searches → filtered-back-to-property timeline, producing the chain-of-title and surviving-liens analysis that title companies charge $150-300 for.

## When to Use This CLI

Use this CLI when you need recorded-document data for Miami-Dade real estate: foreclosure auction underwriting (Max Safe Bid), title research (chain of title, open liens), owner due diligence (cross-property portfolio), or downstream ingest for an analytics pipeline. Best suited for property-by-property lookups (30-50 folios at a time); not a bulk-export tool.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`lien-chain`** — See every recording ever filed against a property in one chronological timeline — deeds, mortgages, satisfactions, lis pendens, federal tax liens, assignments — without manually deed-walking owner-by-owner.

  _When an agent is asked for the full encumbrance history on a Miami-Dade property (foreclosure underwriting, title research), this is the single command that produces it — bypassing the portal's split index._

  ```bash
  miami-dade-clerk-pp-cli lien-chain --folio 30-2232-066-1610 --agent
  ```
- **`surviving-liens`** — Tells you which liens on a property will survive a foreclosure or tax-deed sale, with totals in cents — Federal Tax Liens always survive tax deed; junior mortgages get wiped on senior foreclosure; HOA liens cap at FL Statute 720/718 safe-harbor amounts.

  _Computes the real Max Safe Bid for a foreclosure or tax-deed auction property. Pre-CLI, this number required a $150-300 O&E report from a title company._

  ```bash
  miami-dade-clerk-pp-cli surviving-liens --folio 30-2232-066-1610 --agent
  ```
- **`chain-of-title`** — Returns the ordered list of every deed conveying a property, with grantor, grantee, consideration, and recording date — and detects gaps where a grantee doesn't match the next deed's grantor.

  _The title-industry-standard 60-year chain-of-title deliverable, in one command. Agents can verify clean title before recommending a bid._

  ```bash
  miami-dade-clerk-pp-cli chain-of-title --folio 30-2232-066-1610 --since 1990-01-01 --agent
  ```

### Cross-entity insights
- **`owner-portfolio`** — Give a person or LLC name and see everything Miami-Dade has on them: every folio they appear on, every active mortgage or lien they've taken, every lis pendens or judgment against them.

  _Lead-gen for off-market deals and risk signal for already-distressed owners. Answers 'has this LLC defaulted elsewhere?' in one call._

  ```bash
  miami-dade-clerk-pp-cli owner-portfolio --last-name 'DEOD INVESTMENT LLC' --agent
  ```
- **`case-arc`** — Walks a single court case across all its recorded documents in chronological order: lis pendens → final judgment → certificate of title → satisfaction, with a status classifier (open / judgment-entered / sale-complete / dismissed).

  _Complements case-docket sources by adding the recording-layer view — when a judgment was actually recorded, when the certificate of title issued, whether subsequent satisfactions cleared the lien._

  ```bash
  miami-dade-clerk-pp-cli case-arc --case 2024-020991-CA-01 --agent
  ```

### Agent-native plumbing
- **`enrich`** — For a list of folios, compute lien-chain + surviving-liens + chain-of-title for each, and emit a single JSON file with totals_cents, surviving_lien_count, oldest_deed_date, last_deed_date, current_owner, FTL_count — designed for downstream ingest pipelines.

  _Lets a Tuesday-morning auction-prep workflow process 30+ properties in one invocation. The output JSON drops directly into a Supabase upsert._

  ```bash
  miami-dade-clerk-pp-cli enrich --folio-list folios.csv --out lien-summary.json
  ```
- **`ftl-scan`** — Find every Federal Tax Lien recorded after a given date, optionally filtered to a folio watchlist. FTLs survive tax-deed foreclosure, so any FTL on a TD-bound property is a deal-breaker red flag.

  _The single highest-signal alert for tax-deed investors. Today the v3 underwriter's `uw_td_govt_liens` column has no upstream feed._

  ```bash
  miami-dade-clerk-pp-cli ftl-scan --since 2024-01-01 --folio-list tuesday-auction.csv --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**doc-types** — Enumerate the 80+ recordable document types (3-letter codes + display labels)

- `miami-dade-clerk-pp-cli doc-types` — List all document types: codes (DEE, MOR, LIS, FTL, etc.) and display labels

**environment** — Portal environment metadata (status, maintenance windows, current server date)

- `miami-dade-clerk-pp-cli environment date` — Get current server date
- `miami-dade-clerk-pp-cli environment status` — Get portal status (open / maintenance / outage) and last recording-cutoff date

**search-name** — Search by party name (last + first) + optional document type. Returns name-indexed records: mortgages, satisfactions, assignments, lis pendens, judgments, federal tax liens, liens, court papers.

- `miami-dade-clerk-pp-cli search-name` — Submit a Name/Document search. Returns an encrypted qs token used to fetch the full result set.

**search-property** — Search by property address. Returns property-indexed records (deeds + quit claim deeds only — mortgages and other liens are name-indexed, use search-name).

- `miami-dade-clerk-pp-cli search-property` — Submit a Property/Condo search. Returns an encrypted qs token used to fetch the full result set.

**search-results** — Fetch the full record list for a previously-submitted search (using the qs token from search-name or search-property).

- `miami-dade-clerk-pp-cli search-results` — Get up to 500 recording records for a search. No native pagination; narrow by doc_type or date range to exceed cap.


**Hand-written commands**

- `miami-dade-clerk-pp-cli lien-chain` — Reconstruct the full lien chain for a Miami-Dade folio: every deed + mortgage + satisfaction + assignment + lis...
- `miami-dade-clerk-pp-cli surviving-liens` — Compute which liens on a folio will survive a foreclosure or tax-deed sale. Pairs each lien (MOR/LIE/FTL/JUD/LIS)...
- `miami-dade-clerk-pp-cli chain-of-title` — Return the ordered chain of title for a folio (every deed conveying the property, grantor → grantee → date →...
- `miami-dade-clerk-pp-cli owner-portfolio` — Scan one Name search for a person or LLC; group results into three buckets: properties owned, mortgages/liens taken,...
- `miami-dade-clerk-pp-cli case-arc` — Walk a single court case across all its recorded documents: lis pendens → final judgment → certificate of title...
- `miami-dade-clerk-pp-cli enrich` — Batch-enrich a list of folios: for each, compute lien-chain + surviving-liens + chain-of-title. Emits one JSON row...
- `miami-dade-clerk-pp-cli ftl-scan` — Find every Federal Tax Lien recorded after a given date, optionally filtered to a folio watchlist. FTLs survive...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
miami-dade-clerk-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Compute Max Safe Bid for a foreclosure auction property

```bash
miami-dade-clerk-pp-cli surviving-liens --folio 30-2232-066-1610 --agent --select totals_cents,liens.amount_cents,liens.doc_type,liens.recorded_at
```

Returns only the surviving-lien totals and per-lien details, suppressing the full record metadata. Pipe to the underwriter to compute auction Max Safe Bid.

### 60-year chain of title for a title commitment

```bash
miami-dade-clerk-pp-cli chain-of-title --folio 30-2232-066-1610 --since 1965-01-01 --agent
```

Walks every deed conveying the folio since 1965. Gap detector flags any chain breaks (grantee on one deed not matching grantor on the next).

### Has this LLC defaulted elsewhere in Miami-Dade?

```bash
miami-dade-clerk-pp-cli owner-portfolio --last-name 'DEOD INVESTMENT LLC' --agent
```

One Name search, post-processed into three buckets: properties owned, active mortgages/liens taken, lis pendens/judgments against. Lead-gen + risk signal.

### Tuesday auction prep — bulk-enrich 30 folios

```bash
miami-dade-clerk-pp-cli enrich --folio-list ./auction-tuesday.csv --out ./tuesday-summary.json
```

Orchestrates lien-chain + surviving-liens + chain-of-title for every folio in the CSV. Output JSON has totals_cents, FTL_count, current_owner per folio — drops directly into a Supabase upsert.

## Auth Setup

Public records, no login required. The clerk portal is gated by reCAPTCHA Enterprise v3 (invisible scoring); the CLI mints a fresh token per call from a headless Chromium driver via chromedp, so search commands work out of the box. `auth login --chrome` is only useful for reusing an existing browser session cookie when sticky NetScaler affinity matters.

Run `miami-dade-clerk-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  miami-dade-clerk-pp-cli doc-types --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
miami-dade-clerk-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
miami-dade-clerk-pp-cli feedback --stdin < notes.txt
miami-dade-clerk-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.miami-dade-clerk-pp-cli/feedback.jsonl`. They are never POSTed unless `MIAMI_DADE_CLERK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MIAMI_DADE_CLERK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
miami-dade-clerk-pp-cli profile save briefing --json
miami-dade-clerk-pp-cli --profile briefing doc-types
miami-dade-clerk-pp-cli profile list --json
miami-dade-clerk-pp-cli profile show briefing
miami-dade-clerk-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `miami-dade-clerk-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add miami-dade-clerk-pp-mcp -- miami-dade-clerk-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which miami-dade-clerk-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   miami-dade-clerk-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `miami-dade-clerk-pp-cli <command> --help`.

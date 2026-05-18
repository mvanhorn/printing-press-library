---
name: pp-edgar
description: "An agent-native CLI for SEC EDGAR — six sanctioned endpoints, SQLite-cached filings with FTS5, and LODESTAR-shaped... Trigger phrases: `primary sources for <ticker>`, `insider summary <ticker>`, `recheck delta for <ticker>`, `eightk items <ticker>`, `xbrl pivot`, `use edgar-pp-cli`, `run edgar`."
author: "magoo242"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - edgar-pp-cli
---

# SEC EDGAR — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `edgar-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install edgar --cli-only
   ```
2. Verify: `edgar-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Built for a Claude Code agent doing conviction research on public-company filings. Compound commands collapse a ticker's full primary-source pull into one structured response; insider-summary distinguishes discretionary code-S sales from RSU-tax code-F noise and flags senior officers separately. Local SQLite cache with tiered TTLs makes re-reads near-free; FTS5 over cached bodies eliminates re-hitting EDGAR for the same query.

## When to Use This CLI

Use edgar-pp-cli when an agent needs structured SEC filing data shaped for token-efficient consumption — primary-source bundles, insider-summary with S/F discrimination, cross-quarter MD&A diffs, or local FTS5 over cached filings. The compound primary-sources command is the LODESTAR shape: one call returns the full evidence pack for a Gate 1/2/3 thesis. Do not use this CLI for Federal Register searches, regulations.gov dockets, activist short reports, or NASDAQ short interest — those are out of scope and handled elsewhere.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`since`** — Return only the filings filed since a given timestamp, using a per-CIK local cursor — the entire LODESTAR /$recheck loop in one call.

  _For quarterly thesis rechecks, reach for this before fetching the full submissions index — it eliminates re-paying token cost on filings already seen._

  ```bash
  edgar-pp-cli since AAPL --as-of 2026-05-08 --json
  ```
- **`fts`** — Full-text search over locally-cached filing bodies via FTS5 — ticker- and form-scoped — with snippet windows and byte offsets for precise re-read.

  _Reach for fts during deep-dives where you re-query the same filing body multiple times; reach for the absorbed `efts` command for the first lookup or for cross-issuer queries._

  ```bash
  edgar-pp-cli fts "going concern" --ticker AAPL --form 10-Q --json
  ```

### Service-specific content patterns
- **`eightk-items`** — Enumerate 8-Ks with parsed Item numbers and a --material-only flag that excludes exhibits-only (Item 9.01 alone) refilings.

  _Use this instead of pulling 8-K bodies when you need 'has anything material happened?' — answers in one call without reading filing text._

  ```bash
  edgar-pp-cli eightk-items AAPL --since 2026-05-08 --material-only --json
  ```
- **`ownership-crosses`** — Enumerate 13D and 13G filings against an issuer (when someone else crosses 5% of the ticker), with filer name, percent owned, and filed-at.

  _Use in LODESTAR Gate 3 asymmetric-structure checks to spot activist or institutional concentration without scrolling submissions._

  ```bash
  edgar-pp-cli ownership-crosses AAPL --json
  ```
- **`governance-flags`** — Compose three independent service-specific signals into one call: 8-K Item 4.01 auditor changes, Item 4.02 non-reliance restatements, and NT-10-K late-filing notices (Form 12b-25).

  _Use as an early disqualifier check — if any flag fires, surface to LODESTAR before spending tokens on the full thesis._

  ```bash
  edgar-pp-cli governance-flags AAPL --since 2y --json
  ```

### Cross-entity joins
- **`insider-followthrough`** — For every senior-officer code-S sale of ≥$1M, scan the next 90 days of 8-Ks for material items and emit (sale, subsequent material 8-K, days-between) pairs.

  _Reach for this in LODESTAR Gate 2 execution-validation when an insider sale precedes material disclosures — surface management exits before bad news._

  ```bash
  edgar-pp-cli insider-followthrough AAPL --json
  ```

  **Form 4 ingest cap.** `insider-summary`, `insider-followthrough`, and `primary-sources` ingest Form 4 filings through a shared `--max-form4 N` cap (default `200`) that bounds DB/API pressure on high-volume filers. When the cap clips older filings, the output surfaces `form4_truncated: true` and `form4_total_in_window: <N>` under `form4_skipped` plus a stderr WARN — never silent. Pass `--max-form4 0` to disable; pass a larger value to widen the window.
- **`xbrl-pivot`** — Multi-ticker XBRL pivot that resolves concept aliases (Revenues ↔ RevenueFromContractWithCustomerExcludingAssessedTax ↔ SalesRevenueNet) into a flat ticker×quarter×concept table.

  _For cross-sectional quality screens — pivot before parsing 50 companyfacts JSON blobs by hand._

  ```bash
  edgar-pp-cli xbrl-pivot --tickers AAPL,MSFT,GOOGL --concepts Revenues,NetIncomeLoss --quarters 8 --csv
  ```

### Token-efficient extraction
- **`sections`** — Extract requested Items from a 10-K or 10-Q with byte-offset boundaries; emits ONLY the requested items in compact JSON instead of the full 100KB-10MB HTML body.

  _Use this instead of fetching the raw 10-K body — saves an order of magnitude in tokens when you only need Risk Factors and MD&A._

  ```bash
  edgar-pp-cli sections AAPL --form 10-K --items 1A,7,7A --json
  ```

## Command Reference

**companies** — Company identifiers (ticker → CIK) and per-issuer submissions index

- `edgar-pp-cli companies lookup` — Resolve ticker → CIK (and company name + SIC) from SEC's nightly index. Cache 24h in local SQLite.
- `edgar-pp-cli companies submissions` — Structured submissions index for a company (filing history with accession numbers, form types, filed-at).

**companyfacts** — XBRL company facts (financial concepts: revenue, net income, assets, cash flow, etc.) for a CIK

- `edgar-pp-cli companyfacts <cik>` — All XBRL company facts for a CIK. Optionally filter to a single concept client-side.

**efts** — EDGAR full-text search across all filings (efts.sec.gov). For offline FTS5 over cached filing bodies, use the offline `fts` command instead.

- `edgar-pp-cli efts <q>` — Online full-text search across EDGAR filings. For offline FTS5 over cached bodies, use `edgar-pp-cli fts`.

**filings** — Per-form-type filing retrieval and individual filing document fetch

- `edgar-pp-cli filings browse` — EDGAR filing index for a CIK + form type. Returns HTML/Atom; the generator wraps it but parsing happens in the...
- `edgar-pp-cli filings get` — Fetch raw filing index page or document for a specific accession. Accession must be the no-dashes form (e.g.,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
edgar-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Full LODESTAR research pull

```bash
edgar-pp-cli primary-sources AAPL --json --select shares_outstanding,senior_insider_sales,recent_eightk_items,def14a_governance
```

Compact JSON for an agent — primary-sources returns the full bundle; --select narrows to only the fields LODESTAR Gate evaluations actually need, dropping boilerplate.

### Quarterly recheck delta

```bash
edgar-pp-cli since AAPL --as-of 2026-05-08 --json
```

Only filings filed after the supplied timestamp. Local cursor — no SEC round-trip when the cache is warm. Pair with eightk-items --material-only when the delta includes 8-Ks.

### Insider follow-through pattern

```bash
edgar-pp-cli insider-followthrough AAPL --json
```

Cross-entity join: senior-officer code-S sales of ≥$1M paired with subsequent material 8-K items within 90 days. Surfaces management exits before bad news.

### Cross-sectional XBRL screen

```bash
edgar-pp-cli xbrl-pivot --tickers AAPL,MSFT,GOOGL,META --concepts Revenues,NetIncomeLoss --quarters 8 --csv
```

Multi-ticker pivot with concept-alias resolution — flat table for downstream screening or spreadsheet load. Use --csv for tabular pipes.

### Token-efficient 10-K item extraction

```bash
edgar-pp-cli sections AAPL --form 10-K --items 1A,7,7A --json --select item,text_offset,text
```

Pulls only Risk Factors (1A), MD&A (7), and Quantitative Disclosures (7A) with byte offsets — order of magnitude fewer tokens than the full filing body.

### Local FTS5 deep-dive

```bash
edgar-pp-cli fts "going concern" --ticker AAPL --form 10-Q --json
```

Offline FTS5 over cached filing bodies; emits snippet + byte offsets for precise re-read. Run sync first if cache is cold.

## Auth Setup

No API key — SEC EDGAR is publicly accessible. Identity is the User-Agent: set COMPANY_PP_CONTACT_EMAIL once and every request goes out as `lodestar-edgar-pp-cli <email>`. The CLI refuses to run if the env var is unset, with a clear error pointing at setup. Rate-limited to ≤2 req/sec sustained (well under SEC's 10 req/sec ceiling) with adaptive backoff on 429.

Run `edgar-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  edgar-pp-cli companyfacts mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
edgar-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
edgar-pp-cli feedback --stdin < notes.txt
edgar-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.edgar-pp-cli/feedback.jsonl`. They are never POSTed unless `EDGAR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EDGAR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
edgar-pp-cli profile save briefing --json
edgar-pp-cli --profile briefing companyfacts mock-value
edgar-pp-cli profile list --json
edgar-pp-cli profile show briefing
edgar-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `edgar-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add edgar-pp-mcp -- edgar-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which edgar-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   edgar-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `edgar-pp-cli <command> --help`.

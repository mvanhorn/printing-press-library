# Asoview CLI

**Discover and compare Japan leisure tickets with explicit price units, terms and public dated availability.**

Bounded first-party discovery, lazy detail and honest date/party coverage. Human handoff only.

## Authentication

Selected public GETs need no credentials or paid account.

## Quick Start

```bash
# Inspect runtime capabilities.
asoview-pp-cli doctor --dry-run

# Find Tokyo aquarium candidates.
asoview-pp-cli discover --region prf130000 --category 192 --agent

# Read age bands, terms and validity.
asoview-pp-cli product ticket0000049223 --agent

```

## Agent Usage

Business commands emit compact JSON by default. `--agent` enables the framework's non-interactive defaults. Inspect `agent-context --pretty`, `which "dated stock" --json` or command help for runtime vocabulary.

```bash
./asoview-pp-cli discover --region prf130000 --category 192 --agent --select results.id,results.name_ja,results.booking_url
./asoview-pp-cli product ticket0000049223 --agent --select results.id,results.options,results.validity,results.cancellation
```

Every business response includes `meta.sources` with source URLs, fetch times, cache status and TTL, plus request counts, bytes and latency. Missing/unsupported values are `null`, empty arrays or an explicit coverage reason. JSON stdout contains data; diagnostics use stderr. Explicit projection can omit metadata, so retain `meta` when freshness matters. Currency is JPY; units are the source's Japanese units (`枚`, `名`, etc.) or `null` when absent.

Optional response-cache write failures preserve successful live results and increment `meta.metrics.cache_write_failures`. Explicit inventory refresh still requires its reference inventory to be persisted successfully.

## Commands and coverage

| Command | Behavior |
|---|---|
| `discover` | Native region/category/date/adult/child filters; local `--query` and `--kind`; bounded pagination |
| `product ID` | Advertised bands, source age labels, inclusions, conditions, validity, cancellations, related product summaries; `--full` adds prose |
| `availability ID` | One date's calendar and slots, or one month of calendar days; requested quantity check |
| `options ID` | Advertised bands; `--date` retrieves public dated bands; timed tickets may require `--slot` |
| `compare ID ID` | Two to five products in a common terms/price schema; fails if any source fetch fails |
| `inventory` | Bundled or previously explicitly refreshed Japanese regions/categories; `--refresh-inventory` retrieves and saves the current source page |
| `handoff ID` | Derives a canonical product URL without checking existence or taking any booking action |
| `doctor` | Framework health and source reachability; `--dry-run` performs no network work |

`--region` and `--category` accept source IDs or exact Japanese names. Resolve names with `inventory --kind regions|categories --query TEXT`. This focused inventory contains Japanese region/prefecture filters and the provider's category/genre hierarchy. It does not provide an exhaustive municipal location index. Japanese pages are the verified language surface; other first-party language variants are unverified.

`--query` is a local substring over scanned title/base/category cards. It is **not a global keyword index**: an empty bounded result does not mean Asoview has no matching product. Native date/party search filters narrow candidates and do not prove stock or final prices. Returned totals count source venues, while results contain visible product cards.

Pagination defaults: ten matches from at most one source page. `--pages` allows 1–5; `--limit` allows 1–50; `--page` allows 1–200. Use returned `coverage.next_cursor` with `--cursor` to continue from an unreturned card on the same page. Inventory output defaults to twenty matches, at most one hundred. Slots and dated fee bands are capped at one hundred.

## Price and entry semantics

- `advertised_price` and undated option prices are advertised amounts. A ticket's minimum can be a child or senior band. It is not the adult price or a party total.
- Calendar and slot prices carry a date and distinct `basis`. Dated option bands expose source IDs, exact units and age-label text. Numeric ages are parsed only when stated; school-age labels retain unknown numeric boundaries.
- `options --date ... --party ID:COUNT,ID:COUNT` calculates a **band subtotal**, excluding extra fees, discounts, dependencies and checkout validation. `quote_confirmed` is false and `date_party_total` remains null.
- General admission validity is separate from a reserved date/time. A date-only ticket's opening window is labeled `admission_window_on_selected_date`; source slots carrying an explicit time-ticket schedule ID are labeled `reserved_entry_window`. Ambiguous source windows stay unclassified. No slot is reserved by this tool.
- General tickets with no public reserved-slot inventory return unknown dated stock with a coverage reason. Published excluded dates and expiry remain usable as negative eligibility evidence.
- Request-only, sold-out, closed, deadline-passed and unknown states are distinct. Remaining quantities are public snapshots; zero legacy maximums do not establish a quantity cap, and `quantity_limits_confirmed` stays false. Age/dependency eligibility remains unverified. Activity product age limits can disagree with dated fee labels; both are preserved.
- Cancellation rules and Japanese conditions are source text/structured data, including purchase-type-specific variants. Sales periods are labeled separately from ticket validity.

## Cookbook

```bash
./asoview-pp-cli discover --region prf130000 --category 192 --query 葛西 --pages 2
./asoview-pp-cli options ticket0000049223 --date 2026-10-02 --party 2411665:2,2411667:1
./asoview-pp-cli availability pln3000044589 --month 2026-10
./asoview-pp-cli compare ticket0000049223 ticket0000012233
./asoview-pp-cli handoff ticket0000049223 --agent
```

## Cache and runtime limits

Business commands use a task-specific public response cache under the OS cache directory `asoview-pp-cli/public-v1`. Override with `ASOVIEW_CACHE_DIR` or use `--home DIR`. Freshness: product HTML 24 hours, discovery five minutes, stock/dated fees thirty seconds. `--refresh` refreshes only requested responses; `--no-cache` bypasses all cache reads/writes. `--data-source live` bypasses business-response caching; `local` uses only fresh cache and never calls the source. Reference inventory refresh is explicit and saved for later commands; there is no automatic full catalog crawl.

The dedicated cache is bounded to 128 managed files / 32 MiB, pruned oldest first. Response bodies are capped at 4 MiB. Calls are sequential, paced at least 300 ms apart, and allow one retry for 429/502/503/504 within a five-second retry delay cap. Business command deadlines default to sixty seconds and `--timeout` may reduce them. At most twenty network attempts per business invocation. No browser is needed at runtime; the CLI is a Go executable with the generated Press framework. MCP stdio is also buildable with `go build ./cmd/asoview-pp-mcp`.

## Health Check

```bash
./asoview-pp-cli doctor --json
./asoview-pp-cli product ticket0000049223 --refresh --select meta
```

## Troubleshooting

Exit codes: 0 success, 2 input/projection error, 3 missing source, 4 access denied, 5 network/schema/cache-miss failure, 7 rate limit, 10 cache/config error. Errors never become a successful empty result. Upstream denial or layout changes require rechecking public access; the CLI does not import account cookies or bypass challenges. For cache write problems, use `--no-cache` or a writable `ASOVIEW_CACHE_DIR`. A calendar/fee schema failure is reported, not inferred as sold out.

Asoview's crawler rules disallow API/stock routes. This CLI performs explicit bounded read-only public requests and no crawler traversal. Public website contracts are unofficial and may change; `evidence/research.md` records access and coverage observations.

## Local artifacts and validation

The project is the editable source. `evidence/FINAL.md` records exact canonical staging/library/manuscript paths, independent review, Press acceptance, live source checks and cached/uncached output/request/latency/peak-memory measurements. `scripts/live_check.py` is a live public-source assertion runner; deterministic tests use clearly labeled synthetic responses.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Japan leisure planning
- **`compare`** — Compare a bounded shortlist and preserve missing values.

  _Compare a bounded shortlist and preserve missing values._

  ```bash
  asoview-pp-cli compare ticket0000049223 ticket0000012233 --agent
  ```
- **`discover`** — Filter Japanese names without claiming a global keyword index.

  _Filter Japanese names without claiming a global keyword index._

  ```bash
  asoview-pp-cli discover --region prf130000 --category 192 --query 葛西 --agent
  ```
- **`availability`** — Inspect public stock for a date and quantity.

  _Inspect public stock for a date and quantity._

  ```bash
  asoview-pp-cli availability ticket0000049223 --date 2026-10-02 --agent
  ```
- **`options`** — Read exact date-specific price units and age labels.

  _Read exact date-specific price units and age labels._

  ```bash
  asoview-pp-cli options pln3000044589 --date 2026-10-03 --agent
  ```
- **`inventory`** — Resolve region/category IDs with explicit refresh control.

  _Resolve region/category IDs with explicit refresh control._

  ```bash
  asoview-pp-cli inventory --kind categories --query 水族館 --agent
  ```

## Recipes

### Small shortlist

```bash
asoview-pp-cli discover --region prf130000 --category 192 --agent --select results.id,results.name_ja,results.booking_url
```

Project only handoff fields.

### Dated stock

```bash
asoview-pp-cli availability ticket0000049223 --date 2026-10-02 --agent
```

Stock is a snapshot, no booking.

### Terms comparison

```bash
asoview-pp-cli compare ticket0000049223 ticket0000012233 --agent
```

Compare advertised bands and ticket terms.

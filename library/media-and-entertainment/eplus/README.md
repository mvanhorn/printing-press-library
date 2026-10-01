# eplus CLI

**Discover Japanese performances and plan source-backed ticket sale windows.**

Created by [@zjsng](https://github.com/zjsng) (zjsng).


## Install and run

Requires Go 1.26.6 or newer. The catalog installer becomes available after this publication PR merges.

```bash
# Catalog installation after the publication PR merges:
npx -y @mvanhorn/printing-press-library install eplus --cli-only

# Build this checkout directly:
go build -o build/stage/bin/eplus-pp-cli ./cmd/eplus-pp-cli
./build/stage/bin/eplus-pp-cli --help
```

Use the built path, or add its directory to PATH. All examples below use `eplus-pp-cli`.

```bash
eplus-pp-cli events search --artist Radiohead --limit 3 --agent
eplus-pp-cli events search --from 2026-11-01 --to 2026-11-07 --category theatre --region kanto --agent
eplus-pp-cli events detail 4592490001-P0030001P021003 --agent
eplus-pp-cli international search --category concert --limit 5 --agent
eplus-pp-cli international detail 7078 --agent
eplus-pp-cli compare 7078 7079 --source international --agent
eplus-pp-cli policies --agent
```

## Search and inspect

Domestic search supports keyword/artist, service-date range, category and broad region. The source may retain other regions in its initial data; the CLI applies the region predicate to scanned records. `--venue` and `--location` are substring filters over scanned records: use Japanese names such as 東京都 or 大阪, and `--region kanto`/`kansai` to narrow the source first. `--limit` defaults to 10; `--pages` defaults to 1 (maximum 5); `--page` starts at 1. `meta.source_total` is the source count before local region/venue/location filters. Empty bounded scans explain how to widen them.

International search reads its own catalog, which may be small or empty for a category. Keyword/artist match titles. Date/venue/location filters lazily load tour schedules, bounded by `--max-scan` (default 5, maximum 20). Tour date ranges may describe streaming or archive periods. `international detail` accepts a tour slug, product ID, or returned product URL; use a product URL for exact performance and ticket terms. Search never silently expands to unrelated domestic offerings.

Detail defaults to 20 performance sessions, maximum 100. `compare` accepts two to four IDs from one surface and returns comparable session records plus each `input_id`. Failed reads appear in `meta.fetch_failures` and stderr; they never become empty phantom performances.

Comparison sessions are selected round-robin across successful inputs within the combined `--limit`. The limit must be at least the number of input IDs; capped or incomplete comparisons remain marked partial.

## Domain meaning

- `date` is the printed service date; RFC3339 times have `+09:00` and `timezone: Asia/Tokyo`. Explicit 24:xx–47:xx times normalize into the following calendar day while preserving `date`. Missing times stay `null`.
- A performance keeps `event_id`, `id`, source codes, Japanese name, venue and canonical URL. Sale rounds remain separate. Domestic search round codes and detail booking selectors use different source namespaces and are retained separately; an absent detail selector uses a content-derived ID.
- `kind` describes lottery, first-come or request processing; `phase` describes general sale/presale when explicitly named. Lottery acceptance never implies available seats. Source unknowns stay `unknown`.
- `starts_at`, `ends_at` and `lottery_deadline` preserve exact displayed sale windows. `booking_url` is a handoff, never a purchase action. Closed rounds may have no live booking link.
- Domestic `overseas_bookability` stays `unknown`. International product access is `conditional`; the country, residency, ID, collection and payment conditions on the specific product govern. FAQ guidance is generic.
- Missing fees/payment/collection/eligibility stay empty arrays with `terms_status: unknown` or `partial`, meaning unobserved, not free/unrestricted. Product `conditions` retain the visible terms. Static international JPY 0 placeholders are never treated as free tickets; a known selected variant is priced using the public read-only ticket-data endpoint.

## Agent output and runtime bounds

`--agent` emits compact JSON with `results` and `meta`, including observation URLs, original fetch time, cache status, partial coverage, warnings, request count, response bytes and elapsed time. Planning fields and terms survive compact mode. Project record fields with:

```bash
eplus-pp-cli events search --artist Radiohead --limit 3 --select id,name,date,sales --agent
eplus-pp-cli which "lottery deadlines" --json
eplus-pp-cli agent-context --json
eplus-pp-cli doctor --json
```

Diagnostics go to stderr. `--fresh` bypasses the two-minute cache; `--no-cache` also disables writes. `--data-source live` forces a fresh read; `auto` permits the short public cache; `local` is rejected because there is no offline mirror. `--cache-dir` or `--home` can isolate local state.

Serial HTTP reads, at most two requests/second, three attempts/request, four redirect hops, 4 MiB/body, 32 outbound attempts/command. A command has a 20-second default deadline; explicit `--timeout` is capped at 120 seconds, with at most 20 seconds/request. Lower `--rate-limit` is honored. Cache storage uses 256 URL-checked slots with bounded reads; ephemeral website tokens stay in memory. No browser runtime is required.

Usage errors exit 2; exhausted rate limits exit 7; source/network/parser failures exit nonzero with an actionable diagnostic. `--dry-run` makes no discovery requests. Output delivery supports stdout or a local file.

## Verification and known gaps

See [verification evidence](.manuscripts/20261001-020134-b06f779c/proofs/FINAL.md) for actual live verification, deterministic tests, measurements, independent review and Press receipts. Fixtures prove parser/state behavior; they are not evidence of current listings.

These are undocumented public website contracts and may change. Domestic SSR does not expose all checkout fees or gated eligibility. International variant combinations absent from public HTML remain unknown and require selection on the website. Catalog coverage is bounded; an empty result does not establish that no event exists. Discovery does not verify purchaser eligibility or guarantee inventory at checkout.

## Quick Start

```bash
# Discover bounded domestic performances.
eplus-pp-cli events search --artist Radiohead --limit 3 --agent

```

## Unique Features

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

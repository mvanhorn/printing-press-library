# ecbo cloak CLI

**Discover Japan luggage storage and inspect read-only source offers.**

## Build locally

```sh
go build -o ecbo-cloak-pp-cli ./cmd/ecbo-cloak-pp-cli
./ecbo-cloak-pp-cli --version
```

Requires Go 1.26.6+. Use the absolute binary path or add its directory to your shell PATH.

## Quick Start

```bash
# Check setup without a request
ecbo-cloak-pp-cli doctor --dry-run

# Find listed Tokyo facilities
ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --limit 3 --agent --select id,name,name_ja,booking_url

# Inspect station-specific restrictions
ecbo-cloak-pp-cli facilities get GBy4uBrI --agent

# Read exact source quote and validation
ecbo-cloak-pp-cli offer inspect 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from 2026-10-03T19:00 --to 2026-10-03T21:00 --small 1 --large 1 --agent --select quote,validation,availability,booking_url

```

## Agent Usage

Focused commands always emit compact JSON. Focused commands reject `--csv`, `--plain`, `--quiet`, and explicit `--compact` with usage exit 2; use `--select` to narrow JSON. The defaults supplied by `--agent` are supported. Diagnostics/errors go to stderr. `--agent` also sets noninteractive framework defaults. `--select` narrows each list result or the detail/offer payload, using comma-separated dot paths; metadata/pagination remain. Missing source values are `null`; unknown projections fail.

```sh
./ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --limit 3 --agent --select id,name,name_ja,booking_url
./ecbo-cloak-pp-cli offer inspect 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from 2026-10-03T19:00 --to 2026-10-03T21:00 --large 1 --agent --select quote,validation,availability,booking_url
```

`meta` includes wire request count, cache hits, elapsed milliseconds, source URLs and actual observation timestamps. Price and validation are always live. Detail defaults to a one-hour response cache; discovery to five minutes. `--refresh` refreshes discovery/detail; `--no-cache` bypasses them. Default cache is the OS cache directory under `ecbo-cloak-cli`; `--cache-dir` or `ECBO_CLOAK_CACHE_DIR` selects another. Explicit CLI `--home` confines the focused cache under that root; `--cache-dir` takes precedence. MCP callers cannot relocate cache/home paths. Cache is bounded to 256 files and 2 MiB responses. Requests are sequential, paced at 3/sec, with at most one retry for 429/5xx and a command deadline of 15s (configurable `--timeout`, 1s–60s). Network failures never become empty results.

## Health Check

```sh
./ecbo-cloak-pp-cli doctor --json
./ecbo-cloak-pp-cli doctor --dry-run
```

## Cookbook

```sh
# Nearby time/count filtering; matches still require exact offer validation.
./ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --from 2026-10-03T10:00 --to 2026-10-03T12:00 --large 1 --limit 5
# Next local page within the same source window.
./ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --limit 5 --offset 5
# Explicitly save the full bounded nearby window; then search offline.
./ecbo-cloak-pp-cli inventory refresh --lat 35.6812 --lon 139.7671 --radius-km 5
./ecbo-cloak-pp-cli inventory list --query 東京 --limit 5
```

Inventory refresh replaces one local snapshot, up to 50 facilities; list never refreshes automatically. No nationwide crawler or auto-sync. Source search ignores pagination parameters, so `--offset` paginates only the observed window. Name queries filter that window, not all of Japan. Source total can exceed the accessible window; `partial_coverage` stays explicit.

The source API accepts the declared language headers. The web frontend redirects `zh-CN` facility pages to Japanese, so booking links use its canonical Japanese route for that selection. Translated content may fall back to the provider’s original language.

## Interpreting offers

`listed` means a facility appears in the source. Listed maximum bag counts and source availability ratios are **not remaining capacity**. `source_filtered_match_unvalidated` describes a date/count search match. `source_validated_at_observation` means the provider's read-only validator accepted that exact interval/count request at observation time. It holds no slots and guarantees no subsequent availability. `confirmed_available_capacity` remains `null` because exact remaining counts are not publicly disclosed. A price quote alone does not establish availability.

Details preserve weekly/holiday opening hours, overnight/same-day/holiday flags and facility restrictions. Acceptance and pickup cutoffs are separate fields set to `null` when the source only publishes business hours; do not derive either from closing time. Cross-midnight closing times remain as published. Facility-specific restrictions can include long items requiring two suitcase slots. No guessed totals: requested totals come from the provider's price endpoint, with currency and underlying response preserved.

First-party language variants disagree at exactly 45cm and on calendar/business-day wording. The CLI marks that uncertainty rather than assigning a category silently. Daily category prices are per item/storage day; facility prices override general advertised starting rates. General weight limit is 20kg per piece; valuables/hazards/perishables and other restrictions apply. Review the live facility instructions and [first-party prohibited items](https://help.ecbo.io/en/articles/2018741-what-items-am-i-allowed-to-store).

## Troubleshooting

Exit 0: successful read (including empty local snapshot or source-rejected offer); 2: usage/input/projection; 3: facility not found; 4: access denied; 5: upstream/transport/response-shape failure; 7: exhausted rate limit; 10: local cache/config failure. Retry with `--refresh` for stale detail, use UUID after first legacy resolution to reduce requests, and increase `--timeout` up to 60s for slow responses. Inspect `validation` independently of `quote`; a rejected offer is a domain result.

## Known Gaps

The provider has no documented public API/SLA. Private website contracts may change. Exact remaining-count endpoint is authorization-gated and excluded. Discovery is a nearest 50-result window; no nationwide inventory claim. Structured independent acceptance/pickup cutoffs are unavailable. Arbitrary geography/text geocoding and non-ecbo providers are excluded. Generic generated `source` primitives are read-only raw helpers; prefer the focused commands for bounded normalized output.

## Evidence

See [evidence/final.md](evidence/final.md), [evidence/research.md](evidence/research.md), and the live dogfood, metrics and review artifacts linked there. The linked report records the original generation and local acceptance; publication reruns full live validation on the current source.

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`facilities near`** — Nearby discovery

  ```bash
  ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --limit 3 --agent --select id,name,name_ja,booking_url
  ```
- **`facilities get`** — Facility identity/hours/restrictions

  ```bash
  ecbo-cloak-pp-cli facilities get GBy4uBrI --agent
  ```
- **`offer inspect`** — Source date/time/count price and validity

  ```bash
  ecbo-cloak-pp-cli offer inspect 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from 2026-10-03T19:00 --to 2026-10-03T21:00 --small 1 --large 1 --agent --select quote,validation,availability,booking_url
  ```
- **`inventory refresh`** — Explicit bounded inventory refresh

  ```bash
  ecbo-cloak-pp-cli inventory refresh --lat 35.6812 --lon 139.7671 --limit 3 --agent
  ```
- **`inventory list`** — Offline inventory/query

  ```bash
  ecbo-cloak-pp-cli inventory list --limit 3 --agent
  ```

## Recipes

### Find nearby storage with Japanese names

```bash
ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --limit 3 --agent --select id,name,name_ja,booking_url
```

Find nearby storage with Japanese names

### Inspect source quote and validation separately

```bash
ecbo-cloak-pp-cli offer inspect 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from 2026-10-03T19:00 --to 2026-10-03T21:00 --small 1 --large 1 --agent --select quote,validation,availability,booking_url
```

Inspect source quote and validation separately

### Explicitly refresh a bounded window

```bash
ecbo-cloak-pp-cli inventory refresh --lat 35.6812 --lon 139.7671 --limit 3 --agent
```

Explicitly refresh a bounded window

### Read the last window offline

```bash
ecbo-cloak-pp-cli inventory list --limit 3 --agent
```

Read the last window offline

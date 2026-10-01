# Nifty Onsen CLI

Created by [@zjsng](https://github.com/zjsng) (zjsng).
**Find Japanese day-use baths with source evidence and explicit unknowns.**

## Build and start

Go 1.26.6 or newer:

```sh
go build -o nifty-onsen-pp-cli ./cmd/nifty-onsen-pp-cli
./nifty-onsen-pp-cli doctor --json
./nifty-onsen-pp-cli regions --agent
./nifty-onsen-pp-cli bath search --region=tokyo --filter=sauna --limit=5 --agent
./nifty-onsen-pp-cli bath show --id=onsen012278 --agent
./nifty-onsen-pp-cli bath coupons --id=onsen012278 --limit=3 --agent
./nifty-onsen-pp-cli bath nearby --lat=35.6895 --lon=139.6917 --limit=5 --agent
./nifty-onsen-pp-cli bath compare onsen012278 onsen001483 --agent
```

`filters` lists supported source filters and caveats. `--region` accepts a prefecture slug or Japanese name. Search and nearby default to Nifty's **day-use** classification. `--all-types` omits that default; `--filter=stay` then searches stay listings. Multiple filters within a source category use Nifty's bitmask semantics; inspect results before treating them as a strict intersection.

## Small output and bounded coverage

`--agent` writes compact, single-line JSON: `{meta, results}`. Metadata includes provider, source URL, freshness, request count, warnings and coverage. Missing facts are `null`; evidence claims use `unknown`, `source_claim` or `source_text`. Diagnostics go to stderr.

```sh
./nifty-onsen-pp-cli bath search --region=tokyo --select=id,name,url,rating --agent
./nifty-onsen-pp-cli bath show onsen012278 --select=name,admission,hours,policies --agent
./nifty-onsen-pp-cli bath search --region=tokyo --page=2 --limit=30 --agent
```

`--select` projects result fields and preserves metadata. `items.` and `results.` prefixes also work. Search defaults to 10 of at most 30 organic rows on one page. `--page` advances a **source page**, not an offset within a truncated response; raise `--limit=30` to retain every card before moving on. Details remain lazy. Coupons default to 3, with a cap of 20; `--full-text` includes the complete visible card text. Comparison accepts 2–5 unique IDs or canonical facility URLs; each successful row carries its own freshness, and failures remain explicit in metadata.

Nearby fetches at most 20 map candidates, ranks them by straight-line distance, then applies `--radius-km` (default 20) and `--limit`. `--zoom` controls the source map window. A small or empty shortlist is partial coverage, not proof there are no other facilities in that radius. There are no walking routes or reservable inventory claims.

## Domain interpretation

- A natural hot spring requires an explicit facility source label. Ordinary bath/sento categories remain distinct when the source identifies them; a name containing 温泉 does not establish natural spring water.
- Private baths, private rooms and the combined source filter **貸切風呂、個室風呂** remain separate evidence. A family-bath or combined tag does not prove a rentable private bath, its fee or available sessions.
- Admission stays Japanese source text: weekday/weekend/holiday, age bands, per-person/pair/session conditions and extras are preserved. A search minimum in JPY is a hint with unknown fee basis, not a payable quote.
- Tattoo, child and accessibility policies are unknown unless facility semantic fields explicitly mention them. Evidence text does not generalize admission permission. Source feature tags are not personal accessibility guarantees.
- Coupons preserve app/subscription restrictions, dates, pair/age conditions and add-ons. Acceptance and eligibility remain unknown. Links hand off to public information or the facility's official website; this CLI never issues/redeems coupons or books anything.

## Freshness and resource limits

The cache stores **parsed results**, not raw HTML, cookies or query keys. `auto` uses fresh cache, otherwise refreshes live and may fall back to older parsed facts with an explicit warning. `--data-source=live` forces network; `--data-source=local` requires that exact cached request and shows its age/staleness. `--no-cache` bypasses reads and writes. `--max-age` defaults to 30 minutes; 0 uses this client's 6-hour fallback TTL. `--home=/absolute/path` isolates all runtime files.

One HTTP client per invocation; sequential requests, default pacing 2 requests/second, 20-second HTTP timeout plus the command `--timeout` boundary. Response cap 4 MiB; map query cap 20; cache cap 128 entries / 32 MiB. HTTP 429 retries once only within a five-second retry budget and then raises an actionable rate-limit error. The map cookie and key live only in memory. Structural/usage errors exit 2; source/network errors exit nonzero and explain the fallback command.

## Limitations and evidence

Website layouts and source classifications may change. Public listings are neither complete inventory nor confirmed reservable capacity. No current-crowding, coupon redemption, purchases, reservations, account changes or bulk review collection. The embedded `regions` and `filters` catalogs are marked computed and reject `--data-source=live`.

The generated administrative helpers remain in the scaffold; `bath`, `regions` and `filters` are the provider workflows. MCP source is preserved and builds alongside the CLI. Final verification, measured requests/bytes/latency/RSS, review findings and canonical receipts are in [evidence/FINAL.md](evidence/FINAL.md).

## Quick Start

```bash
# Check the local installation
nifty-onsen-pp-cli doctor --dry-run

# Find source listings
nifty-onsen-pp-cli bath search --query=草津 --agent

# Inspect source admission and hours
nifty-onsen-pp-cli bath show --id=onsen012278 --agent

```

## Unique Features

Verified provider workflows in this build.
- **`bath show`** — Preserve natural hot spring, ordinary bath and private bath/room evidence separately.

  _Preserve natural hot spring, ordinary bath and private bath/room evidence separately._

  ```bash
  nifty-onsen-pp-cli bath show --id=onsen012278 --agent
  ```
- **`bath coupons`** — Read public validity, app/subscription, pair, age and holiday terms without issuing coupons.

  _Read public validity, app/subscription, pair, age and holiday terms without issuing coupons._

  ```bash
  nifty-onsen-pp-cli bath coupons --id=onsen012278 --limit=3 --agent
  ```
- **`bath nearby`** — Rank at most 20 live map candidates by straight-line distance with explicit partial coverage.

  _Rank at most 20 live map candidates by straight-line distance with explicit partial coverage._

  ```bash
  nifty-onsen-pp-cli bath nearby --lat=35.6895 --lon=139.6917 --limit=3 --agent
  ```
- **`bath search`** — Select compact organic listing facts while keeping independent freshness and pagination metadata.

  _Select compact organic listing facts while keeping independent freshness and pagination metadata._

  ```bash
  nifty-onsen-pp-cli bath search --region=tokyo --limit=3 --agent
  ```
- **`bath show`** — Reuse parsed source facts with timestamps and explicit offline/stale provenance.

  _Reuse parsed source facts with timestamps and explicit offline/stale provenance._

  ```bash
  nifty-onsen-pp-cli bath show --id=onsen012278 --agent
  ```

## Recipes

### Shortlist

```bash
nifty-onsen-pp-cli bath search --query=草津 --agent --select=id,name,url
```

Keep only shortlist facts

Homebrew publishing is unavailable until a personal tap is provisioned and verified. GitHub release builds remain configured; no Homebrew tap or formula installation is claimed.

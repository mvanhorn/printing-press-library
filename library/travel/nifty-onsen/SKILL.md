---
name: pp-nifty-onsen
description: Find Japanese day-use onsen, sento and baths with Nifty Onsen; inspect source admission, hours, access, private-bath evidence and public coupon terms; compare a small shortlist or find map candidates near explicit coordinates.
---

# Nifty Onsen

Use the read-only `nifty-onsen-pp-cli` for one-provider bath discovery. Start with `doctor --json`, then discover source geography with `regions --agent` and supported source categories with `filters --agent`. No account/API key is required.

## Build locally now

This workspace is an unpublished verified source build. From the project root, run `go build -o nifty-onsen-pp-cli ./cmd/nifty-onsen-pp-cli`, then verify `./nifty-onsen-pp-cli --version` and use that binary or add its directory to your runtime PATH.

The following generator-owned installation reference applies **only after publication**. No npm/public Go installation was verified for this build; use the local build above now.

## Prerequisites: Install the CLI

This skill drives the `nifty-onsen-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install nifty-onsen --cli-only
   ```
2. Verify: `nifty-onsen-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/cmd/nifty-onsen-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Discover then inspect

```sh
nifty-onsen-pp-cli bath search --region=tokyo --filter=sauna --limit=5 --agent
nifty-onsen-pp-cli bath search --query=草津 --select=id,name,url,rating --agent
nifty-onsen-pp-cli bath show --id=onsen012278 --agent
nifty-onsen-pp-cli bath coupons --id=onsen012278 --limit=3 --agent
nifty-onsen-pp-cli bath nearby --lat=35.6895 --lon=139.6917 --limit=5 --agent
nifty-onsen-pp-cli bath compare onsen012278 onsen001483 --agent
```

Search/nearby default to the source day-use category. Add `--all-types` to omit it; `--filter=stay` then selects stay listings. A hotel can support both classifications. Same-category source filters use bitmasks and may broaden the source match. Inspection, not a listing tag, is the next step for specific needs.

`--agent` emits `{meta, results}` compact JSON. `--select` projects result fields while keeping metadata. Search defaults to 10 of at most 30 cards from one source page: use `--limit=30` before `--page=2` to avoid dropping the tail of page 1. Nearby is at most 20 source candidates, sorted by straight-line distance with local `--radius-km`; widen/change `--zoom` or search by prefecture when coverage is too narrow. Details are lazy. Coupons default to 3; `--limit=20 --full-text` returns more public terms. Compare accepts 2–5 unique IDs/URLs and exposes partial fetch failures.

## Interpret evidence

Preserve Japanese names, IDs and URLs. Treat source ratings as Nifty ratings, not independent judgments. Admission text retains weekday/holiday, age, fee basis and extras; minimum price hints are not payable quotes. Natural hot spring requires an explicit source claim. Distinguish private bath, private room, combined category and family-bath labels; labels do not establish rental terms or capacity.

Tattoo/children/accessibility claims stay unknown unless facility fields explicitly say otherwise. `source_text` retains policy wording and does not establish universal permission. Coupons may require an app, a paid subscription, a pair, specific ages/dates or excluded special periods. Handoff URLs provide information only; eligibility and acceptance remain unknown.

Use freshness/coverage metadata. `--data-source=live` forces a refresh; `auto` may use fresh cache or an explicitly warned stale fallback. `--data-source=local` only works after the exact request has been cached. `--no-cache` bypasses both cache directions. `--timeout=45s` bounds the complete command. Throttling and semantic source errors are errors, not empty data. Embedded catalogs reject live mode. Use `--help` and `--dry-run --agent` for unfamiliar commands.

## Outside scope

Use another workflow for reservations, purchases/payments, coupon issuance/redemption, accounts, crowding, confirmed availability, hotel booking, complete radius inventories or personalized tattoo/accessibility guarantees. Never infer policy from generic navigation, reviews, a facility name or absence of a tag.

## Unique Capabilities

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

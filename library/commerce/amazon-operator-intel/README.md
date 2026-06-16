# amazon-operator-intel-pp-cli

Private local-first Amazon operator control tower for Amazon Seller and Amazon Ads evidence.

This is not another raw Amazon API wrapper. It shells out to `amazon-seller-pp-cli` and `amazon-ads-pp-cli` only when live sync is explicitly requested, merges their `--agent` JSON into a local store, and preserves source evidence on every row.

Default mode is offline:

- `sync` loads an embedded small-business fixture.
- `sync --import dataset.json` imports local JSON.
- `sync --source seller`, `sync --source ads`, `sync --source all`, `--live`, and `--real` opt into read-only child CLI calls.
- Credentials stay owned by the child CLIs.
- Doctors report config presence only and never print secret values.
- Automation output is recommendation/dry-run only in this first release.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Build

```bash
make build
# or
go build ./cmd/amazon-operator-intel-pp-cli
```

## First Commands

```bash
amazon-operator-intel-pp-cli --agent agent-context
amazon-operator-intel-pp-cli --agent doctor
amazon-operator-intel-pp-cli --agent sources doctor
amazon-operator-intel-pp-cli sync
amazon-operator-intel-pp-cli --agent war-room
```

`--agent` sets JSON, compact, no-input, yes, and no-color flags.

## Profiles

Profiles store non-secret defaults only:

```bash
amazon-operator-intel-pp-cli profile save \
  --name shop \
  --marketplace-id ATVPDKIKX0DER \
  --seller-id ASELLER \
  --ads-profile-id 123456789 \
  --days 30 \
  --cogs-file products.csv \
  --seller-store ~/.config/amazon-seller-pp-cli/store.db \
  --ads-report-dir ./reports \
  --target-acos 0.25 \
  --target-margin 0.18
```

Profiles live under `~/.amazon-operator-intel-pp-cli` unless `--home` or `AMAZON_OPERATOR_INTEL_HOME` overrides the directory.

## Live Sync

Live sync is read-only and explicit:

```bash
amazon-operator-intel-pp-cli sync --source seller --marketplace-id ATVPDKIKX0DER
amazon-operator-intel-pp-cli sync --source ads --ads-profile-id 123456789 --ads-report-dir ./reports
amazon-operator-intel-pp-cli sync --source all --marketplace-id ATVPDKIKX0DER --ads-profile-id 123456789
```

Single-source sync merges into the existing local dataset instead of replacing it. `sync --source all` validates required Seller and Ads config before running either child CLI.

## Operator Commands

- `war-room` - morning control tower with sales, profit, risk counts, and top actions.
- `restock-or-kill` - restock, conserve, pause ads, liquidate, fix listing, or monitor decisions.
- `ad-spend-guardrail` - read-only ad waste and margin guardrails.
- `sku-profit-truth` - contribution profit after ads, fees, returns, reimbursements, and COGS.
- `listing-triage` - listing defects ranked by business impact.
- `cash-leaks` - wasted spend, storage, stranded inventory, reimbursements, settlement gaps, returns, and low-margin ad spend.
- `search-term-actions` - promote, negate, rank-defense, and reduction actions across Ads and Brand Analytics.
- `digest daily` / `digest weekly` - owner-facing reports that are safe for empty datasets.
- `operator-plan` - one-week owner/delegate execution plan composed from other command scores.
- `cash-calendar` - inventory, ads, revenue, reimbursements, and storage cash forecast.
- `launch-readiness` - new ASIN/SKU launch decision and 14-day checklist.
- `rank-defense` - defend/reduce/do-not-defend paid search terms based on rank, cash, margin, and inventory.
- `bundle-opportunities` - market-basket bundles, cross-sells, virtual kits, and rejection reasons.
- `vendor-ops readiness|deductions|po-watch|scorecard` - local-file vendor workflows without Vendor Central API calls.

## Import Files

`sync --import` accepts a full `DataSet` JSON object or a `[]SKU` JSON array. Vendor commands accept CSV or JSON files. COGS files may be JSON maps, JSON row arrays, or CSV with `sku,cogs`.

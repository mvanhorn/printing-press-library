# Amazon Operator Intel Printing Press Skill

Use `amazon-operator-intel-pp-cli` when an agent needs a private, local-first Amazon owner/operator control tower across Seller and Ads evidence.

## Rules

- Run `amazon-operator-intel-pp-cli --agent agent-context` before automated workflows and honor schema `amazon-operator-intel.agent-context/v1`.
- Run `amazon-operator-intel-pp-cli --agent sources doctor` before live sync.
- Run `amazon-operator-intel-pp-cli sync` before analysis commands if local data is missing.
- Default `sync` and `sync --import` are offline and must not execute child CLIs.
- Live sync is explicit through `--source seller`, `--source ads`, `--source all`, `--live`, or `--real`.
- Credentials stay in `amazon-seller-pp-cli` and `amazon-ads-pp-cli`; never print secret values.
- Treat automation recommendations as read-only/dry-run in this first release.

## Source Model

The CLI stores SKU, campaign, search-term, listing, purchase-order, vendor-deduction, bundle, launch, and account-health rows with nested source evidence:

- `source.seller` from `amazon-seller-pp-cli`
- `source.ads` from `amazon-ads-pp-cli`
- `source.brand_analytics`
- `source.listings`
- `source.local_import`
- `source.vendor_files`

Do not silently drop unmatched rows. Seller-only SKUs, ads-only ASIN/SKU evidence, unattributed search terms, and listing defects without sales remain in the local dataset.

## Child CLI Sync

Read-only child commands include:

- `amazon-seller-pp-cli fba-inventory --agent --marketplace-ids <marketplace> --granularity-type Marketplace --granularity-id <marketplace>`
- `amazon-seller-pp-cli profitability sku-pnl --agent --marketplace-id <marketplace> --days <days>`
- `amazon-seller-pp-cli listing-intel health-audit --agent --marketplace-id <marketplace>`
- `amazon-seller-pp-cli brand-analytics search-terms --agent --marketplace-id <marketplace> --period WEEK`
- `amazon-ads-pp-cli portfolio-dashboard --agent --report <campaign-performance.csv>`
- `amazon-ads-pp-cli product-ad-profitability --agent --report <product-performance.csv>`
- `amazon-ads-pp-cli search-term-mining --agent --report <search-term-report.csv>`
- `amazon-ads-pp-cli wasted-spend --agent --report <search-term-report.csv>`

`sync --source all` requires Seller marketplace config and Ads profile config before either source runs.

## Commands

- `agent-context` - schema-versioned command, readiness, source-plan, and workflow descriptor.
- `doctor` - fast local readiness; use `--deep` to shell out to child `doctor` commands.
- `sources doctor` - per-source child binary and config readiness.
- `profile save/list/show/delete` - non-secret operator defaults.
- `sync` - fixture, import, or explicit read-only live child sync.
- `war-room` - compact daily control tower.
- `restock-or-kill` - SKU inventory/ad/listing decision queue.
- `ad-spend-guardrail` - ad waste, stockout spend, break-even ACOS, and weak-listing spend.
- `sku-profit-truth` - SKU contribution economics after ads and fees.
- `listing-triage` - listing defects ranked by business impact.
- `cash-leaks` - concise cash leakage queue.
- `search-term-actions` - promote, negative, rank, targeting, and dependency actions.
- `digest daily` / `digest weekly` - owner-facing reports.
- `operator-plan` - one-week execution plan with owners, hours, cash impact, source commands, and validation commands.
- `cash-calendar` - cash pressure forecast and reorder/ad-spend tradeoffs.
- `launch-readiness` - launch decision and 14-day checklist.
- `rank-defense` - defend/reduce/do-not-defend search-term tradeoffs.
- `bundle-opportunities` - bundle/cross-sell/virtual-kit recommendations and rejections.
- `vendor-ops readiness` - local vendor source readiness.
- `vendor-ops deductions` - local deduction dispute ranking.
- `vendor-ops po-watch` - local PO ship-window/fill-rate risk.
- `vendor-ops scorecard` - local-file vendor operational risk summary.

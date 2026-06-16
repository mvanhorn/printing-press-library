# Amazon Operator Intel Research Summary

Source plan: `~/.claude/plans/amazon-operator-intel.md`.

## Product Intent

Amazon Operator Intel is a private local-first composite CLI for a lean Amazon owner/operator. It answers: where is the business leaking money, inventory, attention, or growth this week?

The CLI is intentionally not a raw Amazon API wrapper. It uses the existing `amazon-seller-pp-cli` and `amazon-ads-pp-cli` as child CLIs, preserves source evidence, and keeps credentials owned by those child CLIs.

## Source CLIs

- `amazon-seller-pp-cli`: FBA inventory, profitability, listings, account health, sales intelligence, Brand Analytics. The live adapter uses the actual child CLI inventory flags: `fba-inventory --marketplace-ids <marketplace> --granularity-type Marketplace --granularity-id <marketplace>`.
- `amazon-ads-pp-cli`: profiles, campaign/product performance, ACOS/TACOS, search-term mining, wasted spend, bid/budget planning.

## First Release Scope

The first release includes the full required operator command surface:

- local context and readiness: `agent-context`, `doctor`, `sources doctor`
- state: `profile save/list/show/delete`, `sync`
- daily controls: `war-room`, `digest daily`, `digest weekly`
- inventory/profitability: `restock-or-kill`, `sku-profit-truth`, `cash-leaks`, `cash-calendar`
- ads/search: `ad-spend-guardrail`, `search-term-actions`, `rank-defense`
- listing/growth: `listing-triage`, `launch-readiness`, `bundle-opportunities`
- planning: `operator-plan`
- vendor-style local workflows: `vendor-ops readiness`, `vendor-ops deductions`, `vendor-ops po-watch`, `vendor-ops scorecard`

## Guardrails

- Default sync uses embedded fixture data and makes no child CLI calls.
- `sync --import` imports local JSON and makes no child CLI calls.
- Live sync requires explicit `--source`, `--live`, or `--real`.
- `sync --source all` validates Seller and Ads config before running any child command.
- First-release workflows are read-only; automation recommendations are dry-run only.
- No secret values are printed by doctor or source readiness output.

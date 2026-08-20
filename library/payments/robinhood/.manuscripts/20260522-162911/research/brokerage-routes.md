# Robinhood Brokerage Route Map

Source: community seed routes plus sanitized authenticated Chrome/CDP captures through 2026-05-27.

Personal repo semantics: mapped routes can be executed live with caller-owned `ROBINHOOD_BROKERAGE_TOKEN` or `ROBINHOOD_COOKIE`. Pass `--dry-run` when you want a non-sending test plan.

Current count: 259 route templates.
Risk counts: destructive=4, read=66, sensitive-read=176, write-mutate=1, write-or-sensitive=8, write-safe=4.

Per-endpoint files are generated in `api-map/markdown/endpoints/`. Each starts with `Mutation: yes` or `Mutation: no`.

| Risk | Methods | Categories | Host | Source | Route template |
|---|---|---|---|---|---|
| sensitive-read | GET | money-movement | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/acats-aggregation/fee_reimbursements/history` |
| sensitive-read | GET | money-movement | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/acats/` |
| sensitive-read | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/accounts/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/accounts/?default_to_all_accounts=true` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/accounts/{0}/recent_day_trades/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/accounts/{num}/buying_power_breakdown` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/accounts/stock_loan_payments/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/accounts/sweeps/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/accounts/sweeps/interest/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/accounts/sweeps/timeline_summary/` |
| read | inferred | money-movement | api.robinhood.com | community-seed | `https://api.robinhood.com/ach/received/transfers/` |
| write-or-sensitive | inferred | money-movement | api.robinhood.com | community-seed | `https://api.robinhood.com/ach/relationships/` |
| write-or-sensitive | inferred | money-movement | api.robinhood.com | community-seed | `https://api.robinhood.com/ach/relationships/{0}/` |
| destructive | inferred | money-movement | api.robinhood.com | community-seed | `https://api.robinhood.com/ach/relationships/{0}/unlink/` |
| write-or-sensitive | inferred | money-movement | api.robinhood.com | community-seed | `https://api.robinhood.com/ach/transfers/` |
| sensitive-read | GET | money-movement | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/banking/cross-sell/creditcard/applications/{uuid}` |
| sensitive-read | GET | unknown | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/bw/config` |
| read | GET | account, history-documents | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/cash_journal/margin_interest_charges/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/{id}` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/{id}/{id}` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/accounts` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/accounts/{id}/aggregated_positions` |
| sensitive-read | GET | account, orders | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/accounts/{id}/orders` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/accounts/{id}/pnl_cost_basis` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/cash_settlement_executions` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/futures_account_eligibility/{num}` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/manual_cash_correction` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/ceres/v1/user_settings` |
| read | inferred | auth | api.robinhood.com | community-seed | `https://api.robinhood.com/challenge/{0}/respond/` |
| sensitive-read | GET | orders | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/combo/orders/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/corp_actions/adr_fees/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/corp_actions/drip/enrollment/{num}/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/corp_actions/v2/split_payments/` |
| sensitive-read | GET | account, money-movement | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/crypto-transfers/account/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/devices/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/devices/disable_remove_device/` |
| sensitive-read | GET | watchlists | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/discovery/lists/` |
| sensitive-read | GET | watchlists | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/discovery/lists/default/` |
| sensitive-read | GET | watchlists | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/discovery/lists/items/` |
| sensitive-read | GET | watchlists | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/discovery/lists/user_items/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/discovery/ratings/{id}/overview/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/dividends/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/documents/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/documents/edocs_v2/custodial/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/documents/edocs_v2/ira/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/documents/edocs_v2/managed/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/documents/edocs_v2/rhc/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/documents/edocs_v2/rhd/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/documents/joint_account` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/fundamentals/` |
| write-safe | POST | telemetry-config | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/goku/{id}` |
| write-safe | POST | telemetry-config | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/goku/lcm` |
| write-safe | POST | telemetry-config | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/goku/lcmv2` |
| write-safe | POST | telemetry-config | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized | `https://api.robinhood.com/goku/live_frontend_log_events` |
| read | GET | telemetry-config | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/hippo/ux-flags` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/inbox/notifications/badge` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/inbox/threads/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/instruments/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/instruments/{0}/popularity/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/instruments/{0}/splits/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/instruments/{id}/shorting/` |
| read | GET | telemetry-config | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/kaizen/experiments/{id}/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/margin/{num}/upgrade_restrictions` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/margin/{num}/upgrade_restrictions/` |
| read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/margin/calls/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/earnings/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/equities/summary/robinhood/{id}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/forex/estimated_price/{uuid}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/forex/fundamentals/{uuid}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/forex/historicals/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/forex/historicals/{0}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/forex/quotes/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/marketdata/forex/quotes/{0}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/fundamentals/{id}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/fundamentals/short/v1/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/hedgefunds/summary/{id}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/hedgefunds/transactions/{id}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/historicals/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/historicals/{id}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/insiders/summary/{id}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/insiders/transactions/{id}/` |
| read | GET | marketdata, options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/options/` |
| read | inferred | marketdata, options | api.robinhood.com | community-seed | `https://api.robinhood.com/marketdata/options/historicals/{0}/` |
| read | GET | marketdata, options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/options/strategy/quotes/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/quotes/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/marketdata/quotes/{id}/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/markets/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/markets/{}/hours/{}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/markets/{market}/hours/{date}/` |
| sensitive-read | GET | money-movement | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/midlands/ach/iav_banks/{id}/` |
| read | inferred | watchlists | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/lists/default/` |
| read | inferred | watchlists | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/lists/items/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/movers/sp500/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/news/{0}/?` |
| sensitive-read | GET | notifications | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/midlands/notification_settings/ui_resources/` |
| sensitive-read | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/notifications/notification_tracker/` |
| sensitive-read | GET | notifications | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/midlands/notifications/stack/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/midlands/ratings/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/ratings/{0}/` |
| read | GET | marketdata | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/midlands/ratings/{id}/` |
| read | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/referral/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/tags/tag/{}/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/tags/tag/100-most-popular/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/midlands/tags/tag/top-movers/` |
| sensitive-read | GET | money-movement | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/nimbus/v1/asset_transfers` |
| sensitive-read | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/notifications/devices/` |
| read | inferred | auth | api.robinhood.com | community-seed | `https://api.robinhood.com/oauth2/token/` |
| sensitive-read | GET | options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options-product/tooltips/home-tab/` |
| sensitive-read | GET | options | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/{id}/` |
| sensitive-read | inferred | account, options | api.robinhood.com | community-seed | `https://api.robinhood.com/options/aggregate_positions/` |
| sensitive-read | GET | account, options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/aggregate_positions/?account_numbers=` |
| sensitive-read | GET | options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/chains/` |
| read | inferred | options | api.robinhood.com | community-seed | `https://api.robinhood.com/options/chains/{0}/` |
| sensitive-read | GET | options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/chains/{id}/` |
| sensitive-read | GET | options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/chains/{id}/collateral/` |
| sensitive-read | GET | history-documents, options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/corp_actions/` |
| read | GET | options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/events/` |
| read | GET | marketdata, options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/instruments/` |
| read | inferred | marketdata, options | api.robinhood.com | community-seed | `https://api.robinhood.com/options/instruments/{0}/` |
| sensitive-read | GET | options, orders | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/orders/` |
| destructive | inferred | options, orders | api.robinhood.com | community-seed | `https://api.robinhood.com/options/orders/{0}/cancel/` |
| sensitive-read | inferred | account, options | api.robinhood.com | community-seed | `https://api.robinhood.com/options/positions/` |
| sensitive-read | GET | account, options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/positions/?account_numbers=` |
| sensitive-read | GET | options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized | `https://api.robinhood.com/options/should_show_options_upgrade_on_sdp/` |
| sensitive-read | GET | options | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/options/strategies/` |
| sensitive-read | GET | orders | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/orders/` |
| destructive | inferred | orders | api.robinhood.com | community-seed | `https://api.robinhood.com/orders/{0}/cancel/` |
| sensitive-read | GET | orders | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/orders/session/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/pathfinder/concierge/plus/status/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/pathfinder/issues/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/pathfinder/support_chats/` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/pluto/historical_activities/` |
| sensitive-read | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/portfolios/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/portfolios/{num}/` |
| sensitive-read | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/portfolios/historicals/{0}/` |
| sensitive-read | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/positions/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/positions/?account_number=` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/quotes/` |
| read | inferred | marketdata | api.robinhood.com | community-seed | `https://api.robinhood.com/quotes/historicals/` |
| sensitive-read | GET | unknown | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/settings/education_state/{id}/` |
| write-or-sensitive | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/subscription/subscription_fees/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/user/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/user/additional_info/` |
| sensitive-read | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/user/basic_info/` |
| sensitive-read | inferred | account | api.robinhood.com | community-seed | `https://api.robinhood.com/user/investment_profile/` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/user/verify/email/info/` |
| write-or-sensitive | inferred | money-movement | api.robinhood.com | community-seed | `https://api.robinhood.com/wire/transfers` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/wonka/promotions/upsell_configs/BADGE` |
| sensitive-read | GET | account | api.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/wonka/promotions/upsell_configs/TRANSFER_HUB_ROW_UPSELL` |
| sensitive-read | GET | history-documents | api.robinhood.com | cdp-2026-05-26-stock-account-sanitized; cdp-2026-05-27-stock-account-sanitized | `https://api.robinhood.com/yoda/v1/list_advisor_trades` |
| sensitive-read | GET | money-movement | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/acats/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/account_switcher/instrument/v2/{uuid}/` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/accounts/{id}/{id}` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/accounts/{id}/currency_buying_power/{uuid}/info_alert` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/accounts/{id}/currency_buying_power/USD` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/accounts/{id}/instrument_buying_power/{uuid}/` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/accounts/{id}/unified/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/advisory/fees/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/app-comms/batch/surface/info-banner/` |
| write-mutate | POST | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/app-comms/receipt/seen/{uuid}/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/app-comms/surface/{id}/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/app-comms/surface/alert-sheet` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/app-comms/surface/hero-card` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/app-comms/surface/status-banner` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/crypto-yields/v1/history/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/crypto/crypto_migrations` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/crypto/cryptobility/{uuid}/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/crypto/fundamental_stats/{uuid}/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/crypto/transfers/history/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/edocs_orchestrator/{id}/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/education/tool_tips` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/education/tour/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/equities/history/{id}` |
| sensitive-read | GET | orders | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/equity_trading/order_type_selector/buy/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/feature-discovery/features/investing_below_card` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/gold/{id}/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/gold/get_subscription_list/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/gold/pill` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/gold/sweep_flow_splash/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/home/account_switcher/v2` |
| read | GET | marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/{uuid}/disclosures/` |
| read | GET | marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/{uuid}/etp-details/` |
| read | GET | marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/{uuid}/historical-chart/` |
| sensitive-read | GET | account, marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/{uuid}/margin-requirements/` |
| read | GET | marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/{uuid}/qa/event-info/` |
| read | GET | marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/{uuid}/qa/events-section/` |
| read | GET | marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/{uuid}/stock_detail/` |
| read | GET | marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/{uuid}/v2/warnings/` |
| read | GET | marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/chart-bounds/` |
| read | GET | marketdata | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/instruments/spans/` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/margin/{id}/` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/margin/{id}/buying_power_hub_view` |
| sensitive-read | GET | account, telemetry-config | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/margin/{id}/eligibility` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/margin/{id}/investing_info/` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/margin/{id}/settings/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/market_indices` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/onboarding/{id}/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/p2p/treatment/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/payment_instruments/v2/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/payment_instruments/v2/debitcard/{uuid}/` |
| write-or-sensitive | GET | money-movement, unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/paymenthub/unified_transfers/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/paymenthub/unified_transfers/{uuid}/contribution/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/portfolio/{id}/positions_v2` |
| sensitive-read | GET | money-movement | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/portfolio/acats/bonus-promo-info/` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/portfolio/account/{id}/live` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/portfolio/performance/{id}` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/portfolio/performance/{id}/settings_v2/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/psp/eligible_programs` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/psp/gifts/history/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/questionnaire/questionnaire-completed/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/rad/gifting/gifts` |
| sensitive-read | GET | orders | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/recurring_schedules/` |
| sensitive-read | GET | orders | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/recurring_schedules/equity/next_investment_date/` |
| sensitive-read | GET | orders | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/recurring_tradability/equity/{uuid}/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/recurring_trade_logs/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/region` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/retirement/history/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/rewards/reward/gift/crypto/list/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/rewards/reward/stocks/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/rewards/sdp_referral/card/{uuid}` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/rhy/accounts/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/screeners` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/screeners/presets/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/settings_page//account_contact/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/settings_page//account_preferences/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/settings_page//notifications/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/slip/{id}/` |
| sensitive-read | GET | telemetry-config | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/slip/eligibility/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/slip/hub-card/` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/sms/margin/{id}/{id}` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/tax_info/instrument/{uuid}/withholding_status/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/tax_info/withheld_amount/` |
| sensitive-read | GET | account | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/transfer/accounts/` |
| sensitive-read | GET | unknown | bonfire.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://bonfire.robinhood.com/user_status/stripe/` |
| sensitive-read | GET | money-movement | cashier.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://cashier.robinhood.com/ach/deposit_schedules/` |
| sensitive-read | GET | money-movement | cashier.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://cashier.robinhood.com/ach/relationships/` |
| sensitive-read | GET | unknown | dora.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://dora.robinhood.com/feed/` |
| sensitive-read | GET | unknown | dora.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://dora.robinhood.com/feed/instrument/{uuid}/` |
| read | GET | marketdata | dora.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://dora.robinhood.com/instruments/similar/{uuid}/` |
| sensitive-read | GET | unknown | identi.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://identi.robinhood.com/sorting_hat/v1/user_state/` |
| sensitive-read | GET | unknown | identi.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://identi.robinhood.com/sorting_hat/v4_web/` |
| sensitive-read | GET | unknown | identi.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://identi.robinhood.com/user_info/address/residential/` |
| sensitive-read | GET | unknown | identi.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://identi.robinhood.com/user_info/opt_out_consent/` |
| sensitive-read | GET | unknown | identi.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://identi.robinhood.com/user_info/opt_out_consent/ccpa_marketing/` |
| sensitive-read | GET | unknown | identi.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://identi.robinhood.com/user_info/privacy_consent/` |
| sensitive-read | GET | unknown | identi.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://identi.robinhood.com/user_info/profile_info/` |
| sensitive-read | GET | unknown | identi.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://identi.robinhood.com/user_info/trusted_contact/` |
| sensitive-read | GET | account | minerva.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://minerva.robinhood.com/accounts/` |
| sensitive-read | GET | unknown | minerva.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://minerva.robinhood.com/cards/declined_transactions/` |
| sensitive-read | GET | history-documents, unknown | minerva.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://minerva.robinhood.com/history/transactions/` |
| sensitive-read | GET | account | nummus.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://nummus.robinhood.com/accounts/` |
| sensitive-read | GET | unknown | nummus.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://nummus.robinhood.com/activations/` |
| read | GET | marketdata, unknown | nummus.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://nummus.robinhood.com/currency_pairs/` |
| read | GET | unknown | nummus.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://nummus.robinhood.com/holdings/` |
| write-or-sensitive | GET | orders | nummus.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://nummus.robinhood.com/orders/` |
| write-or-sensitive | inferred | orders | nummus.robinhood.com | community-seed | `https://nummus.robinhood.com/orders/{0}/` |
| destructive | inferred | orders | nummus.robinhood.com | community-seed | `https://nummus.robinhood.com/orders/{0}/cancel/` |
| sensitive-read | GET | account | nummus.robinhood.com | cdp-2026-05-27-stock-account-sanitized | `https://nummus.robinhood.com/portfolios/{uuid}/` |
| sensitive-read | inferred | account | phoenix.robinhood.com | community-seed | `https://phoenix.robinhood.com/accounts/unified` |

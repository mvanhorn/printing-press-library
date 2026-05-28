# Robinhood Live API Surface (CDP capture)

Captured 2026-05-28 via Chrome DevTools Protocol from an authenticated `robinhood.com/?classic=1` reload.
IDs masked (`:uuid`, `:id`); query strings stripped; OPTIONS preflights removed. GET = read; methods
with `[BODY]` carry a request payload. Robinhood is a multi-host REST API (no single base URL).

**Priority per Zayd:** brokerage (`api.robinhood.com`) + `bonfire` = 90%; `nummus` (crypto) = 10% (crypto already has an official API).

> WRITE endpoints (place/cancel order, etc.) fire on interaction, not page load. Order placement is MAPPED ONLY — real trades are never executed (prohibited financial action). Watchlist add/remove is a safe reversible write to capture next.

Total endpoints (GET/POST/PUT/etc., OPTIONS excluded): **127** across 8 hosts.

## api.robinhood.com (62)

- `GET /acats-aggregation/fee_reimbursements/history`
- `GET /acats/`
- `GET /accounts/`
- `GET /accounts/stock_loan_payments/`
- `GET /accounts/sweeps/`
- `GET /accounts/sweeps/interest/`
- `GET /banking/cross-sell/creditcard/applications/:uuid`
- `GET /cash_journal/margin_interest_charges/`
- `GET /ceres/v1/accounts`
- `GET /ceres/v1/accounts/:uuid/aggregated_positions`
- `GET /ceres/v1/accounts/:uuid/orders`
- `GET /ceres/v1/accounts/:uuid/pnl_cost_basis`
- `GET /ceres/v1/cash_settlement_executions`
- `GET /ceres/v1/manual_cash_correction`
- `GET /ceres/v1/user_settings`
- `GET /combo/orders/`
- `GET /corp_actions/adr_fees/`
- `GET /corp_actions/v2/split_payments/`
- `GET /discovery/lists/default/`
- `GET /discovery/lists/items/`
- `GET /discovery/lists/user_items/`
- `GET /dividends/`
- `GET /hippo/ux-flags`
- `GET /inbox/notifications/badge`
- `GET /inbox/threads/`
- `GET /instruments/`
- `GET /kaizen/experiments/:uuid/`
- `GET /marketdata/forex/historicals/`
- `GET /marketdata/forex/quotes/`
- `GET /marketdata/historicals/`
- `GET /marketdata/options/`
- `GET /marketdata/options/strategy/quotes/`
- `GET /marketdata/quotes/`
- `GET /markets/XNYS/hours/2016-12-30/`
- `GET /markets/XNYS/hours/2017-01-01/`
- `GET /markets/XNYS/hours/2017-01-03/`
- `GET /markets/XNYS/hours/2026-05-27/`
- `GET /markets/XNYS/hours/2026-05-28/`
- `GET /markets/XNYS/hours/2026-05-29/`
- `GET /midlands/notifications/stack/`
- `GET /nimbus/v1/asset_transfers`
- `GET /options-product/tooltips/home-tab/`
- `GET /options/aggregate_positions/`
- `GET /options/chains/`
- `GET /options/chains/:uuid/`
- `GET /options/corp_actions/`
- `GET /options/events/`
- `GET /options/instruments/`
- `GET /options/orders/`
- `GET /options/strategies/`
- `GET /orders/`
- `GET /pathfinder/issues/`
- `GET /pathfinder/support_chats/`
- `GET /pluto/historical_activities/`
- `GET /portfolios/:id/`
- `GET /positions/`
- `GET /user/`
- `GET /wonka/promotions/upsell_configs/BADGE`
- `GET /yoda/v1/list_advisor_trades`
- `POST /goku/lcm`
- `POST /goku/lcmv2` **[BODY]**
- `POST /goku/live_frontend_log_events`

## bonfire.robinhood.com (40)

- `GET /acats/`
- `GET /accounts/:id/unified/`
- `GET /advisory/fees/`
- `GET /app-comms/batch/surface/info-banner/`
- `GET /app-comms/surface/alert-sheet`
- `GET /app-comms/surface/full-screen-takeover-upsell/`
- `GET /app-comms/surface/status-banner`
- `GET /crypto-yields/v1/history/`
- `GET /crypto/crypto_migrations`
- `GET /crypto/transfers/history/`
- `GET /education/tool_tips`
- `GET /equities/history/aggregated_borrow_charge`
- `GET /feature-discovery/features/investing_below_card`
- `GET /gold/deposit_boost_adjustments/`
- `GET /gold/deposit_boost_paid_payouts/`
- `GET /gold/get_subscription_fee_list/`
- `GET /gold/pill`
- `GET /gold/sweep_flow_splash/`
- `GET /home/account_switcher/v2`
- `GET /instruments/chart-bounds/`
- `GET /market_indices`
- `GET /onboarding/resume_application_enabled/`
- `GET /p2p/treatment/`
- `GET /paymenthub/unified_transfers/`
- `GET /portfolio/:id/positions_v2`
- `GET /portfolio/account/:id/live`
- `GET /portfolio/performance/:id`
- `GET /portfolio/performance/:id/settings_v2/`
- `GET /psp/eligible_programs`
- `GET /psp/gifts/history/`
- `GET /rad/gifting/gifts`
- `GET /recurring_schedules/`
- `GET /recurring_trade_logs/`
- `GET /region`
- `GET /retirement/history/`
- `GET /rewards/reward/gift/crypto/list/`
- `GET /rewards/reward/stocks/`
- `GET /screeners`
- `GET /screeners/presets/`
- `GET /slip/updated-agreements-required/`

## cdn.robinhood.com (12)

- `GET /app_assets/microgram/app-accounts-post-chart-section/2a783883e08bfec99673cde7d5ac9964e7929a02/info.json`
- `GET /app_assets/microgram/app-accounts-post-chart-section/index.json`
- `GET /app_assets/microgram/app-dashboard-pill/7ffa7eeca49866d169c98804cc698051f3beda0a/info.json`
- `GET /app_assets/microgram/app-dashboard-pill/index.json`
- `GET /app_assets/microgram/app-echo-idl/b0c1c9e3c4ff6c9b9ff3c4b2bf87a1871bf454b4/info.json`
- `GET /app_assets/microgram/app-echo-idl/index.json`
- `GET /app_assets/microgram/app-mcw-fx-rates/b686f1ac6ed65147035344054e6252314caa1f4b/info.json`
- `GET /app_assets/microgram/app-mcw-fx-rates/index.json`
- `GET /app_assets/microgram/app-resurrection-lifetime-improvements/86bf2b39fe8f20aa331da2a22c772b291cd6d57b/info.json`
- `GET /app_assets/microgram/app-resurrection-lifetime-improvements/index.json`
- `GET /app_assets/rhv/live/production/config.json`
- `GET /static_content/structured/en-US/disclosure/4PtlcQJpYlVd58d0tP8HbY.json`

## dora.robinhood.com (1)

- `GET /feed/`

## identi.robinhood.com (3)

- `GET /sorting_hat/v1/user_state/`
- `GET /sorting_hat/v4_web/`
- `GET /user_info/privacy_consent/`

## minerva.robinhood.com (2)

- `GET /accounts/`
- `GET /history/transactions/`

## nummus.robinhood.com (6)

- `GET /accounts/`
- `GET /activations/`
- `GET /currency_pairs/`
- `GET /holdings/`
- `GET /orders/`
- `GET /portfolios/:uuid/`

## robinhood.com (1)

- `GET /`


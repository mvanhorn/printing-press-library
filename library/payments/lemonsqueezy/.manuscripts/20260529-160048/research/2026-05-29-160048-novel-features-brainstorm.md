# Lemon Squeezy CLI — Novel Features Brainstorm (subagent output)

## Customer model

**Persona 1: Maya — Solo SaaS founder, $8k MRR, picked LS over Stripe for the tax-handling.**

Today (without this CLI): Maya runs a 2-person team and lives in the LS dashboard. To answer "how was last week?" she clicks Stores → her store → Subscriptions tab → filters by status, then exports orders to CSV and pivots in a sheet. She has the official `lemonsqueezy.js` SDK in a Next.js admin page she half-built, but stopped because writing a fresh UI per question isn't worth it.

Weekly ritual: Monday morning she eyeballs MRR, scans for failed renewals (dunning), checks if her Founding-Member discount campaign still has redemption capacity, and pokes at the webhooks panel to make sure her PartnerUp-style app got every event since Friday.

Frustration: There's no fast answer to "which subs went `past_due` since Friday and what's the dollar exposure?" The dashboard shows status counts but not deltas, and the SDK gives her primitives, not the rollup.

**Persona 2: Diego — Indie maker selling a $99 desktop app via LS license keys.**

Today: He runs one product with three variants (single-seat / 3-seat / team). Refunds happen weekly. When a refund hits he opens the order, finds the license key, manually disables it, then opens `license-key-instances` to see if the buyer activated multiple machines so he knows how angry to be. None of this is scripted.

Weekly ritual: Sweep last week's refunds, disable the keys, scan for keys with abnormally high activation counts (piracy / seat-sharing), bulk re-issue keys for paying customers whose machines died.

Frustration: License-key ops are four endpoints stitched together by hand. The SDK has the calls but no "given this refunded order id, do the whole cascade" command. He's lost money to keys that stayed active after refund.

**Persona 3: Priya — Course creator running a Founding-Member sale (3 tiers, all 50-redemption capped).**

Today: She launched Lifetime-50, 2yr-50, 1yr-50 as three separate discount codes. She has no live view of how close each is to selling out, so she over-checks the dashboard every few hours during launch week. When the lifetime tier sold out she didn't expire the code for 90 minutes and oversold by four — had to refund manually.

Weekly ritual: During campaigns, she'd run a "where are we" check every couple hours. Outside campaigns, she still wants weekly redemption-pace numbers per code.

Frustration: `discount-redemptions` is a list endpoint. She wants "discount code X: 47/50 used, projected sellout in 6 hours at current pace, sibling code Y has 12/50 and is dragging." There is no campaign view anywhere.

**Persona 4: Sam — SaaS engineer integrating LS webhooks into PartnerUp.**

Today: When a `subscription_payment_failed` webhook doesn't fire (or fires twice), Sam diffs his app's DB against LS. He has to list webhooks per store, eyeball which one points to staging vs prod, hit "test" in the dashboard one event at a time, and pray. There's no replay-by-event-type from the API.

Weekly ritual: Audit webhook coverage (which event types subscribed across which stores), spot-check that no orphaned webhooks point at old ngrok URLs, replay a failing event to verify the new handler.

Frustration: Webhooks live across stores and event types, but the dashboard is per-store. He wants a single "show me every webhook, grouped by URL, with stale/active marker."

## Candidates (pre-cut)

(See Survivors and Killed Candidates below for the surviving + cut sets.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Revenue snapshot | `revenue-snapshot` | 8/10 | hand-code | Reads denormalized `thirty_day_revenue`, `thirty_day_sales`, `total_revenue`, `total_sales` from local `stores` rows + joins local `orders` to compute refund-adjusted net; `// pp:data-source local`, calls hintIfStale | Brief Data Layer flags these as "pre-rolled metrics in every row"; Codebase Intelligence: "sync stores once, you get a baseline" | Use this for a point-in-time revenue/sales rollup including one-off orders. For weekly time-series of MRR with new/churn/expansion split, use `mrr-trend`. |
| 2 | MRR trend | `mrr-trend --weeks 12` | 8/10 | hand-code | Joins local `subscriptions` + `subscription-invoices`, buckets by ISO week, classifies each delta as new/churn/expansion/contraction; `// pp:data-source local` | Brief Top Workflow #2; SDK exposes raw endpoints but not the rollup | Use this for time-series MRR. For a single point-in-time revenue rollup that includes one-off orders, use `revenue-snapshot`. |
| 3 | Churn watch | `churn-watch --since 7d` | 9/10 | hand-code | Cross-joins local `subscriptions` with prior-sync snapshot; lists transitions into past_due/unpaid/cancelled/expired with customer email + last invoice $; `// pp:data-source local` | Brief Top Workflow #2; Maya's weekly ritual; LS state machine documented | Use this for subscription status transitions in a window. For invoice-level failed charges where the subscription is still recoverable, use `dunning-alert`. |
| 4 | Dunning alert | `dunning-alert` | 8/10 | hand-code | SQL join: invoices where status='failed' AND subscription.status IN ('active','past_due'); `// pp:data-source local` | Brief Top Workflow #2; LS-specific because dunning is implicit in the `past_due` state machine | Use this for recoverable failed-charge windows. For status-change events on the subscription itself, use `churn-watch`. |
| 5 | License-key roll-up | `license-rollup` | 8/10 | hand-code | Joins local license-keys × license-key-instances × variants; outputs per-variant + per-key activation stats; `// pp:data-source local` | Brief Top Workflow #3; SDK has endpoints but no rollup; Diego's whole ritual | Use this for seat/usage distribution across keys. To act on one refunded order (disable keys, audit instances), use `refund-cascade`. |
| 6 | Refund cascade | `refund-cascade <order-id>` | 9/10 | hand-code | Given order id: fetches order + order-items, walks to license-keys, lists instances; with `--apply` calls disable-license-key per key; `// pp:data-source auto` | Brief Top Workflow #3; Diego loses money to keys staying active post-refund | Use this for the post-refund disable cascade on a specific order. For routine "find keys with abnormal seat counts" sweeps, use `license-rollup`. |
| 7 | Campaign watch | `campaign-watch [discount-code...]` | 9/10 | hand-code | Per code: reads local discounts (cap, used) + computes redemption velocity from local discount-redemptions over last 24h, projects sellout time; `// pp:data-source local` | Brief Top Workflow #5; Priya oversold by 4 because no live capacity view; Founding-Member sale matches user's MEMORY (`project_monetization_strategy`) | Use this for live capacity + pace tracking during a sale. For broad discount inventory regardless of activity, use the generated `list-discounts`. |
| 8 | Webhook audit | `webhook-audit` | 8/10 | hand-code | Lists local webhooks grouped by URL host, with event-type coverage matrix per store, flags hosts matching localhost/ngrok/*.test/*.local; `// pp:data-source local` | Brief Top Workflow #4; Sam's pain point — dashboard is per-store | Use this for cross-store webhook coverage + stale-host detection. For pruning the dead ones, pipe through the generated `delete-webhook`. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Sub state diff (`sub-state-diff`) | `churn-watch` already shows status transitions in a window with dollar exposure; a generic state-diff is a thinner, less actionable subset. | churn-watch |
| Seat-share sniff (`seat-share-sniff`) | Just `license-rollup` with a WHERE filter; not weekly enough to earn a dedicated command. | license-rollup |
| Founding tier orchestrator (`founding-tiers create`) | One-shot scaffolding (run once per campaign), not weekly; high-risk write surface for a single use. Users compose with 3× generated `create-discount` calls. | campaign-watch |
| Discount monitor (`discount-monitor`) | Subset of `campaign-watch` — listing >80%-full codes is just `campaign-watch` output filtered. | campaign-watch |
| Customer LTV (`customer-ltv`) | No persona ran this weekly; per-customer LTV is a sales-team feature, LS users are indie founders not running renewal calls. | revenue-snapshot |
| Catalog tree (`catalog-tree`) | Pre-launch one-shot; not weekly. The `include=` table-stakes handle the one-off case. | (absorbed `list-stores --include=products,variants,prices`) |
| Stale webhook prune (`webhook-prune`) | Mutation form of `webhook-audit`; users can pipe audit output into generated `delete-webhook`. | webhook-audit |

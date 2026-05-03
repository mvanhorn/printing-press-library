# Pagliacci CLI Absorb Manifest (v3 — reprint on printing-press v3.7.0, home-user persona)

## Ecosystem Scan Results

| # | Tool | Type | Features | Status |
|---|------|------|----------|--------|
| 1 | (none found) | — | — | Greenfield: no community CLIs, MCP servers, Claude plugins, npm/PyPI wrappers |
| 2 | Prior pagliacci-pizza-pp-cli (v0.x, 2026-04-26 from PP v2.3.6) | Internal | 57 absorbed, 6 transcendence | Reference for endpoint inventory; this run regenerates fresh on PP v3.7.0 with the home-user persona pivot. |

No external ecosystem to absorb. Pagliacci's API is undocumented and not widely reverse-engineered. The 33-endpoint inventory is unchanged from JS bundle extraction + browser-sniff in the prior run.

## Absorbed (every feature exposed by the API)

The 57-feature absorb table is unchanged from the 2026-04-26 manifest because the API surface is unchanged — every endpoint detected in `main.BPDM6VAU.js` plus the live captures still applies. Persona shifts don't add or remove API endpoints; they only change which endpoints we feature in the README/SKILL/recipes and which novel commands compose them.

For brevity, the absorb table is reused verbatim from `manuscripts/pagliacci-pizza-pp-cli/20260425-235234/research/2026-04-26-feat-pagliacci-pp-cli-absorb-manifest.md`. Summary by group:

- **System** (2): version, site-wide-message
- **Stores** (5): list, get, list-quotes, get-quote, compute-quote
- **Menu** (4): top, cache, slices, product-price
- **Scheduling** (3): time-window-days, time-windows (all + by date)
- **Address** (6): lookup, get-info, list (auth), get (auth), create (auth), delete (auth)
- **Account/Auth** (7): login, logout, register, password-forgot, password-reset, confirm-email, create-token
- **Customer** (5): get, access-devices list/delete, migrate-question, migrate-answer
- **Cart** (4): get, update, price, send
- **Orders** (6): list, get, list-pending, list-gift-cards, clone, suggestion
- **Rewards** (4): card, history, stored-coupons, coupon-lookup
- **Credit** (3): list, get, delete
- **Gifts** (6): list, get, delete, check (public), value, transfer
- **Feedback** (2): submit, get

**Total absorbed: 57 features mapping to 33 unique endpoints.**

Every command supports: `--json`, `--agent`, `--select`, `--csv`, `--compact`, `--data-source`, `--dry-run`, agent-friendly exit codes.

## Reprint Reconciliation (prior novel features re-scored against home-user persona)

| Prior # | Prior feature | Home-user fit | Verdict | Justification |
|---------|---------------|--------------|---------|---------------|
| T1 | Slices today across all stores | High | **KEEP** | "What pizza/slice should we order tonight?" is a household question; rotating slices give the family variety. Score holds at 8/10. |
| T2 | Open stores tonight | Medium-high | **KEEP** | Last-minute family dinner ("can we still get delivery?") is the same workflow whether the audience is solo or a 4-person household. Score holds at 8/10. |
| T3 | Stack discounts (rewards stack) | Higher than prior | **KEEP, reframed** | Bigger family orders ($40–80) hit reward thresholds where stacking actually saves real money. Reframed example uses a $55 family order. Score 8/10. |
| T4 | Reorder last / by ID | Higher than prior | **KEEP** | "The kids' usual" is the bread-and-butter household workflow. Reorder is the most-used feature in any food-delivery app for households. Score holds at 7/10. |
| T6 | Address-aware delivery time picker | Medium | **KEEP** | Useful for scheduling delivery to land at family dinner time. Less differentiated than the others, but cheap. Score holds at 7/10. |
| T8 | Spend summary | Reframe needed | **KEEP, reframed** | Prior framing ("budget tracker, reimbursement") was office-coordinator. Reframe to "household ordering rhythm" — `orders summary --since 90d` becomes "how often do we order, top items, store mix." Score 6/10. |

**Net result:** 6 of 6 prior novel features kept. Zero dropped. Two reframed for home-user audience (rewards stack, orders summary).

## New transcendence features for home-user persona

The home-user persona surfaces two patterns the prior office-coordinator brief missed:

| # | Feature | Command | Persona | Why Only We Can Do This | Score |
|---|---------|---------|---------|-------------------------|-------|
| T7 | Half-and-half pizza builder | `menu half-half --left pepperoni --right cheese --size large --json` | Family Cook | Composes MenuCache lookups + ProductPrice validation to produce a sendable cart entry for a half-and-half pie. Pagliacci's web UI supports half-and-half, but agents and the raw API require multi-step toppings/region modeling. We collapse it to one command. | 7/10 |
| T8 | Small-party order planner | `order plan --people 6 --address-label home --json` | Small-Party Host, Agent | Composes Store/QuoteStore + TimeWindows + MenuTop + RewardCard/StoredCoupons/StoredCredit into a single recommendation: which store, which delivery slot, suggested cart contents (sized to N people via 2.5 slices/person heuristic), and the optimal discount stack for the resulting total. Each component is a separate API call; the value is the composition. | 6/10 |

**Why these and not others I considered:**

- **Family menu favorites table**: idea was "show our top items from history". Rejected — already covered by `orders summary` reframe (top items is one of its outputs). Adding a separate command would duplicate.
- **Group fan-out cart (multi-address)**: prior brief had this as office-coordinator scope. Dropped — the home-user persona doesn't fan out across colleagues' addresses; one home address per order.
- **Lunch coordinator multi-cart**: same reason — office scope, not home scope.

## Final Transcendence Set (8 features)

| # | Feature | Command | Score | Source |
|---|---------|---------|-------|--------|
| T1 | Slices today across all stores | `slices today` | 8/10 | KEEP from prior |
| T2 | Open stores tonight | `stores tonight` | 8/10 | KEEP from prior |
| T3 | Stack discounts (rewards-aware) | `rewards stack` | 8/10 | KEEP, reframed for family totals |
| T4 | Reorder last / by ID | `orders reorder` | 7/10 | KEEP from prior |
| T5 | Address-aware delivery time picker | `address best-time` | 7/10 | KEEP from prior |
| T6 | Household ordering rhythm | `orders summary` | 6/10 | KEEP, reframed from "spend tracker" |
| T7 | Half-and-half pizza builder | `menu half-half` | 7/10 | NEW for home-user persona |
| T8 | Small-party order planner | `order plan --people N` | 6/10 | NEW for small-party persona |

**Transcendence total: 8 features, all scoring >= 6/10, 3 scoring >= 8/10.**

## Stub policy
None. Every feature listed in this manifest is shipping scope. If implementation is infeasible during build, return to this gate per the rules.

## Summary
- **Absorbed:** 57 features (mapping to 33 unique endpoints)
- **Transcendence:** 8 user-first features for home-user / family / small-party (6 KEEP from prior, 2 NEW)
- **Total:** 65 user-facing capabilities
- **Best existing tool:** None (greenfield API)
- **Our advantage:** First and only CLI for the Pagliacci API, with composed cookie auth (`auth login --chrome`), local SQLite for offline + agent use, and 8 transcendence features no Pagliacci surface offers — including the home-user-specific half-and-half builder and small-party planner.

## Source Priority
Single source. The Multi-Source Priority Gate does not apply — only `pagliacci.com` is named.

## Auth Profile
- Type: `composed`
- Header: `Authorization`
- Format: `PagliacciAuth {customerId}|{authToken}`
- Cookie domain: `pagliacci.com`
- Required cookies: `customerId`, `authToken`
- Login mechanism: `auth login --chrome` reads cookies from active Chrome profile and writes to config. `auth set-token` accepts a paste of the constructed header for users who don't want to use Chrome.

## v3.7.0 MCP Surface Plan
- **Tool count estimate:** 33 typed endpoints + ~13 default framework tools + 8 novel transcendence commands = ~54 total tools, exceeding the 50-tool threshold.
- **Plan:** Apply the Cloudflare pattern in spec enrichment before generation:
  - `mcp.transport: [stdio, http]` — remote-capable
  - `mcp.orchestration: code` — emit `pagliacci_search` + `pagliacci_execute` orchestration pair
  - `mcp.endpoint_tools: hidden` — suppress raw per-endpoint mirrors
- **Why:** scorecard's `mcp_remote_transport` / `mcp_tool_design` / `mcp_surface_strategy` dims need the Cloudflare pattern at this size, and the runtime token cost of 54 raw tools is excessive for any agent host.

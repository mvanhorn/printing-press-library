# polymarket-pp-cli — Absorb Manifest

> Generated 2026-05-23. Companion to the [research brief](./2026-05-23-feat-polymarket-pp-cli-brief.md) and the [novel-features brainstorm](./2026-05-23-novel-features-brainstorm.md).

Polymarket has the deepest competing-tool surface area of any printed CLI so far: official Rust CLI by Polymarket itself (~55 commands), 4 active community MCP servers, 2 official Python SDKs (v1 archived, v2 current), and the Polymarket Agents framework. Our absorb target is 100% of what they collectively expose, then 6 transcendence features no existing tool offers.

## Absorbed (match or beat everything that exists)

Total absorbed features: **72**. Sources: P=Polymarket Rust CLI, IQ=IQAIcom MCP, GX=guangxiang MCP, CI=CarlosIbCu MCP, V2=py-clob-client-v2, AG=Polymarket Agents.

### Discovery (Gamma API, no auth)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| A1 | List markets w/ filters (active, closed, limit, offset, tag) | P `markets list`, IQ `list-active-markets`+`get-markets-by-tag`, GX `get_markets` | Spec-derived endpoint + local SQLite snapshot | Offline FTS5 query, `--select` field narrowing, agent-native JSON |
| A2 | Search markets by text | P `markets search`, IQ `search-markets` | API search + offline FTS5 | Works offline once synced |
| A3 | Get market by id or slug | P `markets get`, IQ `get-market-by-slug`, CI `get_market` | Single endpoint | Same shape across slug and id |
| A4 | List events w/ filters | P `events list`, GX `get_events` | Spec-derived | `--select` narrowing |
| A5 | Get event by id/slug | P `events get`, IQ `get-event-by-slug` | Spec-derived | |
| A6 | Event tags | P `events tags` | Spec-derived | |
| A7 | List series | P `series list` | Spec-derived | |
| A8 | Get series | P `series get` | Spec-derived | |
| A9 | List all tags | IQ `get-all-tags` | Spec-derived | |
| A10 | Comments on markets/events | Gamma `/comments` | Spec-derived | |
| A11 | Lookup public profile | Gamma `/profile` | Spec-derived | |

### Market data (CLOB reads, no auth)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| A12 | Order book for a token | P `clob book`, IQ `get-order-book`, GX `get_orderbook` | Spec-derived + local snapshot | Live + cached comparison |
| A13 | Current price | P `clob price` | Spec-derived | |
| A14 | Midpoint price | P `clob midpoint` | Spec-derived | |
| A15 | Spread | P `clob spread` | Spec-derived | |
| A16 | Price history w/ interval | P `clob price-history`, GX `get_market_prices` | Spec-derived + local time-series store | Diff/window queries enabled |
| A17 | CLOB server time | P `clob server-time` | Spec-derived | |
| A18 | Neg-risk markets | P `clob neg-risk` | Spec-derived | |
| A19 | CLOB markets + single market | P `clob markets`, `clob market <id>` | Spec-derived | |
| A20 | Trade history w/ filters | P `clob trades`, IQ `get-trade-history`, GX `get_trades` | Spec-derived + local persistence | Offline aggregation |

### Trading (CLOB writes, L1+L2 auth)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| A21 | Place limit order (GTC) | P `clob create-order`, IQ `place-order`, V2 `create_and_post_order` | EIP-712 sign locally, POST /order | `--dry-run` prints request without sending |
| A22 | Place limit order (GTD w/ expiration) | P `clob create-order --type GTD`, V2 OrderType.GTD | Same w/ expiration | |
| A23 | Place limit order (FOK) | V2 OrderType.FOK | Same w/ FOK | |
| A24 | Place limit order (FAK) | V2 OrderType.FAK | Same w/ FAK | |
| A25 | Place market order | P `clob market-order`, IQ `place-market-order` | MarketOrderArgs path | |
| A26 | Cancel order by id | P `clob cancel`, IQ `cancel-order` | DELETE /order/{id} | |
| A27 | Cancel multiple orders | P `clob cancel-orders` | Batch cancel | |
| A28 | Cancel all orders for a market | P `clob cancel-market` | Market-scoped cancel | |
| A29 | Cancel all orders | P `clob cancel-all`, IQ `cancel-all-orders` | Global cancel | |
| A30 | List user open orders | P `clob orders`, IQ `get-open-orders` | GET /orders | |
| A31 | Get single order by id | P `clob order <id>`, IQ `get-order` | GET /order/{id} | |
| A32 | List user trades | P `clob trades` (user-scoped) | GET /trades | |
| A33 | Balance check | P `clob balance --asset-type` | GET /balance | |
| A34 | Order with custom tick_size | V2 PartialCreateOrderOptions(tick_size) | Flag on create-order | |

### Rewards (CLOB)

| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|--------------------|
| A35 | Daily rewards | P `clob rewards --date` | Spec-derived |
| A36 | Daily earnings | P `clob earnings --date` | Spec-derived |
| A37 | Earnings per market | P `clob earnings-markets --date` | Spec-derived |
| A38 | Reward percentages | P `clob reward-percentages` | Spec-derived |
| A39 | Current rewards | P `clob current-rewards` | Spec-derived |
| A40 | Reward config for a market | P `clob market-reward` | Spec-derived |
| A41 | Single order scoring | P `clob order-scoring` | Spec-derived |
| A42 | Bulk orders scoring | P `clob orders-scoring` | Spec-derived |

### API key management (L1 → L2 bootstrap)

| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|--------------------|
| A43 | List API keys | P `clob api-keys` | GET /auth/api-keys |
| A44 | Create / derive API key | P `clob create-api-key`/`derive-api-key`, V2 `create_or_derive_api_key` | POST /auth/api-key with EIP-712. **Impl note (from reframed C13):** wrap this in an orchestration mode that also runs A51 if approvals aren't set, providing a single-command path from PK env var → fully trading-ready. |
| A45 | Delete API key | P `clob delete-api-key` | DELETE /auth/api-key |

### Wallet management

| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|--------------------|
| A46 | Generate new wallet | P `wallet create [--force]` | Local keygen → config |
| A47 | Import existing PK | P `wallet import <pk>` | Local config write |
| A48 | Show wallet info | P `wallet show` | Local config read |
| A49 | Print wallet address | P `wallet address` | Local config read |
| A50 | Reset wallet config | P `wallet reset [--force]` | Local config delete |

### On-chain (Polygon)

| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|--------------------|
| A51 | Set all contract approvals (6-tx batch) | P `approve set`, IQ `approve-allowances` | go-ethereum tx batch |
| A52 | CTF split (USDC → YES+NO) | P `ctf split` | Exchange contract call |
| A53 | CTF merge (YES+NO → USDC) | P `ctf merge` | Exchange contract call |
| A54 | CTF redeem (winners → USDC) | P `ctf redeem`, IQ `redeem-positions` | CTF contract call |
| A55 | Check on-chain balance + allowance | IQ `get-balance-allowance` | RPC call |
| A56 | Update on-chain allowance to specific amount | IQ `update-balance-allowance` | ERC20 approve call |

### Data API (analytics)

| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|--------------------|
| A57 | Positions for a wallet w/ valuation + P&L | P `data positions`, IQ `get-positions`, GX `get_user_positions` | GET /positions |
| A58 | Portfolio total value | P `data value` | GET /value |
| A59 | User activity feed (TRADE/SPLIT/MERGE/REDEEM/REWARD/CONVERSION) | GX `get_user_activity` | GET /activity. **Impl note (from reframed C4):** support `--by taker` aggregation for whale-tail analysis. |
| A60 | Market holder distribution | GX `get_market_holders` | GET /holders |

### WebSocket

| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|--------------------|
| A61 | Live order book subscription | Polymarket docs + all SDKs | WSS `/ws/market` with subscription frame |
| A62 | Live user feed | Polymarket docs + V2 | WSS `/ws/user` |

### Framework / meta

| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|--------------------|
| A63 | Doctor / status | P `status`, framework default | Multi-tier auth probe + API reachability. **Impl note (from reframed C9):** distinguish 4 failure modes — API down, Cloudflare-blocked-IP, auth-tier mismatch, rate-limited — each with a specific remediation hint and a distinct typed exit code. |
| A64 | Guided setup wizard | P `setup` | Interactive config bootstrap |
| A65 | Self-upgrade | P `upgrade` | Skip — `go install` handles this; document instead of implementing |
| A66 | Sync to local SQLite | Framework default | Sync command |
| A67 | FTS5 search across synced entities | Framework default | Search command |
| A68 | Raw SQL query | Framework default | SQL command |
| A69 | Reconcile (re-sync diff) | Framework default | Reconcile command |
| A70 | Stale-resource check | Framework default | Stale command |
| A71 | Help / version / agent-context | Framework default | Cobra defaults |
| A72 | MCP server (Cloudflare pattern) | Framework default | `mcp.transport: [stdio, http]`, `mcp.orchestration: code`, `mcp.endpoint_tools: hidden` — required given ~80+ Cobra commands |

**Implementation-note enrichments** (from killed candidates that reframed into absorbed-feature flags):

- `clob create-order` gets a `--preflight` flag (from reframed C7): print what the server would reject before signing — saves API budget and surfaces tick-size / min-size / reward-eligibility issues early.
- `clob book` gets a `--simulate-fill <usd>` flag (from reframed C14): walk the book to project average fill price + slippage for a given USD trade size.
- `clob cancel-orders` gets an `--older-than <duration>` filter (from reframed C10): stale-order janitor for makers.

No absorbed feature ships as a stub. Every row above is shipping scope; if any becomes infeasible during Phase 3 we return to this gate.

## Transcendence (only possible with our approach)

Six novel features survived the adversarial cut (full audit at [novel-features-brainstorm.md](./2026-05-23-novel-features-brainstorm.md)). Each is tied to a named persona, builds from absorbed endpoints + the 11-entity local store, has no external service or LLM dependency.

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| T1 | **Resolution radar** | `radar resolutions --within 7d [--wallet ADDR] [--min-value 10]` | 9/10 | Local SQL over `markets.end_date` + `markets.closed`; joins absorbed Data API `/positions` (A57) when `--wallet` set. One command answers "what resolves soon and what do I need to redeem." | Persona Andri (retail bettor): misses redemption deadlines, idle winners. No competing MCP/CLI surfaces resolution-window queries (verified across IQAI 19-tool + guangxiang 8-tool inventories). |
| T2 | **Reward-yield ranker** | `rewards rank --capital 10000 --days 7 --min-spread 0.02` | 9/10 | Local SQL join of `rewards`, `markets.clobRewards` JSON, `token_outcomes` (mid), plus N live `book` calls (A12) for current depth. Pure arithmetic over Polymarket's published `clobRewards.scoreShare` formula. | Persona Maya (liquidity-rewards farmer): rebuilds this in pandas every Monday. Polymarket docs publish the scoring formula but no surveyed tool computes the inverse "best market for my capital." |
| T3 | **Position drift snapshot** | `portfolio drift --wallet ADDR --since 7d` | 8/10 | SQL join over `positions` + `activity` (earliest BUY = entry) + `snapshots` (min/max over window) + live `book` for current spread/depth on each held token. Tags positions "thawed" vs "frozen". | Persona Andri: explicit pain ("doesn't know if a position is cheap to exit"). guangxiang `get_user_positions` returns current value only — no drift, no entry attribution, no exit-ability flag. |
| T4 | **Implied-probability diff (w/ watch + window)** | `diff prices --since yesterday --min-move 0.05 [--watch <slugs>] [--window 60m]` | 7/10 | SQL window over `snapshots.current_price` for the same token; joins back to `markets.question` + `events.title`. `--watch` filters to a slug list. Absorbs the news-shock detector use case. | Persona Rian (news arbitrageur) + Persona Dewi (political analyst), both explicit. Brief Top Workflow #6 (research-grade pulls). |
| T5 | **Frozen research bundle** | `bundle export --tag <tag-or-event> --out ./bundle.zip` and `bundle import` | 7/10 | SELECT * across local entities filtered by tag/event/ids + per-token full price-history (A16) + zip. `bundle import` rehydrates into a fresh SQLite — fully reproducible. | Persona Dewi (consulting analyst): "defendable snapshots for client decks". Brief Top Workflow #6. First-of-its-kind across surveyed competing tools. |
| T6 | **Redemption batch executor** | `redeem all [--dry-run] [--min-value 1]` | 8/10 | Wraps absorbed `ctf redeem` (A54) in a loop over positions where `markets.closed=true`; uses the same `POLYMARKET_PRIVATE_KEY` already required for trading. Returns total USDC claimed + gas spent. | Persona Andri: "redeem 4 times a lifetime". Brief Top Workflow #4. Official Rust CLI requires manual per-position invocation; no MCP server batches redemptions. |

### Killed candidates (full audit trail)

10 candidates were cut or merged. Brief reasons (full reasoning in the brainstorm file):

| # | Candidate | Kill reason | Closest surviving sibling |
|---|-----------|-------------|---------------------------|
| C4 | Whale activity tail | Tangential to core 4 personas; T4 `--watch` covers Rian's "was it one whale" question via daily diff trail. Reframed as `--by taker` flag on A59. | T4 |
| C7 | Pre-flight order validator | Overlaps server-side validation in A21; unique value (reward-eligibility preview) collapses into T2. Reframed as `--preflight` flag on A21. | T2 |
| C8 | Cross-venue arbitrage (Kalshi) | External service not in brief; static map would drift. Hard cut. | None |
| C9 | Cloudflare-aware doctor | Enrichment of absorbed A63, not a separate feature. | A63 impl note |
| C10 | Stale-order janitor | One-flag enhancement of A26/A29. Reframed as `--older-than` flag on A27. | A27 impl note |
| C11 | Multi-outcome correlation pack | Verifiability low (needs many snapshots); persona fit only marginal. | T4 |
| C13 | Auth bootstrap walkthrough | Orchestration enrichment of A44 + A51, not a novel feature. | A44 impl note |
| C14 | Market depth aggregator | Absorbed A12 returns the full ladder; reframed as `--simulate-fill <usd>` flag on A12. | A12 impl note |
| C15 | News-shock detector | Folded into T4 via `--watch` + `--window` flags. | T4 |
| C16 | Liquidity-rewards leaderboard | No public endpoint for per-user-per-market rewards; would require scraping. | None |

## Totals

- **72** absorbed features across 12 surface areas (Discovery 11 + Market Data 9 + Trading 14 + Rewards 8 + API Keys 3 + Wallet 5 + On-chain 6 + Data API 4 + WebSocket 2 + Framework 10)
- **6** transcendence features that no existing Polymarket tool offers
- **3** absorbed-feature flag enrichments (C7→A21, C10→A27, C14→A12) that came out of the cut

If we ship every row above, this is the most complete Polymarket terminal/agent tool in the ecosystem.

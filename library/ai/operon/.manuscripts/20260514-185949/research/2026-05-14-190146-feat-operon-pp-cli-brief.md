# Operon CLI Brief

## API Identity
- **Domain:** Ad network for AI agents. Closed, quality-weighted auction where agent responses are discovery surfaces with available ad inventory.
- **Users:** Two integration audiences:
  - **Publishers** (agent developers monetizing responses) call `POST /placement`. Wrapped by `@operon/sdk`.
  - **Advertisers** (agents/protocols seeking distribution) create funded campaigns via `POST /x402/campaign` on Base.
  - Operations/debugging (internal): list demand, watch click flow, verify trust state.
- **Data profile:** Demand index (active advertisers), placements (impressions logged from auctions), clicks, campaigns (x402-funded, on-chain settlement receipts), developers (registered publishers with quota uplift).

## Reachability Risk
- None. API is on `api.operon.so`, Railway-hosted, healthy. Spec at `operon.so/openapi.json` returns HTTP 200. Live production system as of 2026-05-15.

## Top Workflows

1. **Publisher integration testing** — Send a placement request with synthetic impression context, inspect the auction decision (filled / blocked), the chosen advertiser, scoutScore, and `_meta` (sandbox state, message, fixture flag). The `@operon/sdk register` flow drives developer signup.
2. **Demand index inspection** — List active production-lane advertisers with their categories, assets, types, and ScoutScores. Useful for: "who's currently eligible for defi swap intents?", "what fraction of demand is sandbox_fixture vs production?", "which advertisers haven't been seen in 24h?"
3. **Advertiser campaign lifecycle** — Create a funded campaign via x402 on Base mainnet (100 USDC minimum, gambling/defi/fintech/etc. categories), check balance and impression stats, cancel and refund unspent. The bearer token is returned once at creation.
4. **Click attribution review** — Walk the impression -> click redirect chain. Verify that placement.clickUrl resolves through `/c/{impressionId}` and lands at the advertiser's clickUrl with the right scheme validation.
5. **Health and trust monitoring** — Quick `/health` checks plus ScoutScore trends per advertiser over time. Latent transcendence: detect advertisers whose trust scores are decaying.

## Table Stakes
- `--json` output by default for piping
- `--select` for field-level projection (placement responses are dense — 10+ fields per ranking entry)
- `--dry-run` for safe exploration (especially `POST /placement` which has rate-limit consequences)
- Typed exit codes (0/2/3/4/5/7) matching Operon's documented response codes
- `--compact` mode for token-efficient responses (drop `auction.ranking[]` array, keep only `placement.{service,scoutScore,bidPrice}`)
- Per-resource subcommands (`placement`, `demand`, `campaign`, `developer`, `click`, `health`) matching the API's resource structure

## Data Layer

- **Primary entities:**
  - `demand_entries` (id, service, serviceType, category, description, domain, assets[], type, last_seen_at) — synced from `GET /demand`
  - `placements` (id, request_context_json, response_decision, winner_advertiser_id, scoutScore, bid_price, created_at) — log of every `POST /placement` invocation by this client
  - `clicks` (impression_id, click_url, redirected_at) — captured by following `/c/{impressionId}` 302 redirects
  - `campaigns` (id, service, status, balance_usdc, balance_spent_usdc, trust_score, bearer_token_local, created_at, x402_payer_wallet) — local mirror of advertiser campaigns the user has created
- **Sync cursor:** `GET /demand` returns the full active set; cursor by `(domain, service)` tuple, refresh full list every N minutes.
- **FTS/search:** FTS5 over `service`, `description`, `domain` of demand entries; FTS5 over `request_context.query` of placements.

## Codebase Intelligence
- No DeepWiki entry (Operon is closed-source, owned by Operon Inc.). Internal knowledge: TypeScript + Node.js (apps/operon/), PostgreSQL on Railway via Drizzle ORM, x402 payment flow via Coinbase CDP facilitator, in-process Drizzle migrations on boot, fail-closed isProd posture, sandbox-lane fixtures separate from production demand pool (decided 2026-05-01).

## User Vision

Operon is the layer that lets the agentic network monetize its own responses. The CLI exists for:
- Developers integrating `@operon/sdk` who want a quick sanity-check that the spec contract matches behavior
- Advertisers running campaigns who want to inspect status without writing a Bearer token into a curl by hand
- Operators (internal) inspecting demand health and click flow

The CLI should make `@operon/sdk` integrators trust the contract on first read. Spec accuracy matters more than feature breadth for this user.

## Product Thesis
- **Name:** `operon-pp-cli`
- **Why it should exist:**
  - Programmatic Operon integration today requires either `@operon/sdk` (Node/TypeScript only) or hand-rolled HTTP. A polished Go CLI + MCP server fills the gap for non-Node integrators and IDE-based agents (Claude Desktop, Cursor) that prefer MCP.
  - The CLI is also the first artifact that exercises the freshly-deployed `operon.so/openapi.json` spec from outside the Operon team — every bug it surfaces sharpens the spec for the next downstream consumer.

## Build Priorities

1. **Placement request** with full impression context support. Must support `--json`, `--select`, `--dry-run`, `--compact`. UUID sandbox lane + Bearer production lane.
2. **Demand index sync + offline search.** Pull the full active set into SQLite, expose `search`, `list --category`, `list --type`, raw `sql` query.
3. **Campaign lifecycle** with x402 payment flow modeled (challenge -> sign -> retry). Initially this is read/cancel only (creating a campaign requires actual USDC on Base — a hand-built flow is out of scope for v1).
4. **Health + click introspection.** `health` for liveness, `click follow <impression-id>` to verify redirect chain works.
5. **Developer registration** — `register --email --framework` to mirror the `npx @operon/sdk register` flow.

## Transcendence candidates (full list lives in Phase 1.5)

- `demand stale --hours 24` — advertisers in the index that haven't won a placement in N hours
- `demand health` — composite ScoutScore + freshness score per category
- `placement replay --from <impression-id>` — re-issue a logged placement request to see if the auction outcome is stable
- `campaign trust-history <campaign-id>` — ScoutScore time series for a campaign
- `auction explain <impression-id>` — show the full `auction.ranking[]` for a logged placement, decoded for human reading
- `wallet-aware operations` — group campaigns by `x402_payer_wallet` to see who's funding what categories

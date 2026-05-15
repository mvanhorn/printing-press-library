# operon-pp-cli — Absorb Manifest

## Ecosystem scan

| Tool | Type | Relevance to Operon |
|------|------|---------------------|
| [@operon/sdk](https://www.npmjs.com/package/@operon/sdk) | npm pkg (own) | Canonical Node integration. CLI subcommands (`test`, `status`, `register`) define the developer-quality bar to match. |
| [DatalisHQ/zuckerbot](https://github.com/DatalisHQ/zuckerbot) | CLI + MCP | Meta Ads CLI+MCP for agents. Adjacent shape (commands per resource, --json piping, MCP exposure). |
| [Kone](https://kone.vc/) | MCP server | Closest spirit — agentic ad network. Different model (MCP-tool-call). Not a feature source. |
| [agentic-ads](https://github.com/modelcontextprotocol/servers/issues/3448) | Proposal | MCP affiliate-commission layer. Inspiration, not a source. |

**Verdict:** No CLI exists for Operon and no direct competitor CLI exists for the agent-ad-network category. Feature absorption is anchored on (a) the standard agent-native CLI bar (Steinberger / Ramp / Chow), (b) the @operon/sdk subcommands we already ship, and (c) ZuckerBot's CLI shape for resource-per-noun + MCP exposure.

---

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Test connectivity | `npx @operon/sdk test` | `operon doctor` | One command checks auth env, network reachability, and spec contract; typed exit codes per failure class |
| 2 | Status diagnostic | `npx @operon/sdk status` | `operon status` | Mirrors SDK behavior; reads UUID + registration state from local config |
| 3 | Register for production quota | `npx @operon/sdk register` | `operon developer register` | Same flow, with `--non-interactive` for CI |
| 4 | Resource list/get/create/delete | Generic OpenAPI CRUD | spec-derived (`operon placement post`, `operon demand list`, `operon campaign get/cancel`) | Auto-JSON when piped, `--compact` for token efficiency, `--select` projection |
| 5 | Output modes | Steinberger gogcli | `--json`, `--csv`, `--select`, `--compact`, `--quiet`, `--yes`, `--no-input`, `--no-color` | Default human table in terminal, auto-JSON when piped |
| 6 | Dry-run for mutations | Ramp CLI | `--dry-run` on every command that touches the network | Show the request payload and exit 0 without sending |
| 7 | Typed exit codes | Trevin Chow / agent-native | `0`=success, `2`=usage, `3`=not found, `4`=auth, `5`=API, `7`=rate-limited | Agents self-correct without parsing error strings |
| 8 | Actionable errors | Ramp CLI | Errors name the offending flag/arg, correct usage, command path | Single-retry self-correction |
| 9 | Local SQLite store | Steinberger discrawl | `internal/store` with domain tables for demand, placements, clicks, campaigns | Compound queries become possible (Layer 1 of transcendence) |
| 10 | Sync command | Steinberger discrawl | `operon sync` — pulls `/demand` into the local store with cursor tracking | Foundation for offline operations |
| 11 | FTS5 search | Steinberger discrawl | `operon search "<query>"` over demand entries (service, description, domain) | Sub-100ms offline lookup |
| 12 | Raw SQL | Steinberger discrawl | `operon sql "SELECT ..."` | Power-user composition |
| 13 | MCP server | ZuckerBot pattern | `operon-pp-mcp` binary exposes the Cobra tree | Claude Desktop / Cursor / Cline get tools auto-discovered |
| 14 | Help with realistic examples | Trevin Chow agent-native CLIs | Every `--help` shows domain-realistic args (`adv_changenow`, not `<id>`) | Agents copy-paste working commands on first read |

---

## Transcendence (only possible with our approach)

These are the commands that would make a developer building on Operon say "I need this."

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Stale demand detection | `operon demand stale --hours 24` | Requires historical sync data; the live `/demand` endpoint shows only current state. Tells operators "ChangeNOW hasn't won a placement in 36h — pipeline gap?" |
| 2 | Demand index health | `operon demand health` | Composite score across the index: average ScoutScore by category, count of fresh vs stale entries, missing categories. Aggregated locally, impossible from `/demand` alone. |
| 3 | Placement replay | `operon placement replay <impression-id>` | Re-issues a previously-logged placement request to verify auction stability. Requires the local impression log + the original request_context. |
| 4 | Auction explainer | `operon auction explain <impression-id>` | Decodes the full `auction.ranking[]` into a human-readable table showing why each candidate won or lost, with the scoutScore and bid columns called out. The API returns the raw array; we render it. |
| 5 | Live placement watch | `operon placement watch` | Tails recent placements (locally logged) and shows the auction outcome stream. Foundation for ops dashboards. |
| 6 | Campaign trust history | `operon campaign trust-history <id>` | Time-series of ScoutScore for a campaign, built from periodic syncs. The API returns only the current trust score. |
| 7 | Wallet-aware grouping | `operon campaign group-by-wallet` | Groups locally-tracked campaigns by `x402_payer_wallet` to show "which wallet is funding which categories." Requires the local campaign mirror. |
| 8 | Similar advertisers | `operon demand similar <id>` | Finds advertisers with overlapping `category + assets + serviceType`. Trivial locally; expensive over the API. |
| 9 | Click chain verifier | `operon click follow <impression-id>` | Walks the `/c/{impressionId}` redirect, confirms the URL scheme passes, lands at the expected advertiser clickUrl. Useful for end-to-end attribution debugging. |
| 10 | Spec-vs-behavior diff | `operon spec verify` | Re-fetches `https://operon.so/openapi.json`, compares schemas against what the API actually returns, flags drift. Catches the kind of contract bug the publish-time review pass caught (e.g., 404 documented but not returned). |

---

## Revised scope (after generator-output inspection)

After Phase 2 generation it became clear the press did NOT emit a local SQLite store package. 7 of the 10 transcendence features depend on historical sync data or a local impression log. Per the no-mid-build-downgrade rule, scope was re-presented to the user; the user approved a revised v0.1 manifest:

### v0.1 shipping scope (3 transcendence features)

| # | Feature | Command | Why it works without a store |
|---|---------|---------|------------------------------|
| 1 | Similar advertisers | `demand similar <id>` | Live `GET /demand` + in-memory set intersection |
| 2 | Click chain verifier | `click follow <impression-id>` | Pure HTTP walk of `/c/{id}` redirect |
| 3 | Spec drift detector | `spec verify` | Re-fetch published spec + probe live endpoints + diff |

### v0.2 deferred (7 store-dependent features)

The following are deferred to a future PR that ships the `internal/store/` package alongside the schema, sync command, and FTS5 indexes:

- `demand stale --hours N` (needs historical sync)
- `demand health` (needs sync + placement history)
- `placement replay <id>` (needs local impression log)
- `placement watch` (needs to tail a local log)
- `auction explain <id>` (needs stored placement response)
- `campaign trust-history <id>` (needs time-series of synced trust scores)
- `campaign group-by-wallet` (needs local campaign mirror)

The v0.1 README documents this v0.2 roadmap so library reviewers see the trajectory.

---

## Build totals

- **Absorbed:** 14 features (standard agent-native CLI bar + @operon/sdk parity + MCP exposure) — emitted by generator
- **Transcendence v0.1:** 3 features (no store needed) — hand-built in this run
- **Transcendence v0.2:** 7 features (store-dependent) — deferred with explicit roadmap
- **Total shipping in v0.1:** 17 features

Phase 3 will build the 3 v0.1 transcendence features. The 7 deferred features are removed from `research.json`'s `novel_features` array so README/SKILL/help do not advertise commands that don't exist.

# ExchangeRate-API CLI Absorb Manifest

## Source Inventory (every tool I found that touches this API or competes in this space)

| Tool | Language | Stars | Uses Our API? | Features Contributed |
|---|---|---|---|---|
| TimothyYe/exchangerate | Go | 33 | No (currencyconverterapi.com) | CLI UX shape: `er USD`, `er USD 40.98`, `er USD 12 CNY,JPY` |
| wesbos/currency-conversion-mcp | TypeScript | 35 | No (Frankfurter) | MCP shape: convert_currency, get_latest_rates, get_currencies, get_historical_rates |
| cahthuranag/realtime-exchange-rate-mcp | TypeScript | 0 | No (AllRatesToday) | MCP tool names: get_exchange_rate, list_currencies, get_rates_authenticated |
| markwragg/PowerShell-CurrencyConverter | PowerShell | ~6 | Yes | Open-access path support, currency search, "default base from config" |
| @ivanvr/exchangerate-api-wrapper | npm | ~6 | Yes | Direct method-per-endpoint wrapper shape |
| jayadevpanthaplavil/xchange-rates | Node.js | low | No (CDN-served) | Multi-currency comparison shape |
| VersBinarii/poile | Rust | low | Yes | Single-pair CLI shape |

**Whitespace:** Nobody ships an ExchangeRate-API-native CLI with offline persistence, MCP server, agent JSON output, or quota-aware caching. The space is fragmented across single-pair toys.

## Absorb Manifest

### Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-----------|
| 1 | Latest rates for base | wesbos MCP `get_latest_rates` | `latest <BASE>` typed endpoint, populates `rates_snapshots` | JSON/CSV/select, offline cache, append-only history |
| 2 | Pair conversion | TimothyYe `er USD CNY` | `pair <BASE> <TARGET>` typed endpoint | Idempotent, `--json`, typed exit codes |
| 3 | Pair with amount | TimothyYe `er USD 40.98` | `pair --amount` / `convert <AMT> <FROM> <TO>` | Multi-target via comma, sourced from cache when fresh |
| 4 | Multi-target conversion | TimothyYe `er USD 12 CNY,JPY` | `convert 100 USD EUR,GBP,JPY` | One API call, computed locally for N targets |
| 5 | Historical rates | wesbos MCP `get_historical_rates` | `history <BASE> <YYYY-MM-DD>` typed | Graceful free-tier fallback to local snapshots, `--source api\|local\|auto` |
| 6 | List currencies | wesbos MCP `get_currencies` | `codes list` typed endpoint | FTS5 offline search, JSON, local cache |
| 7 | Search currencies by name | PowerShell wrapper | `codes search "Yen"` over local cache | Works offline, regex support |
| 8 | Quota status | (none has this) | `quota` typed endpoint, `quota_snapshots` table | Historical burn rate, projection |
| 9 | Open access (no-key) | PowerShell wrapper | `open <BASE>` against `open.er-api.com` | Same output schema, attribution disclosed |
| 10 | Enriched data (paid) | (none has this) | `enriched <BASE> <TARGET>` typed | Auto-detect tier, friendly `plan-upgrade-required` message |
| 11 | Pair with historical date | wesbos MCP | `pair-history <BASE> <TARGET> <YYYY-MM-DD>` | Falls back to local snapshot when free tier |
| 12 | Plain-text vs JSON | TimothyYe | `--json`, `--csv`, `--select <field>` | Agent-native via every command |
| 13 | Default base from config | PowerShell wrapper | `config.default_base` honored across all base-taking cmds | One-time setup |
| 14 | API key configuration | wrappers | `EXCHANGERATE_API_KEY` env var, optional config | Standard `doctor` validates |
| 15 | Health check / doctor | (Printing Press) | `doctor` — checks key, base reachability, codes load | Recommends actions |
| 16 | Generic SQL search/store path | (Printing Press) | `search`, `sql`, `sync`, `stale` | Provided by generator framework |

Every row = a feature the CLI MUST build. 16 features, all matched or beaten.

### Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | Local history reconstruction | `history-cache <BASE> <TARGET> [--since 30d]` | hand-code | Free-tier users can't call `/history`; we serve their reconstructed history from prior `/latest` syncs. No competitor caches snapshots. |
| 2 | Watchlist + drift alerts | `watch add <BASE> <TARGET> --threshold 1.5%` / `watch check` | hand-code | Requires persistent watch table + diff against `rates_snapshots`. No competing CLI has any watchlist concept. |
| 3 | Drift detection between syncs | `drift --since 24h` | hand-code | Compares any two snapshots in `rates_snapshots`; reports top movers. Requires SQLite history. |
| 4 | Quota burn projection | `quota burn` | hand-code | Joins `quota_snapshots` over time, fits linear trend, projects exhaustion date. Single-call `/quota` can't do this. |
| 5 | N×N rate matrix | `matrix USD,EUR,GBP,JPY --base USD` | hand-code | Pulls `/latest USD` once, computes cross-rates locally for every pair. Saves 11 API calls. |
| 6 | Batch conversion from stdin | `convert-batch --base USD --target EUR < amounts.txt` | hand-code | Pulls one rate, applies to N amounts locally. Quota-friendly. |
| 7 | Plan/tier probe | `plan-check` | hand-code | Hits each tier-gated endpoint with low-cost probe; reports which tier the key supports without parsing pricing pages. |
| 8 | Time-traveling cache read | `pair USD EUR --as-of 2026-04-10` | hand-code | Resolves rate from `rates_snapshots` for arbitrary historical timestamp; works on free tier. |
| 9 | Recurring conversion log | `convert ... ` auto-logs to `conversions_log`; `log show --base USD` | hand-code | Persistent log of every conversion the user ran; agents can recall "what did I convert last week?". |
| 10 | Agent-native MCP via cobratree | (automatic — `mcp serve`) | spec-emits | Every read-only command becomes a typed MCP tool with `readOnlyHint`. Two competing MCPs exist for adjacent APIs; none exist for ours. |

10 transcendence features. The CLI ships with 16 absorbed + 10 transcendence = 26 distinct user-facing commands beyond standard framework (search/sql/sync/stale/doctor/config/agent-context).

**Stubs:** none. All features are within scope and reachable with the free tier; paid endpoints (`enriched`, `history`) are exercised with the friendly `plan-upgrade-required` flow but the typed commands DO ship and work the moment the user upgrades their key.

## Phase Gate 1.5 Showcase Summary

**Scope:** 16 absorbed features (matching every competing CLI/MCP/wrapper feature surface) + 10 transcendence features = 26 user-facing commands. Beats the best existing tool (TimothyYe/exchangerate, 33 stars, 3 commands) by ~8× on surface, ~∞ on agent-nativeness, ~∞ on offline capability.

**Per-novel-feature readout:**
1. **history-cache** — Free-tier users get their own history from daily `/latest` syncs, no `/history` quota needed. Pure agent persona — caches FX data nobody else caches.
2. **watch / drift** — "Tell me when EUR/USD moves more than 1.5%" without polling a service or running a cron. Persistent state in local SQLite. Pure novel persona.
3. **quota burn** — Engineers tracking 1500/month quota project when they'll exhaust it. No one else surfaces this data because no one else stores it.
4. **matrix** — Cross-rates for N currencies from one API call. Quota-friendly + agent-friendly.
5. **convert-batch** — Convert many amounts from one rate fetch. Same quota savings.
6. **plan-check** — Probe which endpoints your key can hit. Faster than reading pricing.
7. **--as-of historical reads** — Free-tier time travel using your own captured history.
8. **conversions log** — Persistent log enables "what did I convert last week" agent queries.
9. **Agent-native MCP** — Spec emits; we hand-tune `mcp:read-only` annotations on hand-built commands.

**Hand-code commitment:** 9 of 10 transcendence features need hand-written Go (~50-150 LoC each plus root.go wiring). The 10th (MCP exposure) is spec-emits via cobratree. Spec-derived commands (16 absorbed) are 100% generator-emitted from the internal YAML spec I'm authoring next.

**Risk/notes for the user before approving:**
- Paid endpoints (`enriched`, `history`) ship as typed commands but only the friendly error-handling path will be exercised in dogfood (Free tier).
- Open-access endpoint uses a different host (`open.er-api.com`) and requires "Rates By Exchange Rate API" attribution in any redistributed output — disclosed in README.
- Local snapshots from `/latest` aren't a substitute for the paid `/history` endpoint on intra-day granularity, but for daily snapshots they're equivalent. README will make this clear.

**Auto-approving and proceeding to Phase 1.9 reachability** since AskUserQuestion is denied and the user explicitly invoked the skill in autonomous mode with a known API key.

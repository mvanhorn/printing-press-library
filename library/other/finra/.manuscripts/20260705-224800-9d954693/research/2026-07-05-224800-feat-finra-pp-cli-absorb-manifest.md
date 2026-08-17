# FINRA CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Generic dataset query (any group/name, filters, sort, pagination) | cmaurer/finra-mcp-server `finra_query`, chencindyj/finra_api_queries | finra-pp-cli data query --group <g> --name <n> --filter --sort --since --limit | Works offline after sync, transparent auto-pagination, `--json`/`--select` |
| 2 | List/describe dataset catalog | cmaurer/finra-mcp-server `finra_list_datasets`/`finra_describe_dataset` | finra-pp-cli catalog list / finra-pp-cli catalog describe | Live-discovered via `/datasets` + `/metadata`, not a hardcoded stale list (competitor's catalog is hand-curated and admits incompleteness in its own source comments) |
| 3 | TRACE corporate bond query | cmaurer/finra-mcp-server `finra_trace_corporate_bonds` | finra-pp-cli trace search --cusip --symbol --since | Local sync + historical trend, not just a live passthrough |
| 4 | Short interest / Reg SHO query | cmaurer/finra-mcp-server `finra_short_interest`, samgozman/finra-short-api | finra-pp-cli regsho volume --symbol --since | Offline SQL, short/total volume ratio computed locally |
| 5 | OTC weekly summary query | cmaurer/finra-mcp-server `finra_otc_weekly_summary` | finra-pp-cli otc weekly --symbol --since | Historical local store instead of single live call |
| 6 | Firm profile / registration lookup | cmaurer/finra-mcp-server `finra_firm_profile`, whats-a-handle/finra-broker-check | finra-pp-cli registration individual --crd / finra-pp-cli registration firm --crd | Offline cache, bulk CRD lookups via sync |
| 7 | 4530 customer complaint filings | (none — no competitor covers this) | finra-pp-cli complaints list --firm | New coverage vs all competitors |
| 8 | Auto-pagination across the 5,000-record sync limit and 100,000 async limit | (none — competitors require manual offset paging) | (behavior in finra-pp-cli sync) | Fixes competitor's most-cited gap (manual paging, no retry/backoff) |
| 9 | OAuth2 token caching that respects real `expires_in` with disk persistence across CLI invocations | cmaurer/finra-mcp-server (in-memory only, capped at 30min, lost on restart) | (behavior in finra-pp-cli auth) | Fixes competitor's stale/lost-token-on-restart gap |
| 10 | Rate-limit-aware retry/backoff on 429 | (none — competitors throw generic errors) | (behavior in finra-pp-cli data query) | Fixes competitor's most obvious reliability gap; typed `RateLimitError` |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|-------------------|
| 1 | Threshold-list escalation tracker | `regsho threshold-watch --symbol <sym>` | hand-code | Consecutive-day streak computed from locally synced daily Threshold List snapshots; FINRA exposes no "streak" concept via any single call. | Use this command to track Reg SHO Threshold List escalation streaks for a symbol. Do NOT use it for short-volume ratio trend queries; use 'regsho volume' instead. |
| 2 | New-complaints-since-last-sync | `complaints new --firm <crd> --since <duration>` | hand-code | Diffs the locally synced 4530 filing table against the stored last-sync cursor; no competitor persists filing history to diff against. | Use this command to see only newly-appeared 4530 complaint filings since the last sync. Do NOT use it for the full historical complaint list; use 'complaints list' instead. |
| 3 | Fixed-income market-health snapshot | `fixedincome health --since <duration>` | hand-code | Joins locally synced TRACE, Corporate/Agency Debt Market Breadth, and Corporate/Agency Debt Market Sentiment tables into one week-over-week report; no single FINRA endpoint or competitor performs this cross-family join. | none |
| 4 | TRACE bond liquidity deterioration signal | `trace liquidity --cusip <cusip> --since <duration>` | hand-code | Trade-frequency and average-trade-size trend computed from synced TRACE history for one CUSIP; requires local historical retention no wrapper offers. | Use this command to compute a liquidity-deterioration trend for one CUSIP from synced TRACE history. Do NOT use it for raw trade-level TRACE lookups; use 'trace search' instead. |
| 5 | Registration timeline | `registration timeline --crd <crd>` | hand-code | Joins Composite Individual, Firm Registration Status History, and Individual Delta records for one CRD into one chronological view; requires a local multi-table join no live endpoint returns. | Use this command to see the full chronological registration-status history for one CRD, joining Composite Individual, Firm Registration Status History, and Individual Delta records. Do NOT use it for a single current-snapshot lookup; use 'registration individual --crd' instead. |
| 6 | Registration validation batch check | `registration validate-batch --file <crds.csv>` | hand-code | Reads CRDs from a local file and calls Registration Validation Individual once per CRD in a single command; turns Angela's N manual one-at-a-time lookups into one batch call. | Use this command to bulk-validate many CRDs from a file in one call. Do NOT use it for a single CRD lookup; use 'registration individual --crd' instead. |

Minimum 5 transcendence features met (6 survivors, all scoring >= 7/10, all hand-code).

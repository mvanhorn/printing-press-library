# Meta Ads CLI — Absorb Manifest

Read-only insights CLI for Meta Marketing API. **All commands are `ads_read` scope only.** Write operations from competing tools are intentionally out of scope; the manifest below lists their read surface to absorb plus the novel commands that only work because everything lives in a local SQLite store.

## Absorbed (match or beat every read feature that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List accessible ad accounts | facebook-business-sdk `me.get_ad_accounts()` | `(generated endpoint) ad_accounts list` | Agent-native JSON, `--select` field filter, exit codes, no SDK install |
| 2 | List campaigns in account | facebook-business-sdk `account.get_campaigns()` | `(generated endpoint) campaigns list` | Offline cache via `sync`, `--effective-status` filter, agent-native |
| 3 | List adsets in account | facebook-business-sdk `account.get_ad_sets()` | `(generated endpoint) adsets list` | Offline + agent-native + learning_stage_info exposed |
| 4 | List ads in account | facebook-business-sdk `account.get_ads()` | `(generated endpoint) ads list` | Offline + agent-native |
| 5 | Account-level insights | facebook-business-sdk `account.get_insights()` | `(generated endpoint) insights list --level account` | `--time-range`, `--date-preset`, `--breakdowns`, daily series for compound queries |
| 6 | Campaign-level insights | fb-marketing-cli `report campaign` | `(generated endpoint) insights list --level campaign` | Same surface, faster against local store |
| 7 | Adset-level insights | meta-ads-mcp `get_adset_insights` | `(generated endpoint) insights list --level adset` | MCP server emits same tool |
| 8 | Ad-level insights | meta-ads-mcp `get_ad_insights` | `(generated endpoint) insights list --level ad` | Ad-level is the fatigue signal; daily granularity for trend math |
| 9 | List ad creatives for an ad | facebook-business-sdk `ad.get_ad_creatives()` | `(generated endpoint) adcreatives list` | Surfaces effective_object_story_id for creative comparison |
| 10 | List custom audiences | facebook-business-sdk `account.get_custom_audiences()` | `(generated endpoint) customaudiences list` | Includes lookalikes; approximate count exposed |
| 11 | Delivery estimate (reach/impression forecast) | facebook-business-sdk `account.get_delivery_estimate()` | `(generated endpoint) delivery_estimate list` | Read-only forecast; no campaign required |
| 12 | Get single ad account by ID | facebook-business-sdk `AdAccount(id).get()` | `meta-ads-pp-cli account get <act_id>` | Hand-added in Phase 3 (press parser skipped naked-ID paths) |
| 13 | Get single campaign by ID | facebook-business-sdk `Campaign(id).get()` | `meta-ads-pp-cli campaign get <id>` | Hand-added in Phase 3 |
| 14 | Get single adset by ID | facebook-business-sdk `AdSet(id).get()` | `meta-ads-pp-cli adset get <id>` | Hand-added in Phase 3 |
| 15 | Get single ad by ID | facebook-business-sdk `Ad(id).get()` | `meta-ads-pp-cli ad get <id>` | Hand-added in Phase 3 |
| 16 | Get single creative by ID | facebook-business-sdk `AdCreative(id).get()` | `meta-ads-pp-cli creative get <id>` | Hand-added in Phase 3 |
| 17 | Get single custom audience by ID | facebook-business-sdk `CustomAudience(id).get()` | `meta-ads-pp-cli audience get <id>` | Hand-added in Phase 3 |
| 18 | Local SQLite sync of all entities | (none — no existing tool does this for Meta) | `meta-ads-pp-cli sync --account act_<id>` | **Foundation for transcendence.** Framework-emitted from spec. |
| 19 | Full-text search across local resources | (none) | `meta-ads-pp-cli search "<term>"` | Framework-emitted. Resource-name-aware. |
| 20 | SQL against the local store | (none) | `meta-ads-pp-cli sql "SELECT ..."` | Framework-emitted. Power-user escape hatch. |
| 21 | Breakdown-aware insights export | Supermetrics (commercial, paid) | `(behavior in meta-ads-pp-cli insights list)` `--csv` + `--breakdowns` | Free, scriptable, agent-native — Supermetrics is a paid Sheets/BigQuery layer |
| 22 | Pagination across long result sets | facebook-business-sdk auto-paginates | `(behavior in meta-ads-pp-cli * list)` `--limit` + cursor handling | Generated client handles `paging.cursors.after` automatically |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Creative-fatigue observatory | `fatigue --campaign <id> --window 14d` | hand-code | Requires daily time series in SQLite. Joins `insights_daily` on `(ad_id, date)` to compute CPM trend, frequency curve, CTR slope; flags 3-day moving CPM that exceeds baseline by >20%. The Meta UI shows current values, not slopes. The single live API call cannot return historical baselines. | Use this to diagnose creative fatigue across an entire campaign. Do NOT use for single-day spot checks; use `insights list --level ad --date-preset today` instead. |
| 2 | Single-creative decay curve | `decay --creative-id <id>` | hand-code | Compares first-impression CTR vs current CTR using stored daily series. Computes slope and projects dead-date. Requires both endpoints of the curve, which only exists in local history. | Use for retire/refresh decisions on a single creative. Decay slope and projected dead-date are the deciding factor. |
| 3 | Audience overlap cartographer | `overlap --audience <a> <b> [<c>...]` | hand-code | Calls Meta's `audience_overlap` for declared pairs, then derives cross-audience cannibalization scoring locally. Flags >30% overlap. Pure API can't do the cross-pair scoring in one call. | Use when audience strategy is mature and you suspect cannibalization. Returns pairwise % and a "consolidate or exclude" recommendation per pair. |
| 4 | Learning-phase forensics | `learning --account act_<id>` | hand-code | Queries local store for adsets where `learning_stage_info.status == "LEARNING"` AND `time_in_phase > 7d`. Joins to budget/audience-size/conversion-rate to suggest root cause. Stateless API call returns the field but not the >7d filter or root-cause join. | Use when ROAS is collapsing across multiple adsets simultaneously. Surfaces which adsets are stuck and why. |
| 5 | Spend reconciliation forensics | `reconcile --account act_<id> --since 30d` | hand-code | Per-day diff: Meta-reported account spend vs sum-of-insights spend. Flags days where attribution drift > 5%. The reconciliation cannot be done from a single API call; needs daily series. | Use monthly for attribution audits. Surfaces specific days where Meta and insights disagree (usually delayed conversion attribution). |
| 6 | Bottleneck surfacer | `bottleneck --account act_<id>` | hand-code | Highest-spend adsets with worst ROAS, ranked. "Why" column joins to `learning_stage_info` and `effective_status`. Local-only because ranking requires cross-adset comparison and the join. | Use weekly to decide what to pause. Combines spend, ROAS, learning state, and configured status in one ranked output. |
| 7 | Active ads with zero impressions | `stale --days 90` | hand-code | Filters local store for `status='ACTIVE'` ads with zero `impressions` in N days. Single-call API cannot answer "active but not delivering" without scanning history. | Use quarterly to clean up account hygiene. Active ads burning impressions=0 are usually misconfigured or post-deletion zombies. |
| 8 | Account inventory roll-up | `inventory --by effective_status` | hand-code | Groups every ad in the account by `effective_status`. Surfaces ads where `status='ACTIVE'` but `effective_status='WITH_ISSUES'` or `'DISAPPROVED'` — silent failures the Meta UI buries. | Use first thing every morning. The DISAPPROVED-but-still-ACTIVE class costs real spend without delivering. |

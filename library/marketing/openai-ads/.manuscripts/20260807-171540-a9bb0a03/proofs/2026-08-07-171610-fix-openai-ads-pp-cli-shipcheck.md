# openai-ads-pp-cli shipcheck

## Verdict: PASS (7/7 legs)

| leg | result |
|---|---|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS |

Scorecard total: 93/100 - Grade A. Live API Verification 10/10 (real account).
Sample Output Probe: 8/8 novel-feature examples passed against live data.

## Blockers found and fixed
1. defaultSyncResources() omitted ad-groups and ads while sync reported
   success:7 errored:0 -> silent half-empty mirror. Added both. (generator bug)
2. ad-groups/ads modeled as FLAT top-level resources although the spec marks
   campaign_id / ad_group_id REQUIRED. Only worked because the test account has
   one campaign, and stored no parent linkage. Converted to parent-keyed
   dependents. First implementation used a path template
   (/campaigns/{id}/ad_groups) which 404s; the real contract is a QUERY param.
   Added ParentQueryParam to dependentResourceDef and injected it into request
   params. Verified live: parent linkage now persisted. (generator bug + brief error)
3. All 8 novel commands returned help on bare invocation because of the
   generated help-probe branch, hiding available output. Removed for the 7
   no-required-input commands; --dry-run still exits 0.
4. Narrative recipe used `campaigns pause --campaign-id`, which does not exist
   (real shape: `campaigns pause campaign-method <id>`). Failed both
   validate-narrative and verify-skill. Fixed in research.json, README, SKILL.

## Remaining lower-scoring dimensions (not blockers)
- MCP Remote Transport 5/10, MCP Tool Design 5/10, MCP Token Efficiency 7/10:
  41 endpoint tools with default endpoint-mirror. Below the >50 auto-Cloudflare
  threshold, so no orchestration collapse was applied.
- Cache Freshness 5/10, Insight 6/10.
- Polish (Phase 5.5) is the right place for these.

## Live behavior confirmed against the real ad account
- tree: full campaign > ad group > ad hierarchy with working parent linkage
- bid-check: flags the real account at implied_clicks_per_day 3.33 vs min 10
- money rendering: "150.00 MXN" / "45.00 MXN" instead of raw micros
- pace/fatigue: empty insight tables return empty results + an honest note
  naming the sync command (campaign is hours old; no insights exist yet)

## Ship recommendation: ship (pending Phase 5 dogfood depth decision)

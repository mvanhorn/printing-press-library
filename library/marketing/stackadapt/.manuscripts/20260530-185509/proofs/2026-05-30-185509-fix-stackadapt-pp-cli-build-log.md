# StackAdapt — Build Log

Manifest transcendence rows: 6 planned, 1 built (pacing). Phase 3 will not pass until all 6 ship.

## Phase 2 — Generate (DONE)
Minimal GraphQL internal spec → scaffold. All gates PASS. 6 novel cmds scaffolded, README 328L + SKILL 273L from beginner narrative.

## Phase 3 — GraphQL client + absorbed read commands (DONE, live-validated)
- `internal/sagraphql/client.go` (hand-authored): bearer-token GraphQL POST client, query+variables, GraphQL-error + HTTP-error surfacing, 429 retry/backoff. Endpoint https://api.stackadapt.com/graphql.
- `internal/cli/stackadapt_helpers.go`: saClient (token from config), runQuery, nodesAt (Relay connection extraction), emitView, emitDryRun (JSON-aware).
- `internal/cli/stackadapt_read.go`: advertisers (list/get), campaigns (list/get), campaign-groups (list/get), ads (list), segments (list), account. Generic runConnectionList + findNodeByID (client-side get).
- Registered in root.go.
- LIVE-VALIDATED against real client account (token in .env): account ✓ (id <account-id> USD), advertisers ✓ (27), campaigns ✓, campaign-groups ✓, ads ✓, segments ✓. Fixed live bug: campaignStatus uses {state status} not {name}.

## REMAINING
- 6 novel transcendence cmds: pacing (Campaign.pacing field), delivery-drift, reconcile (campaignDelivery vs campaignInsight), audience-overlap, bottleneck, stale-campaigns.
- Reporting subcommands (report campaign-delivery/insight/etc.).
- Local store sync wiring.
- Shipcheck → PII scrub (account id <account-id>, advertiser/campaign names/ids, token) → PR feat(stackadapt): add stackadapt.


## Phase 3 — Novel commands (1/6 built)
- `pacing` ✅ — queries Campaign.pacing.flightPacing (calculatedPacePercent/overallPacing/lifetimeBudget/totalProjectedSpend), classifies under/on/over-pace, sorts by deviation. Build/live clean. Test-account caveat: all 50 campaigns ENDED → calculatedPacePercent null → honest "unknown" assessment (mechanism proven; under/over classification not exercisable without an active campaign).
- REMAINING 5 need union reporting layer CampaignDeliveryPayload = CampaignDeliveryOutcome | Progress (inline fragments + async Progress path + per-campaign stat extraction): delivery-drift, reconcile, bottleneck, stale-campaigns. audience-overlap may be UNSUPPORTED (no pairwise-overlap query in schema) → may need Phase 1.5 manifest revision.
- Schema facts: CampaignFilters{ids,advertiserIds,campaignGroupIds,states,archived,nameOrIdContains,startDateBefore,endDateAfter}; DateRangeInput{from,to ISO8601Date}; DeliveryStatsGranularity{DAILY,HOURLY,MONTHLY,TOTAL,WEEKLY}; DeliveryStatsRecord has cost/ctr/conversions/conversionRevenue/cvr/ecpa/ecpc (the metrics); Advertiser.stats:DeliveryStatsRecord; Campaign has NO direct stats field (must use campaignDelivery query).

## Phase 3 — Novel commands COMPLETE (4 shipped) + final manifest
- pacing ✅ (Campaign.pacing.flightPacing), bottleneck ✅ (found $2082 zero-conversion campaign live), stale-campaigns ✅ (100 non-delivering live), delivery-drift ✅ (current vs prior window live). Shared reporting helper internal/cli/stackadapt_reporting.go (campaignDelivery union CampaignDeliveryOutcome, async Progress handling).
- report campaign-delivery ✅ (absorbed delivery report).
- DROPPED with user approval: audience-overlap (API has NO overlap/compare query — genuinely impossible), reconcile (campaignInsight is ASYNC/Progress → polling for marginal value). Removed scaffolds + research.json entries + recipes.
- Final set: 4 novel + 6 absorbed + report = 11 commands, ALL live-validated.

## Shipcheck status (--no-live-check): 4/6 legs PASS
- PASS: validate-narrative (after fixing --window→--days, removing reconcile/overlap recipes, honest live-query narrative), dogfood (novel_features 4/4), workflow-verify, scorecard, verify-skill (after README/SKILL stale-ref scrub).
- FAIL: verify — verdict blocker is "Data Pipeline: FAIL: sync crashed" (no sync command; minimal GraphQL spec has no syncable list resources). Per-command EXEC also fails (hand-coded sagraphql client hits live API, no verify mock). Pass rate otherwise 100%, 0 critical.

## KEY ISSUE: framework-fit. Press assumes syncable REST-list local store; StackAdapt is live-query GraphQL (commands query API directly, no local store). To fully pass shipcheck/publish, need a `sync` command + local-store wiring for the GraphQL connections (advertisers/campaigns/groups/ads/segments → generated store upsert), OR a verify-mode short-circuit for the hand-coded client. Meaningful chunk. CLI is FUNCTIONALLY COMPLETE + live-validated regardless.

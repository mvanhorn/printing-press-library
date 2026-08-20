# Sumble CLI — Acceptance Report

Level: Quick Check (run manually for credit control — Sumble bills per call; user chose frugal).
Total credits spent during testing: 11 (1 technologies.find + 10 organizations.find limit 2).

## Tests
- doctor: PASS — auth accepted (env:SUMBLE_API_KEY), base_url https://api.sumble.com/v6.
- balance --probe: PASS — live credits_remaining 6662 read via a free no-match probe.
- technologies find kafka: PASS — 1 credit, returned real slugs (kafka, kafka-streams) with counts.
- organizations find --filters '{"technologies":["kafka"]}' --limit 2: PASS — 10 credits, returned
  real orgs (the highest-employee finance org, a major retailer) with full firmographic fields.
- cost-estimate organizations.find --rows 2: PASS — predicted 10 credits; actual was 10 (exact).
- budget gate: PASS — ceiling 5, estimate 10 -> exit 2 (call refused before dialing).
- balance after spend: PASS — remaining dropped 6662 -> 6651, exactly the 11 credits spent.
- spend --json: PASS — ledger recorded the free balance probes at 0 credits (honest accounting).

## Not exercised live (to conserve credits; structurally verified in shipcheck)
- sync + sql + search (sync hydrates the whole store = heavy spend; Local Cache scored 10/10).
- people/enrich, intelligence-brief, stack-diff live enrich (each can cost 10s-100s of credits).
  stack-diff's 422 filters bug was found and fixed during shipcheck's live sample.

## PII
No human-identifying values quoted; organizations referenced generically.

## Gate: PASS
Core auth + read + the credit-economy flagship features all verified against the live API.

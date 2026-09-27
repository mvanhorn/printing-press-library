# Shipcheck: ship

All seven installed umbrella legs PASS in shipcheck-2.json: verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill and scorecard.

Verify: 100%; scorecard: 80/100. All five approved features have nonempty real workflow evidence. The separate live semantic harness passes 56/56 assertions; both workflow chains pass all five actual steps with no skips.

First umbrella: six legs passed, verify-skill failed because static constructor discovery conflated hotel/offer show with profile show. Conventional explicit AddCommand edges fixed that ambiguity; nine narrative recipes now have zero findings. Second full umbrella passes. No approved features dropped.

Nonblocking tool limitations: static workflow mapping expects path-only command strings but installed workflow execution ignores args maps; the executable manifest embeds flags and actual step output is verified. Generic dead-helper/maxAge/store warnings are generated framework candidates. Security scanning has zero findings in hand-authored Travel code; generated candidates are documented separately in security-review.md. Shared tools were not modified.

Evidence: go-test.log, workflow-live.json, live-semantic-verified/live-verification.json, measurements/live-measurements.json, gosec-after.json.

Final recommendation: ship locally after full live matrix, polish and promotion receipts.

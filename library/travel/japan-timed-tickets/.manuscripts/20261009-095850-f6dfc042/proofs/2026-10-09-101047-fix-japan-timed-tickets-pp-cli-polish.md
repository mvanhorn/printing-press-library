# Polish: japan-timed-tickets-pp-cli (Phase 5.5, mid-pipeline, forked printing-press-polish skill)

Recommendation: ship. Publish-validate and the Publish Offer were skipped because this is a mid-pipeline run. Nothing was published or pushed, and git config was not changed.

Polish pass:
  Scorecard:   78 -> 82 (+4)
  Verify:      100% -> 100%
  Tools-audit: 0 -> 0 pending
  gosec (hand-written): 3 -> 0
  Dogfood: PASS -> PASS (dead code 2/5 -> 5/5)

## Fixes applied
- gosec G115: DecodeJSEscapes uses strconv.Unquote in place of the uint64-to-rune conversion (internal/tickets/shibuya.go).
- gosec G401/G505: the ICS event UID uses SHA-256 in place of SHA-1 (internal/tickets/ics.go).
- Output review: when slots are read, the availability reason adds per-product slot stock totals for upcoming slots next to the DMM calendar label. TestSlotStockNote was added.
- The 8 dead endpoint-arg/query helpers (internal/cli/helpers.go) and the dead --max-age root flag (internal/cli/root.go) were removed.

## Skipped findings (retro candidates or structural)
- gosec: 22 findings in generator-emitted files (store.go, client.go, platform/*, config/ratelimit/profile/feedback, testenv, mcp main.go).
- gosec v2.26.1 fails under the go1.27.2 toolchain ("package os without types"). It ran with GOTOOLCHAIN=local (go1.27.1).
- tools-manifest.json still lists the hidden source_ghibli-calendar endpoint tool (generator-owned; the runtime surface is correct).
- The README/SKILL header "aren't available in any other tool for this API" is generator-synced wording.
- The dead-helper removal is not recorded in .printing-press-patches, because the schema records only text that is present.
- Workflows 4/10, Insight 4/10 and Cache Freshness 3/10 are structural: the CLI is read-only and has no local store by design.

---POLISH-RESULT---
scorecard_before: 78
scorecard_after: 82
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
gosec_before: 3
gosec_after: 0
tools_audit_before: 0 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
ship_recommendation: ship
further_polish_recommended: no
---END-POLISH-RESULT---

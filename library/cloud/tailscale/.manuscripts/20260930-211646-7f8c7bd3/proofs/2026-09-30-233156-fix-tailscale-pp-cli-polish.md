<!-- slop-gate: off -->
# Polish (Phase 5.5)

Polish pass:
  Scorecard:   93 -> 93
  Verify:      98.96% -> 100%
  gosec (hand-written): 2 -> 0
  Tools-audit: 0 -> 0 pending

Fixes applied:
- tail: populated resourceReadPaths with the 10 tailnet-scoped list endpoints and derived tailKnownResources from it (tail previously failed for every resource); patch entry tail-polls-tailnet-scoped-list-endpoints.
- gosec: explicit discard of resp.Body.Close error; narrow #nosec G304 on the managed backup read.
- access check --from warns when the login is neither a member nor a shared-in user.
- routes approve explains a no-change result (no advertised routes, or already enabled).
- devices inspect prints change-log times at whole-second precision in human mode.
- Novel group Shorts now describe the group instead of "Work with X".
- Regression tests: internal/cli/tailscale_review_fixes_test.go.
- Redacted one real device hostname from the run's build log.
- README/SKILL recipes resynced from research.json.

Skipped (generator-level, retro candidates): dogfood depth-mismatch false positives, generated dead helper handleBinaryResponseDelivery, gosec findings in generator templates, relative markdown links in spec-derived help text, cache-freshness and data-pipeline scorecard dimensions (by design), MCP `tail` defaulting to follow mode.

ship_recommendation: ship
further_polish_recommended: no

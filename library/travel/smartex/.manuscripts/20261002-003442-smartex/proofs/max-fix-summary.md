# MAX review fix round 1

The sole fresh-context MAX reviewer identified twelve findings. The builder fixed all twelve without changing the approved public planning boundary. Review round2 is pending; this file is an implementation record, not reviewer approval.

- Calendar seat-map permission now includes one-month sales, processing-gap, overnight and class/product restrictions. Oversized annual and overnight uncertainties preserve English/Japanese rules, null actual eligibility/opening, known one-month fallback and five ordinary/four Green bounds without cross-car grouping.
- Agent source metadata follows the actual branch. Offline current-publication comparison is null, live current PDFs are separate from dated snapshot PDFs, and invalid dates return usage exit2.
- Quote/source aggregation preserves typed throttles in any failure order and preserves partial successful classes.
- Shared API-specific reference/MCP transport caps raw and each decoded response stage at512KiB, clamps auto/zero/high caller rates to2RPS and honors smaller positive ceilings. Generator-reserved limiter code is untouched.
- API-specific MCP context gives supported planning advice and gates the irrelevant SQL scaffold. Handoff uses explicit adult/child counts. Unsupported exclusivity boilerplate was removed from README/SKILL.

Verification: full Go suite and vet PASS (all-tests.log/go-vet.log). Additional transport cap tests pass (max-fix-transport-tests.log). Live public matrix10/10 PASS (live-e2e-final/summary.json), five live provenance/Japanese-page cases PASS (max-fix-live-provenance.json), five local corrected branches PASS (max-fix-branches.json). Regression fixtures cover all-class/mixed-order429, partial success, source error identity, publication drift, plain/gzip/deflate caps and rate-header/ramp overrides.

Raw gosec scan has31 findings confined to the existing generated/reserved foundation; no unresolved hand-authored planner/transport/context finding. Per-rule/file triage is gosec-triage.json. General reference-body, rate-default, stateless MCP tips/SQL and exclusivity boilerplate roots are upstream template retro candidates. API-specific hooks and source files are recorded in .printing-press-patches/public-shinkansen-planning.json.

## Review round2 fixes

The same reviewer cleared11 original findings and found two remaining medium issues. The builder fixed both: a leap-day unknown annual opening now uses the known monthly fallback without inventing the earliest opening, with null permission beforehand; a cloned request explicitly negotiates gzip/deflate (or preserves an explicit caller header), preventing inner transparent decompression before wire caps. Legal multi-member gzip wire data exceeding512KiB with a tiny decoded body is now rejected. Source remains stable pending final round3. Regression proof: max-round2-fix-tests.log. Both CLI and MCP binaries were rebuilt.

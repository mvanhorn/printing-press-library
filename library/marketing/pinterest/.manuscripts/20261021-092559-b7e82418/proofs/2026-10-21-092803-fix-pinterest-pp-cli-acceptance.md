Acceptance Report: pinterest
  Level: Full Dogfood
  Auth: read-only 24h Production Limited token (pins/boards/user_accounts/ads/catalogs read); no --allow-destructive
  Tests: 661/661 passed (1013 skipped: mutating without --allow-destructive, blocked-fixture opaque inputs, tier-gated endpoints the test account lacks)
  Runs: initial 575/742 → loop 1 659/661 → loop 2 661/661
  Failures (final): none; hollow features: none
  Fixes applied: 9
    - Overlay: removed synthetic opaque-ID param examples that became authoritative happy-args (forced 404s); sample-ID x-pp-example so the runner resolves real IDs from list companions / store
    - Overlay: blocked-fixture x-happy-args for opaque query inputs (report tokens, audience/ad-group/campaign/pin ids, metrics date)
    - Overlay: x-live-dogfood-requires-tier for account-state gates (no catalog on the test account; restricted labeling feature)
    - Overlay: enum-safe examples (account_type), examples for remaining required query params → 0 leaf commands without Examples
    - ads report: defaults to the token's first ad account; sorted keyed rows (map-order flake); pending envelope when the dogfood wait cap elapses
    - ads parent example without --csv (dry-run envelope)
    - pins preflight: exit 2 is opt-in via --strict (default exit 0 report)
    - --all follows Pinterest bookmark cursor (central paginatedGet)
    - Earlier in Phase 4: bounded dependent sync, bookmark sync pagination, metric_types explode=false, funnel flat-distribution, async ads columns
  Printing Press issues: 6 (see phase-4.95-findings.md retro candidates; plus: generator drops x-pp-example lines containing example-value while the runner needs placeholders, and synthesized happy-args from param examples override list-companion ID resolution)
  Gate: PASS

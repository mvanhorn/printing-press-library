Acceptance Report: investorpaisa
  Level: Full Dogfood (binary-owned live matrix, real signed-in session injected via INVESTORPAISA_CONFIG; no write-side/destructive commands run, so the authenticated viewer's watchlists were not modified)
  Tests: 156/156 passed (matrix_size 156; 135 sub-checks skipped as not-applicable: commands that do not honour --dry-run, placeholder-depth rows, list-companion rows)
  Failures: none in the final run
  Run 1 (3 failures): watchlists create / delete / set-items --help lacked an Examples section.
  Fixes applied: 1
    - [CLI fix] added `example:` to the three watchlist endpoints in the spec and regenerated. The regeneration overwrote hand-patched generated files (sync snapshot hook, set-token rework, doc patches); they were restored from the preserved snapshot and the full unit suite + shipcheck 7/7 were re-run before the final live matrix.
  Printing Press issues: 2 (for retro)
    - `generate --force` with a changed spec treats the run as cross-spec and discards edits to generated files outside novel-feature files; only the preserve snapshot saved them.
    - cookie-auth CLIs: generated client drops env/set-token cookies whose name contains "." (e.g. Auth.js `__Secure-authjs.session-token`); see Phase 4.95 findings.
  Live spot checks (earlier, same session): movers/screen/sectors/breadth/quote/brief/history outputs agree with the site UI (advance/decline, % above DMAs, index levels, 52-week range, top movers).
  Gate: PASS

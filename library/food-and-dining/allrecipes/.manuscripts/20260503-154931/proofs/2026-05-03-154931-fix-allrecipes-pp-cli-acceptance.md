# Acceptance Report: allrecipes

**Level:** SKIPPED — Phase 5 auto-skip per skill rule

**Reason:** `auth.type: cookie` with `requires_browser_session: true`. The
CLI's recipe-detail surface needs a Cloudflare clearance cookie captured
interactively via `auth login --chrome`. No automated cookie source available
in this unattended generation session.

**Verdict by other gates:**
- shipcheck umbrella (post-polish): all legs PASS, scorecard 87/100, verify 100%, tools-audit 0 pending
- Phase 5.5 polish skill: `ship_recommendation: ship`

**Gate:** SKIP (acceptance not run; mechanical verification stands as the ship signal)

**To complete acceptance later:**
1. Run `allrecipes-pp-cli auth login --chrome` from a desktop session with a logged-in browser
2. Re-run `printing-press shipcheck` against the live API
3. Optional: spot-check the 8 novel features against real recipe URLs

# Lemon Squeezy CLI — Phase 5 Live Dogfood Acceptance Report

## Result

- **Level**: Full Dogfood
- **Matrix size**: 179 tests across every leaf command
- **Passed**: 179
- **Failed**: 0
- **Gate**: PASS

## Coverage

- Doctor + auth (`users me`) against real LS account: PASS
- All 19 resource list/get endpoints with --json + --select + error paths: PASS
- All 8 novel commands (--help + dry-run + mock invocations): PASS
- MCP cobratree mirror: PASS

## Notes

- The CLI is shipped against the user's real Lemon Squeezy account with a Bearer API key set via LEMONSQUEEZY_API_KEY in ~/.zshenv
- No mutations performed; refund-cascade defaults to dry-run, no --apply was passed in the matrix
- Account has 1 store ("partnerup"), 0 subscriptions (pre-launch — matches Founding-Member sale timeline from MEMORY)
- LS rate limit (300 req/min) was not approached
- No PII surfaced in this report per the privacy rule

## Verdict

Gate = PASS → proceed to Phase 5.5 Polish.

# Phase 4.95 — local code review

Review path: direct subagent dispatch (one reviewer covering correctness + security + maintainability personas) over the hand-written layer (ip_*.go, screen/quote/movers/sectors/changes/brief/breadth/history, the sync hook, the reworked set-token). Generator-owned packages (client, store, config, cliutil, mcp) were read-only context.

Autofix summary: 14 findings (0 high, 3 medium, 11 low) fixed in place in one round — snapshot validation before storing (an error envelope or empty list can no longer shadow good data; no more "today" fallback date; pruned to ~500 per kind; history keeps one row per symbol), `screen --saved` refuses `--data-source local`, `ad_ratio` omitted instead of 0 on an all-advancers day, brief picks headline symbols across all watchlists by absolute move, `history --tail` refreshes bars/from/to, `not` and bool comparisons on missing fields fixed, exponent numbers, symbol validation (rejects `.`/`..`/path chars) in quote and history, typed 401/403 detection via client.APIError (HTML-instead-of-JSON counts as not-signed-in), `--since` capped at 3650d, set-token normalisation hardened, text-column `--sort` rejected, terminal-escape scrubbing on server strings, surfaced previously swallowed errors, stale comments refreshed.

Additional finding from review follow-up (generator limitation): the generated client only seeds its cookie jar when the first cookie name has no "." (to tell cookie strings from JWTs), so a session passed through INVESTORPAISA_SESSION_TOKEN / set-token never reached the server (Auth.js cookie is `__Secure-authjs.session-token`). Fixed in the hand-written layer with a client hook (ipSeedSessionCookie, same https + investorpaisa.com guard) and unit-tested. `auth login --chrome` (the primary path) was unaffected and verified live.

Template-shape retro candidates (not patched in place):
- internal/client/cookiejar.go isCookieName rejects "." — breaks env/set-token auth for any site whose session cookie contains a dot (Auth.js, many others).
- dogfood config-consistency heuristic reports garbage fields (`", token, "`, `" || c.CookieCredential() != "`) from generated auth.go/config.go for cookie-auth CLIs.
- Generated `auth set-token` demands a JWT shape even when auth.type is cookie.
- Generated verify mock mode scores exec FAIL for read commands whose responses are not the mock's shape (changes/history/quote/screen) while the overall verdict stays PASS.

Out-of-scope retro candidates: none in internal/cliutil or cobratree.

Convergence: findings cleared at round 1 (tests + shipcheck 7/7 re-run). Skipped: duplicated RS-map/threshold helpers consolidation (low, behaviour-preserving, no correctness impact) and /simplify.

# Artistly CLI — Live Acceptance Report

Level: Quick Check (minimal live smoke, user-approved: ONE generation).
Auth: cookie session imported via `auth login --chrome` (pycookiecheat) from the
user's logged-in Chrome (Profile 5). Authenticated, app reachable.

## Tests (6/6 passed)
1. auth login --chrome — found 2 cookies (artistly_session, XSRF-TOKEN), session saved. PASS
2. doctor — Config ok, Auth configured (browser session), API reachable. PASS
3. designs list --json --select — returned real designs; --select filtered fields; provenance envelope correct. PASS
4. quota — read live generation budget from Inertia shared props (today 0, concurrent limit 65). PASS (proves authenticated Inertia-props extraction)
5. generate (1 image) — submitted successfully; the design was created and rendered to a real CDN image (verified via designs list: status private, 1 image). PASS (submit + async render)
6. designs download <id> — downloaded a real 1024x1024 PNG with templated filename. PASS (download path)

## Bug found & fixed inline
- generate --wait hit "context deadline exceeded" at 60s: the root --timeout (via boundCtx) capped the whole command, overriding --wait-timeout, and Artistly rendering exceeds 60s. FIXED: generate/batch/redo now use the command context directly and bound only the poll loop by --wait-timeout; each HTTP call stays bounded by the client's per-request timeout. Rebuilt; build/vet/tests pass. (CLI fix.)
- The submitted generation completed server-side regardless, confirming submit was correct; the failure was purely the timeout layering.

## Not exercised (intentional, within approved scope)
- Did not run the full binary-owned live matrix (dogfood --live) to avoid unapproved quota cost and live mutations (delete/move/folder-create) on the user's real account. The user approved a minimal smoke (one generation). Destructive commands (designs delete, folders remove) are gated behind --yes and were verified by dry-run + code review, not live deletion.
- prompt enhance/extract: deferred (unbuilt; Inertia-flash result contract not determinable from capture).

Gate: PASS

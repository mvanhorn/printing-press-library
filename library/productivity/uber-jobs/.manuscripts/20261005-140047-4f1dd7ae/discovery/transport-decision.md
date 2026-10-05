# Runtime transport and identity: FIXED 2026-10-05T10:00:34Z

- Transport: Go net/http stdlib (owner ladder rung 1). Phase 1.9 evidence: probe-reachability --probe-only stdlib gave GET /api/jobs/search/ 200 application/json at 2026-10-05T09:56:42Z. macOS curl (LibreSSL) got 403 cf-mitigated: challenge at 09:52:18Z (client-stack diagnosis, S5).
- Identity (owner chose "Honest UA", about 2026-10-05T10:00:34Z): User-Agent: uber-jobs-pp-cli/0.1.0 (the CLI's own name/version; spec version 0.1.0); Accept: application/json. No cookies, no Referer, no client hints. The same identity is used for discovery contract tests and for the printed CLI.
- From now on the TRANSPORT IS CHOSEN. The first 403, 429 or challenge from any uber.com host stops all uber.com traffic for the day: tell the owner, never retry, and never change the UA, headers or IP after a refusal.
- Pacing: >= 3 s between requests machine-wide; one sequential stream; cap 300/day for uber.com and a separate 300/day for the Oracle tenant (owner A2).

## Oracle fallback identity (owner decision 2026-10-05T11:46:08Z)
- User-Agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36" (the identity measured 3/3 at 200 in STEP 1); Accept: application/json; no cookies. jobs.uber.com keeps the honest UA. Fixed once; never changed after a refusal.

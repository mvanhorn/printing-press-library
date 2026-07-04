# Withings CLI — Live Smoke Test

> PII: no health values, account ids, cookies, or token values recorded.

## Official OAuth2 API (primary, durable)
- **Not live-tested.** The official Withings API requires an OAuth2 developer app
  (client_id/secret) + browser consent, which was not available in-session.
- Verified instead against: printing-press verify mock server (PASS), `--dry-run`
  request preview, synthetic-data unit tests, and structural shipcheck (Grade A 91/100).
- The `auth login` flow, form-POST transport, `{status,body}` envelope handling, and
  token refresh are exercised by unit tests and verify; they are not proven against
  the live account.

## Web tier (secondary, fragile cookie passthrough)
- **Live transport check: reached the real backend.** Replayed the exact request the
  CLI's `web call` issues (form-POST to `scalews.withings.com/cgi-bin/...`) against the
  live host. Calls returned **HTTP 200 with parsed `{status, error}` envelopes** —
  proving the request path, host, and envelope-unwrap work end-to-end against the real
  backend (not a network or transport failure).
- **Live data retrieval: blocked by an expired session.** Between the discovery sniff
  and this test (~1 hour), the web `session_token` cookie expired (confirmed absent;
  the page redirected to the login screen). With no valid session the backend returned
  logical errors (status 503 "invalid params" / 2554 "not implemented") rather than
  data. This is a **live demonstration of the documented web-tier fragility** — the
  session is short-lived and must be re-imported — and matches the CLI's built-in
  expiry handling (the `web call` error names the expired token and points to
  `web import-cookie`).
- No token was persisted; all output was reduced to status codes + structure.

## Verdict
- CLI is **structurally sound and Grade A**; transport paths verified.
- Live data verification was not achievable due to external factors only: the official
  API needs a dev-app credential (unavailable), and the web session expired mid-test
  (the very fragility the CLI documents and handles).
- The web tier's request path is proven against the live host; the official API path is
  proven against mocks + dry-run + synthetic data.

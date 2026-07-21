# Manual live verification (2026-07-21)

care.com uses cookie auth over a browser session, so the session-less
publish/CI harness cannot run the Phase 5 live matrix (skip_reason
`cookie-auth-no-harness-session`, matching the merged `jimmy-johns` and
`janeapp` CLIs).

The commands were verified against the real care.com API from a logged-in
session during development:

- `job list --json` returned the account's real open job posts.
- `find`, `recommend`, `caregiver`, `sync`, and the `messages` inbox returned
  live data end to end.

Shipped command examples and GraphQL flag defaults are intentionally
PII-scrubbed placeholders (generic UUIDs, example zip `90210`, example job id
`12345678`), so they do not resolve against a live account by design; that is
why a live matrix over the shipped examples cannot pass and the skip marker is
used instead.

# Known gaps

Honest documentation of where this CLI's coverage is incomplete. The
[main README](./README.md) keeps things tight; this file is the long form.

## Write-endpoint coverage

All 37 write endpoints (POST/PUT/DELETE) have URL routing + HTTP method
verified via `scripts/writes-shipcheck.sh` (a `--dry-run` matrix). Tenant
mutation coverage should be run against an isolated sandbox before
production use; the public repository intentionally does not carry
tenant-specific proof artifacts.

- **Roundtrip candidates.** `tags`, `macros`, `teams`, `views`,
  `widgets`, and `integrations` have create/update/delete shapes that can
  be exercised by `scripts/writes-live-shipcheck.sh` when a sandbox tenant
  is available. Every successful create is paired with a delete on the
  same path; a failed run may leave one marker-named row behind.
- **One-shot candidates.** `tickets`, `customers`, and `custom-fields`
  writes can affect customer or admin-visible state. Use
  `scripts/writes-verify-once.sh` only in an isolated tenant, and clean up
  any marker rows manually.
- **Dry-run-only until sandboxed.** Some endpoints are too risky to
  exercise against production:
  - `customers delete` — cascades through ticket history
  - `users create/update/delete` — invitation emails / seat consumption / lockouts
  - `satisfaction-surveys create/update` — attaches a CSAT score to a real ticket
  - `gorgias-jobs create/update/delete` — async bulk operations
  - `rules create/update/delete` — DSL gate plus auto-fire risk on real tickets

## Other coverage gaps

- **Phone/voice integration.** Tenants without a voice integration have
  no calls in these endpoints, so `phone calls-list`,
  `phone call-events-list`, and `phone call-recordings-list` haven't been
  exercised against voice data. The wire-shape is generated from the same
  spec as everything else; failure modes are likely small but unconfirmed.
- **Global `/messages` endpoint** supports only the `ticket_id` filter —
  no datetime range, no channel filter. Use
  `tickets messages-list <ticket-id>` for per-ticket views.
- **Ticket list has no documented updated-time cutoff.** The CLI's
  `sync --resources tickets --since ...` path requests documented
  `order_by=updated_datetime:desc` and applies the cutoff locally.
  Gorgias rejects unsupported filters such as `updated_datetime__gte`;
  do not add that filter unless Gorgias documents it and a tenant smoke
  accepts it.
- **Remote `search`** uses Gorgias's `POST /search`, which indexes
  customers, agents, tags, teams, and integrations — **not tickets or
  messages**. For ticket/message text search, sync to local first and
  use `gorgias-pp-cli search <query>` against the FTS5 mirror.
- **Customer-list filter incompatibilities.** `--language` and
  `--timezone` are mutually exclusive with `--cursor`/`--limit`/
  `--order-by` on `/customers` (server-side; undocumented in OpenAPI).
  Pick one approach per call.
- **No `--wait` for async jobs.** `gorgias-pp-cli gorgias-jobs create`
  returns the new job's id but doesn't poll for completion or persist it
  in a local ledger. If you need synchronous behavior, call
  `gorgias-jobs get <id>` in a loop yourself. Adding `--wait` is on the
  roadmap.

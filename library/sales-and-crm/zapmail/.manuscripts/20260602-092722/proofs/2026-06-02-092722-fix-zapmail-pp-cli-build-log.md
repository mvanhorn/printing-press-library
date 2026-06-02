# Zapmail CLI Build Log

## Generated (Priority 0 + 1)
- 9 resources, 22 endpoints from hand-authored internal YAML spec (`zapmail-spec.yaml`).
- P0 data layer: SQLite store for user, workspaces, mailboxes, domains, dns, subscriptions, exports, inbox; `sync`, FTS `search`, `sql`.
- P1 absorbed commands (all generated + verified to build/dry-run/json):
  - `user get`, `workspaces list/create`
  - `mailboxes list/get`
  - `domains list/assignable/health-score/ai-finder/available-bulk`
  - `dns list/add`
  - `subscriptions list`, `wallet balance`
  - `exports mailboxes/status/accounts/add-account`
  - `inbox accounts/emails/search/send`
  - `doctor`

## Hand-built (Priority 2 transcendence) - all 7 from the approved manifest
- `analytics --type fleet-health` (local: domains health/abused rollup)
- `analytics --type renewals` (local: subscriptions period-end + price, weekly buckets)
- `analytics --type cost-efficiency` (local: active spend / assigned mailboxes)
- `analytics --type capacity` (local: purchased vs assigned vs available)
- `mailboxes idle` (live: warmed mailboxes + paid-but-unprovisioned count)
- `mailboxes failed` (live: mailboxes not in ACTIVE status)
- `exports watch` (live: poll one export to terminal, typed exit) - replaced approved `exports stalled` (no list-exports endpoint exists; user re-approved the substitution)

## Bonus fix
- Array-body flags (`--apps`, `--ids`, `--keywords`, `--tlds`, `--cc`, `--bcc`, `--domain-names`) now accept comma-separated values (CSV) as well as JSON arrays, via a shared `parseListFlag` helper. The generated default required hand-written JSON (`'["SMARTLEAD"]'`), which broke usability and examples.

## Tests
- `listflag_test.go`: table tests for `parseListFlag`, `parseFlexibleTime`, `round2`.

## Intentionally deferred (documented gap)
- All money-spending endpoints: purchase subscriptions/domains/add-ons/placement-tests, wallet top-up, domain renewals, DNS Shield. v1 is read + management + export + send.
- `mailboxes assign` / `mailboxes update`: awkward request bodies (domain-UUID-keyed map / array-of-objects); deferred from v1 rather than ship unverifiable write commands.
- Multi-workspace switching (`x-workspace-key`): v1 operates in the primary workspace. Fleet analytics reflect the synced (primary) scope.

## Notes
- `inbox send` performs a real outbound email send when not `--dry-run`; gated and verify-safe.
- `mailboxes idle/failed` are live (data-source live) because the mailbox list nests inboxes under domains; pure-local flattening of the store isn't reliable.

# SmartLead CLI — Build Log

## Generated (Priority 0 + 1)
- 39 of 41 spec endpoints generated as typed commands across campaigns, leads,
  email-accounts, webhooks, analytics (nested under campaigns), client.
- SQLite data layer: `resources` generic table + 15 typed sub-resource tables
  (campaigns_leads, statistics, warmup_stats, sequences, webhooks, etc.) + FTS5.
- `sync` syncs campaigns, client, email-accounts, lead-categories, and the
  dependent resources campaigns_leads + statistics (per-campaign).
- Framework commands: sync, search, sql (via analytics), export, import, tail,
  doctor, auth, profile, workflow, agent-context, which, feedback.

## Deferred / skipped
- 2 endpoints dropped from per-field flag generation by the generator (array-of
  -object / bare-object request bodies): `saveSequences` and one of
  `addLeadsToCampaign`/`updateLead`. `sequences save` ships with `--sequences`
  + `--stdin` body input — fully usable, just body-driven not per-field.
- `warmup_stats` and `campaigns_email_accounts` are not auto-synced dependent
  resources (the generator only detected campaigns_leads + statistics). The
  warmup-stats endpoint returns a single object, not a list, so adding it to
  the dependent-sync machinery was judged riskier than the alternative:
  `sender-health` and `warmup-gate` fetch warmup-stats live per account. This
  is honest — those are occasional audit commands, not hot-path reads.

## Built (Priority 2 — transcendence, hand-built)
All 6 approved novel features, in `internal/cli/`:
- `health.go` — campaign health scorecard. Store query: campaigns + statistics
  + campaigns_leads. open/reply/bounce rate, silent-lead count, stale flag.
- `silent.go` — silent-lead finder. Store query over statistics; per-lead
  send/reply timestamp diff with an N-day window.
- `dupes.go` — cross-campaign dupe + domain ledger. Store query over
  campaigns_leads; default / --email / --domain modes.
- `sender_health.go` — sender deliverability ranking. Live: email-accounts +
  per-account warmup-stats; composite score, worst-first.
- `warmup_gate.go` — typed pass/fail launch gate. Live: email-accounts +
  warmup-stats; `--strict` exits non-zero on failure (verify-env guarded).
- `drift.go` — week-over-week reply/open/bounce drift. Live: analytics-by-date
  once per 7-day window; deltas computed offline.
- `transcend.go` — shared pure helpers (domainOf, parseSLTime, warmupInboxRate,
  rate, asInt/asBool/asString, fetchArray, loadCampaignMeta).
- `transcend_test.go` — table tests for all pure helpers.

All 6 registered in root.go. Build/vet/test green. --help and --dry-run
verified exit 0 for every command. mcp:read-only set on all 6 (none mutate).

## Generator limitations noted for retro
- Tag-based resource grouping lost to path-based grouping: 7 OpenAPI tags
  produced 19 resources nested by path. Coherent but deeper command tree than
  the tags intended (e.g. `campaigns analytics get-campaign`).
- Single-object sub-resource endpoints (warmup-stats) are not eligible for
  dependent-resource sync; only list/paginated children are detected.

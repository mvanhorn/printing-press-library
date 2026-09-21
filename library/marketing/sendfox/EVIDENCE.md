# Local evidence format (version 1)

The eight `workflow` reports use supplied files or a SQLite mirror. They never read an account. `examples/snapshot.json` is a complete synthetic example; `examples/kit-snapshot.json` shows the normalized source shape. No sample is real subscriber data.

Use JSON, YAML, or a Markdown document with YAML frontmatter. The top-level `schema_version` must be `1`. Each `resources` entry contains:

```json
{"items": [], "source": "operator export", "observed_at": "2026-09-21T18:00:00Z", "complete": true, "total": 0, "pages": 1, "capped": false, "scopes_complete": true}
```

Declare completeness only after every page and requested child scope is captured. A missing resource, source, timestamp, continuation page or scope is unknown. `--max-age` controls freshness checks; `0` disables age checks explicitly, but never fills missing metadata. Exported rows that disappear are observations, not deletion instructions. Snapshots taken across resources are not a transactional account snapshot.

| Resource key | Row shape / source |
|---|---|
| contacts | Contact objects with id, email, profile and unsubscribe fields |
| unsubscribed | Complete explicit suppression export; email required |
| lists, tags, contact_fields | Root definitions with stable id and name/title/label; field type text/number/date |
| memberships | Flatten list-contact exports to contact_id + list_id |
| contact_tags | Flatten each contact's tags to contact_id + tag_id |
| campaigns | Campaign objects, including status for resend evidence |
| campaign_stats | Authoritative stats plus campaign_id; link_stats is optional and absence means unknown |
| engagement | Flatten scoped rows with campaign_id, type, contact_id, email; type non_openers/openers/clickers |
| activity | Flatten scoped activity with contact_id; preserve all available event fields |
| forms, domains, automations, automation_emails | Corresponding REST objects; domain evidence uses domain and validated_at |

The flat sync mirror supplies root resources. Child memberships, tags, stats, engagement and activity require explicit scoped exports and normalization; their absence is reported rather than inferred from global totals. A local mirror with no evidence returns unknown completeness.

`campaign` supplies the create-campaign fields. An optional `id` selects review evidence; it is excluded from create payloads. Preflight omits `scheduled_at` and reports it as a blocker. `automation` supplies title, trigger_type, trigger_list_id/trigger_campaign_id, and ordered `emails`; the plan always uses active=false, default first delay 0 and subsequent delay 24 hours. Definitions can be authored as YAML frontmatter in Markdown. These commands produce reviewable requests, never execute them.

For migration readiness, put normalized Kit evidence in `kit` or pass `--kit FILE`. Native Kit exports must first be mapped into this documented format; absent dimensions stay unknown. `previous` or `--previous FILE` is the prior Kit snapshot. `mappings` entries contain `kind`, string `source` and `target` IDs, and `approved` boolean. Supported mapping families are lists, tags, contact_fields, forms and automations. Name matches are suggestions only. `requirements` can name hard gaps from the returned `api_gaps` list to make them explicit blockers.

Reports contain findings, blockers, unknowns, source_evidence, proposed_actions and data. `ready` means only that the checks supported by the supplied evidence passed; it never approves delivery, activation or migration. Exit 0 means the report was produced; inspect `ready`, blockers and unknowns. Output samples are bounded by `--limit`; original counts and `output_truncation` remain visible. `workflow export-bundle` is strictly read-only and includes the supplied snapshot and deterministic SHA-256 hashes. Use `workflow snapshot-save --out DIR` to atomically persist the same bundle as a dated JSON snapshot with a checksum in its filename, a returned full SHA-256, directory mode `0700` and file mode `0600`. The separate command makes filesystem mutation explicit and `--out` is required.

`workflow growth-report` uses `contacts.created_at` and suppression timestamps for a one-snapshot window. That observed net is explicitly partial because deleted or otherwise vanished rows cannot be reconstructed. Add `--previous FILE` to compare complete membership sets for exact observed per-list joins, leaves, retention and net growth between those snapshots. Every growth metric includes its formula, window, numerator, denominator, evidence grade, completeness and warnings. A missing timestamp, scope or denominator returns `null`, not zero. `audience-health` applies the same rule to engagement cohorts: "never engaged" is only counted when activity scopes are complete; otherwise the unclassified contacts are "unknown". `campaign-review` recomputes rates from counters, uses recipient-weighted totals and campaign medians, and only emits chronological trends when sortable campaign timestamps exist.

CSV helpers accept email, first_name/first, last_name/last, status and unsubscribed_at. Audits reject malformed CSV and report invalid/duplicate identities. Reconciliation/import additionally require complete fresh contacts and unsubscribed snapshots. Import is a preview unless `--execute --yes` is supplied. Batches stop on ambiguous errors or mismatched acknowledgements; reconcile before retrying. No SendFox idempotency-key guarantee is assumed.

Compatibility: `auth set-token --stdin` accepts a personal access token only through stdin, never a positional argument. It saves locally without verifying against SendFox. `lists contacts <list_id>` aliases `lists contacts-in`; `webhooks` explains the public API gap. Evidence plans preserve request bodies even with `--agent`; `--select` remains available for deliberate projection.

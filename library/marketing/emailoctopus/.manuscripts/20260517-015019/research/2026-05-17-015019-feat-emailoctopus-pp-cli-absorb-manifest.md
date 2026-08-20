# EmailOctopus Absorb Manifest

## Source landscape

No actively-maintained v2-native CLI or SDK exists in any language. Every general-purpose wrapper on GitHub/npm/PyPI targets the deprecated v1 API. Closest competing agent surface is the **Zapier hosted MCP for EmailOctopus** (6 contact-only tools, each costing 2 Zapier tasks). **Activepieces piece #9300** adds contact CRUD + tag ops + list create + triggers within their no-code platform but is not standalone. We absorb their full surface and add the remaining 19+ endpoints they don't expose.

## Absorbed (Priority 1) — match every existing feature

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List all lists | EmailOctopus v2 | `lists list` typed cmd | `--json --select --csv --all` auto-paginate |
| 2 | Get one list | EmailOctopus v2 | `lists get <id>` | `--select --json` |
| 3 | Create list | EmailOctopus v2 | `lists create --name --double-opt-in` | `--dry-run`, agent-native |
| 4 | Update list | EmailOctopus v2 | `lists update <id>` | `--dry-run` |
| 5 | Delete list | EmailOctopus v2 | `lists delete <id>` | `--dry-run`, typed exit codes |
| 6 | List contacts on a list | EmailOctopus v2 | `contacts list <list_id>` | `--json --select --csv --all` |
| 7 | Get contact | EmailOctopus v2 / Zapier `find_contact` | `contacts get <list_id> <contact_id>` | `--json --select` |
| 8 | Create contact | EmailOctopus v2 / Zapier `add_subscriber` | `contacts create <list_id> --email --fields --tags --status` | `--dry-run`, stdin |
| 9 | Upsert contact | EmailOctopus v2 / Zapier `update_subscriber` | `contacts upsert <list_id> --email …` | `--dry-run` |
| 10 | Update contact (by id) | EmailOctopus v2 / Zapier `change_email_address` | `contacts update <list_id> <contact_id>` | `--dry-run` |
| 11 | Delete contact | EmailOctopus v2 | `contacts delete <list_id> <contact_id>` | `--dry-run`, typed exit codes |
| 12 | Batch update contacts | EmailOctopus v2 (only batch endpoint) | `contacts batch <list_id> --stdin` | rate-limit aware, `--dry-run` |
| 13 | List tags | EmailOctopus v2 | `tags list <list_id>` | `--json --select` |
| 14 | Create tag | EmailOctopus v2 / Zapier `add_tag_to_subscriber` (indirect) | `tags create <list_id> --tag` | `--dry-run` |
| 15 | Update tag | EmailOctopus v2 | `tags update <list_id> <tag>` | `--dry-run` |
| 16 | Delete tag | EmailOctopus v2 | `tags delete <list_id> <tag>` | `--dry-run` |
| 17 | Create custom field | EmailOctopus v2 | `fields create <list_id> --tag --type --fallback` | `--dry-run` |
| 18 | Update custom field | EmailOctopus v2 | `fields update <list_id> <tag>` | `--dry-run` |
| 19 | Delete custom field | EmailOctopus v2 | `fields delete <list_id> <tag>` | `--dry-run` |
| 20 | List campaigns | EmailOctopus v2 | `campaigns list` | `--json --select --csv --all` |
| 21 | Get campaign | EmailOctopus v2 | `campaigns get <campaign_id>` | `--json --select` |
| 22 | Campaign contact-level report | EmailOctopus v2 | `campaigns reports contacts <campaign_id>` | filter opened/clicked/bounced |
| 23 | Campaign links report | EmailOctopus v2 | `campaigns reports links <campaign_id>` | `--json --csv` |
| 24 | Campaign summary report | EmailOctopus v2 | `campaigns reports summary <campaign_id>` | `--json --select` |
| 25 | Trigger automation queue | EmailOctopus v2 / Activepieces piece | `automations queue <automation_id> --contact-id` | `--dry-run` |

All 25 endpoints from the official OpenAPI 3.1 spec. Zapier's 6 hosted tools and Activepieces' subset are entirely covered by rows 1, 6-12, 14, and 25.

## Transcendence (Priority 2) — features only possible with our approach

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Per-contact engagement scorecard | `contacts engagement [--inactive-since 90d]` | 9/10 | Joins local `contacts` table with per-campaign contact-report rows synced from `GET /campaigns/{id}/reports/contacts`; computes open/click counts, last-engaged, inactive bucket | Indira's "haven't opened in 90 days" question; Marcus's reactivation cohort; API has no engagement-history endpoint |
| 2 | Cross-list duplicate finder | `contacts dedupe [--merge-into <list>]` | 9/10 | Local SQL `GROUP BY lower(email) HAVING COUNT(DISTINCT list_id) > 1` over synced contacts table | Brief Top Workflow #6; Priya's 18-client agency |
| 3 | Campaign performance digest | `campaigns digest <id> [--md]` | 8/10 | Single command joins summary + links + contacts-report endpoints plus local contact-domain breakdown; table or Markdown for paste-into-Notion | Marcus's Monday Notion doc; Priya's Friday client reports |
| 4 | List churn diff | `lists diff <id> [--since <snapshot>]` | 8/10 | Diff between two `.runstate` snapshots of the same list; classifies added/removed/status-changed contacts | Brief Workflow #6; no `updated_after` filter in API forces this approach |
| 5 | Tag set-algebra query | `tags intersect --has X --not Y [--list <id>]` | 8/10 | Local SQL set operations over contact-tag join table | Marcus's "trial-started AND NOT activated" segmentation; Zapier MCP has zero tag-query tools |
| 6 | CSV upsert with field/tag mapping | `contacts sync-csv <file> --list <id> --map email=Email,tag.plan=Plan` | 7/10 | Parses CSV, builds upsert payloads, dry-runs diff against local store, chunks into `PUT /lists/{id}/contacts/batch` calls with rate-limit pacing | Indira's Sunday Stripe→Sheets→EO ritual; brief Top Workflow #1 |
| 7 | Rate-budgeted bulk delete | `contacts bulk-delete --where "tag:churned AND inactive>180d" --rate 8` | 7/10 | Resolves predicate locally to ID list, then issues real `DELETE` calls paced under 10/sec with resumable progress | Brief Priority 2 explicitly names "rate-limit-budgeted bulk delete" |
| 8 | Bulk automation trigger | `automations trigger-batch <automation_id> --stdin` | 6/10 | Reads contact IDs from stdin/CSV, calls real `POST /automations/{id}/queue` per contact with backoff on 429 | Brief Workflow #3 (only API write path that triggers a send); Marcus's trial-onboarding burst |

## Stubs

None planned. All 25 absorbed endpoints have request/response shapes documented in the OpenAPI spec; all 8 transcendence features are buildable from the synced store + real endpoint calls. No stubbed features in shipping scope.

# Sigma Computing — Absorb Manifest

## Scope summary
- **Spec:** 175 paths (115 GET / 73 POST / 37 DELETE / 22 PATCH / 5 PUT). The generator absorbs the full endpoint surface as typed commands automatically (Priority 0 + 1).
- **Competitive landscape:** No Go SDK, no general-purpose CLI, nothing on PyPI/npm. Best existing tools are narrow: official `sigma-agent-skills` (2 skills, data-model spec only), `sigma-sample-api` (1 script), `embed-sdk` (TS embeds, orthogonal), product-native MCP (chat-only, not scriptable), and a prototype community MCP (`ja2z/mcp-server`, 0 stars). This CLI would be the first full-featured Sigma CLI.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Workbook list/get/copy/export (CSV/PDF/XLSX) | Sigma JS quickstart, Truto | Spec endpoints | `--json`, local store, offline FTS, `--web` |
| 2 | Workbook materialization schedule + run | Sigma API | Spec endpoints | typed error on "must run once" gotcha |
| 3 | Workbook elements/pages/queries/lineage/version-history/bookmarks/embeds/tags | Sigma API | Spec endpoints | `--select` to narrow deep nesting |
| 4 | Member CRUD + deactivate + team-assign + user-attributes | Sigma JS quickstart, sigma-sample-api | Spec endpoints | bulk + idempotent (see transcendence) |
| 5 | Team CRUD + members + user-attributes | Sigma API | Spec endpoints | local store |
| 6 | Connection list/get/test/schema-sync/grants/paths/table-columns | Sigma JS quickstart | Spec endpoints | `--json`, offline search |
| 7 | Data-model spec CRUD (sources/columns/metrics/relationships/elements/lineage/materialization) | official sigma-agent-skills | Spec endpoints | beats the 2-skill scope |
| 8 | Grants list/get on workbooks/datasets/connections/workspaces/paths | Sigma API | Spec endpoints | feeds grant-audit join |
| 9 | Workspaces / tenants / deployment-policies / tags / favorites / reports | Sigma API | Spec endpoints | local store |
| 10 | whoami / auth (OAuth2 client-credentials) | Sigma API | Generated auth + doctor | gh-style `auth status` |
| 11 | api-connectors / api-credentials / saml / allowed-ips / templates / translations | Sigma API | Spec endpoints | full surface coverage |

Every spec endpoint becomes a typed command with `--json`, `--dry-run` (mutations), typed exit codes, and SQLite persistence for list endpoints.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Grant audit (effective access) | `grant audit <resource>` | 8/10 | Local SQLite join expands grants → teams → members and flags org-wide/public; no single API call gives effective member list |
| 2 | Workbook copy + ownership repair | `workbook copy <wb> --to <member>` | 8/10 | Bundles the documented copy-then-`PATCH /v2/inodes/{id}` ownership fix into one safe command |
| 3 | Stale-workbook detection | `workbook stale --days <N>` | 7/10 | Local-data threshold over synced workbooks joined to owner + path; no API filter for "untouched in N days" |
| 4 | Member offboard + content reassign | `member offboard <email> --transfer-to <member>` | 7/10 | Deactivate + enumerate owned inodes from local index + reassign each — closes orphaned-content gap |
| 5 | Bulk member provisioning | `member provision --from <csv>` | 7/10 | Idempotent CSV-driven member-create + team-assign + user-attribute-set; the REST-only path SCIM can't give non-Enterprise orgs |
| 6 | Member access review (reverse audit) | `access review <member-email>` | 6/10 | Local join member → teams → grants → resources: everything one member can reach |
| 7 | Bulk export by search | `export bulk --query <FTS>` | 6/10 | Offline FTS resolves a workbook set, then loops the export endpoint across all matches |

No stubs. All 7 transcendence features are shipping scope, built fully in Phase 3.

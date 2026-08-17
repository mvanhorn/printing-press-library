# Clarify Absorb Manifest

## Absorbed (match or beat everything that exists)

Landscape: no competing CLI exists (GitHub, npm, PyPI all empty). The only surfaces are Clarify's hosted MCP server (closed source, online-only), the official clarifyhq/skills conventions doc, and @getclarify/nodejs-sdk v0.0.5 (thin, early). Absorption target = the hosted MCP's capabilities + the full 75-operation OpenAPI surface.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Query records with filters/include/pagination | Clarify MCP server | (generated endpoint) resources query | Typed flags for all 8 operator families, offline mirror, --json/--select/--csv |
| 2 | Create/update/delete records (incl. upsert match_on) | Clarify MCP server | (generated endpoint) records create/update/delete | --dry-run, collection-field {items:[...]} handled natively |
| 3 | Bulk create/update records | API /records/bulk | (generated endpoint) records bulk | high-volume import path per rate-limit docs |
| 4 | List CRUD + publish/unpublish | API lists endpoints | (generated endpoint) lists | dynamic-list SQL passthrough with version:6 stamping |
| 5 | List rows CSV export | API lists rows/csv | (generated endpoint) lists rows csv | pipe straight to files/agents |
| 6 | Schema introspection (objects/properties/relationships) | API schemas | (generated endpoint) schemas | agents "read the schema first" per docs |
| 7 | Comments create/get/update/delete | API comments | (generated endpoint) comments | - |
| 8 | Record activities feed | API activities | (generated endpoint) activities | timeline access |
| 9 | Meeting transcript/recordings/media/artifacts | API meetings | (generated endpoint) meetings | transcript text to stdout |
| 10 | Workflows CRUD | API workflows | (generated endpoint) workflows | automation management |
| 11 | Users, settings, campaign recipients, record merges, access grants, record relationships | API | (generated endpoint) various | full 75-op surface parity |
| 12 | Local sync + FTS search + read-only SQL | printing-press framework | clarify-pp-cli sync / search / sql | offline mirror no Clarify surface has; local OR-across-fields the API can't express |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Meeting prep pack | prep | 9/10 | hand-code | Joins attendees' person/company/deal rows + transcript FTS excerpts locally; hosted MCP needs 4-6 online calls | Use this command to prepare for one upcoming meeting. Do NOT use it for a general record background bundle; use 'dossier' instead. |
| 2 | Morning briefing | brief | 8/10 | hand-code | Cross-entity local join (today's meetings × companies × open deals × yesterday's activity); no upstream analytics endpoints | Use this command for a start-of-day overview across all meetings and deals. Do NOT use it to prepare for one specific meeting; use 'prep' instead. Do NOT use it to list stalled deals only; use 'stale' instead. |
| 3 | Follow-up gaps | followup | 8/10 | hand-code | Anti-join (meetings with no later activity/comment/task) — inexpressible upstream; --no-deal flag catches orphan companies | none |
| 4 | Stale-deal report | stale | 8/10 | hand-code | Local aggregation over _updated_at by stage; API has no analytics | Use this command to find deals with no recent activity. Do NOT use it for a full daily overview; use 'brief' instead. |
| 5 | Record dossier | dossier | 8/10 | hand-code | One compact --agent payload replacing the hosted MCP's multi-call chain; local mirror with live fetch on miss | Use this command for a complete background bundle on any record. Do NOT use it to prepare for a specific meeting; use 'prep' instead. |
| 6 | Pipeline velocity | velocity | 7/10 | hand-code | Stage-snapshot side table maintained at sync time — history no Clarify surface keeps; includes conversion counts | none |
| 7 | Duplicate finder | dupes | 7/10 | hand-code | Local GROUP BY on normalized emails/domains/names; emits ready-to-run merge commands against the buried merges endpoint | none |

Killed candidates and customer model: see 2026-08-17-103417-novel-features-brainstorm.md.

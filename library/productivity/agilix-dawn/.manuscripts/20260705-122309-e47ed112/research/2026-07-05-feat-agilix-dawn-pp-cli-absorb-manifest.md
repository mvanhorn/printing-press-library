# Agilix Dawn CLI — Absorb Manifest

## Landscape note
No tool targets the modern Dawn `/api`. Existing libraries (beneggett/agilix,
StrongMind/agilix-buzz-client, buzzapi, the Zapier "Agilix Dawn" app) all wrap the
LEGACY Buzz/DLAP `cmd=` RPC API on a different host. Their capability *categories*
(list/create users, list courses, enrollments, reports) map onto our real Dawn
resources, but their command surfaces are not literally absorbable. So "absorb"
here = the full framework surface + every verified real Dawn endpoint.

## Absorbed (foundation + every real endpoint)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List catalog courses/content | Dawn /concept (verified) | (generated endpoint) concept list | search DSL, offline, --json, --select |
| 2 | Get a course by id (full structure) | Dawn /concept/{id} (verified) | (generated endpoint) concept get | full section tree, offline cache |
| 3 | List/search users | Dawn /user (verified) | (generated endpoint) user list | Lucene search, offline, --select |
| 4 | Whoami | Dawn /user/me (verified) | (generated endpoint) user me | scriptable identity check |
| 5 | List organizations | Dawn /organization (verified) | (generated endpoint) organization list | offline, --json |
| 6 | List purchases (commerce) | Dawn /purchase (verified) | (generated endpoint) purchase list | offline, --json |
| 7 | List learner progress | Dawn /progress (verified) | (generated endpoint) progress list | search DSL, offline |
| 8 | List conversations | Dawn /conversation (verified) | (generated endpoint) conversation list | offline, --json |
| 9 | Tenant config | Dawn /config (verified) | (generated endpoint) config get | one-shot introspection |
| 10 | Offline mirror / sync | framework | (behavior in agilix-dawn-pp-cli sync) | local SQLite for all entities |
| 11 | Offline full-text search | framework | (behavior in agilix-dawn-pp-cli search) | FTS over synced concepts/users |
| 12 | SQL over local data | framework | (behavior in agilix-dawn-pp-cli sql) | ad-hoc joins/reporting offline |
| 13 | Health + auth check | framework | (behavior in agilix-dawn-pp-cli doctor) | verifies token + /config reachability |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|-----------------|
| 1 | Course structure tree | course tree c_... | hand-code | Walks concept/{id} nested section→instruction→interaction (34 sections / 392 instructions) the web UI only paginates | Use to inspect a whole course's layout at once. Do NOT use for a flat title list; use 'concept list'. |
| 2 | Course stats rollup | course stats c_... | hand-code | Sums instruction durations (≈47h), points, section/instruction/interaction counts locally — no single API field gives this | Use for total seat-time and content-size metrics of one course. |
| 3 | Curriculum export | course outline c_... --format md\|csv | hand-code | Flattens the section/instruction tree to a syllabus/state-reporting doc | Use to export a course's curriculum. Do NOT use for grades; Dawn exposes no grade collection here. |
| 4 | Catalog drift | catalog diff | hand-code | Compares the current catalog against the last local sync (new/removed courses, price/status/title changes) | Use to see what changed in the catalog since last sync. Requires a prior 'sync'. |
| 5 | Roster export | roster export --format csv | hand-code | Exports users (id, name, email, status, verified) to CSV for reporting | Use to export the user roster. |
| 6 | Purchase reconcile | purchase reconcile | hand-code | Local join of purchases ↔ users (who paid, what they bought) — the API returns ids, not joined records | Use to reconcile Stripe purchases against user records. |

Minimum 5 transcendence features: satisfied (6). All hand-code.

## Stubs
None. All rows above are shipping scope.

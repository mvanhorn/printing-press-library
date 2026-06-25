# Toodledo CLI — Phase 3 Build Log

Manifest transcendence rows: 7 planned, 7 built. Phase 3 gate PASS (novel_features_check: planned 7, found 7).

## Generator output (Priority 0 + most of Priority 1)
- Data layer: types Task/Folder/Context/Goal/Location/Note/Outline/CustomList/Account → store schema + sync.
- Generated resource CRUD: tasks(list,deleted), folders(list,add,edit,delete), contexts(*), goals(*), locations(*), notes(list,add,edit,delete,deleted), outlines(*), lists(*), account(get).
- Framework: sync, search, sql/analytics, reconcile, doctor, auth/login, tail, import, workflow.
- Novel commands emitted as STUBS (to replace): next-actions, review, dashboard, stalled-projects, goal-progress, sync-cost, capture.

## Phase 3 hand-build queue
- [ ] OAuth token exchange + refresh → HTTP Basic client auth (Toodledo requirement; generator emits form-encoded).
- [ ] Ergonomic task writes: tasks add / edit / complete / reopen / delete (JSON-batch `tasks` param + name→id + date parse).
- [ ] Novel: next-actions, review, dashboard, stalled-projects, goal-progress, sync-cost, capture.
- [ ] Cloudflare 403≠401 handling; non-Pro subtask warning.

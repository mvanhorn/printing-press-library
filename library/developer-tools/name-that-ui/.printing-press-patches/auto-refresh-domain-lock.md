# Concurrent auto-refresh domain mirror

Keep the hand-authored NameThatUI mirror in `internal/cli/auto_refresh.go` in
sync with explicit `sync --resources catalog,styles`: after generated catalog
and styles refreshes, populate `components`, `style_details`, and immutable
snapshots unless `PRINTING_PRESS_VERIFY=1`. Serialize the shared-HOME
stale-decision and refresh path with a bounded, context-aware, database-adjacent
exclusive lock. Expired locks are recovered by atomically moving them aside, and
token-checked release prevents a crashed owner's delayed cleanup from deleting
a successor lock.

Register only command paths that this generated CLI exposes; do not add stale
`catalog search` or `styles search` entries.

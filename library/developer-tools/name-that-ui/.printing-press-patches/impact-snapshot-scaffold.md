# Impact snapshots and local impact scanner

Preserve the implemented `internal/cli/impact.go` scaffold and its local-only
`// pp:data-source local` contract without editing generated `root.go`.
NameThatUI sync must snapshot component and style records into generic,
immutable `component_snapshots` and `style_snapshots` resources before their
sync state advances. Keep content-hash deduplication per entity, honest
no-baseline behavior, deterministic local source scanning, source provenance,
and dry runs that open neither SQLite nor the project tree.

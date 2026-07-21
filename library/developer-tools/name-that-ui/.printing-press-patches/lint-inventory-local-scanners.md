# Lint and inventory local scanners

Preserve the implemented `internal/cli/lint.go` and `internal/cli/inventory.go`
scaffolds without changing generated `root.go`. Both commands are local-only,
read-only consumers of the synced `components` mirror. Keep dry runs free of
file, tree, and SQLite reads, including optional learning initialization;
enforce 2 MiB and binary guards; use token-aware
minimum-meaningful-phrase matching; retain deterministic output, explicit
ambiguity, source URLs, and non-null result arrays. Inventory must never follow
symlinks or traverse generated/dependency directories.

For lint, suppress lowercase plain-word API and part-API symbols, but preserve
code-shaped symbols. Match retained API symbols case-sensitively; component,
alias, fuzzy-phrase, and part matching remains case-insensitive.

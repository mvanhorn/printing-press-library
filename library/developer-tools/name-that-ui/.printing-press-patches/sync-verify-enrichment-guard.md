# Verification-safe NameThatUI sync enrichment

Preserve the `internal/cli/sync.go` verification boundary for the hand-authored
NameThatUI HTML mirror. During `PRINTING_PRESS_VERIFY=1`, the Printing Press
mock supplies JSON fixtures: generated sync must consume those fixtures through
its normal upsert pipeline without HTML extraction, while the separate component
and style enrichment must neither crawl nor parse them.

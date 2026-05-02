# Republish Notes

This run republished the Shopify CLI from a fresh generated scaffold and reapplied the Shopify-specific hand finish:

- Added `internal/cli/bulk_operations.go`.
- Registered `bulk-operations` in `internal/cli/root.go`.
- Kept the curated wrapper spec as `spec.yaml`.
- Rewrote package imports to `github.com/mvanhorn/printing-press-library/library/commerce/shopify`.
- Regenerated the direct install skill mirror with `go run ./tools/generate-skills/main.go`.

The original archived printing-run manuscripts were not present under `$HOME/printing-press/manuscripts`, so this manuscript directory records the republish correction pass and its proofs.

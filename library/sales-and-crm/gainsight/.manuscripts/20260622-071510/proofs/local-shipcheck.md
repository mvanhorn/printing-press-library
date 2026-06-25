# Local shipcheck for gainsight-pp-cli

This CLI was generated from a saved local OpenAPI spec and verified locally before publishing.

## Verification run

- `cli-printing-press generate --spec /Users/debmukherjee/printing-press/specs/gainsight-read-openapi.yaml --name gainsight --category sales-and-crm --spec-source docs --spec-url https://example.gainsightcloud.com --mcp-transport stdio --mcp-endpoint-tools hidden --force --json`: passed
- `cli-printing-press dogfood --dir /Users/debmukherjee/printing-press/library/gainsight --spec /Users/debmukherjee/printing-press/library/gainsight/spec.yaml --json`: completed with non-blocking WARN for empty default sync resources
- `cli-printing-press shipcheck --dir /Users/debmukherjee/printing-press/library/gainsight --spec /Users/debmukherjee/printing-press/library/gainsight/spec.yaml --no-live-check --json`: passed

## Live API phase5 status

Live vendor API acceptance was skipped because this environment does not have Gainsight credentials. The machine-readable skip marker is `phase5-skip.json` in this proofs directory.

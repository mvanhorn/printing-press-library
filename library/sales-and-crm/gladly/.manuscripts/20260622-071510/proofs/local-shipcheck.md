# Local shipcheck for gladly-pp-cli

This CLI was generated from a saved local OpenAPI spec and verified locally before publishing.

## Verification run

- `cli-printing-press generate --spec /Users/debmukherjee/printing-press/specs/gladly-read-openapi.yaml --name gladly --category sales-and-crm --spec-source docs --spec-url https://api.gladly.com --mcp-transport stdio --mcp-endpoint-tools hidden --force --json`: passed
- `cli-printing-press dogfood --dir /Users/debmukherjee/printing-press/library/gladly --spec /Users/debmukherjee/printing-press/library/gladly/spec.yaml --json`: completed with non-blocking WARN for empty default sync resources
- `cli-printing-press shipcheck --dir /Users/debmukherjee/printing-press/library/gladly --spec /Users/debmukherjee/printing-press/library/gladly/spec.yaml --no-live-check --json`: passed

## Live API phase5 status

Live vendor API acceptance was skipped because this environment does not have Gladly credentials. The machine-readable skip marker is `phase5-skip.json` in this proofs directory.

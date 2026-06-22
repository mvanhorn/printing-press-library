# Local shipcheck for attentive-pp-cli

This CLI was generated from a saved local OpenAPI spec and verified locally before publishing.

## Verification run

- `cli-printing-press generate --spec /Users/debmukherjee/printing-press/specs/attentive-read-openapi.json --name attentive --category marketing --spec-source docs --spec-url https://api.attentivemobile.com/ --mcp-transport stdio --mcp-endpoint-tools hidden --force --json`: passed
- `cli-printing-press dogfood --dir /Users/debmukherjee/printing-press/library/attentive --spec /Users/debmukherjee/printing-press/library/attentive/spec.json --json`: passed
- `cli-printing-press shipcheck --dir /Users/debmukherjee/printing-press/library/attentive --spec /Users/debmukherjee/printing-press/library/attentive/spec.json --no-live-check --json`: passed

## Live API phase5 status

Live vendor API acceptance was skipped because this environment does not have Attentive credentials. The machine-readable skip marker is `phase5-skip.json` in this proofs directory.

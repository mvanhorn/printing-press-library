# Local shipcheck for postscript-pp-cli

This CLI was generated from a saved local OpenAPI spec and verified locally before publishing.

## Verification run

- `cli-printing-press generate --spec /Users/debmukherjee/printing-press/specs/postscript-read-openapi.yaml --name postscript --category marketing --spec-source docs --spec-url https://api.postscript.io --mcp-transport stdio --mcp-endpoint-tools hidden --force --json`: passed
- `cli-printing-press dogfood --dir /Users/debmukherjee/printing-press/library/postscript --spec /Users/debmukherjee/printing-press/library/postscript/spec.yaml --json`: passed
- `cli-printing-press shipcheck --dir /Users/debmukherjee/printing-press/library/postscript --spec /Users/debmukherjee/printing-press/library/postscript/spec.yaml --no-live-check --json`: passed

## Live API phase5 status

Live vendor API acceptance was skipped because this environment does not have Postscript credentials. The machine-readable skip marker is `phase5-skip.json` in this proofs directory.

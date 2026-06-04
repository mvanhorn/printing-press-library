# Spec sources (historical)

The OpenAPI specs originally fed into [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) to scaffold the initial CLI. **Not load-bearing at runtime** — the binary never reads these files. They're kept for provenance: anyone tracing why a flag, type, or endpoint exists can compare against the spec we started from.

## Files

- `gorgias-crowd.yaml` — primary scaffold spec, derived from a crowd-sniff pass over the npm Gorgias SDK ecosystem plus hand-edits (auth shape, body schemas from Gorgias OpenAPI, path renames, normalized resource/endpoint names).
- `gorgias-jentic-openapi.json` — secondary OpenAPI from `github.com/jentic/jentic-public-apis`. Kept for cross-reference.

Edits to the CLI's behavior live in `internal/cli/` and `internal/mcp/`, not here.

# AllTrails Write Operations

PP-side writes are live-capable but approval-gated.

## Risk Levels

- `read` — does not mutate the account; `mcp:read-only=true`.
- `write-safe` — non-read route with low expected blast radius; still gated.
- `write-mutate` — modifies account data, uploads content, or changes saved state.
- `write-destructive` — deletes/removes account data.

## Live-Write Contract

Reads need no write gate. For every non-read route:

1. CLI help includes `[WRITES TO LIVE ALLTRAILS]`.
2. CLI execution defaults to `--dry-run` unless `--live-write` is passed.
3. The HTTP client blocks live mutating verbs unless `ALLTRAILS_PP_ALLOW_WRITES=1`.
4. MCP write tools use `mcp:read-only=false` plus `mcp:risk=<level>` metadata and default to dry-run unless `ALLTRAILS_PP_ALLOW_WRITES=1`.

This means accidental live writes fail closed even if a generated command path misses a UI-level dry-run check.

## Current Write Route

| Command | Method | Path | Risk | Mutation |
|---|---|---|---|---|
| `alltrails create` | POST | `/api/alltrails/v3/activities/upload` | `write-mutate` | yes |

Use:

```bash
alltrails-pp-cli alltrails create --stdin < activity.json
```

to preview by default, then only after explicit approval:

```bash
ALLTRAILS_PP_ALLOW_WRITES=1 alltrails-pp-cli alltrails create --live-write --stdin < activity.json
```

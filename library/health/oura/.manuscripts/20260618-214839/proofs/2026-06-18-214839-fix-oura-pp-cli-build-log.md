# Oura CLI Build Log

## What Was Built

A **generator-produced** CLI (`cli-printing-press generate`) from Oura's
official OpenAPI 3.1 document. No novel commands were hand-written beyond the
generator's standard scaffold.

### Source spec

- Official spec downloaded from
  `https://cloud.ouraring.com/v2/static/json/openapi-1.34.json` (72 paths).
- Vendored locally as `oura-spec.json` with one correction: the upstream
  `servers[0].url` is the malformed `https://api.None.com` (a Python `None`
  leaked into Oura's generated FastAPI spec); it was replaced with the
  documented host `https://api.ouraring.com`. The OAuth authorize/token URLs in
  the security scheme were already correct and were not modified.

### Generation

```
cli-printing-press generate \
  --spec oura-spec.json \
  --spec-url https://cloud.ouraring.com/v2/static/json/openapi-1.34.json \
  --spec-source official \
  --auth-preference OAuth2 \
  --name oura
```

`--auth-preference OAuth2` is required: the spec advertises both `BearerAuth`
and `OAuth2`, and the parser default would pick the simpler bearer scheme — but
Oura deprecated static personal access tokens in December 2025, so that path is
dead. With OAuth2 pinned, the generator emits the full 3-legged
authorization_code flow.

### Generated surface (absorb layer)

- `usercollection` — list + single commands for every Oura data type: daily
  sleep, sleep, sleep time, daily readiness, daily activity, heart rate,
  workouts, sessions, daily SpO2, daily stress, daily resilience, VO2 max,
  daily cardiovascular age, tags, enhanced tags, rest mode period, ring
  configuration, ring battery level, personal info.
- `webhook` — list/get/create/update/delete webhook subscriptions.
- `sandbox` — Oura sandbox routes (synthetic data for testing).
- Auth: `auth login` / `auth setup` / `auth status` / `auth logout` (OAuth2
  authorization_code, refresh-token persistence + auto-refresh).
- Agent plumbing: `agent-context`, `which`, `doctor`, `feedback`, `import`,
  `profile`, `version`.
- MCP server `oura-pp-mcp` (75 tools, readiness "full").

### Generated transcend layer

- `sync` — SQLite population (disclosed follow-up: `defaultSyncResources` empty).
- `search` — FTS5 over synced records.
- `analytics` — count / group-by / summary over synced rows.
- `workflow` — compound multi-call operations.

## Gates

All 8 generation gates passed: go mod tidy, ensure-safe golang.org/x/net,
govulncheck, go vet, go build, build runnable binary, `--help`, `version`,
`doctor`.

## Post-generation note

A generator bug was found and fixed upstream during verification: the live
dogfood matrix did not skip the top-level `login` alias (only the nested `auth`
group was in the framework-skip set), so an interactive browser command entered
the live matrix. The fix adds `login` to the dogfood framework-skip set in the
generator. The published CLI is pure generator output with no hand-edits.

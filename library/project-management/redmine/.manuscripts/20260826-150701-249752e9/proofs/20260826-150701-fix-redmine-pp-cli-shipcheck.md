# Redmine CLI Shipcheck

## Command
```
cli-printing-press shipcheck --dir <working>/redmine-pp-cli --spec <research>/redmine-openapi-6.1.3-r1.yaml --research-dir <run>
```

## Leg results (final run, `shipcheck --api-key $REDMINE_API_KEY` after Phase 5 live dogfood)
| Leg | Result | Notes |
|---|---|---|
| verify | PASS | live mode against the real instance, 99%+ pass rate after auto-fix loop |
| validate-narrative | PASS | 10/10 narrative commands resolved + full examples passed |
| dogfood | PASS | 0 dead flags, 0 dead functions, novel features 6/6 survived, MCP surface PASS |
| workflow-verify | PASS | no workflow manifest (skip) |
| apify-audit | PASS | not applicable |
| verify-skill | PASS | fixed by installing python3 (missing from devcontainer; `sudo apt-get install -y python3`) |
| scorecard | PASS | 98/100 Grade A, no unverified dimensions once verify ran live (`--api-key`, not `--env-var`, is what actually flips verify to live mode) |

**Verdict: PASS (7/7 legs passed)**

## Minor known gap (non-blocking)
Verify's auth-env check flags `REDMINE_SKIP_TLS_VERIFY` and `REDMINE_PROJECT_ID` as "required per-call env vars, missing" — both are actually optional convenience overrides (a TLS-skip toggle and a global fallback for the `{project_id}` path template var, which every real project-scoped command exposes as its own `--project-id`/`--project` flag). Verify's own fix loop logged these as "requires manual fix" (non-critical) and the leg still passed. Not a functional defect — every project-scoped command was exercised live in Phase 5 without ever needing either var set. Filed as a retro candidate (verify's auth-env classifier conflates optional convenience env vars with required auth).

## Blockers found and fixed during Phase 2/3 (not shipcheck itself, but load-bearing)
1. **`.{format}` path template bug** — the upstream `d-yoshi/redmine-openapi` spec models every collection endpoint as `/resource.{format}` with a required path parameter enum `[json, xml]`. The generator correctly treated this as a required runtime input (`REDMINE_FORMAT` env var), which broke `sync` for every list/create endpoint (91 params across 54 paths). Fixed by rewriting the spec to hardcode `.json` and dropping the `format` param — this is exactly how every real Redmine client operates.
2. **`content_type` MCP schema collision** — the `/uploads.json` endpoint declares a query param `content_type` that collides with the raw-body upload endpoint's generator-reserved `content_type` input. Fixed via `x-param-url-names` to rename the CLI-facing flag to `mime_type` while keeping the wire param as `content_type`.
3. **`c.Get` path-param handling** — hand-written novel commands must call `replacePathParam(path, name, value)` before `c.Get`; the `params` map on `Get` is query-only. Caught via live testing, not by `go build`/`go vet`.
4. **Redmine's default issue-list scope excludes closed issues** — `GET /issues.json` with no `status_id` returns open issues only (Redmine's own API default), so the plain `sync` command never populated closed issues in the local mirror. This silently zeroed `cycle-time` and half of `roadmap burndown`. Fixed by documenting `--resource-param issues-json:status_id=*` in the quickstart/troubleshoots and using it for local verification syncs.
5. **`roadmap burndown` not-found ambiguity** — a "version not found" result and a "version found but zero issues" result serialized to identical JSON (no distinguishing non-zero field). Fixed by returning a proper `usageErr` (exit 2) with an `error` + `available_versions` envelope instead of a silent empty success.

## Fix loops
1 loop (structural fixes above, discovered via live behavioral testing of all 6 novel commands against the real Redmine instance, not by shipcheck alone).

## Scorecard
98/100, Grade A. Breadth 8/10 and MCP Quality 8/10 are the only sub-10 structural dimensions; both reflect the size of the 93-endpoint surface, not a defect.

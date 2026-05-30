# make-pp-cli Build Log

## Phase 2 (generate)
- Spec authored: `research/make.yaml` — 25 resources, 50+ endpoints
- Spec auth: `api_key`, header=Authorization, prefix=Token, env_vars=[MAKE_API_TOKEN]
- Endpoint template vars: `[zone]` → `MAKE_ZONE` env var
- Generator handled Cloudflare MCP pattern (65 endpoints > 50 threshold), suppressed endpoint-mirror tools
- Removed two unwritten `newNovelScenariosListCmd`/`newNovelScenariosRunCmd` AddCommands from scenarios.go (generator stubs collided with absorbed commands; flag-level enrichment chosen instead)

## Phase 3 (build)
- Test-presence pure-logic helpers: `make_helpers.go` (blueprint walkers, remap, canonical JSON, slugify)
- Test file `make_helpers_test.go` covers: reasonFingerprint, compileReasonMatcher, walkBlueprintWebhookRefs, walkBlueprintConnectionRefs, canonicalBlueprintJSON, slugify, applyRemap
- All tests pass: `go test ./internal/cli/ -run ...` 0.274s

### Novel features built
| # | Command | File | Status |
|---|---------|------|--------|
| T1 | `scenarios run <id> --wait --timeout --poll-interval --replay` | scenarios_run.go (rewrite) | shipping |
| T2 | `blueprint sync --repo --team --all-teams --keep-metadata` | blueprint_sync.go | shipping |
| T3 | `blueprint promote --from-team --to-team --scenario --auto-suggest --map --dry-run` | blueprint_promote.go | shipping |
| T4 | `dlq inbox --team --all-teams --age --group-by --retry-all --resolve-all --match-reason` | dlq_inbox.go | shipping |
| T5 | `connections audit --team --all-teams --unused --expiring --errored` | connections_audit.go | shipping |
| T6 | `scenarios list-all --active --stale --folder` (sibling of `list`) | scenarios_all_teams.go | shipping |
| T7 | `hooks map --team --all-teams --orphans --shared` | hooks_map.go | shipping |
| T8 | `blueprint diff <id> --from --to --keep-metadata` + `blueprint restore <id> --snapshot` | blueprint_diff.go, blueprint_restore.go | shipping |

### Absorbed features (50+) — all generated
- scenarios: list, get, create, update, delete, clone, activate, deactivate, run, blueprint, logs
- executions: get
- dlqs: list, get, retry, resolve
- connections: list, get, delete, test
- hooks: list, get, delete, enable, disable, ping, learn_start, learn_stop
- data-stores: list, get, create, update, delete + records sub-resource (list/create/update/delete)
- data-structures: list, get, create, delete
- folders: list, create, update, delete
- templates: list, get
- devices: list, get, delete
- teams: list, get, create, delete
- orgs: list, get
- users: me
- keys: list, get, delete
- functions: list, get, delete
- sdk-apps: list, get
- framework: sync, search, sql (via analytics), doctor, auth, export, import, profile, agent-context, which

### Errored connection audit (deferred)
`connections audit --errored` is reserved for a future iteration once executions sync table is populated. Today it flags unused + expiring/expired only.

## Quality
- `go build ./...` PASS
- `go vet ./...` PASS
- `go mod tidy` PASS (added gopkg.in/yaml.v3 for blueprint promote's --map flag)
- novel-feature helper tests PASS (7 functions)

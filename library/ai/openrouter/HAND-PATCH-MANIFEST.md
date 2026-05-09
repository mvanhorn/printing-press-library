# Hand-Patch Manifest — openrouter-pp-cli

These files are hand-extended on top of printing-press v4.2.0 generation.
If you re-run `/printing-press openrouter` (full regen), restore from .patches/
by running .patches/restore.sh BEFORE you smoke-test the regenerated CLI.

Last polish: 2026-05-09T18:12:57Z  ·  scorecard 91/100 Grade A · verify 100%

## Files

| File | sha256 | LOC | Why hand-built |
|---|---|---|---|
| `cmd/openrouter-pp-mcp/main.go` | `1d7ae72654f2` | 40 | HTTP transport (--http :8092). Generator silently dropped mcp.transport from spec. |
| `internal/cli/usage.go` | `9b0cf8b4ea81` | 17 | Transcendence parent. |
| `internal/cli/usage_cost_by.go` | `a6a72b6a4f1e` | 219 | Transcendence: per-cron cost rollup from tier-2.0 logger. |
| `internal/cli/usage_anomaly.go` | `0e1b9c6afa73` | 158 | Transcendence: z-score cost regression alarm. |
| `internal/cli/models_query.go` | `e981aa670adc` | 211 | Transcendence: DSL → SQL parser over local catalog. |
| `internal/cli/models_query_test.go` | `c61965c884f4` | 84 | Unit tests for the DSL parser. |
| `internal/cli/providers_degraded.go` | `402180ec1802` | 141 | Transcendence: set-diff vs prior snapshot. |
| `internal/cli/generation_explain.go` | `82ce5a6e39c7` | 156 | Transcendence: cost-vs-cheapest-provider delta. |
| `internal/cli/key_eta.go` | `61429dcc77ea` | 118 | Transcendence: weekly-cap projection. |
| `internal/cli/budget.go` | `2afe166ced11` | 64 | Transcendence parent. |
| `internal/cli/budget_set.go` | `f52ac408f19a` | 39 | Transcendence: per-cron weekly budget contracts. |
| `internal/cli/budget_check.go` | `fb9f98658ae2` | 67 | Transcendence: budget gate (--dry-run safe). |
| `internal/cli/endpoints_failover.go` | `103c6c99fe06` | 121 | Transcendence: provider ranking by status+price+latency. |

## Restore after regen

```bash
/Users/rick/printing-press/library/openrouter/.patches/restore.sh
cd /Users/rick/printing-press/library/openrouter && go build -o ./openrouter-pp-cli ./cmd/openrouter-pp-cli && go build -o ./openrouter-pp-mcp ./cmd/openrouter-pp-mcp
./openrouter-pp-cli doctor
launchctl kickstart -k gui/$(id -u)/com.local.openrouter-mcp
```

## Tracked spec config (regen sets these but generator ignores them as of v4.2.0)

```yaml
mcp:
  transport: [stdio, http]
  orchestration: code
  endpoint_tools: hidden
```

## Snapshot-vs-live drift check

```bash
for f in cmd/openrouter-pp-mcp/main.go internal/cli/{usage,usage_cost_by,usage_anomaly,models_query,models_query_test,providers_degraded,generation_explain,key_eta,budget,budget_set,budget_check,endpoints_failover}.go; do
  diff -q "/Users/rick/printing-press/library/openrouter/$f" "/Users/rick/printing-press/library/openrouter/.patches/$f" || echo "DRIFT: $f"
done
```

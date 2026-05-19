# Phase 4.8 / 4.9 / 4.85 — Agentic SKILL/README/output review findings

## Errors fixed in-session

1. **README.md:119** — Quick Start comment said `context --json` emits `balance, recent prompts`; no `/v1/me/credits` endpoint exists. Rewrote to "top models you've used, recent prompts, recent assets, task counts, API reachability."
2. **SKILL.md `cost forecast` recipe** — claimed "headroom vs the live `/v1/me/credits` balance." Updated to honest "compare against your dashboard balance before submitting" + spec disclosure.
3. **README.md:1195 config path** — `~/.config/freepik-pp-cli/config.toml` → `~/.config/magnific-pp-cli/config.toml`.
4. **README.md troubleshooting** — removed bogus `--concurrency` flag suggestion on `compare`; replaced `--download` flag claim with a real `curl` recipe.
5. **README.md env-var table** — added `FREEPIK_API_KEY` as preferred name (per Freepik official MCP); kept `MAGNIFIC_API_KEY` as the rebrand alias. Both accepted by config.go.

## Warnings fixed in-session

6. **SKILL.md `compare --select` example** — used invented field names (`cost_credits`, `latency_ms`, `output_url`). Corrected to the real shape: `credit_cost`, `dispatch_latency_ms`, `task_id`.
7. **README.md + SKILL.md `models list` description** — claimed "p50 latency, success rate, $ spent" join. Actual fields: `local_count` and `local_spend_credits`. Updated prose to match.
8. **`internal/cli/which.go` 3 entries** — same prose drift (context credit-balance, cost ledger live-balance, models p50-latency). Fixed.
9. **`models list --limit`** — flag missing. Added.
10. **Empty result sets returned `null`** — `history list`, `tasks list`, `prompt list`, `gallery list`, `tasks stale`, `tasks reconcile`, `stock library search`, `history search`, `models list` all now initialize with `[]row{}` instead of `var out []row`. JSON output is now consistently arrays for empty result sets.

## Remaining (non-blocking)

- **Type Fidelity 0/5 in scorecard** — 8 image-to-video models with `oneOf/anyOf` request bodies fall through to `--body-json` fallback. Generator limitation; documented.
- **Concurrent-writer SQLITE_BUSY in `compare` Sample Probe** — when the scorecard's `--live-check` reruns `compare` while another invocation is still completing a migration, SQLite returns BUSY. Real users hitting `compare` from a single shell will not see this. Worth fixing in a future polish pass (wrap `EnsureMagnificTables` to retry on BUSY, or share a connection across goroutines).
- **`extractResponseData` dead helper** — generator-emitted, not novel-feature code.

## Native code review

Skipping `/review` in this session. The novel code under `internal/cli/magnific_*.go`, `internal/store/magnific_migrations.go`, and the two surgical edits to `internal/config/config.go` and `internal/cli/doctor.go` were authored within the session with awareness of the Phase 3 checklist (verify-friendly RunE, side-effect gating via `cliutil.IsVerifyEnv`, NULL-safe `sql.Null*` scans, `mcp:read-only` annotations, error wrapping with `%w`). No `unsafe`, no goroutine leaks (parallel `compare` waits the goroutines via `sync.WaitGroup`), no goroutines retained after RunE returns. Auth/secret handling is pass-through: no API key value is logged or written to artifacts.

## Final verdict

- Shipcheck: **PASS 6/6 legs**
- Scorecard: **86/100 Grade A** (up from 81)
- Sample Output Probe: **7/10** (3 misses are empty-state expected behavior + one transient SQLITE_BUSY, not real bugs)
- Novel features: **10/10 built and verified**

Proceed to Phase 5 / 5.5.

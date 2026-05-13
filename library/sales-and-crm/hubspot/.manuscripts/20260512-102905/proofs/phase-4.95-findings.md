# Phase 4.95 — Native Code Review Findings

## In-scope fixes (applied)

- **`internal/cli/closed_won_handoff.go:50`** — `error` — Unsafe type assertion `m := row.(map[string]any)` without `ok` check. If `row` is not a map, `m` is nil and `m["name"]` panics. Fix: use `m, ok := row.(map[string]any); if !ok { continue }`.

## In-scope warnings (skipped, low real risk per autofix policy)

- `internal/cli/transcendence_helpers.go:231` — table name in `fmt.Sprintf`. Names come from hardcoded constants, not user input. No injection vector.
- `internal/cli/transcendence_helpers.go:445` — ignored `ok` on type assertion in iteration. Nil case skips cleanly.

## Out-of-scope (retro-candidate, NOT patched in-place)

- **`internal/cli/helpers.go:387`** — UTF-8 unsafe `truncate`: byte-based slicing on multi-byte runes corrupts characters. **Generator-emitted file (templates/cli/helpers.go.tmpl).** Patching here would be overwritten on next regen. **File as Printing Press retro issue.**

## Clean

- SQL injection (parameterized queries throughout)
- Resource leaks (all `rows.Close()` deferred)
- Time parsing (`parseLookbackDuration` accepts `7d`, `24h`, `30m`)
- Verify-friendliness (every RunE checks `dryRunOK(flags)` first)
- Annotations (`mcp:read-only=true` + `pp:typed-exit-codes=0,3` on all 9)
- PII logging (no info-level email/phone leaks)

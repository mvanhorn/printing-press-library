# Phase 4.95 Local Code Review Findings — Workiz CLI

Review path: direct reviewer-subagent dispatch (correctness, security, maintainability personas) via the Agent tool, run in parallel against the hand-written files (`internal/cli/novel_shared.go`, the 6 novel command files, `internal/config/config.go`, `internal/client/client.go`). `internal/mcp/cobratree/` and `internal/cliutil/` excluded as generator-reserved. Working directory is not a git repo, so subagent dispatch was used instead of a diff-based review skill.

## Autofix summary

All findings below were fixed in-place in this session (no PR yet exists — working copy only). 1 round; all in-scope findings cleared.

## Findings and outcomes

### Security (P0 — fixed)
1. **`doctor` leaked the raw API token in `base_url`** (cleartext in `--json` and human output). Fixed by adding `Config.DisplayBaseURL()` (masks to last 4 chars) and using it in `doctor.go` instead of the raw field.
2. **`--dry-run` printed the injected `auth_secret` POST-body field in cleartext** while the URL/query/Authorization-header were correctly masked in the same preview. Fixed by routing the pretty-printed body through the existing `maskCredentialText` before printing in `client.go`'s `dryRun()`.

### Correctness (critical + high — fixed)
3. **CRITICAL: token rotation and logout corrupted the persisted `base_url`.** Root cause: the original hand-edit mutated `cfg.BaseURL` in place inside `config.Load()`, and `BaseURL` is a persisted TOML field — so saving after that mutation wrote the token-embedded URL to disk, and the next `Load()` would append a *second* token on top (`.../TOKEN1/TOKEN2`), while logout never actually cleared the old token from the persisted URL. Fixed by re-architecting: reverted the `Load()` mutation, added `Config.EffectiveBaseURL()` (computed on demand, never persisted) that does the token-folding, and updated `client.New()` to call it instead of reading `cfg.BaseURL` directly. Verified via a full rotation+logout regression test (`internal/config/config_test.go`) and a live CLI reproduction (`auth set-token` twice, then `auth logout`, inspecting `config.toml` after each step).
4. **HIGH: `snippetAround` could panic on Unicode input.** `strings.ToLower` can change a rune's UTF-8 byte length (e.g. `İ` → `i`), desyncing the byte offset computed against the lowered copy from the original string being sliced — reproduced as a real panic on job/lead notes containing certain non-ASCII characters. Fixed with a rune-safe case-insensitive scan operating entirely in `[]rune` space. Added a regression test with the exact reproduction case.

### Maintainability (P1/P2 — fixed)
5. **P1: all 6 novel commands hand-rolled the JSON-vs-table decision**, omitting `--compact`/`--select` support that the canonical `wantsHumanTable` helper already provides. Fixed by replacing the bespoke condition with `!wantsHumanTable(...)` in all 6 files. Verified `--select`/`--compact` now work on a synthetic-DB smoke test.
6. **P1: `digest`'s missing-mirror JSON fallback returned `[]` instead of its real object shape** (`{since, jobs, leads}`), a schema break on first run. Fixed as part of finding 7's extraction.
7. **P2: the missing-mirror stat+hint+JSON-fallback block was duplicated near-identically across all 6 commands**, with a third, subtly different JSON-decision gate. Extracted into a shared `checkNovelMirror` helper in `novel_shared.go`, parameterized by the correct empty-value type per command — this also structurally prevents the shape bug in finding 6 from recurring.
8. **P2/P3: test coverage gaps** — added cases for `parseWorkizTime`'s untested RFC3339 branch, `wzComments`/`flexibleID`'s untested unrecognized-shape fallback branches, and a sandwiched (both-ellipsis) `snippetAround` case, plus the Unicode panic regression.
9. **P3: dead leftover `var _ = strings.ReplaceAll` in `config.go`** — removed.
10. **Stale digest description/Short text still said "jobs, leads, and clients"** in the Go source (`digest.go`'s `Short` field) even after the Phase 4.9 research.json fix — that fix only covered the generator-synced README/SKILL copies, not this hand-written file. Fixed directly.

### Not fixed (documented, low severity/confidence)
- `parseWorkizTime` assumes UTC for zone-less Workiz timestamps; if Workiz's wire format actually represents account-local business time, `--week`/`--since` windows could be off by the account's UTC offset. Could not verify without a live account — flagged as a residual risk for future live-dogfood verification.
- `team_bottleneck.go` groups by crew member `Name`, not a stable ID; two techs with identical names would merge. Low confidence/severity — Workiz's own assign/unassign API also only accepts crew by name, so this mirrors the upstream API's own limitation.
- `job_audit`'s zero-price check can't distinguish "field missing" from "legitimately free job" — minor semantic nuance, not a crash or data-loss bug.
- `openNovelStore`'s internal empty-path fallback is currently unreachable dead code given all 6 call sites already resolve the path first — harmless, left as a defensive default.
- Redundant `Authorization` header sent alongside the path token and body secret (leftover from the generic `api_key`/`in: header` auth declaration) — confirmed harmless (Workiz ignores unrecognized headers) but flagged by the security reviewer as worth a closer look against real API behavior; deferred since removing it touches doctor/auth flow surfaces without a clear benefit.

### Out of scope
`internal/cliutil/` and `internal/mcp/cobratree/` were not reviewed (generator-reserved).

## Convergence outcome
All in-scope findings cleared in round 1 (`go build`, `go vet`, `go test ./...` all clean after fixes; shipcheck rerun 6/6 PASS, Grade A 92/100 unchanged).

## Review path chosen
Direct reviewer-subagent dispatch via the Agent tool: `compound-engineering:ce-correctness-reviewer`, `compound-engineering:ce-security-reviewer`, `compound-engineering:ce-maintainability-reviewer`, run in parallel.

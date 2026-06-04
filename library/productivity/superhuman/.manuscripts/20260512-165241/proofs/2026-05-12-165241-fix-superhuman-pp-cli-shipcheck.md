# Superhuman CLI Shipcheck Report

## Verdict: HOLD

Shipcheck umbrella exited non-zero. 2 of 6 legs failed; both because the README/SKILL.md document the full 12-feature transcendence manifest, but Phase 3 only hand-built the absorbed-scaffolding fixes (auth setup details + threads list/get body shapes).

## Shipcheck Results

| Leg | Result | Notes |
|-----|--------|-------|
| dogfood | PASS | All command-tree checks pass; novel_features_built synced |
| verify | PASS | 100% (15/15 passed, 0 critical), all 8 quality gates green |
| workflow-verify | PASS | No workflow manifest present (skipped cleanly) |
| verify-skill | FAIL | 22 errors: SKILL.md references 12 transcendence commands (`unified`, `digest`, `opens`, `returning`, `awaiting`, etc.) and their flags that don't exist in `internal/cli/*.go` |
| validate-narrative | FAIL | 9 missing commands, 2 failed examples — same root cause |
| scorecard | **80/100 Grade A** | Output Modes 10/10, Auth 10/10, Error Handling 10/10, Doctor 10/10, Agent Native 10/10, MCP Quality 10/10, Local Cache 10/10 |

## Scorecard gaps
- `mcp_token_efficiency 4/10` — would improve with the recommended Cloudflare-pattern MCP opt-in for >30-tool surfaces
- `workflows 4/10` — no compound workflows (we have intent but not implementation)
- `insight 2/10` — no novel commands shipped (the 12 transcendence features are docs-only)

## What works today
- Generated scaffold passes all 8 quality gates (`go mod tidy`, `govulncheck`, `go vet`, `go build`, binary, `--help`, `version`, `doctor`)
- `auth set-token <JWT>` accepts a Firebase JWT; `auth setup` documents exactly how to lift one from DevTools
- `threads list` sends correct `{filter:{}, limit, offset}` body — matches Superhuman's real API
- `threads get <id>` sends correct `{filter:{threadId}, limit:1}` body
- `drafts list/write`, `messages send`, `reminders create/cancel`, `attachments upload`, `ai`, `teams`, `users` are all scaffolded but request bodies have not been validated against the real API yet
- `doctor`, `sync`, `sql`, `search`, MCP server bundle, `--json`/`--select`/`--csv` flags all generator-standard
- README + SKILL.md correctly highlight the 12 novel features as "Unique Capabilities" — they read as the intended product, but the underlying commands don't exist yet

## What's missing (for the FULL manifest verdict)
1. **auth login --chrome** with CDP attach + automatic Firebase refresh-token exchange. Workaround today: re-paste JWT from DevTools when stale.
2. **Correct request bodies on `drafts write`** — needs the full `userdata.writeMessage` `{writes:[{path,value}]}` envelope with the DraftValue schema.
3. **2-step `messages send`** — needs `/messages/send/log` preamble then `/messages/send`.
4. **The 12 transcendence commands** — all documented in SKILL.md/README/research.json `novel_features`, but no Go code yet:
   - N1 auth login --chrome (the headline durable-auth feature)
   - N2 unified (cross-account inbox)
   - N3 awaiting (reply-tracker)
   - N4 opens (Read-Status leaderboard)
   - N5 latency (reply analytics)
   - N6 triage (YAML rule engine)
   - N7 returning (snooze radar)
   - N8 digest (markdown rollup)
   - N9 snippet use --vars (template engine)
   - N10 send --undo + unsend (hold-and-release queue)
   - N11 remind list (queryable reminders)
   - N12 thread brief (mechanical summary)

## Recommendation

Run `/printing-press-polish superhuman-pp-cli` to drive the diagnostic-fix-rediagnose loop. The polish skill can:
- Sync SKILL.md / README / `.printing-press.json` to match what's actually built (removing references to unbuilt commands so verify-skill passes)
- Or, in a longer pass, hand-build the missing transcendence commands

Alternatively, run `/printing-press-retro` to capture the systemic learning: the generator built a scaffold that the spec described, but the spec only described 10 endpoints — the 12 transcendence features in research.json were never wired through to generated code, leaving SKILL.md promising what wasn't built. This is a retro candidate for the Printing Press itself (research.json novel_features should perhaps map to generator-scaffolded command stubs, not just documentation).

# Phase 4.85 agentic output review — conduyt-crm-pp-cli (2026-09-16)

status: WARN (4 findings, all warnings under the Wave B policy; all four are being fixed before dogfood rather than shipped as gaps)

1. send-check read an unsynced local mirror and reported checked=0 as a passing verdict (exit 0). Fix: auto mode falls back to the live contacts route unless the mirror has a contacts sync_state and rows.
2. send-check swallowed a DNC-lookup 401/403 into a warning and still marked contacts ok. Fix: authErr (exit 4) on 401/403, apiErr (exit 5) otherwise; no go verdict without a DNC answer.
3. send-check --agent stamped meta.source "live" for mirror rows. Fix: agentSource local on the mirror branch.
4. verify-line-type --estimate and hours-audit reported zeros from an unsynced mirror with only a stderr hint. Fix: top-level synced:false + hint in JSON; estimate falls back to a live count (capped, partial flag); hours-audit keeps exit 0 (live-matrix happy-args).

Reviewer note: 5 of 8 live samples were 401 inside the review's own shell (no CONDUYT_API_KEY there); shipcheck 4 with the key had 8/8 passing.
Fixes delegated to Codex (scratchpad pp91-codex-fix16.txt); re-verified by go test + shipcheck 5.

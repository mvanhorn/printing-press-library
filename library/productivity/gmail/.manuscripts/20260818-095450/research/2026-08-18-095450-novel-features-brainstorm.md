# Gmail novel-features brainstorm — subagent audit trail (2026-08-18)

Three-pass output (customer model → 16 candidates → adversarial cut). Survivors
went into the absorb manifest's transcendence table; this file preserves the
full reasoning for retro/dogfood debugging.

## Customer model

**the assistant — the always-on personal-assistant agent.** Today: no Gmail surface at all — popular Gmail MCP servers carry send tools her transitive-barrier rules forbid. Ritual: the operator's cleanup campaign (summarize all categories, unsubscribe, trash, sort, folders) driven by texted instructions with confirm-gated replies. Frustration: answering from conversation needs instant local state and incremental answers — re-listing the mailbox per question is slow and re-reports what the operator already saw.

**the operator — the operator at the desk.** Today: two Gmail web tabs, one per account; cleanup is search→select-all→trash repeated per query per account; unsubscribing is one link at a time. Can't answer: who mails me most across both accounts, which subscriptions I never read, what eats my storage, whether last month's unsubscribes stuck. Frustration: bulk actions are scary (no preview/ledger/undo) and unsubscribe compliance is invisible.

**The morning sweep — the unattended scheduled hygiene job.** Today: doesn't exist; a cron substitute would re-derive everything and double-report daily. Ritual: sync both accounts, compute what's new, run standing cleanup queries into a preview plan, report to the phone; applies only after human confirm. Frustration: no durable state between runs, no named recurring rules, must never prompt.

## Candidates (pre-cut) — 16 generated

1. Mailbox delta report (`delta`) — advance. 2. Unsubscribe compliance check (`unsub verify`) — advance, verifiability flag: dogfood seeds a ledger fixture. 3. Cross-session decision queue (`review`) — descoped into rules/plans. 4. Subscription cadence view (`subs`) — re-proposal risk vs manifest #6. 5. Storage attribution (`storage report`) — advance. 6. Awaiting-reply threads — off-NOI. 7. Label taxonomy audit — adjacent to #8. 8. Saved cleanup rules (`rules`) — advance (local-side; never creates Gmail filters, settings pruned). 9. Mailbox activity audit — occasional cadence. 10. Cross-account overlap — soft cadence. 11. Sort suggestions (`sort suggest`) — advance (pure counting, LLM-free). 12. Biggest items (`big`) — thin slice. 13. Mailbox health score (`score`) — advance. 14. Resumable bulk apply — implementation, not feature. 15. Profile preflight (`doctor`) — press furniture. 16. Trash undo-window watch (`trash report`) — advance.

Hard-constraint scan: no candidate sends, drafts, touches settings/filters, or permanently deletes; no candidate makes an LLM call.

## Survivors (scores = DomainFit+UserPain+Feasibility+Backing)

1. `unsub verify` 10/10 (3+3+2+2) — ledger × post-unsub arrivals; nothing in the landscape verifies compliance.
2. `delta` 9/10 (3+3+1+2) — checkpoint-diff report; the sweep's first question every morning.
3. `storage report` 9/10 (3+2+2+2) — attribution with emit-ready cleanup queries.
4. `rules add/list/run` 9/10 (3+3+1+2) — local rulebook through the plan→confirm engine; the compliant substitute for pruned server-side filters.
5. `sort suggest` 9/10 (3+2+2+2) — majority-label concentration → batchModify plans.
6. `trash report` 8/10 (3+2+2+1) — undo-window countdowns vs Gmail's 30-day purge.
7. `score` 7/10 (3+2+1+1) — hygiene metrics snapshotted for campaign progress.

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Cross-session decision queue | approval state machine is an application; plan→confirm + agent conversation covers it | rules run |
| Subscription cadence view | re-proposal of manifest #6 / Build Priority 5 | unsub verify |
| Awaiting-reply threads | off-mission (triage, not cleanup), 4/10 | delta |
| Mailbox activity audit | occasional cadence, covered surface | delta |
| Cross-account overlap | fails weekly-use after initial pass; becomes a flag on unsub audit | unsub verify |
| Biggest items | thin renaming of covered queries | storage report |
| Resumable bulk apply | hardening of manifest #5, not a feature | trash report |
| Profile preflight | press furniture; consent-verify already manifest #4 | score |
| Label taxonomy audit | adjacent to manifest #8; monthly cadence | sort suggest |

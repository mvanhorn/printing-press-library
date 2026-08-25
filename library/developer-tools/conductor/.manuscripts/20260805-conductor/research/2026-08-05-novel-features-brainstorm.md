## Customer model

### Solo Linear-to-Conductor operator

They launch bounded implementation work from Linear, monitor sessions, collect evidence, and archive completed work. Their main problem is deciding whether `idle` means completion or a queued-message race.

### Engineering lead supervising an agent fleet

They review active sessions each workday, intervene, and verify that steering or cancellation took effect. The API exposes individual status but no fleet exception view.

### Analyst and cleanup reviewer

They preserve a deterministic audit trail and clean up inactive sessions and workspaces. Evidence and queued-message risk currently require several unrelated reads.

## Candidates (pre-cut)

1. `fleet` — attention-ranked cross-workspace view. Keep.
2. `stuck` — detect sessions without status or transcript progress. Keep.
3. `queue-audit` — flag queued or unacknowledged work and false-idle risk. Keep.
4. `cleanup-plan` — non-mutating archive plan with activity and queue evidence. Keep.
5. `audit-export` — deterministic session evidence bundle. Keep.
6. `timeline` — chronological workspace stream. Cut as overlap with `audit-export`.
7. `orphans` — inactive workspace filter. Cut as overlap with `cleanup-plan`.
8. `linear-dispatch` — launch from Linear. Cut because ENG-526 owns the external integration.
9. `summarize` — semantic transcript summary. Cut because it needs an LLM and is not mechanically verifiable.
10. `watch-daemon` — persistent monitoring. Cut as background infrastructure outside bounded CLI scope.
11. `open` — desktop deep-link wrapper. Cut as a thin shell wrapper.
12. `cancel-confirm` — cancel and wait. Cut because cancellation confirmation belongs in the absorbed lifecycle workflow.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How it works | Long description |
|---|---|---|---|---|---|---|
| 1 | Fleet exceptions | `fleet` | 9/10 | hand-code | Joins local projects, workspaces, sessions, messages, and status observations into an attention-ranked fleet view. | none |
| 2 | Stalled-session detector | `stuck` | 8/10 | hand-code | Uses local session-status history and transcript cursors to find sessions past an inactivity threshold. | Use this for stalled sessions; use `fleet` for the full operational view. |
| 3 | Queued-message risk audit | `queue-audit` | 9/10 | hand-code | Correlates ordered messages, transcript cursors, and status observations to flag false-idle and unacknowledged work. | Use this for queue risk; use `cleanup-plan` for a complete cleanup review. |
| 4 | Safe cleanup plan | `cleanup-plan` | 8/10 | hand-code | Computes non-mutating archive candidates and exact commands from local activity and queue evidence. | Use this for a cleanup plan; use `queue-audit` for a focused queue check. |
| 5 | Session audit export | `audit-export` | 8/10 | hand-code | Joins session metadata, messages, status observations, cursors, and deep links into a deterministic evidence bundle. | none |

### Killed candidates

| Feature | Kill reason | Closest survivor |
|---|---|---|
| Workspace activity timeline | Overlaps the more useful review artifact. | `audit-export` |
| Orphaned workspace finder | Subsumed by queue-aware cleanup analysis. | `cleanup-plan` |
| Issue dispatcher | Requires Linear and belongs in ENG-526. | none |
| Semantic transcript summary | Requires an LLM and unverifiable judgment. | `audit-export` |
| Background fleet watcher | Requires persistent process and alert infrastructure. | `fleet` |
| Open session in desktop | Thin deep-link shell wrapper. | `audit-export` |
| Cancellation receipt | Already covered by cancellation plus monitor behavior. | `stuck` |

## Scope disposition

The survivors are preserved for later work. ENG-525 already commits to six hand-written orchestration workflows and does not include these fleet-analysis commands.

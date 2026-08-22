# Jules CLI - Absorb Manifest (Validated by Project A (fleet automation) & Project B (knowledge base))

## Absorbed Features (Match or Beat Everything)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List sources (repos) | @google/julius | `jules-pp-cli source list --json` | Offline mode, structured output |
| 2 | List sessions | @google/julius | `jules-pp-cli session list --json --limit N` | Local sync, search, time filters |
| 3 | Get session details | @google/julius | `jules-pp-cli session get <id> --json` | Full artifact history, offline |
| 4 | Create session | @google/julius | `jules-pp-cli session create --prompt "..." --source <id> --dry-run` | Plan preview, --dry-run before commit |
| 5 | Monitor session (activities) | @google/julius | `jules-pp-cli session watch <id>` (stream updates) | Polling + local cache, --json mode |
| 6 | Send message to session | @google/julius | `jules-pp-cli session message <id> "feedback"` | Approval gates, conditional logic |
| 7 | Approve plan | @google/julius | `jules-pp-cli session approve <id> --dry-run` | Preview changes, rollback option |
| 8 | Delete/cancel session | @google/julius | `jules-pp-cli session delete <id>` | Soft-delete with archive option |
| 9 | GitHub PR integration | @google/julius, google-julius-workflow | `jules-pp-cli session <id> --github-auto-open` | One-click PR workflow |
| 10 | Linear/issue context | google-julius-workflow | `jules-pp-cli session create --linear-issue <id>` | Auto-populate from issue |

## Transcendence Features (Validated by Real Projects)

### Critical (Validated by BOTH Project A (fleet automation) & Project B (knowledge base))
These features solve blocking pain points in both repos:

| # | Feature | Command | Validation | Impact |
|---|---------|---------|-----------|--------|
| 1 | Quota-aware Dispatch Throttling | `jules-pp-cli session create ... --quota-safe` | Project A (fleet automation) #585, Project B (knowledge base) ADR-019 | Prevents `400 FAILED_PRECONDITION`; soft-warn instead of block |
| 2 | Continuous Session Monitoring | `jules-pp-cli monitor --reconcile` | Project A (fleet automation) #1272, Project B (knowledge base) checkpoint registry | Built-in reconciliation (vs. external cron); detects stalled state |
| 3 | Working Tree Checkpoint/Restore | `jules-pp-cli session <id> --checkpoint save/restore` | Project A (fleet automation) #677, Project B (knowledge base) ADR-037 | Prevent mid-session work loss; write-race safety |
| 4 | Automated Zombie Session Archival | `jules-pp-cli archive --stale 7d` | Project A (fleet automation) quota pattern, Project B (knowledge base) state drift | Auto-free quota from abandoned sessions |
| 5 | Pre-flight Conflict Detection | `jules-pp-cli session create ... --check-conflicts` | Project B (knowledge base) parallel dispatch (ADR-019), Project A (fleet automation) #677 | Detect merge conflicts BEFORE parallel dispatch; prevent rework |

### High Priority (Project A (fleet automation) specific, high impact)

| # | Feature | Command | Validation | Impact |
|---|---------|---------|-----------|--------|
| 6 | Pre-submission Diff Validation | `jules-pp-cli session <id> --validate-diff` | Project A (fleet automation) #677 (3 no-op PRs/24h) | Reject empty-diff PRs before autoPr fires |
| 7 | Workflow Trigger Chaining | `jules-pp-cli trigger chain --cron 6am --workflow dispatch` | Project A (fleet automation) #663, #1258 | Enable cron + workflow_run composition without manual dispatch |

### Medium Priority (Project B (knowledge base) specific, enables learning)

| # | Feature | Command | Validation | Impact |
|---|---------|---------|-----------|--------|
| 8 | Persona Memory Learning | `jules-pp-cli persona record <name> <outcome>` | Project B (knowledge base) ADR-037 | Capture Jules outcomes; reuse patterns across runs |

## Summary

**Absorb scope:** 10 features (100% table-stakes from competitors)  
**Critical (both repos):** 5 features (quota, monitoring, checkpoints, conflict detection, archival)  
**Project A (fleet automation):** 2 features (diff validation, trigger chaining)  
**Project B (knowledge base):** 1 feature (persona memory)  
**Total shipping scope:** 18 features  
**Hand-code count:** 8 features (critical + project-specific)  
**Generated endpoint count:** 10 (API endpoints auto-generated)  

## Validation Summary

✓ **10 absorbed features** — verified against 4 competitors (@google/julius, google-julius-workflow, Gemini CLI Jules, julius-cli)  
✓ **5 critical features** — validated by BOTH Project A (fleet automation) & Project B (knowledge base) as blocking pain points  
✓ **2 Project A (fleet automation) features** — directly solve no-op PR & fleet-dispatch issues  
✓ **1 Project B (knowledge base) feature** — enables outcome learning across runs  
✓ **Scope is bounded** — All 18 features map to real, documented problems in production use  

**Ready for Phase 2 (Generate) with high confidence this CLI will ship value.**

## Next Phase: Ready to Approve?

This 18-feature manifest is grounded in:
- Competitor analysis (what we must match)
- Project A (fleet automation)'s real pain points (fleet automation, no-op PRs, quota)
- Project B (knowledge base)'s real pain points (conflict detection, state recovery, persona learning)

All features solve documented issues. No theoretical fluff.

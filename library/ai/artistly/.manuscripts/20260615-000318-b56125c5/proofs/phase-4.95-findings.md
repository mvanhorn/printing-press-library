# Phase 4.95 Local Code Review — findings

Reviewer: general-purpose subagent (security+correctness), scope = hand-authored internal/cli/*.go.

## Autofixed in-place (1 round)
- HIGH #1: pollForNewDesigns / live commands read the 5-minute response cache → poll never saw completion, --wait/--download busy-spun to timeout. Fixed: fetchPersonalDesigns now uses GetNoCache (also resolves MED #2 stale-snapshot for delete/move/redo/export/download).
- LOW #3: batch pacing time.Sleep replaced with ctx-aware select (responsive to cancellation).
- LOW #4: removed dead loop in extractDataPageProps.

## Checked, NOT bugs (reviewer-confirmed)
- Path traversal / filename injection: SAFE — sanitizeFilename strips / \ : * ? " < > | over the whole rendered name; slugify restricts {prompt} to [a-z0-9-]; filepath.Join stays within dir.
- quota.go type assertion: safe (same branch sets it).
- FolderID *int: guarded before deref; marshals to null.
- Resource leaks: response bodies, file handles, store handles all closed on all paths.
- Credential leakage: no token/cookie values printed; only cookie names in errors.
- Context propagation: all network calls take ctx; poll selects on ctx.Done().

## Convergence
Findings cleared in round 1 (all 4 autofixed). No template-shape or out-of-scope findings.

## Review path
Direct subagent dispatch (general-purpose) — harness has no PR yet; working-dir review via Agent tool.

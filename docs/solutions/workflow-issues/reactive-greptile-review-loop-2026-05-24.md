---
title: Reactive Greptile review loops burn tokens and signal missing upfront code review
date: 2026-05-24
category: workflow-issues
module: printing-press-library
problem_type: workflow_issue
component: development_workflow
severity: high
applies_when:
  - Hand-patching generated CLI code (novel commands, PATCH-annotated fixes)
  - Pushing code to a PR gated by automated code review (Greptile)
  - Writing SQL queries against a schema you didn't design
tags:
  - greptile
  - code-review
  - printing-press
  - novel-commands
  - token-efficiency
  - review-loop
---

# Reactive Greptile review loops burn tokens and signal missing upfront code review

## Context

During a multimail CLI reprint (PR #823), hand-written PATCH code for 6 novel analytics commands went through **9 fix commits across 17 Greptile reviews** to reach 4/5. Each cycle was: push → wait 3-5 min for Greptile → read finding → fix one thing → push → repeat. The session burned significant context and tokens on a task that should have been one or two commits.

The root issue wasn't Greptile being picky — every finding it surfaced was a real bug the agent could have caught by reading the code before pushing.

## Guidance

### Before pushing any hand-patched code to a Greptile-gated PR:

1. **Verify column/field semantics against the schema.** Don't assume a column name means what it sounds like. The `parent_id` column in `mailboxes_emails` was the FK to the parent mailbox, not a thread-parent reference. The `$.resource_id` field in audit-log entries stored oversight-item IDs, not mailbox IDs. Both assumptions led to dead code that Greptile caught.

2. **Trace data flow from sync to query.** When a novel command reads from a synced table, read the sync code to understand what transformations happen during ingestion. The `syncDependentResource` function unconditionally overwrote `parent_id` with the mailbox UUID, breaking every query that assumed it held thread data.

3. **Check the API spec for the actual field names.** The multimail API returns `thread_id` in `EmailSummary` for conversation threading. Reading the spec before writing thread-detection SQL would have avoided the parent_id misunderstanding entirely.

4. **Audit LIKE patterns for false-positive matches.** `LIKE '%approve%'` matches "unapprove" and "pre_approval_check". Anchor patterns at the start (`LIKE 'approve%'`) or use NOT LIKE exclusions.

5. **Batch all fixes in one commit.** If you find one issue, assume there are more in the same code. Read ALL the novel command files, trace all their queries, and fix everything before pushing. The user explicitly expressed frustration at the one-fix-at-a-time approach.

### The anti-pattern (what happened):

```
Push → Greptile finds A → Fix A → Push → Greptile finds B → Fix B → Push →
Greptile finds C (which was visible in the same file as A) → Fix C → Push → ...
```

9 commits. 17 reviews. Each review costs 3-5 minutes of wall time plus tokens for the poll loop.

### The correct pattern:

```
Write patches → Self-review all files → Check spec for field semantics →
Trace sync data flow → Fix everything → Push once → Greptile confirms 4/5
```

1-2 commits. 1-2 reviews. Same outcome.

## Why This Matters

- **Token cost scales linearly with review cycles.** Each push triggers CI (5 checks), a Greptile review (3-5 min), and a poll loop. Nine cycles = 45 minutes of pure waiting plus the token cost of monitoring.
- **User trust erodes.** The user said "why are you struggling so bad with this" and "why are you having these problems if you're using the printing press" — directly caused by watching a stream of single-finding fix commits.
- **Greptile is not your code reviewer.** It's a gate, not a pair programmer. Using it as your primary review loop is like using CI test failures to debug — technically works, maximally expensive.

## When to Apply

- Any time you're hand-editing generated code (PATCH-annotated fixes to printed CLIs)
- Any time a PR is gated by automated code review (Greptile, CodeRabbit, etc.)
- Any time you're writing queries against a data layer you didn't author — read the schema, read the sync/ingestion code, read the API spec
- Especially when the user has already expressed frustration at iterative fixing

## Examples

### Before (what happened — 9 commits):

| Commit | Fix | Could have been caught by |
|--------|-----|--------------------------|
| 1 | Merge conflicts | Checking upstream before pushing |
| 2 | Remove stale upstream files | Same merge review |
| 3 | Rewrite module path | Reading CI requirements first |
| 4 | Add missing `printer` field | Reading CI requirements first |
| 5 | Add `$.mailbox_id` to trust_status | Reading oversight_velocity for consistency |
| 6 | Align latency JOIN mailbox filter | Same file as commit 5 |
| 7 | Fix isList + tighten LIKE patterns | Reading data_source.go + basic SQL review |
| 8 | Remove dead `$.resource_id` check | Checking what resource_id actually stores |
| 9 | Use `$.thread_id` instead of parent_id | Reading sync.go + API spec |

### After (what should have happened — 2 commits):

| Commit | Scope |
|--------|-------|
| 1 | Merge upstream, resolve conflicts, fix module path + printer field |
| 2 | All novel command fixes: thread_id, anchored LIKE, correct mailbox association, isList, sync guard |

## Related

- PR #823: https://github.com/mvanhorn/printing-press-library/pull/823
- `.printing-press-patches.json` — the 21 patches that accumulated across 9 commits
- AGENTS.md `Automated code review with Greptile` section — documents the review contract

# EverBee Scorecard Improvement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build cache-aware, evidence-backed EverBee research commands that improve real agent usefulness and local scorecard dimensions without upstream Printing Press changes.

**Architecture:** Add an `internal/research` package for scoped snapshots, freshness planning, normalization, and insight engines. Keep CLI command files thin with an `internal/cli` runtime bridge that handles freshness flags, targeted EverBee fetches, snapshot reuse, and JSON output.

**Tech Stack:** Go 1.26, Cobra, SQLite via existing `internal/store`, existing `internal/client`, `cli-printing-press` verification tools.

---

## Spec Source

Design spec: `docs/superpowers/specs/2026-06-01-everbee-scorecard-improvement-design.md`

Scope decisions:

- Local repository only.
- Quality first; score gains are evidence, not the goal.
- Targeted EverBee refresh allowed when local scoped research is missing or stale.
- Fetched research is stored locally for repeat analysis.
- No broad full-account EverBee pull.
- No upstream generator or scorecard changes.

## Plan Files

Detailed implementation steps are split to keep every plan artifact below the 500-line project limit:

- `docs/superpowers/plans/everbee-scorecard-improvement/task-01-research-core.md`
- `docs/superpowers/plans/everbee-scorecard-improvement/task-02-normalizer-runtime.md`
- `docs/superpowers/plans/everbee-scorecard-improvement/task-03-opportunity-slice.md`
- `docs/superpowers/plans/everbee-scorecard-improvement/task-04-remaining-engines.md`
- `docs/superpowers/plans/everbee-scorecard-improvement/task-05-workflow-verification.md`

## File Structure

- Create `internal/research/types.go` for scope, freshness, snapshot, evidence, and response envelope types.
- Create `internal/research/planner.go` and `internal/research/planner_test.go` for local-versus-refresh decisions.
- Create `internal/research/store.go` and `internal/research/store_test.go` for `research_snapshots` persistence over the existing SQLite database.
- Create `internal/research/normalize.go` and `internal/research/normalize_test.go` for raw EverBee record normalization.
- Create `internal/research/opportunity.go`, `keyword.go`, `shop.go`, `listing.go`, and `trend.go` for focused engines.
- Create engine tests beside each engine file.
- Create `internal/cli/everbee_research_runtime.go` and `internal/cli/everbee_research_runtime_test.go` for command flag parsing, targeted fetch, store wiring, and output printing.
- Modify `internal/cli/everbee_insights.go` only to call the shared runtime from existing commands.
- Add `workflow_verify.yaml` for the primary local research workflow.
- Modify `README.md` and `SKILL.md` only for new freshness flags and cache-aware behavior.

Keep `internal/cli/everbee_insights.go` below 500 lines. If edits push it close to that limit, move command constructors into focused files such as `internal/cli/everbee_opportunity_cmd.go` and `internal/cli/everbee_keyword_cmd.go`.

## Task Order

1. Build the research domain types, freshness planner, and snapshot store.
2. Add evidence normalization and the CLI runtime bridge.
3. Ship the `opportunity shortlist` vertical slice first.
4. Extend the pattern to niche, keywords, tags, listing, shop, competitor, and trend commands.
5. Add workflow metadata, docs, and final verification.

## Execution Rules

- Use TDD in each task file.
- Commit at the end of each task.
- Do not reintroduce deleted generated stub command files.
- Do not hand-edit unrelated generated code.
- Keep every created or edited source file below 500 lines.
- Preserve the current successful polish gates unless the task explicitly improves them.

## Self-Review Summary

- Design goal `targeted refresh`: covered by task 01 and task 02.
- Design goal `local snapshot reuse`: covered by task 01 and task 02.
- Design goal `evidence-backed outputs`: covered by task 03 and task 04.
- Design goal `workflow score`: covered by task 05.
- Design goal `type fidelity`: covered by task 01 and task 02.
- No upstream work included.
- No metric-only workarounds included.
- No task depends on broad account sync.

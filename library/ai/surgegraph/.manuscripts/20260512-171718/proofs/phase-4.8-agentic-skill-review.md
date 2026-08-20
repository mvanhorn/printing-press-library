# Phase 4.8 — Agentic SKILL Review

Reviewer: general-purpose subagent
Target: `working/surgegraph-pp-cli/SKILL.md`

## Verdict

PASS with 1 fix-now finding (Sync diff prose) and 5 polish-eligible warnings deferred to Phase 5.5.

## Findings

| # | Severity | Check | Where | Action |
|---|----------|-------|-------|--------|
| 1 | warning | description-truncation | SKILL frontmatter line 3 ("with a local…") | defer to polish (re-render from shorter narrative.headline) |
| 2 | warning | gated-disclosure | `visibility delta --window` is marked "Reserved" in code | defer to polish (label as reserved in prose OR remove from examples) |
| 3 | warning | gated-disclosure | `visibility portfolio --window` is marked "(reserved)" | defer to polish |
| 4 | **error** | gated-disclosure | `sync diff --since` SKILL prose claims it filters by timestamp, but the code marks `--since` as informational only | **FIXED IN-SESSION:** rewrote `sync diff` narrative + example in `research.json` to describe what the command actually does (emit cursors + row counts); declared `--since` as reserved. Regen will refresh SKILL/README. |
| 5 | warning | flag-form-consistency | `search` `--kind` uses singular kind names in some examples and plurals in others | defer to polish |
| 6 | warning | marketing-jargon | "transcendence" leaks from internal Printing Press vocab into README/SKILL | defer to polish |

## Why the rest is polish-eligible

Polish runs the same diagnostic loop and is allowed to mutate README/SKILL prose and narrative wording. The 5 warnings are all narrative-shape, not behavioral. The error required a real wording change in `research.json`, which polish would have caught but I addressed in-session per the skill's "fix-before-Phase-5" rule.

## Followup

Phase 5.5 polish skill will re-evaluate after its diagnostic pass.

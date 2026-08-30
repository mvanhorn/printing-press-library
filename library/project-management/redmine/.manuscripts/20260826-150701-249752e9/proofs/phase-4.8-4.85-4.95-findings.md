# Phase 4.8 / 4.85 / 4.95 findings

## Phase 4.85 (Agentic Output Review) — via printing-press-output-review sub-skill
**PASS, 0 findings.** 6/6 sampled novel-feature outputs plausible; the one sample with real data (`issues blockers 3 --depth 3`) independently re-verified live against the Redmine instance.

## Phase 4.8 (Agentic SKILL Review) — 2 findings, both fixed
1. `issues_blockers.go`'s Cobra `Long` field referenced a non-existent `issues relations list <id>` command. Real path: `issues relations-json get-issue-relations <id>`.
2. `workload.go`'s `Long` field referenced a non-existent `issues list --assigned-to <user>`. Real path: `issues-json get-issues --assigned-to-id <user>`.

Both were `--help`-text cross-reference hints invisible to `verify-skill`'s mechanical flag/command scan. Everything else checked clean: trigger phrases, verified-set alignment (`novel_features_built` ↔ SKILL "Unique Capabilities"), auth narrative, no marketing-copy smell.

## Phase 4.95 (Local Code Review) — 3 bugs found and fixed in `issues_blockers.go`, all live-verified post-fix
1. `blockerNode.Blocks` was declared but never assigned (`"blocks": 0` on every row). Fixed by tracking parent→child during the BFS walk. Verified: `issues blockers 3` now reports `"blocks": 3` for issue 1 (which blocks issue 3).
2. `fmt.Sscanf(args[0], "%d", &startID)` silently accepted trailing garbage (`"3abc"` → `3`, no error). Replaced with `strconv.ParseInt` for full-string validation.
3. A single failed fetch mid-BFS discarded the entire partial blocker chain already discovered. Changed to accumulate failures and continue (matches the SKILL's parallel-fetch partial-failure convention); also fixed the human-output path to print the note even when `blockers` is non-empty.

Reviewed, no finding: `groupField` in `issues_cycle_time.go` is `fmt.Sprintf`'d into SQL, but its value is restricted to `"tracker"`/`"project"` by an explicit pre-check before the query is built — no injection surface.

Out of scope, untouched: `internal/cliutil/`, `internal/mcp/cobratree/`.

`go build`, `go vet`, `go test ./...` clean after all fixes.

## Phase 4.9 (README/SKILL/AGENTS correctness audit)
Spot-checked README "Unique Features" section (all 6 novel commands present with correct paths and real examples), no placeholder literals in executable examples (`<command>` occurrences are in generic usage-pattern prose, not copy-paste example commands), AGENTS.md unmodified generator boilerplate (accurate — no CRUD/stub claims beyond what's shipped). No findings.

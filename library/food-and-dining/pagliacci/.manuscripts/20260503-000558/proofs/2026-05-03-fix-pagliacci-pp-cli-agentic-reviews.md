# Pagliacci CLI Agentic Reviews (Phase 4.8 / 4.85 / 4.9, 2026-05-03)

## 4.8 — SKILL semantic review

**Verdict: PASS — no findings.**

All 7 checks passed:
- Trigger phrases map to real capabilities
- 8 novel features in research.json `novel_features_built` align with SKILL "Unique Capabilities"
- Each novel feature description matches the actual --help output
- Auth-gated commands disclosed correctly (orders, rewards, address book)
- Auth narrative is accurate (composed cookie auth via Chrome session, no fictitious set-token)
- Recipes match command shapes
- No marketing-copy slop

## 4.85 — Output review

**Verdict: WARN (1 finding).**

`slices today --agent` returns 92 entries of empty `{}` while `slices today --json` returns fully-populated rows. Root cause: `internal/cli/helpers.go` `compactListFields` allow-list (`id, name, title, status, type, ...`) doesn't include the snake_case domain field names emitted by the novel `SliceRow` struct (`store_id`, `slice_name`, `price`), so every field is stripped under `--agent` (which sets `--compact`).

Per Wave B rollout, surfaces as warning, not blocker. Polish skill should fix locally; longer-term this is a printing-press retro candidate (the compact allow-list shape is too narrow for novel commands emitting custom structs).

## 4.9 — README/SKILL correctness audit

**Verdict: ERROR (5 critical findings + ~40 snake-case sites + warnings).**

Most findings are systemic generator-template issues that will affect every printed CLI; logged as retro candidates. The CLI itself works — these are wrong references in the user-facing docs.

### Critical errors (block shipping if not fixed by polish)

1. **Install paths in SKILL.md use binary name not slug.** Three sites (L6 frontmatter, L392, L401) use `library/other/pagliacci-pp-cli/cmd/...` but the public library catalog keys by API slug `pagliacci`, so `go install` would 404. Fix: replace `pagliacci-pp-cli/cmd/` with `pagliacci/cmd/`.
2. **Snake_case command tokens throughout README/SKILL.** The binary uses kebab-case (`compute-quote`, `slot-list`, `window-days`, `confirm-email`, `password-forgot`, `coupon-lookup`, etc.); the README and SKILL show the snake_case spec keys. ~40+ sites in README plus same in SKILL. Bulk fix: replace `_` with `-` in command tokens.
3. **README L404 has empty `Config file: \`\`** — looks like a missing template fill. Either remove or populate.
4. **README troubleshooting commands wrong:** `stores tonight` / `stores list` should be `store tonight` / `store list` (singular); `scheduling time-window-days <storeId> DEL` should be `scheduling window-days <storeId> DEL`.
5. **README per-group Commands tables omit novel leaves** (`menu half-half`, `orders reorder/summary/plan`, `rewards stack`, `store tonight`). They appear in Unique Features but not in the group reference tables — agents looking up commands by group will miss them.

### Cross-cutting passing checks (PASS)
- All 8 `novel_features_built` appear in both files
- No placeholder literals (`<cli>`, `<command>`)
- Composed cookie auth narrative is accurate
- "Pagliacci Pizza" used as brand name in narrative
- Exit codes and recipe commands resolve to real subcommands

## Disposition

The Phase 4 ship threshold remains met:
- shipcheck verdict PASS (5/5 legs)
- scorecard 82/100 ≥ 65
- novel features all wired
- live API smoke (slices today --json) returns real data

Phase 4.8 issues: zero. Phase 4.85: warning, fix in polish. Phase 4.9: errors, fix in polish (these are squarely the polish skill's domain — README/SKILL cleanup is its core function).

Polish skill (Phase 5.5) is required for this reprint. Verdict shifts to **ship-conditional-on-polish**: must run polish before promoting, OR if polish doesn't fully clean up, downgrade to `hold`.

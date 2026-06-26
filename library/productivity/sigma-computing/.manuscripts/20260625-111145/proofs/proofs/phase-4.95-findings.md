# Phase 4.95 Native Code Review — Findings

Scope: 8 hand-written novel-feature files (grant_audit, access_review, workbook_stale, workbook_copy, member_offboard, member_provision, export_bulk, novel_shared). Generator-reserved paths (cliutil, mcp/cobratree) untouched.

## Fixed in-place (autofix)
1. **member_offboard.go — pagination truncation (HIGH).** Owned-files listing read only page 1; a heavy owner's overflow files silently stayed with the deactivated member. Fixed: added `getAllEntries` paginator (follows `nextPage`), offboard now lists all pages.
2. **member_offboard.go — non-atomic partial failure (HIGH).** Reassign-then-deactivate aborted on first PATCH error naming only the failing inode. Fixed: accumulate per-inode results; on failure, leave member ACTIVE and report exactly which inodes already moved + that a re-run is safe.
3. **novel_shared.go / member_provision.go — member & attribute lookups single-page (HIGH).** `lookupMemberIDViaAPI` and `lookupUserAttributeID` read only page 1 → member/attribute on page 2 invisible (false "not found" → spurious create / skipped attr). Fixed: member lookup now uses server-side `?email=` filter + paginated fallback; attribute lookup paginates.
4. **member_provision.go — swallowed lookup error + dead --idempotent branch (MED).** `existingID, _ :=` discarded errors (transient failure → duplicate create, defeating --idempotent); both idempotent branches were identical. Fixed: surface lookup errors; existing member is now a hard error WITHOUT --idempotent and a reported no-op WITH it.
5. **member_provision.go — created-with-empty-id silently skipped team/attrs (MED).** A 2xx create with unparseable id reported clean "created" while skipping team/attribute assignment. Fixed: emit "created, but could not resolve new memberId; team/attributes NOT applied" detail.
6. **export_bulk.go — bare `null` on empty result (Phase 4.85 warning).** Fixed: initialize slice to `[]`, human path prints "no workbooks matched".

## Verified safe (not bugs)
- **access_review.go resolveResource SQL string-build** — `table` comes only from a fixed map (workbooks/connections/workspaces); `inodeType` used as map key; `inodeID` parameterized. No injection.
- **SQLite deadlock fixes (grant_audit/access_review/novel_shared)** — confirmed correct & complete by review; no other follow-up-query-inside-open-rows patterns remain.
- **exportFormatBody reuse** — constructed once, never mutated; safe.

All fixes mechanical/behavior-preserving from the README's perspective; no scope change, no Phase 1.5 re-approval needed. Build + vet + tests green after each.

## Retro candidate
The novel-command store-query skeleton in the press SKILL does not warn about (a) SQLite single-connection deadlock when issuing a follow-up query inside an open rows loop, nor (b) following `nextPage` cursors on Sigma list endpoints. The implementation agent applied both anti-patterns uniformly. Worth a retro to add drain-first + paginate notes to the store-query / API-call RunE skeletons.

## Phase 5.5-pre /simplify (reuse angle)
- **Finding:** `getAllEntries`/`decodeEntries` (novel_shared.go) duplicate `paginatedGet`/`extractPaginatedItems` (helpers.go).
- **Decision: KEEP the self-contained helpers.** Reusing `paginatedGet` would require adding `"entries"` to `extractPaginatedItems`' key list in helpers.go — a generator-emitted DO-NOT-EDIT file that regen overwrites. Coupling novel code to an editable generated file is more fragile than the ~40 lines of duplication. My helpers are verified correct against the spec (Sigma `page` param IS the cursor: "specify further pages using the string returned in the nextPage portion of the response").
- **Retro candidate (added):** `extractPaginatedItems` is missing the `"entries"` envelope key that Sigma (and `unwrapSingleKeyArray` already) recognizes. Adding `entries` to the generator's pagination walker would let Sigma-shaped CLIs reuse `paginatedGet` instead of hand-rolling. Filed for the press maintainers.

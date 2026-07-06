# Phase 4.95 — Local Code Review Findings (agilix-dawn-pp-cli)

Review path: general-purpose reviewer subagent over the 7 hand-written novel files.
Result: 0 errors, 5 warnings — all autofixed in-place (behavior-preserving), 1 round.

## Autofixed
1. catalog_diff.go — corrupt snapshot silently treated as empty baseline → now warns and resets baseline explicitly.
2. catalog_diff.go — non-atomic snapshot write → temp-file + os.Rename atomic write.
3. catalog_diff.go — nondeterministic `Changed` ordering → sort by (id, field).
4. dawn_structure.go — raw concept id concatenated into URL path → url.PathEscape(id).
5. catalog_diff.go — transient empty fetch would overwrite good snapshot → skip write + warn when 0 courses.

## Confirmed clean by reviewer
- Dry-run safety (all 7 return nil before network/file IO), nil/index guards, no leaks,
  correct aggregation math, actionable error handling via classifyAPIError.

## Non-blocking note
- purchase reconcile caps the user join map at 1000; buyers beyond that fall back to id with buyer_matched=false (acceptable degradation).

Post-fix: go build/vet/test clean; catalog diff + course stats re-verified live.

## Authoring extension (edit command group) — review round 2
Review path: focused safety reviewer subagent over internal/cli/dawn_authoring.go.
Result: write-safety confirmed SOUND (single write path, preview-by-default + verify/dogfood
block have no bypass, dry-run safe, patch id-match sound, nil-safe). 1 conditional warning + 1 info, both addressed:
- Single-element array collapse (would drop lone child on add IF Dawn serialized a single
  section/instruction as an object) → added defensive arrOf() normalization. Empirically Dawn
  returns arrays even for single elements (verified live), so this is belt-and-suspenders.
- Version-conflict errors (409/412) now return an actionable "re-run to fetch latest" message.
Live-verified: preview never writes; --apply writes; instruction/section/patch round-trips correct;
test course restored to original empty state after every test.

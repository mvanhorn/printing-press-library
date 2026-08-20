---
title: Fix iCloud Messages typedstream decoder + PR #766 review follow-ups
status: completed
type: fix
created: 2026-05-23
completed: 2026-05-23
plan_depth: standard
target_repo: printing-press-library
target_branch: feat/icloud-messages
target_pr: 766
---

# Fix iCloud Messages typedstream decoder + PR #766 review follow-ups

## Problem Frame

The `feat/icloud-messages` branch (PR #766) ships a vendored typedstream heuristic decoder for the `attributedBody` column of macOS `~/Library/Messages/chat.db`. The decoder's fast-path prefix check rejects every real-world blob, so the `decoded` text-source path is effectively dead in production. The catastrophic failure mode is masked by a green test suite because the fixtures build synthetic blobs that do not match the canonical typedstream shape Apple emits.

Independently, Greptile's automated review on PR #766 flagged four other defects across SQL filters, flag scoping, doctor schema probes, and the decoder's control-byte ratio. Per `AGENTS.md` the PR cannot merge with unresolved Greptile findings, so this plan bundles all six fixes into a single sweep against the same branch.

### Decoder prefix bug (the new finding)

`messages_decode.go` (current `feat/icloud-messages` HEAD) bails out when `bytes.HasPrefix(blob, []byte("streamtyped"))` is false. Per Sardegna's "Reverse Engineering Apple's typedstream Format" (cited in the file header), every NSArchiver typedstream begins with a 2-byte header `\x04\x0B` — a version byte (4) and a length byte (11) — followed by the literal ASCII `streamtyped`. So real blobs start with `\x04\x0Bstreamtyped...`, not `streamtyped...`, and the prefix check rejects 100% of them at the function entry point. The string-tag scan loop downstream is correct; it just never runs.

The unit tests pass because `buildSimpleBlob` in `messages_decode_test.go` writes `[]byte("streamtyped")` directly without the leading version/length pair. The fixtures validate the decoder against synthetic input that does not match what chat.db actually stores, so all eight decoder tests are green while the production code path is unreachable.

### Greptile-flagged issues on PR #766

Greptile confidence score 3/5 with five inline findings (severity tags not surfaced in the summary; treat as blocking per repo convention):

- **G1 (`messages_db.go:299-302`)**: `--since` filter on `list-chats` is a silent no-op on modern macOS — `MAX(m.date)` returns nanoseconds-since-Cocoa-epoch while the comparison threshold is seconds. The condition is always true. `searchMessages` (line 462) and `statsByYear` already handle the unit conversion correctly; `list-chats` is the outlier.
- **G2 (`messages_db.go:473-474`)**: `escapeLike` writes `\%` and `\_` into LIKE patterns but the SQL string carries no `ESCAPE '\'` clause. SQLite treats `\` as literal, so escaping never fires and queries containing `%` in the literal search term silently miss matching rows in the SQL phase. The Go re-filter cannot recover them because they were never returned.
- **G3 (`messages.go:17-18`)**: `--messages-db` is registered on the `messages` persistent-flag set, so it is invisible to the sibling `doctor` command. `doctor --messages-db /some/path` exits with `unknown flag`, but the doctor body reads `f.messagesDBPath` and `AGENTS.md` documents `--messages-db` as a path override at the root level.
- **G4 (`messages_decode.go:202-204`)**: control-byte ratio compares `control` (counted per-rune) against `len(s)` (byte length). On emoji or CJK messages, multi-byte runes inflate the denominator and slacken the 25% rejection threshold below intent.
- **G5 (`doctor.go:194-205`)**: `checkMessagesSchema` probes `message`, `chat`, `handle` but not `chat_message_join` or `message_attachment_join`, which every real query joins. A stripped/old db can report "schema valid" then fail at first query time.

## Scope Boundaries

### In scope

- Permissive substring scan for the typedstream marker in the first ~20 bytes of `attributedBody` (replaces the broken `HasPrefix` check).
- Rune-count denominator for the control-byte ratio.
- A synthetic real-world-shape test fixture: a blob constructed to match the canonical `\x04\x0Bstreamtyped` format, with synthetic non-PII content, validating that the production code path actually decodes.
- The four non-decoder Greptile findings (G1, G2, G3, G5) on the same branch.
- Patch manifest entry for `messages-attributedbody-heuristic` updated to reflect the fixed shape.

### Out of scope

- Full typedstream graph walk or NSArchiver Foundation class registry. The vendored decoder remains a heuristic; this plan corrects its entry condition, not its scope.
- Recovery of message rows whose `attributedBody` is absent or uses formats that genuinely fall outside the typedstream heuristic (e.g., balloon-payload-only rich content rows). Those continue to return `unrecoverable` and that is correct.
- Photos-side commands and the existing photos test suite.
- Real-world blob fixtures captured from any actual chat.db on disk. All test data is synthesized from the canonical format spec to keep private message content out of a public repo.

### Deferred to Follow-Up Work

- Coverage telemetry beyond the existing `text_source` field (e.g., a `messages stats --coverage` flag that reports decoded/text-column/unrecoverable ratios). Useful for tracking regressions but not required for the current bug fix.
- Tapback / associated-message-type rendering (`associated_message_type=2001` and friends). Currently exported with empty text; semantically separate from the decoder fix.

## Key Technical Decisions

### KD1. Permissive prefix scan over format-aware parse

Replace `bytes.HasPrefix(blob, []byte("streamtyped"))` with a substring scan of the first ~20 bytes. Rationale:

- Robust to undocumented variations (e.g., if Apple changes the version byte from 4 to 5 in a future macOS, the scan still finds the marker).
- Matches the heuristic posture already cited in the file header — this is a heuristic decoder, not a faithful NSArchiver parser.
- One-line fix; minimal diff for reviewers.
- The existing string-tag scan downstream is the actual safety check — if the blob isn't a typedstream, the tag scan finds no valid string-tag candidate and the function still returns `unrecoverable`.

The format-aware alternative (parse `\x04\x0B` explicitly, verify length byte matches `"streamtyped"` length) was considered and rejected for diff churn and future-fragility.

### KD2. Synthetic blob fixtures, not captured chat.db rows

All test fixtures are constructed in code from the canonical format spec: `\x04\x0B` + `"streamtyped"` + intervening typedstream metadata + UTF-8 string tag + length-prefixed text. Synthetic message content (`"hello world"`, `"test message"`, etc.) — no real captured content from any developer's chat.db. Rationale:

- The repo is public; committing real `attributedBody` hex from any contributor's chat would leak private messages.
- The decoder is a format check, not a corpus check — synthesizing canonical-shape inputs covers the bug surface as effectively as captured blobs would.
- `buildSimpleBlob` is updated to write the canonical 2-byte header; an extended `buildCanonicalBlob` adds the intervening NSArchiver class-table bytes for higher-fidelity coverage.

### KD3. Bundle Greptile findings with the decoder fix

The repo's `AGENTS.md` makes every Greptile finding blocking before merge. Bundling all six fixes in this plan keeps the PR's review surface contained and prevents a second review cycle on the same branch. Each fix lands as its own commit so reviewers can isolate any one if needed.

### KD4. Use `utf8.RuneCountInString` for the control-byte ratio (G4)

Aligns numerator (counted per-rune via `for _, r := range s`) and denominator (was byte length, now rune count). The 25% threshold then means "25% of characters are control" rather than "25% of bytes are control after multi-byte rune inflation."

### KD5. Move `--messages-db` to root persistent flags (G3)

Cleaner than duplicating on `doctor`. Photos commands already accept `--library` at the root level for the equivalent override; symmetric placement matches existing conventions.

## Implementation Units

### U1. Fix typedstream prefix check in attributedBody decoder

**Goal**: Replace `HasPrefix("streamtyped")` with a permissive substring scan over the first ~20 bytes so real `\x04\x0Bstreamtyped`-prefixed blobs reach the existing string-tag scan loop.

**Requirements**: Resolves the catastrophic decoder failure; the `decoded` `text_source` path becomes reachable in production for the first time.

**Dependencies**: None — entry-point fix.

**Files**:
- `library/media-and-entertainment/icloud/internal/cli/messages_decode.go`

**Approach**:
- Replace lines 71-73 (`if !bytes.HasPrefix(...) { return ..., textSourceUnrecoverable }`) with a scan over `blob[:min(20, len(blob))]` looking for `"streamtyped"` via `bytes.Index`. If absent, return unrecoverable.
- Keep the empty-blob guard at line 63 unchanged.
- Keep the downstream tag-scan loop unchanged — it remains the authoritative validity check.
- Update the function godoc at line 56 to describe the permissive entry condition.

**Patterns to follow**: Match the existing godoc style (intent, returns, panic safety).

**Test scenarios**:
- Happy path: synthetic blob with canonical `\x04\x0Bstreamtyped` header + valid string tag + direct-length-prefixed ASCII text decodes to the expected text with `text_source=decoded`.
- Backward compatibility: synthetic blob starting with bare `"streamtyped"` (the old test fixture shape) still decodes — the permissive scan finds the marker at offset 0.
- Negative: blob with `"streamtyped"` at offset 25 returns `unrecoverable` (marker outside the scan window).
- Negative: random 200-byte buffer with no `streamtyped` substring returns `unrecoverable`.
- Defense: nil and empty blobs still return `unrecoverable` (pre-existing behavior preserved).

**Verification**: `go test ./internal/cli/ -run TestDecodeAttributedBody -v` shows the new canonical-format tests passing alongside the existing ones; old tests continue to pass.

---

### U2. Fix control-byte ratio denominator (Greptile G4)

**Goal**: Replace `len(s)` with `utf8.RuneCountInString(s)` in the control-character rejection threshold so the 25% gate compares like units.

**Requirements**: Greptile G4; correctness on emoji and CJK message bodies.

**Dependencies**: U1 (same file) — sequence after to keep diff hunks separated cleanly.

**Files**:
- `library/media-and-entertainment/icloud/internal/cli/messages_decode.go`

**Approach**:
- At line 202-204, replace `control*4 > len(s)` with `runeCount := utf8.RuneCountInString(s); runeCount > 0 && control*4 > runeCount`.
- `utf8` is already imported; no new imports.
- Keep the early-return for `s == ""` at line 167 (covers the zero-rune case redundantly but harmlessly).

**Patterns to follow**: Single-statement guard mirrors the existing structure.

**Test scenarios**:
- Emoji-heavy text (≤25% control runes) passes `looksLikeMessageText` after the fix.
- CJK-heavy text passes after the fix (regression target — previous byte-denominator was slacker, so this is a parity case, not a behavior shift; document in test name).
- Synthetic high-control string (`"\x01\x02\x03hello"`) is rejected before and after the fix.
- Boundary: exactly 25% control runes is rejected (strict `>` retained).

**Verification**: `TestDecodeAttributedBody_LooksLikeMessageText` cases for "with emoji" and a new CJK case pass.

---

### U3. Add canonical-format synthetic blob test coverage

**Goal**: Add a `buildCanonicalBlob` helper to `messages_decode_test.go` that writes the canonical `\x04\x0Bstreamtyped` header + intervening NSArchiver class-table bytes + string tag + UTF-8 text, then exercise U1's permissive scan against it. Ensures the test suite catches a regression of the prefix bug.

**Requirements**: Prevents recurrence of the false-green decoder test problem.

**Dependencies**: U1 (the production fix the new tests validate).

**Files**:
- `library/media-and-entertainment/icloud/internal/cli/messages_decode_test.go`

**Approach**:
- Keep existing `buildSimpleBlob` for the legacy bare-`"streamtyped"` cases (those exercise the permissive-scan backward-compat case).
- Add `buildCanonicalBlob(text string, prefixVariant byte) []byte` that prepends `\x04\x0B` before `"streamtyped"` and emits a representative NSArchiver class-table pattern observed in published typedstream documentation (e.g., `NSAttributedString` → `NSObject` → `NSString` class chain). All synthetic, derived from the format spec — not lifted from any real chat.db.
- Add `TestDecodeAttributedBody_CanonicalFormat` exercising the four length-prefix variants (direct, 1-byte, 2-byte, 4-byte) against `buildCanonicalBlob`.
- Add `TestDecodeAttributedBody_NonCanonicalRejected` exercising a blob whose `"streamtyped"` literal sits at offset 30 — should return unrecoverable.

**Patterns to follow**: Existing `TestDecodeAttributedBody_Length{1,2,4}Byte` shape; reuse the prefix-variant switch from `buildSimpleBlob`.

**Test scenarios**: enumerated by U3 itself — this unit is the test-scenario unit.

**Verification**: New tests pass alongside U1's behavior change; remove or update any `buildSimpleBlob`-based test if it now becomes redundant.

---

### U4. Fix `--since` epoch unit mismatch on `list-chats` (Greptile G1)

**Goal**: Make `--since` actually filter results on modern macOS by converting `MAX(m.date)` from nanoseconds-since-Cocoa-epoch to seconds before comparing against the user-supplied threshold.

**Requirements**: Greptile G1; user-visible correctness — `messages list-chats --since 2026-01-01` currently returns every chat regardless.

**Dependencies**: None.

**Files**:
- `library/media-and-entertainment/icloud/internal/cli/messages_db.go`

**Approach**:
- At lines 299-302, mirror the `searchMessages` pattern (line 462) and the `statsByYear` CASE-based conversion. Either:
  - Multiply the user's threshold by 1_000_000_000 in Go before binding (matches `searchMessages` style), OR
  - Use the SQL-side `CASE WHEN last_msg.last_date >= <nanosecond magnitude> THEN last_msg.last_date / 1000000000 ELSE last_msg.last_date END + <cocoa epoch>` form per Greptile's suggestion.
- Prefer the Go-side multiplication for symmetry with `searchMessages` — reviewer can compare two adjacent functions for the same pattern.
- Extract the nanosecond multiplier and Cocoa-epoch constant to package-level named constants if not already present.

**Patterns to follow**: `searchMessages` at `messages_db.go:462` is the working sibling.

**Test scenarios**:
- Integration-style: against an in-memory SQLite db seeded with three messages at nanosecond timestamps spanning two years, `list-chats --since <year boundary>` returns only chats with a recent-enough `last_msg.last_date`. Pre-fix this returns all three; post-fix returns one.
- Edge: `--since` of "now" returns zero chats.
- Edge: `--since` of "1970-01-01" (well before any plausible iMessage timestamp) returns all chats.

**Verification**: New integration test against a temp SQLite DB; existing search/stats tests unaffected.

---

### U5. Add `ESCAPE '\'` clause to LIKE patterns (Greptile G2)

**Goal**: Make `escapeLike`-escaped `%` and `_` characters in search queries actually behave as literals at the SQL layer.

**Requirements**: Greptile G2; user-visible correctness — searches containing literal `%` or `_` silently miss matching messages.

**Dependencies**: None.

**Files**:
- `library/media-and-entertainment/icloud/internal/cli/messages_db.go`

**Approach**:
- At line 473-474, append `ESCAPE '\\'` (Go-source double-backslash → single backslash in the SQL string) to the LIKE expression.
- Audit every other LIKE expression in `messages_db.go` for the same omission. If any other LIKE uses `escapeLike`, add the same `ESCAPE` clause.
- Keep the Go-level re-filter in `searchMessages` unchanged — it operates on decoded `attributedBody` text and the escape change doesn't affect that pass.

**Patterns to follow**: SQLite docs `https://www.sqlite.org/lang_expr.html#like` — the `ESCAPE` clause is part of the LIKE expression itself, applies per-expression.

**Test scenarios**:
- Search for `"50%"` against a corpus containing `"got 50% off"` and `"another row"` returns the first row only. Pre-fix the SQL layer returns nothing for the literal-percent case.
- Search for `"foo_bar"` against a corpus containing `"foo_bar"` and `"fooXbar"` returns only the first row.
- Search for a plain word like `"hello"` against `"hello world"` continues to work (regression check).

**Verification**: New `TestSearchMessages_LiteralPercent` and `TestSearchMessages_LiteralUnderscore` cases pass.

---

### U6. Move `--messages-db` to root persistent flags (Greptile G3)

**Goal**: Make `--messages-db /path/to/chat.db` work from both `doctor` and `messages` subcommands.

**Requirements**: Greptile G3; `AGENTS.md`-documented behavior.

**Dependencies**: None.

**Files**:
- `library/media-and-entertainment/icloud/internal/cli/messages.go`
- `library/media-and-entertainment/icloud/internal/cli/root.go`

**Approach**:
- Remove the `--messages-db` registration from the `messages` command's persistent-flag set in `messages.go:17-18`.
- Add it to `root.go`'s persistent flags alongside `--library`, with the same field on `rootFlags`.
- Confirm `doctor.go` already reads from the rootFlags-resident path field. If it currently reads from a messages-local field, plumb through the rootFlags-resident field.

**Patterns to follow**: The existing `--library` flag at root scope.

**Test scenarios**:
- `icloud-pp-cli doctor --messages-db /tmp/foo.db` no longer errors with `unknown flag`. Use `cobra`'s command execution helper or shell out to the built binary in an integration test.
- `icloud-pp-cli messages list-chats --messages-db /tmp/foo.db` continues to work.
- `icloud-pp-cli --messages-db /tmp/foo.db doctor` (flag before subcommand) works — confirms persistent-at-root behavior.

**Verification**: Manual CLI invocations + a unit test that constructs the root command and inspects its persistent flag set.

---

### U7. Probe junction tables in `checkMessagesSchema` (Greptile G5)

**Goal**: Surface `chat_message_join` or `message_attachment_join` absence at `doctor` preflight time rather than at first-query failure.

**Requirements**: Greptile G5.

**Dependencies**: None.

**Files**:
- `library/media-and-entertainment/icloud/internal/cli/doctor.go`

**Approach**:
- At `doctor.go:194-205`, extend the table loop from `{"message", "chat", "handle"}` to `{"message", "chat", "handle", "chat_message_join", "message_attachment_join"}`.
- Keep the existing per-table failure-message format. Doctor's Messages section already uses yellow ⚠ rather than red ✗, so additional schema-missing reports don't escalate severity.

**Patterns to follow**: Existing table loop structure.

**Test scenarios**:
- Against a synthetic SQLite db with only `message`, `chat`, `handle` (missing junction tables), doctor reports the schema-incomplete state.
- Against a complete db, doctor reports schema valid.

**Verification**: `doctor` integration test against two seeded temp databases.

---

## Verification Strategy

- `go test ./...` from `library/media-and-entertainment/icloud/` is green post-changes.
- `python3 .github/scripts/verify-skill/verify_skill.py --dir library/media-and-entertainment/icloud/` continues to pass — no SKILL.md flags reference changed; `--messages-db` was already documented at root scope per `AGENTS.md`.
- `go vet ./...` and `govulncheck ./...` from the icloud directory: clean.
- Manual smoke against a real `~/Library/Messages/chat.db` after fixes: `icloud-pp-cli messages stats --agent` and `icloud-pp-cli messages search "test" --limit 5 --agent` show non-zero `text_source: decoded` counts where previously all results were `text_column` or `unrecoverable`. Smoke is local-only — no captured output enters the PR.
- Greptile re-review on the next push: confirm G1–G5 are resolved (👍 status); no new findings introduced.

## System-Wide Impact

- **Decoder behavior change is observable.** Callers reading the `text_source` field will see `decoded` rows appear in counts where they previously saw `unrecoverable`. This is the intended behavior — callers were already designed to surface the field. SKILL.md already documents the three values.
- **`--since` semantics change on `list-chats`.** Previously a no-op; now filters. Any caller (human or agent) that was depending on the no-op behavior (e.g., passing `--since` "defensively" but expecting all chats back) will see fewer results. The fix matches documented intent.
- **Search results may expand on patterns containing `%` or `_`.** Previously missed; now returned. Strictly additive — no previously-returned row is dropped.
- **`--messages-db` flag scope is wider.** Now accepted by `doctor` and at root level. No removal; flag still works on `messages` subcommands.
- **Doctor preflight may now report previously-silent junction-table absence.** Strictly informational — yellow ⚠, not blocking.

## Risk Analysis

- **R1. Permissive scan over-decodes random binary.** Risk: the substring scan finds `"streamtyped"` inside a non-typedstream blob (e.g., binary payload that happens to contain the byte sequence). Mitigation: the downstream string-tag scan still has to find a `0x2B` tag with a valid length prefix and a UTF-8 candidate passing `looksLikeMessageText`. False positives at all three gates are extremely unlikely. Severity: low.
- **R2. Backward-compat with old test fixtures.** Risk: U1 changes the entry condition from prefix to substring; some tests relying on the old prefix-required behavior may now decode where they previously returned unrecoverable. Mitigation: audit existing tests in U3 and explicitly cover the bare-`"streamtyped"` case (which still passes — the marker is at offset 0). Severity: low.
- **R3. `--since` epoch fix mis-converts on edge cases.** Risk: if any chats genuinely have second-domain timestamps (pre-iCloud-sync messages, imported data), the conditional conversion logic must handle both. Mitigation: mirror `searchMessages` exactly — it has been in production through the photos-side feature set without issue. Severity: low.
- **R4. SQLite `ESCAPE` clause semantics on `COLLATE NOCASE`.** Risk: `ESCAPE` and `COLLATE` interact in subtle ways across SQLite versions. Mitigation: SQLite's documented behavior is that `ESCAPE` is per-expression and orthogonal to `COLLATE`. Add explicit test cases for case-insensitive search with literal `%`. Severity: low.
- **R5. Privacy in test fixtures.** Risk: a future contributor "improves coverage" by pasting hex from their own chat.db. Mitigation: U3's helper docstring states explicitly that fixtures are synthesized from the format spec and not captured. Severity: low if documented; medium if the comment is lost.

## Sequencing

- U1 first (the catastrophic bug; everything else can land regardless of order).
- U3 immediately after U1 to lock the test fixture against regression.
- U2 next (same file as U1; co-resident commits keep file-level diff coherent).
- U4, U5, U6, U7 in any order — independent surfaces.
- Final pass: update `.printing-press-patches.json`'s `messages-attributedbody-heuristic` entry to reference this plan and the corrected behavior summary.

## Privacy Constraint (load-bearing)

**No real message content, phone numbers, contact identifiers, captured `attributedBody` hex, or other PII enters this plan, commits, PR body, or test fixtures.** All test fixtures are synthesized from the published canonical typedstream format (Sardegna's blog, `dgelessus/python-typedstream` documentation, the upstream `teslashibe/imessage-go` MIT reference). Smoke verification against real chat.db data is local-only; any output shared with reviewers is summary-level (counts, ratios) with no decoded text.

This is a public repository. Treat any temptation to "just paste a hex dump for clarity" as a hard stop.

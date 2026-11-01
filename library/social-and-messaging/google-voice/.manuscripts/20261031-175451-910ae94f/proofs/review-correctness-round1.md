# Google Voice correctness + API-contract review — round 1

Target: `working/google-voice-pp-cli`, Phase 17 direct reviewer dispatch. Source line numbers below are relative to that target. Reviewed the approved absorb manifest, native Voice adapter/store/commands, their wiring and tests, and the checked-out upstream protocol. No real sessions, message snapshots or HARs were read; no remote requests were made. No production source or ledger was changed.

Baseline `go test ./internal/voice` passes. Synthetic tests supplied through a Go overlay reproduce the findings below without changing production source. Fixture source: `proofs/review-correctness-round1-synthetic-test.go.txt`. Original temporary overlay: `/private/tmp/gv-review-correctness-r1/overlay.json`. Run from the target: `go test -overlay /private/tmp/gv-review-correctness-r1/overlay.json ./internal/voice -run TestReview -v`.

## Required custom fixes

### 1. [P1] Latest refresh discards a new continuation after prior history exhaustion

- **Location:** `internal/voice/mirror.go:188` (also scheduling at `:273–285`).
- **Impact:** A previously exhausted thread that receives more messages than fit in the 100-message latest page can permanently lose the middle interval. Refresh retains `history_complete=1` and an empty `older_cursor` even when the latest response returns a nonempty cursor. Sync then has no pending work and reports `observed_exhausted`. The same interval gap can occur while an older-history cursor is already saved: retaining that older cursor skips the new gap between the latest page and previously observed messages.
- **Reproduction:** `TestReviewLatestRefreshGap` first commits an exhausted thread, then returns a latest page with a new message and `gap-cursor`. Result: `{Pages:2 Status:observed_exhausted PendingThreads:0 InventoryComplete:true}`; only the empty get cursor was requested; saved cursor remains empty and history complete remains true.
- **Minimal fix:** Persist and finish latest-refresh continuation until reaching previously observed IDs (or source exhaustion), separately from the existing historical backfill cursor. Do not let earlier historical exhaustion suppress newly unfinished refresh work. An implementation that deliberately re-traverses history is acceptable if it remains bounded/resumable and cannot repeatedly reset its progress.

### 2. [P2] Small page budgets starve later folders and falsely report inventory exhaustion

- **Location:** `internal/voice/mirror.go:243–255`, `:303–309`.
- **Impact:** `--folders texts,archived --max-pages 1` always spends its budget on texts. Once texts exhausts, the next invocation starts texts again; archived is never attempted. The final query counts only existing unfinished inventory rows, so the missing requested archived row is treated as complete and status can be `observed_exhausted`.
- **Reproduction:** `TestReviewBoundedFolderDiscovery` runs that combination three times against empty source inventories. Requests are `map[texts:3]`, archived has no observation, and all three results claim inventory complete/exhausted.
- **Minimal fix:** Prioritize unobserved/pending requested folders before restarting completed inventory passes, and require an exhausted saved observation for every requested folder before reporting completion. Preserve fairness when the budget is smaller than the selected folder count.

### 3. [P2] Malformed populated fields are silently converted to empty/exhausted pages

- **Location:** `internal/voice/codec.go:118`, `:135`, `:209` and permissive `array`/`str` helpers.
- **Impact:** A schema change or malformed successful response can skip records and clear checkpoints instead of producing a typed failure. The decoder checks message IDs/timestamps only after coercing the messages field to an array; a non-array bypasses the check entirely. Non-string populated cursors become empty strings, falsely indicating exhaustion.
- **Reproduction:** `TestReviewMalformedPage` accepts all of `[["t-1",0,{},null]]`, `[["t-1",0,[],["wrong-cursor-type"]]]`, and list `[[],123]` without error. All become empty/exhausted pages. Upstream `research/gvoice-source/pkg/libgv/gvproto/threads.proto:23–24` declares repeated messages and a string pagination token.
- **Minimal fix:** Distinguish absent/null optional fields from present fields of the wrong type. Validate populated repeated/message/scalar cursor fields before conversion; reject the page before committing it. Continue accepting absent optional fields and unknown appended fields.

### 4. [P2] Attachment metadata is repeated, and its size field is a variant enum

- **Location:** `internal/voice/codec.go:168–169`; incorrect fixture in `internal/voice/voice_test.go:72`.
- **Impact:** Valid attachment dimension metadata is discarded. The existing flat fixture masks the bug and also presents a size-spec enum as a byte-size field.
- **Ground truth:** `research/gvoice-source/pkg/libgv/gvproto/threads.proto:131–148`: `Attachment.metadata` is **repeated Metadata**; each Metadata is `[SizeSpec,width,height]`. `SizeSpec` is UNKNOWN/ORIGINAL/S128/S512/S1280 (`:132–140`), not attachment byte size.
- **Reproduction:** `TestReviewRepeatedAttachmentMetadata` supplies attachment metadata `[[1,640,480],[2,128,96]]`. Decoded attachment has width 0 and height 0. Current test instead supplies `[1024,640,480]`.
- **Minimal fix:** Decode metadata entries, preserve their size-spec and dimensions (or explicitly select the ORIGINAL entry for top-level dimensions while retaining supplied variants), and do not label the enum as bytes. Replace the flat fixture with the protocol-correct nested form.

### 5. [P2] Voicemail transcript token bytes are returned as encoded strings

- **Location:** `internal/voice/codec.go:173–179`.
- **Impact:** The promised voicemail transcript is unusable encoded text rather than the supplied words.
- **Ground truth:** `research/gvoice-source/pkg/libgv/gvproto/threads.proto:81–88` declares `TranscriptToken.text` as **bytes**, which the protobuf JSON array representation encodes as base64. Upstream `pkg/connector/handlegvoice.go:700–721` consumes the already decoded token bytes and converts valid UTF-8 bytes to text.
- **Reproduction:** `TestReviewTranscriptAndDuration` supplies tokens `SGVsbG8=` and `dGhlcmU=`. Output transcript is `SGVsbG8= dGhlcmU=` instead of `Hello there`.
- **Minimal fix:** Decode token bytes using the protocol's base64 encoding, validate UTF-8, and concatenate the decoded words. Cover malformed encoding and absent tokens without silently claiming encoded text is a transcript.

### 6. [P2] Fractional call durations are dropped

- **Location:** `internal/voice/codec.go:141`, `:65–74`, and integer `Message.Duration` at `:37`.
- **Ground truth:** `research/gvoice-source/pkg/libgv/gvproto/threads.proto:98` declares `float durationSeconds`.
- **Impact/reproduction:** `TestReviewTranscriptAndDuration` supplies duration `1.5`. Because `UseNumber` produces `json.Number`, `Int64()` fails and its ignored error turns the value into zero; `omitempty` then removes the supplied duration entirely. This affects every nonintegral duration, not only values below one second.
- **Minimal fix:** Preserve the source duration using a floating-point or exact decimal representation and validate numeric conversion. Keep timestamps and enum values on their integer parser.

### 7. [P2] Context packets pair old exhaustion with a newer inventory observation date

- **Location:** `internal/voice/history.go:68–71`.
- **Impact:** A fresh inventory read updates `voice_threads.observed_at` without refreshing history. Context presents `older_history_exhausted_at_observation:true` alongside that newer timestamp and omits the actual history observation date and pending refresh state. An agent can attribute historical exhaustion to the wrong observation and miss stale latest data.
- **Reproduction:** `TestReviewContextObservationTimestamp` sets history/last-refresh observations to 2020 and the later inventory observation to 2026 with `needs_refresh=1`. Context shows exhaustion true with `observed_at:2026-01-01...`, with no 2020 history timestamp or pending latest refresh marker.
- **Minimal fix:** Return `history_observed_at` for exhaustion, `last_refresh` for latest collection, and `needs_refresh` separately. Preserve an explicitly named inventory/general observation date only if useful. The manifest explicitly requires separate latest/history/inventory observation timestamps.

### 8. [P2] Accepted nanosecond query bounds are rounded in the wrong direction

- **Location:** `internal/voice/mirror.go:347`; same conversion in `internal/voice/history.go:144`.
- **Impact:** `history query` and `history activity` accept RFC3339Nano bounds but truncate them to milliseconds. For a source message at `2023-11-14T22:13:20.000Z`, inclusive after `...20.000500Z` incorrectly includes it, while exclusive before that same bound incorrectly excludes it.
- **Reproduction:** `TestReviewFractionalMessageBounds` observes after=1 and before=0 where the documented half-open window requires after=0 and before=1.
- **Minimal fix:** Compare stored millisecond timestamps against correctly rounded-up lower and exclusive upper bounds, or retain exact timestamp precision in the comparison. Apply the same boundary policy to both query and activity.

### 9. [P2] Thread get's unread guard admits unknown status

- **Location:** `internal/cli/voice_core.go:276`; MCP Gateway equivalent at `internal/voice/remote.go:229`.
- **Impact:** `threads get --unread` and MCP `threads_get {unread:true}` return a thread when its decoded `Read` is nil. Help promises results only when the thread is currently unread; list already requires an explicit false value. The codec intentionally represents absent status as unknown, so this inconsistency invents unread certainty.
- **Reproduction:** `TestReviewUnknownUnreadGuard` uses an entirely synthetic imported profile and transport. A detail page with `read:null` is returned by the Gateway with `unread:true` rather than filtered out. The CLI predicate is identical.
- **Minimal fix:** Apply the same predicate as list: unread matches only `Read != nil && !*Read`; exclude or explicitly fail unknown state. Keep CLI and typed MCP equivalent.

### 10. [P3] Nested AccountVersion is always omitted

- **Location:** `internal/voice/codec.go:110`.
- **Ground truth:** `research/gvoice-source/pkg/libgv/gvproto/account.proto:16` declares `AccountVersion version = 13`; `:21–22` defines nested `versionToken = 1`.
- **Impact/reproduction:** `TestReviewAccountVersion` supplies account field 13 as `["version-token"]`. `str(at(account,12))` reads the wrapper array as a string, so the otherwise valid account result loses the exposed version.
- **Minimal fix:** Read the first field of the AccountVersion wrapper and cover absent/null/new appended fields.

## Nonblocking observations and coordination

- **Arrivals upper watermark:** `TestReviewArrivalsSnapshot` proves that records inserted after the first page can appear on continuation because its cursor carries only a lower sequence/run bound (`internal/voice/history.go:74–78,105`). The brainstorm specifies a fixed upper watermark, but the current open-ended stream did **not** demonstrate omission or duplication. This is **not included in the required finding count**; decide whether snapshot consistency is part of the final API contract, and add an upper bound if it is.
- **Per-read account verification:** Sync correctly verifies before checkpoint advancement; List/Get do not verify the pinned account on their own. Parent confirmed this is being fixed with one successful Account verification per Remote instance before List/Get. This is cross-review coordination rather than an additional duplicate finding here.
- **MCP continuation config:** Typed list's `NextCursorPath:"cursor"` does not match normalized `next_cursor`, but the raw cursor currently survives the bounded envelope in observed code paths. No proven skipped-record reproduction; no required finding filed.

## Template / reserved package disposition

All ten required findings above arise in custom Voice source or its custom codec fixture. No generator-template or reserved-package fix is proposed. `internal/cliutil/` and `internal/mcp/cobratree/` remain excluded. No general rewrite or feature-scope reduction is requested.

Review outcome: ten concrete custom correctness/API findings; no scope tradeoff identified. Return the merged implementation for round 2 after fixes.

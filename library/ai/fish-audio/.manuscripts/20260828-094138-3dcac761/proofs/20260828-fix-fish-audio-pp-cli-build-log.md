Manifest transcendence rows: 6 planned, 6 built.

# Phase 3 build log — fish-audio-pp-cli

Date: 2026-08-28

## What was built

### Transcendence rows (6 of 6)

| # | Row | Command | State |
|---|---|---|---|
| 1 | Local render history | `render log` | built |
| 2 | Skip identical re-renders | `tts render --skip-if-rendered` | built |
| 3 | Grouped spend report | `render spend --group-by voice\|model\|day` | built |
| 4 | Compare two renders | `render diff <id1> <id2>` | built |
| 5 | Clone fidelity check | `voice verify <model_id>` | built |
| 6 | Pre-batch budget guard | `tts batch --budget-guard` | built |

### Absorbed rows that needed a command path

`tts render` (rows 1, 5, 9, 10, 11, 13, 26, 28, 32), `tts batch` (rows 7, 8, 29, 30),
`tts resolve` (row 22), `asr transcribe` (rows 14, 15, 27), `voice clone` (row 18),
`voice design` (row 16), `voice design-save` (row 17), `voice discover` (rows 21, 33),
`wallet credit` (row 23), `wallet package` (row 24), `wallet balance` (row 25).

Every path resolves as a real Cobra leaf: `--help` exits 0 and the Usage spec line
names the full leaf path.

### New packages

- `internal/fishaudio` — model/format/latency/visibility enum validation, TTS/ASR/
  voice-design cost math, the order-independent render request hash, the WAV
  frame-count header repair, and batch/JSONL/dialogue input parsing. 12 table-driven
  test functions.
- `internal/wer` — word error rate, normalization, and the pass/warn/fail verdict
  bands. 3 test functions.
- `internal/client/fish_audio_raw.go` — `PostRaw`, a raw-bytes POST that reuses the
  generated client's base URL, auth, HTTP client, and adaptive rate limiter.
  POST /v1/tts answers with a chunked audio stream, which none of the generated
  helpers can carry.
- `internal/store/fish_audio_migrations.go` — lazy `EnsureRenderLog` and
  `EnsureVoiceCatalog`, plus the render-log read/write and FTS search methods.
  `store.go` was not edited.

### Notable behavior

- **msgpack** (`github.com/vmihailenco/msgpack/v5`, added with `go get`): a render
  carrying `--reference-audio` switches its encoding to `application/msgpack`
  automatically, because JSON cannot carry the raw bytes the `references` field
  wants.
- **WAV frame-count repair** (absorbed row 12, which the manifest had marked
  `(stub)`): implemented rather than documented. A streamed WAV arrives with RIFF
  and `data` sizes of 0 or 0xFFFFFFFF; both are rewritten before the file is
  written. No ffmpeg dependency: the fix is arithmetic on two 32-bit header fields,
  not a re-encode.
- **Free-tier accounting**: every render records `cost_usd` and
  `cost_usd_paid_equiv`. On `s2.1-pro-free` the first is 0 and the second is the
  paid-rate value, so the free tier's real value stays visible in `render spend`.
- **s2-family gate**: `tts batch --dialogue --model s1` fails with exit 2 before the
  input file is opened.
- **Harness safety**: `tts render --play` refuses under `cliutil.IsAnyHarness()`.
  `tts batch` curtails to one line and `voice discover --refresh` to one catalog
  page under `cliutil.IsDogfoodEnv()`.
- **Parent Shorts**: `render` and `voice` had leaked their capability-group labels
  into user-facing help. Corrected to "Local render history, spend, and diffs" and
  "Clone, design, discover, and verify voices".

## Intentionally deferred

- **WebSocket real-time TTS** (absorbed row 4) stays a stub, per the manifest.
- **`voice design-save` signature capture is defensive.** The published
  `VoiceDesignCandidate` schema names no signature field, but `POST /model`
  documents `voice_design_signatures` as coming "from /v1/voice-design candidates".
  The parser reads `signature`, `voice_design_signature`, and `sig`, stores whatever
  it finds in `candidates.json`, and warns to stderr when a picked candidate has
  none. A live run against a real key is needed to confirm the field name.
- **`GET /model` page shape** is read as `{"items": [...]}`. Confirm against a live
  response before relying on `voice discover --refresh` for a full sync.
- **No live calls were exercised.** No API key was available, so every assertion in
  this build ran through `--dry-run`, unit tests, or the local SQLite paths.

## Generator limitations found

1. **Group names leaked into parent `Short`.** The generated `render` and `voice`
   parents carried their capability-group labels ("Local state that compounds",
   "Verify before you ship") as their `Short`, which is what `--help` shows a user.
   A group label is index metadata, not a command description. Fixed downstream in
   `internal/cli/fish_audio_wiring.go`. The same labels still appear as the `group`
   field in `which`'s curated index, which is correct there.
2. **`tts_create-stream.go` carried an unused `statusCode` variable** — reported by
   an earlier phase of this run. Verified: it is no longer present in the current
   tree, so it was fixed upstream or in an earlier regeneration. Recorded here so
   the finding is not lost.
3. **Novel-hook ordering.** `registerNovelCommand` hooks run *before* root.go
   attaches the generated novel parent groups, so a hook cannot find `render` or
   `voice` on the root to extend them. The workaround is to construct those groups
   inside the hook and let root.go's own `addNovelCommandIfAbsent` no-op. A hook
   that ran after the built-in novel attachment would remove the need.
4. **`POST /model` was generated as a JSON body command.** The spec declares
   `application/json` with `format: binary` fields (`voices`, `cover_image`), so the
   generated `model create` posts JSON and cannot upload a file. `voice clone` and
   `voice design-save` build the multipart body themselves.
5. **The multipart encoder cannot repeat a field name.** `multipartRequestBody`
   takes `map[string]string`, so `voices[]`, `texts[]`, `tags[]`, and
   `voice_design_signatures[]` — which must repeat and stay in matching order —
   cannot be expressed. `buildMultipart` in `internal/cli/fish_audio_shared.go` is
   the ordered, repeat-capable replacement.
6. **No raw-bytes response path.** Generated helpers unmarshal or base64-wrap every
   response, so a chunked audio stream needed `PostRaw`.
7. **`asr` was generated as a bare leaf command**, not a group. `asr transcribe` is
   attached as its subcommand, which Cobra allows alongside the parent's own `RunE`.
8. **Terse generated flag descriptions.** Twenty-two API-facing flags across
   `model_create`, `model_list`, `model_update`, `wallet_api-credit_get`,
   `wallet_package_get`, `tts_create`, and `tts_create-stream` carried
   under-five-word descriptions ("Model tags", "Page size", "Visibility"). All were
   enriched in place.
9. **Templated display string.** `auth.go`'s Short renders "Manage authentication
   for Fishaudio". `research.json` already carries `display_name: "Fish Audio"`, so
   the template is not consuming it. Left as-is and recorded here rather than
   hand-patched, since a reprint would restore it.

## Verification

`go build ./... && go vet ./... && go test -count=1 ./...` all clean. Every command
path resolves. Full assertion output is in the Phase 3 report.

## Phase 4.95 fixes

Code review found three real bugs plus one ordering slip. All four are fixed,
each with a regression test.

1. **Zero-shot render was unreachable, and both voice sources could ride one
   request.** `--voice` was gated as required, so `--reference-audio` alone
   could never run; passing both put `reference_id` and `references` in the same
   body. Now exactly one of the two is required, both together is exit 2, and
   the zero-shot path clears `VoiceID` so only `references` goes on the wire.
   The render log labels a zero-shot row `zero-shot:<file>` rather than leaving
   the voice column blank.
   Tests: `TestTtsRenderRequiresOneVoiceSource`,
   `TestZeroShotRequestDropsReferenceID`, `TestZeroShotBodyOmitsReferenceID`.

2. **`--skip-if-rendered` reported success without producing `--out`.** A hit
   returned exit 0 and `"skipped": true` while the file the caller asked for was
   never created. `reusePriorRender` now returns the prior manifest unchanged
   when its path already is `--out`, otherwise copies the prior bytes to `--out`
   and reports the new path and digest. A prior row whose file is gone, or a
   copy that fails, falls through to a real render with a stderr warning.
   Test: `TestSkipIfRenderedProducesTheRequestedFile`.

3. **Duplicate lines in one batch collapsed into a single render log row.**
   `render_log.request_hash` is UNIQUE, so a repeated line silently overwrote
   its twin and `render spend` under-reported. Units are now deduplicated by
   request hash before dispatch: the audio is fetched once, written to every
   duplicate's output file, and recorded as one row per file keyed by
   `sha256(request_hash + "\x00" + output_path)`. **Batch rows are keyed per
   output file, not per request** — the request hash is the prefix, so a row is
   still traceable to the call that produced it. `count` and the cost fields now
   report the API calls actually made; `deduped` and `files` are new in the
   summary. The budget guard prices the deduplicated jobs so a repetitive batch
   is not refused for a charge it would never incur.
   Tests: `TestDedupeBatchUnitsGroupsIdenticalRequests`,
   `TestBatchRowHashIsPerFile`, `TestBatchDuplicateLinesRecordSeparateRows`,
   `TestBatchRowHash`.

4. **`render diff --dry-run` returned exit 2 instead of an envelope.** The
   two-positional gate ran before `dryRunOK`. The dry-run short-circuit now runs
   first. Partial positionals without `--dry-run` still exit 2.
   Test: `TestRenderDiffDryRunEmitsEnvelope`.

Note on the SKILL.md multi-positional template: it places `dryRunOK` *after* the
`len(args) < N` gate. That ordering makes `--dry-run` unreachable for a bare
probe of a multi-positional command, which is what this bug was. Worth raising
in the generator retro.

`internal/cli/helpers.go` was not touched; the `truncate` rune bug there is
template-shaped and belongs to the generator retro.

Build, vet, and the full test suite are green; `./fish-audio-pp-cli` rebuilt.

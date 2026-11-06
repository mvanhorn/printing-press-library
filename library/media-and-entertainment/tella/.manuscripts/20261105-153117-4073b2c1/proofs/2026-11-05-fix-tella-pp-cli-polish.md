# Tella mid-pipeline polish

| Diagnostic | Before | After |
|---|---:|---:|
| Scorecard | 97/A | 97/A |
| Mock verify | 210/210 | 210/210 |
| Static dogfood | PASS | PASS |
| go vet issues | 0 | 0 |
| Unresolved handwritten gosec | 0 | 0 |
| Raw gosec / package load errors | 80 / 0 | 80 / 0 |
| Tools pending | 1 | 0 (1 precise accept) |
| PII pending / gate failures | 0 / 0 | 0 / 0 |
| Ordinary MCP tools | 73 | 73 |
| Read-only MCP tools | 29 | 30 |

Fixed `videos clips silence-map`: its implementation only GETs silences, uncut transcript and source metadata, including its helper's GET-only path. Added read-only annotation, corrected Short and dry-run intent to name source metadata rather than waveform samples, and extended the existing reprint patch guard. No network/data behavior, flags, operands, or input schemas changed. No research entry/rendered capability uses this command, so no research-description mirror edit was needed. Retired `find-mistakes` remains byte-identical to HEAD; accepted its heuristic finding because it performs no read operation and only prints help or returns retirement error before client creation. Advertising that unusable stub in read-only discovery would be misleading. No blanket accepts or source suppressions.


The command/tool judgment pass read all runtime tool descriptions, all 159 typed manifest entries against parsed spec operation metadata, and source command metadata/operation paths, then inspected ambiguous handwritten read/write paths. Local writes/import/export/sync/apply/download/plan-save surfaces remain mutating. Thin grouping descriptions are category navigation; endpoint schemas and orchestration metadata provide operation parameters and response details. Scorecard's description/token/live dimensions are explicitly unscored; zeroes there are not quality failures. Webhook inbox cursor absence and analytics rename are existing spec/framework limitations, not new CLI faults.

PII audit scanned current CLI high-risk content plus current-run research.json/research Markdown. It reports no findings; detector scope is the floor, not a claim that all possible PII classes were assessed. No source PII replacements or accepts were needed. Both local audit ledgers are populated; tools shows `no pending findings (1 accepted)` without incomplete gates, PII strict exits 0.

Security: re-ran pinned gosec v2.29.0/x-tools0.52.0 after the metadata edit: 80 raw issues, zero load errors. Eleven handwritten findings were individually inspected below. The other 69 generated framework findings remain upstream retro candidates; no reproduced shipping defect was established. Detailed per-finding inventory is in `<local-evidence>`. SQL interpolation uses validated identifiers/parameterized values in inspected paths; credential-shaped strings are enum/env-var names; redirect authorization remains guarded to same origin. No new suppression or security bypass.

- `internal/cli/videos_storyboard_image.go:105` G204: sips is resolved by fixed executable name, invoked without a shell with fixed flags and a locally selected downloaded file; context timeout is 5 seconds. File text is never executable code.
- `internal/cli/videos_clips_cut_snapshots.go:123` G304: Explicit local snapshot-file input; bytes are JSON-decoded and require video/clip IDs, cuts array and expected-cuts array shape before restore. Local CLI file access is the requested operation.
- `internal/cli/novel_workflows.go:635` G304: Explicit storyboard plan-file input decoded as a JSON object before validated workflow steps; no remote-provided filesystem path is being trusted.
- `internal/cli/novel_workflows.go:439` G304: Explicit local media upload-file input; reading selected bytes is the intended upload operation. No path is interpreted by a shell.
- `internal/cli/novel_workflows.go:410` G304: Explicit replacement media filename opened for readability preflight before transcript cutting; open and close errors propagate.
- `internal/cli/lesson_recipe.go:52` G304: Explicit local --recipe input; bounded to 1 MiB and decoded with known YAML fields, single-document enforcement and semantic validation.
- `internal/cli/clips_find_retakes.go:153` G304: Explicit reviewed plan file; JSON decode followed by validateReviewedRetakes against current regenerated plan before any apply.
- `internal/cli/videos_clips_cut_snapshots.go:91` G104: Close occurs after Write already failed; original write failure is returned. Successful path checks Close before atomic rename, preserving data-loss protection.
- `internal/cli/videos_clips_cut_snapshots.go:87` G104: Close occurs after Chmod already failed; original permission error is returned and temporary cleanup remains deferred.
- `internal/cli/novel_workflows.go:459` G104: Response body already consumed with ReadAll; readErr and HTTP failure still propagate. Close is secondary connection cleanup after complete body read.
- `internal/cli/lessons_pass.go:183` G104: workflowFailure emits secondary recovery diagnostics inside a deferred handler while the original primary workflow error is retained and returned.

Product files changed: `internal/cli/novel_workflows.go`; `.printing-press-patches/reprint-current-framework-workflows.json`. Local ledgers: `.printing-press-tools-polish.json` (tracked historical ledger updated), `.printing-press-pii-polish.json` (new local-only). Diagnostics/proofs under `<local-evidence>`; no phase ledger changed. The verification-created module-root binary was removed recoverably; rebuilt polish binaries remain outside the product tree.
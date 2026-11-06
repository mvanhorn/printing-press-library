Manifest transcendence rows: 5 planned, 5 built. All shipping-scope composition features are implemented.

# Tella T2 build proof — local verification

All **5/5 researched composition features** are implemented: automatic video edits, lesson preparation, reviewed delivery, conservative retakes and storyboard files. The absorbed preview, credit-gated Agent and shared idempotency requirements are also implemented: **7/7 mandatory groups**. The catalog manifest retains all 16 prior contracts, with the two researched cookie-retirement reframes, and adds the 5 composition features: **21 total**. This is an implementation/local-fixture count, not live acceptance.

## Scope and retained contracts

## Implementation evidence

- Preview preserves fake fixture URL fragments and expiresAt; dry-run makes no POST. Auto uses real planning reads, cut and editor reference snapshots, replaceExisting:false for zooms, official crop/filler/silence calls, fail-closed layouts and stop-on-error recovery reports. Cut undo remains cut-only, divergence guarded and explicitly exclusive.
- Strict yaml.v3 recipes reject unknown/duplicate fields, invalid enums/numerics, conflicting caption sizes and multiple documents before writes. Defaults preserve styling. Source-only intro/outro insertion orders correctly. The pass re-reads edited playback times, reports retakes without applying them, applies optional caption settings/music, drafts chapters with correct integer cumulative seconds, selects a frame thumbnail and ends with preview, without export.
- Retake plans use official sentence indices and inclusive word ranges, a five-word minimum and four-second adjacency bound, transcript/cuts fingerprints and a reviewed saved-plan requirement. Cookie detection/parser/transport are removed; standalone, bulk and clean compatibility paths refuse before HTTP.
- Ship uses nested export.exportId, full segment encoding, uncached polling and bounded waits. Tests check real file bytes, private mode, no overwrite, missing/failed status, timeout, raw SRT/VTT, signatures, truncated transfer cleanup and cross-origin redirect refusal. CDN requests carry no API credentials.
- Agent checks credits before/after, refuses implied consent and unattended verify/dogfood spend, and requires an intentional video target. Thin execute previews safely then refuses chat starts; the runtime mirror accepts only boolean yes:true. Shared API request keys remain stable across retries, distinct across writes, overrideable and absent from ordinary reads/presigned upload requests.
- Actual WebP files and compact human timeline output coexist with declarative storyboard/apply-storyboard. MCP file parameters are scoped to server tella-artifacts basenames; CLI paths remain operator-selected.

## Verification


| Gate | Evidence |
| --- | --- |
| go test -count=1 ./... | <local-evidence> |
| go vet ./... | <local-evidence> |
| go build ./... | <local-evidence> |
| CLI binary build | <local-evidence> |
| MCP binary build | <local-evidence> |

Go commands use the mandated external GOCACHE/GOTMPDIR/TMPDIR. Slice evidence is in T2-core-*, T2-review-* and T2-lessons-* logs. The initial core test run found the old no-write-retry expectation; it was updated to verify the new keyed-write retry-budget contract. Review, lessons and final full-suite runs pass. No test failure was hidden.

Compiled CLI help passed **134/134 paths**, including **125/125 retained paths**. The compiled MCP stdio initialize/tools/list surface returns **73 tools**, including every new workflow mirror and the explicit Agent consent parameter. These runs used an unavailable loopback API base and no live credentials; no remote calls were needed. Runtime evidence: <local-evidence> and T2-mcp-runtime.json. Both final binary paths, actual byte sizes and SHA256 values are recorded in runtime smoke; binaries exist only at <local-evidence> and <local-evidence>

All required findings 1, 2, 3, 5, 6 and 7 are resolved, with targeted regressions; optional 4, 8 and 9 are addressed as well:

1. Resolve nonempty case-insensitive key aliases once before retries, remove aliases from wire headers, send exactly one canonical Idempotency-Key, reject conflicting aliases and preserve caller maps. Tests cover nil Config, empty/lower/mixed/X aliases, each write verb, 429 retry stability, caller override and reads without keys.
2. Empty video planning returns clips:[]; explicit apply fails before snapshot creation, edits or preview, and cannot claim applied:true.
3. A timeline response must match the requested video identity before its child clips can be used.
4. Reviewed retake files may remove any candidates, including all. Remaining entries must be exact, duplicate-free fresh candidates with matching video/clip/version/fingerprint; added/altered/stale entries are refused.
5. Read at most 1 MiB plus one byte and reject oversize before decoding. Synthetic LimitReader EOF cannot hide trailing padding, unknown fields or another document.
6. Validate required non-null, nonnegative, nondecreasing timelineStartMs offsets on the fresh timeline. Chapter/thumbnail times use these actual offsets, rather than inferred duration sums. The end-to-end fixture deliberately differs from a sum: edited chapter starts are 7s/81s and the thumbnail is 7000ms.
7. Retain the existing case-insensitive failed/error check and prove uppercase/mixed-case failure never returns a leftover download URL.
8. Source-only syntax works without a dummy video: videos storyboard-image --source src_example --output sheet.webp. Video/clip targets still require a video; old paths and syntax remain accepted. The rebuilt MCP schema confirms video is optional.
9. Retain MIME parameter parsing plus mandatory signatures and credential/redirect guards; regressions download parameterized MP4/WebP responses. Explicit global --timeout now controls CDN transfers, including 0 for no whole-transfer deadline; the existing 10-minute default remains and caller cancellation still applies.

On one owned pp-test duplicate of a real recording, lessons pass --recipe RECIPE --apply completed (exit 0), changed duration from 87.326s to 62.864s, and fresh reads proved Inter 64 captions enabled, music present, two chapters and a 62ms thumbnail inpoint. The generated live preview loaded with HTTP 200. lessons ship produced an 8,722,598-byte MP4 and 2,526-byte SRT. ffprobe found H.264/AAC at 1440x1080 and 62.88s; head confirmed SRT numbering/timestamps. The 960x720, 132,628-byte WebP storyboard was opened and inspected.

Actual MCP tools/call verified automatic/lesson/retake planning, HTTP 200 preview, storyboard download and MP4/VTT delivery. Explicit chat consent and thin execute denial were verified without another paid call. Exactly one paid Agent chat was started; balance was 9000 before, 9000 immediately after and 8958.61185 after completion (observed spend 41.38815).

videos auto --fillers --silences fast --apply followed by videos clips undo-last-cuts --apply --confirm-exclusive-editing restored exact prior cuts, verified by fresh read. A separate videos clips clean --range 1000:1100 --apply (one successful cut) followed by videos clips restore-cuts restored exact prior cuts. Both lifecycle proofs are adjacent. Retakes contained zero candidates; no semantic or retake-cut success is claimed.

Expanded matrix: 288 CLI paths and 73 runtime MCP tools, pagination terminated and names unique; 85 pass, zero fail, 276 explicit skips before supplemental workflow results. Current source passes all tests, vet, build, secret and reachable-vulnerability scans. T3 fixes registration/data-source annotations and preserves all 288 paths. Static dogfood remains blocked by a pinned Press false positive: the valid OpenAPI operation ID videos.search is interpreted as a hostname. That finding remains visible. Press was not rebuilt or installed.

Raw outputs, recording text, signed URLs and delivery files are excluded in <local-evidence> Preview URLs are bearer credentials and are omitted here. Cleanup, shipcheck, acceptance, publication, final-head review and installed-binary retest remain outstanding; no completed release is claimed. Non-cut effects retain reference-state evidence rather than full-editor rollback.

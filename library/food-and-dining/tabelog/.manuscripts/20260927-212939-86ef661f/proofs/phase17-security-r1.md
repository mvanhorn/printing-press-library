# Security review — round 1

Result: **3 findings**, all originating in generator templates. No additional concrete security bug was found in the handwritten source, domain, or notebook paths reviewed. No product files were changed.

Review path: direct delegated security reviewer. Read phase 17 instructions, project AGENTS.md, source/saved-list docs, and the implementation contract. Reviewed active CLI/source transport and input paths, MCP registration and recipe handlers, generated client/delivery/store boundaries, normalized records, notebook SQL/transactions, and both command entrypoints. Generator-reserved `internal/cliutil` and `internal/mcp/cobratree` implementations were excluded.

## S1 — P1: MCP recipe positionals can become global flags

**Location:** `internal/mcp/intents.go:112–117`, reached by `handleRefreshOneSavedCandidate` at lines 85 and 90 and compare at line 61.

**Observed:** An actual stdio MCP call to `refresh_one_saved_candidate` with `slug="security-review-empty-list"`, `id="--dry-run"` returned a successful dry-run plan with `ids: []`, rather than rejecting the invalid ID. The helper directly appends untrusted values to argv:

```text
lists refresh security-review-empty-list --dry-run --agent
```

Safe reproducer output is [phase17-security-r1-mcp-repro.json](phase17-security-r1-mcp-repro.json). It used a temporary companion MCP binary and disposable configuration/data/cache roots; no files were created in that home. No webhook, delivery, or source request was invoked.

**Impact and plausible trigger:** The ID parameter is meant to select one saved restaurant. Supplying `--deliver=webhook:<url>` instead would select a hidden output sink while leaving the list as the only positional argument. The refresh command then selects the whole list and successful command output is delivered by `internal/cli/root.go:189–194`; its payload includes saved notes. That delivery impact is inferred from the inspected argv, argument-count, refresh-selection, and delivery code; only the harmless dry-run trigger was executed. A leading-dash slug can likewise change global options instead of naming a list.

**Small fix:** Keep all generated/static flags before a `--` separator, then append recipe positionals as literal arguments. Alternatively reject option-shaped positionals explicitly; validate the refresh ID as an ID before spawning the child. Verify that `--dry-run` and `--deliver=…` as input values are treated as data or rejected, while valid list/ID calls still work.

**Template provenance:** Unmodified upstream v4.32.5 `internal/generator/templates/mcp_intents.go.tmpl:386–391`, invoked by emitted positional bindings at line 193. Local source is under `/Users/zjsng/printing-press/toolchain-fixes/20260927-212939-86ef661f/cli-printing-press-v4.32.5-local-install`; upstream tag commit `7298ce9b1198c6689ae9a850e75b2ca2043c044a`. File as a template-shape candidate and repair durably; changing only the printed helper would leave reprints vulnerable.

## S2 — P2: Generated nonbinary transport reads the entire response before any cap

**Location:** `internal/client/client.go:1330`; `decodeContentEncoding` at lines 1605–1608 returns identity/unencoded bodies unchanged.

**Observed by inspection:** The generated request path calls `io.ReadAll(resp.Body)` without a limited reader. Its 32 MiB limit is applied only while manually inflating gzip/deflate. Unencoded bodies and net/http transparently decoded bodies have no byte cap. This path is still callable through advanced `restaurants get` (`internal/cli/restaurants_get.go:70`); hiding the command does not remove its transport.

**Impact and plausible trigger:** An unexpectedly large or endless successful/error HTML response from an advanced endpoint grows the CLI's allocation until EOF, timeout, or OOM. A timeout bounds duration, not allocated bytes. The handwritten source client correctly enforces a decoded 4 MiB cap, so the main `find/show/areas/cuisines` paths are not affected by this particular defect.

**Small fix:** Bound nonbinary response reads before allocation and reject an oversized body explicitly; apply the same decoded-size check to identity responses and transparent decompression. Preserve the documented binary/stream behavior separately instead of imposing an unrelated whole-call download deadline. No oversized response was requested in this review.

**Template provenance:** `internal/generator/templates/client.go.tmpl:2445` and `:3626` in the same upstream/local source. File as a template-shape candidate, not a handwritten source bug.

## S3 — P2: Delivery's predictable temporary filename follows symlinks

**Location:** `internal/cli/deliver.go:147–151` (`writeDownloadUnder`), callable by `deliverFile` at line 116.

**Observed by inspection:** The function writes `path + ".tmp"` using `os.WriteFile`, which opens an existing target and follows a symlink, then renames that path. The safe single-segment check on the final basename does not protect the temporary file.

**Impact and plausible trigger:** If another local user can create `trip.json.tmp` in a shared writable output directory before the operator uses `--deliver=file:/tmp/trip.json`, the temporary path can point at an unrelated file writable by the CLI user. The write truncates that file before rename. Concurrent deliveries to the same filename can also overwrite each other's temporary data. Delivery was not invoked, as requested by the parent.

**Small fix:** Use `os.CreateTemp` in the destination directory with a random, exclusively created name; write, close, and rename that file, checking errors and cleaning up on failure. This keeps atomic replacement without following an existing temporary symlink.

**Template provenance:** `internal/generator/templates/deliver.go.tmpl:146–150` in the same source. File as a template-shape candidate, not a handwritten notebook bug.

## Boundaries that held

- Native source validation requires exact HTTPS `tabelog.com` and `/en/`, rejects userinfo/dot traversal, validates canonical detail identities and pagination links, and rejects cross-origin/downgrade redirects. Replay transport requires explicit test mode, an HTTP loopback IP, and a port.
- Handwritten live commands pass `boundCtx` through source requests and storage work. Native HTTP calls have a 20-second client timeout and decoded 4 MiB body limit; find is limited to five pages, refresh to twenty saved venues/two workers and a twenty-second deadline. No retry is added for blocks, drift, or 429.
- Notebook names/IDs/notes are validated, SQL values are bound parameters, source snapshot replacement is transactional, and notes/membership are separate. Source data and notes are JSON data; no shell or code evaluation was found in those paths. Failed refreshes preserve saved state.
- MCP serves stdio when explicitly launched. Registered tools use the intended domain CLI surface; legacy raw SQL/search handler functions are not registered. The recipe positional defect above is the active exception to the intended argument boundary.
- The provided [67-case replay proof](e2e-post-review-fixes.jsonl) was inspected for coverage context, including redirect, machine-diagnostic, notebook, partial-refresh, and dry-run cases. It was not rerun. The only new executable check was the safe MCP reproducer.

Out-of-scope retro candidates: none observed. No product edits, phase receipts, full-suite runs, or origin probes were made.

# Hand-edit reprint-guards (2026-07-11)

Applied after generation; re-apply if regen clobbers them. All are documented non-defect fixes.

## database_search.go
- Added `pp:no-error-path-probe: "true"` to Annotations. Discogs `/database/search` accepts any query string, so there is no invalid argument that yields a non-zero exit; the dogfood error_path probe is not applicable. (Matrix honors this annotation.)

## inventory_export_get.go / inventory_export_download.go / inventory_upload_get.go
- Added `pp:typed-exit-codes: "0,3"` to Annotations. These are get-by-id reads; a synthesized/nonexistent id correctly returns 404 → exit 3, which is a valid typed outcome, not a failure. The upstream live-dogfood matrix now honors pp:typed-exit-codes for happy_path/json_fidelity (fixed 2026-07-11, see retro F1), so these now pass full dogfood. The annotation is correct and forward-compatible.

## teach.go — teach-pattern / teach-lookup Annotations
- Added `pp:typed-exit-codes: "0,2"` and `mcp:local-write: "true"` to the `teach-pattern` and `teach-lookup` command Annotations. Both are local-store writers that exit 2 via `usageErr` on missing required flags, exactly like their annotated siblings `teach` / `teach-playbook` / `playbook amend`; the generator template omitted the whole Annotations field for these two. Once the live-dogfood matrix honors typed-exit-codes (retro F1), the missing `teach-pattern` declaration surfaced as a full-dogfood FAIL (`teach-pattern` exit 2 on a flagless invocation). Durable fix is the generator template (`internal/generator/templates/teach.go.tmpl`), landed alongside the matrix fix; this local patch matches that output so a regen carries it forward.

## gosec #nosec suppressions (17, across internal/client, internal/store, internal/learn, internal/config, internal/cli teach*/feedback)
- All 17 gosec findings were false-positives in generated framework files (G304 config/CLI file reads, G201/G202 internal SQL with constant identifiers + parameterized values, G119 deliberate auth-redirect handling, G104/G117 best-effort ops). Each suppressed with a justified inline `#nosec RULE -- reason`. gosec now reports Issues: 0. Durable fix is a generator template change (see retro).

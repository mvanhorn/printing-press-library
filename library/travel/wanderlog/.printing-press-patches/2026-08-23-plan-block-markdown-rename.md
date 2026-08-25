# 2026-08-23 Plan Block Markdown And Rename

## Intent

Preserve P0 agent-edit surfaces across reprints:

- `plan block edit-text` writes plain Quill text unless `--markdown`. Formatted input (`**`, line-start `- ` / `* ` / `# `) fails closed without that flag.
- `--markdown` compiles a small markdown subset to Quill Delta: `**bold**`, `- ` / `* ` bullets (`list:bullet` on the newline), and `# ` headings as bold label lines. Never emit Quill `header` attributes; strip them if they appear and report `stripped: ["header"]`.
- `plan block rename` is the dedicated write path for `place.name` (JSON0 object set). `plan block set-field` keeps `place` protected and points at rename.

## Touched Surface

- `internal/cli/plan_edit.go`: register `rename`; markdown/plain note helpers; `stripped` on edit reports.
- `internal/cli/plan_edit_more.go`: `edit-text --markdown`; `plan block rename`; set-field `place` policy.
- `internal/cli/plan_edit_test.go`: compiler, fail-closed, header strip, and rename op tests (no live ShareDB).

## Verification

- `go test ./internal/cli/ -count=1 -timeout 120s`

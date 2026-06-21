# Local verification

Validated locally on 2026-06-22.

- `go test ./...` passed.
- `cli-printing-press verify-skill --dir library/marketing/postscript` passed.
- `cli-printing-press validate-narrative --strict --full-examples --research library/marketing/postscript/.manuscripts/20260621-194030/research/research.json --binary library/marketing/postscript/postscript-pp-cli` passed.
- `cli-printing-press dogfood --dir library/marketing/postscript --research-dir library/marketing/postscript/.manuscripts/20260621-194030/research --json` passed with `planned: 3` and `found: 3` novel features.
- `cli-printing-press shipcheck --dir library/marketing/postscript --research-dir library/marketing/postscript/.manuscripts/20260621-194030/research --no-live-check --json` passed.
- `cli-printing-press publish validate --dir . --json` passed from the package directory.

Live Postscript API behavior was not checked because credentials were unavailable; see `phase5-skip.json`.

# Local verification

Validated locally on 2026-06-22.

- `go test ./...` passed.
- `cli-printing-press verify-skill --dir library/marketing/attentive` passed.
- `cli-printing-press validate-narrative --strict --full-examples --research library/marketing/attentive/.manuscripts/20260621-194217/research/research.json --binary library/marketing/attentive/attentive-pp-cli` passed.
- `cli-printing-press dogfood --dir library/marketing/attentive --research-dir library/marketing/attentive/.manuscripts/20260621-194217/research --json` passed with `planned: 3` and `found: 3` novel features.
- `cli-printing-press shipcheck --dir library/marketing/attentive --research-dir library/marketing/attentive/.manuscripts/20260621-194217/research --no-live-check --json` passed.
- `cli-printing-press publish validate --dir . --json` passed from the package directory.

Live Attentive API behavior was not checked because credentials were unavailable; see `phase5-skip.json`.

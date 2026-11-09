# dialer patch-settings: typed body with wrapupAutoReleaseSeconds (2026-10-15)

- `spec.yaml` `/dialer/settings` PATCH now carries the typed request body exactly as the app's `public/openapi.yaml` documents it at app main
  ce2d3cdff (live 2026-10-15 8:59 AM PT): autoAdvanceEnabled, pauseSeconds (1..60), advanceOn, openLeadOnDial, smartDialCallerId and the new
  `wrapupAutoReleaseSeconds` (0 = off, otherwise 30..3600; default 300). Only the two dialer settings operations were looked at; GET is unchanged.
- `internal/cli/dialer_patch-settings.go` and `tools-manifest.json` are the Printing Press 4.32.1 output for that one spec change (generated from the
  old and the new spec.yaml into scratch directories, the difference applied here), so the 4.28.0 print and its hand-fixes are untouched. The
  diff between the two scratch trees was exactly those two files. New flags: `--wrapup-auto-release-seconds` (and the typed `--pause-seconds`,
  `--advance-on`, `--auto-advance-enabled`, `--open-lead-on-dial`, `--smart-dial-caller-id`). A flag that was passed is always sent, so `0` (off) is
  never dropped.
- Hand-fix: the generator cut the flag help at the first ".." of "30..3600" (it printed "otherwise 30.."); the usage text is now
  "...0 = off, otherwise 30 to 3600 (default 300)". A reprint must keep this (pinned by `TestDialerPatchSettingsWrapupFlagIsDocumented`).
- `internal/mcp/code_orch.go` needs no change: code orchestration binds path and query params only, and the PATCH body is passed through as given.
- The scratch `printing-press generate` runs ended on the generator's own govulncheck gate (Go 1.26.6 stdlib advisory in the scratch toolchain),
  after the project was written; its own `go test ./...` step passed. This tree's gates are unchanged.
- No version bump: this repo carries no version of its own (manifest.json stays 1.0.0 and the CHANGELOG is release-automation owned); the print
  version stays 4.28.0 in `.printing-press.json`, as the earlier parity lane left it.
- Tests: `internal/cli/dialer_patch_settings_test.go` (valid cases, other flags, flag help, manifest param, and the round 2 invalid matrix).

## Round 2 hand-fix: client-side validation of wrapupAutoReleaseSeconds (Codex round 1 P1, 2026-10-15)

The generated flag was a pflag IntVar, whose base-0 parser reads `0x1e` as 30 and lets 29, 3601 and negatives through, and the `--stdin` path
unmarshals into `map[string]any`, which turns `1e2` into 100. Relying on the API's 400 is not enough: a value could be silently rewritten into a
different, valid one. So this is now a hand-fix on a generated file (a reprint must keep it; the tests fail if it does not):

- `internal/cli/dialer_wrapup_validate.go` (hand-written): `parseWrapupSeconds` accepts only digits (1 to 5 of them, surrounding whitespace trimmed)
  and then exactly 0 or 30..3600; `wrapupSecondsFromJSON` reads the field's RAW JSON token from the `--stdin` body, so `1e2`, `12.5`, `-1`,
  strings, null, booleans and arrays are refused.
- `internal/cli/dialer_patch-settings.go`: the flag is a string (`StringVar`, not `IntVar`); validation runs first in `RunE`, before
  `flags.newClient()` and before any request, on the flag path (only when the flag was passed, so `0` is sent) and on the `--stdin` path (the
  validated integer is written back into the body). Stdin is read from `cmd.InOrStdin()` (the same as `os.Stdin` by default) so it can be tested.
  A bad value is a usage error (exit 2).
- Tests: `TestDialerPatchSettingsWrapupInvalidFlagValuesSendNothing` (19 invalid texts, zero server hits) and
  `TestDialerPatchSettingsWrapupStdinValidation` (14 invalid JSON values with zero hits; 0, 30, 300, 3600 sent as numbers; a body without the
  field passes through untouched), plus the earlier valid-case tests.

## Round 3 hand-fix: malformed --stdin JSON is a usage error (Codex round 2 P2, 2026-10-15)

`wrapupSecondsFromJSON` used to swallow the JSON syntax error (returning nil, nil), so a body such as `{"wrapupAutoReleaseSeconds":0x1e}` passed
validation, the client was created, and the later parse failure exited 1. It now returns the `json.Unmarshal` error ("parsing stdin JSON: ..."),
which the existing `usageErr` turns into exit 2 before `flags.newClient()`. Tests assert `ExitCode(err) == 2` and zero server hits for every
invalid flag text, every invalid stdin value and a malformed-body matrix (`TestDialerPatchSettingsMalformedStdinIsUsageErrorBeforeAnyRequest`).

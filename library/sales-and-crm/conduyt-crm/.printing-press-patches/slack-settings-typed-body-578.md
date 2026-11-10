# integrations post-slack-settings: typed body with channel and notifyEvents (board #578, 2026-10-28)

- The app route POST /api/v1/integrations/slack/settings changed under lane small563 (app main 00749249c, live 2026-10-28 11:40 AM PT): an omitted
  `notifyEvents` keeps the stored list, an explicit `[]` clears every event, a non-array or null is a 400, and the response reports the stored
  list. The live https://conduyt.app/openapi.yaml now types the request body (`channel`, `notifyEvents`). This tree's spec.yaml and
  upstream/openapi.yaml carried the untyped body, so the command had only `--stdin`.
- `spec.yaml` and `upstream/openapi.yaml`: ONLY this operation's requestBody schema was replaced with the live typed one (responses unchanged).
  The rest of the live document was NOT adopted: it differs from this tree by 98 operations, which is a separate reprint.
- `internal/cli/integrations_post-slack-settings.go` and `tools-manifest.json`: new flags `--channel` (string) and `--notify-events`
  (StringSlice: comma list or repeated). Hand-written in the shape the generator emits for a typed body, because a full reprint is out of scope.
  Hand-fixes a reprint must keep (pinned by `internal/cli/integrations_post_slack_settings_test.go`):
  - `cmd.Flags().Changed("notify-events")` decides whether the field is sent, so `--notify-events ""` or `--notify-events=` sends `[]`
    (clear every event) and is never dropped; a flag that was not passed is not sent (keep). The same for `--channel ""` (clear the destination).
  - `Long` states the keep / clear / 400 semantics.
  - `--stdin` reads `cmd.InOrStdin()` (same as `os.Stdin` by default) so it can be tested; its body is passed through untouched, so a non-array
    `notifyEvents` reaches the API and gets its 400.
- `internal/mcp/code_orch.go` needs no change: code orchestration binds path and query params only, the body is passed through.
- No version bump and no CHANGELOG edit: CHANGELOG.md and .printing-press-release.json are release-automation owned (AGENTS.md).

# Shared Drive-folder resolution helper

## Why this patch belongs in the printed tree

`drive import` and `episode import` both need to resolve a "Drive folder"
argument. There is no Google Drive API OAuth flow in this CLI -- Flow's
harvested session token carries no `drive.readonly` scope -- so both
commands resolve their folder argument against a locally mounted Google
Drive for Desktop path instead (`resolveDriveFolder` in
`internal/cli/drive_folder.go`), and reject a bare Drive folder ID with an
actionable error rather than attempting an unsupported live lookup.
`drive_folder.go` also holds `fetchProjectContents`, the shared
`flow.projectInitialData` fetch used by both `scenes gaps` and
`drive import --tag-scene`'s character-name matching.

## Reprint guard

On reprint, preserve or regenerate:

- `internal/cli/drive_folder.go` in full (`resolveDriveFolder`,
  `fetchProjectContents`)
- the `--folder-id` (drive import) and `--scribe-folder`/`--images-folder`
  (episode import) flag descriptions referencing local-path resolution,
  not a Drive API ID lookup
- `characterDisplayNames` in `internal/cli/scenes_gaps.go`, which
  `drive_import.go`'s `--tag-scene` handling depends on

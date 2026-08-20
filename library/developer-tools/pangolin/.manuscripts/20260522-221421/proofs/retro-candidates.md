## Generator bug: `V` prefix inserted on digit-leading path segments

When a path segment begins with a digit (e.g. `/user/{userId}/2fa`), the
generator's promoted-command wiring emits `newUserV2faCmd` while the actual
function in `user_2fa.go` is `newUser2faCmd`. Names disagree.

- File: `internal/cli/promoted_user.go:101`
- Function defined as: `newUser2faCmd` in `internal/cli/user_2fa.go`
- Function referenced as: `newUserV2faCmd` in `internal/cli/promoted_user.go`
- Spec: `POST /user/{userId}/2fa`

The fix should normalise digit-leading identifiers consistently across both
sites. File against the Printing Press for retro.

Patched in place after generation; fix is fragile (regen will overwrite).

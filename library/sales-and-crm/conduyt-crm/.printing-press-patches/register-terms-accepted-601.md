# conduyt-auth register: required --terms-accepted (board #601, 2026-10-29)

- POST /api/v1/auth/register requires `termsAccepted: true` (boolean const true; a missing or other value is a 400, field `termsAccepted`, checked
  before the rate limit is charged) since app lane #575. Until the CLI sent it, every CLI registration answered 400.
- `spec.yaml` and `upstream/openapi.yaml`: ONLY this operation's requestBody schema was replaced with the one from the app export at main 9aef66389
  (adds `termsAccepted`, `signupFunnelId`, `attribution`; `termsAccepted` is in `required`). The rest of that export differs from this tree by many
  operations, which is a separate reprint (same scoping as #578). The two files carry a deep-equal requestBody (pinned by a test).
- `internal/cli/conduyt-auth_register.go` and `tools-manifest.json`: new bool flag `--terms-accepted`, hand-written in the shape the generator emits.
  Hand-fixes a reprint must keep (pinned by `internal/cli/conduyt_auth_register_test.go`):
  - without the flag (or `--terms-accepted=false`) and without `--stdin`, the command returns a local error naming the Terms of Service and the
    Privacy Policy and sends no request; `--dry-run` skips the check like the other required flags.
  - any positional argument is refused locally (`Args`), with a message naming `--terms-accepted` / `--terms-accepted=true`: pflag parses `--terms-accepted false` as the bare flag plus a stray `false`, which would register with termsAccepted true.
  - with the flag the body carries `"termsAccepted": true`.
  - `Long` and the flag usage explain the consent; `--stdin` reads `cmd.InOrStdin()` and is passed through untouched (the body must carry
    `termsAccepted` itself, else the API answers 400).
- Not added (narrowest option): `--signup-funnel-id` and `attribution`, which are optional.
- No version bump and no CHANGELOG edit: release-automation owned (AGENTS.md).

## Reprint 637 (2026-10-30)

The print now also emits `--signup-funnel-id` and the `--attribution-*` flags from the live body; they are kept. `--terms-accepted`
stays the hand-written bool with the local refusal (the print emits a string flag parsed with ParseBool).

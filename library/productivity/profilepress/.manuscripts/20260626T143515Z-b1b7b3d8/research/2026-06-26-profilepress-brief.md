# ProfilePress brief

ProfilePress is a plan-driven Printing Press CLI for operating a user's own LinkedIn account through a user-controlled browser-session model.

## Product posture

- Local-first profile snapshots and change packets.
- Default-private profile edits.
- Explicit opt-in for LinkedIn network notifications.
- Draft-first LinkedIn messaging.
- No credential, cookie, or token collection.
- No auth bypass or bulk social automation.

## Novel features

| Command | Name | Description |
|---------|------|-------------|
| `profilepress apply-packet` | Default-private LinkedIn profile packets | Profile edits default to not notifying the network; notifying requires `--notify-network` plus `--confirm-notify NOTIFY-NETWORK`. |
| `profilepress messages draft` | Draft-first LinkedIn messaging | Messages are stored locally as drafts and require `--confirm-send SEND-MESSAGE` before simulated/live sending. |

## Verification basis

The promoted print was validated with Go tests, build, Printing Press structural verification, publish validation, and focused local workflow smoke tests. Live LinkedIn account mutation/send adapters remain disabled in the published artifact pending explicit browser-session implementation.

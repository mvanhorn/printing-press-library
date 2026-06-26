---
name: pp-profilepress
description: "Printing Press CLI for ProfilePress. Safely operate a user's own LinkedIn profile and messages through local change packets, default-private profile updates, and explicit-send workflows."
author: "mgmarcusa"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - profilepress-pp-cli
---

# ProfilePress — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `profilepress-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install profilepress --cli-only
   ```
2. Verify: `profilepress-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/profilepress/cmd/profilepress-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When Not to Use This CLI

Do not use this CLI for bulk automation, scraping at scale, spam, credential extraction, cookie export, bypassing LinkedIn authentication, or acting on an account the user does not control.

## Command Reference

**Profile packets** — local-first profile edits.

- `profilepress-pp-cli snapshot` — Capture current profile sections into the local mirror from a fixture or user-controlled browser session.
- `profilepress-pp-cli propose-for-job` — Create an opportunity-specific profile change packet.
- `profilepress-pp-cli diff` — Show a human-readable before/after diff.
- `profilepress-pp-cli privacy-check` — Verify whether profile-update broadcast status is safe before writes.
- `profilepress-pp-cli apply-packet` — Apply an approved packet only after privacy preflight, sensitive-change confirmation, and final user approval.
- `profilepress-pp-cli packet export` — Export packet + snapshot data for review.

**Messaging** — draft-first LinkedIn messaging.

- `profilepress-pp-cli messages draft` — Create a local-only message draft.
- `profilepress-pp-cli messages list` — List local drafts and send logs.
- `profilepress-pp-cli messages send` — Send only with explicit confirmation. Current live adapter is disabled; `--simulate-live` is for local workflow testing only.

**Support**

- `profilepress-pp-cli auth status` — Explain auth posture.
- `profilepress-pp-cli doctor` — Check local environment.

## Auth Setup

ProfilePress uses a user-controlled browser-session model. It must not collect passwords, cookies, OAuth tokens, or API secrets. A future live adapter should attach to a browser the user already controls and should never export session material.

Run:

```bash
profilepress-pp-cli auth status
profilepress-pp-cli doctor
```

## Agent Mode

Prefer JSON-producing commands and explicit confirmations. ProfilePress is intentionally non-bulk and confirmation-safe:

- Profile applies require privacy status and sensitive-change confirmation.
- Default network notification state is off: do not pass `--notify-network` unless the user explicitly requests a public network notification.
- If notifying is explicitly desired, pass `--notify-network --confirm-notify NOTIFY-NETWORK`.
- Message sends require `--confirm-send SEND-MESSAGE`.

Examples:

```bash
profilepress-pp-cli snapshot --fixture profile.json
profilepress-pp-cli propose-for-job --change 'headline=New headline' --source-note 'user-provided edit'
profilepress-pp-cli diff
profilepress-pp-cli apply-packet --privacy-status disabled --dry-run --confirm-sensitive APPLY-SENSITIVE
profilepress-pp-cli messages draft --to https://www.linkedin.com/in/example --body-file message.md
profilepress-pp-cli messages send --draft msg_123 --dry-run
```

## Paths and state

By default, local state is stored in:

```text
~/.local/share/profilepress/profilepress.db
```

Set `PROFILEPRESS_DB=/path/to/profilepress.db` or pass `--db /path/to/profilepress.db` to isolate runs.

## Safety defaults

- No credential capture.
- No hidden tokens.
- No network notification by default.
- No message sending by default.
- No bulk social actions.
- Live LinkedIn mutation and live message send adapters remain disabled until explicitly implemented and tested through a user-controlled browser session.

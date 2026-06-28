# profilepress-printed

This directory is the Printing Press generated `profilepress` CLI, augmented with the CEE safety workflow for private LinkedIn operations.

Generated with:

```bash
cli-printing-press generate --plan /home/mgmarcusa/docs/plans/2026-06-26-001-feat-linkedin-private-profile-cli-printing-press-plan.md --name profilepress --output /home/mgmarcusa/profilepress-printed --json
```

## Product stance

`profilepress` operates a user's own LinkedIn account through a user-controlled browser/session model. It does not collect credentials, export cookies, bypass LinkedIn authentication, or bulk-automate social actions.

Default safety posture:

- Profile changes are local change packets until explicitly applied.
- Network notification is off by default.
- `--notify-network` requires `--confirm-notify NOTIFY-NETWORK`.
- Message sending is draft-first and requires `--confirm-send SEND-MESSAGE`.
- Live profile writes are available for supported LinkedIn sections only (`headline` and `about`) through `--live-linkedin`.
- Experience live writes fail closed until ProfilePress emits position-level packets instead of one monolithic Experience blob.
- `--simulate-live` is only for local workflow testing.

## Implemented commands

- `snapshot`
- `privacy-check`
- `propose-for-job`
- `diff`
- `packet export`
- `apply-packet`
- `messages draft`
- `messages list`
- `messages send`
- `auth status`
- `doctor`

## Examples

Create a safe profile edit packet from a fixture:

```bash
profilepress snapshot --fixture profile.json
profilepress propose-for-job --change 'headline=Principal Researcher @ Meta | Human-Centered AI Evaluation & Alignment' --source-note 'user requested removing (UX)'
profilepress diff
profilepress apply-packet --privacy-status disabled --dry-run --confirm-sensitive APPLY-SENSITIVE
profilepress apply-packet --live-linkedin --profile-url 'https://www.linkedin.com/in/example/' --privacy-status disabled --confirm-sensitive APPLY-SENSITIVE --confirm-apply APPLY
```

`--live-linkedin` is not browser steering: it imports the existing local Chrome LinkedIn session and calls LinkedIn-specific SDUI profile-save requests. It currently supports `headline` and `about`; it refuses `experience` until position-level Experience packets are implemented.

Capture a real authenticated LinkedIn profile read-only from your existing Chrome session:

```bash
# Uses your already-authenticated local Chrome LinkedIn session.
# Does not drive, close, relaunch, or remote-control Chrome.
profilepress snapshot \
  --chrome-session \
  --profile-url 'https://www.linkedin.com/in/example/' \
  --expect-name 'Example Person'
```

Fallback: capture from a ProfilePress-owned browser profile:

```bash
# First run opens an isolated browser profile under ~/.local/share/profilepress.
# Log into LinkedIn there once if prompted. Your normal Chrome is not touched.
profilepress snapshot \
  --managed-browser \
  --profile-url 'https://www.linkedin.com/in/example/' \
  --expect-name 'Example Person'
```

Advanced fallback for users who already started a separate Chrome/Chromium with CDP enabled:

```bash
profilepress snapshot \
  --browser-cdp \
  --cdp-url 'http://127.0.0.1:9222' \
  --profile-url 'https://www.linkedin.com/in/example/' \
  --expect-name 'Example Person'
```

The browser paths evaluate visible page text only (`document.body.innerText`, URL, and title), validate that the page is a LinkedIn `/in/<slug>` profile and contains the expected name when provided, then store the parsed sections plus `raw_text` in the local SQLite mirror.

Explicitly allow network notification only when intended:

```bash
profilepress apply-packet --packet pkt_123 --privacy-status disabled --notify-network --confirm-notify NOTIFY-NETWORK --confirm-sensitive APPLY-SENSITIVE --confirm-apply APPLY
```

Draft and send a LinkedIn message safely:

```bash
profilepress messages draft --to https://www.linkedin.com/in/example --body-file message.md
profilepress messages send --draft msg_123 --dry-run
profilepress messages send --draft msg_123 --confirm-send SEND-MESSAGE --simulate-live
```

## Publishing to Printing Press library

Publishing path:

1. Make validation green locally:

```bash
export PATH="/home/mgmarcusa/.local/go/bin:/home/mgmarcusa/.local/bin:$PATH"
go test ./...
go build -buildvcs=false -o bin/profilepress ./cmd/profilepress
cli-printing-press verify --dir /home/mgmarcusa/profilepress-printed --no-spec --cleanup
cli-printing-press dogfood --dir /home/mgmarcusa/profilepress-printed --json
cli-printing-press publish validate --dir /home/mgmarcusa/profilepress-printed --json
```

2. Package it for review:

```bash
cli-printing-press publish package --dir /home/mgmarcusa/profilepress-printed --category productivity --target /tmp/profilepress-publish --json
```

3. Open a PR against `mvanhorn/printing-press-library` with the packaged `library/productivity/profilepress` tree.

The upstream docs prefer the `/printing-press-publish profilepress` skill when available; in this environment we use the equivalent `cli-printing-press publish` commands directly.

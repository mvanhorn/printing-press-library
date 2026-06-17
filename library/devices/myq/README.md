# MyQ CLI

Control a myQ Smart Garage Opener from the terminal with a logged-in myQ account.

The CLI lists devices, checks door state, and sends open/close actions through the legacy myQ login flow. Basic access does not require a paid subscription.

Created by [@erikrogne](https://github.com/erikrogne) (Erik Rogne).

## Install

Install the CLI and the matching skill in one shot:

```bash
npx -y @mvanhorn/printing-press-library install myq
```

CLI only:

```bash
npx -y @mvanhorn/printing-press-library install myq --cli-only
```

Skill only:

```bash
npx -y @mvanhorn/printing-press-library install myq --skill-only
```

### Without Node

If `npx` is unavailable, install the CLI directly via Go:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/myq/cmd/myq-pp-cli@latest
```

## Quick Start

```bash
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli devices
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli state 1234567890
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli open 1234567890
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli close 1234567890
```

## Commands

- `devices` - List the accounts and garage devices visible to your myQ login.
- `state` - Fetch the current door state for a specific device serial number.
- `open` - Open a garage door and wait for the state to change.
- `close` - Close a garage door and wait for the state to change.

## Environment

- `MYQ_USERNAME`
- `MYQ_PASSWORD`
- `MYQ_TIMEOUT`

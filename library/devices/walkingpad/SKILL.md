---
name: walkingpad
description: Control WalkingPad through the generated BLE device CLI.
---

## Prerequisites: Install the CLI

This skill drives the `walkingpad-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install walkingpad --cli-only
   ```
2. Verify: `walkingpad-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use `walkingpad-pp-cli capabilities --json` to inspect callable and withheld BLE capabilities, including safety classes and evidence refs. Use `walkingpad-pp-cli status --json` to inspect replay-backed status output. By default the CLI is replay-backed; build with `-tags ble_live` and pass `--live` to control a real device, `walkingpad-pp-cli doctor` to check live readiness, and `walkingpad-pp-cli scan --live` to discover devices. Use `walkingpad-pp-cli start --dry-run --json` to preview the start write. To run it outside verify mode, pass `--confirm-physical-effect` after checking the dry-run output. Use `walkingpad-pp-cli stop --dry-run --json` to preview the stop write. To run it outside verify mode, pass `--confirm-physical-effect` after checking the dry-run output. Use `walkingpad-pp-cli wake --dry-run --json` to preview the wake write. To run it outside verify mode, pass `--confirm-physical-effect` after checking the dry-run output. Use `walkingpad-pp-cli set-speed <kmh> --dry-run --json` to preview the set-speed write. To run it outside verify mode, pass `--confirm-physical-effect` after checking the dry-run output. Use `walkingpad-pp-cli set-mode <mode> --dry-run --json` to preview the set-mode write. To run it outside verify mode, pass `--confirm-physical-effect` after checking the dry-run output. Session IPC scaffolding is generated only when the device spec enables device-session support.

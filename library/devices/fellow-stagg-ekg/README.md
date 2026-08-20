# Fellow Stagg EKG CLI

Control a Fellow Stagg EKG kettle over its local HTTP command interface from the terminal.

The CLI talks to the kettle at `/cli?cmd=...`, prints structured key/value responses when available, and keeps the low-level command surface available for recovery when you need it.

Created by [@erikrogne](https://github.com/erikrogne) (Erik Rogne).

## Install

Install the CLI and the matching skill in one shot:

```bash
npx -y @mvanhorn/printing-press-library install fellow-stagg-ekg
```

CLI only:

```bash
npx -y @mvanhorn/printing-press-library install fellow-stagg-ekg --cli-only
```

Skill only:

```bash
npx -y @mvanhorn/printing-press-library install fellow-stagg-ekg --skill-only
```

### Without Node

If `npx` is unavailable, install the CLI directly via Go:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/fellow-stagg-ekg/cmd/fellow-stagg-ekg-pp-cli@latest
```

## Quick Start

```bash
# Use either --host or --base-url.
fellow-stagg-ekg-pp-cli --host 192.168.1.86 status

# Fetch the current state or settings.
fellow-stagg-ekg-pp-cli --host 192.168.1.86 state
fellow-stagg-ekg-pp-cli --host 192.168.1.86 settings

# Start heating after setting a target temperature.
fellow-stagg-ekg-pp-cli --host 192.168.1.86 heat --temp 95.5

# Send a raw command for recovery or exploration.
fellow-stagg-ekg-pp-cli --host 192.168.1.86 raw "buz sos"
```

## Commands

- `status` - Show state, settings, and firmware info.
- `state` - Fetch the kettle state.
- `settings` - Fetch the kettle settings.
- `clock` - Fetch the kettle clock.
- `info` - Fetch firmware info.
- `heat` - Start heating, optionally after setting a target temperature.
- `off` - Turn the kettle off.
- `set-temp` - Set the target temperature.
- `set-setting` - Call `setsetting`, `setsettingd`, or `setsettings` directly.
- `units` - Switch the display units.
- `button` - Press or release button 1 or 2.
- `dial` - Rotate the kettle dial.
- `beep` - Run buzzer commands.
- `raw` - Send an arbitrary command string.

## Environment

- `FELLOW_STAGG_HOST`
- `FELLOW_STAGG_URL`
- `FELLOW_STAGG_PORT`
- `FELLOW_STAGG_TIMEOUT`


---
name: pp-fellow-stagg-ekg
description: "Control a Fellow Stagg EKG kettle over its local HTTP CLI. Trigger phrases: `start the kettle`, `set the kettle to 95`, `show kettle status`, `turn the kettle off`, `use fellow stagg ekg`, `run fellow stagg ekg`."
author: "Erik Rogne"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - fellow-stagg-ekg-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/devices/fellow-stagg-ekg/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Fellow Stagg EKG - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `fellow-stagg-ekg-pp-cli` binary.

Install via the Printing Press installer:

```bash
npx -y @mvanhorn/printing-press-library install fellow-stagg-ekg --cli-only
```

If `npx` is unavailable, fall back to Go:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/fellow-stagg-ekg/cmd/fellow-stagg-ekg-pp-cli@latest
```

## When to Use This CLI

Use this CLI when you need to read or control a Fellow Stagg EKG kettle from the terminal: checking status, switching units, starting heat, or sending a raw command while debugging the kettle's local HTTP interface.

## Command Reference

- `fellow-stagg-ekg-pp-cli status` - Show state, settings, and firmware info.
- `fellow-stagg-ekg-pp-cli state` - Fetch the kettle state.
- `fellow-stagg-ekg-pp-cli settings` - Fetch the kettle settings.
- `fellow-stagg-ekg-pp-cli clock` - Fetch the kettle clock.
- `fellow-stagg-ekg-pp-cli info` - Fetch firmware info.
- `fellow-stagg-ekg-pp-cli heat` - Set a target temperature and start heating.
- `fellow-stagg-ekg-pp-cli off` - Turn the kettle off.
- `fellow-stagg-ekg-pp-cli set-temp` - Set the target temperature.
- `fellow-stagg-ekg-pp-cli set-setting` - Send a direct setting update.
- `fellow-stagg-ekg-pp-cli units` - Switch the display units.
- `fellow-stagg-ekg-pp-cli button` - Press or release a button.
- `fellow-stagg-ekg-pp-cli dial` - Rotate the dial.
- `fellow-stagg-ekg-pp-cli beep` - Run the buzzer.
- `fellow-stagg-ekg-pp-cli raw` - Send a raw kettle command.

## Recipes

```bash
# Read the kettle status.
FELLOW_STAGG_HOST=192.168.1.86 fellow-stagg-ekg-pp-cli status
```

```bash
# Heat to a target temperature, then turn heat on.
FELLOW_STAGG_HOST=192.168.1.86 fellow-stagg-ekg-pp-cli heat --temp 95.5
```

```bash
# Switch the display to Fahrenheit.
FELLOW_STAGG_HOST=192.168.1.86 fellow-stagg-ekg-pp-cli units f
```

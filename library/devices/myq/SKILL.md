---
name: pp-myq
description: "Control a myQ Smart Garage Opener from the terminal. Trigger phrases: `open the garage`, `close the garage`, `show myq devices`, `check myq state`, `use myq`, `run myq`."
author: "Erik Rogne"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - myq-pp-cli
---

# MyQ - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `myq-pp-cli` binary.

Install via the Printing Press installer:

```bash
npx -y @mvanhorn/printing-press-library install myq --cli-only
```

If `npx` is unavailable, fall back to Go:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/myq/cmd/myq-pp-cli@latest
```

## When to Use This CLI

Use this CLI when you need to list your myQ devices, inspect door state, or open and close a garage door from the terminal.

## Command Reference

- `myq-pp-cli devices` - List the accounts and garage devices visible to your myQ login.
- `myq-pp-cli state <serial-number>` - Fetch the current door state.
- `myq-pp-cli open <serial-number>` - Open a garage door and wait for the state to change.
- `myq-pp-cli close <serial-number>` - Close a garage door and wait for the state to change.

## Recipes

```bash
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli devices
```

```bash
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli open 1234567890
```

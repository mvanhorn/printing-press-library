# Agent Desktop Printing Press CLI

`agent-desktop-pp-cli` makes the Rust `agent-desktop` desktop automation CLI visible in the Printing Press library.

This package does not copy or reimplement the Rust CLI. It delegates to the real `agent-desktop` binary and installs that binary through the existing remote npm package, which downloads verified GitHub release assets for the selected package version.

`agent-desktop` is a native desktop automation CLI for AI agents. It reads OS accessibility trees, returns structured JSON, assigns snapshot-scoped refs like `@e1`, and lets agents perform semantic actions such as clicking, typing, scrolling, waiting, managing windows, reading notifications, using the clipboard, and taking screenshots.

## Install

```bash
npx -y @mvanhorn/printing-press-library install agent-desktop --cli-only
agent-desktop-pp-cli --version
agent-desktop-pp-cli doctor
```

Then install the real desktop automation binary when needed:

```bash
agent-desktop-pp-cli install --version latest
agent-desktop-pp-cli doctor
```

Load the real binary's version-matched agent docs before desktop automation:

```bash
agent-desktop skills get desktop --full
```

## Commands

```bash
agent-desktop-pp-cli info
agent-desktop-pp-cli install --dry-run
agent-desktop-pp-cli doctor --json
```

The `run` command passes arguments to the real `agent-desktop` executable and preserves its exit code.

## Real agent-desktop workflow

```bash
agent-desktop permissions
agent-desktop snapshot --app Finder -i --compact
agent-desktop click @e5 --snapshot <snapshot_id>
agent-desktop wait --element @e5 --snapshot <snapshot_id> --predicate actionable --timeout 5000
agent-desktop snapshot --app Finder -i --compact
```

The real CLI exposes 54 commands across observation, interaction, scroll, keyboard, mouse, app/window, notifications, clipboard, wait, system, and batch categories. Use `agent-desktop skills get desktop --full` for the complete command guide that ships with the installed binary.

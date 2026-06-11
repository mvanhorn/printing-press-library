# Agent Desktop Printing Press CLI

`agent-desktop-pp-cli` makes the Rust `agent-desktop` desktop automation CLI visible in the Printing Press library.

This package does not copy or reimplement the Rust CLI. It delegates to the real `agent-desktop` binary and installs that binary through the existing remote npm package, which downloads verified GitHub release assets for the selected package version.

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

## Commands

```bash
agent-desktop-pp-cli info
agent-desktop-pp-cli install --dry-run
agent-desktop-pp-cli doctor --json
```

The `run` command passes arguments to the real `agent-desktop` executable and preserves its exit code.

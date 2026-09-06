# HomeClaw Printed CLI Agent Guide

This directory contains the generated `homeclaw-pp-cli` and `homeclaw-pp-mcp` launchers. The package is a thin macOS adapter around the HomeClaw app; it never installs the app, creates symlinks, requests HomeKit permission, or grants access.

## Runtime contract

The CLI delegates arguments to the installed app:

```bash
homeclaw-pp-cli --help
homeclaw-pp-cli status
homeclaw-pp-cli list
```

The MCP entry point is a separate binary with the standard manifest shape:

```bash
homeclaw-pp-mcp
```

Both launchers accept explicit paths through `HOMECLAW_CLI_PATH`, `HOMECLAW_MCP_SERVER_PATH`, and `HOMECLAW_NODE_PATH`. Explicit paths are useful for tests and nonstandard app installations.

## Safety

The wrapper has no installation, privilege escalation, symlink, or HomeKit authorization logic. Review HomeClaw command help before mutating operations and use HomeClaw's own confirmation behavior.

## Generated-tree contract

This is a generated Printing Press package. Durable changes to generated code belong in `.printing-press-patches/`; do not hand-edit generated registry or skill mirrors. `manifest.json`, `tools-manifest.json`, and `.printing-press.json` are the package's publish metadata.

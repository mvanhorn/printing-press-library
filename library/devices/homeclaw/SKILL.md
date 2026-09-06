---
name: pp-homeclaw
description: "Printing Press wrapper for HomeClaw macOS HomeKit CLI and stdio MCP server"
author: "Keith Herrington"
license: "MIT"
argument-hint: "<homeclaw-cli args> | mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - homeclaw-pp-cli
    install:
      - kind: go
        bins: [homeclaw-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/homeclaw/cmd/homeclaw-pp-cli
---

# HomeClaw — Printing Press wrapper

Use `homeclaw-pp-cli` to delegate to the HomeClaw app's existing CLI. HomeClaw must already be installed and running; this wrapper never installs the app, creates symlinks, asks for credentials, or grants HomeKit access.

## Install and verify

```bash
npx -y @mvanhorn/printing-press-library install homeclaw --cli-only
homeclaw-pp-cli --help
```

## Usage

```bash
homeclaw-pp-cli <homeclaw-cli args>
homeclaw-pp-cli mcp
```

For a nonstandard installation, set `HOMECLAW_CLI_PATH`, `HOMECLAW_MCP_SERVER_PATH`, or `HOMECLAW_NODE_PATH`. The `mcp` subcommand launches the installed HomeClaw stdio MCP server through Node.js.

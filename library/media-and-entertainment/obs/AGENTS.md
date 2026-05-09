# OBS Printed CLI Agent Guide

This directory contains `obs-pp-cli`, an agent-optimized CLI for controlling OBS Studio via its built-in WebSocket server.

## Prerequisites

OBS Studio must be running with the WebSocket server enabled:
**OBS → Tools → WebSocket Server Settings → Enable WebSocket server**

## Runtime Discovery

```bash
obs-pp-cli --help
obs-pp-cli <command> --help
```

## Configuration

```bash
# Interactive
obs-pp-cli configure

# Non-interactive (agent use)
obs-pp-cli configure --non-interactive --host localhost --port 4455 --password ""
```

Config is stored in `~/.obs-pp/config.json` (chmod 600). No credentials in source.

## Local Customizations

If you modify this CLI, record each change:

1. Mark changed sites in source with `// PATCH: <summary>`
2. Catalog changes in `.printing-press-patches.json`

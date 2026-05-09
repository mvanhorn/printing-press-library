# Marginal Revolution Printed CLI Agent Guide

This directory contains the `marginalrevolution-pp-cli` printed CLI. It follows the published-library layout used by other Printing Press CLIs, with a read-only surface backed by Marginal Revolution's public RSS feed.

## Local Operating Contract

Start by asking the CLI for current runtime truth:

```bash
marginalrevolution-pp-cli doctor --json
```

Use runtime discovery before assuming a command shape:

```bash
marginalrevolution-pp-cli which "<capability>"
marginalrevolution-pp-cli <command> --help
```

Add `--agent` to command invocations for JSON output:

```bash
marginalrevolution-pp-cli latest --agent
```

The normal site search and WordPress JSON endpoints may present Cloudflare browser challenges to non-browser clients. Keep automated workflows on the RSS-backed commands unless a future patch adds a verified browser-capable transport.

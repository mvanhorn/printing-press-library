# Gorgias CLI Agent Guide

A Go CLI + MCP server for the Gorgias customer-support API. Was originally scaffolded with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), but we've taken ownership of the generated code — edit any file freely.

## Local Operating Contract

Start by asking the CLI for current runtime truth:

```bash
gorgias-pp-cli doctor --json
gorgias-pp-cli agent-context --pretty
```

Use runtime discovery instead of relying on a copied command list:

```bash
gorgias-pp-cli which "<capability>" --json
gorgias-pp-cli <command> --help
```

Add `--agent` to command invocations for JSON, compact list output, non-interactive defaults, no color, and confirmation-safe scripting:

```bash
gorgias-pp-cli <command> --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
gorgias-pp-cli <command> --help
gorgias-pp-cli <command> --dry-run --agent
```

Use `--yes --no-input` only after the target, arguments, and side effects are clear.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`.

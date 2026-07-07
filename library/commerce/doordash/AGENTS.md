# Doordash Printed CLI Agent Guide

This directory is a generated `doordash-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the active installed wrapper for current runtime truth:

```bash
/home/hermes/go/bin/doordash-pp-cli doctor --json
/home/hermes/go/bin/doordash-pp-mcp
```

The active Hermes install is currently a Node/CycleTLS wrapper, not the raw generated Go command surface. Do not assume generated helpers like `agent-context` or `which` exist; if a command is absent, fall back to `--help`, `doctor`, and the repo docs.

Prefer `--json` for agent-readable output. The active wrapper does not currently implement the generated `--agent` convenience flag on every command.

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
/home/hermes/go/bin/doordash-pp-cli <command> --help
/home/hermes/go/bin/doordash-pp-cli <command> --dry-run --json
```

Use `--yes --no-input` only after the target, arguments, and side effects are clear.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`) so a regen carries the intent forward instead of silently dropping it.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the source catalog's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.

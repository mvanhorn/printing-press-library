# Monarch Money Printed CLI Agent Guide

This directory is a `monarch-money-pp-cli` printed CLI package for Monarch Money. Treat broad generator or packaging problems as Printing Press issues; keep local edits narrow and preserve the read-oriented safety model.

## Local Operating Contract

Start by asking the CLI for current runtime truth:

```bash
monarch-money-pp-cli doctor
monarch-money-pp-cli --help
```

Use command help before adding new flags or examples:

```bash
monarch-money-pp-cli <command> --help
```

This CLI is intentionally read-oriented. Do not add Monarch Money mutations unless the command has explicit dry-run and confirmation behavior.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`) so a regen carries the intent forward instead of silently dropping it.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the source catalog's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.

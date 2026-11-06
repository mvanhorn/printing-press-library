# Tella Printed CLI Agent Guide

This directory is a generated `tella-pp-cli` printed CLI produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press). Repair broken published behavior here and record code customizations under `.printing-press-patches/`. Coordinate an upstream generator fix when the defect affects multiple CLIs.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
tella-pp-cli doctor --json
tella-pp-cli agent-context --pretty
```

Use runtime discovery instead of relying on a copied command list:

```bash
tella-pp-cli which captions --json
tella-pp-cli clips captions --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, no color, and confirmation-safe scripting:

```bash
tella-pp-cli account --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
tella-pp-cli videos auto --help
tella-pp-cli videos auto vid_example --silences natural --dry-run --agent
```

Use `--yes --no-input` only after the target, arguments, and side effects are clear.
Paid `agent-chats start` still requires explicit `--yes`; agent mode does not grant paid consent.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`) so a regen carries the intent forward instead of silently dropping it.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the source catalog's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.

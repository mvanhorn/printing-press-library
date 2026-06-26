# ProfilePress Printed CLI Agent Guide

This directory is a generated `profilepress-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
profilepress-pp-cli doctor --json
```

Use runtime discovery instead of relying on a copied command list:

```bash
profilepress-pp-cli <command> --help
```

Before running an unfamiliar command that may mutate LinkedIn state, inspect its help and prefer a dry run:

```bash
profilepress-pp-cli <command> --help
profilepress-pp-cli <command> --dry-run
```

ProfilePress is default-private:

- Do not collect LinkedIn passwords, tokens, cookies, or exported session files.
- Do not bypass LinkedIn authentication or access controls.
- Profile changes must pass privacy preflight and sensitive-change confirmation.
- Network notification is disabled by default. Use `--notify-network --confirm-notify NOTIFY-NETWORK` only when the user explicitly requests a public LinkedIn notification.
- Messages are draft-first. Sending requires `--confirm-send SEND-MESSAGE`.
- Live LinkedIn mutation and live message send adapters are disabled in this package pending an explicit user-controlled browser-session implementation.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`) so a regen carries the intent forward instead of silently dropping it.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the source catalog's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.

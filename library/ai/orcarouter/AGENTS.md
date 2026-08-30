# Orcarouter Printed CLI Agent Guide

This directory is a generated `orcarouter-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
orcarouter-pp-cli doctor --json
orcarouter-pp-cli auth
```

Use runtime discovery instead of relying on a copied command list:

```bash
orcarouter-pp-cli models --help
orcarouter-pp-cli chat --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, no color, and confirmation-safe scripting:

```bash
orcarouter-pp-cli <command> --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help:

```bash
orcarouter-pp-cli <command> --help
orcarouter-pp-cli auth set <key> --dry-run
```

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Release Ledger

`CHANGELOG.md` and `.printing-press-release.json` are the public library's per-CLI release ledger. Fresh prints may carry blank skeletons, but the final `YYYY.M.N` CLI release version is assigned only after a publish PR merges in `mvanhorn/printing-press-library`. Do not hand-bump those files or edit `var version = ...` for release bookkeeping; preserve existing ledger files on reprint and let the library workflow stamp the next release.

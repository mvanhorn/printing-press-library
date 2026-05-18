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

If you modify this CLI beyond what the generator produced, record each customization so it is visible to future maintainers.

1. Mark every changed Go source site with a short comment:

    ```
    // PATCH: <one-line summary>
    ```

2. Catalog the change in `.printing-press-patches.json` at this CLI root. Keep the entry short and include the touched files and validation outcome.

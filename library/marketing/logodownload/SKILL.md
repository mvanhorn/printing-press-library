---
name: pp-logodownload
description: "Printing Press CLI for logodownload.org. Search public logo pages, preview returned image_url values in the terminal, and optionally download selected images."
author: "Joao Bandeira"
license: "Apache-2.0"
argument-hint: "<search term> [--preview] [--download first|all|N]"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - logodownload-pp-cli
    install:
      - kind: go
        bins: [logodownload-pp-cli]
        module: github.com/joabbandeira/logodownload-pp-cli/cmd/logodownload-pp-cli
---

# LogoDownload — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `logodownload-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install logodownload --cli-only
   ```
2. Verify: `logodownload-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/logodownload/cmd/logodownload-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## What This CLI Does

Use `logodownload-pp-cli` when the user wants to find a public logo entry on logodownload.org from a company or brand search term.

The command returns JSON with:

- `title`: result title from logodownload.org.
- `url`: page URL for the logo result.
- `image_url`: preview image URL found in the result card, when available.
- `download_path`: local image path when `--download` is used.

## Search

```bash
logodownload-pp-cli search nike
```

The CLI prints JSON to stdout and diagnostics to stderr. Empty searches return `[]`.

## Preview in Terminal

Print a compact monochrome preview of returned `image_url` values directly in the terminal:

```bash
logodownload-pp-cli search nike --preview
```

The preview is printed to stderr and the JSON results remain on stdout.

Control the layout:

```bash
logodownload-pp-cli search nike --preview --preview-h 10 --preview-w 24 --preview-limit 3
```

Use this when the user needs to visually confirm that the returned `image_url` points to the expected logo before downloading or using it elsewhere.

## Download Selected Images

Download the first returned image:

```bash
logodownload-pp-cli search nike --download first
```

Download a specific 1-based result:

```bash
logodownload-pp-cli search nike --download 2 --output-dir ./logos
```

Download all returned images:

```bash
logodownload-pp-cli search nike --download all --output-dir ./logos
```

Only use `--download` when the user asked for local files. It writes to the local filesystem and includes `download_path` in the JSON output for each downloaded result.

## Safety Notes

- The normal search command is read-only.
- `--preview` fetches returned images and prints a monochrome approximation to stderr.
- `--download` writes local image files only after explicit user intent.
- This CLI does not log in, bypass access controls, or mutate logodownload.org.

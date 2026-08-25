---
name: gfonts
description: Search, browse, and download Google Fonts from the terminal via the gfonts CLI. No API key required. Use when the user asks about fonts, font recommendations, font pairing, or needs to download typefaces.
tags: [fonts, typography, design, cli, google-fonts]
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/gfonts/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# gfonts — Google Fonts CLI

Zero-auth CLI for searching, browsing, and downloading fonts from Google Fonts.

## Prerequisites: Install the CLI

This skill drives the `gfonts-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install gfonts --cli-only
   ```
2. Verify: `gfonts-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/gfonts/cmd/gfonts-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Commands

- `search <query>` — Search fonts by name, category, or designer
- `list` — Browse fonts with filters (--category, --sort, --limit)
- `info <font>` — Show detailed font metadata
- `download <font>` — Download font files (--variant, --output, --show)
- `trending` — Show trending/popular fonts
- `categories` — List all font categories with counts
- `random` — Pick a random font (--category)

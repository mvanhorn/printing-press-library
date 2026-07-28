# LogoDownload CLI

**A Printing Press style CLI for finding public logo assets from logodownload.org by company or brand name.**

`logodownload-pp-cli` turns a plain search term such as `nike`, `banco inter`, or `Bradesco Logo` into agent-friendly JSON with the logo page URL and the preview `image_url` exposed by logodownload.org. It also includes a terminal-native monochrome preview so a human can quickly recognize the logo shape before downloading anything.

Created by [@joabbandeira](https://github.com/joabbandeira) (Joab Bandeira).

## What It Does

This CLI is designed for agents and terminal workflows that need to locate a likely logo asset quickly:

- Search logodownload.org using the same public search route a browser uses.
- Return stable JSON to stdout for scripts and agents.
- Include `title`, logo page `url`, and card `image_url` when available.
- Print a horizontal monochrome preview directly in the terminal with `--preview`.
- Download selected `image_url` files only when explicitly requested with `--download`.
- Keep diagnostics and previews on stderr so stdout remains machine-readable.

The search is read-only by default. The only local write operation is `--download`.

## Install

### From This Repository

```bash
cd /Users/joabbandeira/dev/logodownload
go install ./cmd/logodownload-pp-cli
```

Verify:

```bash
logodownload-pp-cli --version
logodownload-pp-cli search nike
```

If the command is not found, make sure the Go binary directory is on your `PATH`:

```bash
export PATH="$HOME/go/bin:$PATH"
```

### Without Global Install

During development, run the CLI directly:

```bash
go run ./cmd/logodownload-pp-cli search nike
```

### Published Module

When this repository is published as a Go module:

```bash
go install github.com/joabbandeira/logodownload-pp-cli/cmd/logodownload-pp-cli@latest
```

### Printing Press Library Install

After the tool is added to the Printing Press Library catalog, the intended installer flow is:

```bash
npx -y @mvanhorn/printing-press-library install logodownload
```

For CLI only:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --cli-only
```

For focused agent skill only:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --skill-only
```

## Install by Tool

The integration currently ships as a CLI plus a focused agent skill. It does not yet ship an MCP server or `.mcpb` bundle, so MCP-only hosts need that extra package before they can install it as a native extension.

### OpenClaw

Once `logodownload` is available in the Printing Press Library catalog, install both the CLI binary and the OpenClaw skill:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --agent openclaw
```

Then restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

For local development before catalog publication:

```bash
cd /Users/joabbandeira/dev/logodownload
go install ./cmd/logodownload-pp-cli
logodownload-pp-cli --version
```

OpenClaw must be able to resolve `logodownload-pp-cli` from the process `PATH`, not only from an interactive shell.

### Codex

For Codex local development, install the binary from this repository:

```bash
cd /Users/joabbandeira/dev/logodownload
go install ./cmd/logodownload-pp-cli
```

Verify:

```bash
logodownload-pp-cli --version
logodownload-pp-cli search "Bradesco Logo" --preview
```

When the catalog entry is published, the intended focused skill install is:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --agent codex
```

Restart or reload the Codex session if the skill is not visible immediately after installation.

### Claude Code

After catalog publication, install for Claude Code with:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --agent claude-code
```

For CLI-only use in Claude Code before publication:

```bash
cd /Users/joabbandeira/dev/logodownload
go install ./cmd/logodownload-pp-cli
logodownload-pp-cli search nike
```

Make sure the shell used by Claude Code includes the Go binary directory, commonly `$HOME/go/bin`, in `PATH`.

### Cursor

After catalog publication, install the CLI and focused skill for Cursor:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --agent cursor
```

If the Cursor agent cannot see the command after installation, restart the Cursor agent session and confirm:

```bash
logodownload-pp-cli --version
```

### Gemini CLI

After catalog publication, install the CLI and focused skill for Gemini CLI:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --agent gemini-cli
```

If you only need the binary:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --cli-only
```

### GitHub Copilot

If the Printing Press installer and upstream `skills` CLI expose a GitHub Copilot agent target in your environment, use the same catalog install pattern:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --agent github-copilot
```

If that target is not available, install the CLI only and call it from any workflow that can execute shell commands:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --cli-only
logodownload-pp-cli search nike
```

### Hermes

Install the CLI binary first:

```bash
npx -y @mvanhorn/printing-press-library install logodownload --cli-only
```

Then install the focused Hermes skill after the catalog entry exists:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-logodownload --force
```

Inside a Hermes chat session, the equivalent is:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-logodownload --force
```

Restart the Hermes session or gateway if the skill is not visible immediately. Confirm the active Hermes profile can resolve:

```bash
logodownload-pp-cli --version
```

### Claude Desktop

Claude Desktop's one-click extension flow uses MCPB/MCP packaging. This repository does not currently include a `logodownload-pp-mcp` binary or `.mcpb` bundle, so there is no native Claude Desktop extension install yet.

Current options:

- Use `logodownload-pp-cli` from a terminal or from an agent environment that can run shell commands.
- Add an MCP server package later, for example `cmd/logodownload-pp-mcp`, then provide a Claude Desktop config.

Future manual Claude Desktop config would look like this only after an MCP binary exists:

```json
{
  "mcpServers": {
    "logodownload": {
      "command": "logodownload-pp-mcp"
    }
  }
}
```

Until then, install and verify the CLI directly:

```bash
cd /Users/joabbandeira/dev/logodownload
go install ./cmd/logodownload-pp-cli
logodownload-pp-cli search nike
```

### Plain Terminal

Use this when you only need the command line tool and no agent skill:

```bash
cd /Users/joabbandeira/dev/logodownload
go install ./cmd/logodownload-pp-cli
logodownload-pp-cli --version
```

### Docker

Use Docker when you want a self-contained CLI runtime:

```bash
docker build -t logodownload-pp-cli .
docker run --rm logodownload-pp-cli search nike
```

For downloads:

```bash
docker run --rm -v "$PWD/logos:/logos" logodownload-pp-cli search nike --download first --output-dir /logos
```

## Quick Start

Search for a logo:

```bash
logodownload-pp-cli search nike
```

Example output:

```json
[
  {
    "title": "Nike Logo",
    "url": "https://logodownload.org/nike-logo/",
    "image_url": "https://logodownload.org/wp-content/uploads/2014/04/nike-logo-4-1.png"
  }
]
```

Preview results in the terminal:

```bash
logodownload-pp-cli search "Bradesco Logo" --preview
```

Download the first returned preview image:

```bash
logodownload-pp-cli search "Banco Inter" --download first --output-dir ./logos
```

## Unique Features

These capabilities are specific to this CLI's intended agent workflow.

### Browser-equivalent public search

The CLI queries the same public search URL used in a browser:

```bash
logodownload-pp-cli search "mercado livre"
```

If the HTML layout does not produce parseable card results, it falls back to the public WordPress search API.

### Terminal logo preview

Use `--preview` to print a horizontal monochrome approximation of returned `image_url` values:

```bash
logodownload-pp-cli search "Bradesco Logo" --preview --preview-h 10 --preview-w 28 --preview-limit 5
```

The renderer uses a Braille grid, crops empty whitespace around the source image, and preserves the logo aspect ratio. It does not use colors, so it remains useful in plain terminals.

Preview output is written to stderr. JSON remains on stdout.

### Explicit image download

Download the first result:

```bash
logodownload-pp-cli search nike --download first
```

Download a specific 1-based result:

```bash
logodownload-pp-cli search nike --download 2 --output-dir ./logos
```

Download all returned `image_url` values:

```bash
logodownload-pp-cli search nike --download all --output-dir ./logos
```

Downloaded results include `download_path` in the JSON output.

## Usage

```bash
logodownload-pp-cli [flags] <search term>
```

Flags may appear before or after the search term:

```bash
logodownload-pp-cli --preview "Bradesco Logo"
logodownload-pp-cli search "Bradesco Logo" --preview
```

## Command Reference

### Search

```bash
logodownload-pp-cli <search term>
```

Returns a JSON array of logo candidates.

### Preview

```bash
logodownload-pp-cli <search term> --preview
```

Preview options:

- `--preview-h <n>`: terminal rows per logo preview. Default: `12`.
- `--preview-w <n>`: terminal columns per logo preview. Default: `28`.
- `--preview-limit <n>`: maximum results shown in the horizontal preview. Default: `5`.

### Download

```bash
logodownload-pp-cli <search term> --download first
logodownload-pp-cli <search term> --download all
logodownload-pp-cli <search term> --download 3
```

Download options:

- `--output-dir <path>`: directory where images are written. Default: current directory.

### General Flags

- `--limit <n>`: maximum WordPress API fallback results. Default: `10`.
- `--timeout <duration>`: HTTP timeout, such as `10s` or `30s`. Default: `20s`.
- `--version`: print CLI version.

## Output Format

The CLI prints JSON to stdout:

```json
[
  {
    "title": "Banco Inter Logo",
    "url": "https://logodownload.org/banco-inter-logo/",
    "image_url": "https://logodownload.org/wp-content/uploads/2018/11/banco-inter-logo-3-2.png",
    "download_path": "/absolute/path/logos/banco-inter-logo.png"
  }
]
```

Fields:

- `title`: title from the logodownload.org search result.
- `url`: public logo page URL.
- `image_url`: preview image URL extracted from the result card, when available.
- `download_path`: absolute local path added only when `--download` succeeds.

Empty searches return:

```json
[]
```

## Agent Usage

This CLI is intended to be safe and predictable for AI agents:

- **Non-interactive**: every input is provided through flags or positional arguments.
- **Pipeable**: JSON goes to stdout; logs, errors, and previews go to stderr.
- **Read-only by default**: searching and previewing do not write logo files.
- **Explicit local writes**: `--download` is required before any image is saved.
- **No authentication**: the integration only uses public logodownload.org pages.
- **No remote mutation**: it does not log in, publish, edit, comment, or change third-party state.

Recommended agent pattern:

```bash
logodownload-pp-cli search "company name"
```

If the user asks to visually confirm the result:

```bash
logodownload-pp-cli search "company name" --preview --preview-limit 5
```

If the user explicitly asks to download:

```bash
logodownload-pp-cli search "company name" --download first --output-dir ./logos
```

## Exit Behavior

- `0`: command completed successfully.
- `2`: usage error, such as missing search term or invalid `--download` index.
- Other non-zero exits: request, parsing, filesystem, or JSON formatting failure.

## Configuration

No configuration file or credentials are required.

The CLI uses:

- Base URL: `https://logodownload.org`
- Public search path: `/?s=<term>`
- Fallback API: `/wp-json/wp/v2/search`

## Docker

Build:

```bash
docker build -t logodownload-pp-cli .
```

Run:

```bash
docker run --rm logodownload-pp-cli search nike
```

For downloads, mount an output directory:

```bash
docker run --rm -v "$PWD/logos:/logos" logodownload-pp-cli search nike --download first --output-dir /logos
```

## Troubleshooting

### Command not found

Install the CLI and confirm the Go bin directory is visible:

```bash
go install ./cmd/logodownload-pp-cli
export PATH="$HOME/go/bin:$PATH"
logodownload-pp-cli --version
```

### Search returns `[]`

- Try the same term in a browser on logodownload.org.
- Use a shorter brand term, for example `bradesco` instead of a full legal name.
- Check network access from the current environment.
- Increase timeout for slow responses:

```bash
logodownload-pp-cli search "brand name" --timeout 30s
```

### Preview appears too compressed

Increase width or height:

```bash
logodownload-pp-cli search "Bradesco Logo" --preview --preview-w 40 --preview-h 14
```

For several results in one row, lower the limit:

```bash
logodownload-pp-cli search "Bradesco Logo" --preview --preview-limit 3
```

### Download fails

- Confirm the selected result has an `image_url`.
- Use `--download first` before selecting a specific index.
- Confirm the output directory exists or can be created.

```bash
logodownload-pp-cli search nike --download first --output-dir ./logos
```

## Safety Notes

This is a public-web scraper/helper. It should be used respectfully:

- Do not hammer logodownload.org with large automated loops.
- Cache downloaded files when possible.
- Treat returned logos as third-party assets and verify usage rights before publishing them.
- Prefer user confirmation before downloading many results.

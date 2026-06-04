# @mvanhorn/printing-press-library

Browse, search, install, update, and remove [Printing Press](https://printingpress.dev)-generated CLIs. Each install pulls down a Go binary plus its focused agent skill — the skill lands in every supported agent on your machine (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents the upstream [`skills`](https://github.com/vercel-labs/skills) CLI detects).

This package replaces the older `@mvanhorn/printing-press` npm package. Use the unambiguous `printing-press-library` command for catalog discovery and installs.

## Quick start

Search the catalog, browse a category, then install the CLI you want:

```bash
npx -y @mvanhorn/printing-press-library search flights
npx -y @mvanhorn/printing-press-library list --category travel
npx -y @mvanhorn/printing-press-library install airbnb
```

The `-y` flag only tells `npx` to run this wrapper package without an interactive npm prompt; `list` and `search` do not install catalog CLIs.

Or install the starter pack — four hand-picked CLIs and skills in one command:

```bash
npx -y @mvanhorn/printing-press-library install starter-pack
```

The starter pack installs `espn` (live sports), `flight-goat` (flight search), `movie-goat` (movie discovery), and `recipe-goat` (recipe ranking).

## Discover the catalog

List every published CLI:

```bash
npx -y @mvanhorn/printing-press-library list
```

Filter to one category:

```bash
npx -y @mvanhorn/printing-press-library list --category travel
```

Search by keyword across names, categories, APIs, descriptions, binaries, and generated search metadata:

```bash
npx -y @mvanhorn/printing-press-library search flights
npx -y @mvanhorn/printing-press-library search sports
```

Discovery commands print compact cards by default:

```text
airbnb (travel) - airbnb-pp-cli
  Search Airbnb and VRBO, find the host's direct booking site, and report the cheapest
  of three sources side-by-side.
  install: npx -y @mvanhorn/printing-press-library install airbnb
```

Use `--json` when another tool or agent is reading the output:

```bash
npx -y @mvanhorn/printing-press-library search flights --json
npx -y @mvanhorn/printing-press-library list --category travel --json
```

## Installing CLIs and skills

Every install pulls down the Go binary **and** the focused skill in one shot. Use `--cli-only` or `--skill-only` (see [Options](#options)) if you want just one half.

One tool:

```bash
npx -y @mvanhorn/printing-press-library install espn
npx -y @mvanhorn/printing-press-library install airbnb-pp-cli
```

Several at once (bundles and CLI names mix freely):

```bash
npx -y @mvanhorn/printing-press-library install espn sentry dub
npx -y @mvanhorn/printing-press-library install starter-pack cal-com
```

Under the hood: the installer reads the live catalog at [`registry.json`](https://github.com/mvanhorn/printing-press-library/blob/main/registry.json), resolves the CLI's Go module path, runs `go install`, and installs the matching focused skill from `cli-skills/pp-<name>` via `npx skills@latest`.

Names are forgiving: use the catalog slug (`airbnb`), generated binary name (`airbnb-pp-cli`), or API-ish name (`Airbnb Vrbo`) and the installer normalizes it to the right catalog entry.

## Other commands

```bash
npx -y @mvanhorn/printing-press-library list
npx -y @mvanhorn/printing-press-library search sports
npx -y @mvanhorn/printing-press-library list --category travel
npx -y @mvanhorn/printing-press-library list --installed
npx -y @mvanhorn/printing-press-library update espn
npx -y @mvanhorn/printing-press-library reinstall espn
npx -y @mvanhorn/printing-press-library uninstall espn --yes
```

`list` shows the public catalog by default. Use `list --installed` when you only want CLIs already present on your machine.

`reinstall` is an alias for `update`: `reinstall <name>` rebuilds one CLI from the latest catalog code and re-adds its skill, while `reinstall` with no name refreshes every Printing Press CLI already on your `PATH`. Reach for it when a binary or skill needs a clean refresh — `install <name>` overwrites in place too, so either works.

## Options

```bash
# Install only the Go binary, skip the focused skill
npx -y @mvanhorn/printing-press-library install espn --cli-only

# Install only the focused skill, skip the Go binary
# (binary will lazy-install on first agent invocation via the skill's instructions)
npx -y @mvanhorn/printing-press-library install espn --skill-only

# Constrain skill installation to a specific agent (repeatable)
npx -y @mvanhorn/printing-press-library install espn --agent claude-code

# Install the Go binary into a runtime-visible user bin directory
npx -y @mvanhorn/printing-press-library install espn --bin-dir ~/.local/bin

# OpenClaw: target OpenClaw skills and put the binary somewhere gateway PATHs commonly include
npx -y @mvanhorn/printing-press-library install espn --agent openclaw --bin-dir ~/.local/bin

# Machine-readable output
npx -y @mvanhorn/printing-press-library install espn --json
npx -y @mvanhorn/printing-press-library search sports --json
npx -y @mvanhorn/printing-press-library list --installed --json

# Pin to an alternate catalog (mainly for testing)
npx -y @mvanhorn/printing-press-library search sports --registry-url https://example.com/registry.json
```

`--cli-only` and `--skill-only` are mutually exclusive. They both work with bundles — `… install starter-pack --cli-only` installs four binaries with no skills, useful for CI machines that don't run Claude Code.

## Bundles

| Name | Members |
|---|---|
| `starter-pack` | `espn`, `flight-goat`, `movie-goat`, `recipe-goat` |

More bundles will be added over time. To suggest one, open an issue at the [printing-press-library repo](https://github.com/mvanhorn/printing-press-library/issues).

## Requirements

- Node.js 20+
- Go 1.26.3 or newer (for `go install`)
- The Go install directory on your `PATH` so installed CLIs are runnable by name — `$(go env GOPATH)/bin` (usually `$HOME/go/bin`) on macOS/Linux, or `%USERPROFILE%\go\bin` on Windows. Go does not add this to `PATH` for you. If it's missing, `install` still installs the focused skill, then prints the exact, copy-pasteable line to add for your platform and shell (zsh/bash/fish, PowerShell, cmd, or Git Bash).

Use `--bin-dir <dir>` when you want `go install` to write somewhere specific. The installer creates the directory first, sets `GOBIN=<dir>` for the install, and reports the resulting binary path:

```bash
npx -y @mvanhorn/printing-press-library install espn --bin-dir ~/.local/bin
```

Agent and gateway environments often run with a frozen or sanitized `PATH`. Updating `.zshrc`, `.bashrc`, or the Windows user environment may not affect an already-running agent process until you restart that session or gateway. For OpenClaw and similar gateway deployments, prefer installing directly into a user bin directory already exposed to the gateway:

```bash
npx -y @mvanhorn/printing-press-library install <slug> --agent openclaw --bin-dir ~/.local/bin
```

If you installed before `--bin-dir` or cannot reinstall yet, a symlink from the Go-installed binary into that directory is also a reasonable bridge:

```bash
ln -sf "$(go env GOPATH)/bin/<tool>" "$HOME/.local/bin/<tool>"
```

## Migration from @mvanhorn/printing-press

The old package name was ambiguous with the generator repo. If you installed it globally, remove the old package first:

```bash
npm uninstall -g @mvanhorn/printing-press
```

New installs should use:

```bash
npx -y @mvanhorn/printing-press-library <command>
```

The command name is also `printing-press-library`, so global installs are explicit:

```bash
npm install -g @mvanhorn/printing-press-library
printing-press-library search flights
```

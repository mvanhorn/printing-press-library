# @mvanhorn/printing-press

Installer and catalog CLI for [Printing Press](https://printingpress.dev)-generated CLIs. Each install pulls down a Go binary plus its focused Claude Code skill.

## Quick start

The fastest way in is the starter pack — four hand-picked CLIs and skills installed in one command:

```bash
npx -y @mvanhorn/printing-press install starter-pack
```

The starter pack installs `espn` (live sports), `flight-goat` (flight search), `movie-goat` (movie discovery), and `recipe-goat` (recipe ranking).

## Installing CLIs and skills

Every install pulls down the Go binary **and** the focused skill in one shot. Use `--cli-only` or `--skill-only` (see [Options](#options)) if you want just one half.

One tool:

```bash
npx -y @mvanhorn/printing-press install espn
```

Several at once (bundles and CLI names mix freely):

```bash
npx -y @mvanhorn/printing-press install espn linear dub
npx -y @mvanhorn/printing-press install starter-pack hubspot
```

Under the hood: the installer reads the live catalog at [`registry.json`](https://github.com/mvanhorn/printing-press-library/blob/main/registry.json), resolves the CLI's Go module path, runs `go install`, and installs the matching focused skill from `cli-skills/pp-<name>` via `npx skills@latest`.

## Other commands

```bash
npx -y @mvanhorn/printing-press search sports
npx -y @mvanhorn/printing-press list
npx -y @mvanhorn/printing-press update espn
npx -y @mvanhorn/printing-press uninstall espn --yes
```

## Options

```bash
# Install only the Go binary, skip the focused skill
npx -y @mvanhorn/printing-press install espn --cli-only

# Install only the focused skill, skip the Go binary
# (binary will lazy-install on first agent invocation via the skill's instructions)
npx -y @mvanhorn/printing-press install espn --skill-only

# Constrain skill installation to a specific agent (repeatable)
npx -y @mvanhorn/printing-press install espn --agent claude-code

# Machine-readable output
npx -y @mvanhorn/printing-press install espn --json

# Pin to an alternate catalog (mainly for testing)
npx -y @mvanhorn/printing-press search sports --registry-url https://example.com/registry.json
```

`--cli-only` and `--skill-only` are mutually exclusive. They both work with bundles — `… install starter-pack --cli-only` installs four binaries with no skills, useful for CI machines that don't run Claude Code.

## Bundles

| Name | Members |
|---|---|
| `starter-pack` | `espn`, `flight-goat`, `movie-goat`, `recipe-goat` |

More bundles will be added over time. To suggest one, open an issue at the [printing-press-library repo](https://github.com/mvanhorn/printing-press-library/issues).

## Requirements

- Node.js 20+
- Go 1.23+ (for `go install`)
- `$(go env GOPATH)/bin` on `$PATH` (usually `$HOME/go/bin`) so installed CLIs are runnable

While the catalog repository is private, also set `GITHUB_TOKEN` or `GH_TOKEN` for catalog and skill fetches, and ensure Go can read private `github.com/mvanhorn/*` modules.

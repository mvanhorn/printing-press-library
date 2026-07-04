---
name: pp-running-race-results
description: "Look up running race results across NYRR, Mika Timing, Athlinks, and RaceResult by fuzzy race name, bib, runner name, or athlete history."
author: "Jiahong Chen"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - running-race-results-pp-cli
    install:
      - kind: go
        bins: [running-race-results-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/running-race-results/cmd/running-race-results-pp-cli
---

# Running Race Results — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `running-race-results-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install running-race-results --cli-only
   ```
2. Verify: `running-race-results-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/running-race-results/cmd/running-race-results-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use this CLI when a user wants to look up a runner's published race result, search for a runner inside a known race, or inspect a runner's cross-event history from supported race-results providers.

## When Not to Use This CLI

Do not use this CLI for live race tracking, private training analytics, registration, purchases, or changing remote state. It is read-only and reports publicly published race results.

## Unique Capabilities

- **`lookup`** — Resolve a fuzzy race name to the right provider and edition, then return a unified result by bib or runner name.
- **`athlete`** — Show cross-event race history from Athlinks or NYRR by name, racer id, or the optional Athlinks current-user token.

## Command Reference

- `running-race-results-pp-cli lookup "<race name>" <bib>` — Look up one runner in a known race by bib.
- `running-race-results-pp-cli lookup "<race name>" --name "<runner name>"` — Search for runners by name inside the resolved race.
- `running-race-results-pp-cli athlete "<runner name>"` — Search Athlinks for a runner and show cross-event race history.
- `running-race-results-pp-cli athlete --provider nyrr "<runner name>"` — Search NYRR for a runner and show NYRR race history.

## Recipes

### Look Up a Bib

```bash
running-race-results-pp-cli lookup "mini 10k" 19 --year 2026
```

### Return JSON for Automation

```bash
running-race-results-pp-cli lookup "berlin marathon" 73664 --year 2025 --json
```

### Search by Runner Name

```bash
running-race-results-pp-cli lookup "berlin marathon" --name "Runner" --year 2025
```

### Show Athlete History

```bash
running-race-results-pp-cli athlete "Sample Athlete"
```

## Auth Setup

Most commands do not require credentials. `ATHLINKS_TOKEN` is optional and is only needed for `athlete --me` or if an Athlinks endpoint starts returning 401/403 for anonymous requests.

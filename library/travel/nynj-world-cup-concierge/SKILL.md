---
name: pp-nynj-world-cup-concierge
description: "Use the public NYNJ World Cup Concierge to extract official Explore NYNJ, Fan Experiences, and Watch Parties/Public Viewing candidates."
author: "Amit"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nynj-world-cup-concierge-pp-cli
---

# NYNJ World Cup Concierge — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nynj-world-cup-concierge-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install nynj-world-cup-concierge --cli-only
   ```
2. Verify: `nynj-world-cup-concierge-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/nynj-world-cup-concierge/cmd/nynj-world-cup-concierge-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When To Use This CLI

Use this CLI when a user asks for official NYNJ World Cup 26 fan experiences, Explore NYNJ entries, watch-party/public-viewing guidance, or trip-window filtering for NYNJ activities.

## When Not To Use This CLI

Do not use this CLI to book, buy, reserve, register, publish, submit, or mutate remote state. It is read-only and returns public discovery data only.

## Command Reference

**extract** — Fetch public NYNJ World Cup Concierge sources and emit normalized JSON candidates.

- `nynj-world-cup-concierge-pp-cli extract --agent` — Extract all public categories as compact JSON.
- `nynj-world-cup-concierge-pp-cli extract --agent --category "Fan Experiences" --category "Watch Parties" --date-window-start 2026-07-02 --date-window-end 2026-07-06 --exclude-undated` — Extract only activities whose public date ranges overlap July 2 through July 6, 2026.

**doctor** — Validate source reachability and extraction shape.

- `nynj-world-cup-concierge-pp-cli doctor --pretty` — Check source health and required categories.

## Agent Mode

Add `--agent` to `extract` for compact JSON. Use `--pretty` only when a human is inspecting the output.

Important filters:

- `--category <name>` is repeatable.
- `--date-window-start YYYY-MM-DD` and `--date-window-end YYYY-MM-DD` keep candidates whose parsed date range overlaps the window.
- `--exclude-undated` removes general guidance from a date-windowed activity feed.

## Argument Parsing

Parse `$ARGUMENTS`:

1. Empty, `help`, or `--help` means show `nynj-world-cup-concierge-pp-cli --help`.
2. Starts with `install` means install using the command in Prerequisites.
3. Otherwise execute the requested CLI command with `--agent` when machine-readable output is appropriate.

## Direct Use

Run:

```bash
nynj-world-cup-concierge-pp-cli extract --agent --category "Fan Experiences" --category "Watch Parties" --date-window-start 2026-07-02 --date-window-end 2026-07-06 --exclude-undated
```

The response contains `meta`, `categories`, `prompts`, and `candidates`. Parse `candidates[]` for activity records and use `source_url`/`url` for official source attribution.

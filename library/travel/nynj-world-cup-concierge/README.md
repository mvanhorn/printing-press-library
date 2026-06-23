# NYNJ World Cup Concierge CLI

Read-only Printing Press CLI for the public NYNJ World Cup Concierge and Host Committee fan-experience pages.

The CLI extracts normalized JSON candidates from public NYNJ World Cup 26 sources, including Explore NYNJ cards, Fan Experiences, and Watch Parties/Public Viewing guidance. It is designed for trip-planning agents that need official, source-linked activity candidates with stable IDs.

Created by [@amit](https://github.com/amit) (Amit).

## Install

The recommended path installs both the `nynj-world-cup-concierge-pp-cli` binary and the `pp-nynj-world-cup-concierge` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nynj-world-cup-concierge
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nynj-world-cup-concierge --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nynj-world-cup-concierge --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nynj-world-cup-concierge --agent claude-code
npx -y @mvanhorn/printing-press-library install nynj-world-cup-concierge --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/nynj-world-cup-concierge/cmd/nynj-world-cup-concierge-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nynj-world-cup-concierge-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install nynj-world-cup-concierge --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nynj-world-cup-concierge --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nynj-world-cup-concierge --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install nynj-world-cup-concierge --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Commands

```bash
nynj-world-cup-concierge-pp-cli extract --agent
nynj-world-cup-concierge-pp-cli doctor --pretty
```

Filter to a trip window:

```bash
nynj-world-cup-concierge-pp-cli extract \
  --agent \
  --category "Fan Experiences" \
  --category "Watch Parties" \
  --date-window-start 2026-07-02 \
  --date-window-end 2026-07-06 \
  --exclude-undated
```

## Sources

- https://nynjfwc26.com/destination/
- https://nynjfwc26.com/fan-events/
- https://nynj-ai.neurun.com/api/race/event/guid/ef742ab9-0cc1-45dc-a173-739ec1eeb541
- https://nynj-ai.neurun.com/api/prompts/by-event/ef742ab9-0cc1-45dc-a173-739ec1eeb541?lang=en

## Safety

This CLI is read-only. It does not authenticate, book, purchase, submit, or mutate remote state.

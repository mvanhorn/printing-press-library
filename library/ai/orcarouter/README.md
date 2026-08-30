# OrcaRouter CLI

**Agent-first OrcaRouter gateway access — model catalog, chat completions, auth health, and doctor diagnostics for the OpenAI-compatible OrcaRouter endpoint.**

[OrcaRouter](https://www.orcarouter.ai) is an OpenAI-compatible AI gateway built for both models and agents. Like OpenRouter, it exposes a provider/model namespace across many models — but it also combines adaptive routing, automatic failover, zero-markup inference, observability, guardrails, and agent-tool governance behind the same endpoint. This CLI lets the cron job and the AI agent calling out to `Bash` use that stack directly, without treating OrcaRouter as an anonymous custom base URL.

It also runs gateway-level, zero-trust security for AI agents on the same endpoint — screening every prompt/response and governing every tool call on a default-deny basis, with no application code changes.

## Install

The recommended path installs both the `orcarouter-pp-cli` binary and the `pp-orcarouter` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install orcarouter
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install orcarouter --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary:

```bash
npx -y @mvanhorn/printing-press-library install orcarouter --skill-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/orcarouter/cmd/orcarouter-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/orcarouter-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Auth

The CLI reads the `ORCAROUTER_API_KEY` environment variable. You can also persist a key to the local config file (`~/.config/orcarouter-pp-cli/config.toml`):

```bash
export ORCAROUTER_API_KEY=sk-orca-...
orcarouter-pp-cli auth
orcarouter-pp-cli auth set sk-orca-...
```

Point at a different gateway for testing:

```bash
export ORCAROUTER_BASE_URL=https://api.orcarouter.ai/v1
```

## Usage

Verify auth and connectivity first:

```bash
orcarouter-pp-cli doctor
orcarouter-pp-cli doctor --json
```

Discover the model namespace:

```bash
orcarouter-pp-cli models
orcarouter-pp-cli models get orcarouter/fusion
```

Send a chat completion through the gateway:

```bash
orcarouter-pp-cli chat orcarouter/fusion-flash "explain this in one line"
orcarouter-pp-cli chat orcarouter/free "hello" --max-tokens 20 --temperature 0.2
```

For agent-friendly output, add `--agent` to any command (JSON + compact + non-interactive):

```bash
orcarouter-pp-cli models --agent
orcarouter-pp-cli chat orcarouter/fusion "summarize" --max-tokens 50 --agent
```

## Commands

| Command | What it does |
|---------|--------------|
| `orcarouter-pp-cli doctor` | Check auth and connectivity to the OrcaRouter gateway |
| `orcarouter-pp-cli models` | List the OrcaRouter model catalog |
| `orcarouter-pp-cli models get <model-id>` | Show a single model by id |
| `orcarouter-pp-cli chat <model-id> <prompt>` | Send an OpenAI-compatible chat completion |
| `orcarouter-pp-cli auth` | Show the current auth state |
| `orcarouter-pp-cli auth set <api-key>` | Save the OrcaRouter API key to the local config file |
| `orcarouter-pp-cli version` | Print version |

Learn more at [OrcaRouter](https://www.orcarouter.ai).

Created by [@martinzudergaming-a11y](https://github.com/martinzudergaming-a11y).

Contributors: [@martinzudergaming-a11y](https://github.com/martinzudergaming-a11y) (martinzudergaming-a11y).

## Support

Discord: discord.gg/YEubt8enRA · X: https://x.com/OrcaRouter

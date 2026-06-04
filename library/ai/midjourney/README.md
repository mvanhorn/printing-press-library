# Midjourney CLI

Unofficial Printing Press CLI for Midjourney's web app API.

This CLI was generated from browser-observed Midjourney traffic and then patched with a small hand-written layer for creation and export workflows that are not represented well by read-only endpoint mirroring.

Important scope note: this is not an official Midjourney API client. It uses the same authenticated browser session surfaces that the web app uses, so endpoint shapes can change without notice.

## What Works

Read-only inspection commands:

- `midjourney-pp-cli explore list`
- `midjourney-pp-cli explore style-likes`
- `midjourney-pp-cli folders`
- `midjourney-pp-cli generations list`
- `midjourney-pp-cli generations updates`
- `midjourney-pp-cli moodboards`
- `midjourney-pp-cli profiles following`
- `midjourney-pp-cli profiles personalized`
- `midjourney-pp-cli queue`
- `midjourney-pp-cli rankings contests-count`
- `midjourney-pp-cli rankings model-ratings`
- `midjourney-pp-cli storage`

Creation/export commands added after traffic capture:

- `midjourney-pp-cli imagine "<prompt>"`
- `midjourney-pp-cli rerun <job-id>`
- `midjourney-pp-cli download <job-id> --index 0 --out image.png`

The mutating commands call Midjourney. Use `--dry-run` first when testing payloads.

Created by [@dave-agent-cerebro](https://github.com/dave-agent-cerebro) (Dave Fano).

## Install

The recommended path installs both the `midjourney-pp-cli` binary and the `pp-midjourney` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install midjourney
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install midjourney --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install midjourney --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install midjourney --agent claude-code
npx -y @mvanhorn/printing-press-library install midjourney --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/midjourney/cmd/midjourney-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/midjourney-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-midjourney --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-midjourney --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install midjourney --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Authentication

Midjourney does not expose a normal public API key for this workflow. The CLI uses an authenticated browser session.

There are two paths:

1. Browser-backed commands use Chrome DevTools Protocol against a browser that is already logged into `https://www.midjourney.com`.
2. Read-only HTTP commands can use `MIDJOURNEY_COOKIE_HEADER` when you intentionally provide a cookie header for the logged-in session.

For browser-backed creation/export, keep a Chrome instance with remote debugging enabled and logged into Midjourney, then pass its CDP URL if it is not the default:

```bash
midjourney-pp-cli imagine "simple red cube on a white desk" \
  --browser \
  --browser-cdp http://127.0.0.1:18800 \
  --dry-run
```

For cookie-backed read-only commands:

```bash
export MIDJOURNEY_COOKIE_HEADER="<cookie header from your own logged-in browser session>"
midjourney-pp-cli doctor
```

Do not commit, paste, or print real cookie headers. Treat `MIDJOURNEY_COOKIE_HEADER` as sensitive credential material.

## Quick Start

Verify the CLI:

```bash
midjourney-pp-cli doctor
midjourney-pp-cli agent-context --pretty
```

List recent generations:

```bash
midjourney-pp-cli generations list --json
```

Submit an image job:

```bash
midjourney-pp-cli imagine "small red cube on a white studio desk" \
  --ar 1:1 \
  --version 7 \
  --json \
  --yes
```

Preview the same request without sending it:

```bash
midjourney-pp-cli imagine "small red cube on a white studio desk" \
  --ar 1:1 \
  --version 7 \
  --dry-run \
  --json \
  --yes
```

Export all four images from a completed Midjourney job:

```bash
for i in 0 1 2 3; do
  midjourney-pp-cli download <job-id> \
    --index "$i" \
    --out "midjourney-$i.png" \
    --json \
    --yes
done
```

## Imagine Options

The `imagine` command builds the same kind of prompt string observed in the Midjourney Create UI, then submits it through `POST /api/submit-jobs`.

Common flags:

- `--ar 4:3` appends `--ar 4:3`
- `--version 7` appends `--v 7`
- `--niji 6` appends `--niji 6` and suppresses `--v`
- `--style raw` appends `--style raw`
- `--raw` is shorthand for `--style raw`
- `--style cute` appends `--style cute`
- `--quality 2` appends `--q 2`
- `--stylize 250` appends `--stylize 250`
- `--chaos 30` appends `--chaos 30`
- `--weird 25` appends `--weird 25`
- `--seed 123` appends `--seed 123`
- `--tile` appends `--tile`
- `--draft` appends `--draft`
- `--sref <url-or-code>` appends a style reference
- `--oref <url-or-code>` appends an omni reference
- `--image-prompt <url>` prepends an image prompt URL
- `--profile-id <profile>` appends `--profile <profile>`
- `--speed fast|relax|turbo` sets the request `f.mode`

Example with style and omni references:

```bash
midjourney-pp-cli imagine "pleasant illustrated person organizing notes on a desk" \
  --ar 4:3 \
  --version 7 \
  --oref <omni-reference-url> \
  --sref <style-reference-url> \
  --profile-id <profile-id> \
  --json \
  --yes
```

Example Niji request:

```bash
midjourney-pp-cli imagine "tiny friendly robot arranging sticky notes" \
  --ar 4:5 \
  --style cute \
  --niji 6 \
  --json \
  --yes
```

## Rerun

Rerun submits the observed Midjourney `reroll` payload:

```bash
midjourney-pp-cli rerun <job-id> --json --yes
```

Dry-run first when integrating:

```bash
midjourney-pp-cli rerun <job-id> --dry-run --json --yes
```

## Download

Direct server-side fetches from Midjourney CDN can fail with Cloudflare protection, and browser-side `fetch()` can be blocked by CORS. The `download` command avoids that by opening the rendered Midjourney job page through Chrome CDP, finding the rendered image, and saving a cropped PNG.

```bash
midjourney-pp-cli download <job-id> --index 0 --out image.png --json --yes
```

Indexes are `0`, `1`, `2`, and `3` for the standard four-image Midjourney batch.

## Agent Usage

This CLI is designed for AI agent consumption:

- Use `--agent` for JSON, compact output, no color, no prompts, and confirmation-safe defaults.
- Use `--dry-run` before mutating commands.
- Use `which "<capability>" --json` to discover commands at runtime.
- Use `agent-context --pretty` for the current command surface and profiles.
- Read-only endpoint mirror commands are safe inspection tools.
- `imagine` and `rerun` mutate remote Midjourney state and should be treated as external actions.
- `download` writes a local file and requires an output path.

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Verification

Development checks:

```bash
go test ./...
go vet ./...
go build -o ./midjourney-pp-cli ./cmd/midjourney-pp-cli
./midjourney-pp-cli --help
./midjourney-pp-cli doctor --json
```

For vulnerability scanning when `govulncheck` is installed:

```bash
govulncheck ./...
```

Printing Press verify mode is respected by mutating commands. When `PRINTING_PRESS_VERIFY=1` is set without `PRINTING_PRESS_VERIFY_LIVE_HTTP=1`, `imagine` and `rerun` return the standard synthetic verify noop instead of calling Midjourney.

## Discovery Notes

Captured behavior:

- The UI posts `POST /api/prompt-session-log` before `POST /api/submit-jobs`.
- `POST /api/submit-jobs` is the mutating endpoint for tested image-generation variants.
- Model and option selection is primarily encoded as prompt suffixes.
- `--v 7` produced `v7_diffusion`.
- `--v 7 --style raw` produced `v7_raw_diffusion`.
- `--v 6.1 --style raw` produced `v6-1_raw_diffusion`.
- `--v 6` produced `v6_diffusion`.
- `--niji 6` produced `v6_diffusion_anime`.
- `--v 7 --draft` produced `v7_draft_diffusion`.
- Draft and tile are incompatible; Midjourney returns `invalid_parameter`.
- `--turbo` and `--relax` prompt suffixes were accepted in UI capture, with response flags resolving to `turbo` and `relaxed`.

The capture summaries are intentionally not committed because browser captures can contain account-specific identifiers or session-derived data. Keep raw capture files local and commit only redacted conclusions.

## Contributing Back To Printing Press

This is a printed CLI, not a generator change. Per the Printing Press repository guidance:

- Use the supported public-library publishing flow for generated CLIs.
- Run the `/printing-press-publish` skill instead of manually copying files into `mvanhorn/printing-press-library`.
- Do not hand-edit the public-library registry, generated README catalog cells, or mirrored `cli-skills` output.
- Include provenance in the PR: source type `sniffed`, auth requirement `logged-in browser session / cookie header`, live smoke evidence, and explicit out-of-scope notes.
- Keep the community PR template intact.
- Use the AI/automation disclosure honestly; for this work, `Human-reviewed` is the right category once a human reviews the diff before submission.
- List verification commands actually run.
- For generator-level improvements discovered here, open an issue in `mvanhorn/cli-printing-press` instead of hiding them in this printed CLI.

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).

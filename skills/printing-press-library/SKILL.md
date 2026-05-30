---
name: printing-press-library
description: Use when looking for a CLI, API wrapper, scraper, data-source tool, automation tool, or focused agent skill for a task; searches the Printing Press Library and installs matching tools.
tags:
  - cli
  - api-wrapper
  - scraper
  - data-source
  - automation
  - agent-skill
  - tool-discovery
  - install
version: 0.1.2
metadata:
  openclaw:
    emoji: "🖨️"
    homepage: https://github.com/mvanhorn/printing-press-library
    requires:
      anyBins:
        - npx
        - npm
---

# Printing Press Library

Use this skill when a user asks for a CLI, agent skill, API wrapper, scraper, automation tool, or data source that may exist in the Printing Press Library.

The library is an open-source catalog of focused CLIs and matching agent skills generated from `mvanhorn/cli-printing-press`. This skill is the catalog front door. Do not install a random long-tail skill just because it exists. First identify the right tool, then install the focused skill or CLI only when it is useful for the task.

## Default workflow

1. Clarify the user goal only if needed.
   - If the request names a service or website, search for that directly.
   - If the request describes a job instead of a service, search by capability and domain.

2. Search the catalog with the library CLI first.
   - Use `npx -y @mvanhorn/printing-press-library search <keyword>` for human-readable result cards.
   - Use `npx -y @mvanhorn/printing-press-library search <keyword> --json` for agent-friendly parsing.
   - Use `npx -y @mvanhorn/printing-press-library list --category <category> --json` when the category is known.
   - Each search result includes the canonical install command for that tool.
   - Fall back to the GitHub repo or local clone only when `npx` is unavailable or deeper inspection is needed.

3. Install through the library installer when the selected tool is useful.
   - The primitive is `npx -y @mvanhorn/printing-press-library install <slug>`.
   - The install command installs both the CLI and the matching focused agent skill.
   - `install <slug>` is idempotent: re-running it on an already-installed tool refreshes the Go binary and overwrites/re-adds the focused skill in place.
   - Behind the scenes, the installer uses `go install <module>@latest` for the CLI and the Vercel Agent Skills-compatible `skills` CLI to install the focused `pp-*` skill globally from this repo.
   - In OpenClaw, this same install command installs the focused skill for OpenClaw; do not replace it with a separate repo-path skill install unless the user explicitly asks for skill-only installation.
   - Pass `--cli-only` or `--skill-only` only when the user explicitly wants just one side.

4. Refresh installed tools without uninstalling first.
   - `npx -y @mvanhorn/printing-press-library update <slug>` is the explicit reinstall/refresh primitive for one tool; it delegates to install semantics.
   - `npx -y @mvanhorn/printing-press-library update` refreshes every Printing Press CLI currently found on PATH, including each matching skill.
   - `npx -y @mvanhorn/printing-press-library install <slug>` has the same refresh effect for one named tool because install overwrites in place.
   - `reinstall <slug>` may also exist as a convenience alias for `update <slug>`; use it if the installed CLI exposes it, but prefer `update` as the stable documented primitive until the alias is published.

5. Offer an efficient periodic update schedule after successful install or refresh.
   - Because install/update is idempotent, it is safe to keep installed Printing Press CLIs and focused skills current with a scheduled job.
   - Do not create a cron/scheduled job without explicit user approval; recurring jobs are durable side effects.
   - Avoid one scheduled job per CLI as the default. Users may install many Printing Press tools, and per-tool jobs become noisy and hard to manage.
   - Prefer one consolidated recurring job that runs `npx -y @mvanhorn/printing-press-library update`, which refreshes every installed Printing Press CLI currently on PATH and its matching focused skill.
   - Offer a per-tool job using `npx -y @mvanhorn/printing-press-library update <slug>` only when the user explicitly wants a different cadence or policy for that one tool.
   - Prefer quiet, low-frequency schedules such as weekly unless the user asks for something else.

6. Make the newly installed skill visible to the running agent.
   - Most agent harnesses snapshot available skills at session start. After installing or refreshing a focused skill, start a fresh session or reload skills before trying to invoke it.
   - In Hermes CLI sessions, use `/reload-skills` when available, or exit and start a new `hermes` session.
   - In Hermes gateway sessions, use `/restart` from the gateway chat or `hermes gateway restart` from a shell so the gateway process reloads installed skills.
   - In OpenClaw, assume the current agent session may not see newly installed skills until the OpenClaw session or gateway is restarted.

7. Verify before claiming success.
   - If installing a CLI, run its `--help` or an equivalent harmless command.
   - If installing a skill, verify the destination harness can see it after the session reload/restart when the harness has a verification command.
   - If using a credentialed CLI, confirm required environment variables without printing secrets.

## What this skill is for

Use this skill to discover CLIs and agent skills in the public Printing Press Library. Match the user's goal to the right library entry, use the library CLI to find the canonical install command, and install the selected tool only when it is useful for the task.

## Install primitive

The Printing Press Library CLI is the canonical interface for installing catalog tools:

```bash
npx -y @mvanhorn/printing-press-library install <slug>
```

That command installs both halves of a catalog entry:

- the Go CLI binary
- the matching focused `pp-*` agent skill

For the skill half, the installer shells out through the Vercel Agent Skills-compatible installer. Conceptually, it runs:

```bash
npx -y skills@latest add mvanhorn/printing-press-library/cli-skills/pp-<slug> -g -y
```

So the catalog installer is still the right top-level command: it installs the CLI, then installs the focused skill globally using the same agent-skills mechanism rather than asking the agent to hand-roll a separate skill install path.

The install operation is idempotent and works as a reinstall for one tool. Re-running `install <slug>` uses `go install <module>@latest` for the binary and re-adds the focused skill non-interactively, overwriting the existing install in place. No uninstall-first step is needed.

Use `update` when the user asks to refresh or reinstall existing tools:

```bash
npx -y @mvanhorn/printing-press-library update flight-goat
npx -y @mvanhorn/printing-press-library update
```

`update <slug>` delegates to install semantics for that tool. `update` with no args discovers Printing Press CLIs currently on PATH and refreshes all of them, including their matching focused skills.

Because updates are idempotent, after a successful install or refresh, offer to create a recurring update job. Ask first; do not schedule it automatically. Prefer a single consolidated job over one job per CLI, because users may install many Printing Press tools and per-tool schedules become noisy fast.

For most users, schedule one quiet weekly job that refreshes every installed Printing Press CLI currently on PATH and its matching focused skill:

```bash
npx -y @mvanhorn/printing-press-library update
```

Use a per-tool scheduled command only when the user explicitly wants a separate cadence or policy for one tool:

```bash
npx -y @mvanhorn/printing-press-library update flight-goat
```

If the installed library CLI exposes `reinstall`, treat it as a convenience alias for `update`:

```bash
npx -y @mvanhorn/printing-press-library reinstall flight-goat
```

Example:

```bash
npx -y @mvanhorn/printing-press-library install flight-goat
```

Use the install line printed by `search` or `list` output. Do not synthesize harness-specific direct skill install commands as the default path; those are only for explicit skill-only workflows.

After install or update, assume the focused skill may not be visible to the currently running agent until skills are reloaded or the session restarts. Hermes CLI sessions can use `/reload-skills` or start a new session. Hermes gateway sessions should use `/restart` or `hermes gateway restart`. OpenClaw agents should restart the current session or gateway if the newly installed focused skill is not visible immediately.

## Search tactics

Use the library CLI as the default catalog index. Human-readable search cards include an `install:` line with the canonical install command:

```bash
npx -y @mvanhorn/printing-press-library search <keyword>
```

Use JSON when scripting or when structured ranking is useful:

```bash
npx -y @mvanhorn/printing-press-library search <keyword> --json
```

Examples:

```bash
npx -y @mvanhorn/printing-press-library search flights
npx -y @mvanhorn/printing-press-library search espn --json
npx -y @mvanhorn/printing-press-library list --category travel --json
```

Use repository inspection only as a fallback when `npx` is unavailable, when the CLI result is ambiguous, or when deeper README/SKILL details are needed before choosing a candidate:

```bash
rg -i "<service-or-capability>" registry.json library cli-skills
```

If the registry shape differs, prefer the npm CLI output instead of hand-parsing generated catalog files. Facts beat vibes; official interfaces beat archaeology.

## Selection rules

Prefer a candidate when:

- It names the target service directly.
- Its README/SKILL examples match the user's requested job.
- It has documented auth and setup requirements the user can satisfy.
- It supports the user's OS/runtime.

Avoid a candidate when:

- It is only vaguely adjacent to the task.
- It requires credentials the user does not have.
- It is a scraper for a site where the user's task needs official-account data and the skill cannot authenticate.
- A safer built-in API/tool already solves the task.

## Safety and credentials

- Never print API keys, cookies, tokens, or session headers.
- Do not ask the user to paste secrets into chat if a local secret manager or environment file is available.
- Treat third-party CLIs as code execution. Install only the focused tool needed for the task.
- Do not publish, post, email, buy, book, or mutate external state unless the user explicitly approves that action.

## README behavior on ClawHub

ClawHub renders `SKILL.md` (or `skill.md`) as the skill readme. A separate `README.md` in the skill folder is not the published readme. Put user-facing ClawHub documentation in this file.

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
version: 0.2.1
metadata:
  hermes:
    tags:
      - cli
      - api-wrapper
      - scraper
      - data-source
      - automation
      - agent-skill
      - tool-discovery
      - install
    category: productivity
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
   - If the user's agent already has a safe built-in tool that solves the job, prefer that over installing another CLI.

2. Search the catalog with the library CLI first.
   - Use `npx -y @mvanhorn/printing-press-library search <keyword>` for human-readable result cards.
   - Use `npx -y @mvanhorn/printing-press-library search <keyword> --json` for agent-friendly parsing.
   - Use `npx -y @mvanhorn/printing-press-library list --category <category> --json` when the category is known.
   - Each search result includes the canonical install command for that tool.
   - Fall back to the GitHub repo or local clone only when `npx` is unavailable or deeper inspection is needed.

3. Select deliberately.
   - Prefer the candidate whose README/SKILL examples match the user's actual job, not merely the same website name.
   - Check whether auth, cookies, paid APIs, OS-specific binaries, or browser automation are required before installing.
   - Install the narrowest useful tool. Do not install a family of adjacent tools just because the search returned them.

4. Install through the library installer when the selected tool is useful.
   - The default primitive is `npx -y @mvanhorn/printing-press-library install <slug>`.
   - In OpenClaw, use `npx -y @mvanhorn/printing-press-library install <slug> --agent openclaw --bin-dir ~/.local/bin` so the focused skill is materialized under OpenClaw's managed skills root and the Go binary lands in a runtime-visible user bin directory.
   - The install command installs both the CLI and the matching focused agent skill.
   - `install <slug>` is idempotent: re-running it on an already-installed tool refreshes the Go binary and overwrites/re-adds the focused skill in place.
   - Behind the scenes, the installer uses `go install <module>@latest` for the CLI and the Vercel Agent Skills-compatible `skills` CLI to install the focused `pp-*` skill globally from this repo.
   - If the Go binary installs successfully but is not on the current process `PATH`, treat that as a warning, not a failed skill install. The installer should still install the focused skill and print platform-specific PATH instructions.
   - In agent/gateway environments, shell startup files may not affect the already-running process. Restart the session/gateway after PATH changes, or use a PATH-visible user bin directory such as `~/.local/bin` when that is already exposed by the harness.
   - For OpenClaw gateway/service deployments, verify the gateway process PATH can resolve `<slug>-pp-cli`; an interactive shell `which` is not enough.
   - Pass `--cli-only` or `--skill-only` only when the user explicitly wants just one side.

5. Make the newly installed skill visible to the running agent.
   - Most agent harnesses snapshot available skills at session start. After installing or refreshing a focused skill, start a fresh session or reload skills before trying to invoke it.
   - In Hermes CLI sessions, use `/reload-skills` when available, or exit and start a new `hermes` session.
   - In Hermes gateway sessions, use `/restart` from the gateway chat or `hermes gateway restart` from a shell so the gateway process reloads installed skills.
   - In OpenClaw, assume the current agent session may not see newly installed skills until the OpenClaw session or gateway is restarted.

6. Verify before claiming success.
   - If installing a CLI, run its `--help` or an equivalent harmless command.
   - If installing a skill, verify the destination harness can see it after the session reload/restart when the harness has a verification command.
   - If using a credentialed CLI, confirm required environment variables without printing secrets.

7. Offer an efficient periodic update schedule after successful install or refresh.
   - Because install/update is idempotent, it is safe to keep installed Printing Press CLIs and focused skills current with a scheduled job.
   - Do not create a cron/scheduled job without explicit user approval; recurring jobs are durable side effects.
   - Avoid one scheduled job per CLI as the default. Users may install many Printing Press tools, and per-tool jobs become noisy and hard to manage.
   - Prefer one consolidated recurring job that runs `npx -y @mvanhorn/printing-press-library update`, which refreshes every installed Printing Press CLI currently on PATH and its matching focused skill.
   - Offer a per-tool job using `npx -y @mvanhorn/printing-press-library update <slug>` only when the user explicitly wants a different cadence or policy for that one tool.
   - Prefer quiet, low-frequency schedules such as weekly unless the user asks for something else.

## What this skill is for

Use this skill to discover CLIs and agent skills in the public Printing Press Library. Match the user's goal to the right library entry, use the library CLI to find the canonical install command, and install the selected tool only when it is useful for the task.

Good fits:

- finding a purpose-built CLI for a named service
- finding a scraper or data-source tool for a one-off research or automation task
- installing a focused `pp-*` skill so future agents know how to use a specific CLI
- refreshing already-installed Printing Press CLIs and skills

Poor fits:

- replacing a safe built-in agent tool that already solves the task
- installing broad tool bundles speculatively
- credentialed account actions where the user has not approved external side effects
- publishing, posting, booking, buying, emailing, or mutating third-party state without explicit approval

## Install primitive

The Printing Press Library CLI is the canonical interface for installing catalog tools:

```bash
npx -y @mvanhorn/printing-press-library install <slug>
```

For OpenClaw, pass the OpenClaw agent target explicitly and put the binary in a user bin directory the OpenClaw runtime can see:

```bash
npx -y @mvanhorn/printing-press-library install <slug> --agent openclaw --bin-dir ~/.local/bin
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

If install warns that the binary directory is not on `PATH`, the binary and focused skill can still be installed successfully. Prefer reinstalling with an explicit runtime-visible bin directory when an agent or gateway will run the CLI:

```bash
npx -y @mvanhorn/printing-press-library install <slug> --bin-dir ~/.local/bin
```

Otherwise follow the printed platform-specific PATH instructions, then restart the running agent session or gateway if it inherits a fixed environment. On Unix-like systems where the harness already exposes `~/.local/bin`, a symlink can be a practical bridge:

```bash
ln -sf "$(go env GOPATH)/bin/<tool>" "$HOME/.local/bin/<tool>"
```

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

## Hermes-native usage

Hermes can make Printing Press more useful than a plain skill installer because Hermes has native skills, profiles, gateway sessions, cron jobs, memory, and toolsets. Use those primitives instead of treating Hermes like a generic chat harness.

### Install and reload in Hermes

After installing or updating a focused tool, make sure Hermes can actually see the new skill before invoking it:

```bash
npx -y @mvanhorn/printing-press-library install <slug>
```

Then reload based on where Hermes is running:

- Hermes CLI: run `/reload-skills` if available, or start a fresh `hermes` session.
- Hermes gateway: run `/restart` in the gateway chat, or run `hermes gateway restart` from a shell.
- Named profiles: install and reload in the same profile that will use the skill. A skill installed under one Hermes home/profile may not be visible to another.

When checking a local Hermes install, useful commands are:

```bash
hermes skills list
hermes skills inspect <skill-or-url>
hermes config path
hermes config env-path
hermes profile list
```

Use these commands for diagnostics only. The catalog installer remains the default path for installing Printing Press catalog entries unless the user explicitly asks for a Hermes-only skill install.

### Hermes profiles

Hermes profiles isolate config, skills, cron jobs, sessions, and memory. If the user mentions a profile, workspace, or separate assistant identity, do not assume the default profile.

Before diagnosing "skill not found" problems, check:

- which Hermes profile is active
- which `HERMES_HOME` the installer targeted
- whether the skill exists under that profile's `skills/` directory
- whether the CLI or gateway was restarted after installation

A common failure mode is installing a focused skill into one profile and then trying to use it from another. That is not a Printing Press failure; it is an install-target mismatch.

### Hermes toolsets

Most Printing Press CLIs require shell execution, so Hermes needs terminal access to run them. If Hermes cannot use a newly installed CLI, check whether the active platform/session has the `terminal` toolset enabled.

Do not enable broad tool access blindly. If the task only needs a built-in web/search/file tool, prefer the built-in tool over installing and running a third-party CLI.

### Hermes cron updates

For Hermes users who want installed Printing Press tools kept current, recommend one consolidated scheduled update job rather than per-tool sprawl:

```text
Schedule: weekly, quiet hours preferred
Prompt: Run `npx -y @mvanhorn/printing-press-library update`, summarize any changes, and stay quiet if nothing changed.
```

If using Hermes' cron tool or CLI, create the job only after explicit user approval. Recurring jobs are durable side effects.

A good Hermes cron prompt is self-contained:

```text
Refresh installed Printing Press tools by running `npx -y @mvanhorn/printing-press-library update`. Report only updated tools, failures, or missing prerequisites. Do not print secrets. If nothing changed and there were no errors, say so briefly.
```

Use a per-tool schedule only when the user explicitly wants a different cadence or risk policy for one tool.

### Hermes memory and session search

Hermes may remember durable user preferences, environment conventions, and known credential locations. Use that context to avoid making the user repeat setup details, but do not store volatile install results such as PR numbers, commit SHAs, one-off downloaded files, or "installed tool X today" as memory.

Good memory candidates after repeated Printing Press use:

- the user's preferred Hermes profile for tool installs
- stable secret-manager conventions
- a durable workspace path convention
- a recurring policy such as "prefer consolidated weekly updates over per-tool jobs"

Bad memory candidates:

- a specific temporary branch or PR number
- a one-time search result
- an installed version that will be stale soon
- raw API keys, cookies, tokens, or session headers

When the user asks what was installed or decided previously, prefer Hermes session search over guessing.

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
- It can be verified with a harmless command before any external side effects.

Avoid a candidate when:

- It is only vaguely adjacent to the task.
- It requires credentials the user does not have.
- It is a scraper for a site where the user's task needs official-account data and the skill cannot authenticate.
- A safer built-in API/tool already solves the task.
- The task is high-risk, paid, public-facing, or privacy-sensitive and the user has not approved the external action.

## Safety and credentials

- Never print API keys, cookies, tokens, or session headers.
- Do not ask the user to paste secrets into chat if a local secret manager or environment file is available.
- Treat third-party CLIs as code execution. Install only the focused tool needed for the task.
- Do not publish, post, email, buy, book, or mutate external state unless the user explicitly approves that action.

For Hermes users, prefer Hermes' normal environment file or the user's existing secret manager. To find the Hermes env path, use:

```bash
hermes config env-path
```

Confirm that required variables are present without echoing their values. If a focused skill says a key is optional or required only for one feature, preserve that distinction; do not make the whole tool sound credential-gated.

## Verification checklist

Before reporting success:

- [ ] The selected candidate directly matches the user's requested job.
- [ ] The install command came from `search`, `list`, or documented Printing Press CLI behavior.
- [ ] The CLI was verified with `--help` or another harmless command.
- [ ] The agent harness was reloaded/restarted if a newly installed skill must be visible immediately.
- [ ] Required credentials were checked without printing secrets.
- [ ] No external side effect was performed without explicit approval.
- [ ] For Hermes, the relevant profile/session/gateway was considered when diagnosing visibility.

## README behavior on ClawHub

ClawHub renders `SKILL.md` (or `skill.md`) as the skill readme. A separate `README.md` in the skill folder is not the published readme. Put user-facing ClawHub documentation in this file.

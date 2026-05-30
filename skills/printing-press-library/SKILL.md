---
name: printing-press-library
description: Discover and install Printing Press Library CLIs and focused agent skills.
version: 0.1.0
metadata:
  openclaw:
    emoji: "🖨️"
    homepage: https://github.com/mvanhorn/printing-press-library
    requires:
      anyBins:
        - npx
        - npm
        - hermes
        - clawhub
---

# Printing Press Library

Use this skill when a user asks for a CLI, agent skill, API wrapper, scraper, automation tool, or data source that may exist in the Printing Press Library.

The library is an open-source catalog of focused CLIs and matching agent skills generated from `mvanhorn/cli-printing-press`. This skill is the catalog front door. Do not install a random long-tail skill just because it exists. First identify the right tool, then install the focused skill or CLI only when it is useful for the task.

## Default workflow

1. Clarify the user goal only if needed.
   - If the request names a service or website, search for that directly.
   - If the request describes a job instead of a service, search by capability and domain.

2. Search the catalog.
   - Use the GitHub repo, local clone, or npm package docs if available.
   - Relevant local paths when the repo is cloned:
     - `registry.json`
     - `library/<category>/<slug>/README.md`
     - `library/<category>/<slug>/SKILL.md`
     - `cli-skills/pp-<slug>/SKILL.md`

3. Prefer a focused `pp-*` skill before installing or running a CLI.
   - Focused skills include usage guidance, auth notes, examples, and install instructions.
   - The CLI binary should be installed only when the task actually needs execution.

4. Verify before claiming success.
   - If installing a skill, verify the destination harness can see it.
   - If installing a CLI, run its `--help` or an equivalent harmless command.
   - If using a credentialed CLI, confirm required environment variables without printing secrets.

## Install paths

### OpenClaw / ClawHub

OpenClaw users should install this discovery skill from ClawHub:

```bash
clawhub install printing-press-library
```

To install a focused skill from ClawHub if it is published there:

```bash
clawhub install <skill-slug>
```

If a focused skill is not on ClawHub, use the Vercel Agent Skills-compatible GitHub path below.

### Vercel Agent Skills-compatible harnesses

Install this discovery skill globally:

```bash
npx skills add mvanhorn/printing-press-library/skills/printing-press-library -g -y
```

Or select it explicitly from the repo root:

```bash
npx skills add mvanhorn/printing-press-library -g -y --skill printing-press-library
```

Install a focused per-CLI skill globally:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-<slug> -g -y
```

Example:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-espn -g -y
```

### Hermes

Install this discovery skill:

```bash
hermes skills install mvanhorn/printing-press-library/skills/printing-press-library --force
```

Install a focused per-CLI skill:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-<slug> --force
```

### CLI-first install

For humans, scripts, and CLI-first usage, use the npm installer:

```bash
npx -y @mvanhorn/printing-press-library install <slug>
```

That path installs the CLI and the matching skill using the library installer.

## Search tactics

Use whichever source is available:

```bash
# From a local clone
python3 - <<'PY'
import json
from pathlib import Path
q = 'espn'
registry = json.loads(Path('registry.json').read_text())
for item in registry:
    haystack = json.dumps(item).lower()
    if q.lower() in haystack:
        print(item.get('slug') or item.get('name'), item.get('title') or item.get('description', ''))
PY
```

```bash
# Broad repo text search when local tools are available
rg -i "<service-or-capability>" registry.json library cli-skills
```

If the registry shape differs, inspect `registry.json` first instead of guessing. Generated catalogs move; facts beat vibes.

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

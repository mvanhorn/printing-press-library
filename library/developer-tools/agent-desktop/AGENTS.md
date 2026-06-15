# AGENTS.md

This package is a Printing Press bridge for `agent-desktop`.

- Do not reimplement desktop automation here.
- The real CLI is the Rust `agent-desktop` package published from `https://github.com/lahfir/agent-desktop`.
- Keep this wrapper limited to install, doctor, info, and pass-through delegation.
- Keep `SKILL.md` command examples aligned with Cobra commands in `internal/cli`.

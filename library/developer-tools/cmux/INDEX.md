---
name: cmux-pp-cli
type: printed-cli
status: active
description: Every cmux feature, plus persisted state, cross-surface search, and a notify-driven event stream no other cmux tool offers.
binary_name: cmux-pp-cli
binary_install_path: ./cmd/cmux-pp-cli

trigger_phrases:
  - "search across cmux panes"
  - "which cmux workspace is awaiting input"
  - "show stuck cmux panes"
  - "watch cmux notifications"
  - "use cmux"
  - "run cmux-pp-cli"
related_skills: []
printed_from: 2026-05-16 via Printing Press v4.4.0 (run 20260516-122549)
printed_by: dstevens
---

# Cmux

Every cmux feature, plus persisted state, cross-surface search, and a notify-driven event stream no other cmux tool offers.

## When to use

(fill in 2-4 bullets describing situations where this skill is the right tool)

## Install

```bash
bash ~/Documents/Dev/cc-skills/scripts/install.sh cmux-pp-cli
```

The installer handles symlink + `go install` + (if applicable) OS-keychain credential storage + `doctor`. See [INDEX-template.md](../INDEX-template.md) for the manual install path.

## Invoke

```bash
cmux-pp-cli --help
cmux-pp-cli doctor
# (add 2-3 realistic invocations)
```

## Trigger phrases

- `search across cmux panes`
- `which cmux workspace is awaiting input`
- `show stuck cmux panes`
- `watch cmux notifications`
- `use cmux`
- `run cmux-pp-cli`

## Related

(fill in)

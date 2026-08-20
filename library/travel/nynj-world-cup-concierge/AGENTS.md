# NYNJ World Cup Concierge CLI Agent Guide

This directory is a `nynj-world-cup-concierge-pp-cli` printed CLI for public NYNJ World Cup Concierge discovery data. Keep changes narrow and record any hand customizations under `.printing-press-patches/`.

## Local Operating Contract

Start by asking the CLI for current runtime truth:

```bash
nynj-world-cup-concierge-pp-cli doctor --agent
nynj-world-cup-concierge-pp-cli agent-context --pretty
```

Use `extract --agent` for machine-readable output:

```bash
nynj-world-cup-concierge-pp-cli extract --agent
```

For trip-windowed activity feeds, use explicit filters instead of hard-coded assumptions:

```bash
nynj-world-cup-concierge-pp-cli extract --agent --category "Fan Experiences" --category "Watch Parties" --date-window-start 2026-07-02 --date-window-end 2026-07-06 --exclude-undated
```

This CLI is read-only. It does not book, reserve, purchase, register, authenticate, or mutate remote state.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`) so a regen carries the intent forward instead of silently dropping it.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the source catalog's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.

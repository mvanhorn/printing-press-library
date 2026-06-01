# NYNJ World Cup Concierge CLI Agent Guide

This directory is a `nynj-world-cup-concierge-pp-cli` printed CLI for public NYNJ World Cup Concierge discovery data. Keep changes narrow and record any hand customizations in `.printing-press-patches.json`.

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

The core extractor is a hand port of the Trip Control Tower Python prototype into the generated Go command surface. Preserve the scope: Explore NYNJ cards, Fan Experiences, and Watch Parties/Public Viewing guidance from public NYNJ sources.

If you modify this CLI beyond the current implementation, update `.printing-press-patches.json` with the files changed, the reason, and the validation outcome. Keep the index concise; the git diff remains the source of implementation detail.

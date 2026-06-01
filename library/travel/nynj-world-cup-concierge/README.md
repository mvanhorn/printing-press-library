# NYNJ World Cup Concierge CLI

Read-only Printing Press CLI for the public NYNJ World Cup Concierge and Host Committee fan-experience pages.

The CLI extracts normalized JSON candidates from public NYNJ World Cup 26 sources, including Explore NYNJ cards, Fan Experiences, and Watch Parties/Public Viewing guidance. It is designed for trip-planning agents that need official, source-linked activity candidates with stable IDs.

## Install

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/nynj-world-cup-concierge/cmd/nynj-world-cup-concierge-pp-cli@latest
```

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

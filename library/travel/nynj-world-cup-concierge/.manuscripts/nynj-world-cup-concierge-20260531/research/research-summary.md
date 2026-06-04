# Research Summary

The CLI targets public NYNJ World Cup Concierge and Host Committee data surfaces:

- `https://nynjfwc26.com/destination/`
- `https://nynjfwc26.com/fan-events/`
- `https://nynj-ai.neurun.com/api/race/event/guid/ef742ab9-0cc1-45dc-a173-739ec1eeb541`
- `https://nynj-ai.neurun.com/api/prompts/by-event/ef742ab9-0cc1-45dc-a173-739ec1eeb541?lang=en`

The extraction scope is intentionally narrow: Explore NYNJ cards, Fan Experiences, and Watch Parties/Public Viewing guidance. The generated CLI preserves the Trip Control Tower prototype's date-window filtering so downstream workflows can request activities overlapping a trip window such as July 2 through July 6, 2026.

Validation covered fixture-based unit tests, live source smoke tests, `cli-printing-press verify-skill`, full live dogfood, scoped `go build`, and scoped `govulncheck`.

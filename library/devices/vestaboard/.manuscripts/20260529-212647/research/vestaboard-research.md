# Vestaboard research — 20260529-212647

## API surface

Vestaboard exposes a small REST API for reading and updating a single board.
This print targets the public/private API shape documented at
<https://docs.vestaboard.com/> and the character-code reference at
<https://docs.vestaboard.com/docs/charactercodes>.

Generated commands cover:

- `message get` — fetch the current board message/layout.
- `message send` — post a new message body, accepting either `--body-json` or
  JSON from stdin for agent-safe scripting.
- `message preview` — render the board's 6x22 character-code layout as readable
  text so agents do not have to inspect raw integer arrays.
- `characters` — print the local Vestaboard character-code table needed to build
  `characters` array payloads by hand.
- `transition get` / `transition set` — inspect and update board transition
  configuration when the account/API key exposes those endpoints.

## Authentication notes

The API uses a per-call API key supplied through `VESTABOARD_API_KEY`. The key is
optional for build, help, and local preview/table operations, but required for
live `message` and `transition` calls. No browser-session or OAuth token is
required.

## Generation decisions

- The upstream OpenAPI coverage is minimal, so this print carries a bundled
  internal YAML spec and keeps the CLI source self-contained.
- The character-code table is intentionally local static data. Vestaboard does
  not expose an endpoint for this table, but it is necessary for composing and
  interpreting `characters` array payloads.
- Color chips render as one-column lowercase initials in previews:
  `r/o/y/g/b/v/w/k`. Code `71` renders as a filled cell (`█`). Unknown integer
  codes render as `?` so unexpected payloads are visible instead of silently
  blank.
- `message send` keeps raw JSON body input because Vestaboard payloads may be
  `text`-based or `characters`-array-based.

## Validation captured from the print

- `dogfood-results.json` reports the Cobra command tree as fully registered
  (`defined=29`, `registered=29`).
- Novel features check found both planned features: `message preview` and
  `characters`.
- MCP surface parity passed by runtime Cobra walking.
- Live Phase 5 was skipped because no `VESTABOARD_API_KEY` was available in the
  generation environment; see `proofs/phase5-skip.json`.

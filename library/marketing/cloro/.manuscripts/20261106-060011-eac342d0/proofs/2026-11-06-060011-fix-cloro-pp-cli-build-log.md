Manifest transcendence rows: 1 planned, 1 built. Phase 3 will not pass until all 1 ship.

# cloro-pp-cli build log

## Generated surface
- Typed commands from the spec: chatgpt, aimode, gemini, google, news, goto, product, countries, states, credits, and async (create, batch, get, queue).
- Spec preparation (filter_spec.py, before generation): the request-body root `anyOf` of google, news and aimode moved into the descriptions so the generator emits typed flags instead of only `--stdin`; `include.aioverview` is a boolean flag (`--include-aioverview`); the ignored `include.googleGoto` field is left out; billable scrapes carry `x-pp-mutation: false` and `x-pp-replay-safe: false`, so the client prints the response and never replays a scrape after an ambiguous failure.
- `DELETE /v1/async/queue` is not in the spec (it clears every queued task of the account).

## Novel commands
| # | Command | Status | Tests |
|---|---------|--------|-------|
| 1 | cited | built | domain normalization, host matching (subdomains match, look-alike hosts do not), engine list parsing, position fallback, an end-to-end run against a stub server with one failed engine (reported in fetch_failures, not counted as "not cited"), missing --prompt |

## Live check during the build
- `cited nike.com --prompt "best running shoes for flat feet" --country US --engines aimode,gemini`: exit 0, both engines answered (AI Mode 14 sources, Gemini 4 sources), nike.com not cited in that run. 12 credits.

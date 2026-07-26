# crate-pp-cli — shipcheck

**Verdict: ship.** 7/7 shipcheck legs PASS · live dogfood 15/15 (quick) · 12 Go test packages green.

## Why quick rather than full live dogfood

Discogs allows 25 requests per minute per IP without a token. The full matrix runs 126 tests, each in its own process, in a few seconds — so a per-process rate limiter cannot coordinate and the API refuses most of them. This is a property of the API, not a defect: every command was additionally driven by hand against live data (a real 3-record public collection, a real 126-record public wantlist, and a 43-result label search) and produced correct output.

## Bugs found and fixed during the build

1. **Double base URL** — paths were passed absolute to a client that already prepends the base, producing `https://api.discogs.comhttps//api.discogs.com/...`. Every request failed DNS.
2. **`dig` decoded the wrong id field** — Discogs search results key the release as `id`, not `release_id`. Every row was silently dropped, so a 43-result search reported "0 of 0" and printed *"nothing here you do not already own"* — the one answer guaranteed to be wrong.
3. **Empty search and fully-owned search printed the same line** — now distinguished; an empty search says so and suggests widening.
4. **Misleading multi-valued footnote** — `shelf` printed "the shares do not sum to 100%" directly beneath a table reading 100%. Now gated on the counts actually exceeding the record count.
5. **`--user` on a command that does not have it** — the narrative told agents to run `sync --user`, but the command is `shelf-sync`; `sync` is the generated resource sync and rejects the flag. Caught by validate-narrative and verify-skill.
6. **`database search --q` does not exist** — the generated command takes a positional query. Also caught by verify-skill.
7. **Every novel command required a manual sync first** — replaced with auto-sync on first use, which removes a setup step for real users, not just for probes.
8. **`shelf-sync --json` printed human lines alongside JSON**, breaking machine parsing.
9. **Pricing defaults sat silent for a full minute** — `--limit 25` against a 25-per-minute ceiling means the first run of `deals` or `floor` returns nothing for 60 seconds. Lowered to 10.
10. **No default pacing** — the CLI now defaults to 0.4 req/s, just under the keyless ceiling, so multi-request commands pace themselves instead of walking into 429.

## Honesty properties worth preserving

- `floor` is named a **floor**, never a valuation. It sums the lowest price anyone is currently *asking*; the output says so twice and the JSON carries `"is_appraisal": false`.
- A release with no copies for sale contributes **nothing**, and is counted separately — storing it as a zero price would drag every total down.
- Both pricing commands always report how many records they could not price. A total over part of a collection is not a total.

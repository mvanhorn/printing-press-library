# hostfully-pp-cli build log (reprint)

## Built
- Ported every hand-written command from the prior printed CLI: whois, inbox (threads/messages/show/message/reply), quote, jobs (+ hidden `jobs list` alias), today, availability, guests, fetch, mock mode, and the archive analytics (vacancy, maintenance, turnover, occupancy, channels, revenue, patterns).
- Replaced the prior hardcoded unit->property map with a unit registry: HOSTFULLY_UNITS, units.json (HOSTFULLY_UNITS_FILE), properties in the local store, or live agency /properties discovery. Distinct trailing numbers in property names become unit numbers; otherwise 1..N by name. New `units` command shows the mapping and its source.
- Added client.GetNoRetry / WithNoRetry (one-line hook in the generated retry loop) for the leads-page bisection.
- Adapted ported code to the current generated client (context-aware Get/Post, read-intent PostQuery for GraphQL and quote calculation) and error helpers.
- `--mock` is now a root persistent flag registered from a novel hook instead of a root.go edit.
- Scrubbed operator-specific names, real phone numbers and property UIDs from code, tests and help text; tests pin placeholder UIDs.

## Deferred
- None of the approved rows.

## Generator limitations found
- Generated internal/mcp tests fail when HOSTFULLY_API_KEY is set in the environment (credential-scoped store filename vs expected data.db); generation was run with credentials unset.
- dogfood's novel-feature scanner does not follow `addNovelCommandIfAbsent` wiring, so a two-word novel command (`jobs list`) reports a false depth mismatch; shipped `jobs` as a leaf instead.

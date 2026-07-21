# Agentic Review — robinhood-agentic-pp-cli (Phases 4.8 / 4.9)

Two parallel reviewers (SKILL semantic + README/SKILL/AGENTS correctness) audited the shipped docs against the built CLI. They surfaced 13 error-level and 8 warning-level findings; every one was fixed. All fixes re-verified: shipcheck 7/7 PASS, `go build`/`go vet`/`go test ./...` green.

## Error-level findings (all fixed)

1. **`--live-write` flag claimed but nonexistent** (README:129, SKILL:239). The auth narrative claimed placement needed `ROBINHOOD_AGENTIC_PP_ALLOW_WRITES=1` "plus `--live-write`" and that place "defaults to the server-side review simulation." No such flag exists, and the shipped model blocks the mutation at the transport with an error (not a review fallback). → Rewrote the auth narrative in README, SKILL, and research.json to the actual model: reads + `review` always allowed; place/cancel/watchlist/scan writes blocked unless the env gate is set; `guard` policy + `audit` journal on top; review-first is the recommended preflight.

2. **`brief` overclaimed scope** (README:202, SKILL:97, root `--help` Highlights, research.json). Descriptions promised "watchlist movers, earnings for held symbols," which the shipped command did not return. → Rather than downgrade the claim, **completed the feature**: `brief` now fetches top movers among held symbols (quote %-change) and upcoming earnings for held symbols, with pure `topMovers`/`positionSymbols` unit tests. Descriptions updated to match; root Highlights re-synced via dogfood.

3. **`brief` recipe selected nonexistent fields** (README:222, SKILL:200): `--select portfolio.total_value,orders,movers` — `orders` is `open_orders`, and `movers` didn't exist. → Fixed recipe to `--select portfolio.total_value,delta,open_orders,movers` (movers now exists).

4. **Examples omitted the required `--account`** for `portfolio winrate`, `wheel status`, `equities settle` (README:168/175/191, SKILL:63/70/86) — copy-paste would exit 2. → Added `--account` to every example in README, SKILL, and research.json.

## Warning-level findings (all fixed)

- **`writes are triple-gated (env + flag + review-first)`** (SKILL:44) implied a nonexistent flag → `(env write gate + guard policy + review-first)`.
- **Non-goals incomplete** (SKILL:44): added dividends feed and banking/credit-card MCP to the anti-trigger sentence.
- **`watchlists quotes Tech`** used a display name where a list-id is required → replaced with a UUID + "get the id from `watchlists list` first" (README:246, SKILL:224, research.json).
- **Quotes "batched transparently past 20"** (README:140) misdescribed behavior → "max 20 symbols per call."
- **`accounts list`** (README:138, 486, 487) — `accounts` has no subcommands → `accounts`.
- **Token env marked Required: Yes** (README:469) → "No (optional override)" with an accurate description (auth login is primary).
- **Hidden command groups** (README:261): softened the "run `--help` for the full reference" line since the six resource groups are `Hidden:true` in top-level help; point to `<group> --help` + the Commands section.

## Not changed (reviewed, correct as-is)
- Value-prop line describing `review` as "the server-side order simulation as the default dry-run" — accurate: `equities review` / `options review` ARE the server-side dry-run analog; not a claim about place's default.
- Claude Desktop MCPB token prompt — the bundle path legitimately accepts either reused OAuth tokens or a pasted token override.

## Output review (Phase 4.85)
Structural output plausibility verified for every command via dry/empty paths (correct exit codes, honest empty envelopes). Live-output plausibility review over real account data is deferred to Phase 5, which is read-only and pending Kevin's one-command `auth login` (see acceptance report).

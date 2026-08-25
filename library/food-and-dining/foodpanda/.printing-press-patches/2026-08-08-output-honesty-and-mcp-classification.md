# Output honesty in novel commands, and local-write MCP classification

Applied during `/printing-press-polish`. Each item below is a decision a
regeneration would otherwise re-derive incorrectly, because the naive
implementation is the wrong one in every case.

## Missing data must not render as zero

`market-compare` built its median and minimum delivery fee from every sampled
vendor including the ones foodpanda returned no price for. Those arrive as
`MinDeliveryFee == 0`, so an unpriced market rendered a confident `0 MYR` —
read by a user as free delivery rather than absent data.

Only priced vendors enter the fee arrays. `fee_priced_count` is always emitted,
an unpriced market gets `n/a` plus a `fee_note`, and a market priced by fewer
than a fifth of its sample is labelled `thin fee data` with the count inline
(`5.90 MYR (1)`), because a one-vendor median is not comparable to a
48-vendor one.

Fees also print with two decimals. `%.0f` turned Singapore's 4.30 SGD into
`4 SGD`; SGD, MYR and HKD fees are single-digit with real cents.

## An aggregate cannot rank against its own components

`digest` splits a blended star rating into per-topic scores. foodpanda attaches
an `overall` pseudo-topic to every review alongside the real ones, and it was
competing in the best/worst ranking — so `worst_topic` reported `overall`,
which is the exact number the command exists to decompose. It answers nothing
about whether the food or the rider was the problem.

`overall` stays in `topics` and is excluded from best/worst selection only.

## Truncation must be disclosed where it happens

`coverage` stops after `--max-scan-pages` and reported only what it saw. At a
dense coordinate that meant "192 vendors deliver here" when 620 were available
— a 69% undercount presented as a complete answer. The scan-cap note existed
but fired only when the result set came back empty, which is the one case where
the truncation does not mislead.

`available_count` and `scan_cap_hit` are now always emitted, the note fires
whenever the cap is hit, and the human header reads `192 of 620 … (partial —
scan capped)`.

## A flat market is not a broken sort

`fees` at a coordinate where foodpanda runs a single flat rate returns rows
that are identical in every column, so `--sort` visibly does nothing. Say the
fee structure is uniform rather than letting the user conclude the sort was
ignored.

## `dish` and `menu-diff` are local-write, not read-only

Both persist menu snapshots via `SaveMenuSnapshot`, so `mcp:read-only` was a
false promise. It matters most for `menu-diff`: `--save` defaults to true, so a
call records the snapshot the *next* call diffs against — the command is not
idempotent, which is precisely what `readOnlyHint=true` claims.

Both carry `mcp:local-write` instead. Writes never leave the CLI's own SQLite
store, so the tools stay non-destructive and closed-world; they simply are not
pure reads. Escape hatches: `menu-diff --save=false`, `dish --no-snapshot`.

## Deliberately not changed

`find`'s zero `match_score` rows are the feature, not a defect. foodpanda's
search endpoint is fuzzy and never returns an empty set, so labelling the
non-matches `0`/`none` beside `search_caveat` and the
`strong_matches`/`weak_or_no_match` counts is the disclosure this command
exists to provide. Defaulting `--min-score` above 0 would hide the upstream
noise it is meant to expose.

`defaultSyncResources()` stays empty. The only bulk-list endpoint is
geo-scoped and needs latitude, longitude and country at runtime, so there is no
safe zero-arg default; bare `sync` emits a hint pointing at `--resources`, and
README Quick Start carries the fully-formed command.

---

## SUPERSEDED 2026-08-20 — read this before acting on the section above

The `dish` / `menu-diff` local-write classification above describes a design the
code has since moved past. **Do not re-apply it on reprint.** Verified against
the tree on 2026-08-20:

- `dish --snapshot` defaults to **false** (`internal/cli/dish.go`), not "always
  snapshots with a `--no-snapshot` escape hatch". There is no `--no-snapshot`
  flag; that name never shipped.
- `menu-diff --save` defaults to **false** (`internal/cli/menu_diff.go`), not
  true. A plain diff is idempotent.
- Because both writes are now opt-in and off by default, `mcp:read-only` is
  honest for the default path and is what both commands actually carry.
  `mcp:local-write` was correct only under the old always-write defaults.

Opt-in-and-off is the stronger design: it makes the common path a pure read
instead of annotating a write as one. The reasoning in the original section is
still why the *old* code could not claim `readOnlyHint`; it is no longer a
description of this CLI.

Residual: with `--snapshot` / `--save` passed, both commands do write locally
while still advertising `mcp:read-only`. Inert today — the MCP server exposes
only menu_get, reviews_list, vendors_list, vendors_search, search, sql and
context, so neither command is reachable over MCP. It becomes real the moment
the MCP surface is widened to the Cobra tree (`cli-printing-press mcp-sync`).

The output-honesty items in the first half of this file (fee_priced_count,
scan_cap_hit, the `overall` topic exclusion, two-decimal fees) are all still
present and correct.

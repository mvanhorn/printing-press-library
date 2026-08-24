# Novel-features brainstorm — Woolworths AU (first print)

Subagent output, preserved verbatim for retro/dogfood debugging.
Customer model and killed candidates do NOT enter the manifest but must be persisted.

## Customer model

**Priya Raman — the Sunday-night online shop.** Household of four, western Sydney, ~$260/week,
delivery Monday.
- *Today:* keeps a running list in Notes, types each line into the site search box, eyeballs the
  first tile, clicks Add. 25-40 searches one at a time. No idea what last week's total was for
  the same list because Woolworths only shows the current order.
- *Weekly ritual:* Sunday 8:30pm, search-add-search-add for ~35 minutes, glance at Specials at
  the end, check out.
- *Frustration:* cannot answer "is this list more expensive than a fortnight ago, and which three
  lines caused it?" The trolley shows a total; nothing shows a trend.

**Dane Whitlock — the half-price cycle hunter.** OzBargain regular, chest freezer, stockpiles.
- *Today:* watches Wednesday catalogue rollover, scrolls `specialsgroup.3676` (1,655 items live),
  keeps a mental model of which brands cycle. Decides on gut feel whether an item returns in three
  weeks or twelve.
- *Weekly ritual:* Wednesday morning, open Half Price group, scroll several pages, screenshot
  anything interesting, cross-reference OzBargain for "is this the real low?"
- *Frustration:* the site cannot tell him what is *new* in the half-price set this week vs last
  (flat list, no diff), nor when the current window closes / when the item last ran.

**Ellen Novak — the SAVE-badge sceptic.** Retired, single-person household, Ballarat, buys bulk on
unit price.
- *Today:* distrusts "SAVE $2.50" having watched shelf prices step up before a "special".
  Compares `CupString` across pack sizes by hand, gets confused when one is per 100 g and the next
  per 1 kg. Skips multibuy tiles because she cannot do per-unit math at the shelf.
- *Weekly ritual:* fortnightly, pick a category, page through sorted by price, hand-convert unit
  prices onto paper, buy the winner in quantity.
- *Frustration:* no way to know whether today's price is genuinely low or merely lower than an
  inflated was-price; no way to compare 2-for-$9 against a bigger single pack without arithmetic.

**Marcus Iyer — the agent harness builder.** Melbourne dev wiring household automation into Claude
Code and Home Assistant.
- *Today:* installed elijah-g/Woolworths-mcp, hit the Puppeteer dependency, found (per its own
  issue #2) that raw responses need ~99% slimming before the model can use them. Today's prices
  only; no history behind any tool call.
- *Weekly ritual:* scheduled job that should answer "what's worth buying this week" and instead
  dumps hundreds of KB of product JSON into a context window.
- *Frustration:* every existing tool is a thin endpoint proxy. Nothing returns a *verdict* — a
  small, decided, agent-shaped answer — and nothing has memory, so the agent can never say
  "cheaper than usual" about anything.

## Candidates (pre-cut)

16 generated. 6 kept, 10 killed. Sources: (a) persona-driven, (b) service-specific content
pattern, (c) cross-entity local query.

Kept: `real-special` (a/b), `cycle` (b), `basket` (a/c), `swap` (a/c), `specials-diff` (b/c),
`multibuy` (b).

Killed inline with reasons — see Killed candidates table below.

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `cheapest --category` | Thin renaming of `browse/category` + `SortType=CUPAsc`, already emitted by the generated surface | `swap` |
| `pantry scan <barcode>` | Barcode lookup lives only on the revoked mobile API (401 `invalid_client`); a local barcode index means walking the whole catalogue, which the brief forbids | `basket` |
| `watch --notify` | Alerting needs an external mail/webhook service; polling half duplicates framework `tail --follow` | `specials-diff` |
| `history <stockcode>` | Raw dump of one local table with no synthesis — absorbed item 23 in its weakest form | `real-special` |
| `macro --per protein` | `Nutrition` only arrives per-product-detail, so any useful ranking violates crawl-breadth and returns silently partial results | `multibuy` |
| `origin` | Fails weekly use (one-time per-product preference check) and needs a detail call per line | `basket` |
| `stores near --open-now` | Absorbed store locator plus one clock predicate; a wrapper | none (dropped outright) |
| `scout <term>` | Both underlying endpoints absorbed; added value is a single ratio — a curiosity, not a ritual | `specials-diff` |
| `fill <list-file>` | Resolution logic is `basket`'s, write is the absorbed trolley endpoint; a loop over absorbed surface | `basket` (folded in as a flag) |
| `track add\|rm\|ls` | Sync-scoping plumbing edited monthly at most, not a user-facing weekly query | `cycle` (belongs to the data layer) |

## Pass 3 force-answers (survivors)

- **`real-special`** — Weekly: yes. Wrapper: no, one endpoint cannot answer it at all.
  Transcendence: local SQLite history + was-price content pattern. Sibling killed: `history`
  (raw dump makes the user do the judging). Buildability: `hand-code`.
- **`cycle`** — Weekly: yes, stockpile decisions are per-item and recurring. Wrapper: no, the API
  has no historical endpoint of any kind. Transcendence: local episode detection. Sibling killed:
  `scout`. Buildability: `hand-code`.
- **`swap`** — Weekly: yes, substitution hits several lines every shop. Wrapper: no — `CUPAsc`
  orders within one measure basis only; cross-basis normalisation and historical-low annotation
  are ours. Sibling killed: `cheapest`. Buildability: `hand-code`.
- **`basket`** — Weekly: yes, this is Priya's Sunday ritual verbatim. Wrapper: no, N resolutions
  plus a historical join in one call. Transcendence: agent-shaped verdict output + local history.
  Sibling killed: `fill`. Buildability: `hand-code`.
- **`specials-diff`** — Weekly: yes, keyed to Wednesday rollover. Wrapper: no, needs two snapshots
  and the API only exposes now. Sibling killed: `watch`. Buildability: `hand-code`.
- **`multibuy`** — Weekly: yes, multibuy tiles appear on most category passes. Wrapper: no, the
  math is not in any response field. Sibling killed: `macro`. Buildability: `hand-code`.

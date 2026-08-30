# Agoda CLI build log

Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

## Correction recorded during the build
At the Phase 1.5 gate I told the user that novel row #8 (offline corpus search)
would be covered by a framework-provided `search` command, reducing hand-code
from 8 to 7. That was wrong for this CLI: the generator emits `search`/`sync`
only for resources it classifies as syncable, and this spec's two REST resources
(a destination lookup and a per-hotel review fetch) are not. The CLI shipped with
no `search` command at all.

Rather than silently drop an approved shipping-scope feature, it was hand-built:
a `properties` table plus an FTS5 index, populated as a side effect of every live
search, and a root `search` command over it. Hand-code count is therefore 8, not 7.

## Built
### Sibling client: internal/agoda/
- `client.go` - paced HTTP client. `cliutil.AdaptiveLimiter` starting at 1.5 rps,
  `*cliutil.RateLimitError` on exhausted 429s, GraphQL-error detection (Agoda
  returns HTTP 200 with a populated `errors` array on failure).
- `search.go` - citySearch build + parse. Embeds the captured 30KB operation
  document because introspection is disabled upstream.
- `pricetrend.go` - priceTrendSearch; tolerates both object and array shapes for
  `PriceTrendSearchDetails`.
- `suggest.go` - destination autocomplete (text -> cityId).
- `requestid.go` - v4 ids for non-nullable searchId/correlation fields.
- `search_test.go` - 11 table-driven tests covering the currency double-write,
  markup computation, sold-out rows, string-encoded numbers, and date normalization.

### Commands (all hand-written)
| Command | Kind |
|---|---|
| `hotels search` | novel - true all-in pricing, `--sort true-price` |
| `hotels rank` | novel - re-rank by all-in cost, reports measured rank movement |
| `hotels fees` | novel - fee-ratio outliers vs destination median |
| `prices cheapest` | novel - whole-window sweep via priceTrendSearch |
| `vip delta` | novel - authenticated vs anonymous price diff |
| `watch run` / `watch add` / `watch list` | novel - local price history + drop detection |
| `compare` | novel - finalists side by side |
| `search` | novel - offline FTS5 over the accumulated corpus |
| `destinations` | absorbed - generator-emitted from spec |
| `reviews` | absorbed - generator-emitted from spec |

### Store
`internal/store/agoda_migrations.go` (hand-authored, regen-safe):
`price_observations`, `price_watches`, `properties`, `properties_fts` (FTS5 + triggers).

## Behavioral verification performed during the build
- True all-in pricing verified against live Tokyo data; markups 10-30.3%.
- `hotels rank`: 43 of 49 properties changed position once fees were included.
- `hotels fees`: destination median 21.0%, 5 outliers correctly flagged.
- `prices cheapest`: 62 days covered in one call; cheapest 40.3% below median.
- `watch run`: drop detection proven by seeding synthetic history 25% above
  current price - exactly 1 drop reported (-20.0%), 0 false positives across the
  other 48 priced properties.
- `search`: 88-property corpus after two live searches; "shinjuku" returned 10
  Shinjuku hotels, "osaka" returned only Osaka hotels, nonsense returned 0.
- Error paths: missing destination, one-id compare, non-numeric ids, bad window,
  and unsupported `--sort` all exit 2 with actionable messages.

## Real API behavior discovered during the build (not a code bug)
`citySearch` returns a **rotating subset** of a city's inventory per call rather
than a stable page, so a specific property can be absent from any single search.
This intermittently made `compare` report a bookable hotel as missing.

Fix: `compare` re-searches up to 4 times, cycling Agoda's supported sort fields
(`Ranking`, `Price` asc/desc, `Distance`), because each ordering returns a
different window of inventory. Verified 4/4 successful resolutions afterwards
versus roughly 1-in-2 before. When a property still does not surface, the
command says exactly that rather than implying it is unavailable.

Valid sort fields were probed empirically: `Ranking`, `Price`, and `Distance`
are accepted; `ReviewScore`, `Rating`, and `PriceAsc` return an opaque HTTP 400,
so `--sort` validates against a closed set up front.

## Intentionally deferred
- **PointsMAX vs AgodaCash valuation** - cut at Phase 1.5 and still cut. The
  `pricing.pointmax` field exists but its earn-rate semantics were never verified;
  shipping a wrong points valuation is worse than shipping none.
- **`saved list` / `account`** - the authenticated wishlist and VIP-tier surfaces
  were modelled from one observed `/bff/trips/...` call plus authenticated page
  state. A populated response was never observed, so they were not built rather
  than shipped on inference.
- Flights, activities, transfers, car rental - explicitly out of scope per the
  user's hotels-only directive, even though endpoints for some were observed.

## Generator limitations found
- Novel-command scaffolds default to `Annotations: {"mcp:read-only": "false"}`
  even for obviously read-only features; each was corrected by hand.
- Scaffold parent commands inherit the research.json `group` string as their
  Cobra `Short`, producing help text like "Honest pricing" for the `hotels`
  parent. Corrected by hand.
- All scaffold flags are emitted as `StringVar` regardless of the semantic type
  implied by the example (`--nights 2` became a string flag).

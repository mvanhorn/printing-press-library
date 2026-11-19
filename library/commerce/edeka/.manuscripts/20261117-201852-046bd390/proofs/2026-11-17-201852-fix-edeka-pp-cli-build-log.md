Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# EDEKA build log (2026-11-17)

## Built
- internal/edeka (hand-written): session (cookie jar, recording transport for Set-Cookie, locked atomic save, EDEKA_SESSION_FILE), market resolution (--market > EDEKA_MARKET > session > saved selection > only logged-in market), shop client (https://*.edeka.shop only, rate limiter, typed HTTP/429 errors, CSRF stoken cache with one refresh-and-retry), parsers (product grid, basket, calendar slot JSON, categories, product microdata, payment options, order review, generic form fields, order history), best-match ranking (category path, non-grocery penalty, plant-drink aliases), CDP loopback cookie import.
- Generated endpoints (spec): markets zip, markets cities, suggest, slots selected, slots reservation. A client hook rewrites the placeholder host market.edeka.shop to the selected market and seeds the session cookies (no second cookie copy in the generated credential store).
- Absorbed (hand-written): markets search/select/current, offers, products search/show, categories, basket show/set/remove/clear, slots list/reserve/release, payment options, order list, auth import/session.
- Transcendence (7/7): basket add (id or best match, --grams for weight articles), list add, slots earliest, order place (preview by default, --confirm to order, pay at handover only), deals check, price history (local SQLite edeka_prices), compare markets.
- Priority 1 review gate: products search / offers / basket show checked with --help, --dry-run and --json (all OK).

## Live proof during the build (account at doerflinger-ulm, no order placed)
- markets search 89081 -> 2 markets; markets search Stuttgart -> 2 markets (town lookup).
- products search milch, offers, categories --offers, products show 5915407005MXST, deals check, compare markets --zip 89081.
- auth import --file (9 cookies, logged in), auth session; basket show; basket add hafermilch -> best match; basket add <id>; basket set 3 -> 2; basket remove; weight article add with --grams 200 and removal through the basket form; list add --dry-run (2 matched, 1 not matched).
- slots earliest, slots list, slots selected (generated, through the hook): "reserved for 10 minutes".
- payment options (4 methods); order place --slot earliest --payment mxcash -> full review (slot, payment, items, total 5,97 EUR, ready_to_order true), NOT placed.
- Silent renewal: a session file without sid -> basket show works and a new sid is persisted; without oxid_1 -> exit code 4.

## Deferred / not built
- order show and order cancel: the test account has no order (placing one is not authorized), so the order detail page and any cancel control could not be observed.
- Delivery slots: the calendar only lists delivery areas until an area is chosen; the CLI reports the areas and a hint.
- Recipes: the shops have none.

## Generator notes
- Single-endpoint resources are promoted to `<resource> <arg>` commands; the spec gives markets and slots two endpoints each and names the suggester resource `suggest` to keep hand-written subcommands (markets search, products ...) clean.
- A --force regeneration merged a stale cliutil test copy and broke credentials_test.go (undefined helpers); fixed by restoring both files from a clean generation and re-applying the Windows skip patch.
- The sniffer collapses all OXID controllers into GET /index.php; the spec is hand-authored.

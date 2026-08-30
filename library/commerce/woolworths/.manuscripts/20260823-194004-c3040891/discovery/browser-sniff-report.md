# Woolworths (AU) — authenticated browser-sniff report

**Backend:** Claude chrome-MCP against the user's own Chrome (fresh capture tab in the MCP tab
group; no pre-existing tab was navigated).
**Date:** 2026-08-23. **Session:** user logged in manually; agent never handled credentials.
**Primary goal:** "See my shopping lists and past orders, and get items from them into my trolley."

## Auth model — settled

The fresh tab redirected to `https://auth.woolworths.com.au/u/login/identifier?...` with
`universal-login` in the state payload. **Woolworths uses Auth0 universal login with MFA.**

Consequences for the printed CLI:
- There is no username/password flow to implement. Do not attempt one.
- The only honest shape is **cookie import from a logged-in browser** (`auth login --chrome`)
  plus replay. This matches the `up_woolies` maintainer's note that Woolworths removed the
  programmatic login endpoint in favour of mandatory MFA.
- A new tab in the same Chrome profile inherits the session, so profile-level cookie import is
  sufficient; no per-tab capture is needed.

## Endpoints discovered (authenticated) — NOT in any existing tool

Captured by installing a `fetch`/`XHR` interceptor in the page and navigating client-side
(a full reload destroys the interceptor).

| Endpoint | Method | Status | Envelope |
|---|---|---|---|
| `/api/v3/ui/savedlists` | GET | 200 | `{success, data:{q:[...]}, statusCode, version, errors}` |
| `/api/v3/ui/savedlists/{listId}` | GET | 200 | same, `data.q` is a single object with `products[]` + `freeTexts[]` |
| `/api/v3/ui/pastshops` | GET | 200 | `{History:[], Errors:[], HttpStatusCode}` |
| `/api/v3/ui/savedlists/reorder` | (GET probe) | 400 | `{"Message":"The request is invalid."}` — exists, needs params/POST |
| `/apis/ui/UserPreferences/SetUserPreferenceListViewGroupBy` | POST | — | body `{Value}` |
| `/api/ui/v2/bootstrap` | GET | 200 | app config |

### Saved-list object shape (verified)
```
{ id:number, name:string, color:null, isGuest:boolean, isDefault:boolean,
  productCount:number, freeTextCount:number, referenceId:null, timestamp:number }
```
Detail adds `products:[]` and `freeTexts:[]`. **`freeTexts` is notable** — Woolworths lists hold
free-text lines as well as resolved products, which is exactly the "milk" -> stockcode resolution
problem the `basket` novel feature solves.

## The naming lesson

The resource is **`savedlists`**, not `lists`. Every guess failed:
`/apis/ui/lists`, `/apis/ui/Lists`, `/api/v3/ui/lists`, `/apis/ui/shoppinglists`,
`/apis/ui/list/{id}` all returned 404. `/api/v3/ui/lists` returned an ASP.NET
`"No HTTP resource was found"` — proving `/api/v3/ui/` is a live API root with a different
resource name. No amount of path guessing would have found `savedlists`; the interceptor did.

**Two API generations coexist on one site:**
- `/apis/ui/*` — older, PascalCase request and response keys (`Search/products`, `Trolley`).
- `/api/v3/ui/*` — newer, camelCase, wrapped in `{success, data:{q}, statusCode, version, errors}`.

The generated client must handle both envelopes. Do not assume one response convention.

## Replayability — PASS

All discovered endpoints are plain JSON GETs replayable with the session cookie jar. No
page-context execution, no persisted GraphQL hashes, no BFF proxy envelope, no WebSocket.
Nothing requires a resident browser at CLI runtime. The printed CLI ships pure HTTP; the browser
is generation-time discovery plus one-off cookie import only.

## Honest coverage gaps

- **`savedlists` item shape is unverified.** The test account's only list has
  `productCount: 0`, so `products[]` and `freeTexts[]` were both empty. The envelope is
  confirmed; the per-item field names are not.
- **`pastshops` returned `History: []`.** The account has no completed orders, so order-history
  item fields are unverified. Commands built on it must handle the empty case as the *normal*
  case and must not claim verified field coverage.
- **Everyday Rewards was not probed** and is deliberately out of scope: MFA-gated, ~30-minute
  tokens, 401 on every prior probe.

Both gaps are shape-unknown, not reachability-unknown. The endpoints answer 200 with a stable
envelope; only the array element schema is unconfirmed. Any command shipped against them must
degrade honestly on empty rather than assert a schema it has not seen.

---

# Follow-up capture: coverage gaps CLOSED (2026-08-24)

The original capture ran against an account with one empty list and no order
history, so `products[]` and `History[]` item shapes were recorded as UNVERIFIED.
A second account with real history was signed in and both gaps are now closed.

## savedlists — VERIFIED

`GET /api/v3/ui/savedlists` -> 9 lists, productCount `[42,0,3,0,0,0,0,0,0]`,
freeTextCount all `0`.

`GET /api/v3/ui/savedlists/{id}` on the 42-product list returned 42 items.
Verified `products[]` element shape:

```
{ id:number, listId:number, articleNumber:string, quantity:number,
  checked:boolean, referenceId:null, timestamp:number }
```

**Key finding: `articleNumber` IS the product `Stockcode`.** Probed the first 5
articleNumbers against `GET /apis/ui/product/detail/{stockcode}`: 5/5 HTTP 200,
5/5 with `Product.Stockcode` exactly equal to the articleNumber, 0 mismatches.

Consequence: a saved list can be priced with no extra identifier mapping - feed
`articleNumber` straight into `products detail` or the comma-separated
`products batch` endpoint. A "price my saved list" feature is therefore buildable
on the existing verified surface.

Note the list item carries NO price, name, or size - only the identifier,
quantity and a `checked` flag. Any display of a saved list must join against
product detail.

## pastshops — VERIFIED, and narrower than assumed

`GET /api/v3/ui/pastshops` -> 152 History entries. Verified element shape:

```
{ ProductCount:number, UniqueProductCount:number, BasketId:string,
  Channel:string, Date:string }
```

**Limitation: past shops are basket SUMMARIES, not itemised orders.** There are
no line items, no prices and no product identifiers on this endpoint. It answers
"how many items did I buy, through which channel, on what date" - it cannot
answer "what did I buy". Any feature promising itemised order history would need
a further endpoint that this capture did not find (and the Everyday Rewards
e-receipts stack, which is MFA-gated and out of scope, is where itemisation
actually lives).

## Authenticated commands: VERIFIED THROUGH THE BINARY (2026-08-24)

Cookie import DOES work on Windows. The `auth login --cookies-file` help text
claiming it "requires a cookie extraction tool (pycookiecheat, cookies, or
cookie-scoop-cli)" is WRONG - file import needs no extractor at all. Filed as a
retro candidate.

Verified end to end with a real session:
- `auth login --cookies-file <cookie-header file>` -> "OK Found 3 cookies", session saved
- `auth status` -> Authenticated, Source: config
- `savedlists list` -> 9 lists, the default "Weekly Basic" carrying 42 products
- `savedlists get 14768019` -> 42 products, 0 freeTexts
- `pastshops list` -> 152 entries, channels {instore, online}, spanning 2025-08-27 to 2026-08-23
- **End-to-end identifier proof:** articleNumber 274326 from the saved list fed
  straight into `products detail` returned stockcode=274326, $5.00, $2.86/100G.
  Confirms articleNumber == Stockcode against the shipped binary, not just the browser.

Two capture gotchas worth recording for the next person:
1. DevTools "Copy value" on the RESPONSE headers yields `set-cookie:` lines with
   expires/path/secure/HttpOnly attributes. The CLI needs the single-line REQUEST
   `Cookie:` header. A set-cookie dump can be converted by keeping only the
   leading name=value of each line.
2. Notepad silently appends a second extension (`wool-cookies.txt.txt`).

Session cookies are ~1-hour JWTs, so this import expires and must be repeated.
The plaintext cookie files were deleted after import; the CLI keeps its own copy
in credentials.toml.

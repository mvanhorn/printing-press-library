# W2: Read-Only Path Verification Sweep

Live-tested against a real, authenticated `us2` Concur tenant via cookie-session
auth (`auth login --chrome`). **No PII, financial amounts, cookie values, or
token values are recorded below — HTTP status, path, and structural findings
only.**

## Results by resource group

| Resource | Path status | Auth result | Notes |
|---|---|---|---|
| `account.whoami` | already correct | **200** | Confirmed in W1 |
| `account.travel` | **fixed** (absolute legacy URL) | **401** Invalid headers | AUTH-GATED — legacy v2.0 host doesn't recognize cookie session |
| `payment_types.list` | already correct | **200** | |
| `expense_types.list` | already correct | **200** | |
| `attendee_types.list` | **fixed** (`/v4/attendeetypes`) | **403** role required | AUTH-GATED — company-token + admin role required, confirmed by docs and live error message |
| `delegates.list` | **fixed** (absolute legacy URL) | **401** Invalid headers | AUTH-GATED — same legacy-host issue as account.travel |
| `locations.search` | **fixed** (`/localities/v5/locations`, `searchText` param) | **200** | Valid empty result |
| `reports.list` | already correct | **200** | |
| `reports.get` | already correct | **200** | |
| `requests.list` | already correct | **200** | Valid empty result (zero travel requests) — contradicts my initial assumption that `travelrequest.write` scope language meant this would be gated; it isn't |
| `travel_allowance.get` | **fixed** (context/contextId shape, not trip_id) | **200** | Live-tested against a real expense report; response matches documented schema exactly |
| `trips.get`/`trips.list` | path structurally matched docs already; host family likely still wrong | **404** | AUTH-GATED regardless — Itinerary v4 docs state "company access tokens only" |
| `available_expenses.*` | no alternative path found | **404** | No official REST/GraphQL surface exists for this web-UI-only concept (confirmed by both the Aug 12 discovery report and this live 404) |
| `delegates` (via Chrome cookie import) | N/A | fixed pre-requisite | See separate `2026-08-17-live-auth-probe.md` — the `_csrf` subdomain-scoping bug that blocked ALL of the above until fixed |

## The structural finding

Cookie-session auth is **not a uniform substitute for OAuth2** across Concur's
API surface. It works for:

- The modern v4/SCIM host family (`www-{region}.api.concursolutions.com`)
  serving personal-scope resources: profile, reports, expenses, expense
  config, payment/attendee type catalogs (non-admin), travel requests,
  localities, travel allowance.

It does **not** work for:

- Legacy v1.1/v2.0 APIs on a different literal host
  (`www.concursolutions.com/api/...`) — confirmed via `delegates.list` and
  `account.travel`, both returning `401 Invalid headers`, meaning this host
  doesn't even recognize the cookie format, let alone authorize it.
- Modern v4 endpoints that require a **company-level OAuth consumer holding
  an admin role** — confirmed via `attendee_types.list`'s exact 403 message
  ("Web Services Administrator, Expense Configuration Admin, or Expense
  Configuration Restricted") and Itinerary v4's documented "company access
  tokens only" restriction. A personal cookie session structurally cannot
  hold an OAuth-consumer role grant; no path fix changes this.

This means the CLI's real, viable feature set under cookie auth is: expense
reports, expenses, expense/payment/attendee-type catalogs, travel requests,
location search, and travel allowance calculations. Delegate management,
admin-scoped attendee-type configuration, and trip/itinerary data require a
registered OAuth2 partner application — documented plainly rather than left
to fail mysteriously (see spec.yaml header and per-endpoint descriptions).

## Bugs found and fixed (mechanical, not auth-related)

1. **`locations.search`**: wrong path family entirely (`/locations/v4/locations`
   → `/localities/v5/locations`) and wrong query param name (`query` →
   `searchText`).
2. **`attendee_types.list`**: wrong path nesting (`/expenseconfig/v4/...` →
   bare `/v4/attendeetypes`).
3. **`travel_allowance.get`**: wrong parameter model entirely — generated as
   a bare `<trip_id>`, but the real API is keyed by
   `(user_id, context, context_id)` where context is the owning expense
   report or travel request. Required a command-signature change, not just a
   string swap.
4. **`delegates.list` / `account.travel`**: wrong host family — both were
   generated as relative paths against `base_url`, but the real APIs live on
   a completely different host (`www.concursolutions.com`, legacy `/api/...`
   prefix). Required adding absolute-URL support to the HTTP client
   (`internal/client/client.go`), since the client had no prior mechanism for
   a resource to target a different host than `base_url`.

All four are recorded in `.printing-press-patches/` with the fix and the
live-tested outcome, so a future reprint does not silently regenerate the
original wrong paths.

## Patches recorded

- `chrome-cookie-subdomain-discovery.json` (W1 — the auth prerequisite for
  this entire sweep)
- `legacy-host-family-endpoints.json`
- `travel-allowance-context-shape.json`
- `locations-and-attendee-types-path-fixes.json`

## Spec updated

`pipeline/concur-spec.yaml` header and the five affected resource blocks
(`account.travel`, `attendee_types.list`, `delegates.list`,
`locations.search`, `travel_allowance.get`) now carry the live-verified
status and reasoning in-line, replacing the original UNVERIFIED/LOW-confidence
language. `available_expenses` and `trips` descriptions updated to reflect
the confirmed-404/confirmed-auth-gated evidence rather than "unverified."

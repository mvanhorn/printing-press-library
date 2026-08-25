# Concur Discovery Report

Live authenticated browser-sniff against a real company Concur instance
(`us2` data center). **No PII, cookie values, or token values are recorded
below** — structural findings only.

## Auth Mechanism (confirmed empirically)

- Login flow: `us2.concursolutions.com/home` → redirect to `/signin` →
  Concur username entry → HRD (Home Realm Discovery) page → **Okta SSO**
  (username/password + TOTP MFA) → SAML redirect back to
  `us2.concursolutions.com`.
- Session is carried via cookies scoped to `.concursolutions.com` (parent
  domain) and `us2.concursolutions.com` (subdomain). No `Authorization`
  header observed on the app's own API calls — pure cookie auth.
- Cookie inventory (names/flags only, domain `.concursolutions.com` unless
  noted):
  - `JWT` — httpOnly, secure, SameSite=Lax. **Strong candidate for
    press-auth's `jwt_carrier_cookie`** (name literally says JWT).
  - `_csrf` — httpOnly, secure, scoped to `us2.concursolutions.com`. Matches
    the `x-csrf-token` header the API advertises via
    `Access-Control-Allow-Headers` (see below) — likely required for
    mutating GraphQL operations.
  - `origin_dc`, `brandingid`, `OTSESSIONAABQRN`, `OTSESSIONAABQRD`,
    `OTSEC670817`, `ak_bmsc` — httpOnly, secure. Session/bot-mitigation
    plumbing (Akamai `ak_bmsc`/`bm_sv`/`bm_sz`/`_abck` = bot manager;
    `OT*` = OneTrust consent).
  - `TAsessionID`, `travel_outtask_id` — not httpOnly, JS-readable. Likely
    Travel-module session correlation (TripLink/Outtask legacy naming).
  - Non-auth: `notice_behavior`, `OTLang`, `OTDefaultLang`, `s_fid`, `s_cc`,
    `_gcl_au`, `_uetsid`, `_uetvid`, `AWSALBTG`, `AWSALBTGCORS`,
    `dtCookied7d7pgji` — analytics/CDN/load-balancer affinity, not auth.
- CORS confirms cross-subdomain credentialed requests are intentional:
  `Access-Control-Allow-Credentials: true`,
  `Access-Control-Allow-Origin: https://us2.concursolutions.com` (exact
  origin echo, not wildcard) on responses from `www-us2.api.concursolutions.com`.
- `Access-Control-Allow-Headers` on the GraphQL endpoint advertises:
  `x-mx-reqtoken, x-csrf-token, appid, userid, x-token, x-apollo-tracing,
  x-sessionid, graphql-query-id, apollo-query-plan-experimental,
  concur-correlationid, concur-debug, newrelic, traceparent, tracestate,
  concur-route` — several of these (`x-csrf-token`, `x-sessionid`,
  `appid`, `userid`) suggest some mutating operations may require explicit
  headers beyond the cookie jar. Not yet verified against a real mutation
  (report create/submit) — flag as an open question for the CLI's first
  live dogfood pass.

## Internal API Surface (confirmed empirically)

- **The web app does NOT call the documented partner REST API
  (`api.concursolutions.com` v3/v4).** It calls an undocumented internal
  GraphQL BFF: `POST https://www-us2.api.concursolutions.com/cds/graphql`
  ("cds" likely = Concur Data Service). Same host *family* as the
  documented partner API (`www-{region}.api.concursolutions.com` is a
  real, documented base-URI variant per developer.concur.com/platform/base-uris.html)
  but a different, undocumented path (`/cds/graphql` vs `/expensereports/v4/...`).
  Cookie-authenticated, not OAuth2 Bearer.
- Sibling REST-shaped internal endpoint also observed:
  `GET www-us2.api.concursolutions.com/messagenexus/v1/messages/newMessageCount`
  — same host, same cookie auth, REST-shaped (not GraphQL). Confirms the
  internal API surface is mixed GraphQL + REST, not GraphQL-only.
- One GraphQL operation captured in full: `GetOnScreenHelpData` (session
  bootstrap/user-context query fired on page load — NOT expense/travel
  specific). Confirms operation-name-based GraphQL BFF pattern per
  browser-sniff-capture.md's detection criteria. Did not capture
  expense-report-specific or trip-specific operation names in this pass —
  see "Gaps" below.
- Region: this company's Concur entity resolves to the `us2` data center
  (`us2.concursolutions.com` web app, `www-us2.api.concursolutions.com`
  API). Base URIs are per-tenant; the generated CLI must treat region as
  configurable, not hardcoded (matches the documented multi-datacenter
  Base URIs reference).

## Prior Art Cross-Reference

A private, working Playwright-based automation tool for this exact Concur
tenant already exists locally (`work-projects/expense-report-filer`,
`work-projects/magnite-playwright-okta-auth`). It automates the UI
directly (Page Object pattern) rather than calling the GraphQL BFF, and
confirms:

- Direct deep-link works: `https://us2.concursolutions.com/nui/expense?confNum=new`
  opens the create-report dialog directly (skips nav clicking).
- Real workflow, confirmed working end-to-end: create report → pull
  "Available Expenses" queue (corporate card charges/e-receipts not yet on
  a report) → "Move" action onto the new report → per-expense-type
  Business Purpose + optional reimbursement-cap split (config-driven) →
  submit for approval.
- Auth: Concur signin → Okta HRD → Okta login (password + TOTP from macOS
  Keychain) → SAML redirect. Session persisted as Playwright
  `storage_state` (cookies + localStorage).
- UI has no stable `data-automation` attributes — the prior art uses
  accessible role/name selectors throughout, consistent with what this
  browser-sniff session observed.

## Gaps / Open Questions For Generation

1. **Expense-report and trip-list GraphQL operation names are not yet
   captured.** Only the generic session-bootstrap operation
   (`GetOnScreenHelpData`) was captured before pivoting to prior-art
   review. The generated spec's expense/travel endpoints will be authored
   from (a) the officially documented v3/v4 REST contract as the field/shape
   reference, wired as cookie-authenticated calls against the confirmed
   `www-{region}.api.concursolutions.com` host, with (b) a fallback path
   documented in the README for the user to re-run a scoped browser-sniff
   (creating one report, viewing one trip) and correct any operation
   names/paths that don't match once dogfooded against their live account.
2. **Whether documented v3/v4 REST paths are reachable at all with a
   cookie-only session (no OAuth token) is unverified.** Likely not — the
   web app itself uses the GraphQL BFF, not the partner REST API, which
   suggests the two are gated independently. Do not assume interchangeability.
3. **Whether mutating GraphQL operations need one of the advertised
   extra headers** (`x-csrf-token` from the `_csrf` cookie, most likely)
   is unverified — only a read-only session-bootstrap call was captured.

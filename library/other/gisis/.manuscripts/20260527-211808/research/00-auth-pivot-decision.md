# GISIS Auth Pivot — 2026-05-27

## Discovery (Phase 1 pre-research probe)

The execution log (`Machine/Claude/Meta/2026-05-27-Vessel-MCP-Execution-Log.md`)
pre-flight finding "28 public modules confirmed accessible without login" is
**wrong on the auth front**. Both target modules return HTTP 200 to anonymous
curl but the body is the WebLogin.aspx form, not the module content.

Evidence:
- `curl https://gisis.imo.org/Public/SHIPS/Default.aspx` returns HTML containing
  `<form ... action="./WebLogin.aspx?App=GISISPublic&ReturnUrl=https://gisis.imo.org/Public/SHIPS/Default.aspx">`
- Form has `ctl00$cpMain$txtUsername` text input and `ctl00$cpMain$hfTurnstileToken`
  (Cloudflare Turnstile) hidden input
- Explicit `btnRegister` button — registration is free
- Same shape for `MCI/Default.aspx`

"Public" in GISIS means free-to-register, not anonymous.

## Auth model

- type: cookie (or composed: cookie + Turnstile-gated login)
- Session: stored in ASP.NET cookies, scoped to gisis.imo.org
- Bot challenge: Cloudflare Turnstile on the login flow (scripted login impractical)
- Login is interactive: user logs in via real Chrome, CLI imports cookies

## Implementation plan

1. User registers at https://webaccounts.imo.org/ (free, email)
2. User logs into GISIS via Chrome at the module landing page
3. Browser-sniff captures the authenticated session
4. Generated CLI uses `auth login --chrome` pattern or press-auth companion
5. Phase 5 live smoke runs against same cookies

## Decision

User selected "Register, log in, continue (recommended)" at the auth pivot
question. Proceeding once user confirms login is complete.

## Action items for execution log update (post-run)

- Update Phase 1a pre-flight findings: modules require free registered IMO
  Web Accounts login, not anonymous access
- Update auth model in Vessel-AIS-MCP-Plan: GISIS source is cookie-auth,
  not no-auth
- Note: Cloudflare Turnstile blocks programmatic login — user must log in
  via Chrome and CLI imports cookies (browser-clearance / press-auth pattern)

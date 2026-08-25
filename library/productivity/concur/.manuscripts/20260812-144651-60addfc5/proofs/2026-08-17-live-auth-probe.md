# Live Auth Probe (W1 decision gate)

**No PII, cookie values, or token values are recorded below — structural findings only.**

## Command sequence

```bash
./concur-pp-cli auth login --chrome
./concur-pp-cli auth status --json
./concur-pp-cli account whoami --json
```

## Result: cookie auth reaches the documented REST API — CONFIRMED

| Step | Result |
|---|---|
| `auth login --chrome` (before fix) | FAIL — `cookie "_csrf" not found for concursolutions.com` |
| `auth login --chrome` (after fix) | PASS — `OK Found 2 cookies for concursolutions.com` |
| `auth status` | `Authenticated`, source `config`, domain `concursolutions.com` |
| `account whoami --json` | **HTTP 200**, exit code 0, live SCIM-shaped (`urn:ietf:params:scim:schemas` style `com:concur:*` schema namespaces) user-profile payload returned from `www-us2.api.concursolutions.com` |

## Blocker found and fixed before this probe could pass

`auth login --chrome` initially failed even though the user was actively logged into
`concursolutions.com` in Chrome. Root cause: the extraction step
(`extractViaPycookiecheat`) queried `pycookiecheat` against the bare apex domain
(`concursolutions.com`) only. Concur scopes `_csrf` host-only to the tenant's regional
web subdomain (`us2.concursolutions.com` for this tenant) — a real cookie-jar apex-only
query cannot see a host-only subdomain cookie. `JWT` is domain-scoped
(`.concursolutions.com`) and was found fine, masking the issue as a single-cookie
failure rather than a systemic domain-matching bug.

The Chrome-profile *discovery* step (a separate code path, used only for
auto-detecting which profile to read) uses a loose `host_key LIKE '%domain%'` SQL
match and correctly reported "required cookies present" — creating a misleading
signal that the user was logged in and had both cookies, right before the real
extraction step failed to retrieve one of them. This looked exactly like "not logged
in" but was actually a domain-mismatch bug. See discovery report
(`discovery/concur-discovery-report.md`, "Auth Mechanism" section) which had already
flagged the `_csrf` subdomain scoping as an open risk on Aug 12.

**Fix**: `auth.go` now discovers every Chrome `host_key` that holds a required
cookie (via the same DB-copy-then-sqlite3-query approach the discovery step already
used) and queries `pycookiecheat` against each discovered host, merging results.
Recorded in `.printing-press-patches/chrome-cookie-subdomain-discovery.json`.
`extractViaCookiesCLI` and `extractViaCookieScoop` were not patched — same latent bug,
but neither tool is installed here to test against; follow-up.

## Verdict on Open Question #1 (from Aug 12 discovery report)

> "Whether cookie-session auth authenticates the documented v3/v4-style REST paths at
> all is UNVERIFIED."

**Resolved: YES.** A cookie session (no OAuth2 Bearer token) authenticates at least
`account.whoami` on the documented REST host family
(`www-us2.api.concursolutions.com`) with HTTP 200. This does not yet confirm every
other resource group — that is W2's job — but it confirms the REST-shaped CLI is
generationally sound, not built on a wrong contract. No Akamai bot-management
challenge was encountered (no 403 with Akamai reference ID).

## Not yet answered

- Whether `_csrf` needs to additionally be sent as an `x-csrf-token` request header
  (rather than / in addition to a `Cookie:` header) for mutating calls — still
  unverified, still gated on a real write attempt (W4/W5, requires explicit
  authorization).
- Whether `available_expenses.*`, `trips.*`, `travel_allowance.get`, and
  `requests.*` paths are real — W2's job next.

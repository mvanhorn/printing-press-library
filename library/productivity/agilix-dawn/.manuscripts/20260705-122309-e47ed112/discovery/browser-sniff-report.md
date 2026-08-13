# Browser-Sniff Discovery Report — Agilix Dawn (drivered.agilixdawn.com)

## Method
Chrome extension (chrome-MCP) against the user's authenticated session. Opened a
fresh capture tab, read real XHR traffic, and replayed read-only GET calls with
the session token to capture response *shapes only* (all values stripped to
types — no PII, no values, and the token was never written to any artifact).

## Runtime / reachability
- Transport: plain HTTPS, standard_http. No Cloudflare/WAF/bot challenge. Clean 200s.
- SPA: SvelteKit, same-origin REST BFF at `https://drivered.agilixdawn.com/api/*`.
- Any unrecognized `/api/<x>` path falls through to the SPA HTML catch-all
  (verified with a control probe `/api/foobar` -> 200 HTML).

## Auth (verified)
- Session token in localStorage key `session` (no auth cookie; only GA cookies).
- Transport: `Authorization: <rawtoken>` header (no "Bearer " prefix) — verified 200.
  Also `?_authorization=<token>` query param — verified live (notification long-poll).
- Service/automation path: Dawn API-user accounts (`api_` id prefix) — durable tokens.
- 403 error envelope: {description, message, requestId, status}.

## Confirmed real API collection endpoints (GET /api/<name>?search=<urlencoded JSON>)
Response envelope: {"totalMatches": int, "matches": [ ... ]}
- concept        — catalog items / courses (RICH: sections->instructions->interactions,
                   resources, publisher, pricing, enrollment settings, languages)
- user           — users (id, email, givenName, familyName, status, verified)
- organization   — orgs (id, name, admin[], author[], email, payment.accountId, status)
- purchase       — commerce (id, charge, currency, objects[], purchasingUser, user[], state)
- progress       — learner progress (enveloped; empty for admin account, real for learners)
- conversation   — messaging (enveloped)
- resource       — real endpoint (HTTP 400 without required params; by-id/param fetch)

## Confirmed singletons / by-id
- GET /api/user/me      — current user (id, email, givenName, familyName, status, verified, version)
- GET /api/config       — tenant config (public; rootOrg, version, auth.oidcProvider, payment)
- GET /api/concept/{id} — full course object (returned directly, not enveloped)
- PUT /api/auth/user    — session touch (internal; not exposed in CLI)
- GET /api/notification — long-poll (internal; not exposed in CLI)

## NOT real endpoints (returned SPA catch-all — earlier research over-inferred these)
enrollment, enrollmentGroup, course, grade, certification, certificate, activity,
offer, order, report, role, publisher (as top-level search). Enrollment/group data
is reachable only via `join` inside progress queries, not as standalone collections.

## Search DSL (Lucene-style, inside the `search` param)
{ "query": "<lucene>", "start": 0, "limit": N, "sort": [{"field":"asc|desc"}],
  "join": ["path", ...], "include": ["field", ...] }
- `search={...}` wrapper is MANDATORY. Top-level `?limit=2` returns totalMatches
  but an EMPTY matches[]. Verified.
- Object id prefixes: u_ user, c_ concept/content, r_ resource, org_ org, api_ API user.

## Tenant context
Idaho Home Driver Education — Idaho Parent-Led driver's-ed curriculum. Small
catalog (2 concepts). Stripe-backed commerce (live publishable key present in config).

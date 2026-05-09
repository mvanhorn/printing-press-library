# Auth0 SPA localStorage Harvest

## Why we don't run an OAuth code flow

Peloton's web SPA authenticates against Auth0
(`peloton.auth0.com`) using
[`@auth0/auth0-spa-js`](https://github.com/auth0/auth0-spa-js).
Standalone OAuth would mean:

1. Registering a public client in Peloton's Auth0 tenant — we don't
   own that tenant, so we can't.
2. Driving Auth0's hosted login UI from a CLI — would require Device
   Code Flow, which Peloton's tenant has not enabled (no
   `urn:ietf:params:oauth:grant-type:device_code` grant available).
3. Solving CAPTCHAs or "unusual activity" interstitials Auth0 throws
   at non-browser clients — the SPA gets a clean session, a CLI
   does not.

The browser already has all of those problems solved: the user
signs in interactively, the SDK persists the OAuth response in
`localStorage`, and the access token is **right there** for any
JavaScript context that can read the SDK's cache key.

## Cache key shape

`auth0-spa-js` writes the session into `localStorage` under a key
shaped like:

```
@@auth0spajs@@::<client_id>::<audience>::<scope>
```

For Peloton's SPA, the values look like:

| Slot | Value |
|---|---|
| `client_id` | Auth0 SPA client id (Peloton-tenant-specific; rotates occasionally) |
| `audience` | `https://api.onepeloton.com/` |
| `scope` | `openid offline_access` |

The cached value is a JSON envelope:

```jsonc
{
  "body": {
    "access_token": "eyJ…",   // bearer for api.onepeloton.com
    "refresh_token": "v1.…",
    "id_token": "eyJ…",
    "expires_in": 3600,
    "token_type": "Bearer",
    "scope": "openid offline_access"
  },
  "expiresAt": 1747380000     // ms epoch
}
```

A separate sibling key holds the cached user profile:

```
@@auth0spajs@@::<client_id>::@@user@@
```

…with `decodedToken.user.sub` of shape `auth0|<bare-id>`.

## The harvest

`internal/cli/auth_login.go::readTokenExpr` is a small JS expression
that runs in the chromedp page context after navigation:

```js
for (var i = 0; i < localStorage.length; i++) {
  var k = localStorage.key(i);
  if (k && k.indexOf('@@auth0spajs@@::') === 0
       && k.indexOf('api.onepeloton.com') !== -1) {
    var v = JSON.parse(localStorage.getItem(k));
    if (v && v.body && v.body.access_token) out.token = v.body.access_token;
  }
  if (k && k.indexOf('@@auth0spajs@@::') === 0
       && k.indexOf('@@user@@') !== -1) {
    var u = JSON.parse(localStorage.getItem(k));
    var sub = u.decodedToken.user.sub || '';
    out.user_id = sub.indexOf('|') !== -1 ? sub.split('|').pop() : sub;
  }
}
```

Two design choices worth calling out:

1. **Discover the `client_id` at evaluation time** rather than
   hardcoding. The substring match on
   `@@auth0spajs@@::` + `api.onepeloton.com` lets a Peloton client
   rotation slide through without code changes.
2. **Strip the `auth0|` prefix from `sub`**.
   Peloton's `/api/user/{id}/workouts` wants the bare id; the JWT
   `sub` claim is always `auth0|<id>`. We learned this the slow way
   — initial commit harvested the raw `sub`, every workout call
   404'd, and the fix landed in commit
   [`1a73c159`](../../../../../commit/1a73c159) before the WIP
   handoff.

## Token lifetime + refresh

Access tokens live ~1 hour Peloton-side. Auth0 also issues a
refresh token, but using it requires the client_id and an Auth0
refresh-grant call; that path is more fragile than just re-running
`auth login`, which finishes in seconds against the persistent
Chrome profile (session cookies are still good, the user just
clicks through). We deliberately don't implement refresh — it adds
tenant-shape coupling without saving real time.

## What this enables that nothing else in the catalog does

`peloton-pp-cli auth login` is the **first chromedp-driven OAuth
SPA harvest** in the printing-press catalog:

| Approach | Catalog precedent |
|---|---|
| Static API key in env | most generator-emitted CLIs |
| Cookie scrape from real-browser cookie store (`kooky`) | instacart |
| Surf with Chrome TLS fingerprint, no cookies | apartments, redfin |
| **chromedp + Auth0 localStorage harvest** | **peloton** (new) |
| Interactive browser-cookie-clearance import | dominos (sessionStorage harvest) |

The closest precedent is dominos, which spawns Chrome and reads
**sessionStorage** for a bearer token. Peloton's SDK uses
**localStorage**, the cache-key format is different, and the
extracted ID needs a prefix strip. The patterns rhyme; the code
doesn't share.

## When this will break

- Auth0 SPA SDK swap. `localStorage` writes happen via
  [`InMemoryCache`](https://github.com/auth0/auth0-spa-js/blob/main/src/cache/cache-memory.ts)
  by default but Peloton has the `@LocalStorage` cache enabled.
  If they switch to the in-memory default, our harvest sees nothing.
- Auth0 cache-key format change. Auth0 has bumped the key format
  twice in the SDK's history (once added the `@@scope` slot, once
  added the audience). Future bumps would require updating the
  `indexOf` substrings.
- Peloton hardens against scripted browsers. `chromedp.Flag(
  "disable-blink-features", "AutomationControlled")` is the obvious
  defense; they could add Cloudflare Turnstile or similar that
  blocks chromedp before the user even gets to type a password.

For now: stable enough to ship. Worth re-checking on a cadence.
